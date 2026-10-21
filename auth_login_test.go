//go:build !js

package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/store"
)

func TestAuthLoginHeadlessCookieDoesNotReplaceAccountOnRejection(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "credentials.json")
	db := filepath.Join(dir, "rainforest.db")
	cs := credstore.NewFileStore(vault)
	old := map[string]string{"session-id": "old-session", "ubid-main": "old-ubid", "at-main": "old-token"}
	if err := cs.Save(credstore.Credentials{Region: aphotos.RegionUS, Cookies: old}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	st, err := store.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := st.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Region = "us"
	cfg.MaxUploadConcurrency = 3
	if err := st.UpdateSettings(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	st.Close()

	secret := "new-secret-value"
	var stderr bytes.Buffer
	code := runAuthCommandIO([]string{"login", "--region", "de", "--cookie", "session-id=sid; ubid-acbde=ubid; at-acbde=" + secret}, authIO{
		stdout:    io.Discard,
		stderr:    &stderr,
		term:      func() bool { return false },
		transport: roundTripStatus(401),
	}, dir)
	if code == 0 {
		t.Fatal("expected rejected login to fail")
	}
	if strings.Contains(stderr.String(), secret) {
		t.Fatal("stderr included the cookie")
	}
	saved, err := cs.Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Cookies["at-main"] != "old-token" || saved.Region != aphotos.RegionUS {
		t.Fatalf("rejected login replaced the account: %+v", saved.Region)
	}
	st, err = store.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg, err = st.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != "us" || cfg.MaxUploadConcurrency != 3 {
		t.Fatalf("settings changed on failed login: %+v", cfg)
	}
}

func TestAuthLoginHeadlessCookieSavesVerifiedSession(t *testing.T) {
	dir := t.TempDir()
	secret := "piped-secret"
	var stdout bytes.Buffer
	code := runAuthCommandIO([]string{"login", "--cookie-file", writeCookieFile(t, dir, secret)}, authIO{
		stdin:     strings.NewReader(""),
		stdout:    &stdout,
		stderr:    io.Discard,
		term:      func() bool { return false },
		transport: roundTripStatus(200),
	}, dir)
	if code != 0 {
		t.Fatalf("login failed: %s", stdout.String())
	}
	saved, err := credstore.NewFileStore(filepath.Join(dir, "credentials.json")).Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Region != aphotos.RegionDE || saved.Cookies["at-acbde"] != secret {
		t.Fatalf("saved account = %+v", saved)
	}
	st, err := store.Open(context.Background(), filepath.Join(dir, "rainforest.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg, err := st.GetSettings(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != "de" {
		t.Fatalf("settings region = %q", cfg.Region)
	}
	if !strings.Contains(stdout.String(), "Signed in to Amazon Photos (de, experimental)") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatal("stdout included the cookie")
	}
}

func TestAuthLoginStdinAndInteractivePaste(t *testing.T) {
	dir := t.TempDir()
	secret := "stdin-secret"
	code := runAuthCommandIO([]string{"login", "--region", "us"}, authIO{
		stdin:     strings.NewReader("session-id=sid; ubid-main=ubid; at-main=" + secret),
		stdout:    io.Discard,
		stderr:    io.Discard,
		term:      func() bool { return false },
		transport: roundTripStatus(200),
	}, dir)
	if code != 0 {
		t.Fatal(code)
	}
	saved, err := credstore.NewFileStore(filepath.Join(dir, "credentials.json")).Load()
	if err != nil {
		t.Fatal(err)
	}
	if saved.Cookies["at-main"] != secret {
		t.Fatalf("stdin cookie not saved")
	}

	dir = t.TempDir()
	var prompts []string
	code = runAuthCommandIO([]string{"login"}, authIO{
		stdout: io.Discard,
		stderr: io.Discard,
		term:   func() bool { return true },
		line: func(prompt string) (string, error) {
			prompts = append(prompts, prompt)
			return "us", nil
		},
		secret: func(prompt string) (string, error) {
			prompts = append(prompts, prompt)
			return "session-id=sid; ubid-main=ubid; at-main=hidden-secret", nil
		},
		transport: roundTripStatus(200),
	}, dir)
	if code != 0 {
		t.Fatal(code)
	}
	if len(prompts) != 2 || !strings.Contains(prompts[0], "Marketplace") || !strings.Contains(prompts[1], "hidden") {
		t.Fatalf("prompts = %#v", prompts)
	}
}

func TestAuthLoginUsage(t *testing.T) {
	var stderr bytes.Buffer
	if code := runAuthCommandIO(nil, authIO{stderr: &stderr}, t.TempDir()); code != 2 || !strings.Contains(stderr.String(), "auth login") {
		t.Fatalf("code/usage = %d %q", code, stderr.String())
	}
	stderr.Reset()
	if code := runAuthCommandIO([]string{"login", "--help"}, authIO{stderr: &stderr}, t.TempDir()); code != 0 {
		t.Fatalf("help code = %d", code)
	}
}

func writeCookieFile(t *testing.T, dir, secret string) string {
	t.Helper()
	path := filepath.Join(dir, "cookies.txt")
	data := "session-id=sid; ubid-acbde=ubid; at-acbde=" + secret
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type statusTrip int

func roundTripStatus(status int) http.RoundTripper { return statusTrip(status) }

func (s statusTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return nil, errUnexpected(req.Method)
	}
	body := `{"message":"rejected"}`
	if s == 200 {
		body = `{"data":[{"id":"root","kind":"FOLDER","ownerId":"owner","status":"AVAILABLE"}]}`
	}
	return &http.Response{
		StatusCode: int(s),
		Status:     http.StatusText(int(s)),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

type unexpected string

func errUnexpected(method string) error { return unexpected(method) }

func (e unexpected) Error() string { return "unexpected method " + string(e) }
