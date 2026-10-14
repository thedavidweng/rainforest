package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/store"
)

type BackupRuleInput struct {
	LocalPath         string                 `json:"localPath"`
	IncludeVideos     bool                   `json:"includeVideos"`
	ScheduleMode      store.ScheduleMode     `json:"scheduleMode"`
	IntervalMinutes   int                    `json:"intervalMinutes"`
	ConflictPolicy    aphotos.ConflictPolicy `json:"conflictPolicy"`
	DestinationNodeID string                 `json:"destinationNodeId"`
	Paused            bool                   `json:"paused"`
}

func (e *Engine) ListBackupRules(ctx context.Context) ([]store.BackupRule, error) {
	st := e.Store()
	if st == nil {
		return nil, nil
	}
	return st.ListBackupRules(ctx)
}

func (e *Engine) AddBackupRule(ctx context.Context, input BackupRuleInput) (store.BackupRule, error) {
	st := e.Store()
	if st == nil {
		return store.BackupRule{}, errors.New("state store is not initialized")
	}
	cleaned := strings.TrimSpace(input.LocalPath)
	if cleaned == "" {
		return store.BackupRule{}, &aphotos.ConfigError{
			Field:   "local_path",
			Message: "backup folder path is required",
		}
	}
	absPath, err := filepath.Abs(cleaned)
	if err != nil {
		return store.BackupRule{}, fmt.Errorf("resolve backup path %q: %w", cleaned, err)
	}

	destCap := e.resolveDestinationCapability(ctx, input.DestinationNodeID)
	status := store.BackupStatusActive
	if input.Paused {
		status = store.BackupStatusPaused
	}

	now := time.Now().UTC()
	rule, err := st.CreateBackupRule(ctx, store.BackupRule{
		ID:                uuid.NewString(),
		LocalPath:         absPath,
		IncludeVideos:     input.IncludeVideos,
		ScheduleMode:      input.ScheduleMode,
		IntervalMinutes:   input.IntervalMinutes,
		ConflictPolicy:    input.ConflictPolicy,
		DestinationNodeID: destCap.DestinationNodeID,
		Paused:            input.Paused,
		Status:            status,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		return store.BackupRule{}, err
	}
	e.emitBackupStatus(rule)

	if !rule.Paused && !e.isBackupsStopped(ctx) {
		e.bgWG.Add(1)
		go func(ruleID string) {
			defer e.bgWG.Done()
			_, _ = e.ScanNow(context.Background(), ruleID)
		}(rule.ID)
	}
	return rule, nil
}

func (e *Engine) UpdateBackupRule(ctx context.Context, ruleID string, input BackupRuleInput) (store.BackupRule, error) {
	st := e.Store()
	if st == nil {
		return store.BackupRule{}, errors.New("state store is not initialized")
	}
	existing, err := st.GetBackupRule(ctx, ruleID)
	if err != nil {
		return store.BackupRule{}, err
	}

	absPath := existing.LocalPath
	if cleaned := strings.TrimSpace(input.LocalPath); cleaned != "" {
		resolved, rErr := filepath.Abs(cleaned)
		if rErr != nil {
			return store.BackupRule{}, fmt.Errorf("resolve backup path %q: %w", cleaned, rErr)
		}
		absPath = resolved
	}

	destCap := e.resolveDestinationCapability(ctx, input.DestinationNodeID)
	status := store.BackupStatusActive
	if input.Paused {
		status = store.BackupStatusPaused
	}

	existing.LocalPath = absPath
	existing.IncludeVideos = input.IncludeVideos
	existing.ScheduleMode = input.ScheduleMode
	existing.IntervalMinutes = input.IntervalMinutes
	existing.ConflictPolicy = input.ConflictPolicy
	existing.DestinationNodeID = destCap.DestinationNodeID
	existing.Paused = input.Paused
	existing.Status = status
	existing.UpdatedAt = time.Now().UTC()

	updated, err := st.UpdateBackupRule(ctx, existing)
	if err != nil {
		return store.BackupRule{}, err
	}
	e.emitBackupStatus(updated)

	if !updated.Paused && !e.isBackupsStopped(ctx) {
		e.bgWG.Add(1)
		go func(id string) {
			defer e.bgWG.Done()
			_, _ = e.ScanNow(context.Background(), id)
		}(updated.ID)
	}
	return updated, nil
}

func (e *Engine) PauseBackupRule(ctx context.Context, ruleID string) (store.BackupRule, error) {
	st := e.Store()
	if st == nil {
		return store.BackupRule{}, store.ErrNotFound
	}
	rule, err := st.GetBackupRule(ctx, ruleID)
	if err != nil {
		return store.BackupRule{}, err
	}
	rule.Paused = true
	rule.Status = store.BackupStatusPaused
	rule.UpdatedAt = time.Now().UTC()
	updated, err := st.UpdateBackupRule(ctx, rule)
	if err != nil {
		return store.BackupRule{}, err
	}
	e.cancelActiveBackupRuleTransfer(ctx, ruleID)
	e.emitBackupStatus(updated)
	return updated, nil
}

func (e *Engine) ResumeBackupRule(ctx context.Context, ruleID string) (store.BackupRule, error) {
	st := e.Store()
	if st == nil {
		return store.BackupRule{}, store.ErrNotFound
	}
	rule, err := st.GetBackupRule(ctx, ruleID)
	if err != nil {
		return store.BackupRule{}, err
	}
	rule.Paused = false
	rule.Status = store.BackupStatusActive
	rule.LastError = ""
	rule.UpdatedAt = time.Now().UTC()
	updated, err := st.UpdateBackupRule(ctx, rule)
	if err != nil {
		return store.BackupRule{}, err
	}
	e.emitBackupStatus(updated)

	if !e.isBackupsStopped(ctx) {
		e.bgWG.Add(1)
		go func(id string) {
			defer e.bgWG.Done()
			_, _ = e.ScanNow(context.Background(), id)
		}(updated.ID)
	}
	return updated, nil
}

func (e *Engine) RemoveBackupRule(ctx context.Context, ruleID string) error {
	st := e.Store()
	if st == nil {
		return store.ErrNotFound
	}
	e.cancelActiveBackupRuleTransfer(ctx, ruleID)
	return st.DeleteBackupRule(ctx, ruleID)
}

func (e *Engine) SetBackupsStopped(ctx context.Context, stopped bool) error {
	st := e.Store()
	if st == nil {
		return nil
	}
	cfg, err := st.GetSettings(ctx)
	if err != nil {
		return err
	}
	cfg.BackupsStopped = stopped
	if err := st.UpdateSettings(ctx, cfg); err != nil {
		return err
	}
	if stopped {
		e.cancelAllActiveBackupTransfers(ctx)
	}
	return nil
}

func (e *Engine) cancelActiveBackupRuleTransfer(ctx context.Context, ruleID string) {
	e.scanMu.Lock()
	trID := e.scanning[ruleID]
	e.scanMu.Unlock()
	if trID != "" {
		_ = e.CancelTransfer(ctx, trID)
	}
}

func (e *Engine) cancelAllActiveBackupTransfers(ctx context.Context) {
	e.scanMu.Lock()
	ids := make([]string, 0, len(e.scanning))
	for _, trID := range e.scanning {
		if trID != "" {
			ids = append(ids, trID)
		}
	}
	e.scanMu.Unlock()
	for _, trID := range ids {
		_ = e.CancelTransfer(ctx, trID)
	}
}

func (e *Engine) isBackupsStopped(ctx context.Context) bool {
	st := e.Store()
	if st == nil {
		return false
	}
	cfg, err := st.GetSettings(ctx)
	if err != nil {
		return false
	}
	return cfg.BackupsStopped
}

func (e *Engine) runScheduledBackupScans(ctx context.Context) {
	if e.isBackupsStopped(ctx) || e.Client() == nil {
		return
	}
	st := e.Store()
	if st == nil {
		return
	}
	rules, err := st.ListBackupRules(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for i := range rules {
		r := &rules[i]
		if r.Paused {
			continue
		}
		interval := time.Duration(max(1, r.IntervalMinutes)) * time.Minute
		if r.ScheduleMode == store.ScheduleImmediate {
			interval = 5 * time.Minute
		}
		if r.LastScanAt == nil || now.Sub(*r.LastScanAt) >= interval {
			_, _ = e.ScanNow(ctx, r.ID)
		}
	}
}

type backupCandidate struct {
	relPath       string
	absPath       string
	name          string
	mediaType     store.MediaType
	size          int64
	mtimeNs       int64
	reconcileOnly bool
	prevMD5       string
	existingItem  *store.TransferItem
}

func (e *Engine) ScanNow(ctx context.Context, ruleID string) (*store.Transfer, error) {
	st := e.Store()
	if st == nil {
		return nil, store.ErrNotFound
	}

	e.scanMu.Lock()
	if _, busy := e.scanning[ruleID]; busy {
		e.scanMu.Unlock()
		return nil, nil
	}
	e.scanning[ruleID] = ""
	e.scanMu.Unlock()
	defer func() {
		e.scanMu.Lock()
		delete(e.scanning, ruleID)
		e.scanMu.Unlock()
	}()

	rule, err := st.GetBackupRule(ctx, ruleID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	rootInfo, statErr := os.Stat(rule.LocalPath)
	if statErr != nil || !rootInfo.IsDir() {
		errMsg := "backup source folder is unavailable or disconnected"
		if statErr != nil {
			errMsg = formatErrMsg(statErr)
		}
		_ = st.SetBackupRuleStatus(ctx, rule.ID, store.BackupStatusSourceUnavailable, errMsg, &now)
		if updated, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
			e.emitBackupStatus(updated)
		}
		return nil, fmt.Errorf("backup source unavailable: %s", errMsg)
	}

	_ = st.SetBackupRuleStatus(ctx, rule.ID, store.BackupStatusScanning, "", nil)
	if scanningRule, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
		e.emitBackupStatus(scanningRule)
	}

	trackedList, err := st.ListTrackedFiles(ctx, rule.ID)
	if err != nil {
		_ = st.SetBackupRuleStatus(ctx, rule.ID, store.BackupStatusError, formatErrMsg(err), &now)
		return nil, err
	}
	trackedByRel := make(map[string]store.TrackedFile, len(trackedList))
	for i := range trackedList {
		tf := trackedList[i]
		trackedByRel[tf.RelPath] = tf
	}

	var openUnconfirmedByPath map[string]store.TransferItem
	if reconTransfers, rErr := st.ListTransfersNeedingReconciliation(ctx); rErr == nil {
		for i := range reconTransfers {
			rt := &reconTransfers[i]
			if rt.Kind != store.TransferKindBackup {
				continue
			}
			if rItems, liErr := st.ListTransferItems(ctx, rt.ID); liErr == nil {
				for j := range rItems {
					it := rItems[j]
					if it.Status == store.ItemStatusUnconfirmed && strings.TrimSpace(it.LocalPath) != "" {
						if openUnconfirmedByPath == nil {
							openUnconfirmedByPath = make(map[string]store.TransferItem)
						}
						openUnconfirmedByPath[it.LocalPath] = it
					}
				}
			}
		}
	}

	var candidates []backupCandidate
	var scanWarnings []string
	walkErr := filepath.WalkDir(rule.LocalPath, func(p string, d fs.DirEntry, wErr error) error {
		if wErr != nil {
			if p == rule.LocalPath {
				return wErr
			}
			if !errors.Is(wErr, os.ErrNotExist) {
				scanWarnings = append(scanWarnings, fmt.Sprintf("%s: %s", filepath.Base(p), formatErrMsg(wErr)))
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if p != rule.LocalPath && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") || !d.Type().IsRegular() {
			return nil
		}
		mt, supported := ClassifyMediaFile(d.Name())
		if !supported || (!rule.IncludeVideos && mt == store.MediaTypeVideo) {
			return nil
		}

		info, iErr := d.Info()
		if iErr != nil {
			if !errors.Is(iErr, os.ErrNotExist) {
				scanWarnings = append(scanWarnings, fmt.Sprintf("%s: %s", d.Name(), formatErrMsg(iErr)))
			}
			return nil
		}
		rel, rErr := filepath.Rel(rule.LocalPath, p)
		if rErr != nil {
			scanWarnings = append(scanWarnings, fmt.Sprintf("%s: %s", d.Name(), formatErrMsg(rErr)))
			return nil
		}
		rel = filepath.ToSlash(rel)

		size := info.Size()
		mtimeNs := info.ModTime().UnixNano()
		var reconcileOnly bool
		var prevMD5 string
		var existingItem *store.TransferItem
		if prev, ok := trackedByRel[rel]; ok {
			if prev.Status == store.ItemStatusConfirmed && prev.Size == size && prev.MtimeNs == mtimeNs {
				return nil
			}
			if prev.Status == store.ItemStatusUnconfirmed {
				reconcileOnly = true
				prevMD5 = prev.MD5
				if ex, found := openUnconfirmedByPath[p]; found {
					cp := ex
					existingItem = &cp
				}
			}
		}

		candidates = append(candidates, backupCandidate{
			relPath:       rel,
			absPath:       p,
			name:          d.Name(),
			mediaType:     mt,
			size:          size,
			mtimeNs:       mtimeNs,
			reconcileOnly: reconcileOnly,
			prevMD5:       prevMD5,
			existingItem:  existingItem,
		})
		return nil
	})
	if walkErr != nil {
		status := store.BackupStatusError
		if _, sErr := os.Stat(rule.LocalPath); sErr != nil {
			status = store.BackupStatusSourceUnavailable
		}
		_ = st.SetBackupRuleStatus(ctx, rule.ID, status, formatErrMsg(walkErr), &now)
		if updated, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
			e.emitBackupStatus(updated)
		}
		return nil, walkErr
	}

	var uploadCandidates []backupCandidate
	var reconcileCandidates []backupCandidate
	for _, c := range candidates {
		if c.reconcileOnly {
			reconcileCandidates = append(reconcileCandidates, c)
		} else {
			uploadCandidates = append(uploadCandidates, c)
		}
	}

	client := e.Client()
	if len(reconcileCandidates) > 0 && client != nil {
		for _, rc := range reconcileCandidates {
			probeItem := store.TransferItem{
				LocalPath: rc.absPath,
				Name:      rc.name,
				Size:      rc.size,
				MD5:       rc.prevMD5,
			}
			node, fErr := findRemoteNodeForItem(ctx, client, probeItem, rule.DestinationNodeID)
			if fErr == nil && node != nil && strings.TrimSpace(node.ID) != "" {
				verifiedMD5 := rc.prevMD5
				if node.ContentProperties != nil && strings.TrimSpace(node.ContentProperties.MD5) != "" {
					verifiedMD5 = strings.ToLower(strings.TrimSpace(node.ContentProperties.MD5))
				}
				syncedAt := time.Now().UTC()
				_ = st.UpsertTrackedFile(ctx, store.TrackedFile{
					BackupRuleID: rule.ID,
					RelPath:      rc.relPath,
					AbsPath:      rc.absPath,
					Size:         rc.size,
					MtimeNs:      rc.mtimeNs,
					MD5:          verifiedMD5,
					RemoteNodeID: node.ID,
					Status:       store.ItemStatusConfirmed,
					LastSyncedAt: syncedAt,
				})
				if rc.existingItem != nil {
					rc.existingItem.Status = store.ItemStatusConfirmed
					rc.existingItem.RemoteNodeID = node.ID
					rc.existingItem.MD5 = verifiedMD5
					rc.existingItem.BytesDone = rc.existingItem.Size
					rc.existingItem.ErrorMessage = ""
					rc.existingItem.UpdatedAt = syncedAt
					_ = st.UpdateTransferItem(ctx, *rc.existingItem)
					if updTr, uErr := recomputeAndPersistTransfer(ctx, st, rc.existingItem.TransferID, ""); uErr == nil {
						e.emitTransferStage(updTr)
					}
				}
				continue
			}
			if rc.existingItem == nil {
				uploadCandidates = append(uploadCandidates, rc)
			} else {
				scanWarnings = append(scanWarnings, fmt.Sprintf("%s: awaiting upload confirmation (will not auto-replay without manual retry)", rc.name))
			}
		}
	} else if len(reconcileCandidates) > 0 && client == nil {
		for _, rc := range reconcileCandidates {
			scanWarnings = append(scanWarnings, fmt.Sprintf("%s: awaiting upload confirmation (offline)", rc.name))
		}
	}

	candidates = uploadCandidates

	if len(candidates) == 0 {
		scanDone := time.Now().UTC()
		if len(scanWarnings) > 0 {
			warnMsg := strings.Join(scanWarnings, "; ")
			_ = st.SetBackupRuleStatus(ctx, rule.ID, store.BackupStatusError, warnMsg, &scanDone)
			if updated, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
				e.emitBackupStatus(updated)
			}
			return nil, fmt.Errorf("backup scan incomplete: %s", warnMsg)
		}
		finalStatus := store.BackupStatusActive
		if rule.Paused {
			finalStatus = store.BackupStatusPaused
		}
		_ = st.SetBackupRuleStatus(ctx, rule.ID, finalStatus, "", &scanDone)
		if updated, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
			e.emitBackupStatus(updated)
		}
		return nil, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].relPath < candidates[j].relPath
	})

	if client == nil {
		_ = st.SetBackupRuleStatus(ctx, rule.ID, store.BackupStatusError, ErrOffline.Error(), &now)
		if updated, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
			e.emitBackupStatus(updated)
		}
		return nil, ErrOffline
	}

	trID := uuid.NewString()
	var totalBytes int64
	items := make([]store.TransferItem, 0, len(candidates))
	for _, c := range candidates {
		totalBytes += c.size
		items = append(items, store.TransferItem{
			ID:         uuid.NewString(),
			TransferID: trID,
			LocalPath:  c.absPath,
			Name:       c.name,
			MediaType:  c.mediaType,
			Size:       c.size,
			MD5:        c.prevMD5,
			Status:     store.ItemStatusQueued,
			UpdatedAt:  now,
		})
	}
	tr, err := st.CreateTransfer(ctx, store.Transfer{
		ID:             trID,
		Kind:           store.TransferKindBackup,
		Stage:          store.StageRunning,
		SourceSummary:  fmt.Sprintf("Backup %s (%d items)", filepath.Base(rule.LocalPath), len(items)),
		DestNodeID:     rule.DestinationNodeID,
		ConflictPolicy: rule.ConflictPolicy,
		BytesTotal:     totalBytes,
		ItemsTotal:     len(items),
		CreatedAt:      now,
		UpdatedAt:      now,
	}, items)
	if err != nil {
		return nil, err
	}
	e.emitTransferStage(tr)
	e.scanMu.Lock()
	e.scanning[ruleID] = tr.ID
	e.scanMu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	ctrl := newTransferControl(cancel)
	e.activeMu.Lock()
	e.active[tr.ID] = ctrl
	e.activeMu.Unlock()
	defer func() {
		e.activeMu.Lock()
		delete(e.active, tr.ID)
		e.activeMu.Unlock()
		cancel()
	}()

	workerCount := 2
	if cfg, sErr := st.GetSettings(ctx); sErr == nil && cfg.MaxUploadConcurrency > 0 {
		workerCount = cfg.MaxUploadConcurrency
	}
	if workerCount > len(candidates) && len(candidates) > 0 {
		workerCount = len(candidates)
	}
	if workerCount <= 0 {
		workerCount = 1
	}

	indexCh := make(chan int, len(candidates))
	for i := range candidates {
		indexCh <- i
	}
	close(indexCh)

	var errMu sync.Mutex
	var lastRuleErr string
	if len(scanWarnings) > 0 {
		lastRuleErr = strings.Join(scanWarnings, "; ")
	}
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range indexCh {
				if wErr := ctrl.waitIfPaused(runCtx); wErr != nil {
					return
				}
				if e.isBackupsStopped(runCtx) {
					cancel()
					return
				}
				itemErr := e.processSingleBackupItem(runCtx, st, client, rule, tr, candidates[idx], items[idx])
				if itemErr != "" {
					errMu.Lock()
					if lastRuleErr == "" {
						lastRuleErr = itemErr
					}
					errMu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	bgCtx := context.Background()
	if errors.Is(runCtx.Err(), context.Canceled) {
		remItems, lErr := st.ListTransferItems(bgCtx, tr.ID)
		if lErr == nil {
			cancelNow := time.Now().UTC()
			for i := range remItems {
				it := &remItems[i]
				if it.Status == store.ItemStatusQueued || it.Status == store.ItemStatusRunning {
					it.Status = store.ItemStatusCancelled
					it.ErrorMessage = "cancelled"
					it.UpdatedAt = cancelNow
					_ = st.UpdateTransferItem(bgCtx, *it)
				}
			}
		}
	}

	finalTr, _ := recomputeAndPersistTransfer(bgCtx, st, tr.ID, "")
	e.emitTransferStage(finalTr)
	scanDone := time.Now().UTC()
	if lastRuleErr == "" && (finalTr.Stage == store.StageFailed || finalTr.Stage == store.StagePartial || finalTr.Stage == store.StageUnconfirmed) {
		lastRuleErr = finalTr.ErrorMessage
		if lastRuleErr == "" {
			lastRuleErr = fmt.Sprintf("backup transfer finished in stage %s", finalTr.Stage)
		}
	}
	finalRuleStatus := store.BackupStatusActive
	if rule.Paused {
		finalRuleStatus = store.BackupStatusPaused
	} else if lastRuleErr != "" {
		finalRuleStatus = store.BackupStatusError
	}
	_ = st.SetBackupRuleStatus(ctx, rule.ID, finalRuleStatus, lastRuleErr, &scanDone)
	if updated, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
		e.emitBackupStatus(updated)
	}
	if len(scanWarnings) > 0 {
		return &finalTr, fmt.Errorf("backup scan completed with errors: %s", strings.Join(scanWarnings, "; "))
	}
	return &finalTr, nil
}

func (e *Engine) processSingleBackupItem(
	ctx context.Context,
	st *store.Store,
	client *aphotos.Client,
	rule store.BackupRule,
	tr store.Transfer,
	c backupCandidate,
	item store.TransferItem,
) string {
	bgCtx := context.Background()
	statBefore, sErr := os.Stat(c.absPath)
	if sErr != nil {
		item.Status = store.ItemStatusSkipped
		item.ErrorMessage = formatErrMsg(sErr)
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)
		return ""
	}
	sizeBefore := statBefore.Size()
	mtimeBefore := statBefore.ModTime().UnixNano()
	if sizeBefore != c.size || mtimeBefore != c.mtimeNs {
		item.Status = store.ItemStatusSkipped
		item.ErrorMessage = errFileModifiedMidFlight.Error()
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)
		return ""
	}

	if c.reconcileOnly {
		probeItem := item
		if probeItem.MD5 == "" {
			probeItem.MD5 = c.prevMD5
		}
		if node, fErr := findRemoteNodeForItem(ctx, client, probeItem, rule.DestinationNodeID); fErr == nil && node != nil && strings.TrimSpace(node.ID) != "" {
			verifiedMD5 := probeItem.MD5
			if node.ContentProperties != nil && strings.TrimSpace(node.ContentProperties.MD5) != "" {
				verifiedMD5 = strings.ToLower(strings.TrimSpace(node.ContentProperties.MD5))
			}
			item.Status = store.ItemStatusConfirmed
			item.RemoteNodeID = node.ID
			item.MD5 = verifiedMD5
			item.BytesDone = sizeBefore
			item.ErrorMessage = ""
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(bgCtx, item)
			_ = st.UpsertTrackedFile(bgCtx, store.TrackedFile{
				BackupRuleID: rule.ID,
				RelPath:      c.relPath,
				AbsPath:      c.absPath,
				Size:         sizeBefore,
				MtimeNs:      mtimeBefore,
				MD5:          verifiedMD5,
				RemoteNodeID: node.ID,
				Status:       store.ItemStatusConfirmed,
				LastSyncedAt: item.UpdatedAt,
			})
			return ""
		}
		item.Status = store.ItemStatusUnconfirmed
		item.MD5 = c.prevMD5
		item.ErrorMessage = "awaiting remote upload confirmation; will not auto-replay without manual retry"
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)
		return item.ErrorMessage
	}

	item.Status = store.ItemStatusRunning
	item.UpdatedAt = time.Now().UTC()
	_ = st.UpdateTransferItem(bgCtx, item)

	res, upErr := client.UploadFile(ctx, c.absPath, aphotos.UploadOptions{
		Name:            c.name,
		ParentNodeID:    rule.DestinationNodeID,
		AutoResolveRoot: strings.TrimSpace(rule.DestinationNodeID) == "",
		ConflictPolicy:  rule.ConflictPolicy,
		Progress: func(p aphotos.TransferProgress) {
			item.BytesDone = p.BytesTransferred
			e.emitTransferProgress(tr, item)
		},
	})

	md5Hex := ""
	if res != nil {
		md5Hex = res.MD5
	}

	statAfter, afterErr := os.Stat(c.absPath)
	if afterErr != nil || statAfter.Size() != sizeBefore || statAfter.ModTime().UnixNano() != mtimeBefore {
		item.Status = store.ItemStatusUnconfirmed
		item.MD5 = md5Hex
		if res != nil && strings.TrimSpace(res.NodeID) != "" {
			item.RemoteNodeID = res.NodeID
		}
		item.ErrorMessage = errFileModifiedMidFlight.Error()
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)
		_ = st.UpsertTrackedFile(bgCtx, store.TrackedFile{
			BackupRuleID: rule.ID,
			RelPath:      c.relPath,
			AbsPath:      c.absPath,
			Size:         sizeBefore,
			MtimeNs:      mtimeBefore,
			MD5:          md5Hex,
			RemoteNodeID: item.RemoteNodeID,
			Status:       store.ItemStatusUnconfirmed,
			LastSyncedAt: item.UpdatedAt,
		})
		return item.ErrorMessage
	}
	switch {
	case res != nil && (res.Status == aphotos.TransferConfirmed || res.Status == aphotos.TransferSkipped):
		item.Status = store.ItemStatusConfirmed
		if res.Status == aphotos.TransferSkipped {
			item.Status = store.ItemStatusSkipped
		}
		item.RemoteNodeID = res.NodeID
		item.MD5 = md5Hex
		item.BytesDone = sizeBefore
		item.ErrorMessage = ""
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)

		_ = st.UpsertTrackedFile(bgCtx, store.TrackedFile{
			BackupRuleID: rule.ID,
			RelPath:      c.relPath,
			AbsPath:      c.absPath,
			Size:         sizeBefore,
			MtimeNs:      mtimeBefore,
			MD5:          md5Hex,
			RemoteNodeID: res.NodeID,
			Status:       store.ItemStatusConfirmed,
			LastSyncedAt: item.UpdatedAt,
		})
		return ""

	case res != nil && res.Status == aphotos.TransferUnconfirmed:
		item.Status = store.ItemStatusUnconfirmed
		item.MD5 = md5Hex
		item.ErrorMessage = formatErrMsg(upErr)
		if item.ErrorMessage == "" {
			item.ErrorMessage = "backup upload unconfirmed"
		}
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)

		_ = st.UpsertTrackedFile(bgCtx, store.TrackedFile{
			BackupRuleID: rule.ID,
			RelPath:      c.relPath,
			AbsPath:      c.absPath,
			Size:         sizeBefore,
			MtimeNs:      mtimeBefore,
			MD5:          md5Hex,
			Status:       store.ItemStatusUnconfirmed,
			LastSyncedAt: item.UpdatedAt,
		})
		return ""

	case errors.Is(upErr, context.Canceled):
		item.Status = store.ItemStatusCancelled
		item.ErrorMessage = "cancelled"
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)
		return ""

	default:
		item.Status = store.ItemStatusFailed
		item.ErrorMessage = formatErrMsg(upErr)
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(bgCtx, item)
		return item.ErrorMessage
	}
}
