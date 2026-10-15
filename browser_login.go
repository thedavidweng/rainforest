package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/locales"
)

func browserLogin(ctx context.Context, rc aphotos.RegionConfig, verify func(map[string]string) error) (retErr error) {
	if err := clearAmazonCookies(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	win := mygo.NewWindow(mygo.WindowOptions{Title: locales.BrowserLoginTitle(), URL: "about:blank", Width: 540, Height: 760, Page: mygo.PageOptions{DevTools: mygo.DevToolsDisabled}})
	defer func() { win.Destroy(); retErr = errors.Join(retErr, clearAmazonCookies()) }()
	win.OnClosed(cancel)
	win.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		u, err := url.Parse(e.URL)
		if err != nil || u.Scheme != "https" || !amazonCookieDomain(u.Hostname()) {
			e.PreventDefault()
		}
	})
	ready := make(chan struct{}, 1)
	win.Page().OnDOMReady(func() {
		select {
		case ready <- struct{}{}:
		default:
		}
	})
	failures := make(chan error, 1)
	win.Page().OnDidFailLoad(func(_ *mygo.LoadError) {
		select {
		case failures <- errors.New(locales.LoginPageFailed()):
		default:
		}
	})
	if err := win.Page().LoadURL("https://www.amazon." + rc.Domain + "/photos"); err != nil {
		return err
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	loaded := false
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return errors.New(locales.LoginTimedOut())
			}
			return ctx.Err()
		case err := <-failures:
			return err
		case <-ready:
			loaded = true
		case <-ticker.C:
			if !loaded {
				continue
			}
			cookies, err := mygo.Cookies.List(mygo.CookieFilter{URL: rc.DriveURL + "/nodes"})
			if err != nil {
				return err
			}
			captured, err := loginCookies(rc, cookies)
			if err != nil {
				return err
			}
			if captured == nil {
				continue
			}
			err = verify(captured)
			var authErr *aphotos.AuthenticationError
			if errors.As(err, &authErr) {
				continue
			}
			return err
		}
	}
}

func clearAmazonCookies() error {
	cookies, err := mygo.Cookies.List(mygo.CookieFilter{})
	if err != nil {
		return err
	}
	for _, cookie := range cookies {
		if amazonCookieDomain(cookie.Domain) {
			if err := mygo.Cookies.Delete(cookie); err != nil {
				return err
			}
		}
	}
	return nil
}

func amazonCookieDomain(domain string) bool {
	prefix, suffix, ok := strings.Cut(strings.TrimPrefix(domain, "."), "amazon.")
	if !ok || (prefix != "" && !strings.HasSuffix(prefix, ".")) {
		return false
	}
	rc, err := aphotos.ResolveRegion(aphotos.Region(suffix))
	return err == nil && rc.Domain == suffix
}

func loginCookies(rc aphotos.RegionConfig, cookies []mygo.Cookie) (map[string]string, error) {
	out := make(map[string]string, len(rc.RequiredCookies))
	for _, key := range rc.RequiredCookies {
		for _, cookie := range cookies {
			if cookie.Name != key || cookie.Value == "" || (!cookie.Expires.IsZero() && !cookie.Expires.After(time.Now())) {
				continue
			}
			if prior := out[key]; prior != "" && prior != cookie.Value {
				return nil, errors.New(locales.ConflictingCookies())
			}
			out[key] = cookie.Value
		}
		if out[key] == "" {
			return nil, nil
		}
	}
	return out, nil
}
