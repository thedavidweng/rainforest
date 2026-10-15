package view

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const welcomeWidth = 460

type welcomeState struct {
	region      string
	picked      bool
	signingIn   bool
	cancelled   bool
	err         string
	cookiesOpen bool
	cookies     string
	importing   bool
}

func (a *App) welcomeRegion() string {
	if a.settings != nil && a.settings.Region != "" {
		return a.settings.Region
	}
	if a.auth.Region != "" {
		return a.auth.Region
	}
	return "us"
}

func regionOptions(m locales.Messages) []string {
	opts := make([]string, len(regionCodes))
	for i, code := range regionCodes {
		opts[i] = regionLabel(m, code)
	}
	return opts
}

func (w *welcomeState) regionCode(m locales.Messages) string {
	for _, code := range regionCodes {
		if regionLabel(m, code) == w.region {
			return code
		}
	}
	return "us"
}

func (a *App) welcome(c *ui.Context) {
	m := tr(c)
	t := c.Theme()
	w := &a.welcomeUI
	if !w.picked {
		w.region = regionLabel(m, a.welcomeRegion())
	}
	busy := w.signingIn || w.importing
	ui.Column(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).Center().Padding(max(c.TitleBar().Height, headerHeight), 0, 0).Children(func() {
			ui.Scroll(c).Shrink(1).Padding(8, 24, 16).Children(func() {
				ui.Column(c).Width(welcomeWidth).MaxWidth(welcomeWidth).AlignItems(ui.Stretch).Gap(20).Children(func() {
					ui.Column(c).AlignItems(ui.Center).Gap(20).Children(func() {
						a.logoTile(c, 64)
						ui.Column(c).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(c, m.WelcomeTitle()).FontSize(24).FontWeight(700).TextAlign(ui.Center)
							ui.Text(c, m.WelcomeBody()).TextColor(t.TextMuted).TextAlign(ui.Center)
						})
					})
					if a.auth.ErrorMessage != "" || a.auth.ErrorCode != "" {
						body := a.auth.ErrorMessage
						if isAuthError(a.auth.ErrorCode) || body == "" {
							body = m.ErrorSessionExpired()
						}
						banner(c, toneWarning, iconWarning, "", body, "", nil)
					}
					a.welcomeForm(c, m, w, busy)
					a.welcomeOtherWays(c, m, w, busy)
				})
			})
		})
		ui.Text(c, m.WelcomeFooter()).FontSize(12).TextColor(t.TextMuted).TextAlign(ui.Center).
			MaxWidth(680).AlignSelf(ui.Center).Padding(8, 24, 20)
	})
}

func (a *App) welcomeForm(c *ui.Context, m locales.Messages, w *welcomeState, busy bool) {
	t := c.Theme()
	ui.Column(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
		ui.Field(c, m.WelcomeMarketplace(), func() {
			sel := outlined(c, ui.Select(c, &w.region, regionOptions(m))).Disabled(busy)
			if sel.Changed() {
				w.picked = true
			}
		})
		if w.regionCode(m) != "us" {
			ui.Text(c, m.WelcomeExperimental()).FontSize(12).TextColor(t.Warning)
		}
	})
	ui.Column(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
		switch {
		case w.signingIn:
			a.welcomeProgress(c, m)
		case primaryButton(c, iconSignIn, m.WelcomeSignIn(), w.importing).Height(40).Clicked():
			a.signIn(w.regionCode(m))
		}
		switch {
		case w.err != "":
			ui.Text(c, w.err).FontSize(12).TextColor(t.Danger)
		case w.cancelled:
			ui.Text(c, m.WelcomeCancelled()).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (a *App) welcomeProgress(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	ui.Row(c).Height(36).Gap(10).Padding(0, 12).Radius(radiusPill).Background(t.Surface).Children(func() {
		ui.Spinner(c).Label(m.WelcomeSigningIn())
		ui.Text(c, m.WelcomeSigningIn()).Grow(1).SingleLine()
		if secondaryButton(c, nil, m.CommonCancel(), false).Clicked() {
			a.do(func(context.Context) func() {
				a.svc.Auth.CancelLogin()
				return nil
			})
		}
	})
}

func (a *App) welcomeOtherWays(c *ui.Context, m locales.Messages, w *welcomeState, busy bool) {
	t := c.Theme()
	ui.Collapsible(c, m.WelcomeOtherWays(), &w.cookiesOpen, func() {
		ui.Column(c).Gap(10).Padding(8, 0, 0).AlignItems(ui.Stretch).Children(func() {
			ui.Text(c, m.WelcomeImportHelp()).FontSize(12).TextColor(t.TextMuted)
			outlined(c, ui.TextArea(c, &w.cookies)).Font("monospace").FontSize(12).Lines(4, 4).
				Placeholder(m.WelcomeCookiesPlaceholder()).Label(m.WelcomeOtherWays()).Disabled(busy)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, nil, m.WelcomeImportFile(), busy).Clicked() {
					a.importCookieFile(w.regionCode(m))
				}
				pasted := secondaryButton(c, nil, m.WelcomeImportPasted(), busy || strings.TrimSpace(w.cookies) == "")
				if pasted.Clicked() {
					a.importCookies(w.regionCode(m), w.cookies)
				}
			})
		})
	})
}

func (a *App) signIn(region string) {
	w := &a.welcomeUI
	w.signingIn, w.cancelled, w.err = true, false, ""
	a.do(func(ctx context.Context) func() {
		st, err := a.svc.Auth.Login(ctx, region)
		return func() {
			w.signingIn = false
			if err != nil {
				a.setLoginError(err)
				return
			}
			a.setAuth(*st)
		}
	})
}

func (a *App) setLoginError(err error) {
	w := &a.welcomeUI
	if errCode(err) == gui.ErrInterrupted {
		w.cancelled = true
		return
	}
	w.err = errMessage(err)
}

func (a *App) importCookies(region, raw string) {
	a.runImport(func(ctx context.Context) (*gui.AuthStatus, error) {
		return a.svc.Auth.ImportCookies(ctx, gui.ImportCookiesInput{RawCookies: raw, Region: region})
	})
}

func (a *App) importCookieFile(region string) {
	a.runImport(func(ctx context.Context) (*gui.AuthStatus, error) {
		return a.svc.Auth.PickAndImportCookieFile(ctx, region)
	})
}

func (a *App) runImport(call func(context.Context) (*gui.AuthStatus, error)) {
	w := &a.welcomeUI
	w.importing, w.cancelled, w.err = true, false, ""
	a.do(func(ctx context.Context) func() {
		st, err := call(ctx)
		return func() {
			w.importing = false
			if err != nil {
				w.err = errMessage(err)
				return
			}
			if st.Authenticated {
				w.cookies = ""
			} else if st.ErrorMessage != "" {
				w.err = st.ErrorMessage
			}
			a.setAuth(*st)
		}
	})
}
