package view

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

type tone int

const (
	toneNeutral tone = iota
	toneAccent
	toneSuccess
	toneWarning
	toneDanger
)

func toneColor(c *ui.Context, k tone) ui.Color {
	t := c.Theme()
	switch k {
	case toneAccent:
		return t.Accent
	case toneSuccess:
		return t.Success
	case toneWarning:
		return t.Warning
	case toneDanger:
		return t.Danger
	}
	return t.TextMuted
}

func toneTint(c *ui.Context, k tone) ui.Color {
	p := paletteOf(c)
	switch k {
	case toneAccent:
		return p.AccentTint
	case toneSuccess:
		return p.SuccessTint
	case toneWarning:
		return p.WarningTint
	case toneDanger:
		return p.DangerTint
	}
	return c.Theme().Surface
}

func card(c *ui.Context, fn func()) ui.Element {
	t := c.Theme()
	p := paletteOf(c)
	e := ui.Column(c).
		Background(p.Card).
		Border(1, p.CardBorder).
		Radius(radiusCard).
		Padding(20).
		Gap(12)
	if !t.Dark {
		e.Shadow(0, 2, 12, 0, ui.RGBA(0, 0, 0, 0.06))
	}
	return e.Children(fn)
}

func pageHeader(c *ui.Context, title, subtitle string, actions func()) {
	t := c.Theme()
	ui.Row(c).Justify(ui.Center).Padding(pagePadTop, pagePadX, 16).DragWindow().Children(func() {
		ui.Row(c).Grow(1).MaxWidth(maxContent).Gap(16).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(26).FontWeight(700).SingleLine()
				if subtitle != "" {
					ui.Text(c, subtitle).TextColor(t.TextMuted)
				}
			})
			if actions != nil {
				ui.Row(c).Gap(8).Children(actions)
			}
		})
	})
}

func pageFrame(c *ui.Context, title, subtitle string, actions, body func()) {
	ui.Column(c).Grow(1).Children(func() {
		pageHeader(c, title, subtitle, actions)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Row(c).Justify(ui.Center).Padding(4, pagePadX, pagePadX).Children(func() {
				ui.Column(c).Grow(1).MaxWidth(maxContent).Gap(sectionGap).Children(body)
			})
		})
	})
}

func statusPill(c *ui.Context, label string, k tone) ui.Element {
	col := toneColor(c, k)
	return ui.Row(c).Height(22).Padding(0, 10).Gap(6).Radius(radiusPill).Background(toneTint(c, k)).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(col)
		ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(col).SingleLine()
	})
}

func banner(c *ui.Context, k tone, icon *ui.SVG, title, body, actionLabel string, action func()) ui.Element {
	t := c.Theme()
	col := toneColor(c, k)
	return ui.Row(c).Gap(12).Padding(12, 14).Radius(14).Background(toneTint(c, k)).Children(func() {
		ui.Icon(c, icon).FontSize(16).TextColor(col).Margin(1, 0, 0, 0)
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			if title != "" {
				ui.Text(c, title).FontWeight(600)
			}
			if body != "" {
				ui.Text(c, body).TextColor(t.Text)
			}
		})
		if action != nil && actionLabel != "" {
			if secondaryButton(c, nil, actionLabel, false).Clicked() {
				action()
			}
		}
	})
}

func emptyState(c *ui.Context, icon *ui.SVG, title, body, actionLabel string, action func()) {
	t := c.Theme()
	ui.Column(c).Padding(40, 24).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(72, 72).Radius(36).Background(t.Surface).Center().Children(func() {
			ui.Icon(c, icon).FontSize(40).TextColor(t.TextMuted)
		})
		ui.Text(c, title).FontSize(17).FontWeight(600).TextAlign(ui.Center)
		if body != "" {
			ui.Text(c, body).TextColor(t.TextMuted).MaxWidth(360).TextAlign(ui.Center)
		}
		if action != nil && actionLabel != "" {
			if primaryButton(c, nil, actionLabel, false).Clicked() {
				action()
			}
		}
	})
}

func outlined(c *ui.Context, e ui.Element) ui.Element {
	t := c.Theme()
	return e.Background(t.Background).Border(1, t.Border).Radius(radiusField)
}

func pillButton(c *ui.Context, icon *ui.SVG, label string, primary, disabled bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Height(buttonHeight).Padding(0, 16).Gap(8).Radius(radiusPill).
		Center().Shrink(0).Cursor(ui.CursorPointer).Disabled(disabled)
	bg, fg := t.Background, t.Text
	switch {
	case primary:
		bg, fg = t.Accent, t.AccentText
		if b.Pressed() {
			bg = t.AccentPressed
		} else if b.Hovered() {
			bg = t.AccentHover
		}
	default:
		b.Border(1, t.Border)
		if !t.Dark {
			b.Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.08))
		}
		if b.Pressed() {
			bg = t.SurfacePressed
		} else if b.Hovered() {
			bg = t.SurfaceHover
		}
	}
	b.Background(bg)
	if disabled {
		b.Opacity(0.45)
	}
	return b.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).FontSize(18).TextColor(fg)
		}
		ui.Text(c, label).FontSize(14).FontWeight(500).TextColor(fg).SingleLine()
	})
}

func primaryButton(c *ui.Context, icon *ui.SVG, label string, disabled bool) ui.Element {
	return pillButton(c, icon, label, true, disabled)
}

func secondaryButton(c *ui.Context, icon *ui.SVG, label string, disabled bool) ui.Element {
	return pillButton(c, icon, label, false, disabled)
}

func iconButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(iconBtnSize, iconBtnSize).Radius(radiusPill).
		Center().Shrink(0).Cursor(ui.CursorPointer)
	switch {
	case b.Pressed():
		b.Background(t.SurfacePressed)
	case b.Hovered():
		b.Background(t.SurfaceHover)
	}
	return b.Children(func() {
		ui.Icon(c, icon).FontSize(20).TextColor(t.Text)
	})
}

func sectionTitle(c *ui.Context, text string, trailing func()) {
	ui.Row(c).Gap(8).Children(func() {
		ui.Text(c, text).FontSize(17).FontWeight(600).Grow(1).SingleLine()
		if trailing != nil {
			trailing()
		}
	})
}

const (
	iconCanvas = 1024
	iconTile   = 824
)

func (a *App) logoTile(c *ui.Context, size float32) {
	if a.icon == nil {
		return
	}
	full := size * iconCanvas / iconTile
	ui.Box(c).Size(size, size).Shrink(0).Center().Children(func() {
		ui.Image(c, a.icon).Size(full, full).Shrink(0).Fit(ui.Contain)
	})
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", max(n, 0))
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n) / 1024
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v >= 10 {
		return fmt.Sprintf("%.1f %s", v, units[i])
	}
	return fmt.Sprintf("%.2f %s", v, units[i])
}

func relativeTime(c *ui.Context, rfc3339 string) string {
	at, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return ""
	}
	m := tr(c)
	now := c.Now()
	at = at.In(now.Location())
	elapsed := now.Sub(at)
	switch {
	case elapsed < time.Minute:
		return m.TimeJustNow()
	case elapsed < time.Hour:
		return m.TimeMinutesAgo(int(elapsed / time.Minute))
	}
	y, mo, d := now.Date()
	startOfToday := time.Date(y, mo, d, 0, 0, 0, 0, now.Location())
	switch {
	case !at.Before(startOfToday):
		return m.TimeHoursAgo(int(elapsed / time.Hour))
	case !at.Before(startOfToday.AddDate(0, 0, -1)):
		return m.TimeYesterday()
	}
	return i18n.FormatDate(c.Region().FormatLocale(c.Locale()), at, i18n.DateOptions{Date: i18n.MediumStyle})
}

func dialogFrame(c *ui.Context, backdrop, panel ui.Element, width float32) {
	t := c.Theme()
	backdrop.Background(ui.RGBA(0, 0, 0, 0.45))
	panel.Width(width).MaxHeightPercent(92).Radius(radiusDialog).Clip().
		Background(paletteOf(c).Card).Border(1, t.Border).
		Shadow(0, 16, 48, 0, ui.RGBA(0, 0, 0, 0.28)).AlignItems(ui.Stretch)
}

func stepper(c *ui.Context, value *float64, lo, hi, step float64, label string) bool {
	t := c.Theme()
	m := tr(c)
	changed := false
	ui.Row(c).Height(fieldHeight).Padding(0, 2).Gap(2).AlignItems(ui.Center).Radius(radiusPill).
		Background(t.Background).Border(1, t.Border).Shrink(0).Children(func() {
		if iconButton(c, iconMinus, m.SettingsDecrease()+" "+label).Size(28, 28).Disabled(*value <= lo).Clicked() {
			*value = max(*value-step, lo)
			changed = true
		}
		ui.Text(c, strconv.Itoa(int(*value))).FontWeight(500).FontFeatures("tnum").Width(32).TextAlign(ui.Center).SingleLine()
		if iconButton(c, iconPlus, m.SettingsIncrease()+" "+label).Size(28, 28).Disabled(*value >= hi).Clicked() {
			*value = min(*value+step, hi)
			changed = true
		}
	})
	return changed
}
