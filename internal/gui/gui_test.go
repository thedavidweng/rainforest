package gui_test

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
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
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/internal/store"
)

func TestErrorJSONEnvelopeAndSecretRedaction(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		in           error
		wantCode     string
		wantCategory string
		wantRetryMS  int64
	}{
		{
			name:         "offline error",
			in:           engine.ErrOffline,
			wantCode:     gui.ErrAuthRequired,
			wantCategory: "auth",
		},
		{
			name:         "credstore not configured",
			in:           credstore.ErrNotConfigured,
			wantCode:     gui.ErrAuthRequired,
			wantCategory: "auth",
		},
		{
			name:         "store not found",
			in:           store.ErrNotFound,
			wantCode:     gui.ErrResourceNotFound,
			wantCategory: "api",
		},
		{
			name: "auth 401 with secret in message",
			in: &aphotos.AuthenticationError{
				StatusCode: 401,
				Endpoint:   "https://www.amazon.com/drive/v1/nodes?session-id=140-9999999-8888888",
				RequestID:  "req-401",
				Message:    "Cookie: session-id=140-9999999-8888888; at-main=Atza|SuperSecretTokenValue",
			},
			wantCode:     gui.ErrAuthFailed,
			wantCategory: "auth",
		},
		{
			name: "rate limit 429 with retry after",
			in: &aphotos.RateLimitError{
				StatusCode: 429,
				Endpoint:   "/drive/v1/search",
				RetryAfter: 2500 * time.Millisecond,
				Message:    "too many requests",
			},
			wantCode:     gui.ErrRateLimited,
			wantCategory: "network",
			wantRetryMS:  2500,
		},
		{
			name: "config error",
			in: &aphotos.ConfigError{
				Field:   "region",
				Message: "explicit region required when cookies are at-main=Atza|Secret123",
			},
			wantCode:     gui.ErrConfigError,
			wantCategory: "config",
		},
		{
			name: "conflict error",
			in: &aphotos.ConflictError{
				StatusCode:        409,
				Endpoint:          "/cdproxy/nodes",
				ConflictingNodeID: "node-conflict-1",
				Message:           "node already exists",
			},
			wantCode:     gui.ErrConflict,
			wantCategory: "api",
		},
		{
			name:         "context canceled",
			in:           context.Canceled,
			wantCode:     gui.ErrInterrupted,
			wantCategory: "interrupted",
		},
		{
			name:         "filesystem error",
			in:           &fs.PathError{Op: "open", Path: "/tmp/missing.txt", Err: fs.ErrNotExist},
			wantCode:     gui.ErrFilesystemError,
			wantCategory: "filesystem",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := gui.ToError(tc.in)
			if mapped == nil {
				t.Fatal("expected non-nil error")
			}
			var ge *gui.Error
			if !errors.As(mapped, &ge) {
				t.Fatalf("expected *gui.Error, got %T", mapped)
			}
			if ge.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", ge.Code, tc.wantCode)
			}
			if ge.Category != tc.wantCategory {
				t.Errorf("Category = %q, want %q", ge.Category, tc.wantCategory)
			}
			if ge.RetryAfterMS != tc.wantRetryMS {
				t.Errorf("RetryAfterMS = %d, want %d", ge.RetryAfterMS, tc.wantRetryMS)
			}

			raw := ge.Error()
			if raw == "" || raw[0] != '{' {
				t.Fatalf("expected Error() to return JSON string, got %q", raw)
			}
			for _, secret := range []string{"140-9999999-8888888", "Atza|SuperSecretTokenValue", "Atza|Secret123"} {
				if containsString(raw, secret) {
					t.Errorf("Error() leaked secret %q in %s", secret, raw)
				}
			}

			var decoded gui.Error
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(ge.Error()): %v", err)
			}
			if decoded.Code != tc.wantCode || decoded.Category != tc.wantCategory {
				t.Errorf("decoded = %+v, want code=%q category=%q", decoded, tc.wantCode, tc.wantCategory)
			}
		})
	}
}

func containsString(haystack, needle string) bool {
	return needle != "" && strings.Contains(haystack, needle)
}

func TestTrayTrackerAndQuitLifecycleDecisions(t *testing.T) {
	t.Parallel()

	tracker := gui.NewTrayTracker()
	if tracker.ActiveCount() != 0 {
		t.Fatalf("initial ActiveCount = %d, want 0", tracker.ActiveCount())
	}

	menu := tracker.Menu()
	if len(menu) == 0 || menu[0].Label != "No active transfers" || menu[0].Action != gui.TrayActionNone {
		t.Fatalf("unexpected initial menu: %+v", menu)
	}

	trUpload := gui.TransferDTO{
		ID:            "tr-1",
		Kind:          string(store.TransferKindUpload),
		Stage:         string(store.StageRunning),
		SourceSummary: "photos (3 items)",
		BytesDone:     50,
		BytesTotal:    100,
		ItemsDone:     1,
		ItemsTotal:    3,
	}
	trDownload := gui.TransferDTO{
		ID:            "tr-2",
		Kind:          string(store.TransferKindDownload),
		Stage:         string(store.StageRunning),
		SourceSummary: "Recovery download (2 items)",
		BytesDone:     75,
		BytesTotal:    100,
		ItemsDone:     1,
		ItemsTotal:    2,
	}

	tracker.Apply(trUpload)
	tracker.Apply(trDownload)
	if tracker.ActiveCount() != 2 {
		t.Fatalf("ActiveCount = %d, want 2", tracker.ActiveCount())
	}

	menu = tracker.Menu()
	if len(menu) < 5 {
		t.Fatalf("expected at least 5 menu rows, got %+v", menu)
	}
	if !containsString(menu[0].Label, "↑") || !containsString(menu[0].Label, "50%") {
		t.Errorf("upload tray row = %q, want ↑ and 50%%", menu[0].Label)
	}
	if !containsString(menu[1].Label, "↓") || !containsString(menu[1].Label, "75%") {
		t.Errorf("download tray row = %q, want ↓ and 75%%", menu[1].Label)
	}

	trUpload.Stage = string(store.StageCompleted)
	tracker.Apply(trUpload)
	if tracker.ActiveCount() != 1 {
		t.Fatalf("ActiveCount after completion = %d, want 1", tracker.ActiveCount())
	}
	tracker.Remove("tr-2")
	if tracker.ActiveCount() != 0 {
		t.Fatalf("ActiveCount after remove = %d, want 0", tracker.ActiveCount())
	}

	if got := gui.DecideQuit(0, 0, false); got != gui.QuitImmediately {
		t.Errorf("DecideQuit(0, 0, false) = %v, want QuitImmediately", got)
	}
	if got := gui.DecideQuit(0, 1, false); got != gui.QuitPromptConfirm {
		t.Errorf("DecideQuit(0, 1, false) = %v, want QuitPromptConfirm when active backups exist and closeToTray is false", got)
	}
	if got := gui.DecideQuit(0, 1, true); got != gui.QuitImmediately {
		t.Errorf("DecideQuit(0, 1, true) = %v, want QuitImmediately when closeToTray is true and no active transfer is running", got)
	}
	if got := gui.DecideQuit(1, 1, true); got != gui.QuitPromptConfirm {
		t.Errorf("DecideQuit(1, 1, true) = %v, want QuitPromptConfirm", got)
	}

	msg := gui.QuitMessage(2, 1, true)
	for _, phrase := range []string{"Pause Task", "Stop Backups", "Close Window to Tray", "Quit Rainforest"} {
		if !containsString(msg, phrase) {
			t.Errorf("QuitMessage(%d, %d, true) = %q, missing %q", 2, 1, msg, phrase)
		}
	}
}

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

type fakePicker struct {
	cookieFile string
	files      []string
	directory  string
	err        error
}

func (p *fakePicker) PickCookieFile(ctx context.Context) (string, error) {
	return p.cookieFile, p.err
}

func (p *fakePicker) PickFiles(ctx context.Context) ([]string, error) {
	return p.files, p.err
}

func (p *fakePicker) PickDirectory(ctx context.Context, title string) (string, error) {
	return p.directory, p.err
}

func TestAuthOfflineBootCookieImportUsageAndLogout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes" && strings.Contains(req.URL.Query().Get("filters"), "isRoot:true") {
			return jsonResponse(http.StatusOK, map[string]any{
				"count": 1,
				"data":  []map[string]any{{"id": "root-us-1", "ownerId": "owner-us-1", "kind": "FOLDER"}},
			}), nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/account/usage" {
			return jsonResponse(http.StatusOK, map[string]any{
				"lastCalculated": "2026-10-14T10:00:00Z",
				"photo": map[string]any{
					"billable": map[string]any{"bytes": 0, "count": 0},
					"total":    map[string]any{"bytes": 10240, "count": 5},
				},
				"video": map[string]any{
					"billable": map[string]any{"bytes": 40960, "count": 2},
					"total":    map[string]any{"bytes": 40960, "count": 2},
				},
			}), nil
		}
		return jsonResponse(http.StatusNotFound, map[string]any{"message": "not found"}), nil
	})

	svc := gui.NewEmptyServices()
	if svc.Auth == nil || svc.Transfers == nil || svc.Backups == nil || svc.Downloads == nil || svc.Settings == nil {
		t.Fatal("NewEmptyServices must return non-nil service pointers for MyGo bindings")
	}

	closeSvc, err := svc.Init(gui.Options{
		DBPath:    filepath.Join(dir, "state.db"),
		VaultPath: filepath.Join(dir, "credentials.json"),
		Transport: rt,
	})
	if err != nil {
		t.Fatalf("svc.Init: %v", err)
	}
	defer closeSvc()

	var eventsMu sync.Mutex
	var eventNames []string
	svc.SetEmitter(func(name string, data any) {
		eventsMu.Lock()
		eventNames = append(eventNames, name)
		eventsMu.Unlock()
	})

	st0, err := svc.Auth.Status(ctx)
	if err != nil {
		t.Fatalf("Auth.Status (offline): %v", err)
	}
	if st0.Configured || st0.Authenticated {
		t.Fatalf("expected offline unconfigured status, got %+v", st0)
	}

	if _, err := svc.Auth.Usage(ctx); err == nil {
		t.Fatal("expected Auth.Usage to fail when offline")
	} else {
		var ge *gui.Error
		if !errors.As(err, &ge) || ge.Code != gui.ErrAuthRequired {
			t.Fatalf("expected ErrAuthRequired from Usage when offline, got %v", err)
		}
	}

	cookieHeader := "session_id=140-1111111-2222222; ubid_main=130-3333333-4444444; at_main=Atza|ValidToken123"
	st1, err := svc.Auth.ImportCookies(ctx, gui.ImportCookiesInput{
		RawCookies: cookieHeader,
		Region:     "us",
	})
	if err != nil {
		t.Fatalf("Auth.ImportCookies: %v", err)
	}
	if !st1.Configured || !st1.Authenticated || st1.Region != "us" || st1.RootNodeID != "root-us-1" {
		t.Fatalf("unexpected AuthStatus after import: %+v", st1)
	}
	if len(st1.CookieKeys) != 3 {
		t.Errorf("CookieKeys = %v, want 3 keys", st1.CookieKeys)
	}

	usage, err := svc.Auth.Usage(ctx)
	if err != nil {
		t.Fatalf("Auth.Usage: %v", err)
	}
	if usage.Photo.Total.Count != 5 || usage.Video.Total.Bytes != 40960 || usage.TotalBytes != 51200 {
		t.Errorf("unexpected UsageSummary: %+v", usage)
	}

	cookieFile := filepath.Join(dir, "cookies.json")
	if err := os.WriteFile(cookieFile, []byte(`{"session-id":"140-7777777-8888888","ubid-main":"130-5555555-6666666","at-main":"Atza|FileToken"}`), 0o600); err != nil {
		t.Fatalf("WriteFile cookies.json: %v", err)
	}
	svc.SetFilePicker(&fakePicker{cookieFile: cookieFile})
	stPicked, err := svc.Auth.PickAndImportCookieFile(ctx, "us")
	if err != nil {
		t.Fatalf("PickAndImportCookieFile: %v", err)
	}
	if !stPicked.Authenticated {
		t.Errorf("expected Authenticated after PickAndImportCookieFile, got %+v", stPicked)
	}

	stOut, err := svc.Auth.Logout(ctx)
	if err != nil {
		t.Fatalf("Auth.Logout: %v", err)
	}
	if stOut.Configured || stOut.Authenticated {
		t.Errorf("expected unconfigured status after Logout, got %+v", stOut)
	}

	eventsMu.Lock()
	defer eventsMu.Unlock()
	var authEvents int
	for _, name := range eventNames {
		if name == gui.EventAuthChanged {
			authEvents++
		}
	}
	if authEvents < 3 {
		t.Errorf("expected at least 3 auth-changed events, got %d (%v)", authEvents, eventNames)
	}
}

func writePNGFile(t *testing.T, path string, w, h int) (pngBytes []byte, md5Hex string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 40, G: 160, B: 100, A: 255})
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
	return data, hex.EncodeToString(sum[:])
}

func waitForTransferStage(t *testing.T, svc *gui.Services, id string, wantTerminal bool) gui.TransferDTO {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		list, err := svc.Transfers.List(context.Background())
		if err == nil {
			for i := range list.History {
				if list.History[i].ID == id && wantTerminal {
					return list.History[i]
				}
			}
			for i := range list.Active {
				if list.Active[i].ID == id && !wantTerminal {
					return list.Active[i]
				}
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for transfer %s (wantTerminal=%v)", id, wantTerminal)
	return gui.TransferDTO{}
}

func TestFullEndToEndFacadeFlow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	uploadDir := filepath.Join(dir, "upload-src")
	photo1Path := filepath.Join(uploadDir, "vacation.png")
	photo1Bytes, photo1MD5 := writePNGFile(t, photo1Path, 32, 24)
	if err := os.WriteFile(filepath.Join(uploadDir, "readme.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("WriteFile readme.txt: %v", err)
	}

	backupDir := filepath.Join(dir, "backup-src")
	backupPhotoPath := filepath.Join(backupDir, "camera.png")
	backupBytes, backupMD5 := writePNGFile(t, backupPhotoPath, 24, 24)

	var uploadedMu sync.Mutex
	uploadedByNode := map[string][]byte{
		"node-vacation": photo1Bytes,
		"node-camera":   backupBytes,
	}

	var uploadCounter atomic.Int32
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes":
			f := req.URL.Query().Get("filters")
			if strings.Contains(f, "isRoot:true") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "root-us", "ownerId": "owner-1", "kind": "FOLDER"}},
				}), nil
			}
			if strings.Contains(f, "kind:FOLDER") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "folder-2026", "name": "2026 Trips", "kind": "FOLDER"}},
				}), nil
			}
			if strings.Contains(f, "kind:VISUAL_COLLECTION") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "album-family", "name": "Family", "kind": "VISUAL_COLLECTION"}},
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil

		case req.Method == http.MethodGet && req.URL.Path == "/drive/v1/account/usage":
			return jsonResponse(http.StatusOK, map[string]any{
				"lastCalculated": "2026-10-14T12:00:00Z",
				"photo": map[string]any{
					"billable": map[string]any{"bytes": 0, "count": 0},
					"total":    map[string]any{"bytes": int64(len(photo1Bytes) + len(backupBytes)), "count": 2},
				},
				"video": map[string]any{
					"billable": map[string]any{"bytes": 0, "count": 0},
					"total":    map[string]any{"bytes": 0, "count": 0},
				},
			}), nil

		case req.Method == http.MethodGet && req.URL.Path == "/drive/v1/search":
			return jsonResponse(http.StatusOK, map[string]any{
				"count": 2,
				"data":  []map[string]any{{"id": "node-vacation", "name": "vacation.png", "kind": "FILE"}},
			}), nil

		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/cdproxy/nodes"):
			uploadCounter.Add(1)
			name := req.URL.Query().Get("name")
			body, _ := io.ReadAll(req.Body)
			sum := md5.Sum(body)
			md5Hex := hex.EncodeToString(sum[:])
			nodeID := "node-" + strings.TrimSuffix(name, filepath.Ext(name))
			uploadedMu.Lock()
			uploadedByNode[nodeID] = body
			uploadedMu.Unlock()
			return jsonResponse(http.StatusCreated, map[string]any{
				"id":   nodeID,
				"name": name,
				"kind": "FILE",
				"contentProperties": map[string]any{
					"size": len(body),
					"md5":  md5Hex,
				},
			}), nil

		case req.Method == http.MethodPost && req.URL.Path == "/drive/v1/nodes":
			return jsonResponse(http.StatusCreated, map[string]any{
				"id":   "probe-folder-1",
				"name": ".rainforest-doctor-probe",
				"kind": "FOLDER",
			}), nil

		case req.Method == http.MethodPatch && req.URL.Path == "/drive/v1/trash":
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/contentRedirection"):
			parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/drive/v1/nodes/"), "/")
			nodeID := parts[0]
			uploadedMu.Lock()
			payload := uploadedByNode[nodeID]
			uploadedMu.Unlock()
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(bytes.NewReader(payload)),
			}, nil
		}
		return jsonResponse(http.StatusNotFound, map[string]any{"message": "unhandled route: " + req.URL.Path}), nil
	})

	var eventsMu sync.Mutex
	eventCounts := map[string]int{}
	var loginHookCalled bool

	svc, closeSvc, err := gui.Open(gui.Options{
		DBPath:    filepath.Join(dir, "state.db"),
		VaultPath: filepath.Join(dir, "credentials.json"),
		Transport: rt,
		Emitter: func(name string, data any) {
			eventsMu.Lock()
			eventCounts[name]++
			eventsMu.Unlock()
		},
		Picker: &fakePicker{
			files:     []string{photo1Path},
			directory: uploadDir,
		},
		LaunchAtLoginFn: func(enabled bool) error {
			loginHookCalled = enabled
			return nil
		},
	})
	if err != nil {
		t.Fatalf("gui.Open: %v", err)
	}
	defer closeSvc()

	// 1. Offline doctor check
	docOffline, err := svc.Settings.Doctor(ctx)
	if err != nil {
		t.Fatalf("Settings.Doctor (offline): %v", err)
	}
	if docOffline.Healthy || !docOffline.Unconfigured {
		t.Fatalf("expected offline doctor to report Unconfigured=true Healthy=false, got %+v", docOffline)
	}

	// 2. Import cookies -> goes online
	authSt, err := svc.Auth.ImportCookies(ctx, gui.ImportCookiesInput{
		RawCookies: "session-id=140-0000000-1111111; ubid-main=130-2222222-3333333; at-main=Atza|LiveSession",
		Region:     "us",
	})
	if err != nil {
		t.Fatalf("Auth.ImportCookies: %v", err)
	}
	if !authSt.Authenticated || authSt.Region != "us" {
		t.Fatalf("expected authenticated US status, got %+v", authSt)
	}

	// 3. Upload preflight & execution
	svc.EmitFilesDropped([]string{uploadDir}, 120, 80)
	plan, err := svc.Transfers.PlanUpload(ctx, gui.UploadPlanInput{
		Paths:          []string{uploadDir},
		ConflictPolicy: "skip",
	})
	if err != nil {
		t.Fatalf("Transfers.PlanUpload: %v", err)
	}
	if plan.PhotoCount != 1 || plan.SkippedUnsupported != 1 || len(plan.Items) != 1 {
		t.Fatalf("unexpected UploadPlanDTO: %+v", plan)
	}
	if !strings.HasPrefix(plan.Items[0].ThumbnailDataURL, "data:image/png;base64,") {
		t.Errorf("expected local preview thumbnail in plan item, got %q", plan.Items[0].ThumbnailDataURL)
	}

	upTr, err := svc.Transfers.StartUpload(ctx, gui.StartUploadInput{
		Paths:          []string{uploadDir},
		ConflictPolicy: "skip",
	})
	if err != nil {
		t.Fatalf("Transfers.StartUpload: %v", err)
	}
	finalUp := waitForTransferStage(t, svc, upTr.ID, true)
	if finalUp.Stage != string(store.StageCompleted) || finalUp.ItemsDone != 1 {
		t.Fatalf("upload transfer finished with %+v, want completed with 1 item", finalUp)
	}

	upItems, err := svc.Transfers.GetItems(ctx, upTr.ID)
	if err != nil {
		t.Fatalf("Transfers.GetItems: %v", err)
	}
	if len(upItems) != 1 || upItems[0].RemoteNodeID != "node-vacation" || upItems[0].MD5 != photo1MD5 {
		t.Fatalf("unexpected upload items: %+v", upItems)
	}

	// 4. Backups add / pause / resume / scan / remove
	rule, err := svc.Backups.Add(ctx, gui.BackupRuleInput{
		LocalPath:      backupDir,
		IncludeVideos:  true,
		ScheduleMode:   "immediate",
		ConflictPolicy: "skip",
		Paused:         true,
	})
	if err != nil {
		t.Fatalf("Backups.Add: %v", err)
	}
	if !rule.Paused || rule.Status != string(store.BackupStatusPaused) {
		t.Fatalf("expected paused initial rule, got %+v", rule)
	}

	scanTr, err := svc.Backups.ScanNow(ctx, rule.ID)
	if err != nil {
		t.Fatalf("Backups.ScanNow: %v", err)
	}
	if scanTr == nil || scanTr.Stage != string(store.StageCompleted) || scanTr.ItemsDone != 1 {
		t.Fatalf("unexpected backup scan transfer: %+v", scanTr)
	}

	pausedRule, err := svc.Backups.Pause(ctx, rule.ID)
	if err != nil || !pausedRule.Paused {
		t.Fatalf("Backups.Pause: %v, %+v", err, pausedRule)
	}
	resumedRule, err := svc.Backups.Resume(ctx, rule.ID)
	if err != nil || resumedRule.Paused {
		t.Fatalf("Backups.Resume: %v, %+v", err, resumedRule)
	}
	if activeBackups := svc.ActiveBackupCount(ctx); activeBackups != 1 {
		t.Errorf("ActiveBackupCount = %d, want 1", activeBackups)
	}
	if err := svc.Backups.SetAllStopped(ctx, true); err != nil {
		t.Fatalf("Backups.SetAllStopped: %v", err)
	}
	if activeBackups := svc.ActiveBackupCount(ctx); activeBackups != 0 {
		t.Errorf("ActiveBackupCount when stopped = %d, want 0", activeBackups)
	}
	if err := svc.Backups.SetAllStopped(ctx, false); err != nil {
		t.Fatalf("Backups.SetAllStopped(false): %v", err)
	}

	// 5. Recovery Downloads: ListDestinations + StartDownload
	sources, err := svc.Downloads.ListDestinations(ctx)
	if err != nil {
		t.Fatalf("Downloads.ListDestinations: %v", err)
	}
	if len(sources.ConfirmedItems) < 2 || len(sources.Folders) != 1 || len(sources.Albums) != 1 {
		t.Fatalf("unexpected RecoverySourcesDTO: %+v", sources)
	}

	dlDest := filepath.Join(dir, "recovered")
	dlTr, err := svc.Downloads.StartDownload(ctx, gui.DownloadInput{
		Items: []gui.DownloadItemInput{
			{
				NodeID:       "node-camera",
				Name:         "camera.png",
				ExpectedSize: int64(len(backupBytes)),
				ExpectedMD5:  backupMD5,
				MediaType:    "photo",
			},
		},
		DestDir: dlDest,
	})
	if err != nil {
		t.Fatalf("Downloads.StartDownload: %v", err)
	}
	finalDL := waitForTransferStage(t, svc, dlTr.ID, true)
	if finalDL.Stage != string(store.StageCompleted) || finalDL.ItemsDone != 1 {
		t.Fatalf("download transfer finished with %+v, want completed", finalDL)
	}
	recoveredBytes, err := os.ReadFile(filepath.Join(dlDest, "camera.png"))
	if err != nil {
		t.Fatalf("ReadFile recovered camera.png: %v", err)
	}
	if !bytes.Equal(recoveredBytes, backupBytes) {
		t.Fatal("recovered file bytes do not match original backup bytes")
	}

	// 6. Settings Update & Doctor online
	enableLogin := true
	maxUp := 3
	updatedSettings, err := svc.Settings.Update(ctx, gui.SettingsInput{
		LaunchAtLogin:        &enableLogin,
		MaxUploadConcurrency: &maxUp,
	})
	if err != nil {
		t.Fatalf("Settings.Update: %v", err)
	}
	if !updatedSettings.LaunchAtLogin || !loginHookCalled || updatedSettings.MaxUploadConcurrency != 3 {
		t.Errorf("unexpected updated settings: %+v (loginHookCalled=%v)", updatedSettings, loginHookCalled)
	}

	docOnline, err := svc.Settings.Doctor(ctx)
	if err != nil {
		t.Fatalf("Settings.Doctor (online): %v", err)
	}
	if !docOnline.Healthy || len(docOnline.Checks) != 5 {
		t.Fatalf("expected healthy 5-grade doctor report, got %+v", docOnline)
	}
	if docOnline.Checks[4].Status != "skipped" {
		t.Errorf("expected grade 5 write_probe skipped by default, got %+v", docOnline.Checks[4])
	}
	if _, err := svc.Settings.DoctorWithWriteProbe(ctx, false); err == nil {
		t.Fatal("expected DoctorWithWriteProbe(false) to require confirmation")
	}
	docProbe, err := svc.Settings.DoctorWithWriteProbe(ctx, true)
	if err != nil {
		t.Fatalf("Settings.DoctorWithWriteProbe(true): %v", err)
	}
	if !docProbe.Healthy || len(docProbe.Checks) != 5 || docProbe.Checks[4].Status != "pass" {
		t.Fatalf("expected healthy grade 5 pass from DoctorWithWriteProbe(true), got %+v", docProbe)
	}

	// 7. Remove backup rule & clear history
	if err := svc.Backups.Remove(ctx, rule.ID); err != nil {
		t.Fatalf("Backups.Remove: %v", err)
	}
	if err := svc.Transfers.ClearHistory(ctx); err != nil {
		t.Fatalf("Transfers.ClearHistory: %v", err)
	}
	afterClear, err := svc.Transfers.List(ctx)
	if err != nil {
		t.Fatalf("Transfers.List after clear: %v", err)
	}
	if len(afterClear.History) != 0 {
		t.Errorf("expected empty transfer history after ClearHistory, got %d items", len(afterClear.History))
	}

	eventsMu.Lock()
	defer eventsMu.Unlock()
	for _, requiredEvent := range []string{
		gui.EventAuthChanged,
		gui.EventFilesDropped,
		gui.EventTransferStage,
		gui.EventBackupStatus,
		gui.EventNotification,
	} {
		if eventCounts[requiredEvent] == 0 {
			t.Errorf("expected event %q to be emitted at least once, counts=%v", requiredEvent, eventCounts)
		}
	}
}

func TestGUIPackageZeroMyGoOrWebviewImports(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", entry.Name(), err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(p, "mygo") || strings.Contains(p, "webview") || strings.Contains(p, "wails") {
				t.Errorf("file %s imports forbidden desktop/webview package %q", entry.Name(), p)
			}
		}
	}
}

func TestTransferControlsPickerAndExperimentalRegionCapability(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	imgPath := filepath.Join(dir, "sample.png")
	writePNGFile(t, imgPath, 16, 16)

	blockUpload := make(chan struct{})
	var closeOnce sync.Once
	releaseUpload := func() { closeOnce.Do(func() { close(blockUpload) }) }
	defer releaseUpload()

	var failUploads atomic.Bool
	failUploads.Store(true)

	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/drive/v1/nodes" {
			if strings.Contains(req.URL.Query().Get("filters"), "isRoot:true") {
				return jsonResponse(http.StatusOK, map[string]any{
					"count": 1,
					"data":  []map[string]any{{"id": "root-ca", "ownerId": "owner-ca", "kind": "FOLDER"}},
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{"count": 0, "data": []any{}}), nil
		}
		if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/cdproxy/nodes") {
			_, _ = io.Copy(io.Discard, req.Body)
			<-blockUpload
			if failUploads.Load() {
				return jsonResponse(http.StatusInternalServerError, map[string]any{"message": "simulated failure"}), nil
			}
			return jsonResponse(http.StatusCreated, map[string]any{
				"id":   "node-sample-ca",
				"name": "sample.png",
				"kind": "FILE",
			}), nil
		}
		return jsonResponse(http.StatusNotFound, map[string]any{"message": "not found"}), nil
	})

	svc, closeSvc, err := gui.Open(gui.Options{
		DBPath:    filepath.Join(dir, "state.db"),
		VaultPath: filepath.Join(dir, "credentials.json"),
		Transport: rt,
		Picker: &fakePicker{
			files:     []string{imgPath},
			directory: dir,
		},
	})
	if err != nil {
		t.Fatalf("gui.Open: %v", err)
	}
	defer closeSvc()

	cookieFile := filepath.Join(dir, "ca-cookies.json")
	if err := os.WriteFile(cookieFile, []byte(`{"session-id":"140-111-222","ubid-acbca":"130-333-444","at-acbca":"Atza|CA"}`), 0o600); err != nil {
		t.Fatalf("WriteFile ca-cookies: %v", err)
	}
	stCA, err := svc.Auth.ImportCookieFile(ctx, cookieFile, "ca")
	if err != nil {
		t.Fatalf("ImportCookieFile(ca): %v", err)
	}
	if stCA.Region != "ca" || stCA.Evidence != "experimental" {
		t.Fatalf("expected experimental CA status, got %+v", stCA)
	}

	pickedFiles, err := svc.Transfers.PickFiles(ctx)
	if err != nil || len(pickedFiles) != 1 || pickedFiles[0] != imgPath {
		t.Fatalf("PickFiles: %v, %v", err, pickedFiles)
	}
	if d, err := svc.Transfers.PickDirectory(ctx); err != nil || d != dir {
		t.Fatalf("PickDirectory: %v, %q", err, d)
	}
	if d, err := svc.Backups.PickFolder(ctx); err != nil || d != dir {
		t.Fatalf("PickFolder: %v, %q", err, d)
	}
	if d, err := svc.Downloads.PickTargetDirectory(ctx); err != nil || d != dir {
		t.Fatalf("PickTargetDirectory: %v, %q", err, d)
	}

	sources, err := svc.Downloads.ListDestinations(ctx)
	if err != nil {
		t.Fatalf("ListDestinations: %v", err)
	}
	if sources.Evidence != "experimental" || !strings.Contains(sources.CapabilityNotes, "Experimental region") {
		t.Errorf("expected experimental capability notes, got %+v", sources)
	}

	tr, err := svc.Transfers.StartUpload(ctx, gui.StartUploadInput{
		Paths:             []string{imgPath},
		DestinationNodeID: "custom-folder-ignored-in-experimental",
	})
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}
	if tr.DestNodeID != "" {
		t.Errorf("expected empty DestNodeID in experimental region, got %q", tr.DestNodeID)
	}

	paused, err := svc.Transfers.Pause(ctx, tr.ID)
	if err != nil || paused.Stage != string(store.StagePaused) {
		t.Fatalf("Pause: %v, %+v", err, paused)
	}
	resumed, err := svc.Transfers.Resume(ctx, tr.ID)
	if err != nil || resumed.Stage != string(store.StageRunning) {
		t.Fatalf("Resume: %v, %+v", err, resumed)
	}
	cancelled, err := svc.Transfers.Cancel(ctx, tr.ID)
	if err != nil || cancelled.Stage != string(store.StageCancelled) {
		t.Fatalf("Cancel: %v, %+v", err, cancelled)
	}

	failUploads.Store(false)
	releaseUpload()

	retried, err := svc.Transfers.Retry(ctx, tr.ID)
	if err != nil {
		t.Fatalf("Retry: %v", err)
	}
	finalRetried := waitForTransferStage(t, svc, retried.ID, true)
	if finalRetried.Stage != string(store.StageCompleted) {
		t.Fatalf("expected completed transfer after Retry, got %+v", finalRetried)
	}

	ruleDir := filepath.Join(dir, "rule-folder")
	if err := os.MkdirAll(ruleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll ruleDir: %v", err)
	}
	rule, err := svc.Backups.Add(ctx, gui.BackupRuleInput{
		LocalPath:     ruleDir,
		IncludeVideos: false,
		Paused:        true,
	})
	if err != nil {
		t.Fatalf("Backups.Add: %v", err)
	}
	updatedRule, err := svc.Backups.Update(ctx, rule.ID, gui.BackupRuleInput{
		LocalPath:       ruleDir,
		IncludeVideos:   true,
		ScheduleMode:    "interval",
		IntervalMinutes: 30,
		ConflictPolicy:  "rename",
		Paused:          true,
	})
	if err != nil {
		t.Fatalf("Backups.Update: %v", err)
	}
	if !updatedRule.IncludeVideos || updatedRule.IntervalMinutes != 30 || updatedRule.ConflictPolicy != "rename" {
		t.Errorf("unexpected updated rule: %+v", updatedRule)
	}
	rules, err := svc.Backups.List(ctx)
	if err != nil || len(rules) != 1 {
		t.Fatalf("Backups.List: %v, %+v", err, rules)
	}
}
