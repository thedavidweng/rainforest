package engine_test

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/engine"
	"github.com/thedavidweng/rainforest/internal/store"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(b)),
	}
}

func newTestClient(t *testing.T, region aphotos.Region, rt http.RoundTripper) *aphotos.Client {
	t.Helper()
	rc, err := aphotos.ResolveRegion(region)
	if err != nil {
		t.Fatalf("ResolveRegion(%q): %v", region, err)
	}
	cookies := map[string]string{
		rc.RequiredCookies[0]: "140-test-session",
		rc.RequiredCookies[1]: "130-test-ubid",
		rc.RequiredCookies[2]: "Atza|TestToken",
	}
	client, err := aphotos.NewClient(aphotos.Options{
		Region:     region,
		Cookies:    cookies,
		HTTPClient: &http.Client{Transport: rt},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func newTestEngine(t *testing.T, client *aphotos.Client) (*engine.Engine, *store.Store) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cs := credstore.NewFileStore(filepath.Join(dir, "credentials.json"))
	eng := engine.New(engine.Config{
		Store:     st,
		CredStore: cs,
		Client:    client,
	})
	t.Cleanup(func() { _ = eng.Close() })
	return eng, st
}

func writePNGFile(t *testing.T, path string, w, h int) (size int64, md5Hex string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 20, G: 120, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	data := buf.Bytes()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	sum := md5.Sum(data)
	return int64(len(data)), hex.EncodeToString(sum[:])
}

func writeRawFile(t *testing.T, path string, content []byte) int64 {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
	return int64(len(content))
}

func waitForTransferTerminal(t *testing.T, eng *engine.Engine, transferID string) store.Transfer {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tr, err := eng.GetTransfer(context.Background(), transferID)
		if err == nil && store.IsTerminalStage(tr.Stage) {
			return tr
		}
		time.Sleep(15 * time.Millisecond)
	}
	tr, _ := eng.GetTransfer(context.Background(), transferID)
	t.Fatalf("timed out waiting for transfer %s to finish; current stage=%s", transferID, tr.Stage)
	return store.Transfer{}
}

func TestPlanUploadCountsMediaPreviewAndDestinationCapability(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	client := newTestClient(t, aphotos.RegionCA, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("PlanUpload must not perform network I/O, got %s %s", req.Method, req.URL)
		return nil, nil
	}))
	eng, _ := newTestEngine(t, client)

	srcDir := filepath.Join(t.TempDir(), "drop")
	photoPath := filepath.Join(srcDir, "sub", "sunset.png")
	photoSize, _ := writePNGFile(t, photoPath, 48, 32)
	videoPath := filepath.Join(srcDir, "clip.mp4")
	videoSize := writeRawFile(t, videoPath, []byte("fake-mp4-video-stream-bytes"))
	txtPath := filepath.Join(srcDir, "notes.txt")
	writeRawFile(t, txtPath, []byte("ignored text document"))

	plan, err := eng.PlanUpload(ctx, []string{srcDir}, engine.PlanOptions{
		ConflictPolicy:  aphotos.ConflictSkip,
		IncludePreviews: true,
	})
	if err != nil {
		t.Fatalf("PlanUpload: %v", err)
	}
	if plan.PhotoCount != 1 || plan.VideoCount != 1 || plan.TotalCount != 2 {
		t.Fatalf("counts = (photo=%d, video=%d, total=%d), want (1, 1, 2)", plan.PhotoCount, plan.VideoCount, plan.TotalCount)
	}
	if plan.TotalBytes != photoSize+videoSize {
		t.Errorf("TotalBytes = %d, want %d", plan.TotalBytes, photoSize+videoSize)
	}
	if plan.SkippedUnsupported != 1 {
		t.Errorf("SkippedUnsupported = %d, want 1", plan.SkippedUnsupported)
	}
	if !plan.Destination.RootOnly || plan.Destination.Evidence != aphotos.EvidenceExperimental {
		t.Errorf("Destination = %+v, want RootOnly=true and Evidence=experimental for CA", plan.Destination)
	}

	var foundPhotoPreview bool
	for _, it := range plan.Items {
		if it.Name == "sunset.png" {
			foundPhotoPreview = true
			if it.Width != 48 || it.Height != 32 {
				t.Errorf("sunset.png dimensions = %dx%d, want 48x32", it.Width, it.Height)
			}
			if !strings.HasPrefix(it.ThumbnailDataURL, "data:image/png;base64,") {
				t.Errorf("expected local data URL thumbnail on sunset.png, got %q", it.ThumbnailDataURL)
			}
		}
	}
	if !foundPhotoPreview {
		t.Error("expected sunset.png in plan items")
	}
}

func TestStartUploadPauseResumeCancelAndRetryWithoutDuplicatePost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	dir := t.TempDir()
	p1 := filepath.Join(dir, "img1.png")
	_, md51 := writePNGFile(t, p1, 16, 16)
	p2 := filepath.Join(dir, "img2.png")
	size2, md52 := writePNGFile(t, p2, 16, 16)
	var uploadCalls atomic.Int32
	blockSecond := make(chan struct{})
	var closeOnce sync.Once
	unblockSecond := func() { closeOnce.Do(func() { close(blockSecond) }) }
	defer unblockSecond()

	client := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes" {
			f := req.URL.Query().Get("filters")
			if strings.Contains(f, "isRoot:true") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "root-us", "ownerId": "owner-1", "kind": "FOLDER"}},
				}), nil
			}
			if strings.Contains(f, md52) {
				// Pretend img2 already landed remotely so RetryTransfer promotes it without re-POSTing!
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data": []map[string]any{{
						"id":   "node-img2-existing",
						"name": "img2.png",
						"kind": "FILE",
						"contentProperties": map[string]any{
							"md5":  md52,
							"size": size2,
						},
					}},
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil
		}
		if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/cdproxy/nodes") {
			uploadCalls.Add(1)
			name := req.URL.Query().Get("name")
			_, _ = io.Copy(io.Discard, req.Body)
			if name == "img1.png" {
				return jsonResponse(http.StatusCreated, map[string]any{
					"id":   "node-img1",
					"name": "img1.png",
					"kind": "FILE",
					"contentProperties": map[string]any{
						"md5": md51,
					},
				}), nil
			}
			if name == "img2.png" {
				<-blockSecond
				// Return 201 without node ID -> SDK marks TransferUnconfirmed
				return jsonResponse(http.StatusCreated, map[string]any{
					"name": "img2.png",
				}), nil
			}
		}
		return jsonResponse(http.StatusInternalServerError, map[string]any{"message": "unexpected"}), nil
	}))

	eng, st := newTestEngine(t, client)
	// Force single upload worker so items execute sequentially
	cfg, _ := st.GetSettings(ctx)
	cfg.MaxUploadConcurrency = 1
	_ = st.UpdateSettings(ctx, cfg)

	tr, err := eng.StartUpload(ctx, engine.UploadJobRequest{
		Paths:          []string{p1, p2},
		ConflictPolicy: aphotos.ConflictSkip,
	})
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}

	// Pause and resume while second item is blocked
	if err := eng.PauseTransfer(ctx, tr.ID); err != nil {
		t.Fatalf("PauseTransfer: %v", err)
	}
	pausedTr, err := eng.GetTransfer(ctx, tr.ID)
	if err != nil || pausedTr.Stage != store.StagePaused {
		t.Fatalf("expected stage paused, got %+v (err=%v)", pausedTr, err)
	}
	if err := eng.ResumeTransfer(ctx, tr.ID); err != nil {
		t.Fatalf("ResumeTransfer: %v", err)
	}
	unblockSecond()

	doneTr := waitForTransferTerminal(t, eng, tr.ID)
	if doneTr.Stage != store.StageUnconfirmed {
		t.Fatalf("expected stage unconfirmed when item 2 returned no ID, got %s", doneTr.Stage)
	}
	if doneTr.ItemsDone != 1 || doneTr.ItemsUnconfirmed != 1 {
		t.Fatalf("unexpected counts: done=%d unconfirmed=%d", doneTr.ItemsDone, doneTr.ItemsUnconfirmed)
	}

	// RetryTransfer should check remote MD5 for img2, find node-img2-existing, and confirm without another POST
	postsBeforeRetry := uploadCalls.Load()
	retried, err := eng.RetryTransfer(ctx, tr.ID)
	if err != nil {
		t.Fatalf("RetryTransfer: %v", err)
	}
	finalTr := waitForTransferTerminal(t, eng, retried.ID)
	if finalTr.Stage != store.StageCompleted {
		t.Fatalf("expected stage completed after retry, got %s (err=%s)", finalTr.Stage, finalTr.ErrorMessage)
	}
	if uploadCalls.Load() != postsBeforeRetry {
		t.Fatalf("expected RetryTransfer to reconcile existing remote MD5 without re-POSTing; posts before=%d after=%d", postsBeforeRetry, uploadCalls.Load())
	}
}

func TestReconcileOnStartupPromotesMatchedAndMarksInterruptedUnconfirmed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	matchedMD5 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	missingMD5 := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	client := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes" {
			f := req.URL.Query().Get("filters")
			if strings.Contains(f, matchedMD5) {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data": []map[string]any{{
						"id":   "remote-reconciled-1",
						"name": "crash1.jpg",
						"kind": "FILE",
						"contentProperties": map[string]any{
							"md5":  matchedMD5,
							"size": 500,
						},
					}},
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil
		}
		t.Fatalf("unexpected request during startup reconciliation: %s %s", req.Method, req.URL)
		return nil, nil
	}))

	eng, st := newTestEngine(t, client)
	now := time.Now().UTC()
	_, err := st.CreateTransfer(ctx, store.Transfer{
		ID:         "tr-crash",
		Kind:       store.TransferKindUpload,
		Stage:      store.StageRunning,
		BytesTotal: 1000,
		ItemsTotal: 2,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, []store.TransferItem{
		{
			ID:         "it-crash-1",
			TransferID: "tr-crash",
			LocalPath:  "/tmp/crash1.jpg",
			Name:       "crash1.jpg",
			MediaType:  store.MediaTypePhoto,
			Size:       500,
			MD5:        matchedMD5,
			Status:     store.ItemStatusRunning,
			UpdatedAt:  now,
		},
		{
			ID:         "it-crash-2",
			TransferID: "tr-crash",
			LocalPath:  "/tmp/crash2.jpg",
			Name:       "crash2.jpg",
			MediaType:  store.MediaTypePhoto,
			Size:       500,
			MD5:        missingMD5,
			Status:     store.ItemStatusRunning,
			UpdatedAt:  now,
		},
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	if err := eng.ReconcileOnStartup(ctx); err != nil {
		t.Fatalf("ReconcileOnStartup: %v", err)
	}

	items, err := st.ListTransferItems(ctx, "tr-crash")
	if err != nil {
		t.Fatalf("ListTransferItems: %v", err)
	}
	for _, it := range items {
		switch it.ID {
		case "it-crash-1":
			if it.Status != store.ItemStatusConfirmed || it.RemoteNodeID != "remote-reconciled-1" {
				t.Errorf("item 1 not promoted to confirmed: %+v", it)
			}
		case "it-crash-2":
			if it.Status != store.ItemStatusUnconfirmed {
				t.Errorf("item 2 status = %s, want unconfirmed", it.Status)
			}
		}
	}

	tr, err := st.GetTransfer(ctx, "tr-crash")
	if err != nil {
		t.Fatalf("GetTransfer: %v", err)
	}
	if tr.Stage != store.StageUnconfirmed || tr.ItemsDone != 1 || tr.ItemsUnconfirmed != 1 {
		t.Errorf("unexpected reconciled transfer: %+v", tr)
	}
}

func TestOneWayBackupIncrementalMediaFilterMidWriteAndNoRemoteDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var uploadedNames []string
	var uploadMu sync.Mutex
	var trashCalled atomic.Bool
	var mutateDuringUpload func(name string)

	client := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/trash") || strings.Contains(req.URL.Path, "/purge") || req.Method == http.MethodDelete {
			trashCalled.Store(true)
			t.Errorf("PROHIBITED destructive call made: %s %s", req.Method, req.URL.Path)
			return jsonResponse(http.StatusForbidden, map[string]any{"message": "forbidden delete"}), nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes" {
			if strings.Contains(req.URL.Query().Get("filters"), "isRoot:true") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "root-backup", "ownerId": "owner-1", "kind": "FOLDER"}},
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil
		}
		if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/cdproxy/nodes") {
			name := req.URL.Query().Get("name")
			_, _ = io.Copy(io.Discard, req.Body)
			if mutateDuringUpload != nil {
				mutateDuringUpload(name)
			}
			uploadMu.Lock()
			uploadedNames = append(uploadedNames, name)
			uploadMu.Unlock()
			return jsonResponse(http.StatusCreated, map[string]any{
				"id":   "node-" + name,
				"name": name,
				"kind": "FILE",
			}), nil
		}
		return jsonResponse(http.StatusNotFound, nil), nil
	}))

	eng, st := newTestEngine(t, client)
	backupDir := filepath.Join(t.TempDir(), "camera-roll")
	photo1 := filepath.Join(backupDir, "p1.png")
	writePNGFile(t, photo1, 10, 10)
	video1 := filepath.Join(backupDir, "v1.mp4")
	writeRawFile(t, video1, []byte("video-bytes-1"))

	// Add rule with IncludeVideos=false (photos only) and ScheduleImmediate
	rule, err := eng.AddBackupRule(ctx, engine.BackupRuleInput{
		LocalPath:      backupDir,
		IncludeVideos:  false,
		ScheduleMode:   store.ScheduleImmediate,
		ConflictPolicy: aphotos.ConflictSkip,
	})
	if err != nil {
		t.Fatalf("AddBackupRule: %v", err)
	}

	// Wait for immediate scan to complete p1.png
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tf, err := st.GetTrackedFile(ctx, rule.ID, "p1.png")
		if err == nil && tf.Status == store.ItemStatusConfirmed {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	uploadMu.Lock()
	if len(uploadedNames) != 1 || uploadedNames[0] != "p1.png" {
		t.Fatalf("uploadedNames = %v, want [p1.png] (photos only)", uploadedNames)
	}
	uploadMu.Unlock()

	// Second scan with no changes should upload nothing
	if _, err := eng.ScanNow(ctx, rule.ID); err != nil {
		t.Fatalf("ScanNow unchanged: %v", err)
	}
	uploadMu.Lock()
	if len(uploadedNames) != 1 {
		t.Fatalf("expected 0 new uploads on unchanged scan, got %v", uploadedNames)
	}
	uploadMu.Unlock()

	// Enable IncludeVideos=true -> v1.mp4 should now be uploaded
	rule, err = eng.UpdateBackupRule(ctx, rule.ID, engine.BackupRuleInput{
		LocalPath:       backupDir,
		IncludeVideos:   true,
		ScheduleMode:    store.ScheduleInterval,
		IntervalMinutes: 15,
		ConflictPolicy:  aphotos.ConflictSkip,
	})
	if err != nil {
		t.Fatalf("UpdateBackupRule: %v", err)
	}
	for time.Now().Before(deadline) {
		tf, err := st.GetTrackedFile(ctx, rule.ID, "v1.mp4")
		if err == nil && tf.Status == store.ItemStatusConfirmed {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	// Mid-write changing file: create changing.png that mutates during upload stream
	changingPath := filepath.Join(backupDir, "changing.png")
	writePNGFile(t, changingPath, 10, 10)
	mutateDuringUpload = func(name string) {
		if name == "changing.png" {
			_ = os.WriteFile(changingPath, []byte("still-writing-more-bytes-mid-flight"), 0o644)
		}
	}
	midTr, err := eng.ScanNow(ctx, rule.ID)
	if err != nil {
		t.Fatalf("ScanNow mid-write: %v", err)
	}
	mutateDuringUpload = nil
	if midTr == nil || midTr.Stage != store.StageUnconfirmed {
		t.Fatalf("expected mid-write transfer to end in StageUnconfirmed, got %+v", midTr)
	}
	tfChanging, err := st.GetTrackedFile(ctx, rule.ID, "changing.png")
	if err != nil || tfChanging.Status != store.ItemStatusUnconfirmed {
		t.Fatalf("mid-write changing file must be tracked as unconfirmed (not skipped or confirmed), got %+v (err=%v)", tfChanging, err)
	}

	// Subsequent scan while changing.png is unconfirmed in an open transfer must not auto-re-POST it
	postsBeforeRescan := len(uploadedNames)
	_, _ = eng.ScanNow(ctx, rule.ID)
	if len(uploadedNames) != postsBeforeRescan {
		t.Fatalf("expected unconfirmed file not to be re-POSTed on periodic scan, got uploads=%v", uploadedNames)
	}

	// Remove changing.png and photo1 -> MUST NEVER call trash/delete
	_ = os.Remove(changingPath)
	if err := os.Remove(photo1); err != nil {
		t.Fatalf("Remove photo1: %v", err)
	}
	if _, err := eng.ScanNow(ctx, rule.ID); err != nil {
		t.Fatalf("ScanNow after local file delete: %v", err)
	}
	if trashCalled.Load() {
		t.Fatal("local file deletion triggered a remote delete/trash call")
	}

	if err := os.RemoveAll(backupDir); err != nil {
		t.Fatalf("RemoveAll backupDir: %v", err)
	}
	_, _ = eng.ScanNow(ctx, rule.ID)
	unavailableRule, err := st.GetBackupRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetBackupRule: %v", err)
	}
	if unavailableRule.Status != store.BackupStatusSourceUnavailable {
		t.Fatalf("expected rule status source_unavailable when directory is missing, got %s", unavailableRule.Status)
	}

	if err := eng.RemoveBackupRule(ctx, rule.ID); err != nil {
		t.Fatalf("RemoveBackupRule: %v", err)
	}
	if trashCalled.Load() {
		t.Fatal("removing backup rule triggered a remote delete/trash call")
	}
}

func TestRecoveryDownloadVerifiedAndErrorReporting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fileBytes := []byte("original-raw-photo-bytes-for-recovery")
	sum := md5.Sum(fileBytes)
	expectedMD5 := hex.EncodeToString(sum[:])

	client := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes":
			f := req.URL.Query().Get("filters")
			if strings.Contains(f, "isRoot:true") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "root-dl", "ownerId": "owner-dl", "kind": "FOLDER"}},
				}), nil
			}
			if strings.Contains(f, "VISUAL_COLLECTION") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "album-1", "name": "Trip", "kind": "VISUAL_COLLECTION"}},
				}), nil
			}
			if strings.Contains(f, "kind:FOLDER") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "folder-1", "name": "2026", "kind": "FOLDER"}},
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil

		case req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes/album-1/children":
			return jsonResponse(http.StatusOK, map[string]any{
				"count": 2,
				"data": []map[string]any{
					{
						"id":   "node-ok",
						"name": "recovered.jpg",
						"kind": "FILE",
						"contentProperties": map[string]any{
							"size": len(fileBytes),
							"md5":  expectedMD5,
						},
					},
					{
						"id":   "node-bad",
						"name": "corrupt.jpg",
						"kind": "FILE",
						"contentProperties": map[string]any{
							"size": len(fileBytes),
							"md5":  "00000000000000000000000000000000",
						},
					},
				},
			}), nil

		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/contentRedirection"):
			h := http.Header{
				"Content-Type":        []string{"image/jpeg"},
				"Content-Disposition": []string{`attachment; filename="recovered.jpg"`},
			}
			return &http.Response{
				StatusCode:    http.StatusOK,
				Header:        h,
				ContentLength: int64(len(fileBytes)),
				Body:          io.NopCloser(bytes.NewReader(fileBytes)),
			}, nil
		}
		return jsonResponse(http.StatusNotFound, nil), nil
	}))

	eng, _ := newTestEngine(t, client)
	sources, err := eng.ListRecoverySources(ctx)
	if err != nil {
		t.Fatalf("ListRecoverySources: %v", err)
	}
	if sources.Evidence != aphotos.EvidenceVerified || len(sources.Albums) != 1 || len(sources.Folders) != 1 {
		t.Fatalf("unexpected recovery sources: %+v", sources)
	}

	destDir := filepath.Join(t.TempDir(), "restored")
	dlTr, err := eng.StartDownload(ctx, engine.DownloadJobRequest{
		ContainerNodeID: "album-1",
		DestDir:         destDir,
	})
	if err != nil {
		t.Fatalf("StartDownload: %v", err)
	}

	finalTr := waitForTransferTerminal(t, eng, dlTr.ID)
	if finalTr.Stage != store.StagePartial {
		t.Fatalf("expected StagePartial (1 confirmed, 1 failed checksum), got %s", finalTr.Stage)
	}
	if finalTr.ItemsDone != 1 || finalTr.ItemsFailed != 1 {
		t.Fatalf("expected 1 done and 1 failed, got done=%d failed=%d", finalTr.ItemsDone, finalTr.ItemsFailed)
	}

	restoredBytes, err := os.ReadFile(filepath.Join(destDir, "recovered.jpg"))
	if err != nil {
		t.Fatalf("ReadFile recovered.jpg: %v", err)
	}
	if !bytes.Equal(restoredBytes, fileBytes) {
		t.Errorf("recovered file content mismatch")
	}

	// Verify download conflict policy skip / fail / rename
	skipTr, err := eng.StartDownload(ctx, engine.DownloadJobRequest{
		Items: []engine.DownloadItemSpec{{
			NodeID:       "node-ok",
			Name:         "recovered.jpg",
			ExpectedSize: int64(len(fileBytes)),
			ExpectedMD5:  expectedMD5,
		}},
		DestDir:        destDir,
		ConflictPolicy: aphotos.ConflictSkip,
	})
	if err != nil {
		t.Fatalf("StartDownload(skip): %v", err)
	}
	doneSkip := waitForTransferTerminal(t, eng, skipTr.ID)
	if doneSkip.Stage != store.StageCompleted || doneSkip.ItemsSkipped != 1 {
		t.Fatalf("expected StageCompleted with 1 skipped on ConflictSkip, got %+v", doneSkip)
	}

	failTr, err := eng.StartDownload(ctx, engine.DownloadJobRequest{
		Items: []engine.DownloadItemSpec{{
			NodeID:       "node-ok",
			Name:         "recovered.jpg",
			ExpectedSize: int64(len(fileBytes)),
			ExpectedMD5:  expectedMD5,
		}},
		DestDir:        destDir,
		ConflictPolicy: aphotos.ConflictFail,
	})
	if err != nil {
		t.Fatalf("StartDownload(fail): %v", err)
	}
	doneFail := waitForTransferTerminal(t, eng, failTr.ID)
	if doneFail.Stage != store.StageFailed || doneFail.ItemsFailed != 1 {
		t.Fatalf("expected StageFailed with 1 failed on ConflictFail, got %+v", doneFail)
	}

	renameTr, err := eng.StartDownload(ctx, engine.DownloadJobRequest{
		Items: []engine.DownloadItemSpec{{
			NodeID:       "node-ok",
			Name:         "recovered.jpg",
			ExpectedSize: int64(len(fileBytes)),
			ExpectedMD5:  expectedMD5,
		}},
		DestDir:        destDir,
		ConflictPolicy: aphotos.ConflictRename,
	})
	if err != nil {
		t.Fatalf("StartDownload(rename): %v", err)
	}
	doneRename := waitForTransferTerminal(t, eng, renameTr.ID)
	if doneRename.Stage != store.StageCompleted || doneRename.ItemsDone != 1 {
		t.Fatalf("expected StageCompleted with 1 done on ConflictRename, got %+v", doneRename)
	}
	if _, statErr := os.Stat(filepath.Join(destDir, "recovered (1).jpg")); statErr != nil {
		t.Fatalf("expected disambiguated file recovered (1).jpg on ConflictRename, got %v", statErr)
	}
}

func TestCancelTransferAndBackupPauseResumeAndGlobalStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	blockUpload := make(chan struct{})
	var closeOnce sync.Once
	unblock := func() { closeOnce.Do(func() { close(blockUpload) }) }
	defer unblock()

	client := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes" {
			return jsonResponse(http.StatusOK, map[string]any{
				"count": 1,
				"data":  []map[string]any{{"id": "root-cancel", "ownerId": "owner-c", "kind": "FOLDER"}},
			}), nil
		}
		if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/cdproxy/nodes") {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-blockUpload:
				return jsonResponse(http.StatusCreated, map[string]any{"id": "node-ok", "name": "a.png", "kind": "FILE"}), nil
			}
		}
		return jsonResponse(http.StatusNotFound, nil), nil
	}))

	eng, _ := newTestEngine(t, client)
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.png")
	writePNGFile(t, p1, 8, 8)

	tr, err := eng.StartUpload(ctx, engine.UploadJobRequest{
		Paths:          []string{p1},
		ConflictPolicy: aphotos.ConflictFail,
	})
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}

	if err := eng.CancelTransfer(ctx, tr.ID); err != nil {
		t.Fatalf("CancelTransfer: %v", err)
	}
	unblock()

	cancelledTr, err := eng.GetTransfer(ctx, tr.ID)
	if err != nil || cancelledTr.Stage != store.StageCancelled {
		t.Fatalf("expected StageCancelled, got %+v (err=%v)", cancelledTr, err)
	}

	// Verify global stop and per-rule pause/resume
	if err := eng.SetBackupsStopped(ctx, true); err != nil {
		t.Fatalf("SetBackupsStopped: %v", err)
	}
	rule, err := eng.AddBackupRule(ctx, engine.BackupRuleInput{
		LocalPath:      dir,
		IncludeVideos:  true,
		ScheduleMode:   store.ScheduleImmediate,
		ConflictPolicy: aphotos.ConflictSkip,
	})
	if err != nil {
		t.Fatalf("AddBackupRule: %v", err)
	}
	pausedRule, err := eng.PauseBackupRule(ctx, rule.ID)
	if err != nil || !pausedRule.Paused || pausedRule.Status != store.BackupStatusPaused {
		t.Fatalf("PauseBackupRule failed: %+v (err=%v)", pausedRule, err)
	}
	resumedRule, err := eng.ResumeBackupRule(ctx, rule.ID)
	if err != nil || resumedRule.Paused || resumedRule.Status != store.BackupStatusActive {
		t.Fatalf("ResumeBackupRule failed: %+v (err=%v)", resumedRule, err)
	}
}

func TestReconcileRejectsSameNameSameSizeWithDifferentOrMissingMD5AndHandlesNilContentProperties(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	localMD5 := "11111111111111111111111111111111"
	otherMD5 := "22222222222222222222222222222222"

	client := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes" {
			f := req.URL.Query().Get("filters")
			if strings.Contains(f, "isRoot:true") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "root-1", "ownerId": "owner-1", "kind": "FOLDER", "isRoot": true}},
				}), nil
			}
			if strings.Contains(f, localMD5) {
				return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil
			}
			if strings.Contains(f, "name:collision.jpg") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 2,
					"data": []map[string]any{
						{
							"id":   "node-nil-cp",
							"name": "collision.jpg",
							"kind": "FILE",
						},
						{
							"id":   "node-diff-md5",
							"name": "collision.jpg",
							"kind": "FILE",
							"contentProperties": map[string]any{
								"size": 1024,
								"md5":  otherMD5,
							},
						},
					},
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes/album-nil/children" {
			return jsonResponse(http.StatusOK, map[string]any{
				"count": 1,
				"data": []map[string]any{
					{
						"id":   "node-nil-dl",
						"name": "nil_meta.mp4",
						"kind": "FILE",
					},
				},
			}), nil
		}
		if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/contentRedirection") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Disposition": []string{`attachment; filename="nil_meta.mp4"`}},
				Body:       io.NopCloser(strings.NewReader("video-data")),
			}, nil
		}
		return jsonResponse(http.StatusNotFound, nil), nil
	}))

	eng, st := newTestEngine(t, client)
	now := time.Now().UTC()
	_, err := st.CreateTransfer(ctx, store.Transfer{
		ID:         "tr-same-size-diff-md5",
		Kind:       store.TransferKindUpload,
		Stage:      store.StageRunning,
		BytesTotal: 1024,
		ItemsTotal: 1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, []store.TransferItem{
		{
			ID:         "it-collision",
			TransferID: "tr-same-size-diff-md5",
			LocalPath:  "/nonexistent/collision.jpg",
			Name:       "collision.jpg",
			MediaType:  store.MediaTypePhoto,
			Size:       1024,
			MD5:        localMD5,
			Status:     store.ItemStatusRunning,
			UpdatedAt:  now,
		},
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	if err := eng.ReconcileOnStartup(ctx); err != nil {
		t.Fatalf("ReconcileOnStartup: %v", err)
	}

	items, err := st.ListTransferItems(ctx, "tr-same-size-diff-md5")
	if err != nil || len(items) != 1 {
		t.Fatalf("ListTransferItems: %v, %+v", err, items)
	}
	if items[0].Status != store.ItemStatusUnconfirmed {
		t.Fatalf("expected item to stay unconfirmed when remote candidate has different MD5 or nil ContentProperties, got %+v", items[0])
	}

	// Also verify StartDownload with container node whose child has nil ContentProperties does not panic
	dlTr, err := eng.StartDownload(ctx, engine.DownloadJobRequest{
		ContainerNodeID: "album-nil",
		DestDir:         t.TempDir(),
	})
	if err != nil {
		t.Fatalf("StartDownload with nil ContentProperties: %v", err)
	}
	doneDL := waitForTransferTerminal(t, eng, dlTr.ID)
	if doneDL.Stage != store.StageCompleted {
		t.Fatalf("expected StageCompleted for nil ContentProperties download, got %+v", doneDL)
	}
}

func TestBackupScanReportsPartialDirectoryWalkError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}
	t.Parallel()
	ctx := context.Background()

	client := newTestClient(t, aphotos.RegionUS, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil
	}))
	eng, st := newTestEngine(t, client)

	backupDir := filepath.Join(t.TempDir(), "partial-scan")
	lockedSub := filepath.Join(backupDir, "locked")
	if err := os.MkdirAll(lockedSub, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Chmod(lockedSub, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(lockedSub, 0o700) })

	rule, err := eng.AddBackupRule(ctx, engine.BackupRuleInput{
		LocalPath:     backupDir,
		IncludeVideos: true,
		ScheduleMode:  store.ScheduleInterval,
		Paused:        true,
	})
	if err != nil {
		t.Fatalf("AddBackupRule: %v", err)
	}

	_, scanErr := eng.ScanNow(ctx, rule.ID)
	if scanErr == nil {
		t.Fatal("expected ScanNow to return error when a subdirectory cannot be read")
	}
	updatedRule, err := st.GetBackupRule(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetBackupRule: %v", err)
	}
	if updatedRule.Status != store.BackupStatusError || updatedRule.LastError == "" {
		t.Fatalf("expected rule status error with LastError populated, got %+v", updatedRule)
	}
}
