package engine

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/store"
)

var (
	ErrOffline               = errors.New("account credentials not configured")
	errFileModifiedMidFlight = errors.New("file modified while upload was in progress")
)

type Observer struct {
	OnTransferStage    func(tr store.Transfer)
	OnTransferProgress func(tr store.Transfer, item store.TransferItem)
	OnBackupStatus     func(rule store.BackupRule)
}

type Config struct {
	Store        *store.Store
	CredStore    credstore.Store
	Client       *aphotos.Client
	Observer     Observer
	PollInterval time.Duration
}

type activeTransferControl struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	paused   bool
	resumeCh chan struct{}
	doneCh   chan struct{}
}

func newTransferControl(cancel context.CancelFunc) *activeTransferControl {
	ch := make(chan struct{})
	close(ch)
	return &activeTransferControl{
		cancel:   cancel,
		paused:   false,
		resumeCh: ch,
		doneCh:   make(chan struct{}),
	}
}

func (c *activeTransferControl) setPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if paused && !c.paused {
		c.paused = true
		c.resumeCh = make(chan struct{})
	} else if !paused && c.paused {
		c.paused = false
		close(c.resumeCh)
	}
}

func (c *activeTransferControl) isPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

func (c *activeTransferControl) waitIfPaused(ctx context.Context) error {
	for {
		c.mu.Lock()
		paused := c.paused
		ch := c.resumeCh
		c.mu.Unlock()
		if !paused {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
		}
	}
}

type Engine struct {
	mu           sync.RWMutex
	store        *store.Store
	credStore    credstore.Store
	client       *aphotos.Client
	observer     Observer
	pollInterval time.Duration

	activeMu sync.Mutex
	active   map[string]*activeTransferControl

	liveMu sync.Mutex
	live   map[string]*transferLive

	thumbs thumbnailCache

	scanMu   sync.Mutex
	scanning map[string]string
	bgCancel context.CancelFunc
	bgWG     sync.WaitGroup
}

func New(cfg Config) *Engine {
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = 30 * time.Second
	}
	return &Engine{
		store:        cfg.Store,
		credStore:    cfg.CredStore,
		client:       cfg.Client,
		observer:     cfg.Observer,
		pollInterval: poll,
		active:       make(map[string]*activeTransferControl),
		scanning:     make(map[string]string),
	}
}

func (e *Engine) Store() *store.Store {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.store
}

func (e *Engine) CredStore() credstore.Store {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.credStore
}

func (e *Engine) Client() *aphotos.Client {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.client
}

func (e *Engine) SetClient(c *aphotos.Client) {
	e.mu.Lock()
	old := e.client
	e.client = c
	e.mu.Unlock()
	if old != c {
		e.activeMu.Lock()
		for _, ctrl := range e.active {
			ctrl.setPaused(false)
			ctrl.cancel()
		}
		e.activeMu.Unlock()
	}
	if old != nil && old != c {
		_ = old.Close()
	}
}

func (e *Engine) SetObserver(obs Observer) {
	e.mu.Lock()
	e.observer = obs
	e.mu.Unlock()
}

func (e *Engine) emitTransferStage(tr store.Transfer) {
	if tr.ID != "" {
		st := e.transferLive(tr.ID)
		st.mu.Lock()
		if st.base.ID == "" || !tr.UpdatedAt.Before(st.base.UpdatedAt) {
			st.base = tr
		}
		if store.IsTerminalStage(tr.Stage) {
			st.inflight = nil
		}
		st.mu.Unlock()
	}
	e.mu.RLock()
	fn := e.observer.OnTransferStage
	e.mu.RUnlock()
	if fn != nil {
		fn(tr)
	}
}

func (e *Engine) emitTransferProgress(tr store.Transfer, item store.TransferItem) {
	out := tr
	if tr.ID != "" {
		st := e.transferLive(tr.ID)
		st.mu.Lock()
		delete(st.inflight, item.ID)
		st.base = tr
		out = st.snapshotLocked()
		st.mu.Unlock()
	}
	e.emitTransferProgressRaw(out, item)
}

// emitLiveByteProgress folds one in-flight item into the last aggregate.
// The transfer value captured when a batch starts stays at zero done-counts,
// and publishing it replaces newer totals in the UI.
func (e *Engine) emitLiveByteProgress(fallback store.Transfer, item store.TransferItem) {
	if fallback.ID == "" {
		e.emitTransferProgressRaw(fallback, item)
		return
	}
	st := e.transferLive(fallback.ID)
	st.mu.Lock()
	if store.IsTerminalStage(st.base.Stage) {
		st.mu.Unlock()
		return
	}
	if st.base.ID == "" {
		st.base = fallback
	}
	if st.inflight == nil {
		st.inflight = map[string]int64{}
	}
	st.inflight[item.ID] = item.BytesDone
	out := st.snapshotLocked()
	st.mu.Unlock()
	e.emitTransferProgressRaw(out, item)
}

func (e *Engine) emitTransferProgressRaw(tr store.Transfer, item store.TransferItem) {
	e.mu.RLock()
	fn := e.observer.OnTransferProgress
	e.mu.RUnlock()
	if fn != nil {
		fn(tr, item)
	}
}

type transferLive struct {
	mu       sync.Mutex
	base     store.Transfer
	inflight map[string]int64
}

func (e *Engine) transferLive(id string) *transferLive {
	e.liveMu.Lock()
	defer e.liveMu.Unlock()
	if e.live == nil {
		e.live = map[string]*transferLive{}
	}
	st := e.live[id]
	if st == nil {
		st = &transferLive{}
		e.live[id] = st
	}
	return st
}

func (st *transferLive) snapshotLocked() store.Transfer {
	tr := st.base
	for _, n := range st.inflight {
		tr.BytesDone += n
	}
	if tr.BytesTotal > 0 && tr.BytesDone > tr.BytesTotal {
		tr.BytesDone = tr.BytesTotal
	}
	return tr
}

func (e *Engine) emitBackupStatus(rule store.BackupRule) {
	e.mu.RLock()
	fn := e.observer.OnBackupStatus
	e.mu.RUnlock()
	if fn != nil {
		fn(rule)
	}
}

func (e *Engine) Close() error {
	e.mu.Lock()
	bgCancel := e.bgCancel
	e.bgCancel = nil
	c := e.client
	e.client = nil
	e.mu.Unlock()

	if bgCancel != nil {
		bgCancel()
	}

	e.activeMu.Lock()
	for _, ctrl := range e.active {
		ctrl.setPaused(false)
		ctrl.cancel()
	}
	e.activeMu.Unlock()

	e.bgWG.Wait()
	if c != nil {
		_ = c.Close()
	}
	return nil
}

func (e *Engine) StartBackground(ctx context.Context) {
	e.mu.Lock()
	if e.bgCancel != nil {
		e.mu.Unlock()
		return
	}
	bgCtx, cancel := context.WithCancel(ctx)
	e.bgCancel = cancel
	poll := e.pollInterval
	e.mu.Unlock()

	e.bgWG.Go(func() {
		ticker := time.NewTicker(poll)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				e.runScheduledBackupScans(bgCtx)
			}
		}
	})
}

func (e *Engine) ReconcileOnStartup(ctx context.Context) error {
	st := e.Store()
	if st == nil {
		return nil
	}
	transfers, err := st.ListTransfersNeedingReconciliation(ctx)
	if err != nil {
		return err
	}
	client := e.Client()
	var firstRemoteErr error

	for ti := range transfers {
		tr := &transfers[ti]
		if err := ctx.Err(); err != nil {
			return err
		}
		destination := DestinationCapability{}
		destinationReadable := false
		if client != nil && (tr.Kind == store.TransferKindUpload || tr.Kind == store.TransferKindBackup) {
			destination = e.probeDestination(ctx, client, tr.DestNodeID)
			probeErr := destination.requireReadable()
			if probeErr == nil {
				probeErr = e.bindDestination(ctx, "transfer:"+tr.ID, destination)
			}
			if probeErr != nil {
				if firstRemoteErr == nil {
					firstRemoteErr = probeErr
				}
			} else {
				destinationReadable = true
			}
		}
		items, iErr := st.ListTransferItems(ctx, tr.ID)
		if iErr != nil {
			return iErr
		}
		for i := range items {
			it := &items[i]
			if it.Status != store.ItemStatusRunning && it.Status != store.ItemStatusUnconfirmed && it.Status != store.ItemStatusQueued {
				continue
			}
			if tr.Kind == store.TransferKindUpload || tr.Kind == store.TransferKindBackup {
				if destinationReadable && (it.Status == store.ItemStatusRunning || it.Status == store.ItemStatusUnconfirmed) {
					node, fErr := findExistingInDestination(ctx, client, destination, *it)
					if fErr != nil && firstRemoteErr == nil {
						firstRemoteErr = fErr
					}
					if fErr == nil && node != nil && strings.TrimSpace(node.ID) != "" {
						it.Status = reconciledStatus(destination)
						it.RemoteNodeID = node.ID
						it.BytesDone = it.Size
						it.ErrorMessage = ""
						it.UpdatedAt = time.Now().UTC()
						_ = st.UpdateTransferItem(ctx, *it)
						continue
					}
				}
				switch it.Status {
				case store.ItemStatusRunning:
					it.Status = store.ItemStatusUnconfirmed
					if it.ErrorMessage == "" {
						it.ErrorMessage = "interrupted before upload confirmation; awaiting reconciliation or retry"
					}
					it.UpdatedAt = time.Now().UTC()
					_ = st.UpdateTransferItem(ctx, *it)
				case store.ItemStatusQueued:
					it.Status = store.ItemStatusUnconfirmed
					it.ErrorMessage = "transfer interrupted before item execution"
					it.UpdatedAt = time.Now().UTC()
					_ = st.UpdateTransferItem(ctx, *it)
				}
			} else if tr.Kind == store.TransferKindDownload {
				if it.Status == store.ItemStatusRunning || it.Status == store.ItemStatusQueued {
					it.Status = store.ItemStatusFailed
					it.ErrorMessage = "download interrupted before completion"
					it.UpdatedAt = time.Now().UTC()
					_ = st.UpdateTransferItem(ctx, *it)
				}
			}
		}

		updatedTr, rErr := recomputeAndPersistTransfer(ctx, st, tr.ID, "")
		if rErr != nil {
			return rErr
		}
		e.emitTransferStage(updatedTr)
	}
	return firstRemoteErr
}

func findRemoteNodeForItem(ctx context.Context, client *aphotos.Client, item store.TransferItem, parentNodeID string) (*aphotos.Node, error) {
	if client == nil {
		return nil, ErrOffline
	}
	md5Hex := strings.ToLower(strings.TrimSpace(item.MD5))
	if md5Hex == "" && strings.TrimSpace(item.LocalPath) != "" {
		if computed, err := computeFileMD5(item.LocalPath); err == nil {
			md5Hex = computed
		}
	}
	if md5Hex == "" {
		return nil, nil
	}

	parentID := strings.TrimSpace(parentNodeID)
	name := strings.TrimSpace(item.Name)

	matchesCandidate := func(cand *aphotos.Node) bool {
		if cand == nil || strings.TrimSpace(cand.ID) == "" || cand.Status == aphotos.NodeStatusTrash {
			return false
		}
		if cand.ContentProperties == nil {
			return false
		}
		candMD5 := strings.ToLower(strings.TrimSpace(cand.ContentProperties.MD5))
		if candMD5 == "" || !strings.EqualFold(candMD5, md5Hex) {
			return false
		}
		if item.Size > 0 && cand.ContentProperties.Size > 0 && cand.ContentProperties.Size != item.Size {
			return false
		}
		if parentID != "" && len(cand.Parents) > 0 {
			hasParent := slices.Contains(cand.Parents, parentID)
			if !hasParent {
				return false
			}
		}
		return true
	}

	fb := aphotos.NewFilter().Kind(aphotos.NodeKindFile).Status(aphotos.NodeStatusAvailable).MD5(md5Hex)
	if parentID != "" {
		fb = fb.ParentID(parentID)
	}
	page, err := client.QueryNodes(ctx, aphotos.NodeQueryOptions{
		Filter: fb,
		Limit:  10,
	})
	if err != nil {
		return nil, err
	}
	for i := range page.Items {
		cand := &page.Items[i]
		if matchesCandidate(cand) && (name == "" || cand.Name == name) {
			c := *cand
			return &c, nil
		}
	}
	for i := range page.Items {
		cand := &page.Items[i]
		if matchesCandidate(cand) {
			c := *cand
			return &c, nil
		}
	}

	if name != "" {
		nameFB := aphotos.NewFilter().Kind(aphotos.NodeKindFile).Status(aphotos.NodeStatusAvailable).NameExact(name)
		if parentID != "" {
			nameFB = nameFB.ParentID(parentID)
		}
		namePage, nErr := client.QueryNodes(ctx, aphotos.NodeQueryOptions{
			Filter: nameFB,
			Limit:  10,
		})
		if nErr != nil {
			return nil, nErr
		}
		for i := range namePage.Items {
			cand := &namePage.Items[i]
			if matchesCandidate(cand) {
				c := *cand
				return &c, nil
			}
		}
	}

	return nil, nil
}

func recomputeAndPersistTransfer(ctx context.Context, st *store.Store, transferID string, forceStage store.TransferStage) (store.Transfer, error) {
	tr, err := st.GetTransfer(ctx, transferID)
	if err != nil {
		return store.Transfer{}, err
	}
	items, err := st.ListTransferItems(ctx, transferID)
	if err != nil {
		return store.Transfer{}, err
	}

	var bytesDone, bytesTotal int64
	var doneCount, skippedCount, failedCount, unconfirmedCount, cancelledCount, runningOrQueued, albumPending int
	var firstErr string

	for i := range items {
		it := &items[i]
		bytesTotal += it.Size
		switch it.Status {
		case store.ItemStatusAlbumPending:
			albumPending++
			bytesDone += it.Size
			if firstErr == "" {
				firstErr = it.ErrorMessage
			}
		case store.ItemStatusConfirmed:
			doneCount++
			bytesDone += it.Size
		case store.ItemStatusSkipped:
			skippedCount++
			bytesDone += it.Size
		case store.ItemStatusFailed:
			failedCount++
			if firstErr == "" && it.ErrorMessage != "" {
				firstErr = it.ErrorMessage
			}
		case store.ItemStatusUnconfirmed:
			unconfirmedCount++
			if firstErr == "" && it.ErrorMessage != "" {
				firstErr = it.ErrorMessage
			}
		case store.ItemStatusCancelled:
			cancelledCount++
		case store.ItemStatusRunning, store.ItemStatusQueued:
			runningOrQueued++
			bytesDone += it.BytesDone
		}
	}

	tr.BytesDone = bytesDone
	if bytesTotal > 0 {
		tr.BytesTotal = bytesTotal
	}
	tr.ItemsDone = doneCount
	tr.ItemsSkipped = skippedCount
	tr.ItemsFailed = failedCount
	tr.ItemsUnconfirmed = unconfirmedCount + albumPending
	tr.ItemsTotal = len(items)

	switch {
	case forceStage != "":
		tr.Stage = forceStage
	case runningOrQueued > 0:
		if tr.Stage != store.StagePaused {
			tr.Stage = store.StageRunning
		}
	case albumPending > 0:
		tr.Stage = store.StagePartial
	case unconfirmedCount > 0:
		tr.Stage = store.StageUnconfirmed
	case cancelledCount > 0 && doneCount == 0 && failedCount == 0:
		tr.Stage = store.StageCancelled
	case failedCount > 0 && (doneCount > 0 || skippedCount > 0):
		tr.Stage = store.StagePartial
	case failedCount > 0:
		tr.Stage = store.StageFailed
	default:
		tr.Stage = store.StageCompleted
	}

	if firstErr != "" && tr.ErrorMessage == "" {
		tr.ErrorMessage = aphotos.Redact(firstErr)
	}
	now := time.Now().UTC()
	tr.UpdatedAt = now
	if store.IsTerminalStage(tr.Stage) && tr.FinishedAt == nil {
		tr.FinishedAt = &now
	}

	if err := st.UpdateTransfer(ctx, tr); err != nil {
		return store.Transfer{}, err
	}
	return tr, nil
}

func formatErrMsg(err error) string {
	if err == nil {
		return ""
	}
	return aphotos.Redact(fmt.Sprintf("%v", err))
}
