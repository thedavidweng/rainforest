//go:build !js

package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/autostart"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/internal/view"
	"github.com/thedavidweng/rainforest/locales"
)

var version = "0.1.0"

//go:embed resources/icon.png
var appIcon []byte

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("rainforest %s\n", version)
		return
	}

	mygo.App.SetName("Rainforest")
	mygo.App.SetVersion(version)

	svc := gui.NewEmptyServices()

	mygo.App.WhenReady(func() {
		setupDesktop(svc)
	})

	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type desktopShell struct {
	svc        *gui.Services
	win        *mygo.Window
	app        atomic.Pointer[view.App]
	tray       *mygo.Tray
	quitting   atomic.Bool
	confirming atomic.Bool
}

func resolveDesktopDataDir(primary string) (string, error) {
	return credstore.ResolveDataDir(primary, os.UserConfigDir, os.UserHomeDir)
}

func setupDesktop(svc *gui.Services) {
	primaryDir, _ := mygo.App.Path(mygo.PathUserData)
	userDataDir, err := resolveDesktopDataDir(primaryDir)
	if err != nil {
		mygo.Dialog.Error(locales.StorageError(), err.Error())
		mygo.App.Exit(1)
		return
	}
	shell := &desktopShell{svc: svc}
	autostartMgr := autostart.NewDefaultManager()
	picker := &nativeFilePicker{}
	svc.SetFilePicker(picker)
	svc.SetEmitter(shell.handleEvent)
	svc.SetLaunchAtLoginHook(autostartMgr.SetEnabled)

	if _, err := svc.Init(gui.Options{
		DBPath:          filepath.Join(userDataDir, "rainforest.db"),
		VaultPath:       filepath.Join(userDataDir, "credentials.json"),
		BrowserLogin:    browserLogin,
		Emitter:         shell.handleEvent,
		Picker:          picker,
		LaunchAtLoginFn: autostartMgr.SetEnabled,
	}); err != nil {
		mygo.Dialog.Error(locales.StartupError(), err.Error())
		mygo.App.Exit(1)
		return
	}

	var win *mygo.Window
	app := view.New(svc, view.Options{Update: func(f func()) { win.Update(f) }, Version: version, Icon: appIcon, SetLocale: mygo.App.SetLocale})
	win = mygo.NewWindow(mygo.WindowOptions{
		Title:         "Rainforest",
		Width:         1080,
		Height:        720,
		MinWidth:      860,
		MinHeight:     580,
		StateKey:      "main",
		TitleBarStyle: mygo.TitleBarHidden,
		Vibrancy:      mygo.VibrancySidebar,
		Content:       ui.View(app.View),
	})
	shell.win = win
	picker.win.Store(win)
	shell.app.Store(app)
	app.Start()
	shell.rebuildMenu()
	mygo.App.OnLocaleChanged(func(_ string) {
		shell.rebuildMenu()
		shell.rebuildTrayMenu()
	})

	tray, trayErr := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIconPNG,
		IconIsTemplate: true,
		ToolTip:        "Rainforest",
	})
	if trayErr == nil && tray != nil {
		shell.tray = tray
		tray.OnClick(shell.showWindow)
		shell.rebuildTrayMenu()
	}

	win.OnClose(func(e *mygo.CloseEvent) {
		if shell.quitting.Load() {
			return
		}
		if shell.closeToTrayEnabled() {
			e.PreventDefault()
			win.Hide()
			return
		}
		if shell.shouldPromptQuit() {
			e.PreventDefault()
			go shell.requestQuit()
		}
	})

	mygo.App.OnWindowAllClosed(func() {
		if !shell.closeToTrayEnabled() {
			go shell.requestQuit()
		}
	})

	mygo.App.OnActivate(func(_ bool) {
		shell.showWindow()
	})

	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if shell.quitting.Load() {
			return
		}
		if shell.shouldPromptQuit() {
			e.PreventDefault()
			go shell.requestQuit()
			return
		}
		shell.quitting.Store(true)
	})

	mygo.App.OnQuit(func() {
		svc.Close()
	})
}

func (d *desktopShell) showWindow() {
	if d.win == nil || d.win.IsDestroyed() {
		return
	}
	if d.win.IsMinimized() {
		d.win.Restore()
	}
	d.win.Show()
	d.win.Focus()
}

func (d *desktopShell) handleEvent(name string, data any) {
	if app := d.app.Load(); app != nil {
		app.HandleEvent(name, data)
	}
	switch name {
	case gui.EventTransferStage, gui.EventTransferProgress:
		d.rebuildTrayMenu()
	case gui.EventNotification:
		if v, ok := data.(gui.NotificationEvent); ok {
			if d.notificationsEnabled() {
				n := mygo.NewNotification(mygo.NotificationOptions{
					ID:    v.TransferID,
					Title: v.Title,
					Body:  v.Body,
				})
				_ = n.Show()
			}
		}
	}
}

func (d *desktopShell) rebuildTrayMenu() {
	if d.tray == nil {
		return
	}
	rows := d.svc.TrayTracker().Menu()
	items := make([]*mygo.MenuItem, 0, len(rows))
	for _, row := range rows {
		switch row.Action {
		case gui.TrayActionSeparator:
			items = append(items, mygo.Separator())
		case gui.TrayActionShow:
			items = append(items, &mygo.MenuItem{
				Label: locales.ShowRainforest(),
				Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					d.showWindow()
				},
			})
		case gui.TrayActionQuit:
			items = append(items, &mygo.MenuItem{
				Label: locales.QuitRainforest(),
				Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
					go d.requestQuit()
				},
			})
		default:
			items = append(items, &mygo.MenuItem{
				Label:    row.Label,
				Disabled: true,
			})
		}
	}
	d.tray.SetMenu(mygo.NewMenu(items))
}

func (d *desktopShell) closeToTrayEnabled() bool {
	cfg, err := d.svc.Settings.Get(context.Background())
	if err != nil || cfg == nil {
		return true
	}
	return cfg.CloseToTray
}

func (d *desktopShell) notificationsEnabled() bool {
	cfg, err := d.svc.Settings.Get(context.Background())
	if err != nil || cfg == nil {
		return true
	}
	return cfg.NotificationsEnabled
}

func (d *desktopShell) activeTransferCount() int {
	active := d.svc.TrayTracker().ActiveCount()
	if list, err := d.svc.Transfers.List(context.Background()); err == nil && list != nil {
		active = len(list.Active)
	}
	return active
}

func (d *desktopShell) shouldPromptQuit() bool {
	return gui.DecideQuit(
		d.activeTransferCount(),
		d.svc.ActiveBackupCount(context.Background()),
		d.closeToTrayEnabled(),
	) != gui.QuitImmediately
}

func (d *desktopShell) requestQuit() {
	if d.quitting.Load() {
		return
	}
	activeTransfers := d.activeTransferCount()
	activeBackups := d.svc.ActiveBackupCount(context.Background())
	closeToTray := d.closeToTrayEnabled()

	if gui.DecideQuit(activeTransfers, activeBackups, closeToTray) == gui.QuitImmediately {
		d.confirmQuit()
		return
	}
	if !d.confirming.CompareAndSwap(false, true) {
		return
	}
	defer d.confirming.Store(false)

	res, err := mygo.Dialog.Message(mygo.MessageOptions{
		Parent:        d.win,
		Type:          mygo.MessageQuestion,
		Title:         locales.QuitTitle(),
		Message:       locales.QuitMessage(activeTransfers, activeBackups),
		Buttons:       []string{locales.KeepRunning(), locales.QuitRainforest()},
		DefaultButton: 0,
		CancelButton:  0,
	})
	if err == nil && res.Button == 1 {
		d.confirmQuit()
	}
}

func (d *desktopShell) confirmQuit() {
	d.quitting.Store(true)
	d.svc.Close()
	mygo.App.Quit()
}

type nativeFilePicker struct {
	win atomic.Pointer[mygo.Window]
}

func (p *nativeFilePicker) PickCookieFile(ctx context.Context) (string, error) {
	if v, ok := os.LookupEnv("RAINFOREST_PICK_FILES"); ok {
		if strings.TrimSpace(v) == "" {
			return "", nil
		}
		parts := strings.Split(v, string(os.PathListSeparator))
		return strings.TrimSpace(parts[0]), nil
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:   p.win.Load(),
		Title:    locales.SelectCookieFile(),
		Multiple: false,
		Filters: []mygo.FileFilter{
			{Name: locales.CookieFiles(), Extensions: []string{"txt", "json"}},
			{Name: locales.AllFiles(), Extensions: []string{"*"}},
		},
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func (p *nativeFilePicker) PickFiles(ctx context.Context) ([]string, error) {
	if v, ok := os.LookupEnv("RAINFOREST_PICK_FILES"); ok {
		if strings.TrimSpace(v) == "" {
			return []string{}, nil
		}
		return strings.Split(v, string(os.PathListSeparator)), nil
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:   p.win.Load(),
		Title:    locales.SelectUploadFiles(),
		Multiple: true,
	})
	if err != nil || len(paths) == 0 {
		return []string{}, err
	}
	return paths, nil
}

func (p *nativeFilePicker) PickDirectory(ctx context.Context, title string) (string, error) {
	if v, ok := os.LookupEnv("RAINFOREST_PICK_DIR"); ok {
		return strings.TrimSpace(v), nil
	}
	switch title {
	case "Select Folder to Upload":
		title = locales.SelectUploadFolder()
	case "Select Folder to Back Up":
		title = locales.SelectBackupFolder()
	case "Select Download Destination Directory":
		title = locales.SelectDownloadFolder()
	}
	if strings.TrimSpace(title) == "" {
		title = locales.SelectFolder()
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:            p.win.Load(),
		Title:             title,
		Directory:         true,
		Multiple:          false,
		CreateDirectories: true,
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}
