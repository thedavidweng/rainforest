package view

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRecentUploadsCardShowsTilesAndLink(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.noText(locales.UploadRecentUploads())
	now := time.Now().UTC().Format(time.RFC3339)
	h.app.recent = []gui.RecentUploadDTO{
		{Name: "beach.jpg", RemoteNodeID: "n1", UpdatedAt: now, Thumbnail: jpegBytes(t)},
		{Name: "clip.mov", RemoteNodeID: "n2", UpdatedAt: now},
	}
	h.app.lastUpload = now
	h.drain()
	h.wantText(locales.UploadRecentUploads())
	h.wantText(locales.UploadViewInPhotos())
	if !strings.HasPrefix(h.app.photosURL(), "https://www.amazon.") || !strings.HasSuffix(h.app.photosURL(), "/photos") {
		t.Fatalf("photosURL = %q", h.app.photosURL())
	}
	h.wantText(locales.UploadRecent())
}

func TestRecentUploadsRefreshHidesCardWhenEmpty(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.recent = []gui.RecentUploadDTO{{Name: "x.jpg", RemoteNodeID: "n1"}}
	h.drain()
	h.app.refreshRecent()
	h.drain()
	h.noText(locales.UploadRecentUploads())
}

func TestPickerNavigatesAndChoosesFolder(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	var chosen []string
	h.app.openPicker("", func(id string) { chosen = append(chosen, id) })
	h.drain()
	h.wantText(locales.PickerTitle())
	h.wantText("Trips")
	h.wantText("Family")
	if err := h.tt.Click("Trips"); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText("Japan")
	if err := h.tt.Click(locales.PickerChoose()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if len(chosen) != 1 || chosen[0] != "trips" {
		t.Fatalf("chosen = %q", chosen)
	}
	if h.app.picker.open {
		t.Fatal("picker still open")
	}
	h.app.destinationPath(locales.In("en"), "japan", "")
	h.drain()
	if got := h.app.destinationPath(locales.In("en"), "japan", ""); got != "Amazon Photos"+pathSeparator+"Trips"+pathSeparator+"Japan" {
		t.Fatalf("path = %q", got)
	}
}

func TestPickerShowsLoadFailure(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.openPicker("missing", func(string) {})
	h.drain()
	if st := h.app.folders["missing"]; st == nil || st.state != loadFailed {
		t.Fatalf("state = %+v", st)
	}
	h.wantText(locales.PickerChoose())
}

func openReview(t *testing.T, dir string) *harness {
	t.Helper()
	h := newHarness(t, true)
	h.start()
	h.app.openUploadReview([]string{dir})
	h.drain()
	return h
}

func TestReviewKeepStructureOnlyForFolders(t *testing.T) {
	files := t.TempDir()
	writeJPEGs(t, files, 2)
	h := openReview(t, filepath.Join(files, "photo-0.jpg"))
	h.noText(locales.OptionKeepStructure())

	root := t.TempDir()
	sub := filepath.Join(root, "Trip")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJPEGs(t, sub, 2)
	h = openReview(t, root)
	h.wantText(locales.OptionKeepStructure())
	h.wantText(locales.OptionKeepStructureHint())
	if !h.app.pages.review.plan.KeepFolderStructure {
		t.Fatal("plan does not keep structure by default")
	}
	if err := h.tt.Click(locales.OptionKeepStructure()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.pages.review.keepFolders || h.app.pages.review.plan.KeepFolderStructure {
		t.Fatalf("switch did not turn off: keep=%v plan=%v", h.app.pages.review.keepFolders, h.app.pages.review.plan.KeepFolderStructure)
	}
}

func TestReviewChangeDestinationUsesPicker(t *testing.T) {
	root := t.TempDir()
	writeJPEGs(t, root, 1)
	h := openReview(t, root)
	h.wantText(locales.DestLibrary())
	if err := h.tt.Click(locales.DestChange()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if err := h.tt.Click("Trips"); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if err := h.tt.Click(locales.PickerChoose()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.pages.review.nodeID != "trips" {
		t.Fatalf("nodeID = %q", h.app.pages.review.nodeID)
	}
	h.wantText(locales.DestLibrary() + pathSeparator + "Trips")
}

func TestBackupSplitSelectsRuleAndShowsFiles(t *testing.T) {
	h := openBackup(t)
	first := h.addRule(t.TempDir())
	second := h.addRule(t.TempDir())
	h.wantText(filepath.Base(first.LocalPath))
	h.wantText(filepath.Base(second.LocalPath))
	h.wantText(locales.BackupFilesEmptyTitle())

	if err := h.tt.Click(filepath.Base(second.LocalPath)); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.backup.selected != second.ID {
		t.Fatalf("selected = %q, want %q", h.app.backup.selected, second.ID)
	}

	h.app.files[second.ID] = &filesState{state: loadDone, files: []gui.TrackedFileDTO{{RelPath: "2024/a.jpg", Size: 2048, Status: "confirmed"}}}
	st := h.app.files[second.ID]
	st.tree = buildFileTree(st.files)
	st.outline.Open.Add("2024")
	h.drain()
	h.wantText("a.jpg")
	h.wantText(formatBytes(2048))
	h.noText(locales.BackupFilesEmptyTitle())
}

func TestBuildFileTreeGroupsFoldersFirst(t *testing.T) {
	tree := buildFileTree([]gui.TrackedFileDTO{
		{RelPath: "b.jpg", Size: 1},
		{RelPath: "z/one.jpg", Size: 10},
		{RelPath: "z/deep/two.jpg", Size: 5},
		{RelPath: "a.jpg", Size: 2},
	})
	if got := strings.Join(tree.roots, ","); got != "z,a.jpg,b.jpg" {
		t.Fatalf("roots = %s", got)
	}
	z := tree.nodes["z"]
	if !z.dir || z.size != 15 || z.count != 2 {
		t.Fatalf("z = %+v", z)
	}
	if got := strings.Join(tree.children("z"), ","); got != "z/deep,z/one.jpg" {
		t.Fatalf("children = %s", got)
	}
	if tree.children("a.jpg") != nil {
		t.Fatal("file reported children")
	}
}

func TestBackupDialogKeepsStructureByDefault(t *testing.T) {
	h := openBackup(t)
	dir := t.TempDir()
	h.app.openFolderForm(nil, dir)
	h.drain()
	h.wantText(locales.OptionKeepStructure())
	if !h.app.backup.form.keep {
		t.Fatal("keep structure off by default")
	}
	if err := h.tt.Click(locales.OptionKeepStructure()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.clickDialogConfirm()
	rules, err := h.svc.Backups.List(t.Context())
	if err != nil || len(rules) != 1 {
		t.Fatalf("rules = %v, %v", rules, err)
	}
	if rules[0].KeepFolderStructure {
		t.Fatal("rule kept structure after switching it off")
	}
}

func TestFriendlyErrorMapsRawSDKText(t *testing.T) {
	m := locales.In("en")
	cases := []struct {
		raw, want string
	}{
		{"aphotos not found error on /cdproxy/nodes (HTTP 404): not found", m.ErrorMissing()},
		{"dial tcp: lookup x: no such host", m.ErrorNetwork()},
		{"aphotos rate limit error on /nodes (HTTP 429): slow down", m.ErrorRateLimited()},
	}
	for _, tc := range cases {
		text, detail := friendlyError(m, "", tc.raw)
		if text != tc.want || detail != tc.raw {
			t.Errorf("friendlyError(%q) = %q, %q; want %q", tc.raw, text, detail, tc.want)
		}
	}
	if text, detail := friendlyError(m, "", "disk full"); text != "disk full" || detail != "" {
		t.Errorf("unmapped = %q, %q", text, detail)
	}
}

func TestStepperClampsAtBounds(t *testing.T) {
	value := 2.0
	changed := false
	tt := ui.NewTester(func(c *ui.Context) {
		applyTheme(c)
		ui.Row(c).Padding(20).Children(func() {
			changed = stepper(c, &value, 1, 3, 1, "Uploads") || changed
		})
	}, 300, 100)
	tt.Frame()
	if err := tt.Click(locales.SettingsIncrease() + " Uploads"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if value != 3 || !changed {
		t.Fatalf("value = %v changed = %v", value, changed)
	}
	if err := tt.Click(locales.SettingsIncrease() + " Uploads"); err == nil && value != 3 {
		t.Fatalf("value passed upper bound: %v", value)
	}
}
