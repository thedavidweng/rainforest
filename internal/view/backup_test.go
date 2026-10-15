package view

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

type dirPicker struct{ dir string }

func (p dirPicker) PickCookieFile(context.Context) (string, error)        { return "", nil }
func (p dirPicker) PickFiles(context.Context) ([]string, error)           { return nil, nil }
func (p dirPicker) PickDirectory(context.Context, string) (string, error) { return p.dir, nil }

func openBackup(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, true)
	h.start()
	if err := h.tt.Click(locales.NavBackup()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	return h
}

func (h *harness) addRule(dir string) gui.BackupRuleDTO {
	h.t.Helper()
	rule, err := h.svc.Backups.Add(h.t.Context(), gui.BackupRuleInput{
		LocalPath: dir, IncludeVideos: true, ScheduleMode: scheduleImmediate, ConflictPolicy: "skip",
	})
	if err != nil {
		h.t.Fatalf("Backups.Add: %v", err)
	}
	h.app.refreshBackups()
	h.drain()
	return *rule
}

func (h *harness) clickDialogConfirm() {
	h.t.Helper()
	r, ok := h.tt.Find(locales.CommonCancel())
	if !ok {
		h.t.Fatalf("no dialog footer; texts: %q", h.tt.Texts())
	}
	h.tt.ClickAt(r.X+r.W+60, r.Y+r.H/2)
	h.drain()
}

func (h *harness) chooseRuleMenu(name, item string) {
	h.t.Helper()
	if !h.tt.HasText(name) {
		h.t.Fatalf("no rule %q; texts: %q", name, h.tt.Texts())
	}
	if err := h.tt.Click(locales.CommonMoreActions()); err != nil {
		h.t.Fatalf("more actions: %v", err)
	}
	if err := h.tt.ChooseMenuItem(item); err != nil {
		h.t.Fatalf("menu %q: %v (items %q)", item, err, h.tt.Menu())
	}
	h.drain()
}

func TestBackupEmptyStateOffersAddFolder(t *testing.T) {
	h := openBackup(t)
	h.wantText(locales.BackupEmptyTitle())
	h.noText(locales.BackupPauseAll())
	h.wantText(locales.BackupAddFolder())
}

func TestBackupAddRuleThroughDialog(t *testing.T) {
	h := openBackup(t)
	dir := t.TempDir()
	h.svc.SetFilePicker(dirPicker{dir: dir})
	if err := h.tt.Click(locales.BackupAddFolder()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if !h.app.backup.open || h.app.backup.form.path != dir {
		t.Fatalf("dialog open=%v path=%q", h.app.backup.open, h.app.backup.form.path)
	}
	h.wantText(locales.BackupFolder())
	h.clickDialogConfirm()
	if h.app.backup.open {
		t.Fatalf("dialog still open, err %q", h.app.backup.form.err)
	}
	rules, err := h.svc.Backups.List(t.Context())
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules = %v, %v", rules, err)
	}
	if rules[0].LocalPath != dir || !rules[0].IncludeVideos || rules[0].ScheduleMode != scheduleImmediate {
		t.Fatalf("stored rule = %+v", rules[0])
	}
	h.wantText(filepath.Base(dir))
}

func TestBackupSaveRejectsInvalidInterval(t *testing.T) {
	h := openBackup(t)
	h.app.openFolderForm(nil, t.TempDir())
	h.app.backup.form.schedule = scheduleInterval
	h.app.backup.form.minutes = 1
	h.drain()
	h.clickDialogConfirm()
	h.wantText(locales.BackupErrorInterval())
	if !h.app.backup.open {
		t.Fatal("dialog closed on invalid input")
	}
	if rules, _ := h.svc.Backups.List(t.Context()); len(rules) != 0 {
		t.Fatalf("rule saved despite error: %v", rules)
	}
}

func TestBackupEditKeepsPausedState(t *testing.T) {
	h := openBackup(t)
	rule := h.addRule(t.TempDir())
	if _, err := h.svc.Backups.Pause(t.Context(), rule.ID); err != nil {
		t.Fatal(err)
	}
	h.app.refreshBackups()
	h.drain()
	h.chooseRuleMenu(filepath.Base(rule.LocalPath), locales.CommonEdit())
	if !h.app.backup.open || h.app.backup.form.ruleID != rule.ID {
		t.Fatalf("edit dialog not opened: %+v", h.app.backup)
	}
	h.wantText(locales.BackupEditFolder())
	h.app.backup.form.schedule = scheduleInterval
	h.app.backup.form.minutes = 45
	h.drain()
	h.clickDialogConfirm()
	rules, err := h.svc.Backups.List(t.Context())
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules = %v, %v", rules, err)
	}
	got := rules[0]
	if got.ScheduleMode != scheduleInterval || got.IntervalMinutes != 45 || !got.Paused {
		t.Fatalf("after edit = %+v", got)
	}
}

func TestBackupRemoveRequiresConfirmation(t *testing.T) {
	h := openBackup(t)
	rule := h.addRule(t.TempDir())
	name := filepath.Base(rule.LocalPath)
	h.chooseRuleMenu(name, locales.BackupRemoveMenu())
	h.wantText(locales.BackupRemoveTitle(name))
	if err := h.tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if rules, _ := h.svc.Backups.List(t.Context()); len(rules) != 1 {
		t.Fatalf("cancel removed the rule: %v", rules)
	}
	h.chooseRuleMenu(name, locales.BackupRemoveMenu())
	if err := h.tt.Click(locales.CommonRemove()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if rules, _ := h.svc.Backups.List(t.Context()); len(rules) != 0 {
		t.Fatalf("rule not removed: %v", rules)
	}
	h.wantText(locales.BackupEmptyTitle())
}

func TestBackupPauseAndResumeRule(t *testing.T) {
	h := openBackup(t)
	rule := h.addRule(t.TempDir())
	name := filepath.Base(rule.LocalPath)
	h.chooseRuleMenu(name, locales.CommonPause())
	if got, _ := h.svc.Backups.List(t.Context()); len(got) != 1 || !got[0].Paused {
		t.Fatalf("not paused: %+v", got)
	}
	h.wantText(locales.RulePaused())
	h.chooseRuleMenu(name, locales.CommonResume())
	if got, _ := h.svc.Backups.List(t.Context()); len(got) != 1 || got[0].Paused {
		t.Fatalf("not resumed: %+v", got)
	}
}

func TestBackupPauseAllAndResumeAll(t *testing.T) {
	h := openBackup(t)
	h.addRule(t.TempDir())
	if err := h.tt.Click(locales.BackupPauseAll()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if !h.app.backupsStopped() {
		t.Fatal("backups not stopped")
	}
	h.wantText(locales.BackupAllPaused())
	h.wantText(locales.BackupResumeAll())
	if err := h.tt.Click(locales.BackupResumeAll()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.backupsStopped() {
		t.Fatal("backups still stopped")
	}
	h.noText(locales.BackupAllPaused())
}

func TestBackupShowsUnavailableSource(t *testing.T) {
	h := openBackup(t)
	h.app.rules = []gui.BackupRuleDTO{{ID: "x", LocalPath: "/Volumes/Gone/Trips", ScheduleMode: scheduleImmediate, Status: "source_unavailable"}}
	h.drain()
	h.wantText("Trips")
	h.wantText(locales.BackupUnavailable())
}
