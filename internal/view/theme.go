package view

import "github.com/egoist/mygo/ui"

const (
	radiusCard    = 20
	radiusDialog  = 20
	radiusField   = 10
	radiusTile    = 4
	radiusFeat    = 12
	radiusPill    = 999
	pagePadX      = 40
	pagePadTop    = 24
	sectionGap    = 32
	maxContent    = 920
	sidebarWidth  = 248
	headerHeight  = 52
	buttonHeight  = 34
	fieldHeight   = 36
	iconBtnSize   = 32
	navItemHeight = 48
	navIconSize   = 24
)

type palette struct {
	Sidebar     ui.Color
	Card        ui.Color
	NavPill     ui.Color
	NavIcon     ui.Color
	CardBorder  ui.Color
	AccentTint  ui.Color
	SuccessTint ui.Color
	WarningTint ui.Color
	DangerTint  ui.Color
}

func applyTheme(c *ui.Context) {
	t := *c.Theme()
	t.Radius = radiusField
	t.Spacing = 4
	if t.Dark {
		t.Background = ui.Hex("#0E0E0E")
		t.Surface = ui.Hex("#1C1C1C")
		t.SurfaceHover = ui.Hex("#262626")
		t.SurfacePressed = ui.Hex("#303030")
		t.Border = ui.Hex("#2E2E2E")
		t.Text = ui.Hex("#F5F5F5")
		t.TextMuted = ui.Hex("#A3A3A3")
		t.Accent = ui.Hex("#F5F5F5")
		t.AccentHover = ui.Hex("#FFFFFF")
		t.AccentPressed = ui.Hex("#DADADA")
		t.AccentText = ui.Hex("#0F1111")
		t.Success = ui.Hex("#34C38F")
		t.Warning = ui.Hex("#F0A43C")
		t.Danger = ui.Hex("#FF6B6B")
		t.Selection = ui.Hex("#FFFFFF").Alpha(0.22)
		t.Focus = ui.Hex("#FFFFFF").Alpha(0.45)
		t.Inverse = ui.Hex("#F5F5F5")
		t.InverseText = ui.Hex("#0F1111")
	} else {
		t.Background = ui.Hex("#FFFFFF")
		t.Surface = ui.Hex("#F2F2F2")
		t.SurfaceHover = ui.Hex("#EAEAEA")
		t.SurfacePressed = ui.Hex("#E0E0E0")
		t.Border = ui.Hex("#DDDDDD")
		t.Text = ui.Hex("#0F1111")
		t.TextMuted = ui.Hex("#5F6368")
		t.Accent = ui.Hex("#0F1111")
		t.AccentHover = ui.Hex("#2B2E30")
		t.AccentPressed = ui.Hex("#000000")
		t.AccentText = ui.Hex("#FFFFFF")
		t.Success = ui.Hex("#067D62")
		t.Warning = ui.Hex("#B25E00")
		t.Danger = ui.Hex("#C40C2F")
		t.Selection = ui.Hex("#0F1111").Alpha(0.14)
		t.Focus = ui.Hex("#0F1111").Alpha(0.4)
		t.Inverse = ui.Hex("#0F1111")
		t.InverseText = ui.Hex("#FFFFFF")
	}
	c.SetTheme(&t)
}

func paletteOf(c *ui.Context) palette {
	t := c.Theme()
	tint := float32(0.08)
	p := palette{}
	if t.Dark {
		tint = 0.14
		p.Sidebar = ui.Hex("#0E0E0E")
		p.Card = ui.Hex("#161616")
		p.NavPill = ui.Hex("#232323")
		p.NavIcon = ui.Hex("#C8C8C8")
		p.CardBorder = t.Border
	} else {
		p.Sidebar = ui.Hex("#FFFFFF")
		p.Card = ui.Hex("#FFFFFF")
		p.NavPill = ui.Hex("#EFEFEF")
		p.NavIcon = ui.Hex("#3A3D40")
		p.CardBorder = ui.Hex("#E8E8E8")
	}
	p.AccentTint = t.Accent.Alpha(tint)
	p.SuccessTint = t.Success.Alpha(tint)
	p.WarningTint = t.Warning.Alpha(tint)
	p.DangerTint = t.Danger.Alpha(tint)
	return p
}
