package view

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/thedavidweng/rainforest/locales"
)

func openSettings(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, true)
	h.tt.SetSize(1080, 1800)
	h.start()
	if err := h.tt.Click(locales.NavSettings()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	return h
}

func (h *harness) clickSwitchOf(title string) {
	h.t.Helper()
	r, ok := h.tt.Find(title)
	if !ok {
		h.t.Fatalf("no row %q; texts: %q", title, h.tt.Texts())
	}
	h.tt.ClickAt(1018, r.Y+r.H/2)
	h.drain()
}

func TestSettingsShowsAccountAndUsage(t *testing.T) {
	h := openSettings(t)
	h.wantText(locales.SettingsAccount())
	h.wantText("amazon.com")
	h.wantText(locales.RegionNameUs())
	h.wantText(locales.SettingsVerified())
	h.wantText(formatBytes(10240))
	h.wantText(locales.SettingsUsageCount(5))
	h.wantText("Version test")
}

func TestSettingsSwitchPersists(t *testing.T) {
	h := openSettings(t)
	before, err := h.svc.Settings.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	h.clickSwitchOf(locales.SettingsNotifications())
	after, err := h.svc.Settings.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if after.NotificationsEnabled == before.NotificationsEnabled {
		t.Fatalf("notifications still %v", after.NotificationsEnabled)
	}
	if h.app.settings.NotificationsEnabled != after.NotificationsEnabled {
		t.Fatalf("view state %v, stored %v", h.app.settings.NotificationsEnabled, after.NotificationsEnabled)
	}
}

func TestSettingsConflictPolicyPersists(t *testing.T) {
	h := openSettings(t)
	if err := h.tt.Click(locales.ConflictSkip()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if err := h.tt.Click(locales.ConflictRename()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	got, err := h.svc.Settings.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultConflictPolicy != "rename" {
		t.Fatalf("policy = %q", got.DefaultConflictPolicy)
	}
}

func TestSettingsSignOutAsksFirst(t *testing.T) {
	h := openSettings(t)
	if err := h.tt.Click(locales.SettingsSignOut()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.SettingsSignOutTitle())
	h.wantText(locales.SettingsSignOutBody())
	if !h.app.signedIn() {
		t.Fatal("signed out before confirming")
	}

	if err := h.tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.noText(locales.SettingsSignOutTitle())
	if !h.app.signedIn() {
		t.Fatal("cancel signed out")
	}
}

func TestSettingsSignOutConfirmed(t *testing.T) {
	h := openSettings(t)
	if err := h.tt.Click(locales.SettingsSignOut()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.tt.KeyDown(ui.KeyEnter)
	h.tt.KeyUp(ui.KeyEnter)
	h.drain()
	h.wantText(locales.WelcomeTitle())
	st, err := h.svc.Auth.Status(t.Context())
	if err != nil || st.Configured {
		t.Fatalf("status = %+v, err = %v", st, err)
	}
}

func TestSettingsDiagnosticsRendersResults(t *testing.T) {
	h := openSettings(t)
	if err := h.tt.Click(locales.SettingsRunDiagnostics()); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.wantText(locales.SettingsDiagnosticsRunning())
	h.drain()
	h.noText(locales.SettingsDiagnosticsRunning())
	report := h.app.settingsUI.diag
	if report == nil || len(report.Checks) == 0 {
		t.Fatalf("no report: %+v err=%q", report, h.app.settingsUI.diagErr)
	}
	h.wantText(checkTitle(report.Checks[0].Name))
	h.wantText(locales.SettingsLevel(report.Checks[0].Grade))
}

func TestSettingsRendersInChinese(t *testing.T) {
	h := newHarness(t, true)
	h.tt.SetLocale("zh-Hans")
	h.start()
	zh := locales.In("zh-Hans")
	if err := h.tt.Click(zh.NavSettings()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(zh.SettingsAccount())
	h.wantText(zh.SettingsRunDiagnostics())
}
