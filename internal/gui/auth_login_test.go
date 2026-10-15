package gui_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/gui"
)

func TestBrowserLoginVerifiesBeforeSaving(t *testing.T) {
	for _, region := range []aphotos.Region{aphotos.RegionUS, aphotos.RegionCA, aphotos.RegionUK, aphotos.RegionJP, aphotos.RegionDE} {
		for _, status := range []int{200, 401, 403} {
			t.Run(string(region)+http.StatusText(status), func(t *testing.T) {
				dir := t.TempDir()
				vault := filepath.Join(dir, "credentials.json")
				rc, _ := aphotos.ResolveRegion(region)
				cs := credstore.NewFileStore(vault)
				old := map[string]string{"session-id": "old-session", "ubid-main": "old-ubid", "at-main": "old-token"}
				if err := cs.Save(credstore.Credentials{Region: aphotos.RegionUS, Cookies: old}); err != nil {
					t.Fatal(err)
				}
				incoming := map[string]string{}
				for _, key := range rc.RequiredCookies {
					incoming[key] = "new-secret"
				}
				svc, closeFn, err := gui.Open(gui.Options{
					DBPath: filepath.Join(dir, "db"), VaultPath: vault, PollInterval: time.Hour,
					BrowserLogin: func(ctx context.Context, got aphotos.RegionConfig, verify func(map[string]string) error) error {
						if got.Region != region {
							t.Fatal("wrong region")
						}
						return verify(incoming)
					}, Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != "GET" {
							t.Fatal("login wrote to Amazon")
						}
						if strings.Contains(req.Header.Get("Cookie"), "old-token") {
							return jsonResponse(200, json.RawMessage(`{"data":[{"id":"old-root","kind":"FOLDER","ownerId":"old-owner"}]}`)), nil
						}
						if req.URL.Host != "www.amazon."+rc.Domain {
							t.Errorf("wrong host: %s", req.URL.Host)
						}
						if status != 200 {
							return jsonResponse(status, json.RawMessage(`{"message":"rejected"}`)), nil
						}
						return jsonResponse(200, json.RawMessage(`{"data":[{"id":"new-root","kind":"FOLDER","ownerId":"new-owner","status":"AVAILABLE"}]}`)), nil
					}),
				})
				if err != nil {
					t.Fatal(err)
				}
				defer closeFn()
				result, err := svc.Auth.Login(context.Background(), string(region))
				saved, loadErr := cs.Load()
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				if status != 200 {
					if err == nil {
						t.Fatal("expected failed login")
					}
					if saved.Cookies["at-main"] != "old-token" {
						t.Fatal("failed login replaced old account")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if !result.Authenticated || result.OwnerID != "new-owner" || result.Region != string(region) || result.Evidence != string(rc.Evidence) {
					t.Fatalf("wrong status: %+v", result)
				}
				if saved.Region != region || saved.Cookies[rc.RequiredCookies[2]] != "new-secret" {
					t.Fatal("new account not saved")
				}
			})
		}
	}
}

func TestBrowserLoginCancellationAndRetry(t *testing.T) {
	dir := t.TempDir()
	entered := make(chan struct{}, 1)
	svc, closeFn, err := gui.Open(gui.Options{
		DBPath: filepath.Join(dir, "db"), VaultPath: filepath.Join(dir, "vault"), PollInterval: time.Hour,
		BrowserLogin: func(ctx context.Context, _ aphotos.RegionConfig, _ func(map[string]string) error) error {
			entered <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	for range 2 {
		done := make(chan error, 1)
		go func() { _, err := svc.Auth.Login(context.Background(), "us"); done <- err }()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("login did not start")
		}
		if _, err := svc.Auth.Login(context.Background(), "ca"); err == nil {
			t.Fatal("concurrent login allowed")
		}
		svc.Auth.CancelLogin()
		select {
		case err := <-done:
			var ge *gui.Error
			if !errors.As(err, &ge) || ge.Code != gui.ErrInterrupted {
				t.Fatalf("cancel error=%v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("cancel did not finish")
		}
	}
	status, err := svc.Auth.Status(context.Background())
	if err != nil || status.Configured {
		t.Fatalf("cancel saved session: %+v %v", status, err)
	}
}

func TestCloseCancelsBrowserWithoutWaitingForWindowCleanup(t *testing.T) {
	dir := t.TempDir()
	entered, release := make(chan struct{}), make(chan struct{})
	svc, _, err := gui.Open(gui.Options{
		DBPath: filepath.Join(dir, "db"), VaultPath: filepath.Join(dir, "vault"), PollInterval: time.Hour,
		BrowserLogin: func(ctx context.Context, _ aphotos.RegionConfig, _ func(map[string]string) error) error {
			close(entered)
			<-ctx.Done()
			<-release
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := svc.Auth.Login(context.Background(), "us"); done <- err }()
	<-entered
	closed := make(chan struct{})
	go func() { svc.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("close waited for native window cleanup")
	}
	close(release)
	if err := <-done; err == nil {
		t.Fatal("closed app accepted login")
	}
}
