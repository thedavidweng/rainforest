package view

import (
	"cmp"
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const itemListMaxHeight = 280

type activityState struct {
	expanded         map[string]bool
	confirmClear     bool
	dismissedWarning string
}

func (a *App) activityPage(c *ui.Context, r *ui.Route) {
	m := tr(c)
	r.Title(m.ActivityTitle())
	st := &a.pages.activity
	var actions func()
	if len(a.history) > 0 {
		actions = func() {
			if secondaryButton(c, nil, m.ActivityClearHistory(), a.preview).Clicked() {
				st.confirmClear = true
			}
		}
	}
	pageFrame(c, m.ActivityTitle(), m.ActivitySubtitle(), actions, func() {
		a.activityBody(c, m)
	})
	if ui.AlertDialog(c, &st.confirmClear, m.ActivityClearTitle(), m.ActivityClearBody(), m.CommonCancel(), m.ActivityClearConfirm()) == 1 {
		a.clearHistory()
	}
}

func (a *App) activityBody(c *ui.Context, m locales.Messages) {
	st := &a.pages.activity
	if w := a.transferWarning; w != "" && w != st.dismissedWarning {
		banner(c, toneWarning, iconWarning, "", w, m.CommonDismiss(), func() { st.dismissedWarning = w })
	}
	if len(a.active) == 0 && len(a.history) == 0 {
		if a.transfersLoaded {
			emptyState(c, iconArrowsDownUp, m.ActivityEmptyTitle(), m.ActivityEmptyBody(), m.ActivityEmptyAction(), func() {
				a.router.Push("/upload")
			})
		}
		return
	}
	a.transferSection(c, m.ActivityInProgress(), a.active)
	a.transferSection(c, m.ActivityEarlier(), a.history)
}

func (a *App) transferSection(c *ui.Context, title string, list []gui.TransferDTO) {
	if len(list) == 0 {
		return
	}
	ui.Column(c).Gap(10).Children(func() {
		sectionTitle(c, title, nil)
		for i := range list {
			a.transferRow(c.Key(list[i].ID), list[i], false)
		}
	})
}

func (a *App) clearHistory() {
	a.do(func(ctx context.Context) func() {
		err := a.svc.Transfers.ClearHistory(ctx)
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			a.history = nil
		}
	})
}

func (a *App) transferCall(call func(ctx context.Context, id string) (*gui.TransferDTO, error), id string) {
	a.callTransfer(func(ctx context.Context) (*gui.TransferDTO, error) { return call(ctx, id) }, nil)
}

func (a *App) pauseOrResume(t gui.TransferDTO) {
	if t.Stage == "paused" {
		a.transferCall(a.svc.Transfers.Resume, t.ID)
		return
	}
	a.transferCall(a.svc.Transfers.Pause, t.ID)
}

func canRetry(t gui.TransferDTO) bool {
	return isTerminalStage(t.Stage) && t.Stage != "completed"
}

func transferFraction(t gui.TransferDTO) float64 {
	var f float64
	switch {
	case t.BytesTotal > 0:
		f = float64(t.BytesDone) / float64(t.BytesTotal)
	case t.ItemsTotal > 0:
		f = float64(t.ItemsDone+t.ItemsSkipped) / float64(t.ItemsTotal)
	}
	return min(max(f, 0), 1)
}

func (a *App) transferRow(c *ui.Context, t gui.TransferDTO, compact bool) {
	m := tr(c)
	th := c.Theme()
	st := &a.pages.activity
	active := !isTerminalStage(t.Stage)
	expanded := !compact && st.expanded[t.ID]
	body := func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Box(c).Size(40, 40).Shrink(0).Radius(radiusFeat).Background(th.Surface).Center().Children(func() {
				ui.Icon(c, kindIcon(t.Kind)).FontSize(22).TextColor(th.Text)
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, t.SourceSummary).FontWeight(600).SingleLine()
				a.transferMeta(c, m, t)
			})
			label, k := transferStatus(m, t.Kind, t.Stage)
			statusPill(c, label, k).Shrink(0)
			if !compact {
				a.transferActions(c, m, t, active, expanded)
			}
		})
		if active {
			ui.Row(c).Gap(10).Children(func() {
				ui.Progress(c, transferFraction(t)).Height(4).Radius(2).Grow(1).Label(stageLabel(m, t))
				ui.Text(c, fmt.Sprintf("%d%%", int(transferFraction(t)*100))).FontSize(12).
					TextColor(th.TextMuted).FontFeatures("tnum").Width(36).TextAlign(ui.End)
			})
		}
		if !compact {
			a.transferNote(c, m, t)
		}
		if expanded {
			a.itemList(c, m, t.ID)
		}
	}
	if compact {
		ui.Column(c).Padding(12, 14).Gap(8).Children(body)
		return
	}
	card(c, body).Padding(16, 20).Gap(10).
		ContextMenu(func(menu *ui.Menu) { a.transferMenu(menu, m, t, active) })
}

func stageLabel(m locales.Messages, t gui.TransferDTO) string {
	s, _ := transferStatus(m, t.Kind, t.Stage)
	return s
}

func (a *App) transferMeta(c *ui.Context, m locales.Messages, t gui.TransferDTO) {
	th := c.Theme()
	kind := kindLabel(m, t.Kind)
	var meta string
	if isTerminalStage(t.Stage) {
		stamp := t.FinishedAt
		if stamp == "" {
			stamp = t.UpdatedAt
		}
		meta = m.ActivityMetaDone(kind, t.ItemsTotal, formatBytes(t.BytesTotal), relativeTime(c, stamp))
	} else {
		meta = m.ActivityMetaActive(kind, t.ItemsDone+t.ItemsSkipped, t.ItemsTotal, formatBytes(t.BytesDone), formatBytes(t.BytesTotal))
	}
	ui.Row(c).Gap(5).Children(func() {
		ui.Text(c, meta).FontSize(12).TextColor(th.TextMuted).FontFeatures("tnum").SingleLine().Shrink(1)
		if t.ItemsFailed > 0 {
			ui.Text(c, "· "+m.ActivityFailedCount(t.ItemsFailed)).FontSize(12).TextColor(th.Danger).Shrink(0)
		}
		if t.ItemsUnconfirmed > 0 {
			ui.Text(c, "· "+m.ActivityNeedsCheckingCount(t.ItemsUnconfirmed)).FontSize(12).TextColor(th.Warning).Shrink(0)
		}
	})
}

func (a *App) transferNote(c *ui.Context, m locales.Messages, t gui.TransferDTO) {
	th := c.Theme()
	switch {
	case t.Stage == "unconfirmed":
		if t.ErrorMessage != "" {
			errorText(c, t.ErrorCode, t.ErrorMessage).FontSize(12).TextColor(th.Warning)
			return
		}
		note := m.ActivityUnconfirmed()
		if t.Kind == "download" {
			note = m.ActivityUnconfirmedDownload()
		}
		ui.Text(c, note).FontSize(12).TextColor(th.Warning)
	case t.ErrorMessage != "":
		errorText(c, t.ErrorCode, t.ErrorMessage).FontSize(12).TextColor(th.Danger)
	}
}

func (a *App) transferActions(c *ui.Context, m locales.Messages, t gui.TransferDTO, active, expanded bool) {
	st := &a.pages.activity
	ui.Row(c).Gap(4).Shrink(0).Children(func() {
		switch {
		case active:
			icon, name := iconPause, m.CommonPause()
			if t.Stage == "paused" {
				icon, name = iconPlay, m.CommonResume()
			}
			if iconButton(c, icon, name).Disabled(a.preview).Clicked() {
				a.pauseOrResume(t)
			}
			if iconButton(c, iconX, m.ActivityCancel()).Disabled(a.preview).Clicked() {
				a.transferCall(a.svc.Transfers.Cancel, t.ID)
			}
		case canRetry(t):
			if secondaryButton(c, nil, m.CommonRetry(), a.preview).Clicked() {
				a.transferCall(a.svc.Transfers.Retry, t.ID)
			}
		}
		icon, name := iconCaretRight, m.CommonExpand()
		if expanded {
			icon, name = iconCaretDown, m.CommonCollapse()
		}
		if iconButton(c, icon, name).Clicked() {
			st.setExpanded(t.ID, !expanded)
		}
	})
}

func (s *activityState) setExpanded(id string, on bool) {
	if s.expanded == nil {
		s.expanded = map[string]bool{}
	}
	s.expanded[id] = on
}

func (a *App) transferMenu(menu *ui.Menu, m locales.Messages, t gui.TransferDTO, active bool) {
	st := &a.pages.activity
	resume := t.Stage == "paused"
	name := m.CommonPause()
	if resume {
		name = m.CommonResume()
	}
	if menu.Item(name).Disabled(a.preview || !active).Chosen() {
		a.pauseOrResume(t)
	}
	if menu.Item(m.CommonCancel()).Disabled(a.preview || !active).Chosen() {
		a.transferCall(a.svc.Transfers.Cancel, t.ID)
	}
	if menu.Item(m.CommonRetry()).Disabled(a.preview || !canRetry(t)).Chosen() {
		a.transferCall(a.svc.Transfers.Retry, t.ID)
	}
	menu.Separator()
	if menu.Item(m.ActivityDetails()).Chosen() {
		st.setExpanded(t.ID, true)
	}
}

func (a *App) itemList(c *ui.Context, m locales.Messages, transferID string) {
	th := c.Theme()
	st := a.itemsFor(transferID)
	ui.Column(c).Gap(4).Padding(10, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(th.Border).Children(func() {
		switch {
		case st.state == loadFailed:
			ui.Text(c, st.err).TextColor(th.Danger)
		case st.state != loadDone:
			ui.Spinner(c).Label(m.ActivityLoadingItems())
		case len(st.items) == 0:
			ui.Text(c, m.ActivityNoItems()).TextColor(th.TextMuted)
		default:
			ui.List(c, nil, len(st.items), func(i int) {
				itemRow(c, m, st.items[i])
			}).MaxHeight(itemListMaxHeight)
		}
	})
}

func itemRow(c *ui.Context, m locales.Messages, it gui.TransferItemDTO) {
	th := c.Theme()
	icon := iconImage
	if it.MediaType == "video" {
		icon = iconFilmStrip
	}
	ui.Row(c).Gap(10).Padding(6, 0).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, icon).FontSize(16).TextColor(th.TextMuted).Margin(2, 0, 0, 0).Shrink(0)
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(10).Children(func() {
				ui.Text(c, it.Name).Grow(1).MinWidth(0).SingleLine().Tooltip(it.Name)
				ui.Text(c, formatBytes(it.Size)).FontSize(12).TextColor(th.TextMuted).FontFeatures("tnum").Shrink(0)
				label, k := itemStatus(m, it.Status)
				statusPill(c, label, k).Shrink(0)
			})
			if it.ErrorMessage != "" {
				text, detail := friendlyError(m, "", it.ErrorMessage)
				ui.Text(c, text).FontSize(12).TextColor(th.Danger).SingleLine().Tooltip(cmp.Or(detail, text))
			}
			if it.RemoteNodeID != "" {
				ui.Text(c, it.RemoteNodeID).FontSize(11).Font("monospace").TextColor(th.TextMuted).SingleLine().Selectable()
			}
		})
	})
}
