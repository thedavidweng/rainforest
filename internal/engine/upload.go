package engine

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/store"
)

type UploadJobRequest struct {
	Plan              *UploadPlan
	Paths             []string
	DestinationNodeID string
	ConflictPolicy    aphotos.ConflictPolicy
	IncludeVideos     *bool
}

func (e *Engine) StartUpload(ctx context.Context, req UploadJobRequest) (*store.Transfer, error) {
	st := e.Store()
	if st == nil {
		return nil, errors.New("state store is not initialized")
	}
	if e.Client() == nil {
		return nil, ErrOffline
	}

	plan := req.Plan
	if plan == nil {
		var err error
		plan, err = e.PlanUpload(ctx, req.Paths, PlanOptions{
			DestinationNodeID: req.DestinationNodeID,
			ConflictPolicy:    req.ConflictPolicy,
			IncludeVideos:     req.IncludeVideos,
			IncludePreviews:   false,
		})
		if err != nil {
			return nil, err
		}
	}
	if len(plan.Items) == 0 {
		return nil, &aphotos.ConfigError{
			Field:   "paths",
			Message: "no supported photo or video files found to upload",
		}
	}

	now := time.Now().UTC()
	trID := uuid.NewString()
	items := make([]store.TransferItem, 0, len(plan.Items))
	for _, pi := range plan.Items {
		items = append(items, store.TransferItem{
			ID:         uuid.NewString(),
			TransferID: trID,
			LocalPath:  pi.LocalPath,
			Name:       pi.Name,
			MediaType:  pi.MediaType,
			Size:       pi.Size,
			Status:     store.ItemStatusQueued,
			UpdatedAt:  now,
		})
	}

	tr, err := st.CreateTransfer(ctx, store.Transfer{
		ID:             trID,
		Kind:           store.TransferKindUpload,
		Stage:          store.StageQueued,
		SourceSummary:  plan.SourceSummary,
		DestNodeID:     plan.Destination.DestinationNodeID,
		ConflictPolicy: plan.ConflictPolicy,
		BytesTotal:     plan.TotalBytes,
		ItemsTotal:     len(items),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, items)
	if err != nil {
		return nil, err
	}

	e.emitTransferStage(tr)
	e.launchUploadWorkers(tr.ID, false)
	return &tr, nil
}

func (e *Engine) GetTransfer(ctx context.Context, id string) (store.Transfer, error) {
	st := e.Store()
	if st == nil {
		return store.Transfer{}, store.ErrNotFound
	}
	return st.GetTransfer(ctx, id)
}

func (e *Engine) ListTransfers(ctx context.Context, limit int) ([]store.Transfer, error) {
	st := e.Store()
	if st == nil {
		return nil, nil
	}
	return st.ListTransfers(ctx, limit)
}

func (e *Engine) ListTransferItems(ctx context.Context, transferID string) ([]store.TransferItem, error) {
	st := e.Store()
	if st == nil {
		return nil, nil
	}
	return st.ListTransferItems(ctx, transferID)
}

func (e *Engine) ClearFinishedTransfers(ctx context.Context) error {
	st := e.Store()
	if st == nil {
		return nil
	}
	return st.ClearFinishedTransfers(ctx)
}

func (e *Engine) PauseTransfer(ctx context.Context, transferID string) error {
	st := e.Store()
	if st == nil {
		return store.ErrNotFound
	}
	tr, err := st.GetTransfer(ctx, transferID)
	if err != nil {
		return err
	}
	if store.IsTerminalStage(tr.Stage) {
		return nil
	}

	e.activeMu.Lock()
	ctrl := e.active[transferID]
	e.activeMu.Unlock()
	if ctrl != nil {
		ctrl.setPaused(true)
	}

	tr.Stage = store.StagePaused
	tr.UpdatedAt = time.Now().UTC()
	if err := st.UpdateTransfer(ctx, tr); err != nil {
		return err
	}
	e.emitTransferStage(tr)
	return nil
}

func (e *Engine) ResumeTransfer(ctx context.Context, transferID string) error {
	st := e.Store()
	if st == nil {
		return store.ErrNotFound
	}
	tr, err := st.GetTransfer(ctx, transferID)
	if err != nil {
		return err
	}
	if store.IsTerminalStage(tr.Stage) {
		return nil
	}

	tr.Stage = store.StageRunning
	tr.UpdatedAt = time.Now().UTC()
	if err := st.UpdateTransfer(ctx, tr); err != nil {
		return err
	}
	e.emitTransferStage(tr)

	e.activeMu.Lock()
	ctrl := e.active[transferID]
	e.activeMu.Unlock()
	if ctrl != nil {
		ctrl.setPaused(false)
		return nil
	}

	if tr.Kind == store.TransferKindDownload {
		e.launchDownloadWorkers(transferID)
	} else {
		e.launchUploadWorkers(transferID, true)
	}
	return nil
}

func (e *Engine) CancelTransfer(ctx context.Context, transferID string) error {
	st := e.Store()
	if st == nil {
		return store.ErrNotFound
	}
	tr, err := st.GetTransfer(ctx, transferID)
	if err != nil {
		return err
	}
	if store.IsTerminalStage(tr.Stage) {
		return nil
	}

	e.activeMu.Lock()
	ctrl := e.active[transferID]
	e.activeMu.Unlock()
	if ctrl != nil {
		ctrl.setPaused(false)
		ctrl.cancel()
	}

	items, err := st.ListTransferItems(ctx, transferID)
	if err == nil {
		now := time.Now().UTC()
		for i := range items {
			it := &items[i]
			if it.Status == store.ItemStatusQueued || it.Status == store.ItemStatusRunning {
				it.Status = store.ItemStatusCancelled
				it.ErrorMessage = "cancelled by user"
				it.UpdatedAt = now
				_ = st.UpdateTransferItem(ctx, *it)
			}
		}
	}

	updated, err := recomputeAndPersistTransfer(ctx, st, transferID, store.StageCancelled)
	if err != nil {
		return err
	}
	e.emitTransferStage(updated)
	return nil
}

func (e *Engine) RetryTransfer(ctx context.Context, transferID string) (*store.Transfer, error) {
	st := e.Store()
	if st == nil {
		return nil, store.ErrNotFound
	}
	if e.Client() == nil {
		return nil, ErrOffline
	}
	tr, err := st.GetTransfer(ctx, transferID)
	if err != nil {
		return nil, err
	}
	items, err := st.ListTransferItems(ctx, transferID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	hasRetryable := false
	for i := range items {
		it := &items[i]
		if it.Status == store.ItemStatusFailed || it.Status == store.ItemStatusCancelled || it.Status == store.ItemStatusUnconfirmed || it.Status == store.ItemStatusQueued {
			it.Status = store.ItemStatusQueued
			it.ErrorMessage = ""
			it.BytesDone = 0
			it.UpdatedAt = now
			if err := st.UpdateTransferItem(ctx, *it); err != nil {
				return nil, err
			}
			hasRetryable = true
		}
	}
	if !hasRetryable {
		return &tr, nil
	}

	tr.Stage = store.StageQueued
	tr.ErrorCode = ""
	tr.ErrorMessage = ""
	tr.FinishedAt = nil
	tr.UpdatedAt = now
	if err := st.UpdateTransfer(ctx, tr); err != nil {
		return nil, err
	}
	e.emitTransferStage(tr)

	if tr.Kind == store.TransferKindDownload {
		e.launchDownloadWorkers(transferID)
	} else {
		e.launchUploadWorkers(transferID, true)
	}
	return &tr, nil
}

func (e *Engine) launchUploadWorkers(transferID string, checkRemoteBeforePost bool) {
	runCtx, cancel := context.WithCancel(context.Background())
	ctrl := newTransferControl(cancel)

	e.activeMu.Lock()
	if existing, ok := e.active[transferID]; ok {
		existing.setPaused(false)
		e.activeMu.Unlock()
		cancel()
		return
	}
	e.active[transferID] = ctrl
	e.activeMu.Unlock()

	e.bgWG.Add(1)
	go func() {
		defer e.bgWG.Done()
		defer func() {
			e.activeMu.Lock()
			delete(e.active, transferID)
			e.activeMu.Unlock()
			cancel()
		}()
		e.executeUploadTransfer(runCtx, transferID, ctrl, checkRemoteBeforePost)
	}()
}

func (e *Engine) executeUploadTransfer(ctx context.Context, transferID string, ctrl *activeTransferControl, checkRemoteBeforePost bool) {
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
		if items[i].Status == store.ItemStatusQueued || items[i].Status == store.ItemStatusUnconfirmed {
			indexCh <- i
		}
	}
	close(indexCh)

	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				if wErr := ctrl.waitIfPaused(ctx); wErr != nil {
					return
				}
				e.processSingleUploadItem(ctx, st, tr, items[idx], checkRemoteBeforePost)
			}
		}()
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

func (e *Engine) processSingleUploadItem(
	ctx context.Context,
	st *store.Store,
	tr store.Transfer,
	item store.TransferItem,
	checkRemoteBeforePost bool,
) {
	client := e.Client()
	if client == nil {
		item.Status = store.ItemStatusFailed
		item.ErrorMessage = ErrOffline.Error()
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(context.Background(), item)
		return
	}

	infoBefore, statErr := os.Stat(item.LocalPath)
	if statErr != nil {
		item.Status = store.ItemStatusFailed
		item.ErrorMessage = formatErrMsg(statErr)
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(context.Background(), item)
		return
	}
	mtimeBefore := infoBefore.ModTime().UnixNano()
	sizeBefore := infoBefore.Size()
	item.Size = sizeBefore

	if item.MD5 == "" {
		if md5Hex, hErr := computeFileMD5(item.LocalPath); hErr == nil {
			item.MD5 = md5Hex
		} else if errors.Is(hErr, errFileModifiedMidFlight) {
			item.Status = store.ItemStatusSkipped
			item.ErrorMessage = errFileModifiedMidFlight.Error()
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(context.Background(), item)
			return
		}
	}

	if checkRemoteBeforePost {
		if existing, fErr := findRemoteNodeForItem(ctx, client, item, tr.DestNodeID); fErr == nil && existing != nil && strings.TrimSpace(existing.ID) != "" {
			item.Status = store.ItemStatusConfirmed
			item.RemoteNodeID = existing.ID
			item.BytesDone = item.Size
			item.ErrorMessage = ""
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(context.Background(), item)
			if curTr, rErr := recomputeAndPersistTransfer(context.Background(), st, tr.ID, ""); rErr == nil {
				e.emitTransferProgress(curTr, item)
			}
			return
		}
	}

	item.Status = store.ItemStatusRunning
	item.UpdatedAt = time.Now().UTC()
	_ = st.UpdateTransferItem(context.Background(), item)

	res, upErr := client.UploadFile(ctx, item.LocalPath, aphotos.UploadOptions{
		Name:            item.Name,
		ParentNodeID:    tr.DestNodeID,
		AutoResolveRoot: strings.TrimSpace(tr.DestNodeID) == "",
		ConflictPolicy:  tr.ConflictPolicy,
		Progress: func(p aphotos.TransferProgress) {
			item.BytesDone = p.BytesTransferred
			e.emitTransferProgress(tr, item)
		},
	})

	infoAfter, afterErr := os.Stat(item.LocalPath)
	if afterErr != nil || infoAfter.Size() != sizeBefore || infoAfter.ModTime().UnixNano() != mtimeBefore {
		item.Status = store.ItemStatusUnconfirmed
		item.ErrorMessage = errFileModifiedMidFlight.Error()
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(context.Background(), item)
		return
	}

	if res != nil && res.MD5 != "" {
		item.MD5 = res.MD5
	}

	switch {
	case res != nil && res.Status == aphotos.TransferConfirmed && strings.TrimSpace(res.NodeID) != "":
		item.Status = store.ItemStatusConfirmed
		item.RemoteNodeID = res.NodeID
		item.BytesDone = item.Size
		item.ErrorMessage = ""
	case res != nil && res.Status == aphotos.TransferSkipped:
		item.Status = store.ItemStatusSkipped
		item.RemoteNodeID = res.NodeID
		item.BytesDone = item.Size
		item.ErrorMessage = ""
	case res != nil && res.Status == aphotos.TransferUnconfirmed:
		item.Status = store.ItemStatusUnconfirmed
		item.ErrorMessage = formatErrMsg(upErr)
		if item.ErrorMessage == "" && res.ErrorMessage != "" {
			item.ErrorMessage = aphotos.Redact(res.ErrorMessage)
		}
		if item.ErrorMessage == "" {
			item.ErrorMessage = "upload completed without confirmed remote node ID"
		}
	case errors.Is(upErr, context.Canceled):
		item.Status = store.ItemStatusCancelled
		item.ErrorMessage = "cancelled"
	default:
		item.Status = store.ItemStatusFailed
		item.ErrorMessage = formatErrMsg(upErr)
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
