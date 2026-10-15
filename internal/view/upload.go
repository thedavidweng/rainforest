package view

import (
	"context"
	"os"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const (
	recentTransfers = 4
	maxThumbs       = 24
	thumbColumns    = 8
	reviewWidth     = 640
	dropZoneHeight  = 280
)

var conflictPolicies = []string{"skip", "rename", "fail"}

type pageState struct {
	review   reviewState
	activity activityState
}

type reviewState struct {
	open          bool
	paths         []string
	includeVideos bool
	conflict      string
	nodeID        string
	keepFolders   bool
	hasFolder     bool
	plan          *gui.UploadPlanDTO
	loading       bool
	starting      bool
	err           string
	seq           int
}

func (r *reviewState) canConfirm() bool {
	return r.plan != nil && !r.loading && !r.starting &&
		r.plan.TotalCount > 0 && destinationReadable(r.plan.Destination.Status)
}

func (a *App) openUploadReview(paths []string) {
	if len(paths) == 0 {
		return
	}
	conflict := "skip"
	if a.settings != nil && a.settings.DefaultConflictPolicy != "" {
		conflict = a.settings.DefaultConflictPolicy
	}
	a.pages.review = reviewState{
		open: true, paths: paths, includeVideos: true, conflict: conflict,
		keepFolders: true, hasFolder: containsDirectory(paths),
	}
	a.replan()
}

func containsDirectory(paths []string) bool {
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func (a *App) replan() {
	r := &a.pages.review
	r.seq++
	seq := r.seq
	r.loading, r.err = true, ""
	includeVideos, keepFolders := r.includeVideos, r.keepFolders
	in := gui.UploadPlanInput{
		Paths:               r.paths,
		DestinationNodeID:   r.nodeID,
		ConflictPolicy:      r.conflict,
		IncludeVideos:       &includeVideos,
		KeepFolderStructure: &keepFolders,
	}
	a.do(func(ctx context.Context) func() {
		plan, err := a.svc.Transfers.PlanUpload(ctx, in)
		return func() {
			if r.seq != seq {
				return
			}
			r.loading = false
			if err != nil {
				r.plan, r.err = nil, errMessage(err)
				return
			}
			r.plan = plan
		}
	})
}

func (a *App) startUpload() {
	r := &a.pages.review
	if !r.canConfirm() {
		return
	}
	r.starting = true
	includeVideos, keepFolders := r.includeVideos, r.keepFolders
	in := gui.StartUploadInput{
		Paths:               r.paths,
		DestinationNodeID:   r.nodeID,
		ConflictPolicy:      r.conflict,
		IncludeVideos:       &includeVideos,
		KeepFolderStructure: &keepFolders,
	}
	a.do(func(ctx context.Context) func() {
		t, err := a.svc.Transfers.StartUpload(ctx, in)
		return func() {
			r.starting = false
			if err != nil {
				a.toastError(err)
				return
			}
			a.upsertTransfer(*t)
			r.open = false
			a.toastAction(locales.ToastUploadStarted(), locales.CommonView(), func() { a.router.Push("/activity") })
		}
	})
}

func (a *App) pickFiles() {
	a.do(func(ctx context.Context) func() {
		paths, err := a.svc.Transfers.PickFiles(ctx)
		return func() { a.picked(paths, err) }
	})
}

func (a *App) pickDirectory() {
	a.do(func(ctx context.Context) func() {
		path, err := a.svc.Transfers.PickDirectory(ctx)
		var paths []string
		if path != "" {
			paths = []string{path}
		}
		return func() { a.picked(paths, err) }
	})
}

func (a *App) picked(paths []string, err error) {
	if err != nil {
		a.toastError(err)
		return
	}
	a.openUploadReview(paths)
}

func (a *App) uploadPage(c *ui.Context, r *ui.Route) {
	m := tr(c)
	r.Title(m.UploadTitle())
	pageFrame(c, m.UploadTitle(), m.UploadSubtitle(), nil, func() {
		a.dropZone(c, m)
		a.recentUploads(c, m)
		a.recentActivity(c, m)
		ui.Text(c, m.UploadSupported()).FontSize(12).TextColor(c.Theme().TextMuted)
	})
	a.reviewDialog(c, m)
}

func (a *App) dropZone(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	zone := ui.Column(c).MinHeight(dropZoneHeight).Padding(32).Gap(8).Center().Radius(radiusCard)
	if paths := zone.DroppedFiles(); len(paths) > 0 {
		a.openUploadReview(paths)
	}
	over := zone.FileDragOver()
	if over {
		zone.Background(t.Surface).Border(2, t.Accent)
	} else {
		zone.Background(t.Background).Border(1.5, t.Border).BorderStyle(ui.BorderDashed)
	}
	zone.Children(func() {
		ui.Box(c).Size(72, 72).Radius(36).Background(t.Surface).Center().Margin(0, 0, 8).Children(func() {
			ui.Icon(c, iconCloudArrowUp).FontSize(36).TextColor(t.Text)
		})
		ui.Text(c, m.UploadDropTitle()).FontSize(20).FontWeight(600).TextAlign(ui.Center)
		ui.Text(c, m.UploadDropBody()).TextColor(t.TextMuted).TextAlign(ui.Center).MaxWidth(420)
		ui.Row(c).Gap(10).Margin(12, 0, 0).Children(func() {
			if primaryButton(c, iconPlus, m.UploadChooseFiles(), false).Clicked() {
				a.pickFiles()
			}
			if secondaryButton(c, iconFolder, m.UploadChooseFolder(), false).Clicked() {
				a.pickDirectory()
			}
		})
	})
}

func (a *App) recentActivity(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	recent := make([]gui.TransferDTO, 0, recentTransfers)
	recent = append(recent, a.active...)
	recent = append(recent, a.history...)
	recent = recent[:min(len(recent), recentTransfers)]
	ui.Column(c).Gap(10).Children(func() {
		sectionTitle(c, m.UploadRecent(), func() {
			link := ui.ButtonBase(c).Label(m.CommonViewAll()).Padding(4, 12).Radius(radiusPill).Cursor(ui.CursorPointer)
			if link.Hovered() {
				link.Background(t.SurfaceHover)
			}
			link.Children(func() { ui.Text(c, m.CommonViewAll()).FontWeight(500) })
			if link.Clicked() {
				a.router.Push("/activity")
			}
		})
		if len(recent) == 0 {
			ui.Text(c, m.UploadNothingYet()).TextColor(t.TextMuted)
			return
		}
		card(c, func() {
			ui.Column(c).Gap(1).Dividers(1, t.Border).Children(func() {
				for i := range recent {
					a.transferRow(c.Key(recent[i].ID), recent[i], true)
				}
			})
		}).Padding(0)
	})
}

func (a *App) reviewDialog(c *ui.Context, m locales.Messages) {
	r := &a.pages.review
	ui.DialogBase(c, &r.open, func(backdrop, panel ui.Element) {
		dialogFrame(c, backdrop, panel, reviewWidth)
		a.reviewHeader(c, m)
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Padding(8, 28, 24).Gap(20).Children(func() { a.reviewBody(c, m) })
		})
		a.reviewFooter(c, m)
	})
}

func (a *App) reviewHeader(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	r := &a.pages.review
	ui.Row(c).Padding(24, 20, 8, 28).Gap(12).Shrink(0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, m.ReviewTitle()).FontSize(20).FontWeight(700)
			if r.plan != nil {
				ui.Text(c, r.plan.SourceSummary).TextColor(t.TextMuted).SingleLine()
			}
		})
		if iconButton(c, iconX, m.CommonClose()).Clicked() {
			r.open = false
		}
	})
}

func (a *App) reviewBody(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	r := &a.pages.review
	if r.loading {
		ui.Row(c).Gap(8).Children(func() {
			ui.Spinner(c).Label(m.ReviewChecking())
			ui.Text(c, m.ReviewChecking()).TextColor(t.TextMuted)
		})
	}
	if r.err != "" {
		banner(c, toneDanger, iconWarningCircle, "", r.err, "", nil)
	}
	if r.plan != nil {
		a.reviewStats(c, m, r.plan)
		a.reviewThumbs(c, m, r.plan)
	}
	a.reviewOptions(c, m)
}

func statTile(c *ui.Context, value, caption string) ui.Element {
	t := c.Theme()
	return ui.Column(c).Grow(1).Basis(0).Padding(14).Gap(2).Radius(radiusFeat).Background(t.Surface).Children(func() {
		ui.Text(c, value).FontSize(20).FontWeight(700).FontFeatures("tnum").SingleLine()
		ui.Text(c, caption).FontSize(12).TextColor(t.TextMuted).SingleLine()
	})
}

func (a *App) reviewStats(c *ui.Context, m locales.Messages, p *gui.UploadPlanDTO) {
	videos := "—"
	if a.pages.review.includeVideos {
		videos = strconv.Itoa(p.VideoCount)
	}
	ui.Row(c).Gap(10).AlignItems(ui.Stretch).Children(func() {
		statTile(c, strconv.Itoa(p.PhotoCount), m.ReviewPhotos())
		statTile(c, videos, m.ReviewVideos())
		statTile(c, formatBytes(p.TotalBytes), m.ReviewTotalSize())
		statTile(c, strconv.Itoa(p.SkippedUnsupported), m.ReviewSkipped()).Tooltip(m.ReviewSkippedTip())
	})
}

func (a *App) reviewThumbs(c *ui.Context, m locales.Messages, p *gui.UploadPlanDTO) {
	if len(p.Items) == 0 {
		if !a.pages.review.loading {
			ui.Text(c, m.ReviewNoFiles()).TextColor(c.Theme().TextMuted)
		}
		return
	}
	shown := p.Items
	more := 0
	if len(shown) > maxThumbs {
		shown = shown[:maxThumbs-1]
		more = len(p.Items) - len(shown)
	}
	ui.Grid(c).Columns(thumbColumns).Gap(3).Children(func() {
		for i := range shown {
			a.thumbTile(c, shown[i])
		}
		if more > 0 {
			t := c.Theme()
			ui.Box(c).AspectRatio(1).Radius(radiusTile).Background(t.Surface).Center().Children(func() {
				ui.Text(c, m.ReviewMore(more)).FontWeight(600).TextColor(t.TextMuted).FontFeatures("tnum")
			})
		}
	})
}

func (a *App) thumbTile(c *ui.Context, it gui.PlanItemDTO) {
	t := c.Theme()
	tile := ui.Box(c).AspectRatio(1).Radius(radiusTile).Clip().Background(t.Surface).Center().Tooltip(it.Name)
	bm := a.thumbnail(it.LocalPath, it.Thumbnail)
	tile.Children(func() {
		if bm != nil {
			ui.Image(c, bm).Fill().Fit(ui.Cover)
			return
		}
		icon := iconImage
		if it.MediaType == "video" {
			icon = iconFilmStrip
		}
		ui.Icon(c, icon).FontSize(24).TextColor(t.TextMuted)
	})
}

func (a *App) reviewOptions(c *ui.Context, m locales.Messages) {
	r := &a.pages.review
	ui.Form(c, func() {
		ui.Field(c, m.OptionIncludeVideos(), func() {
			if ui.Switch(c, &r.includeVideos).Changed() {
				a.replan()
			}
		})
		if r.hasFolder {
			ui.Field(c, m.OptionKeepStructure(), func() {
				supported := r.plan == nil || r.plan.Destination.StructureSupported
				if a.structureSwitch(c, m, &r.keepFolders, supported) {
					a.replan()
				}
			})
		}
		ui.Field(c, m.OptionIfExists(), func() {
			chosen := conflictLabel(m, r.conflict, false)
			labels := make([]string, len(conflictPolicies))
			for i, p := range conflictPolicies {
				labels[i] = conflictLabel(m, p, false)
			}
			if outlined(c, ui.Select(c, &chosen, labels)).MaxWidth(260).Changed() {
				for i, l := range labels {
					if l == chosen {
						r.conflict = conflictPolicies[i]
					}
				}
				a.replan()
			}
		})
		ui.Field(c, m.ReviewUploadTo(), func() { a.reviewDestination(c, m) })
	})
}

func (a *App) structureSwitch(c *ui.Context, m locales.Messages, keep *bool, supported bool) bool {
	t := c.Theme()
	shown := *keep && supported
	hint := m.OptionKeepStructureHint()
	if !supported {
		hint = m.OptionKeepStructureAlbumHint()
	}
	changed := false
	ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
		if ui.Switch(c, &shown).Disabled(!supported).Changed() {
			*keep, changed = shown, true
		}
		ui.Text(c, hint).FontSize(12).TextColor(t.TextMuted).Grow(1).MinWidth(0)
	})
	return changed
}

func (a *App) reviewDestination(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	r := &a.pages.review
	name := ""
	label, k := m.DestChecking(), toneAccent
	message := ""
	if r.plan != nil {
		dest := r.plan.Destination
		name = dest.Name
		if !r.loading {
			label, k = capabilityStatus(m, dest.Status)
			if !destinationReadable(dest.Status) {
				message = dest.Message
			}
		}
	}
	ui.Column(c).Gap(6).Children(func() {
		a.destinationRow(c, m, a.destinationPath(m, r.nodeID, name), func() {
			a.openPicker(r.nodeID, func(id string) {
				r.nodeID = id
				a.replan()
			})
		}, func() {
			if r.err == "" {
				statusPill(c, label, k).Shrink(0)
			}
		})
		if message != "" {
			ui.Text(c, message).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (a *App) reviewFooter(c *ui.Context, m locales.Messages) {
	t := c.Theme()
	r := &a.pages.review
	ui.Row(c).Padding(16, 28, 24).Gap(8).Shrink(0).Children(func() {
		ui.Text(c, m.ReviewFooterNote()).FontSize(12).TextColor(t.TextMuted).Grow(1)
		if secondaryButton(c, nil, m.CommonCancel(), false).Clicked() {
			r.open = false
		}
		count := 0
		if r.plan != nil {
			count = r.plan.TotalCount
		}
		if primaryButton(c, nil, m.ReviewConfirm(count), !r.canConfirm()).Clicked() {
			a.startUpload()
		}
	})
}
