package view

import (
	"context"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const (
	recentTiles   = 8
	recentColumns = 8
)

func (a *App) refreshRecent() {
	a.do(func(ctx context.Context) func() {
		items, err := a.svc.Transfers.RecentUploads(ctx, recentTiles)
		last, lastErr := a.svc.Transfers.LastUploadAt(ctx)
		return func() {
			if err != nil {
				return
			}
			a.recent = items
			if lastErr == nil {
				a.lastUpload = last
			}
		}
	})
}

func (a *App) photosURL() string {
	return "https://www." + regionHost(a.auth.Region) + "/photos"
}

func (a *App) recentUploads(c *ui.Context, m locales.Messages) {
	if len(a.recent) == 0 {
		return
	}
	t := c.Theme()
	shown := a.recent[:min(len(a.recent), recentTiles)]
	last := a.lastUpload
	if last == "" {
		last = shown[0].UpdatedAt
	}
	ui.Column(c).Gap(10).Children(func() {
		sectionTitle(c, m.UploadRecentUploads(), nil)
		card(c, func() {
			ui.Grid(c).Columns(recentColumns).Gap(8).Children(func() {
				for i := range shown {
					a.recentTile(c, shown[i])
				}
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				if when := relativeTime(c, last); when != "" {
					ui.Text(c, m.UploadLastUploaded(when)).TextColor(t.TextMuted).Grow(1).SingleLine()
				} else {
					ui.Box(c).Grow(1)
				}
				ui.Link(c, m.UploadViewInPhotos(), a.photosURL()).FontWeight(500).Disabled(a.preview)
			})
		})
	})
}

func (a *App) recentTile(c *ui.Context, it gui.RecentUploadDTO) {
	t := c.Theme()
	bm := a.thumbnail(it.RemoteNodeID+"|"+it.UpdatedAt, it.Thumbnail)
	ui.Box(c).AspectRatio(1).Radius(radiusFeat).Clip().Background(t.Surface).Center().Tooltip(it.Name).Children(func() {
		if bm != nil {
			ui.Image(c, bm).Fill().Fit(ui.Cover)
			return
		}
		icon := iconImage
		if it.MediaType == "video" {
			icon = iconFilmStrip
		}
		ui.Icon(c, icon).FontSize(28).TextColor(t.TextMuted)
	})
}
