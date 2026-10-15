package view

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResponse(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(b)),
	}
}

func folderNode(id, name, parent string) map[string]any {
	node := map[string]any{"id": id, "name": name, "kind": "FOLDER", "status": "AVAILABLE", "ownerId": "owner-us-1"}
	if parent == "" {
		node["isRoot"] = true
		node["name"] = ""
	} else {
		node["parents"] = []string{parent}
	}
	return node
}

func nodeResponse(rest string) *http.Response {
	id, children := strings.CutSuffix(rest, "/children")
	folders := map[string][]map[string]any{
		"root-us-1": {folderNode("trips", "Trips", "root-us-1"), folderNode("family", "Family", "root-us-1")},
		"trips":     {folderNode("japan", "Japan", "trips")},
	}
	if children {
		data := folders[id]
		return jsonResponse(http.StatusOK, map[string]any{"count": len(data), "data": data})
	}
	switch id {
	case "root-us-1":
		return jsonResponse(http.StatusOK, folderNode(id, "", ""))
	case "trips", "family":
		return jsonResponse(http.StatusOK, folderNode(id, strings.ToUpper(id[:1])+id[1:], "root-us-1"))
	case "japan":
		return jsonResponse(http.StatusOK, folderNode(id, "Japan", "trips"))
	}
	return jsonResponse(http.StatusNotFound, map[string]any{"message": "not found"})
}

func stubTransport() http.RoundTripper {
	return roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/drive/v1/nodes" && strings.Contains(req.URL.Query().Get("filters"), "isRoot:true"):
			return jsonResponse(http.StatusOK, map[string]any{
				"count": 1,
				"data":  []map[string]any{{"id": "root-us-1", "ownerId": "owner-us-1", "kind": "FOLDER", "status": "AVAILABLE", "isRoot": true}},
			}), nil
		case strings.HasPrefix(req.URL.Path, "/drive/v1/nodes/"):
			return nodeResponse(strings.TrimPrefix(req.URL.Path, "/drive/v1/nodes/")), nil
		case req.URL.Path == "/drive/v1/account/usage":
			return jsonResponse(http.StatusOK, map[string]any{
				"photo": map[string]any{
					"billable": map[string]any{"bytes": 0, "count": 0},
					"total":    map[string]any{"bytes": 10240, "count": 5},
				},
			}), nil
		}
		return jsonResponse(http.StatusNotFound, map[string]any{"message": "not found"}), nil
	})
}

type harness struct {
	t     *testing.T
	svc   *gui.Services
	app   *App
	tt    *ui.Tester
	queue []func()
}

func newHarness(t *testing.T, signedIn bool) *harness {
	t.Helper()
	dir := t.TempDir()
	svc, closeSvc, err := gui.Open(gui.Options{
		DBPath:    filepath.Join(dir, "state.db"),
		VaultPath: filepath.Join(dir, "credentials.json"),
		Transport: stubTransport(),
	})
	if err != nil {
		t.Fatalf("gui.Open: %v", err)
	}
	t.Cleanup(closeSvc)
	if signedIn {
		_, err := svc.Auth.ImportCookies(t.Context(), gui.ImportCookiesInput{
			RawCookies: "session_id=140-1111111-2222222; ubid_main=130-3333333-4444444; at_main=Atza|ValidToken123",
			Region:     "us",
		})
		if err != nil {
			t.Fatalf("ImportCookies: %v", err)
		}
	}
	return newHarnessFor(t, svc)
}

func newHarnessFor(t *testing.T, svc *gui.Services) *harness {
	t.Helper()
	icon, err := os.ReadFile("../../resources/icon.png")
	if err != nil {
		t.Fatalf("read icon: %v", err)
	}
	h := &harness{t: t, svc: svc}
	h.app = New(svc, Options{Version: "test", Icon: icon})
	h.app.spawn = func(fn func()) { h.queue = append(h.queue, fn) }
	h.tt = ui.NewTester(h.app.View, 1080, 720)
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	return h
}

func (h *harness) drain() {
	h.t.Helper()
	for i := 0; ; i++ {
		if i > 100 {
			h.t.Fatal("work keeps queueing more work")
		}
		queue := h.queue
		h.queue = nil
		for _, fn := range queue {
			fn()
		}
		h.tt.Frame()
		if len(h.queue) == 0 {
			return
		}
	}
}

func (h *harness) start() {
	h.t.Helper()
	h.app.Start()
	h.drain()
}

func (h *harness) wantText(s string) {
	h.t.Helper()
	if !h.tt.HasText(s) {
		h.t.Errorf("text %q not shown; texts: %q", s, h.tt.Texts())
	}
}

func (h *harness) noText(s string) {
	h.t.Helper()
	if h.tt.HasText(s) {
		h.t.Errorf("text %q is shown; texts: %q", s, h.tt.Texts())
	}
}
