package view

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const (
	pickerWidth      = 520
	pickerListHeight = 300
	pathSeparator    = " › "
)

type folderState struct {
	state   loadState
	listing *gui.FolderListingDTO
	err     string
}

type pickerState struct {
	open       bool
	current    string
	crumb      int
	manual     string
	manualOpen bool
	list       ui.ListState
	onChoose   func(id string)
}

func (a *App) folderFor(id string) *folderState {
	st := a.folders[id]
	if st == nil {
		st = &folderState{}
		a.folders[id] = st
	}
	if st.state == loadIdle {
		a.loadFolder(id)
	}
	return st
}

func (a *App) loadFolder(id string) {
	st := a.folders[id]
	if st == nil {
		st = &folderState{}
		a.folders[id] = st
	}
	st.state = loadRunning
	a.do(func(ctx context.Context) func() {
		listing, err := a.svc.Transfers.ListFolders(ctx, id)
		return func() {
			if err != nil {
				st.state, st.err = loadFailed, errMessage(err)
				return
			}
			st.state, st.listing, st.err = loadDone, listing, ""
		}
	})
}

func (a *App) staleFolders() {
	for _, st := range a.folders {
		if st.state != loadRunning {
			st.state = loadIdle
		}
	}
}

func (a *App) destinationPath(m locales.Messages, id, fallbackName string) string {
	if id == "" {
		return m.DestLibrary()
	}
	st := a.folderFor(id)
	switch {
	case st.listing != nil:
		return folderPath(m, st.listing.Path)
	case st.state == loadFailed && fallbackName != "":
		return fallbackName
	case st.state == loadFailed:
		return id
	}
	return m.DestLibrary() + pathSeparator + "…"
}

func folderNames(m locales.Messages, path []gui.FolderRefDTO) []string {
	names := make([]string, 0, len(path)+1)
	names = append(names, m.DestLibrary())
	for i, f := range path {
		if i == 0 && f.Name == "" {
			continue
		}
		names = append(names, f.Name)
	}
	return names
}

func folderPath(m locales.Messages, path []gui.FolderRefDTO) string {
	return strings.Join(folderNames(m, path), pathSeparator)
}

func (a *App) openPicker(start string, onChoose func(id string)) {
	a.staleFolders()
	p := &a.picker
	p.open, p.current, p.crumb, p.manual, p.manualOpen, p.onChoose = true, start, 0, "", false, onChoose
	p.list.ScrollTo(0, ui.Start)
}

func (a *App) pickerNavigate(id string) {
	p := &a.picker
	p.current = id
	p.list.ScrollTo(0, ui.Start)
}

func (a *App) pickerConfirm(id string) {
	p := &a.picker
	p.open = false
	if p.onChoose != nil {
		p.onChoose(id)
	}
}

func (a *App) pickerDialog(c *ui.Context) {
	m := tr(c)
	p := &a.picker
	ui.DialogBase(c, &p.open, func(backdrop, panel ui.Element) {
		dialogFrame(c, backdrop, panel, pickerWidth)
		st := a.folderFor(p.current)
		ui.Row(c).Padding(24, 20, 8, 28).Gap(12).Shrink(0).Children(func() {
			ui.Text(c, m.PickerTitle()).FontSize(20).FontWeight(700).Grow(1)
			if iconButton(c, iconX, m.CommonClose()).Clicked() {
				p.open = false
			}
		})
		ui.Column(c).Padding(4, 28, 8).Gap(12).Children(func() {
			a.pickerCrumbs(c, m, st)
			a.pickerFolders(c, m, st)
			ui.Collapsible(c, m.PickerManual(), &p.manualOpen, func() { a.pickerManualID(c, m) })
		})
		a.pickerFooter(c, m, st)
	})
}

func (a *App) pickerCrumbs(c *ui.Context, m locales.Messages, st *folderState) {
	p := &a.picker
	var path []gui.FolderRefDTO
	if st.listing != nil {
		path = st.listing.Path
	}
	labels := folderNames(m, path)
	p.crumb = len(labels) - 1
	if ui.Breadcrumbs(c, labels, &p.crumb).Label(m.PickerPath()).Changed() {
		target := ""
		if i := p.crumb - (len(labels) - len(path)); p.crumb > 0 && i >= 0 && i < len(path) {
			target = path[i].ID
		}
		a.pickerNavigate(target)
	}
}

func (a *App) pickerFolders(c *ui.Context, m locales.Messages, st *folderState) {
	t := c.Theme()
	p := &a.picker
	ui.Column(c).Height(pickerListHeight).Radius(radiusField).Border(1, t.Border).Clip().Children(func() {
		switch {
		case st.listing == nil && st.state == loadFailed:
			ui.Column(c).Grow(1).Center().Gap(10).Padding(16).Children(func() {
				ui.Text(c, st.err).TextColor(t.Danger).TextAlign(ui.Center)
				if secondaryButton(c, nil, m.CommonRetry(), false).Clicked() {
					st.state = loadIdle
				}
			})
		case st.listing == nil:
			ui.Column(c).Grow(1).Center().Children(func() { ui.Spinner(c).Label(m.CommonLoading()) })
		case len(st.listing.Folders) == 0:
			ui.Column(c).Grow(1).Center().Children(func() {
				ui.Text(c, m.PickerEmpty()).TextColor(t.TextMuted)
			})
		default:
			folders := st.listing.Folders
			p.list.Key = func(i int) any { return folders[i].ID }
			ui.List(c, &p.list, len(folders), func(i int) {
				a.pickerRow(c.Key(folders[i].ID), folders[i])
			}).Grow(1).Padding(4)
		}
	})
	if st.listing != nil && st.listing.Truncated {
		ui.Text(c, m.PickerTruncated()).FontSize(12).TextColor(t.TextMuted)
	}
}

func (a *App) pickerRow(c *ui.Context, f gui.FolderRefDTO) {
	t := c.Theme()
	row := ui.ButtonBase(c).Label(f.Name).Row().Gap(10).Padding(9, 10).Radius(radiusField).AlignItems(ui.Center).
		Cursor(ui.CursorPointer)
	switch {
	case row.Pressed():
		row.Background(t.SurfacePressed)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	row.Children(func() {
		ui.Icon(c, iconFolder).FontSize(20).TextColor(t.TextMuted)
		ui.Text(c, f.Name).Grow(1).MinWidth(0).SingleLine()
		ui.Icon(c, iconCaretRight).FontSize(16).TextColor(t.TextMuted)
	})
	if row.Clicked() {
		a.pickerNavigate(f.ID)
	}
}

func (a *App) pickerManualID(c *ui.Context, m locales.Messages) {
	p := &a.picker
	use := func() {
		if id := strings.TrimSpace(p.manual); id != "" {
			a.pickerConfirm(id)
		}
	}
	ui.Row(c).Gap(8).Padding(8, 0, 0).Children(func() {
		in := outlined(c, ui.TextInput(c, &p.manual)).Placeholder(m.PickerManualPlaceholder()).Label(m.PickerManual()).Grow(1).MinWidth(0)
		if in.Submitted() {
			use()
		}
		if secondaryButton(c, nil, m.PickerManualUse(), strings.TrimSpace(p.manual) == "").Clicked() {
			use()
		}
	})
}

func (a *App) pickerFooter(c *ui.Context, m locales.Messages, st *folderState) {
	p := &a.picker
	ui.Row(c).Gap(8).Padding(8, 28, 24).Shrink(0).Justify(ui.End).Children(func() {
		if secondaryButton(c, nil, m.CommonCancel(), false).Clicked() {
			p.open = false
		}
		if primaryButton(c, nil, m.PickerChoose(), st.listing == nil).Clicked() {
			id := st.listing.ID
			if len(st.listing.Path) <= 1 {
				id = ""
			}
			a.pickerConfirm(id)
		}
	})
}

func (a *App) destinationRow(c *ui.Context, m locales.Messages, path string, change, status func()) {
	t := c.Theme()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, iconFolder).FontSize(18).TextColor(t.TextMuted)
		pathText := ui.Text(c, path).FontWeight(600).SingleLine().MinWidth(0).Tooltip(path)
		if status == nil {
			pathText.Grow(1)
		} else {
			pathText.Shrink(1)
			status()
			ui.Box(c).Grow(1)
		}
		if secondaryButton(c, nil, m.DestChange(), false).Clicked() {
			change()
		}
	})
}
