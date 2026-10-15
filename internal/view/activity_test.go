package view

import (
	"testing"

	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

func sampleTransfers() []gui.TransferDTO {
	now := "2026-10-14T10:00:00Z"
	return []gui.TransferDTO{
		{ID: "a1", Kind: "upload", Stage: "running", SourceSummary: "Summer trip", ItemsTotal: 120, ItemsDone: 34, BytesTotal: 4 << 30, BytesDone: 1 << 30, CreatedAt: now, UpdatedAt: now},
		{ID: "a2", Kind: "backup", Stage: "paused", SourceSummary: "Pictures", ItemsTotal: 40, ItemsDone: 10, BytesTotal: 1 << 29, BytesDone: 1 << 27, CreatedAt: now, UpdatedAt: now},
		{ID: "h1", Kind: "upload", Stage: "failed", SourceSummary: "Wedding", ItemsTotal: 12, ItemsFailed: 3, BytesTotal: 1 << 28, ErrorMessage: "Network is unreachable", CreatedAt: now, UpdatedAt: now, FinishedAt: now},
		{ID: "h2", Kind: "backup", Stage: "completed", SourceSummary: "Holiday", ItemsTotal: 120, ItemsDone: 120, BytesTotal: 4 << 30, BytesDone: 4 << 30, CreatedAt: now, UpdatedAt: now, FinishedAt: now},
		{ID: "h3", Kind: "download", Stage: "unconfirmed", SourceSummary: "Family", ItemsTotal: 5, ItemsUnconfirmed: 2, CreatedAt: now, UpdatedAt: now, FinishedAt: now},
	}
}

func seedActivity(h *harness) {
	list := sampleTransfers()
	for i := range list {
		h.app.upsertTransfer(list[i])
	}
	h.app.transfersLoaded = true
	h.app.router.Replace("/activity")
	h.drain()
}

func TestActivityEmptyState(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.router.Replace("/activity")
	h.drain()
	h.wantText(locales.ActivityEmptyTitle())
	h.noText(locales.ActivityClearHistory())
	if err := h.tt.Click(locales.ActivityEmptyAction()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.router.Path() != "/upload" {
		t.Fatalf("path = %q", h.app.router.Path())
	}
}

func TestActivityRendersActiveAndHistory(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	seedActivity(h)
	for _, s := range []string{
		locales.ActivityInProgress(), locales.ActivityEarlier(), "Summer trip", "Wedding", "Holiday",
		locales.StageUploading(), locales.StagePaused(), locales.StageFailed(), locales.StageCompleted(),
		locales.ActivityFailedCount(3), locales.CommonRetry(), locales.ActivityUnconfirmedDownload(), "Network is unreachable",
	} {
		h.wantText(s)
	}
	h.wantText("25%")
}

func TestActivityControlsCallThroughToService(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	seedActivity(h)
	for _, label := range []string{locales.CommonPause(), locales.CommonResume(), locales.ActivityCancel(), locales.CommonRetry()} {
		h.app.toasts = nil
		if err := h.tt.Click(label); err != nil {
			t.Fatalf("click %q: %v", label, err)
		}
		h.drain()
		if len(h.queue) != 0 {
			t.Fatalf("%q left work queued", label)
		}
		if !h.hasErrorToast() {
			t.Errorf("%q: service was not called (no error toast for unknown transfer)", label)
		}
	}
}

func TestActivityContextMenuOffersActions(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	seedActivity(h)
	if err := h.tt.RightClick("Wedding"); err != nil {
		t.Fatal(err)
	}
	menu := h.tt.Menu()
	want := map[string]bool{locales.CommonRetry(): false, locales.ActivityDetails(): false}
	for _, item := range menu {
		if _, ok := want[item]; ok {
			want[item] = true
		}
	}
	for item, ok := range want {
		if !ok {
			t.Errorf("menu %q lacks %q", menu, item)
		}
	}
	if err := h.tt.ChooseMenuItem(locales.ActivityDetails()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if !h.app.pages.activity.expanded["h1"] {
		t.Fatal("Show details did not expand the row")
	}
}

func TestActivityExpandLoadsItems(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	seedActivity(h)
	h.app.items["h2"] = &itemsState{state: loadDone, items: []gui.TransferItemDTO{
		{ID: "i1", TransferID: "h2", Name: "IMG_0001.jpg", Size: 2048, Status: "confirmed", RemoteNodeID: "node-abc"},
		{ID: "i2", TransferID: "h2", Name: "IMG_0002.jpg", Size: 4096, Status: "failed", ErrorMessage: "Upload rejected"},
	}}
	row := h.tt
	if err := row.Click(locales.CommonExpand()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if len(h.app.pages.activity.expanded) != 1 {
		t.Fatalf("expanded = %v", h.app.pages.activity.expanded)
	}
}

func TestActivityLoadsItemsFromServiceOnExpand(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	seedActivity(h)
	h.app.pages.activity.setExpanded("h1", true)
	h.tt.Frame()
	h.wantText(locales.ActivityLoadingItems())
	h.drain()
	if st := h.app.items["h1"]; st == nil || st.state == loadIdle || st.state == loadRunning {
		t.Fatalf("items state = %+v", st)
	}
}

func TestActivityClearHistoryAsksFirst(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	seedActivity(h)
	if err := h.tt.Click(locales.ActivityClearHistory()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.ActivityClearTitle())
	if len(h.app.history) == 0 {
		t.Fatal("history cleared before confirming")
	}
	if err := h.tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if len(h.app.history) == 0 {
		t.Fatal("history cleared after cancel")
	}

	if err := h.tt.Click(locales.ActivityClearHistory()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if err := h.tt.Click(locales.ActivityClearConfirm()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if len(h.app.history) != 0 || len(h.app.active) != 2 {
		t.Fatalf("history=%d active=%d", len(h.app.history), len(h.app.active))
	}
	h.noText(locales.ActivityClearHistory())
}

func TestActivityWarningBannerIsDismissible(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	seedActivity(h)
	h.app.transferWarning = "Some history could not be read."
	h.drain()
	h.wantText("Some history could not be read.")
	if err := h.tt.Click(locales.CommonDismiss()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.noText("Some history could not be read.")
}

func (h *harness) hasErrorToast() bool {
	return h.tt.HasText("not found") || h.tt.HasText("Not found")
}
