package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/store"
)

type RecoveryContainer struct {
	ID   string           `json:"id"`
	Name string           `json:"name"`
	Kind aphotos.NodeKind `json:"kind"`
}

type RecoverySources struct {
	Region         aphotos.Region         `json:"region"`
	Evidence       aphotos.EvidenceStatus `json:"evidence"`
	RootNodeID     string                 `json:"rootNodeId,omitempty"`
	ConfirmedItems []store.TransferItem   `json:"confirmedItems"`
	Folders        []RecoveryContainer    `json:"folders"`
	Albums         []RecoveryContainer    `json:"albums"`
	Warning        string                 `json:"warning,omitempty"`
}

type DownloadItemSpec struct {
	NodeID       string          `json:"nodeId"`
	Name         string          `json:"name,omitempty"`
	ExpectedSize int64           `json:"expectedSize,omitempty"`
	ExpectedMD5  string          `json:"expectedMd5,omitempty"`
	MediaType    store.MediaType `json:"mediaType,omitempty"`
}

type DownloadJobRequest struct {
	NodeIDs         []string               `json:"nodeIds,omitempty"`
	Items           []DownloadItemSpec     `json:"items,omitempty"`
	ContainerNodeID string                 `json:"containerNodeId,omitempty"`
	DestDir         string                 `json:"destDir"`
	ConflictPolicy  aphotos.ConflictPolicy `json:"conflictPolicy,omitempty"`
}

func (e *Engine) ListRecoverySources(ctx context.Context) (*RecoverySources, error) {
	st := e.Store()
	var confirmed []store.TransferItem
	if st != nil {
		var err error
		confirmed, err = st.ListConfirmedItems(ctx, 200)
		if err != nil {
			return nil, err
		}
	}

	destCap := e.ProbeDestination(ctx, "")
	out := &RecoverySources{
		Region:         destCap.Region,
		Evidence:       destCap.Evidence,
		ConfirmedItems: confirmed,
	}

	client := e.Client()
	if client == nil {
		out.Warning = "Offline: showing confirmed local transfer history only."
		return out, nil
	}

	out.RootNodeID = destCap.rootID

	if folders, _, err := client.ListFolders(ctx, "", 100, 0); err == nil {
		for i := range folders {
			f := &folders[i]
			if strings.TrimSpace(f.ID) == "" {
				continue
			}
			out.Folders = append(out.Folders, RecoveryContainer{
				ID:   f.ID,
				Name: f.Name,
				Kind: aphotos.NodeKindFolder,
			})
		}
	} else if out.Warning == "" {
		out.Warning = formatErrMsg(err)
	}

	if albums, _, err := client.ListAlbums(ctx, 100, 0); err == nil {
		for i := range albums {
			a := &albums[i]
			if strings.TrimSpace(a.ID) == "" {
				continue
			}
			out.Albums = append(out.Albums, RecoveryContainer{
				ID:   a.ID,
				Name: a.Name,
				Kind: aphotos.NodeKindVisualCollection,
			})
		}
	} else if out.Warning == "" {
		out.Warning = formatErrMsg(err)
	}

	if out.Evidence != aphotos.EvidenceVerified && out.Warning == "" {
		out.Warning = fmt.Sprintf("Region %q remote folder/album listing is experimental.", out.Region)
	}
	return out, nil
}

func (e *Engine) StartDownload(ctx context.Context, req DownloadJobRequest) (*store.Transfer, error) {
	st := e.Store()
	if st == nil {
		return nil, errors.New("state store is not initialized")
	}
	client := e.Client()
	if client == nil {
		return nil, ErrOffline
	}

	destDir := strings.TrimSpace(req.DestDir)
	if destDir == "" {
		return nil, &aphotos.ConfigError{
			Field:   "dest_dir",
			Message: "download destination directory is required",
		}
	}
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return nil, fmt.Errorf("resolve destination directory %q: %w", destDir, err)
	}
	if err := os.MkdirAll(absDestDir, 0o755); err != nil {
		return nil, fmt.Errorf("create destination directory: %w", err)
	}

	specs := make([]DownloadItemSpec, 0, len(req.Items)+len(req.NodeIDs))
	seen := make(map[string]struct{})

	for _, it := range req.Items {
		id := strings.TrimSpace(it.NodeID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		specs = append(specs, it)
	}

	for _, rawID := range req.NodeIDs {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		specs = append(specs, DownloadItemSpec{NodeID: id})
	}

	if containerID := strings.TrimSpace(req.ContainerNodeID); containerID != "" {
		page, cErr := client.ListAllChildren(ctx, containerID, aphotos.ChildrenOptions{
			Limit: 200,
		})
		if cErr != nil {
			return nil, cErr
		}
		for i := range page.Items {
			node := &page.Items[i]
			if node.Kind != "" && node.Kind != aphotos.NodeKindFile {
				continue
			}
			id := strings.TrimSpace(node.ID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			var expSize int64
			var expMD5, contentType string
			if node.ContentProperties != nil {
				expSize = node.ContentProperties.Size
				expMD5 = node.ContentProperties.MD5
				contentType = node.ContentProperties.ContentType
			}
			mt := store.MediaTypePhoto
			if m, ok := ClassifyMediaFile(node.Name); ok {
				mt = m
			} else if strings.HasPrefix(strings.ToLower(contentType), "video/") {
				mt = store.MediaTypeVideo
			}
			specs = append(specs, DownloadItemSpec{
				NodeID:       id,
				Name:         node.Name,
				ExpectedSize: expSize,
				ExpectedMD5:  expMD5,
				MediaType:    mt,
			})
		}
	}

	if len(specs) == 0 {
		return nil, &aphotos.ConfigError{
			Field:   "node_ids",
			Message: "no remote files selected for download",
		}
	}

	// Enrich any bare node IDs from local confirmed transfer items when available
	if confirmed, cErr := st.ListConfirmedItems(ctx, 500); cErr == nil && len(confirmed) > 0 {
		byNode := make(map[string]store.TransferItem, len(confirmed))
		for i := range confirmed {
			c := confirmed[i]
			byNode[c.RemoteNodeID] = c
		}
		for i := range specs {
			if prev, ok := byNode[specs[i].NodeID]; ok {
				if specs[i].Name == "" {
					specs[i].Name = prev.Name
				}
				if specs[i].ExpectedSize == 0 {
					specs[i].ExpectedSize = prev.Size
				}
				if specs[i].ExpectedMD5 == "" {
					specs[i].ExpectedMD5 = prev.MD5
				}
				if specs[i].MediaType == "" {
					specs[i].MediaType = prev.MediaType
				}
			}
		}
	}

	now := time.Now().UTC()
	trID := uuid.NewString()
	var totalBytes int64
	items := make([]store.TransferItem, 0, len(specs))
	for _, sp := range specs {
		name := strings.TrimSpace(sp.Name)
		if name == "" {
			name = sp.NodeID
		}
		mt := sp.MediaType
		if mt == "" {
			if classified, ok := ClassifyMediaFile(name); ok {
				mt = classified
			} else {
				mt = store.MediaTypePhoto
			}
		}
		totalBytes += sp.ExpectedSize
		items = append(items, store.TransferItem{
			ID:           uuid.NewString(),
			TransferID:   trID,
			LocalPath:    filepath.Join(absDestDir, name),
			Name:         name,
			MediaType:    mt,
			Size:         sp.ExpectedSize,
			MD5:          strings.ToLower(strings.TrimSpace(sp.ExpectedMD5)),
			RemoteNodeID: sp.NodeID,
			Status:       store.ItemStatusQueued,
			UpdatedAt:    now,
		})
	}

	conflictPolicy := req.ConflictPolicy
	if conflictPolicy == "" {
		if cfg, sErr := st.GetSettings(ctx); sErr == nil && cfg.DefaultConflictPolicy != "" {
			conflictPolicy = cfg.DefaultConflictPolicy
		}
		if conflictPolicy == "" {
			conflictPolicy = aphotos.ConflictSkip
		}
	}

	summary := fmt.Sprintf("Recovery download (%d items)", len(items))
	tr, err := st.CreateTransfer(ctx, store.Transfer{
		ID:             trID,
		Kind:           store.TransferKindDownload,
		Stage:          store.StageQueued,
		SourceSummary:  summary,
		DestNodeID:     strings.TrimSpace(req.ContainerNodeID),
		DestLocalDir:   absDestDir,
		ConflictPolicy: conflictPolicy,
		BytesTotal:     totalBytes,
		ItemsTotal:     len(items),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, items)
	if err != nil {
		return nil, err
	}

	e.emitTransferStage(tr)
	e.launchDownloadWorkers(tr.ID)
	return &tr, nil
}

func (e *Engine) launchDownloadWorkers(transferID string) {
	runCtx, cancel := context.WithCancel(context.Background())
	ctrl := newTransferControl(cancel)

	e.activeMu.Lock()
	if existing, ok := e.active[transferID]; ok && existing == ctrl {
		existing.setPaused(false)
		e.activeMu.Unlock()
		cancel()
		return
	}
	e.active[transferID] = ctrl
	e.activeMu.Unlock()

	e.bgWG.Go(func() {
		defer func() {
			e.activeMu.Lock()
			if e.active[transferID] == ctrl {
				delete(e.active, transferID)
			}
			e.activeMu.Unlock()
			close(ctrl.doneCh)
			cancel()
		}()
		e.executeDownloadTransfer(runCtx, transferID, ctrl)
	})
}

func (e *Engine) executeDownloadTransfer(ctx context.Context, transferID string, ctrl *activeTransferControl) {
	st := e.Store()
	if st == nil {
		return
	}
	tr, err := st.GetTransfer(ctx, transferID)
	if err != nil {
		return
	}
	if !ctrl.isPaused() && tr.Stage != store.StagePaused {
		tr.Stage = store.StageRunning
		tr.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransfer(ctx, tr)
		e.emitTransferStage(tr)
	}

	items, err := st.ListTransferItems(ctx, transferID)
	if err != nil {
		return
	}

	workerCount := 2
	if cfg, sErr := st.GetSettings(ctx); sErr == nil && cfg.MaxUploadConcurrency > 0 {
		workerCount = cfg.MaxUploadConcurrency
	}
	if workerCount > len(items) && len(items) > 0 {
		workerCount = len(items)
	}
	if workerCount <= 0 {
		workerCount = 1
	}

	indexCh := make(chan int, len(items))
	for i := range items {
		if items[i].Status == store.ItemStatusQueued {
			indexCh <- i
		}
	}
	close(indexCh)

	var wg sync.WaitGroup
	for range workerCount {
		wg.Go(func() {
			for idx := range indexCh {
				if wErr := ctrl.waitIfPaused(ctx); wErr != nil {
					return
				}
				e.processSingleDownloadItem(ctx, st, tr, items[idx])
			}
		})
	}
	wg.Wait()

	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	finalTr, err := recomputeAndPersistTransfer(context.Background(), st, transferID, "")
	if err == nil {
		e.emitTransferStage(finalTr)
	}
}

func (e *Engine) processSingleDownloadItem(
	ctx context.Context,
	st *store.Store,
	tr store.Transfer,
	item store.TransferItem,
) {
	client := e.Client()
	if client == nil {
		item.Status = store.ItemStatusFailed
		item.ErrorMessage = ErrOffline.Error()
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(context.Background(), item)
		return
	}

	fileName := strings.TrimSpace(item.Name)
	if fileName == "" {
		fileName = item.RemoteNodeID
	}

	policy := tr.ConflictPolicy
	if policy == "" {
		policy = aphotos.ConflictSkip
	}

	if fileName != "" {
		targetPath := filepath.Join(tr.DestLocalDir, fileName)
		if existingInfo, statErr := os.Stat(targetPath); statErr == nil {
			switch policy {
			case aphotos.ConflictSkip:
				item.Status = store.ItemStatusSkipped
				item.LocalPath = targetPath
				if item.Size <= 0 {
					item.Size = existingInfo.Size()
				}
				item.BytesDone = item.Size
				item.ErrorMessage = ""
				item.UpdatedAt = time.Now().UTC()
				_ = st.UpdateTransferItem(context.Background(), item)
				if curTr, rErr := recomputeAndPersistTransfer(context.Background(), st, tr.ID, ""); rErr == nil {
					e.emitTransferProgress(curTr, item)
				}
				return
			case aphotos.ConflictFail:
				item.Status = store.ItemStatusFailed
				item.ErrorMessage = fmt.Sprintf("local file already exists: %s", fileName)
				item.UpdatedAt = time.Now().UTC()
				_ = st.UpdateTransferItem(context.Background(), item)
				if curTr, rErr := recomputeAndPersistTransfer(context.Background(), st, tr.ID, ""); rErr == nil {
					e.emitTransferProgress(curTr, item)
				}
				return
			case aphotos.ConflictRename:
				fileName = disambiguateLocalFileName(tr.DestLocalDir, fileName)
				item.Name = fileName
				item.LocalPath = filepath.Join(tr.DestLocalDir, fileName)
			}
		}
	}

	item.Status = store.ItemStatusRunning
	item.UpdatedAt = time.Now().UTC()
	_ = st.UpdateTransferItem(context.Background(), item)

	if fileName == item.RemoteNodeID {
		fileName = ""
	}
	res, dlErr := client.Download(ctx, aphotos.DownloadRequest{
		NodeID:         item.RemoteNodeID,
		DestDir:        tr.DestLocalDir,
		FileName:       fileName,
		ExpectedSize:   item.Size,
		ExpectedMD5:    item.MD5,
		ConflictPolicy: policy,
		Progress: func(p aphotos.TransferProgress) {
			item.BytesDone = p.BytesTransferred
			e.emitLiveByteProgress(tr, item)
		},
	})

	switch {
	case dlErr == nil && res != nil && (res.Status == aphotos.TransferConfirmed || res.Status == aphotos.TransferSkipped):
		item.Status = store.ItemStatusConfirmed
		if res.Status == aphotos.TransferSkipped {
			item.Status = store.ItemStatusSkipped
		}
		if res.LocalPath != "" {
			item.LocalPath = res.LocalPath
		}
		if res.Name != "" {
			item.Name = res.Name
		}
		if res.Bytes > 0 {
			item.Size = res.Bytes
			item.BytesDone = res.Bytes
		}
		if res.MD5 != "" {
			item.MD5 = res.MD5
		}
		item.ErrorMessage = ""
	case errors.Is(dlErr, context.Canceled):
		item.Status = store.ItemStatusCancelled
		item.ErrorMessage = "cancelled"

	default:
		item.Status = store.ItemStatusFailed
		item.ErrorMessage = formatErrMsg(dlErr)
		if item.ErrorMessage == "" && res != nil && res.ErrorMessage != "" {
			item.ErrorMessage = aphotos.Redact(res.ErrorMessage)
		}
	}

	item.UpdatedAt = time.Now().UTC()
	_ = st.UpdateTransferItem(context.Background(), item)
	if curTr, rErr := recomputeAndPersistTransfer(context.Background(), st, tr.ID, ""); rErr == nil {
		e.emitTransferProgress(curTr, item)
	}
}

func disambiguateLocalFileName(destDir, fileName string) string {
	ext := filepath.Ext(fileName)
	stem := strings.TrimSuffix(fileName, ext)
	for idx := 1; idx < 10000; idx++ {
		cand := fmt.Sprintf("%s (%d)%s", stem, idx, ext)
		if _, err := os.Stat(filepath.Join(destDir, cand)); errors.Is(err, os.ErrNotExist) {
			return cand
		}
	}
	return fmt.Sprintf("%s (%s)%s", stem, uuid.NewString()[:8], ext)
}
