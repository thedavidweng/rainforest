package gui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/engine"
	"github.com/thedavidweng/rainforest/internal/store"
)

type Options struct {
	DBPath          string
	VaultPath       string
	HTTPClient      *http.Client
	Transport       http.RoundTripper
	PollInterval    time.Duration
	Emitter         Emitter
	Picker          FilePicker
	LaunchAtLoginFn func(enabled bool) error
	BrowserLogin    func(context.Context, aphotos.RegionConfig, func(map[string]string) error) error
}

type FilePicker interface {
	PickCookieFile(ctx context.Context) (string, error)
	PickFiles(ctx context.Context) ([]string, error)
	PickDirectory(ctx context.Context, title string) (string, error)
}

type Services struct {
	Auth      *Auth
	Transfers *Transfers
	Backups   *Backups
	Downloads *Downloads
	Settings  *Settings

	state *appState
}

type appState struct {
	mu               sync.RWMutex
	opts             Options
	store            *store.Store
	credStore        credstore.Store
	engine           *engine.Engine
	emitter          Emitter
	picker           FilePicker
	tray             *TrayTracker
	launchAtLoginFn  func(enabled bool) error
	notifiedStages   map[string]store.TransferStage
	lastReconcileErr string
}

func NewEmptyServices() *Services {
	st := &appState{
		tray:           NewTrayTracker(),
		notifiedStages: make(map[string]store.TransferStage),
	}
	return &Services{
		Auth:      &Auth{state: st},
		Transfers: &Transfers{state: st},
		Backups:   &Backups{state: st},
		Downloads: &Downloads{state: st},
		Settings:  &Settings{state: st},
		state:     st,
	}
}

func Open(opts Options) (*Services, func(), error) {
	svc := NewEmptyServices()
	closeFn, err := svc.Init(opts)
	if err != nil {
		return nil, func() {}, err
	}
	return svc, closeFn, nil
}

func (s *Services) Init(opts Options) (func(), error) {
	if s.state == nil {
		s.state = &appState{
			tray:           NewTrayTracker(),
			notifiedStages: make(map[string]store.TransferStage),
		}
		if s.Auth != nil {
			s.Auth.state = s.state
		}
		if s.Transfers != nil {
			s.Transfers.state = s.state
		}
		if s.Backups != nil {
			s.Backups.state = s.state
		}
		if s.Downloads != nil {
			s.Downloads.state = s.state
		}
		if s.Settings != nil {
			s.Settings.state = s.state
		}
	}

	ctx := context.Background()
	st, err := store.Open(ctx, opts.DBPath)
	if err != nil {
		return func() {}, toError(err)
	}

	cs := credstore.NewFileStore(opts.VaultPath)
	cfg, err := st.GetSettings(ctx)
	if err != nil {
		_ = st.Close()
		return func() {}, toError(err)
	}

	client, creds, _ := buildClientFromVault(cs, cfg, opts)
	if creds.Region != "" && cfg.Region == "" {
		cfg.Region = string(creds.Region)
		_ = st.UpdateSettings(ctx, cfg)
	}

	s.state.mu.Lock()
	s.state.opts = opts
	s.state.store = st
	s.state.credStore = cs
	if opts.Emitter != nil {
		s.state.emitter = opts.Emitter
	}
	if opts.Picker != nil {
		s.state.picker = opts.Picker
	}
	if opts.LaunchAtLoginFn != nil {
		s.state.launchAtLoginFn = opts.LaunchAtLoginFn
	}
	if s.state.tray == nil {
		s.state.tray = NewTrayTracker()
	}
	if s.state.notifiedStages == nil {
		s.state.notifiedStages = make(map[string]store.TransferStage)
	}

	eng := engine.New(engine.Config{
		Store:        st,
		CredStore:    cs,
		Client:       client,
		Observer:     s.state.makeObserver(),
		PollInterval: opts.PollInterval,
	})
	s.state.engine = eng
	s.state.mu.Unlock()

	s.state.recordReconcileResult(eng.ReconcileOnStartup(ctx))
	eng.StartBackground(ctx)

	return s.Close, nil
}

func (s *Services) Close() {
	if s == nil || s.state == nil {
		return
	}
	s.Auth.CancelLogin()
	s.state.close()
}

func (s *Services) SetEmitter(em Emitter) {
	if s == nil || s.state == nil {
		return
	}
	s.state.mu.Lock()
	s.state.emitter = em
	s.state.mu.Unlock()
}

func (s *Services) SetFilePicker(p FilePicker) {
	if s == nil || s.state == nil {
		return
	}
	s.state.mu.Lock()
	s.state.picker = p
	s.state.mu.Unlock()
}

func (s *Services) SetLaunchAtLoginHook(fn func(enabled bool) error) {
	if s == nil || s.state == nil {
		return
	}
	s.state.mu.Lock()
	s.state.launchAtLoginFn = fn
	s.state.mu.Unlock()
}

func (s *Services) TrayTracker() *TrayTracker {
	if s == nil || s.state == nil {
		return NewTrayTracker()
	}
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	return s.state.tray
}

func (s *Services) EmitFilesDropped(paths []string, x, y int) {
	if s == nil || s.state == nil {
		return
	}
	s.state.emit(EventFilesDropped, FilesDroppedEvent{
		Paths: paths,
		X:     x,
		Y:     y,
	})
}

func (s *Services) ActiveBackupCount(ctx context.Context) int {
	if s == nil || s.state == nil {
		return 0
	}
	st, _, err := s.state.deps()
	if err != nil {
		return 0
	}
	cfg, err := st.GetSettings(ctx)
	if err == nil && cfg.BackupsStopped {
		return 0
	}
	rules, err := st.ListBackupRules(ctx)
	if err != nil {
		return 0
	}
	count := 0
	for i := range rules {
		if !rules[i].Paused {
			count++
		}
	}
	return count
}

func buildClientFromVault(cs credstore.Store, cfg store.Settings, opts Options) (*aphotos.Client, credstore.Credentials, error) {
	if cs == nil {
		return nil, credstore.Credentials{}, credstore.ErrNotConfigured
	}
	creds, err := cs.Load()
	if err != nil {
		return nil, credstore.Credentials{}, err
	}
	client, err := aphotos.NewClient(aphotos.Options{
		Region:                 creds.Region,
		Cookies:                creds.Cookies,
		DriveURL:               creds.DriveURL,
		UploadURL:              creds.UploadURL,
		ThumbnailURL:           creds.ThumbnailURL,
		HTTPClient:             opts.HTTPClient,
		Transport:              opts.Transport,
		MaxMetadataConcurrency: cfg.MaxMetadataConcurrency,
		MaxUploadConcurrency:   cfg.MaxUploadConcurrency,
	})
	if err != nil {
		return nil, creds, err
	}
	return client, creds, nil
}

func (st *appState) deps() (*store.Store, *engine.Engine, error) {
	if st == nil {
		return nil, nil, newError(ErrConfigError, "GUI services are not initialized", nil)
	}
	st.mu.RLock()
	db := st.store
	eng := st.engine
	st.mu.RUnlock()
	if db == nil || eng == nil {
		return nil, nil, newError(ErrConfigError, "GUI services are not initialized", nil)
	}
	return db, eng, nil
}

func (st *appState) credVault() (credstore.Store, error) {
	if st == nil {
		return nil, newError(ErrConfigError, "GUI services are not initialized", nil)
	}
	st.mu.RLock()
	cs := st.credStore
	st.mu.RUnlock()
	if cs == nil {
		return nil, newError(ErrConfigError, "GUI credential store is not initialized", nil)
	}
	return cs, nil
}

func (st *appState) filePicker() (FilePicker, error) {
	if st == nil {
		return nil, newError(ErrConfigError, "file picker is not configured", nil)
	}
	st.mu.RLock()
	p := st.picker
	st.mu.RUnlock()
	if p == nil {
		return nil, newError(ErrConfigError, "native file picker is not configured", nil)
	}
	return p, nil
}

func (st *appState) emit(name string, data any) {
	if st == nil {
		return
	}
	st.mu.RLock()
	em := st.emitter
	st.mu.RUnlock()
	if em != nil {
		em(name, data)
	}
}

func (st *appState) reopen(ctx context.Context) error {
	st.mu.RLock()
	db := st.store
	cs := st.credStore
	eng := st.engine
	opts := st.opts
	st.mu.RUnlock()

	if db == nil || cs == nil || eng == nil {
		return newError(ErrConfigError, "GUI services are not initialized", nil)
	}

	cfg, err := db.GetSettings(ctx)
	if err != nil {
		return toError(err)
	}

	client, creds, cErr := buildClientFromVault(cs, cfg, opts)
	if cErr != nil && !errors.Is(cErr, credstore.ErrNotConfigured) {
		eng.SetClient(nil)
		return toError(cErr)
	}

	if creds.Region != "" && string(creds.Region) != cfg.Region {
		cfg.Region = string(creds.Region)
		_ = db.UpdateSettings(ctx, cfg)
	}

	eng.SetClient(client)
	if client != nil {
		st.recordReconcileResult(eng.ReconcileOnStartup(ctx))
	}
	return nil
}

func (st *appState) recordReconcileResult(err error) {
	if st == nil {
		return
	}
	msg := ""
	if err != nil {
		msg = aphotos.Redact(err.Error())
	}
	st.mu.Lock()
	st.lastReconcileErr = msg
	st.mu.Unlock()
	if msg != "" {
		st.emit(EventNotification, NotificationEvent{
			Title: "Startup Recovery Warning",
			Body:  msg,
			Level: "warning",
		})
	}
}

func (st *appState) reconcileWarning() string {
	if st == nil {
		return ""
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.lastReconcileErr
}

func (st *appState) close() {
	st.mu.Lock()
	eng := st.engine
	db := st.store
	st.engine = nil
	st.store = nil
	st.mu.Unlock()

	if eng != nil {
		_ = eng.Close()
	}
	if db != nil {
		_ = db.Close()
	}
}

func (st *appState) makeObserver() engine.Observer {
	return engine.Observer{
		OnTransferStage: func(tr store.Transfer) {
			dto := toTransferDTO(tr)
			st.mu.RLock()
			tray := st.tray
			db := st.store
			st.mu.RUnlock()
			if tray != nil {
				tray.Apply(dto)
			}
			st.emit(EventTransferStage, dto)
			st.maybeEmitNotification(db, tr)
		},
		OnTransferProgress: func(tr store.Transfer, item store.TransferItem) {
			trDTO := toTransferDTO(tr)
			itemDTO := toTransferItemDTO(item)
			st.mu.RLock()
			tray := st.tray
			st.mu.RUnlock()
			if tray != nil {
				tray.Apply(trDTO)
			}
			st.emit(EventTransferProgress, TransferProgressEvent{
				Transfer: trDTO,
				Item:     itemDTO,
			})
		},
		OnBackupStatus: func(rule store.BackupRule) {
			st.emit(EventBackupStatus, toBackupRuleDTO(rule))
		},
	}
}

func (st *appState) maybeEmitNotification(db *store.Store, tr store.Transfer) {
	if !store.IsTerminalStage(tr.Stage) || tr.Stage == store.StageCancelled {
		return
	}
	if db != nil {
		if cfg, err := db.GetSettings(context.Background()); err == nil && !cfg.NotificationsEnabled {
			return
		}
	}

	st.mu.Lock()
	if st.notifiedStages == nil {
		st.notifiedStages = make(map[string]store.TransferStage)
	}
	if prev, ok := st.notifiedStages[tr.ID]; ok && prev == tr.Stage {
		st.mu.Unlock()
		return
	}
	st.notifiedStages[tr.ID] = tr.Stage
	st.mu.Unlock()

	var ev NotificationEvent
	ev.TransferID = tr.ID
	summary := strings.TrimSpace(tr.SourceSummary)
	if summary == "" {
		summary = string(tr.Kind)
	}

	switch tr.Stage {
	case store.StageCompleted:
		ev.Title = "Transfer Completed"
		ev.Body = fmt.Sprintf("%s finished (%d items confirmed).", summary, tr.ItemsDone)
		ev.Level = "info"
	case store.StagePartial:
		ev.Title = "Transfer Completed with Warnings"
		ev.Body = fmt.Sprintf("%s finished with %d confirmed and %d failed items.", summary, tr.ItemsDone, tr.ItemsFailed)
		ev.Level = "warning"
	case store.StageUnconfirmed:
		ev.Title = "Transfer Needs Verification"
		ev.Body = fmt.Sprintf("%s has %d unconfirmed item(s).", summary, tr.ItemsUnconfirmed)
		ev.Level = "warning"
	case store.StageFailed:
		ev.Title = "Transfer Failed"
		msg := strings.TrimSpace(tr.ErrorMessage)
		if msg == "" {
			msg = fmt.Sprintf("%d item(s) failed.", tr.ItemsFailed)
		}
		ev.Body = fmt.Sprintf("%s: %s", summary, aphotos.Redact(msg))
		ev.Level = "error"
	default:
		return
	}
	st.emit(EventNotification, ev)
}

func formatTimeISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatOptionalTimeISO(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
