package view

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const testCookies = "session_id=140-1111111-2222222; ubid_main=130-3333333-4444444; at_main=Atza|ValidToken123"

func newLoginHarness(t *testing.T, login func(context.Context) error) *harness {
	t.Helper()
	dir := t.TempDir()
	svc, closeSvc, err := gui.Open(gui.Options{
		DBPath:    filepath.Join(dir, "state.db"),
		VaultPath: filepath.Join(dir, "credentials.json"),
		Transport: stubTransport(),
		BrowserLogin: func(ctx context.Context, _ aphotos.RegionConfig, _ func(map[string]string) error) error {
			return login(ctx)
		},
	})
	if err != nil {
		t.Fatalf("gui.Open: %v", err)
	}
	t.Cleanup(closeSvc)
	return newHarnessFor(t, svc)
}

func TestWelcomeShowsMarketplaceAndFooter(t *testing.T) {
	h := newHarness(t, false)
	h.start()
	h.wantText(locales.WelcomeMarketplace())
	h.wantText(regionLabel(locales.In("en"), "us"))
	h.wantText(locales.WelcomeSignIn())
	h.wantText(locales.WelcomeFooter())
	h.noText(locales.WelcomeExperimental())
}

func TestWelcomeWarnsForExperimentalMarketplace(t *testing.T) {
	h := newHarness(t, false)
	h.start()
	h.app.welcomeUI.region = "jp"
	h.app.welcomeUI.picked = true
	h.tt.Frame()
	h.wantText(locales.WelcomeExperimental())
}

func TestWelcomeSessionExpiredBanner(t *testing.T) {
	h := newHarness(t, false)
	h.start()
	h.app.setAuth(gui.AuthStatus{Configured: true, ErrorCode: gui.ErrAuthFailed, ErrorMessage: "raw server text"})
	h.drain()
	h.wantText(locales.ErrorSessionExpired())
	h.noText("raw server text")
}

func TestWelcomeSignInWithoutBrowserShowsError(t *testing.T) {
	h := newHarness(t, false)
	h.start()
	if err := h.tt.Click(locales.WelcomeSignIn()); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	h.wantText(locales.WelcomeSigningIn())
	h.drain()
	h.noText(locales.WelcomeSigningIn())
	h.wantText("Web sign-in is available in the Rainforest desktop app.")
}

func TestWelcomeInterruptedSignInShowsCancelledNote(t *testing.T) {
	h := newLoginHarness(t, func(ctx context.Context) error { return errors.Join(context.Canceled, ctx.Err()) })
	h.start()
	if err := h.tt.Click(locales.WelcomeSignIn()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.WelcomeCancelled())
	h.wantText(locales.WelcomeSignIn())
}

func TestWelcomeCancelStopsPendingSignIn(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	h := newLoginHarness(t, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	})
	h.start()
	if err := h.tt.Click(locales.WelcomeSignIn()); err != nil {
		t.Fatal(err)
	}
	h.tt.Frame()
	signIn, cancel := h.queue[0], h.queue[1:]
	h.queue = nil
	done := make(chan struct{})
	go func() { signIn(); close(done) }()
	<-started

	if err := h.tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	cancel = append(cancel, h.queue...)
	h.queue = nil
	for _, fn := range cancel {
		fn()
	}
	<-cancelled
	<-done
	h.drain()
	h.wantText(locales.WelcomeCancelled())
}

func TestWelcomeCookieImportNeedsPastedText(t *testing.T) {
	h := newHarness(t, false)
	h.start()
	h.noText(locales.WelcomeImportPasted())
	if err := h.tt.Click(locales.WelcomeOtherWays()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.WelcomeImportPasted())
	h.wantText(locales.WelcomeImportHelp())

	if err := h.tt.Click(locales.WelcomeImportPasted()); err != nil {
		t.Fatal(err)
	}
	if len(h.queue) != 0 {
		t.Fatalf("import started with nothing pasted: %d jobs", len(h.queue))
	}

	h.tt.Frame()
	help, ok := h.tt.Find(locales.WelcomeImportHelp())
	if !ok {
		t.Fatal("no import help")
	}
	h.tt.ClickAt(help.X+20, help.Y+help.H+30)
	h.tt.Type(testCookies)
	h.tt.Frame()
	if err := h.tt.Click(locales.WelcomeImportPasted()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	h.wantText(locales.NavUpload())
	h.noText(locales.WelcomeTitle())
}

func TestWelcomeRendersInChinese(t *testing.T) {
	h := newHarness(t, false)
	h.tt.SetLocale("zh-Hans")
	h.start()
	zh := locales.In("zh-Hans")
	h.wantText(zh.WelcomeSignIn())
	h.wantText(zh.WelcomeFooter())
	h.wantText(regionLabel(zh, "us"))
}

func TestWelcomeLanguageChangePreservesMarketplace(t *testing.T) {
	h := newHarness(t, false)
	h.start()
	h.app.welcomeUI.region, h.app.welcomeUI.picked = "jp", true
	h.tt.SetLocale("zh-Hans")
	h.drain()
	h.wantText(regionLabel(locales.In("zh-Hans"), "jp"))
	if h.app.welcomeUI.region != "jp" {
		t.Fatal("marketplace changed")
	}
}
