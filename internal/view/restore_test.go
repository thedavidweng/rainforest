package view

import (
	"testing"

	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

func openRestore(t *testing.T, rec *gui.RecoverySourcesDTO) *harness {
	t.Helper()
	h := newHarness(t, true)
	h.start()
	h.app.recovery, h.app.recoveryState = rec, loadDone
	if err := h.tt.Click(locales.NavRestore()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	return h
}

func recoveryFixture() *gui.RecoverySourcesDTO {
	return &gui.RecoverySourcesDTO{
		Evidence: "verified",
		ConfirmedItems: []gui.TransferItemDTO{
			{ID: "i1", Name: "IMG_1.HEIC", Size: 2048, RemoteNodeID: "n1", MediaType: "photo"},
			{ID: "i2", Name: "Party.mov", Size: 4096, RemoteNodeID: "n2", MediaType: "video"},
			{ID: "i3", Name: "Local-only.jpg", Size: 1, MediaType: "photo"},
		},
		Albums:  []gui.RecoveryContainerDTO{{ID: "a1", Name: "Summer", Kind: "album"}},
		Folders: []gui.RecoveryContainerDTO{{ID: "f1", Name: "Summer", Kind: "folder"}},
	}
}

func (h *harness) restoreReady() bool {
	h.t.Helper()
	_, ok := h.app.restoreRequest(h.app.restoreChoices(locales.In("en")))
	return ok
}

func (h *harness) chooseRestoreDest(dir string) {
	h.t.Helper()
	h.svc.SetFilePicker(dirPicker{dir: dir})
	if err := h.tt.Click(locales.CommonChoose()); err != nil {
		h.t.Fatal(err)
	}
	h.drain()
}

func TestRestoreListsOnlyDownloadableItems(t *testing.T) {
	h := openRestore(t, recoveryFixture())
	h.wantText("IMG_1.HEIC")
	h.wantText("Party.mov")
	h.noText("Local-only.jpg")
}

func TestRestoreEmptyState(t *testing.T) {
	h := openRestore(t, &gui.RecoverySourcesDTO{Evidence: "verified"})
	h.wantText(locales.RestoreEmptyTitle())
}

func TestRestoreUploadedNeedsSelectionAndDestination(t *testing.T) {
	h := openRestore(t, recoveryFixture())
	if h.restoreReady() {
		t.Fatal("ready with nothing selected")
	}
	if err := h.tt.Click("IMG_1.HEIC"); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.RestoreDownloadItems(1))
	if h.restoreReady() {
		t.Fatal("ready without a destination")
	}
	h.chooseRestoreDest(t.TempDir())
	if !h.restoreReady() {
		t.Fatal("not ready with a selection and destination")
	}
}

func TestRestoreSelectAllTogglesEveryItem(t *testing.T) {
	h := openRestore(t, recoveryFixture())
	if err := h.tt.Click(locales.RestoreSelectAll()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.RestoreDownloadItems(2))
	h.wantText(locales.RestoreSelected(2, formatBytes(6144)))
	if err := h.tt.Click(locales.RestoreSelectAll()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.noText(locales.RestoreDownloadItems(2))
}

func TestRestoreStartDownloadForSelectedItems(t *testing.T) {
	h := openRestore(t, recoveryFixture())
	if err := h.tt.Click(locales.RestoreSelectAll()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	dest := t.TempDir()
	h.chooseRestoreDest(dest)
	if err := h.tt.Click(locales.RestoreDownloadItems(2)); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.restore.starting {
		t.Fatal("still starting")
	}
	if len(h.app.active)+len(h.app.history) != 1 {
		t.Fatalf("transfers: active %v history %v; toasts %v", h.app.active, h.app.history, h.app.toasts)
	}
	if picked, _ := h.app.restoreSelection(h.app.restoreItems()); len(picked) != 0 {
		t.Fatal("selection not cleared after starting")
	}
	list, err := h.svc.Transfers.List(t.Context())
	if err != nil || len(list.Active)+len(list.History) != 1 {
		t.Fatalf("stored transfers = %+v, %v", list, err)
	}
}

func TestRestoreContainerModeNeedsChoice(t *testing.T) {
	h := openRestore(t, recoveryFixture())
	if err := h.tt.Click(locales.RestoreSourceContainer()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.restore.mode != restoreContainer {
		t.Fatalf("mode = %d", h.app.restore.mode)
	}
	h.chooseRestoreDest(t.TempDir())
	if h.restoreReady() {
		t.Fatal("ready without choosing a container")
	}
	h.app.restore.container = locales.RestoreAlbumOption("Summer")
	h.drain()
	if !h.restoreReady() {
		t.Fatal("not ready after choosing an album")
	}
	in, _ := h.app.restoreRequest(h.app.restoreChoices(locales.In("en")))
	if in.ContainerNodeID != "a1" {
		t.Fatalf("container = %q", in.ContainerNodeID)
	}
	h.wantText(locales.RestoreDownloadAlbum())
	h.app.restore.container = locales.RestoreFolderOption("Summer")
	h.drain()
	h.wantText(locales.RestoreDownloadFolder())
}

func TestRestoreContainerModeBrowsesFolders(t *testing.T) {
	rec := recoveryFixture()
	rec.Albums, rec.Folders = nil, nil
	h := openRestore(t, rec)
	if err := h.tt.Click(locales.RestoreSourceContainer()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.restore.mode != restoreContainer {
		t.Fatalf("mode = %d, want container", h.app.restore.mode)
	}
	h.chooseRestoreDest(t.TempDir())
	if h.restoreReady() {
		t.Fatal("ready before choosing a folder")
	}
	if err := h.tt.Click(locales.RestoreBrowseFolders()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.PickerTitle())
	if err := h.tt.Click("Trips"); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if err := h.tt.Click(locales.PickerChoose()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	in, ok := h.app.restoreRequest(nil)
	if !ok || in.ContainerNodeID != "trips" {
		t.Fatalf("request = %+v, ok %v", in, ok)
	}
	h.wantText(locales.RestoreDownloadFolder())
	h.wantText(locales.DestLibrary() + pathSeparator + "Trips")
}

func TestRestoreByIDParsesAndDedupes(t *testing.T) {
	h := openRestore(t, recoveryFixture())
	if err := h.tt.Click(locales.RestoreSourceById()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.chooseRestoreDest(t.TempDir())
	if h.restoreReady() {
		t.Fatal("ready without ids")
	}
	h.app.restore.ids = "node-a, node-b\nnode-a;node-c"
	h.drain()
	h.wantText(locales.RestoreIdCount(3))
	in, ok := h.app.restoreRequest(nil)
	if !ok || len(in.NodeIDs) != 3 {
		t.Fatalf("request = %+v, ok %v", in, ok)
	}
}

func TestRestoreShowsExperimentalBanner(t *testing.T) {
	rec := recoveryFixture()
	rec.Evidence = "experimental"
	rec.CapabilityNotes = "Listing is unverified."
	h := openRestore(t, rec)
	h.wantText(locales.RestoreExperimental())
	h.wantText("Listing is unverified.")
}

func TestRestoreSurfacesLoadFailureWithRetry(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.recoveryState, h.app.recoveryErr = loadFailed, "boom"
	if err := h.tt.Click(locales.NavRestore()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText("boom")
	h.wantText(locales.CommonRetry())
}
