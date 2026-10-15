package view

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

func TestLoadingBeforeAuthStatus(t *testing.T) {
	h := newHarness(t, false)
	h.tt.Frame()
	h.noText(locales.WelcomeTitle())
	h.noText(locales.NavUpload())
}

func TestWelcomeWhenSignedOut(t *testing.T) {
	h := newHarness(t, false)
	h.start()
	h.wantText(locales.WelcomeTitle())
	h.noText(locales.NavUpload())
}

func TestShellShowsSidebarAndNavigates(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	for _, label := range []string{
		locales.NavUpload(), locales.NavActivity(), locales.NavBackup(), locales.NavRestore(), locales.NavSettings(),
	} {
		h.wantText(label)
	}
	h.wantText("amazon.com")
	h.noText("Rainforest")
	h.noText(locales.AppTagline())
	h.wantText(locales.AccountUsage(5, "10.0 KB"))
	h.noText(locales.WelcomeTitle())

	pages := map[string]string{
		locales.NavActivity(): "/activity",
		locales.NavBackup():   "/backup",
		locales.NavRestore():  "/restore",
		locales.NavSettings(): "/settings",
		locales.NavUpload():   "/upload",
	}
	for label, path := range pages {
		if err := h.tt.Click(label); err != nil {
			t.Fatalf("click %q: %v", label, err)
		}
		h.drain()
		if got := h.app.router.Path(); got != path {
			t.Errorf("after %q path = %q, want %q", label, got, path)
		}
	}
}

func TestSignOutReturnsToWelcome(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.HandleEvent(gui.EventAuthChanged, gui.AuthStatus{})
	h.drain()
	h.wantText(locales.WelcomeTitle())
	h.noText(locales.NavUpload())
}

func TestActivityBadgeFollowsLiveTransferEvents(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	running := gui.TransferDTO{ID: "t1", Kind: "upload", Stage: "running", SourceSummary: "Trip"}
	h.app.HandleEvent(gui.EventTransferStage, running)
	h.drain()
	if len(h.app.active) != 1 || len(h.app.history) != 0 {
		t.Fatalf("active=%d history=%d", len(h.app.active), len(h.app.history))
	}
	h.wantText("1")

	running.Stage = "completed"
	h.app.HandleEvent(gui.EventTransferProgress, gui.TransferProgressEvent{Transfer: running})
	h.drain()
	if len(h.app.active) != 0 || len(h.app.history) != 1 {
		t.Fatalf("active=%d history=%d", len(h.app.active), len(h.app.history))
	}
}

func TestBackupEventsUpsertRulesAndFlagSidebar(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	rule := gui.BackupRuleDTO{ID: "r1", LocalPath: "/tmp/photos", Status: "active"}
	h.app.HandleEvent(gui.EventBackupStatus, rule)
	rule.Status = "error"
	h.app.HandleEvent(gui.EventBackupStatus, rule)
	h.drain()
	if len(h.app.rules) != 1 || !h.app.backupNeedsAttention() {
		t.Fatalf("rules = %+v", h.app.rules)
	}
	h.wantText("!")
}

func TestDroppedFilesOpenReviewOnUploadPage(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	if err := h.tt.Click(locales.NavSettings()); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeJPEGs(t, dir, 1)
	h.app.openUploadReview([]string{filepath.Join(dir, "photo-0.jpg")})
	h.app.router.Push("/upload")
	h.drain()
	if got := h.app.pages.review.paths; len(got) != 1 {
		t.Fatalf("review paths = %v", got)
	}
	if plan := h.app.pages.review.plan; plan == nil || plan.TotalCount != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if h.app.router.Path() != "/upload" {
		t.Fatalf("path = %q", h.app.router.Path())
	}
}

func TestToastsQueuedFromAsyncResultsShowOnNextFrame(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.do(func(context.Context) func() {
		return func() { h.app.toast("Hello toast") }
	})
	h.drain()
	h.wantText("Hello toast")
}
