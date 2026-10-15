package view

import (
	"context"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const (
	restoreUploaded = iota
	restoreContainer
	restoreByID

	comboboxThreshold = 15
	restoreListHeight = 360
)

type restoreChoice struct {
	label string
	id    string
	album bool
}

type restoreState struct {
	mode      int
	selected  map[string]*bool
	list      ui.ListState
	container string
	folderID  string
	ids       string
	dest      string
	conflict  string
	starting  bool
}

func (a *App) restorePage(c *ui.Context, r *ui.Route) {
	m := tr(c)
	r.Title(m.RestoreTitle())
	a.ensureRecovery()
	pageFrame(c, m.RestoreTitle(), m.RestoreSubtitle(), nil, func() { a.restoreBody(c, m) })
}

func (a *App) restoreBody(c *ui.Context, m locales.Messages) {
	rs := &a.restore
	if rs.conflict == "" {
		rs.conflict = "skip"
		if a.settings != nil && a.settings.DefaultConflictPolicy != "" {
			rs.conflict = a.settings.DefaultConflictPolicy
		}
	}
	choices := a.restoreChoices(m)
	ui.Column(c).Gap(16).Children(func() {
		a.restoreBanners(c, m)
		a.restoreModes(c, m)
		switch rs.mode {
		case restoreUploaded:
			a.restoreUploadedCard(c, m)
		case restoreContainer:
			a.restoreContainerCard(c, m, choices)
		default:
			a.restoreIDsCard(c, m)
		}
		a.restoreDestination(c, m)
		a.restoreFooter(c, m, choices)
	})
}

func (a *App) restoreBanners(c *ui.Context, m locales.Messages) {
	if a.recoveryState == loadFailed {
		text, detail := friendlyError(m, "", a.recoveryErr)
		banner(c, toneDanger, iconWarningCircle, text, "", m.CommonRetry(), a.refreshRecovery).Tooltip(detail)
	}
	rec := a.recovery
	if rec == nil {
		return
	}
	if rec.Evidence != "verified" {
		banner(c, toneWarning, iconWarning, m.RestoreExperimental(), rec.CapabilityNotes, "", nil)
	}
	if rec.Warning != "" {
		banner(c, toneWarning, iconInfo, "", rec.Warning, "", nil)
	}
}

func (a *App) restoreModes(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	rs := &a.restore
	labels := []string{m.RestoreSourceUploaded(), m.RestoreSourceContainer(), m.RestoreSourceById()}
	ui.Row(c).Children(func() {
		seg := ui.SegmentedBase(c.Key("restore-mode"), &rs.mode, len(labels))
		seg.Track.Padding(3).Gap(2).Radius(radiusPill).Background(t.Surface)
		seg.Track.Children(func() {
			for i, label := range labels {
				s := seg.Segment(i).Padding(6, 18).Radius(radiusPill).Label(label).Cursor(ui.CursorPointer)
				fg := t.TextMuted
				switch {
				case i == rs.mode:
					s.Background(t.Background).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.12))
					fg = t.Text
				case s.Hovered():
					fg = t.Text
				}
				s.Children(func() {
					ui.Text(c, label).FontWeight(500).TextColor(fg).SingleLine()
				})
			}
		})
	})
}

func (a *App) restoreItems() []gui.TransferItemDTO {
	if a.recovery == nil {
		return nil
	}
	var out []gui.TransferItemDTO
	for i := range a.recovery.ConfirmedItems {
		if a.recovery.ConfirmedItems[i].RemoteNodeID != "" {
			out = append(out, a.recovery.ConfirmedItems[i])
		}
	}
	return out
}

func (a *App) restoreFlag(id string) *bool {
	rs := &a.restore
	if rs.selected == nil {
		rs.selected = map[string]*bool{}
	}
	if rs.selected[id] == nil {
		rs.selected[id] = new(bool)
	}
	return rs.selected[id]
}

func (a *App) restoreSelection(items []gui.TransferItemDTO) (picked []gui.TransferItemDTO, size int64) {
	for i := range items {
		if f := a.restore.selected[items[i].ID]; f != nil && *f {
			picked = append(picked, items[i])
			size += items[i].Size
		}
	}
	return picked, size
}

const (
	checkOff = iota
	checkOn
	checkMixed
)

func checkMark(c *ui.Context, state int) {
	t := c.Theme()
	box := ui.Box(c).Size(18, 18).Shrink(0).Radius(5).Center()
	if state == checkOff {
		box.Background(t.Background).Border(1.5, t.TextMuted.Alpha(0.7))
		return
	}
	box.Background(t.Accent).Children(func() {
		if state == checkMixed {
			ui.Box(c).Size(8, 2).Radius(1).Background(t.AccentText)
			return
		}
		ui.Icon(c, iconCheck).FontSize(12).TextColor(t.AccentText)
	})
}

func (a *App) restoreUploadedCard(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	rs := &a.restore
	items := a.restoreItems()
	card(c, func() {
		switch {
		case a.recoveryState == loadRunning || a.recoveryState == loadIdle:
			ui.Row(c).Justify(ui.Center).Padding(24, 0).Children(func() {
				ui.Spinner(c).Label(m.CommonLoading())
			})
			return
		case len(items) == 0:
			emptyState(c, iconCloudArrowDown, m.RestoreEmptyTitle(), m.RestoreEmptyBody(), "", nil)
			return
		}
		picked, size := a.restoreSelection(items)
		a.restoreSelectAll(c, m, items, len(picked), size)
		ui.Box(c).Height(1).Background(t.Border).Margin(0, -20)
		rs.list.Key = func(i int) any { return items[i].ID }
		ui.List(c, &rs.list, len(items), func(i int) {
			a.restoreRow(c.Key(items[i].ID), items[i])
		}).MaxHeight(restoreListHeight)
	})
}

func (a *App) restoreSelectAll(c *ui.Context, m locales.Messages, items []gui.TransferItemDTO, count int, size int64) {
	t := c.Theme()
	all := count == len(items)
	state := checkOff
	switch {
	case all:
		state = checkOn
	case count > 0:
		state = checkMixed
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		box := ui.CheckboxBase(c.Key("select-all"), &all).Row().Gap(10).Padding(2, 4).Radius(6).Cursor(ui.CursorPointer)
		box.Children(func() {
			checkMark(c, state)
			ui.Text(c, m.RestoreSelectAll()).FontWeight(500)
		})
		if box.Changed() {
			for i := range items {
				*a.restoreFlag(items[i].ID) = all
			}
		}
		ui.Box(c).Grow(1)
		if count > 0 {
			ui.Text(c, m.RestoreSelected(count, formatBytes(size))).FontSize(12).TextColor(t.TextMuted).FontFeatures("tnum")
		}
	})
}

func (a *App) restoreRow(c *ui.Context, it gui.TransferItemDTO) {
	t := c.Theme()
	flag := a.restoreFlag(it.ID)
	state := checkOff
	if *flag {
		state = checkOn
	}
	row := ui.CheckboxBase(c, flag).Row().Gap(10).Padding(8, 10).Radius(radiusFeat).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label(it.Name)
	switch {
	case *flag:
		row.Background(paletteOf(c).AccentTint)
	case row.Hovered():
		row.Background(t.SurfaceHover)
	}
	icon := iconImage
	if it.MediaType == "video" {
		icon = iconFilmStrip
	}
	row.Children(func() {
		checkMark(c, state)
		ui.Icon(c, icon).FontSize(20).TextColor(t.TextMuted)
		ui.Text(c, it.Name).Grow(1).MinWidth(0).SingleLine()
		ui.Text(c, formatBytes(it.Size)).FontSize(12).TextColor(t.TextMuted).FontFeatures("tnum").MinWidth(64).TextAlign(ui.End)
		ui.Text(c, relativeTime(c, it.UpdatedAt)).FontSize(12).TextColor(t.TextMuted).MinWidth(88).TextAlign(ui.End).SingleLine()
	})
}

func (a *App) restoreChoices(m locales.Messages) []restoreChoice {
	if a.recovery == nil {
		return nil
	}
	seen := map[string]int{}
	var out []restoreChoice
	add := func(label, id string, album bool) {
		seen[label]++
		if n := seen[label]; n > 1 {
			label = label + " (" + strconv.Itoa(n) + ")"
		}
		out = append(out, restoreChoice{label: label, id: id, album: album})
	}
	for _, al := range a.recovery.Albums {
		add(m.RestoreAlbumOption(al.Name), al.ID, true)
	}
	for _, f := range a.recovery.Folders {
		add(m.RestoreFolderOption(f.Name), f.ID, false)
	}
	return out
}

func (a *App) restoreContainerCard(c *ui.Context, m locales.Messages, choices []restoreChoice) {
	t := c.Theme()
	rs := &a.restore
	labels := make([]string, len(choices))
	for i := range choices {
		labels[i] = choices[i].label
	}
	card(c, func() {
		if len(labels) > 0 {
			var picker ui.Element
			if len(labels) > comboboxThreshold {
				picker = ui.Combobox(c, &rs.container, labels).Placeholder(m.RestoreContainerPlaceholder())
			} else {
				picker = outlined(c, ui.Select(c, &rs.container, labels))
			}
			if picker.Label(m.RestoreSourceContainer()).MaxWidth(420).Changed() {
				rs.folderID = ""
			}
		}
		a.restoreBrowseRow(c, m)
		if rs.container == "" && rs.folderID == "" {
			ui.Text(c, m.RestoreContainerPlaceholder()).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (a *App) restoreBrowseRow(c *ui.Context, m locales.Messages) {
	rs := &a.restore
	ui.Field(c, m.RestoreFolderLabel(), func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if rs.folderID != "" {
				ui.Icon(c, iconFolder).FontSize(18).TextColor(c.Theme().TextMuted)
				ui.Text(c, a.destinationPath(m, rs.folderID, "")).FontWeight(600).SingleLine().Grow(1).MinWidth(0)
			} else {
				ui.Box(c).Grow(1)
			}
			if secondaryButton(c, nil, m.RestoreBrowseFolders(), false).Clicked() {
				a.openPicker(rs.folderID, func(id string) {
					rs.folderID = id
					if id != "" {
						rs.container = ""
					}
				})
			}
		})
	})
}

func parseNodeIDs(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t' || r == '\r' }) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func (a *App) restoreIDsCard(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	rs := &a.restore
	card(c, func() {
		outlined(c, ui.TextArea(c, &rs.ids)).Font("monospace").Placeholder(m.RestoreIdsPlaceholder()).Lines(4, 8).Label(m.RestoreSourceById())
		if n := len(parseNodeIDs(rs.ids)); n > 0 {
			ui.Text(c, m.RestoreIdCount(n)).FontSize(12).TextColor(t.TextMuted).FontFeatures("tnum")
		}
	})
}

func (a *App) restoreDestination(c *ui.Context, m locales.Messages) {
	rs := &a.restore
	card(c, func() {
		ui.Form(c, func() {
			ui.Field(c, m.RestoreSaveTo(), func() {
				ui.Row(c).Gap(8).Children(func() {
					outlined(c, ui.TextInput(c, &rs.dest)).Placeholder(m.RestoreFolderPlaceholder()).Grow(1).MinWidth(0)
					if secondaryButton(c, nil, m.CommonChoose(), false).Clicked() {
						a.pickRestoreFolder()
					}
				})
			})
			ui.Field(c, m.OptionIfExists(), func() {
				codeSelect(c, &rs.conflict, []string{"skip", "rename", "fail"}, func(code string) string {
					return conflictLabel(m, code, true)
				})
			})
		})
	})
}

func (a *App) restoreRequest(choices []restoreChoice) (in gui.DownloadInput, ok bool) {
	rs := &a.restore
	in = gui.DownloadInput{DestDir: strings.TrimSpace(rs.dest), ConflictPolicy: rs.conflict}
	switch rs.mode {
	case restoreUploaded:
		picked, _ := a.restoreSelection(a.restoreItems())
		for i := range picked {
			it := &picked[i]
			in.Items = append(in.Items, gui.DownloadItemInput{
				NodeID: it.RemoteNodeID, Name: it.Name, ExpectedSize: it.Size, ExpectedMD5: it.MD5, MediaType: it.MediaType,
			})
		}
		ok = len(picked) > 0
	case restoreContainer:
		in.ContainerNodeID = rs.folderID
		ok = rs.folderID != ""
		for _, ch := range choices {
			if rs.folderID == "" && ch.label == rs.container {
				in.ContainerNodeID, ok = ch.id, true
			}
		}
	default:
		in.NodeIDs = parseNodeIDs(rs.ids)
		ok = len(in.NodeIDs) > 0
	}
	return in, ok && in.DestDir != ""
}

func (a *App) restoreFooter(c *ui.Context, m locales.Messages, choices []restoreChoice) {
	t := c.Theme()
	rs := &a.restore
	in, ok := a.restoreRequest(choices)
	label := m.RestoreDownload()
	switch rs.mode {
	case restoreUploaded:
		if n := len(in.Items); n > 0 {
			label = m.RestoreDownloadItems(n)
		}
	case restoreContainer:
		if rs.folderID != "" {
			label = m.RestoreDownloadFolder()
		}
		for _, ch := range choices {
			if rs.folderID == "" && ch.label == rs.container {
				label = m.RestoreDownloadFolder()
				if ch.album {
					label = m.RestoreDownloadAlbum()
				}
			}
		}
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, m.RestoreVerifyNote()).FontSize(12).TextColor(t.TextMuted).Grow(1)
		if primaryButton(c, nil, label, !ok || rs.starting).Clicked() {
			a.startDownload(in)
		}
	})
}

func (a *App) pickRestoreFolder() {
	a.do(func(ctx context.Context) func() {
		path, err := a.svc.Downloads.PickTargetDirectory(ctx)
		return func() {
			if err != nil {
				a.toastError(err)
				return
			}
			if path != "" {
				a.restore.dest = path
			}
		}
	})
}

func (a *App) startDownload(in gui.DownloadInput) {
	a.restore.starting = true
	a.do(func(ctx context.Context) func() {
		t, err := a.svc.Downloads.StartDownload(ctx, in)
		return func() {
			a.restore.starting = false
			if err != nil {
				a.toastError(err)
				return
			}
			a.upsertTransfer(*t)
			a.restore.selected = nil
			a.restore.ids = ""
			a.toastAction(locales.ToastDownloadStarted(), locales.CommonView(), func() { a.router.Push("/activity") })
		}
	})
}
