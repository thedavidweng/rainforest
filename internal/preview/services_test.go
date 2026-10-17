package preview

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/view"
	"github.com/thedavidweng/rainforest/locales"
)

func TestPreviewUsesRealViews(t *testing.T) {
	icon, err := os.ReadFile("../../resources/icon.png")
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan func(), 32)
	app := view.NewWithServices(Services(), view.Options{Preview: true, Icon: icon, Update: func(fn func()) { updates <- fn }})
	tt := ui.NewTester(app.View, 1080, 720)
	tt.SetLocale("en")
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	app.Start()
	waitText := func(want string) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for {
			tt.Frame()
			if tt.HasText(want) {
				return
			}
			select {
			case fn := <-updates:
				fn()
			case <-deadline:
				t.Fatalf("missing %q; visible: %q", want, tt.Texts())
			}
		}
	}
	waitText("Family weekend")
	for _, page := range []struct{ nav, content string }{
		{locales.NavActivity(), locales.ActivitySubtitle()},
		{locales.NavBackup(), locales.BackupSubtitle()},
		{locales.NavRestore(), locales.RestoreSubtitle()},
		{locales.NavSettings(), locales.SettingsAccount()},
	} {
		if err := tt.Click(page.nav); err != nil {
			t.Fatal(err)
		}
		waitText(page.content)
	}
}
