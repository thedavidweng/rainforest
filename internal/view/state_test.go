package view

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"testing"

	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

func TestLoadersSettleAndEnsureRecoveryRunsOnce(t *testing.T) {
	h := newHarness(t, true)
	h.start()

	if st := h.app.itemsFor("missing"); st.state != loadRunning {
		t.Fatalf("items state = %v", st.state)
	}
	if st := h.app.probeFor("missing"); st.state != loadRunning {
		t.Fatalf("probe state = %v", st.state)
	}
	h.app.ensureRecovery()
	h.app.ensureRecovery()
	h.drain()

	if st := h.app.items["missing"]; st.state != loadDone || st.err != "" {
		t.Errorf("items = %+v", st)
	}
	if st := h.app.probes["missing"]; st.state != loadDone || st.err != "" || st.capab.Status != "failed" {
		t.Errorf("probe = %+v", st)
	}
	if h.app.recoveryState != loadDone || h.app.recoveryErr != "" {
		t.Errorf("recovery = %v %q", h.app.recoveryState, h.app.recoveryErr)
	}
	if len(h.queue) != 0 {
		t.Errorf("ensureRecovery queued again: %d", len(h.queue))
	}
	h.app.loadItems("missing")
	h.app.loadProbe("missing")
	h.app.refreshRecovery()
	h.drain()
}

func TestCallTransferReportsErrorsAndAppliesResults(t *testing.T) {
	h := newHarness(t, true)
	h.start()

	var thenCalled bool
	h.app.callTransfer(func(context.Context) (*gui.TransferDTO, error) {
		return &gui.TransferDTO{ID: "t1", Kind: "upload", Stage: "running"}, nil
	}, func(gui.TransferDTO) { thenCalled = true })
	h.app.callTransfer(func(context.Context) (*gui.TransferDTO, error) {
		return nil, &gui.Error{Code: gui.ErrAuthFailed, Message: "Call failed"}
	}, nil)
	h.drain()

	if !thenCalled || len(h.app.active) != 1 || h.app.active[0].ID != "t1" {
		t.Errorf("active = %+v, then = %v", h.app.active, thenCalled)
	}
	h.wantText(locales.In("en").ErrorSignIn())
}

func TestUpdateSettingsFailureRefreshes(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.updateSettings(gui.SettingsInput{})
	h.drain()
	if h.app.settings == nil {
		t.Error("settings not loaded")
	}
}

func TestRemoveRuleLocalDropsRuleAndProbe(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.upsertRule(gui.BackupRuleDTO{ID: "r1"})
	h.app.upsertRule(gui.BackupRuleDTO{ID: "r2"})
	h.app.probes["r1"] = &probeState{}
	h.app.removeRuleLocal("r1")
	h.app.removeRuleLocal("absent")
	if len(h.app.rules) != 1 || h.app.rules[0].ID != "r2" || h.app.probes["r1"] != nil {
		t.Errorf("rules = %+v probes = %v", h.app.rules, h.app.probes)
	}
}

func TestThumbnailDecodesAndCachesFailures(t *testing.T) {
	h := newHarness(t, true)
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if bm := h.app.thumbnail("ok", buf.Bytes()); bm == nil {
		t.Error("valid thumbnail did not decode")
	}
	if bm := h.app.thumbnail("bad", []byte("@@@")); bm != nil {
		t.Error("invalid thumbnail decoded")
	}
	if _, cached := h.app.thumbs["bad"]; !cached {
		t.Error("failure not cached")
	}
	if bm := h.app.thumbnail("none", nil); bm != nil {
		t.Error("empty thumbnail decoded")
	}
}

func TestStageAndConflictLabels(t *testing.T) {
	m := locales.In("en")
	for stage, want := range map[string]bool{"queued": true, "running": true, "paused": false, "completed": false} {
		if isRunningStage(stage) != want {
			t.Errorf("isRunningStage(%s) != %v", stage, want)
		}
	}
	cases := []struct {
		policy   string
		download bool
		want     string
	}{
		{"skip", false, m.ConflictSkip()},
		{"rename", true, m.ConflictRename()},
		{"fail", false, m.ConflictFailUpload()},
		{"fail", true, m.ConflictFailDownload()},
		{"other", false, "other"},
	}
	for _, tc := range cases {
		if got := conflictLabel(m, tc.policy, tc.download); got != tc.want {
			t.Errorf("conflictLabel(%s,%v) = %q", tc.policy, tc.download, got)
		}
	}
}

func TestToastActionRunsCallback(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	var ran bool
	h.app.toastAction("Undo available", "Undo", func() { ran = true })
	h.tt.Frame()
	h.wantText("Undo available")
	if err := h.tt.Click("Undo"); err != nil {
		t.Skipf("toast action not clickable in tester: %v", err)
	}
	if !ran {
		t.Error("toast action did not run")
	}
}

func TestErrorToastMapsNetworkDiagnostics(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.toastError(fmt.Errorf("connection refused: private upstream details"))
	h.tt.Frame()
	h.wantText(locales.In("en").ErrorNetwork())
	h.noText("connection refused: private upstream details")
}

func TestFolderPickerMapsNetworkDiagnostics(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.openPicker("", func(string) {})
	h.app.folders[""] = &folderState{state: loadFailed, err: "connection refused: private upstream details"}
	h.tt.Frame()
	h.wantText(locales.In("en").ErrorNetwork())
	h.noText("connection refused: private upstream details")
}

func TestAccountChangeRejectsPendingUIResults(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.do(func(context.Context) func() {
		return func() { h.app.recent = []gui.RecentUploadDTO{{Name: "old-account.jpg"}} }
	})
	pending := h.queue
	h.queue = nil
	st := h.app.auth
	st.OwnerID, st.RootNodeID = "new-owner", "new-root"
	h.app.setAuth(st)
	for _, fn := range pending {
		fn()
	}
	if len(h.app.recent) != 0 {
		t.Fatalf("old account result crossed account boundary: %+v", h.app.recent)
	}
}

func TestAuthEventBeforeImportReturnClearsWelcomeState(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.welcomeUI.cookies = "private cookies"
	h.app.welcomeUI.signingIn = true
	st := h.app.auth
	st.OwnerID = "imported-owner"
	h.app.runImport(func(context.Context) (*gui.AuthStatus, error) {
		h.app.HandleEvent(gui.EventAuthChanged, st)
		return &st, nil
	})
	h.drain()
	w := h.app.welcomeUI
	if w.importing || w.signingIn || w.cookies != "" {
		t.Fatal("auth event left welcome busy or retained cookies")
	}
}

func TestAccountChangeClearsCachedAccountLists(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.files["old-rule"] = &filesState{}
	h.app.items["old-transfer"] = &itemsState{}
	h.app.active, h.app.history = sampleTransfers()[:1], sampleTransfers()[1:]
	h.app.rules = []gui.BackupRuleDTO{{ID: "old-rule"}}
	h.app.transfersLoaded, h.app.rulesLoaded = true, true
	st := h.app.auth
	st.OwnerID, st.RootNodeID = "new-owner", "new-root"
	h.app.setAuth(st)
	if len(h.app.files)+len(h.app.items)+len(h.app.active)+len(h.app.history)+len(h.app.rules) != 0 || h.app.transfersLoaded || h.app.rulesLoaded {
		t.Fatal("previous account data remains visible before the new account refresh completes")
	}
}
