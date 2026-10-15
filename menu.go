package main

import (
	"runtime"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/thedavidweng/rainforest/locales"
)

func (d *desktopShell) rebuildMenu() {
	mygo.App.SetMenu(mygo.NewMenu(d.menuTemplate(runtime.GOOS, mygo.App.Locale())))
}

func (d *desktopShell) navigate(path string) {
	d.showWindow()
	d.win.Update(func() { d.app.Load().Navigate(path) })
}

func (d *desktopShell) showAbout() {
	if runtime.GOOS == "darwin" {
		mygo.App.ShowAboutPanel(mygo.AboutPanelOptions{
			ApplicationName: "Rainforest", ApplicationVersion: version,
			Copyright: locales.AboutCopyright(),
			Credits:   locales.SettingsAboutDisclaimer() + "\n\n" + locales.SettingsLicense() + "\nhttps://github.com/thedavidweng/rainforest",
		})
		return
	}
	d.showWindow()
	d.win.Update(func() { d.app.Load().ShowAbout() })
}

func (d *desktopShell) openExternal(url string) {
	if err := mygo.Shell.OpenExternal(url); err != nil {
		mygo.Dialog.Error("Rainforest", err.Error())
	}
}

func (d *desktopShell) menuTemplate(platform, locale string) []*mygo.MenuItem {
	m := locales.In(locale)
	settings := &mygo.MenuItem{
		ID: "settings", Label: m.MenuSettings(), Accelerator: "CmdOrCtrl+,",
		Click: func(*mygo.MenuItem, *mygo.Window) { d.navigate("/settings") },
	}
	about := &mygo.MenuItem{
		ID: "about", Label: m.MenuAbout(),
		Click: func(*mygo.MenuItem, *mygo.Window) { d.showAbout() },
	}
	quit := &mygo.MenuItem{ID: "quit", Role: mygo.RoleQuit, Accelerator: "CmdOrCtrl+Q"}
	file := &mygo.MenuItem{Label: m.MenuFile()}
	for _, entry := range []struct {
		id, label, key string
		folder         bool
	}{
		{"upload-files", m.MenuUploadFiles(), "CmdOrCtrl+O", false},
		{"upload-folder", m.MenuUploadFolder(), "CmdOrCtrl+Shift+O", true},
	} {
		file.Submenu = append(file.Submenu, &mygo.MenuItem{
			ID: entry.id, Label: entry.label, Accelerator: entry.key,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				d.showWindow()
				d.win.Update(func() { d.app.Load().OpenUpload(entry.folder) })
			},
		})
	}
	file.Submenu = append(file.Submenu, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleClose})
	view := &mygo.MenuItem{Label: m.MenuView()}
	for i, entry := range []struct{ path, label string }{
		{"/upload", m.NavUpload()},
		{"/activity", m.NavActivity()},
		{"/backup", m.NavBackup()},
		{"/restore", m.NavRestore()},
	} {
		view.Submenu = append(view.Submenu, &mygo.MenuItem{
			Label: entry.label, Accelerator: "CmdOrCtrl+" + strconv.Itoa(i+1),
			Click: func(*mygo.MenuItem, *mygo.Window) { d.navigate(entry.path) },
		})
	}
	language := &mygo.MenuItem{ID: "language", Label: m.LanguageTitle(), Submenu: []*mygo.MenuItem{
		{Label: m.LanguageSystem(), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.SetLocale("") }},
		mygo.Separator(),
	}}
	for _, tag := range mygo.App.Languages() {
		language.Submenu = append(language.Submenu, &mygo.MenuItem{
			Label: i18n.Autonym(tag), Type: mygo.MenuItemRadio, Checked: tag == locale,
			Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.SetLocale(tag) },
		})
	}
	view.Submenu = append(view.Submenu, mygo.Separator(), language, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleToggleFullScreen})
	help := &mygo.MenuItem{Label: m.MenuHelp(), Submenu: []*mygo.MenuItem{
		{Label: m.MenuHelpGuide(), Click: func(*mygo.MenuItem, *mygo.Window) {
			d.openExternal("https://github.com/thedavidweng/rainforest#readme")
		}},
		{Label: m.MenuReportIssue(), Click: func(*mygo.MenuItem, *mygo.Window) {
			d.openExternal("https://github.com/thedavidweng/rainforest/issues")
		}},
	}}
	var menu []*mygo.MenuItem
	if platform == "darwin" {
		menu = append(menu, &mygo.MenuItem{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			about, mygo.Separator(), settings, mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(), quit,
		}})
		help.Role = mygo.RoleHelp
	} else {
		file.Submenu = append(file.Submenu, mygo.Separator(), settings, mygo.Separator(), quit)
		help.Submenu[0].Accelerator = "F1"
		help.Submenu = append(help.Submenu, mygo.Separator(), about)
	}
	return append(menu, file, &mygo.MenuItem{Role: mygo.RoleEditMenu}, view, &mygo.MenuItem{Role: mygo.RoleWindowMenu}, help)
}
