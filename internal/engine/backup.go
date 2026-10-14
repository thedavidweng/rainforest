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
	// Strictly one-way: removing a backup rule deletes only local tracking rows via SQLite cascade, NEVER remote cloud nodes.
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
	return nil
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
	relPath   string
	absPath   string
	name      string
	mediaType store.MediaType
	size      int64
	mtimeNs   int64
}

func (e *Engine) ScanNow(ctx context.Context, ruleID string) (*store.Transfer, error) {
	st := e.Store()
	if st == nil {
		return nil, store.ErrNotFound
	}

	e.scanMu.Lock()
	if e.scanning[ruleID] {
		e.scanMu.Unlock()
		return nil, nil
	}
	e.scanning[ruleID] = true
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

	var candidates []backupCandidate
	walkErr := filepath.WalkDir(rule.LocalPath, func(p string, d fs.DirEntry, wErr error) error {
		if wErr != nil {
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
			return nil
		}
		rel, rErr := filepath.Rel(rule.LocalPath, p)
		if rErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		size := info.Size()
		mtimeNs := info.ModTime().UnixNano()
		if prev, ok := trackedByRel[rel]; ok {
			if prev.Status == store.ItemStatusConfirmed && prev.Size == size && prev.MtimeNs == mtimeNs {
				return nil
			}
		}

		candidates = append(candidates, backupCandidate{
			relPath:   rel,
			absPath:   p,
			name:      d.Name(),
			mediaType: mt,
			size:      size,
			mtimeNs:   mtimeNs,
		})
		return nil
	})
	if walkErr != nil {
		_ = st.SetBackupRuleStatus(ctx, rule.ID, store.BackupStatusError, formatErrMsg(walkErr), &now)
		return nil, walkErr
	}

	// Note: Any tracked files missing locally are intentionally ignored.
	// Backup is strictly one-way incremental upload; local deletes/moves/renames NEVER delete remote files.

	if len(candidates) == 0 {
		finalStatus := store.BackupStatusActive
		if rule.Paused {
			finalStatus = store.BackupStatusPaused
		}
		scanDone := time.Now().UTC()
		_ = st.SetBackupRuleStatus(ctx, rule.ID, finalStatus, "", &scanDone)
		if updated, gErr := st.GetBackupRule(ctx, rule.ID); gErr == nil {
			e.emitBackupStatus(updated)
		}
		return nil, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].relPath < candidates[j].relPath
	})

	client := e.Client()
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

	var lastRuleErr string
	for i, c := range candidates {
		if err := ctx.Err(); err != nil {
			break
		}
		item := items[i]

		// Pre-upload stability check: verify file still exists and matches candidate size/mtime
		statBefore, sErr := os.Stat(c.absPath)
		if sErr != nil {
			item.Status = store.ItemStatusSkipped
			item.ErrorMessage = formatErrMsg(sErr)
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(ctx, item)
			continue
		}
		sizeBefore := statBefore.Size()
		mtimeBefore := statBefore.ModTime().UnixNano()
		if sizeBefore != c.size || mtimeBefore != c.mtimeNs {
			item.Status = store.ItemStatusSkipped
			item.ErrorMessage = errFileModifiedMidFlight.Error()
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(ctx, item)
			continue
		}

		item.Status = store.ItemStatusRunning
		item.UpdatedAt = time.Now().UTC()
		_ = st.UpdateTransferItem(ctx, item)

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

		// Post-upload stability check: verify file was not modified mid-flight while streaming!
		statAfter, afterErr := os.Stat(c.absPath)
		if afterErr != nil || statAfter.Size() != sizeBefore || statAfter.ModTime().UnixNano() != mtimeBefore {
			item.Status = store.ItemStatusSkipped
			item.ErrorMessage = errFileModifiedMidFlight.Error()
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(ctx, item)
			continue
		}

		md5Hex := ""
		if res != nil {
			md5Hex = res.MD5
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
			_ = st.UpdateTransferItem(ctx, item)

			_ = st.UpsertTrackedFile(ctx, store.TrackedFile{
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

		case res != nil && res.Status == aphotos.TransferUnconfirmed:
			item.Status = store.ItemStatusUnconfirmed
			item.MD5 = md5Hex
			item.ErrorMessage = formatErrMsg(upErr)
			if item.ErrorMessage == "" {
				item.ErrorMessage = "backup upload unconfirmed"
			}
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(ctx, item)

			_ = st.UpsertTrackedFile(ctx, store.TrackedFile{
				BackupRuleID: rule.ID,
				RelPath:      c.relPath,
				AbsPath:      c.absPath,
				Size:         sizeBefore,
				MtimeNs:      mtimeBefore,
				MD5:          md5Hex,
				Status:       store.ItemStatusUnconfirmed,
				LastSyncedAt: item.UpdatedAt,
			})

		default:
			item.Status = store.ItemStatusFailed
			item.ErrorMessage = formatErrMsg(upErr)
			item.UpdatedAt = time.Now().UTC()
			_ = st.UpdateTransferItem(ctx, item)
			if lastRuleErr == "" {
				lastRuleErr = item.ErrorMessage
			}
		}
	}

	finalTr, _ := recomputeAndPersistTransfer(ctx, st, tr.ID, "")
	e.emitTransferStage(finalTr)

	scanDone := time.Now().UTC()
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
	return &finalTr, nil
}
