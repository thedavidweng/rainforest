package view

import (
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/locales"
)

func (a *App) View(c *ui.Context) {
	applyTheme(c)
	a.flushToasts(c)
	switch {
	case !a.authLoaded:
		a.loading(c)
	case !a.signedIn():
		a.welcomeShell(c)
	default:
		a.mainShell(c)
	}
	a.flushToasts(c)
}

func dragStrip(c *ui.Context) {
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(max(c.TitleBar().Height, headerHeight)).DragWindow()
}

func (a *App) loading(c *ui.Context) {
	dragStrip(c)
	ui.Column(c).Fill().Center().Children(func() {
		ui.Spinner(c).Label(tr(c).CommonLoading())
	})
}

func (a *App) welcomeShell(c *ui.Context) {
	dragStrip(c)
	a.welcome(c)
}

func (a *App) takeDrops(e ui.Element) bool {
	if paths := e.DroppedFiles(); len(paths) > 0 {
		a.openUploadReview(paths)
		a.router.Push("/upload")
	}
	return e.FileDragOver()
}

func (a *App) mainShell(c *ui.Context) {
	t := c.Theme()
	a.ensureUsage()
	root := ui.Row(c).Fill().AlignItems(ui.Stretch)
	dragging := a.takeDrops(root)
	root.Children(func() {
		a.sidebar(c)
		ui.Column(c).Grow(1).Background(t.Background).Children(func() {
			a.router.View(c, a.route(c)).Grow(1)
		})
		if dragging && a.router.Path() != "/upload" {
			a.dropOverlay(c)
		}
		a.pickerDialog(c)
	})
}

func (a *App) route(c *ui.Context) func(r *ui.Route) {
	return func(r *ui.Route) {
		switch {
		case r.Match("/activity"):
			a.activityPage(c, r)
		case r.Match("/backup"):
			a.backupPage(c, r)
		case r.Match("/restore"):
			a.restorePage(c, r)
		case r.Match("/settings"):
			a.settingsPage(c, r)
		default:
			a.uploadPage(c, r)
		}
	}
}

func (a *App) dropOverlay(c *ui.Context) {
	t := c.Theme()
	p := paletteOf(c)
	ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Center().
		Background(p.AccentTint).Border(2, t.Accent).Children(func() {
		ui.Row(c).Gap(10).Padding(14, 24).Radius(radiusPill).Background(t.Background).Border(1, t.Border).Children(func() {
			ui.Icon(c, iconCloudArrowUp).FontSize(22).TextColor(t.Text)
			ui.Text(c, tr(c).DropOverlay()).FontSize(15).FontWeight(600)
		})
	})
}

func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	m := tr(c)
	col := ui.Column(c).Width(sidebarWidth).Shrink(0).AlignItems(ui.Stretch).Background(paletteOf(c).Sidebar)
	if t.Dark {
		col.BorderWidth(0, 1, 0, 0).BorderColor(t.Border)
	}
	col.Children(func() {
		ui.Box(c).Height(max(c.TitleBar().Height, headerHeight)).Shrink(0).DragWindow()
		a.lockup(c, m)
		a.navItems(c, m)
		a.accountBlock(c, m)
	})
}

func (a *App) lockup(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	ui.Row(c).Gap(12).Padding(4, 28, 20).DragWindow().Children(func() {
		a.logoTile(c, 32)
		ui.Column(c).Children(func() {
			ui.Text(c, "Rainforest").FontSize(18).FontWeight(700).SingleLine()
			ui.Text(c, m.AppTagline()).FontSize(12).TextColor(t.TextMuted).SingleLine()
		})
	})
}

func (a *App) currentSection() string {
	path := a.router.Path()
	if i := strings.IndexByte(path[1:], '/'); i >= 0 {
		path = path[:i+1]
	}
	return path
}

func (a *App) backupNeedsAttention() bool {
	for i := range a.rules {
		if st := a.rules[i].Status; st == "error" || st == "source_unavailable" {
			return true
		}
	}
	return false
}

type navEntry struct {
	path     string
	label    string
	inactive *ui.SVG
	selected *ui.SVG
	badge    func()
}

func (a *App) navEntries(c *ui.Context, m locales.Messages) []navEntry {
	t := c.Theme()
	return []navEntry{
		{"/upload", m.NavUpload(), iconCloudArrowUpBold, iconCloudArrowUpFill, nil},
		{"/activity", m.NavActivity(), iconArrowsDownUpBold, iconArrowsDownUpFill, func() {
			if n := len(a.active); n > 0 {
				ui.Badge(c, countBadge(n)).Background(t.Accent).TextColor(t.AccentText)
			}
		}},
		{"/backup", m.NavBackup(), iconFolderBold, iconFolderFill, func() {
			if a.backupNeedsAttention() {
				ui.Badge(c, "!").Background(t.Warning).TextColor(t.AccentText)
			}
		}},
		{"/restore", m.NavRestore(), iconCloudArrowDownBold, iconCloudArrowDownFill, nil},
		{"/settings", m.NavSettings(), iconGearBold, iconGearFill, nil},
	}
}

func (a *App) navItems(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	p := paletteOf(c)
	entries := a.navEntries(c, m)
	page := a.currentSection()
	cur := slices.IndexFunc(entries, func(e navEntry) bool { return e.path == page })
	item := func(i int) {
		e := entries[i]
		selected := i == cur
		row := ui.ButtonBase(c.Key(e.path)).Label(e.label).Row().Height(navItemHeight).Padding(0, 16).Gap(16).
			Radius(radiusPill).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		switch {
		case selected:
			row.Background(p.NavPill)
		case row.Hovered():
			row.Background(t.Surface.Alpha(0.6))
		}
		icon, fg, weight := e.inactive, p.NavIcon, 400
		if selected {
			icon, fg, weight = e.selected, t.Text, 700
		}
		row.Children(func() {
			ui.Icon(c, icon).FontSize(navIconSize).TextColor(fg)
			ui.Text(c, e.label).FontSize(16).FontWeight(weight).Grow(1).SingleLine()
			if e.badge != nil {
				e.badge()
			}
		})
		if row.Clicked() {
			a.router.Push(e.path)
		}
		if row.Shortcut(0, ui.KeyDown) && cur+1 < len(entries) {
			a.router.Push(entries[cur+1].path)
		}
		if row.Shortcut(0, ui.KeyUp) && cur > 0 {
			a.router.Push(entries[cur-1].path)
		}
	}
	ui.Column(c).Padding(0, 12).Gap(4).Grow(1).Children(func() {
		for i := 0; i < len(entries)-1; i++ {
			item(i)
		}
	})
	ui.Column(c).Padding(0, 12, 8).Gap(8).Shrink(0).Children(func() {
		ui.Box(c).Height(1).Background(t.Border).Margin(0, 4)
		item(len(entries) - 1)
	})
}

func (a *App) accountBlock(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	ui.Column(c).Gap(8).Padding(8, 12, 16).Shrink(0).Children(func() {
		if a.backupsStopped() {
			pause := ui.ButtonBase(c).Label(m.BackupsPausedNotice()).Row().Gap(6).Padding(2, 4).Radius(6).Cursor(ui.CursorPointer)
			if pause.Hovered() {
				pause.Background(t.SurfaceHover)
			}
			pause.Children(func() {
				ui.Icon(c, iconPause).FontSize(14).TextColor(t.Warning)
				ui.Text(c, m.BackupsPausedNotice()).FontSize(12).TextColor(t.Warning)
			})
			if pause.Clicked() {
				a.router.Push("/backup")
			}
		}
		acct := ui.ButtonBase(c).Label(regionHost(a.auth.Region)).Row().Gap(10).Padding(6, 8).Radius(radiusPill).Cursor(ui.CursorPointer)
		if acct.Hovered() {
			acct.Background(t.SurfaceHover)
		}
		acct.Children(func() {
			ui.Box(c).Size(32, 32).Shrink(0).Radius(16).Background(t.Text).Center().Children(func() {
				ui.Text(c, regionCode(a.auth.Region)).FontSize(11).FontWeight(600).TextColor(t.Background)
			})
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, regionHost(a.auth.Region)).FontWeight(600).SingleLine()
				ui.Text(c, a.usageSummary(m)).FontSize(12).TextColor(t.TextMuted).SingleLine().FontFeatures("tnum")
			})
		})
		if acct.Clicked() {
			a.router.Push("/settings")
		}
	})
}

func (a *App) usageSummary(m locales.Messages) string {
	if a.usage == nil {
		return m.AccountSignedIn()
	}
	return m.AccountUsage(int(a.usage.Photo.Total.Count), formatBytes(a.usage.TotalBytes))
}

func countBadge(n int) string {
	if n > 99 {
		return "99+"
	}
	return strconv.Itoa(n)
}
