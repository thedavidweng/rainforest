package main

import (
	"testing"

	"github.com/egoist/mygo"
)

func TestDesktopMenuPlatformConventions(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		t.Run(platform, func(t *testing.T) {
			d := &desktopShell{}
			items := map[string]*mygo.MenuItem{}
			var visit func([]*mygo.MenuItem)
			visit = func(menu []*mygo.MenuItem) {
				for _, item := range menu {
					if item.ID != "" {
						items[item.ID] = item
					}
					visit(item.Submenu)
				}
			}
			menu := d.menuTemplate(platform, "en")
			visit(menu)
			for id, key := range map[string]string{"settings": "CmdOrCtrl+,", "quit": "CmdOrCtrl+Q", "upload-files": "CmdOrCtrl+O", "upload-folder": "CmdOrCtrl+Shift+O"} {
				if item := items[id]; item == nil || item.Accelerator != key || item.Click == nil && item.Role == "" {
					t.Errorf("%s: missing action or shortcut %s: %+v", id, key, item)
				}
			}
			if items["about"] == nil || items["language"] == nil {
				t.Fatal("About or language menu missing")
			}
			if platform == "darwin" && (menu[0].Role != mygo.RoleAppMenu || menu[0].Submenu[0].ID != "about") {
				t.Fatal("macOS About must lead the app menu")
			}
			if items["quit"].Role != mygo.RoleQuit {
				t.Fatal("quit must use the framework lifecycle and quit confirmation")
			}
		})
	}
}
