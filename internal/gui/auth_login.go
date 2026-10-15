package gui

import (
	"context"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
)

func (a *Auth) Login(ctx context.Context, region string) (*AuthStatus, error) {
	rc, err := aphotos.ResolveRegion(aphotos.Region(region))
	if err != nil {
		return nil, toError(err)
	}
	if !a.mu.TryLock() {
		return nil, newError(ErrConflict, "Account change already in progress. Finish or cancel it first.", nil)
	}
	defer a.mu.Unlock()
	cs, err := a.state.credVault()
	if err != nil {
		return nil, err
	}
	a.state.mu.RLock()
	opts := a.state.opts
	a.state.mu.RUnlock()
	if opts.BrowserLogin == nil {
		return nil, newError(ErrConfigError, "Web sign-in is available in the Rainforest desktop app.", nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	a.loginMu.Lock()
	a.cancelLogin = cancel
	a.loginMu.Unlock()
	defer func() { cancel(); a.loginMu.Lock(); a.cancelLogin = nil; a.loginMu.Unlock() }()
	var verified *AuthStatus
	var credentials credstore.Credentials
	err = opts.BrowserLogin(ctx, rc, func(cookies map[string]string) error {
		creds, err := credstore.ValidateAndNormalize(cookies, string(rc.Region), credstore.HostOverrides{})
		if err != nil {
			return err
		}
		client, err := aphotos.NewClient(aphotos.Options{Region: rc.Region, Cookies: creds.Cookies, HTTPClient: opts.HTTPClient, Transport: opts.Transport})
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()
		probeCtx, cancelProbe := context.WithTimeout(ctx, 30*time.Second)
		defer cancelProbe()
		root, err := client.GetRoot(probeCtx)
		if err != nil {
			return err
		}
		credentials = creds
		verified = &AuthStatus{Configured: true, Authenticated: true, Region: string(rc.Region), Evidence: string(rc.Evidence), RootNodeID: root.ID, OwnerID: root.OwnerID, CookieKeys: append([]string(nil), rc.RequiredCookies...), VaultPath: cs.Path()}
		return nil
	})
	if err != nil {
		return nil, toError(err)
	}
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	if err = ctx.Err(); err != nil {
		return nil, toError(err)
	}
	if verified == nil {
		return nil, newError(ErrAuthFailed, "Amazon Photos session was not verified. Sign in again.", nil)
	}
	if err = cs.Save(credentials); err != nil {
		return nil, toError(err)
	}
	if err := a.state.reopen(ctx); err != nil {
		return nil, err
	}
	a.state.emit(EventAuthChanged, *verified)
	return verified, nil
}

func (a *Auth) CancelLogin() {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	if a.cancelLogin != nil {
		a.cancelLogin()
	}
}
