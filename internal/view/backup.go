package view

import (
	"cmp"
	"context"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const (
	scheduleImmediate = "immediate"
	scheduleInterval  = "interval"

	defaultIntervalMinutes = 30
	minIntervalMinutes     = 5
	maxIntervalMinutes     = 1440
	ruleListWidth          = 250
)

type folderForm struct {
	ruleID   string
	paused   bool
	path     string
	videos   bool
	keep     bool
	schedule string
	minutes  float64
	conflict string
	nodeID   string
	err      string
	saving   bool
}

type backupState struct {
	open       bool
	form       folderForm
	removeOpen bool
	removeRule gui.BackupRuleDTO
	selected   string
	split      float32
}

func (a *App) backupPage(c *ui.Context, r *ui.Route) {
	m := tr(c)
	r.Title(m.BackupTitle())
	if a.rulesLoaded && len(a.rules) > 0 {
		ui.Column(c).Grow(1).Children(func() {
			pageHeader(c, m.BackupTitle(), m.BackupSubtitle(), func() { a.backupActions(c, m) })
			a.backupSplit(c, m)
		})
	} else {
		pageFrame(c, m.BackupTitle(), m.BackupSubtitle(), func() { a.backupActions(c, m) }, func() { a.backupEmpty(c, m) })
	}
	a.folderDialog(c, m)
	a.removeDialog(c, m)
}

func (a *App) backupActions(c *ui.Context, m locales.Messages) {
	if len(a.rules) == 0 {
		return
	}
	label, stop := m.BackupPauseAll(), true
	if a.backupsStopped() {
		label, stop = m.BackupResumeAll(), false
	}
	if secondaryButton(c, nil, label, a.settings == nil).Clicked() {
		a.setBackupsStopped(stop)
	}
	if primaryButton(c, iconPlus, m.BackupAddFolder(), false).Clicked() {
		a.addFolder()
	}
}

func (a *App) backupEmpty(c *ui.Context, m locales.Messages) {
	if !a.rulesLoaded {
		ui.Row(c).Justify(ui.Center).Padding(40, 0).Children(func() {
			ui.Spinner(c).Label(m.CommonLoading())
		})
		return
	}
	emptyState(c, iconFolder, m.BackupEmptyTitle(), m.BackupEmptyBody(), m.BackupAddFolder(), a.addFolder)
}

func (a *App) selectedRule() gui.BackupRuleDTO {
	for i := range a.rules {
		if a.rules[i].ID == a.backup.selected {
			return a.rules[i]
		}
	}
	a.backup.selected = a.rules[0].ID
	return a.rules[0]
}

func (a *App) backupSplit(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	b := &a.backup
	if b.split == 0 {
		b.split = ruleListWidth
	}
	stopped := a.backupsStopped()
	selected := a.selectedRule()
	ui.Column(c).Grow(1).Gap(12).Padding(0, pagePadX, pagePadX/2).Children(func() {
		if stopped {
			banner(c, toneWarning, iconPauseCircle, m.BackupAllPaused(), "", m.CommonResume(), func() { a.setBackupsStopped(false) })
		}
		ui.Row(c).Gap(8).Children(func() {
			ui.Icon(c, iconInfo).FontSize(14).TextColor(t.TextMuted).Margin(2, 0, 0, 0)
			ui.Text(c, m.BackupOneWayNote()).TextColor(t.TextMuted).Grow(1)
		})
		ui.Split(c, &b.split, func() {
			a.ruleList(c, m, stopped)
		}, func() {
			a.ruleDetail(c.Key(selected.ID), m, selected, stopped)
		}).Grow(1)
	})
}

func (a *App) ruleList(c *ui.Context, m locales.Messages, stopped bool) {
	t := c.Theme()
	ui.Column(c).Grow(1).MinWidth(0).Padding(0, 12, 0, 0).Gap(6).Children(func() {
		ui.Text(c, m.BackupFolders()).FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(0, 10)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for i := range a.rules {
					a.ruleRow(c.Key(a.rules[i].ID), m, a.rules[i], stopped)
				}
			})
		})
	})
}

func (a *App) ruleRow(c *ui.Context, m locales.Messages, rule gui.BackupRuleDTO, stopped bool) {
	t := c.Theme()
	label, tn := ruleStatus(m, rule, stopped)
	selected := rule.ID == a.backup.selected
	halted := rule.Paused || stopped || rule.Status == "source_unavailable"
	b := ui.ButtonBase(c).Label(filepath.Base(rule.LocalPath)).Radius(radiusFeat).Padding(10).Gap(10).
		AlignItems(ui.Center).Cursor(ui.CursorPointer).Tooltip(rule.LocalPath)
	switch {
	case selected:
		b.Background(t.Surface)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	fg := t.Text
	if halted {
		fg = t.TextMuted
	}
	b.Children(func() {
		ui.Icon(c, iconFolder).FontSize(22).TextColor(fg).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, filepath.Base(rule.LocalPath)).FontWeight(600).TextColor(fg).SingleLine()
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(toneColor(c, tn)).Shrink(0)
				ui.Text(c, label).FontSize(12).TextColor(t.TextMuted).SingleLine()
			})
			if rule.Status == "scanning" {
				ui.Progress(c, -1).Height(3)
			}
		})
	})
	if b.Clicked() {
		a.backup.selected = rule.ID
	}
}

func (a *App) ruleDetail(c *ui.Context, m locales.Messages, rule gui.BackupRuleDTO, stopped bool) {
	t := c.Theme()
	unavailable := rule.Status == "source_unavailable"
	status, tn := ruleStatus(m, rule, stopped)
	ui.Column(c).Grow(1).MinWidth(0).Gap(12).Padding(0, 0, 0, 12).Children(func() {
		ui.Row(c).Wrap().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(200).Gap(2).Children(func() {
				ui.Text(c, filepath.Base(rule.LocalPath)).FontSize(20).FontWeight(700).SingleLine()
				ui.Text(c, rule.LocalPath).FontSize(12).TextColor(t.TextMuted).SingleLine().Selectable()
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
				statusPill(c, status, tn)
				busy := rule.Status == "scanning" || rule.Paused || stopped
				if secondaryButton(c, nil, m.BackupNow(), busy).Clicked() {
					a.scanNow(rule.ID)
				}
				iconButton(c, iconDotsThree, m.CommonMoreActions()).Menu(func(menu *ui.Menu) {
					a.ruleMenu(menu, m, rule)
				})
			})
		})
		ui.Text(c, ruleMeta(c, m, rule)).FontSize(13).TextColor(t.TextMuted)
		if rule.DestinationNodeID != "" {
			a.ruleDestination(c, m, rule.DestinationNodeID)
		}
		a.ruleBanners(c, m, rule, unavailable, stopped)
		a.filesPanel(c, m, rule)
	})
}

func (a *App) ruleBanners(c *ui.Context, m locales.Messages, rule gui.BackupRuleDTO, unavailable, stopped bool) {
	if unavailable {
		banner(c, toneWarning, iconWarning, "", m.BackupUnavailable(), "", nil)
	}
	if rule.Paused && !stopped {
		banner(c, toneWarning, iconPauseCircle, m.BackupRulePaused(), "", m.CommonResume(), func() { a.setRulePaused(rule.ID, false) })
	}
	if rule.LastError != "" {
		text, detail := friendlyError(m, "", rule.LastError)
		banner(c, toneDanger, iconWarningCircle, "", text, "", nil).Tooltip(cmp.Or(detail, text))
	}
}

func ruleMeta(c *ui.Context, m locales.Messages, rule gui.BackupRuleDTO) string {
	media := m.BackupMediaPhotos()
	if rule.IncludeVideos {
		media = m.BackupMediaAll()
	}
	schedule := m.BackupScheduleImmediate()
	if rule.ScheduleMode == scheduleInterval {
		schedule = m.BackupEveryMinutes(rule.IntervalMinutes)
	}
	structure := m.BackupFlat()
	if rule.KeepFolderStructure {
		structure = m.BackupKeepsFolders()
	}
	checked := m.BackupNotChecked()
	if when := relativeTime(c, rule.LastScanAt); when != "" {
		checked = m.BackupChecked(when)
	}
	return strings.Join([]string{media, schedule, structure, checked}, " · ")
}

func (a *App) ruleDestination(c *ui.Context, m locales.Messages, nodeID string) {
	t := c.Theme()
	st := a.probeFor(nodeID)
	label, tn := probeStatus(m, st)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, m.DestUploadsTo(a.destinationPath(m, nodeID, ""))).FontSize(13).TextColor(t.TextMuted).SingleLine().MinWidth(0)
		statusPill(c, label, tn).Shrink(0)
	})
}

func (a *App) ruleMenu(menu *ui.Menu, m locales.Messages, rule gui.BackupRuleDTO) {
	toggle := m.CommonPause()
	if rule.Paused {
		toggle = m.CommonResume()
	}
	if menu.Item(toggle).Chosen() {
		a.setRulePaused(rule.ID, !rule.Paused)
	}
	if menu.Item(m.CommonEdit()).Chosen() {
		a.openFolderForm(&rule, rule.LocalPath)
	}
	menu.Separator()
	if menu.Item(m.BackupRemoveMenu()).Chosen() {
		a.backup.removeRule, a.backup.removeOpen = rule, true
	}
}

func (a *App) removeDialog(c *ui.Context, m locales.Messages) {
	b := &a.backup
	name := filepath.Base(b.removeRule.LocalPath)
	if ui.AlertDialog(c, &b.removeOpen, m.BackupRemoveTitle(name), m.BackupRemoveBody(), m.CommonCancel(), m.CommonRemove()) == 1 {
		a.removeRule(b.removeRule.ID)
	}
}

func (a *App) setBackupsStopped(stopped bool) {
	a.do(func(ctx context.Context) func() {
		err := a.svc.Backups.SetAllStopped(ctx, stopped)
		return func() {
			if err != nil {
				a.toastError(err)
			}
			a.refreshSettings()
			a.refreshBackups()
		}
	})
}

func (a *App) setRulePaused(id string, paused bool) {
	a.do(func(ctx context.Context) func() {
		var rule *gui.BackupRuleDTO
		var err error
		if paused {
			rule, err = a.svc.Backups.Pause(ctx, id)
		} else {
			rule, err = a.svc.Backups.Resume(ctx, id)
		}
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.upsertRule(*rule)
		}
	})
}

func (a *App) removeRule(id string) {
	a.do(func(ctx context.Context) func() {
		err := a.svc.Backups.Remove(ctx, id)
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.removeRuleLocal(id)
		}
	})
}

func (a *App) scanNow(id string) {
	a.do(func(ctx context.Context) func() {
		t, err := a.svc.Backups.ScanNow(ctx, id)
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.refreshBackups()
			if t == nil {
				return
			}
			a.upsertTransfer(*t)
			a.toastAction(locales.ToastBackupStarted(), locales.CommonView(), func() { a.router.Push("/activity") })
		}
	})
}

func (a *App) pickBackupFolder(then func(path string)) {
	a.do(func(ctx context.Context) func() {
		path, err := a.svc.Backups.PickFolder(ctx)
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			if path != "" {
				then(path)
			}
		}
	})
}

func (a *App) addFolder() {
	a.pickBackupFolder(func(path string) { a.openFolderForm(nil, path) })
}

func (a *App) openFolderForm(rule *gui.BackupRuleDTO, path string) {
	f := folderForm{
		path:     path,
		videos:   true,
		keep:     true,
		schedule: scheduleImmediate,
		minutes:  defaultIntervalMinutes,
		conflict: "skip",
	}
	if a.settings != nil && a.settings.DefaultConflictPolicy != "" {
		f.conflict = a.settings.DefaultConflictPolicy
	}
	if rule != nil {
		f.ruleID, f.paused = rule.ID, rule.Paused
		f.videos, f.schedule = rule.IncludeVideos, rule.ScheduleMode
		if rule.ConflictPolicy != "" {
			f.conflict = rule.ConflictPolicy
		}
		if rule.IntervalMinutes > 0 {
			f.minutes = float64(rule.IntervalMinutes)
		}
		if rule.DestinationNodeID != "" {
			f.nodeID = rule.DestinationNodeID
			a.probeFor(f.nodeID)
		}
	}
	a.backup.form = f
	a.backup.open = true
}

func (a *App) saveFolder(m locales.Messages) {
	f := &a.backup.form
	path := strings.TrimSpace(f.path)
	switch {
	case path == "":
		f.err = m.BackupErrorFolder()
		return
	case f.schedule == scheduleInterval && (f.minutes < minIntervalMinutes || f.minutes > maxIntervalMinutes):
		f.err = m.BackupErrorInterval()
		return
	}
	keep := f.keep
	in := gui.BackupRuleInput{
		KeepFolderStructure: &keep,
		LocalPath:           path,
		IncludeVideos:       f.videos,
		ScheduleMode:        f.schedule,
		IntervalMinutes:     int(f.minutes),
		ConflictPolicy:      f.conflict,
		DestinationNodeID:   strings.TrimSpace(f.nodeID),
		Paused:              f.paused,
	}
	id := f.ruleID
	f.err, f.saving = "", true
	a.do(func(ctx context.Context) func() {
		var rule *gui.BackupRuleDTO
		var err error
		if id == "" {
			rule, err = a.svc.Backups.Add(ctx, in)
		} else {
			rule, err = a.svc.Backups.Update(ctx, id, in)
		}
		return func() {
			a.backup.form.saving = false
			if err != nil {
				a.backup.form.err = errMessage(err)
				return
			}
			a.upsertRule(*rule)
			a.backup.open = false
		}
	})
}

func (a *App) folderDialog(c *ui.Context, m locales.Messages) {
	b := &a.backup
	ui.DialogBase(c, &b.open, func(backdrop, panel ui.Element) {
		dialogFrame(c, backdrop, panel, 520)
		ui.Column(c).Children(func() {
			title := m.BackupAddFolder()
			if b.form.ruleID != "" {
				title = m.BackupEditFolder()
			}
			ui.Text(c, title).FontSize(20).FontWeight(700).Padding(24, 28, 4)
			ui.Column(c).Gap(16).Padding(12, 28, 20).Children(func() {
				a.folderFields(c, m)
			})
			a.folderFooter(c, m)
		})
	})
}

func (a *App) folderFields(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	f := &a.backup.form
	ui.Form(c, func() {
		ui.Field(c, m.BackupFolder(), func() {
			ui.Row(c).Gap(8).Children(func() {
				outlined(c, ui.TextInput(c, &f.path)).ReadOnly(true).Grow(1).MinWidth(0)
				if secondaryButton(c, nil, m.CommonChoose(), false).Clicked() {
					a.pickBackupFolder(func(path string) { f.path, f.err = path, "" })
				}
			})
		})
		ui.Field(c, m.ReviewUploadTo(), func() {
			a.destinationRow(c, m, a.destinationPath(m, f.nodeID, ""), func() {
				a.openPicker(f.nodeID, func(id string) { f.nodeID = id })
			}, nil)
		})
		ui.Field(c, m.OptionIncludeVideos(), func() { ui.Switch(c, &f.videos) })
		ui.Field(c, m.OptionKeepStructure(), func() {
			a.structureSwitch(c, m, &f.keep, a.folderStructureSupported(f.nodeID))
		})
		ui.Field(c, m.BackupBackUp(), func() {
			codeSelect(c.Key("schedule"), &f.schedule, []string{scheduleImmediate, scheduleInterval}, func(code string) string {
				if code == scheduleInterval {
					return m.BackupScheduleInterval()
				}
				return m.BackupScheduleImmediate()
			})
		})
		if f.schedule == scheduleInterval {
			ui.Field(c, m.BackupEvery(), func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.NumberInput(c, &f.minutes, minIntervalMinutes, maxIntervalMinutes, 5).Width(150).Shrink(0).FontFeatures("tnum")
					ui.Text(c, m.BackupMinutes()).TextColor(t.TextMuted).Margin(0, 0, 0, 4)
				})
			})
		}
		ui.Field(c, m.OptionIfExists(), func() {
			codes := []string{"skip", "rename"}
			if f.conflict == "fail" {
				codes = append(codes, "fail")
			}
			codeSelect(c.Key("conflict"), &f.conflict, codes, func(code string) string { return conflictLabel(m, code, false) })
		})
	})
}

func (a *App) folderFooter(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	b := &a.backup
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 28, 24).Children(func() {
		ui.Text(c, b.form.err).TextColor(t.Danger).FontSize(12).Grow(1)
		if secondaryButton(c, nil, m.CommonCancel(), false).Clicked() {
			b.open = false
		}
		confirm := m.BackupAddFolder()
		if b.form.ruleID != "" {
			confirm = m.BackupSaveChanges()
		}
		if primaryButton(c, nil, confirm, b.form.saving).Clicked() {
			a.saveFolder(m)
		}
	})
}

func codeSelect(c *ui.Context, value *string, codes []string, label func(string) string) {
	labels := make([]string, len(codes))
	current := ""
	for i, code := range codes {
		labels[i] = label(code)
		if code == *value {
			current = labels[i]
		}
	}
	if outlined(c, ui.Select(c, &current, labels)).Changed() {
		for i := range labels {
			if labels[i] == current {
				*value = codes[i]
			}
		}
	}
}

func (a *App) folderStructureSupported(nodeID string) bool {
	if nodeID == "" {
		return true
	}
	st := a.probeFor(nodeID)
	return st.state != loadDone || st.capab.StructureSupported
}
