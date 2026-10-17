package view

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
)

type Options struct {
	Preview   bool
	SetLocale func(string)
	Update    func(func())
	Version   string
	Icon      []byte
}

type loadState int

const (
	loadIdle loadState = iota
	loadRunning
	loadDone
	loadFailed
)

type itemsState struct {
	state loadState
	items []gui.TransferItemDTO
	err   string
}

type probeState struct {
	state loadState
	capab gui.DestinationCapabilityDTO
	err   string
}

type App struct {
	setLocale func(string)
	aboutOpen bool
	svc       Services
	preview   bool
	update    func(func())
	spawn     func(func())
	version   string
	router    *ui.Router
	toasts    []ui.Toast

	authSeq    int
	authLoaded bool
	auth       gui.AuthStatus

	usage      *gui.UsageSummary
	usageState loadState

	settings *gui.SettingsDTO

	active          []gui.TransferDTO
	history         []gui.TransferDTO
	transferWarning string
	transfersLoaded bool

	rules       []gui.BackupRuleDTO
	rulesLoaded bool

	recovery      *gui.RecoverySourcesDTO
	recoveryState loadState
	recoveryErr   string

	recent     []gui.RecentUploadDTO
	lastUpload string

	items   map[string]*itemsState
	probes  map[string]*probeState
	folders map[string]*folderState
	files   map[string]*filesState
	thumbs  map[string]*ui.Bitmap
	icon    *ui.Bitmap

	picker pickerState

	backup  backupState
	restore restoreState

	welcomeUI  welcomeState
	settingsUI settingsState

	pages pageState
}

func New(svc *gui.Services, opts Options) *App {
	return NewWithServices(Services{Auth: svc.Auth, Transfers: svc.Transfers, Backups: svc.Backups, Downloads: svc.Downloads, Settings: svc.Settings}, opts)
}

func NewWithServices(svc Services, opts Options) *App {
	update := opts.Update
	if update == nil {
		update = func(fn func()) { fn() }
	}
	setLocale := opts.SetLocale
	if setLocale == nil {
		setLocale = func(string) {}
	}
	icon, _ := ui.DecodeBitmap(opts.Icon)
	return &App{
		setLocale: setLocale,
		icon:      icon,
		svc:       svc,
		preview:   opts.Preview,
		update:    update,
		spawn:     func(fn func()) { go fn() },
		version:   opts.Version,
		router:    ui.NewRouter("/upload"),
		items:     map[string]*itemsState{},
		probes:    map[string]*probeState{},
		folders:   map[string]*folderState{},
		files:     map[string]*filesState{},
		thumbs:    map[string]*ui.Bitmap{},
	}
}

func (a *App) Start() {
	a.refreshAuth()
	a.refreshSettings()
	a.refreshTransfers()
	a.refreshBackups()
	a.refreshRecent()
}

func (a *App) do(work func(ctx context.Context) func()) {
	seq := a.authSeq
	a.spawn(func() {
		if apply := work(context.Background()); apply != nil {
			a.update(func() {
				if a.authSeq == seq {
					apply()
				}
			})
		}
	})
}

func (a *App) signedIn() bool {
	return a.authLoaded && a.auth.Authenticated
}

func (a *App) backupsStopped() bool {
	return a.settings != nil && a.settings.BackupsStopped
}

func (a *App) HandleEvent(name string, data any) {
	switch name {
	case gui.EventTransferStage:
		if t, ok := data.(gui.TransferDTO); ok {
			a.update(func() {
				a.upsertTransfer(t)
				if isTerminalStage(t.Stage) {
					a.refreshRecent()
				}
			})
		}
	case gui.EventTransferProgress:
		if p, ok := data.(gui.TransferProgressEvent); ok {
			a.update(func() {
				a.upsertTransfer(p.Transfer)
				a.upsertItem(p.Item)
			})
		}
	case gui.EventBackupStatus:
		if r, ok := data.(gui.BackupRuleDTO); ok {
			a.update(func() { a.upsertRule(r) })
		}
	case gui.EventAuthChanged:
		if st, ok := data.(gui.AuthStatus); ok {
			a.update(func() { a.setAuth(st) })
		}
	}
}

func (a *App) setAuth(st gui.AuthStatus) {
	was := a.signedIn()
	changed := a.auth.Authenticated != st.Authenticated || a.auth.OwnerID != st.OwnerID || a.auth.RootNodeID != st.RootNodeID || a.auth.Region != st.Region
	if changed {
		a.authSeq++
		a.welcomeUI = welcomeState{}
		a.settingsUI = settingsState{}
		a.backup = backupState{}
		a.files = map[string]*filesState{}
		a.items = map[string]*itemsState{}
		a.active, a.history, a.transferWarning = nil, nil, ""
		a.transfersLoaded = false
		a.rules, a.rulesLoaded = nil, false
		a.pages = pageState{}
		a.picker = pickerState{}
		a.restore = restoreState{}
		a.recent, a.lastUpload = nil, ""
	}
	a.auth = st
	a.authLoaded = true
	switch now := a.signedIn(); {
	case now && (!was || changed):
		a.usageState = loadIdle
		a.usage = nil
		a.probes = map[string]*probeState{}
		a.folders = map[string]*folderState{}
		a.recovery, a.recoveryState = nil, loadIdle
		a.refreshSettings()
		a.refreshTransfers()
		a.refreshBackups()
		a.refreshRecent()
	case !now && was:
		a.usage, a.usageState = nil, loadIdle
		a.recovery, a.recoveryState = nil, loadIdle
		a.probes = map[string]*probeState{}
		a.folders = map[string]*folderState{}
		a.router.Replace("/upload")
	}
}

func (a *App) refreshAuth() {
	a.do(func(ctx context.Context) func() {
		st, err := a.svc.Auth.Status(ctx)
		return func() {
			if err != nil {
				a.setAuth(gui.AuthStatus{ErrorCode: errCode(err), ErrorMessage: errMessage(err)})
				return
			}
			a.setAuth(*st)
		}
	})
}

func (a *App) ensureUsage() {
	if a.usageState != loadIdle || !a.signedIn() {
		return
	}
	a.usageState = loadRunning
	a.do(func(ctx context.Context) func() {
		u, err := a.svc.Auth.Usage(ctx)
		return func() {
			if err != nil {
				a.usageState = loadFailed
				return
			}
			a.usage, a.usageState = u, loadDone
		}
	})
}

func (a *App) refreshSettings() {
	a.do(func(ctx context.Context) func() {
		s, err := a.svc.Settings.Get(ctx)
		return func() {
			if err != nil {
				a.failed(err)
				return
			}
			a.settings = s
		}
	})
}

func (a *App) updateSettings(in gui.SettingsInput) {
	a.do(func(ctx context.Context) func() {
		s, err := a.svc.Settings.Update(ctx, in)
		return func() {
			if err != nil {
				a.toastError(err)
				a.refreshSettings()
				return
			}
			a.settings = s
		}
	})
}

func (a *App) refreshTransfers() {
	a.do(func(ctx context.Context) func() {
		list, err := a.svc.Transfers.List(ctx)
		return func() {
			a.transfersLoaded = true
			if err != nil {
				a.failed(err)
				return
			}
			a.active, a.history, a.transferWarning = list.Active, list.History, list.Warning
		}
	})
}

func (a *App) refreshBackups() {
	a.do(func(ctx context.Context) func() {
		rules, err := a.svc.Backups.List(ctx)
		return func() {
			a.rulesLoaded = true
			if err != nil {
				a.failed(err)
				return
			}
			a.rules = rules
		}
	})
}

func (a *App) ensureRecovery() {
	if a.recoveryState != loadIdle || !a.signedIn() {
		return
	}
	a.refreshRecovery()
}

func (a *App) refreshRecovery() {
	a.recoveryState = loadRunning
	a.do(func(ctx context.Context) func() {
		src, err := a.svc.Downloads.ListDestinations(ctx)
		return func() {
			if err != nil {
				a.recoveryState, a.recoveryErr = loadFailed, errMessage(err)
				return
			}
			a.recovery, a.recoveryState, a.recoveryErr = src, loadDone, ""
		}
	})
}

func (a *App) failed(err error) {
	if a.signedIn() {
		a.toastError(err)
	}
}

func (a *App) callTransfer(call func(ctx context.Context) (*gui.TransferDTO, error), then func(gui.TransferDTO)) {
	a.do(func(ctx context.Context) func() {
		t, err := call(ctx)
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.upsertTransfer(*t)
			if then != nil {
				then(*t)
			}
		}
	})
}

func (a *App) upsertTransfer(t gui.TransferDTO) {
	target := &a.active
	other := &a.history
	if isTerminalStage(t.Stage) {
		target, other = other, target
	}
	*other = removeTransfer(*other, t.ID)
	for i := range *target {
		if (*target)[i].ID == t.ID {
			(*target)[i] = t
			return
		}
	}
	*target = append([]gui.TransferDTO{t}, *target...)
}

func removeTransfer(list []gui.TransferDTO, id string) []gui.TransferDTO {
	for i := range list {
		if list[i].ID == id {
			return append(list[:i:i], list[i+1:]...)
		}
	}
	return list
}

func (a *App) upsertItem(it gui.TransferItemDTO) {
	st := a.items[it.TransferID]
	if st == nil || st.state != loadDone || it.ID == "" {
		return
	}
	for i := range st.items {
		if st.items[i].ID == it.ID {
			st.items[i] = it
			return
		}
	}
	st.items = append(st.items, it)
}

func (a *App) upsertRule(r gui.BackupRuleDTO) {
	for i := range a.rules {
		if a.rules[i].ID == r.ID {
			if a.rules[i].Status != r.Status || a.rules[i].LastScanAt != r.LastScanAt {
				a.staleFiles(r.ID)
			}
			a.rules[i] = r
			return
		}
	}
	a.rules = append(a.rules, r)
}

func (a *App) removeRuleLocal(id string) {
	for i := range a.rules {
		if a.rules[i].ID == id {
			a.rules = append(a.rules[:i:i], a.rules[i+1:]...)
			delete(a.probes, id)
			delete(a.files, id)
			return
		}
	}
}

func (a *App) itemsFor(transferID string) *itemsState {
	st := a.items[transferID]
	if st == nil {
		st = &itemsState{}
		a.items[transferID] = st
	}
	if st.state == loadIdle {
		a.loadItems(transferID)
	}
	return st
}

func (a *App) loadItems(transferID string) {
	st := a.items[transferID]
	if st == nil {
		st = &itemsState{}
		a.items[transferID] = st
	}
	st.state = loadRunning
	a.do(func(ctx context.Context) func() {
		items, err := a.svc.Transfers.GetItems(ctx, transferID)
		return func() {
			if err != nil {
				st.state, st.err = loadFailed, errMessage(err)
				return
			}
			st.state, st.items, st.err = loadDone, items, ""
		}
	})
}

func (a *App) probeFor(nodeID string) *probeState {
	st := a.probes[nodeID]
	if st == nil {
		st = &probeState{}
		a.probes[nodeID] = st
	}
	if st.state == loadIdle {
		a.loadProbe(nodeID)
	}
	return st
}

func (a *App) loadProbe(nodeID string) {
	st := a.probes[nodeID]
	if st == nil {
		st = &probeState{}
		a.probes[nodeID] = st
	}
	st.state = loadRunning
	a.do(func(ctx context.Context) func() {
		capab, err := a.svc.Transfers.ProbeDestination(ctx, nodeID)
		return func() {
			if err != nil {
				st.state, st.err = loadFailed, errMessage(err)
				return
			}
			st.state, st.capab, st.err = loadDone, *capab, ""
		}
	})
}

func (a *App) thumbnail(key string, data []byte) *ui.Bitmap {
	if bm, ok := a.thumbs[key]; ok {
		return bm
	}
	var bm *ui.Bitmap
	if len(data) > 0 {
		if decoded, err := ui.DecodeBitmap(data); err == nil {
			bm = decoded
		}
	}
	a.thumbs[key] = bm
	return bm
}

func (a *App) Navigate(path string) {
	if path == "/settings" || a.signedIn() {
		a.router.Push(path)
	}
}
func (a *App) ShowAbout() { a.aboutOpen = true }
func (a *App) OpenUpload(folder bool) {
	if !a.signedIn() {
		return
	}
	a.router.Push("/upload")
	if folder {
		a.pickDirectory()
	} else {
		a.pickFiles()
	}
}
