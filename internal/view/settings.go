package view

import (
	"context"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const (
	repoURL       = "https://github.com/thedavidweng/rainforest"
	controlWidth  = 180
	maxUploads    = 8
	maxMetaChecks = 16
)

type settingsState struct {
	signOutAsk  bool
	diagWrite   bool
	diagRunning bool
	diag        *gui.DoctorReportDTO
	diagErr     string
}

func (a *App) settingsPage(c *ui.Context, r *ui.Route) {
	m := tr(c)
	r.Title(m.SettingsTitle())
	pageFrame(c, m.SettingsTitle(), m.SettingsSubtitle(), func() {
		if !a.signedIn() && secondaryButton(c, nil, m.CommonClose(), false).Clicked() {
			a.router.Replace("/upload")
		}
	}, func() {
		section(c, m.LanguageTitle(), func() { card(c, func() { settingRow(c, m.LanguageTitle(), "", func() { a.languageControl(c, m) }) }) })
		if a.signedIn() {
			section(c, m.SettingsAccount(), func() { a.accountCard(c, m) })
		}
		if a.settings != nil {
			section(c, m.SettingsTransfers(), func() { ui.Column(c).Disabled(a.preview).Children(func() { a.transfersCard(c, m) }) })
			section(c, m.SettingsComputer(), func() { ui.Column(c).Disabled(a.preview).Children(func() { a.computerCard(c, m) }) })
		}
		section(c, m.SettingsDiagnostics(), func() { ui.Column(c).Disabled(a.preview).Children(func() { a.diagnosticsCard(c, m) }) })
		section(c, m.SettingsAbout(), func() { a.aboutCard(c, m) })
	})
	a.signOutDialog(c, m)
}

func section(c *ui.Context, title string, body func()) {
	ui.Column(c).Gap(10).Children(func() {
		sectionTitle(c, title, nil)
		body()
	})
}

func (a *App) accountCard(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	card(c, func() {
		region := a.auth.Region
		if region == "" {
			region = a.welcomeRegion()
		}
		evidence := a.auth.Evidence
		if evidence == "" && a.settings != nil {
			evidence = a.settings.Evidence
		}
		ui.Row(c).Gap(12).Children(func() {
			ui.Box(c).Size(40, 40).Radius(20).Background(t.Text).Center().Children(func() {
				ui.Text(c, regionCode(region)).FontSize(13).FontWeight(600).TextColor(t.Background)
			})
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, regionHost(region)).FontWeight(600).SingleLine()
				ui.Text(c, regionName(m, region)).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			if evidence == "verified" {
				statusPill(c, m.SettingsVerified(), toneSuccess)
			} else {
				statusPill(c, m.SettingsExperimental(), toneWarning)
			}
			if secondaryButton(c, nil, m.SettingsSignOut(), a.preview).Clicked() {
				a.settingsUI.signOutAsk = true
			}
		})
		ui.Divider(c)
		a.usageGrid(c, m)
	})
}

func (a *App) usageGrid(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	switch a.usageState {
	case loadFailed:
		ui.Text(c, m.SettingsUsageUnavailable()).TextColor(t.TextMuted)
		return
	case loadDone:
	default:
		ui.Row(c).Gap(8).Padding(4, 0).Children(func() {
			ui.Spinner(c).Label(m.CommonLoading())
			ui.Text(c, m.CommonLoading()).TextColor(t.TextMuted)
		})
		return
	}
	u := a.usage
	billableItems := u.Photo.Billable.Count + u.Video.Billable.Count + u.Other.Billable.Count + u.Doc.Billable.Count
	ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
		usageTile(c, m.SettingsUsagePhotos(), u.Photo.Total.Bytes, u.Photo.Total.Count)
		usageTile(c, m.SettingsUsageVideos(), u.Video.Total.Bytes, u.Video.Total.Count)
		usageTile(c, m.SettingsUsageTotal(), u.TotalBytes, u.TotalItems)
		usageTile(c, m.SettingsUsageBillable(), u.TotalBillableBytes, billableItems)
	})
}

func usageTile(c *ui.Context, label string, bytes, count int64) {
	t := c.Theme()
	m := tr(c)
	ui.Column(c).Grow(1).Basis(0).Gap(2).Padding(14).Radius(radiusFeat).Background(t.Surface).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).SingleLine()
		ui.Text(c, formatBytes(bytes)).FontSize(20).FontWeight(700).FontFeatures("tnum").SingleLine()
		ui.Text(c, m.SettingsUsageCount(int(count))).FontSize(12).TextColor(t.TextMuted).FontFeatures("tnum").SingleLine()
	})
}

func settingRow(c *ui.Context, title, help string, control func()) {
	t := c.Theme()
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
			ui.Text(c, title)
			if help != "" {
				ui.Text(c, help).FontSize(12).TextColor(t.TextMuted)
			}
		})
		control()
	})
}

func (a *App) transfersCard(c *ui.Context, m locales.Messages) {
	s := a.settings
	card(c, func() {
		upload := float64(s.MaxUploadConcurrency)
		settingRow(c, m.SettingsUploadConcurrency(), m.SettingsUploadConcurrencyHelp(), func() {
			if stepper(c, &upload, 1, maxUploads, 1, m.SettingsUploadConcurrency()) {
				v := int(upload)
				s.MaxUploadConcurrency = v
				a.updateSettings(gui.SettingsInput{MaxUploadConcurrency: &v})
			}
		})
		ui.Divider(c)
		checks := float64(s.MaxMetadataConcurrency)
		settingRow(c, m.SettingsMetadataConcurrency(), "", func() {
			if stepper(c, &checks, 1, maxMetaChecks, 1, m.SettingsMetadataConcurrency()) {
				v := int(checks)
				s.MaxMetadataConcurrency = v
				a.updateSettings(gui.SettingsInput{MaxMetadataConcurrency: &v})
			}
		})
		ui.Divider(c)
		policies := []string{"skip", "rename", "fail"}
		labels := make([]string, len(policies))
		current := ""
		for i, p := range policies {
			labels[i] = conflictLabel(m, p, false)
			if p == s.DefaultConflictPolicy {
				current = labels[i]
			}
		}
		settingRow(c, m.OptionIfExists(), "", func() {
			sel := outlined(c, ui.Select(c, &current, labels)).Width(controlWidth).Label(m.OptionIfExists())
			if sel.Changed() {
				for i, l := range labels {
					if l == current {
						p := policies[i]
						s.DefaultConflictPolicy = p
						a.updateSettings(gui.SettingsInput{DefaultConflictPolicy: &p})
					}
				}
			}
		})
	})
}

func (a *App) computerCard(c *ui.Context, m locales.Messages) {
	s := a.settings
	keepRunning := m.SettingsKeepRunningTray()
	if runtime.GOOS == "darwin" {
		keepRunning = m.SettingsKeepRunningMenuBar()
	}
	card(c, func() {
		a.switchRow(c, m.SettingsOpenAtLogin(), "", &s.LaunchAtLogin, func(v bool) gui.SettingsInput {
			return gui.SettingsInput{LaunchAtLogin: &v}
		})
		ui.Divider(c)
		a.switchRow(c, keepRunning, "", &s.CloseToTray, func(v bool) gui.SettingsInput {
			return gui.SettingsInput{CloseToTray: &v}
		})
		ui.Divider(c)
		a.switchRow(c, m.SettingsNotifications(), m.SettingsNotificationsHelp(), &s.NotificationsEnabled, func(v bool) gui.SettingsInput {
			return gui.SettingsInput{NotificationsEnabled: &v}
		})
	})
}

func (a *App) switchRow(c *ui.Context, title, help string, value *bool, input func(bool) gui.SettingsInput) {
	settingRow(c, title, help, func() {
		if ui.Switch(c, value).Label(title).Changed() {
			a.updateSettings(input(*value))
		}
	})
}

func (a *App) diagnosticsCard(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	w := &a.settingsUI
	card(c, func() {
		ui.Text(c, m.SettingsDiagnosticsBody()).TextColor(t.TextMuted)
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c, &w.diagWrite, m.SettingsDiagnosticsWrite()).Disabled(w.diagRunning)
			ui.Spacer(c)
			if w.diagRunning {
				ui.Spinner(c).Label(m.SettingsDiagnosticsRunning())
				ui.Text(c, m.SettingsDiagnosticsRunning()).TextColor(t.TextMuted)
			} else if secondaryButton(c, nil, m.SettingsRunDiagnostics(), false).Clicked() {
				a.runDiagnostics()
			}
		})
		if w.diagErr != "" {
			ui.Text(c, w.diagErr).TextColor(t.Danger)
		}
		if w.diag != nil {
			ui.Divider(c)
			a.doctorResults(c, m, w.diag)
		}
	})
}

func (a *App) doctorResults(c *ui.Context, m locales.Messages, report *gui.DoctorReportDTO) {
	t := c.Theme()
	ui.Column(c).Gap(12).Children(func() {
		for _, check := range report.Checks {
			label, icon, k := doctorStatus(m, check.Status)
			ui.Row(c).Gap(10).AlignItems(ui.Start).Children(func() {
				ui.Icon(c, icon).FontSize(16).TextColor(toneColor(c, k)).Margin(1, 0, 0, 0).Tooltip(label)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, checkTitle(check.Name)).FontWeight(600)
					if check.Message != "" {
						ui.Text(c, check.Message).FontSize(12).TextColor(t.TextMuted)
					}
					if check.Hint != "" {
						ui.Text(c, check.Hint).FontSize(12).TextColor(t.Warning)
					}
				})
				ui.Text(c, m.SettingsLevel(check.Grade)).FontSize(12).TextColor(t.TextMuted).FontFeatures("tnum")
			})
		}
	})
}

func (a *App) runDiagnostics() {
	w := &a.settingsUI
	w.diagRunning, w.diagErr = true, ""
	write := w.diagWrite
	a.do(func(ctx context.Context) func() {
		var report *gui.DoctorReportDTO
		var err error
		if write {
			report, err = a.svc.Settings.DoctorWithWriteProbe(ctx, true)
		} else {
			report, err = a.svc.Settings.Doctor(ctx)
		}
		return func() {
			w.diagRunning = false
			if err != nil {
				w.diagErr = errMessage(err)
				return
			}
			w.diag = report
		}
	})
}

func (a *App) aboutCard(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	card(c, func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			a.logoTile(c, 32)
			ui.Column(c).Children(func() {
				ui.Text(c, "Rainforest").FontSize(15).FontWeight(600)
				if a.version != "" {
					ui.Text(c, m.SettingsVersion(a.version)).FontSize(12).TextColor(t.TextMuted)
				}
			})
		})
		ui.Text(c, m.AboutCopyright()).FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, m.SettingsAboutDisclaimer()).TextColor(t.TextMuted)
		ui.Row(c).Gap(6).Children(func() {
			ui.Text(c, m.SettingsLicense()).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, "·").FontSize(12).TextColor(t.TextMuted)
			ui.Link(c, m.SettingsSourceCode(), repoURL).FontSize(12)
		})
	})
}

func (a *App) signOutDialog(c *ui.Context, m locales.Messages) {
	w := &a.settingsUI
	if ui.AlertDialog(c, &w.signOutAsk, m.SettingsSignOutTitle(), m.SettingsSignOutBody(), m.CommonCancel(), m.SettingsSignOut()) != 1 {
		return
	}
	a.do(func(ctx context.Context) func() {
		st, err := a.svc.Auth.Logout(ctx)
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.setAuth(*st)
		}
	})
}

func checkTitle(name string) string {
	spaced := strings.ReplaceAll(name, "_", " ")
	if spaced == "" {
		return spaced
	}
	return strings.ToUpper(spaced[:1]) + spaced[1:]
}

func (a *App) languageControl(c *ui.Context, m locales.Messages) {
	tags := []string{"en", "zh-Hans", ""}
	labels := []string{"English", "简体中文", m.LanguageSystem()}
	current := m.LanguageSystem()
	for i, tag := range tags {
		if tag == c.Locale() {
			current = labels[i]
		}
	}
	if outlined(c, ui.Select(c, &current, labels)).Label(m.LanguageTitle()).Width(controlWidth).Changed() {
		for i, label := range labels {
			if current == label {
				a.setLocale(tags[i])
				break
			}
		}
	}
}

func (a *App) aboutDialog(c *ui.Context) {
	m := tr(c)
	ui.DialogBase(c, &a.aboutOpen, func(backdrop, panel ui.Element) {
		dialogFrame(c, backdrop, panel, 480)
		ui.Column(c).Padding(24).Gap(16).Children(func() {
			ui.Text(c, m.MenuAbout()).FontSize(20).FontWeight(700)
			a.aboutCard(c, m)
			if secondaryButton(c, nil, m.CommonClose(), false).Clicked() {
				a.aboutOpen = false
			}
		})
	})
}
