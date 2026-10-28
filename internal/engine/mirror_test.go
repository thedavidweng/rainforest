package engine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/engine"
	"github.com/thedavidweng/rainforest/internal/store"
)

type fakeFolder struct{ id, name, parent string }

type fakeUpload struct{ name, parent string }

type fakeDrive struct {
	mu            sync.Mutex
	folders       []fakeFolder
	uploads       []fakeUpload
	creates       int
	createConflic bool
	queryStatus   int
	nextID        int
}

func newFakeDrive() *fakeDrive {
	return &fakeDrive{folders: []fakeFolder{{id: "dest", name: "Dest", parent: "root"}}}
}

func (f *fakeDrive) folderPath(id string) string {
	var parts []string
	for id != "root" && id != "" {
		found := false
		for _, fo := range f.folders {
			if fo.id == id {
				parts = append([]string{fo.name}, parts...)
				id = fo.parent
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return strings.Join(parts, "/")
}

func (f *fakeDrive) uploadedTo() map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]string{}
	for _, u := range f.uploads {
		out[u.name] = f.folderPath(u.parent)
	}
	return out
}

func (f *fakeDrive) createdPaths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.folders))
	for _, fo := range f.folders {
		out = append(out, f.folderPath(fo.id))
	}
	sort.Strings(out)
	return out
}

func parseFilter(raw string) map[string]string {
	out := map[string]string{}
	for clause := range strings.SplitSeq(raw, " AND ") {
		k, v, ok := strings.Cut(clause, ":")
		if !ok {
			continue
		}
		if uq, err := strconv.Unquote(v); err == nil {
			v = uq
		}
		out[k] = v
	}
	return out
}

func folderNode(fo fakeFolder) map[string]any {
	return map[string]any{"id": fo.id, "name": fo.name, "kind": "FOLDER", "status": "AVAILABLE", "parents": []string{fo.parent}}
}

func (f *fakeDrive) transport(t *testing.T) roundTripFunc {
	return func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		path := req.URL.Path
		switch {
		case req.Method == http.MethodGet && path == "/drive/v1/nodes":
			flt := parseFilter(req.URL.Query().Get("filters"))
			if flt["isRoot"] == "true" {
				return jsonResponse(200, map[string]any{"count": 1, "data": []any{map[string]any{"id": "root", "isRoot": true, "kind": "FOLDER", "status": "AVAILABLE"}}}), nil
			}
			if f.queryStatus != 0 {
				return jsonResponse(f.queryStatus, map[string]any{"message": "boom"}), nil
			}
			data := []any{}
			if flt["kind"] == "FOLDER" {
				for _, fo := range f.folders {
					if fo.name == flt["name"] && fo.parent == flt["parents"] {
						data = append(data, folderNode(fo))
					}
				}
			}
			return jsonResponse(200, map[string]any{"count": len(data), "data": data}), nil
		case req.Method == http.MethodPost && path == "/drive/v1/nodes":
			var body struct {
				Name    string   `json:"name"`
				Parents []string `json:"parents"`
			}
			raw, _ := io.ReadAll(req.Body)
			if err := jsonUnmarshal(raw, &body); err != nil {
				t.Errorf("create folder body: %v", err)
			}
			f.creates++
			f.nextID++
			fo := fakeFolder{id: "new-" + strconv.Itoa(f.nextID), name: body.Name, parent: body.Parents[0]}
			f.folders = append(f.folders, fo)
			if f.createConflic {
				f.createConflic = false
				return jsonResponse(http.StatusConflict, map[string]any{"message": "exists", "nodeId": fo.id}), nil
			}
			return jsonResponse(http.StatusCreated, folderNode(fo)), nil
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/drive/v1/nodes/") && strings.HasSuffix(path, "/children"):
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/drive/v1/nodes/"), "/children")
			limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
			var all []any
			for _, fo := range f.folders {
				if fo.parent == id {
					all = append(all, folderNode(fo))
				}
			}
			end := min(offset+limit, len(all))
			page := []any{}
			if offset < end {
				page = all[offset:end]
			}
			return jsonResponse(200, map[string]any{"count": len(all), "data": page}), nil
		case req.Method == http.MethodGet && strings.HasPrefix(path, "/drive/v1/nodes/"):
			id := strings.TrimPrefix(path, "/drive/v1/nodes/")
			if id == "root" {
				return jsonResponse(200, map[string]any{"id": "root", "isRoot": true, "kind": "FOLDER", "status": "AVAILABLE"}), nil
			}
			for _, fo := range f.folders {
				if fo.id == id {
					return jsonResponse(200, folderNode(fo)), nil
				}
			}
			return jsonResponse(404, map[string]any{"message": "not found"}), nil
		case req.Method == http.MethodPost && strings.Contains(path, "/cdproxy/nodes"):
			_, _ = io.Copy(io.Discard, req.Body)
			name := req.URL.Query().Get("name")
			f.uploads = append(f.uploads, fakeUpload{name: name, parent: req.URL.Query().Get("parentNodeId")})
			return jsonResponse(201, map[string]any{"id": "file-" + name, "name": name, "kind": "FILE"}), nil
		}
		return jsonResponse(404, map[string]any{"message": "unhandled " + req.Method + " " + path}), nil
	}
}

func TestUploadMirrorsLocalFolderStructureAndReusesRemoteFolders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	drive := newFakeDrive()
	drive.folders = append(drive.folders, fakeFolder{id: "trips", name: "Trips", parent: "dest"})
	eng, st := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))

	src := t.TempDir()
	trips := filepath.Join(src, "Trips")
	writePNGFile(t, filepath.Join(trips, "a.png"), 8, 8)
	writePNGFile(t, filepath.Join(trips, "sub", "b.png"), 9, 8)
	writePNGFile(t, filepath.Join(trips, "sub", "deep", "c.png"), 10, 8)
	writePNGFile(t, filepath.Join(trips, "sub", "d.png"), 11, 8)
	loose := filepath.Join(src, "loose.png")
	writePNGFile(t, loose, 12, 8)

	tr, err := eng.StartUpload(ctx, engine.UploadJobRequest{Paths: []string{trips, loose}, DestinationNodeID: "dest"})
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}
	if final := waitForTransferTerminal(t, eng, tr.ID); final.Stage != store.StageCompleted {
		t.Fatalf("stage = %s (%s)", final.Stage, final.ErrorMessage)
	}

	want := map[string]string{"a.png": "Dest/Trips", "b.png": "Dest/Trips/sub", "d.png": "Dest/Trips/sub", "c.png": "Dest/Trips/sub/deep", "loose.png": "Dest"}
	got := drive.uploadedTo()
	for name, dir := range want {
		if got[name] != dir {
			t.Errorf("%s uploaded to %q, want %q", name, got[name], dir)
		}
	}
	if drive.creates != 2 {
		t.Errorf("CreateFolder calls = %d, want 2 (sub, deep); folders=%v", drive.creates, drive.createdPaths())
	}
	items, _ := st.ListTransferItems(ctx, tr.ID)
	dirs := map[string]string{}
	for _, it := range items {
		dirs[it.Name] = it.RemoteDir
	}
	if dirs["c.png"] != "Trips/sub/deep" || dirs["loose.png"] != "" || dirs["a.png"] != "Trips" {
		t.Errorf("persisted RemoteDir = %v", dirs)
	}
}

func TestUploadCreateConflictRequeriesFolder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	drive := newFakeDrive()
	drive.createConflic = true
	eng, _ := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))

	src := filepath.Join(t.TempDir(), "Album")
	writePNGFile(t, filepath.Join(src, "x.png"), 8, 8)
	tr, err := eng.StartUpload(ctx, engine.UploadJobRequest{Paths: []string{src}, DestinationNodeID: "dest"})
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}
	if final := waitForTransferTerminal(t, eng, tr.ID); final.Stage != store.StageCompleted {
		t.Fatalf("stage = %s (%s)", final.Stage, final.ErrorMessage)
	}
	if drive.creates != 1 || drive.uploadedTo()["x.png"] != "Dest/Album" {
		t.Errorf("creates=%d uploads=%v", drive.creates, drive.uploadedTo())
	}
}

func TestUploadKeepFolderStructureDisabledIsFlat(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	drive := newFakeDrive()
	eng, _ := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))

	src := filepath.Join(t.TempDir(), "Trips")
	writePNGFile(t, filepath.Join(src, "sub", "x.png"), 8, 8)
	off := false
	tr, err := eng.StartUpload(ctx, engine.UploadJobRequest{Paths: []string{src}, DestinationNodeID: "dest", KeepFolderStructure: &off})
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}
	waitForTransferTerminal(t, eng, tr.ID)
	if drive.creates != 0 || drive.uploadedTo()["x.png"] != "Dest" {
		t.Errorf("creates=%d uploads=%v", drive.creates, drive.uploadedTo())
	}
}

func TestPlanUploadStructureDefaultsAndAlbumDestination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	drive := newFakeDrive()
	eng, _ := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))

	src := filepath.Join(t.TempDir(), "Trips")
	file := filepath.Join(src, "sub", "x.png")
	writePNGFile(t, file, 8, 8)

	plan, err := eng.PlanUpload(ctx, []string{src}, engine.PlanOptions{DestinationNodeID: "dest"})
	if err != nil || !plan.KeepFolderStructure || !plan.Destination.StructureSupported || plan.Items[0].RemoteDir != "Trips/sub" {
		t.Fatalf("directory plan: err=%v plan=%+v", err, plan)
	}
	plan, err = eng.PlanUpload(ctx, []string{file}, engine.PlanOptions{DestinationNodeID: "dest"})
	if err != nil || plan.KeepFolderStructure || plan.Items[0].RemoteDir != "" {
		t.Fatalf("file-only plan: err=%v plan=%+v", err, plan)
	}

	albumClient := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/drive/v1/nodes" && strings.Contains(req.URL.Query().Get("filters"), "isRoot:true"):
			return jsonResponse(200, map[string]any{"count": 1, "data": []any{map[string]any{"id": "root", "kind": "FOLDER", "status": "AVAILABLE"}}}), nil
		case req.URL.Path == "/drive/v1/nodes/album-1":
			return jsonResponse(200, map[string]any{"id": "album-1", "name": "Album", "kind": "VISUAL_COLLECTION", "status": "AVAILABLE"}), nil
		}
		return jsonResponse(404, nil), nil
	}))
	albumEng, _ := newTestEngine(t, albumClient)
	plan, err = albumEng.PlanUpload(ctx, []string{src}, engine.PlanOptions{DestinationNodeID: "album-1"})
	if err != nil {
		t.Fatalf("album plan: %v", err)
	}
	if plan.KeepFolderStructure || plan.Destination.StructureSupported || plan.Items[0].RemoteDir != "" {
		t.Errorf("album plan must be flat: %+v", plan)
	}
}

func TestBackupMirrorsSubdirectoriesUnderDestination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	drive := newFakeDrive()
	eng, st := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))

	src := filepath.Join(t.TempDir(), "camera")
	writePNGFile(t, filepath.Join(src, "top.png"), 8, 8)
	writePNGFile(t, filepath.Join(src, "2026", "jan", "x.png"), 9, 8)

	rule, err := eng.AddBackupRule(ctx, engine.BackupRuleInput{LocalPath: src, DestinationNodeID: "dest", Paused: true, ConflictPolicy: aphotos.ConflictSkip})
	if err != nil {
		t.Fatalf("AddBackupRule: %v", err)
	}
	if !rule.KeepFolderStructure {
		t.Fatal("rules keep folder structure by default")
	}
	if _, err := eng.ScanNow(ctx, rule.ID); err != nil {
		t.Fatalf("ScanNow: %v", err)
	}
	got := drive.uploadedTo()
	if got["top.png"] != "Dest" || got["x.png"] != "Dest/2026/jan" {
		t.Errorf("uploads = %v", got)
	}
	if tf, err := st.GetTrackedFile(ctx, rule.ID, "2026/jan/x.png"); err != nil || tf.Status != store.ItemStatusConfirmed {
		t.Errorf("tracked = %+v, %v", tf, err)
	}

	flatDrive := newFakeDrive()
	flatEng, _ := newTestEngine(t, newTestClient(t, aphotos.RegionUS, flatDrive.transport(t)))
	off := false
	flat, err := flatEng.AddBackupRule(ctx, engine.BackupRuleInput{LocalPath: src, DestinationNodeID: "dest", Paused: true, KeepFolderStructure: &off})
	if err != nil {
		t.Fatalf("AddBackupRule flat: %v", err)
	}
	if _, err := flatEng.ScanNow(ctx, flat.ID); err != nil {
		t.Fatalf("ScanNow flat: %v", err)
	}
	if flatDrive.creates != 0 || flatDrive.uploadedTo()["x.png"] != "Dest" {
		t.Errorf("flat rule creates=%d uploads=%v", flatDrive.creates, flatDrive.uploadedTo())
	}
}

func TestReconcileKeepsItemUnconfirmedWhenMirroredParentUnresolvable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "p.png")
	size, md5Hex := writePNGFile(t, file, 8, 8)

	run := func(t *testing.T, drive *fakeDrive) store.TransferItem {
		eng, st := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))
		now := time.Now().UTC()
		_, err := st.CreateTransfer(ctx, store.Transfer{ID: "tr", Kind: store.TransferKindUpload, Stage: store.StageRunning, DestNodeID: "dest", CreatedAt: now, UpdatedAt: now}, []store.TransferItem{
			{ID: "it", LocalPath: file, Name: "p.png", MediaType: store.MediaTypePhoto, Size: size, MD5: md5Hex, Status: store.ItemStatusRunning, RemoteDir: "Trips/sub", UpdatedAt: now},
		})
		if err != nil {
			t.Fatalf("CreateTransfer: %v", err)
		}
		_ = eng.ReconcileOnStartup(ctx)
		it, err := st.GetTransferItem(ctx, "it")
		if err != nil {
			t.Fatalf("GetTransferItem: %v", err)
		}
		if len(drive.uploads) != 0 || drive.creates != 0 {
			t.Errorf("reconcile must not upload or create: uploads=%v creates=%d", drive.uploads, drive.creates)
		}
		return it
	}

	t.Run("folder lookup fails", func(t *testing.T) {
		t.Parallel()
		d := newFakeDrive()
		d.queryStatus = http.StatusInternalServerError
		if it := run(t, d); it.Status != store.ItemStatusUnconfirmed {
			t.Errorf("status = %s, want unconfirmed", it.Status)
		}
	})
	t.Run("mirrored folder missing", func(t *testing.T) {
		t.Parallel()
		if it := run(t, newFakeDrive()); it.Status != store.ItemStatusUnconfirmed {
			t.Errorf("status = %s, want unconfirmed", it.Status)
		}
	})
}

func TestListFoldersBreadcrumbsAndTruncation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	drive := newFakeDrive()
	drive.folders = append(drive.folders, fakeFolder{id: "trips", name: "Trips", parent: "dest"})
	for i := range 1005 {
		drive.folders = append(drive.folders, fakeFolder{id: "bulk-" + strconv.Itoa(i), name: "B" + strconv.Itoa(i), parent: "root"})
	}
	eng, _ := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))

	l, err := eng.ListFolders(ctx, "trips")
	if err != nil {
		t.Fatalf("ListFolders: %v", err)
	}
	if l.Folder.Name != "Trips" || len(l.Path) != 3 || l.Path[0].Name != "" || l.Path[1].Name != "Dest" || l.Path[2].ID != "trips" || len(l.Children) != 0 || l.Truncated {
		t.Errorf("listing = %+v", l)
	}

	root, err := eng.ListFolders(ctx, "")
	if err != nil {
		t.Fatalf("ListFolders root: %v", err)
	}
	if root.Folder.ID != "root" || root.Folder.Name != "" || len(root.Path) != 1 || len(root.Children) != 1000 || !root.Truncated {
		t.Errorf("root listing: folder=%+v path=%v children=%d truncated=%v", root.Folder, root.Path, len(root.Children), root.Truncated)
	}
}

func TestLocalThumbnailHonorsEXIFOrientationAndRecentUploads(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	eng, st := newTestEngine(t, nil)

	img := image.NewRGBA(image.Rect(0, 0, 800, 400))
	for y := range 400 {
		for x := range 800 {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: 90, B: 200, A: 255})
		}
	}
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, nil); err != nil {
		t.Fatal(err)
	}
	exif := []byte{0xFF, 0xE1, 0x00, 0x22, 'E', 'x', 'i', 'f', 0, 0, 'M', 'M', 0, 0x2A, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0}
	raw := append(append([]byte{0xFF, 0xD8}, exif...), enc.Bytes()[2:]...)
	path := filepath.Join(t.TempDir(), "rot.jpg")
	writeRawFile(t, path, raw)

	th, err := eng.LocalThumbnail(path)
	if err != nil {
		t.Fatalf("LocalThumbnail: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(th.Data))
	if err != nil || cfg.Width != 160 || cfg.Height != 320 {
		t.Errorf("thumbnail dims = %dx%d (%v), want 160x320", cfg.Width, cfg.Height, err)
	}

	now := time.Now().UTC()
	items := make([]store.TransferItem, 0, 4)
	for i, n := range []string{"one", "two", "dup"} {
		items = append(items, store.TransferItem{
			ID: "i" + n, LocalPath: path, Name: n + ".jpg", MediaType: store.MediaTypePhoto, Size: 1, RemoteNodeID: map[string]string{"one": "n1", "two": "n2", "dup": "n1"}[n],
			Status: store.ItemStatusConfirmed, UpdatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}
	items = append(items, store.TransferItem{ID: "gone", LocalPath: filepath.Join(t.TempDir(), "missing.jpg"), Name: "gone.jpg", MediaType: store.MediaTypePhoto, RemoteNodeID: "n3", Status: store.ItemStatusConfirmed, UpdatedAt: now.Add(-time.Hour)})
	if _, err := st.CreateTransfer(ctx, store.Transfer{ID: "tr", Kind: store.TransferKindUpload, Stage: store.StageCompleted, CreatedAt: now, UpdatedAt: now}, items); err != nil {
		t.Fatal(err)
	}

	recent, err := eng.RecentUploads(ctx, 100)
	if err != nil {
		t.Fatalf("RecentUploads: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("recent = %d entries, want 3 (deduped by remote node)", len(recent))
	}
	if recent[0].Item.RemoteNodeID != "n1" || len(recent[0].Thumbnail) == 0 || len(recent[2].Thumbnail) != 0 {
		t.Errorf("unexpected recent ordering/thumbnails: %+v", recent)
	}
	if at, err := eng.LastUploadAt(ctx); err != nil || at == nil || !at.Equal(now.Add(2*time.Second)) {
		t.Errorf("LastUploadAt = %v, %v", at, err)
	}
}

func jsonUnmarshal(raw []byte, v any) error { return json.Unmarshal(raw, v) }

func TestRetryKeepsMissingMirroredParentUnconfirmed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	drive := newFakeDrive()
	eng, st := newTestEngine(t, newTestClient(t, aphotos.RegionUS, drive.transport(t)))
	file := filepath.Join(t.TempDir(), "p.png")
	size, hash := writePNGFile(t, file, 8, 8)
	now := time.Now().UTC()
	_, err := st.CreateTransfer(ctx, store.Transfer{ID: "retry-mirror", Kind: store.TransferKindUpload, Stage: store.StageUnconfirmed, DestNodeID: "dest", CreatedAt: now, UpdatedAt: now}, []store.TransferItem{
		{ID: "retry-item", LocalPath: file, Name: "p.png", Size: size, MD5: hash, Status: store.ItemStatusUnconfirmed, RemoteDir: "Trips/sub", UpdatedAt: now},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.RetryTransfer(ctx, "retry-mirror"); err != nil {
		t.Fatal(err)
	}
	waitForTransferTerminal(t, eng, "retry-mirror")
	item, err := st.GetTransferItem(ctx, "retry-item")
	if err != nil {
		t.Fatal(err)
	}
	drive.mu.Lock()
	defer drive.mu.Unlock()
	if item.Status != store.ItemStatusUnconfirmed || drive.creates != 0 || len(drive.uploads) != 0 {
		t.Fatalf("retry must preserve uncertainty without remote writes: item=%+v creates=%d uploads=%v", item, drive.creates, drive.uploads)
	}
}
