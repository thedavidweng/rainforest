package view

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg
var iconFiles embed.FS

//go:embed brand-leaf.svg
var brandLeaf []byte

var iconBrandLeaf = ui.MustParseSVG(brandLeaf)

func loadIcon(name string) *ui.SVG {
	data, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic(err)
	}
	return ui.MustParseSVG(data)
}

var (
	iconArrowsDownUp       = loadIcon("arrows-down-up")
	iconArrowsDownUpBold   = loadIcon("arrows-down-up-bold")
	iconArrowsDownUpFill   = loadIcon("arrows-down-up-fill")
	iconCaretDown          = loadIcon("caret-down")
	iconCaretRight         = loadIcon("caret-right")
	iconCheck              = loadIcon("check")
	iconCheckCircle        = loadIcon("check-circle")
	iconCircleDashed       = loadIcon("circle-dashed")
	iconCircleNotch        = loadIcon("circle-notch")
	iconClock              = loadIcon("clock")
	iconCloudArrowDown     = loadIcon("cloud-arrow-down")
	iconCloudArrowDownBold = loadIcon("cloud-arrow-down-bold")
	iconCloudArrowDownFill = loadIcon("cloud-arrow-down-fill")
	iconCloudArrowUp       = loadIcon("cloud-arrow-up")
	iconCloudArrowUpBold   = loadIcon("cloud-arrow-up-bold")
	iconCloudArrowUpFill   = loadIcon("cloud-arrow-up-fill")
	iconDotsThree          = loadIcon("dots-three")
	iconFilmStrip          = loadIcon("film-strip")
	iconFolder             = loadIcon("folder")
	iconFolderBold         = loadIcon("folder-bold")
	iconFolderFill         = loadIcon("folder-fill")
	iconFolderOpen         = loadIcon("folder-open")
	iconGearBold           = loadIcon("gear-bold")
	iconGearFill           = loadIcon("gear-fill")
	iconImage              = loadIcon("image")
	iconInfo               = loadIcon("info")
	iconMinus              = loadIcon("minus")
	iconPause              = loadIcon("pause")
	iconPauseCircle        = loadIcon("pause-circle")
	iconPlay               = loadIcon("play")
	iconPlus               = loadIcon("plus")
	iconSignIn             = loadIcon("sign-in")
	iconWarning            = loadIcon("warning")
	iconWarningCircle      = loadIcon("warning-circle")
	iconX                  = loadIcon("x")
	iconXCircle            = loadIcon("x-circle")
)
