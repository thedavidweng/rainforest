package view

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg icons/icons.txt
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
	iconArrowClockwise        = loadIcon("arrow-clockwise")
	iconArrowSquareOut        = loadIcon("arrow-square-out")
	iconArrowsClockwise       = loadIcon("arrows-clockwise")
	iconArrowsDownUp          = loadIcon("arrows-down-up")
	iconArrowsDownUpBold      = loadIcon("arrows-down-up-bold")
	iconArrowsDownUpFill      = loadIcon("arrows-down-up-fill")
	iconCaretDown             = loadIcon("caret-down")
	iconCaretRight            = loadIcon("caret-right")
	iconCheck                 = loadIcon("check")
	iconCheckCircle           = loadIcon("check-circle")
	iconCircleDashed          = loadIcon("circle-dashed")
	iconCircleNotch           = loadIcon("circle-notch")
	iconClock                 = loadIcon("clock")
	iconClockCounterClockwise = loadIcon("clock-counter-clockwise")
	iconCloud                 = loadIcon("cloud")
	iconCloudArrowDown        = loadIcon("cloud-arrow-down")
	iconCloudArrowDownBold    = loadIcon("cloud-arrow-down-bold")
	iconCloudArrowDownFill    = loadIcon("cloud-arrow-down-fill")
	iconCloudArrowUp          = loadIcon("cloud-arrow-up")
	iconCloudArrowUpBold      = loadIcon("cloud-arrow-up-bold")
	iconCloudArrowUpFill      = loadIcon("cloud-arrow-up-fill")
	iconCloudCheck            = loadIcon("cloud-check")
	iconCloudSlash            = loadIcon("cloud-slash")
	iconDotsThree             = loadIcon("dots-three")
	iconDownloadSimple        = loadIcon("download-simple")
	iconFile                  = loadIcon("file")
	iconFilmStrip             = loadIcon("film-strip")
	iconFolder                = loadIcon("folder")
	iconFolderBold            = loadIcon("folder-bold")
	iconFolderFill            = loadIcon("folder-fill")
	iconFolderOpen            = loadIcon("folder-open")
	iconGear                  = loadIcon("gear")
	iconGearBold              = loadIcon("gear-bold")
	iconGearFill              = loadIcon("gear-fill")
	iconGlobe                 = loadIcon("globe")
	iconHardDrive             = loadIcon("hard-drive")
	iconImage                 = loadIcon("image")
	iconImages                = loadIcon("images")
	iconInfo                  = loadIcon("info")
	iconListChecks            = loadIcon("list-checks")
	iconMinus                 = loadIcon("minus")
	iconPause                 = loadIcon("pause")
	iconPauseCircle           = loadIcon("pause-circle")
	iconPencilSimple          = loadIcon("pencil-simple")
	iconPlay                  = loadIcon("play")
	iconPlus                  = loadIcon("plus")
	iconShieldCheck           = loadIcon("shield-check")
	iconSignIn                = loadIcon("sign-in")
	iconSignOut               = loadIcon("sign-out")
	iconStethoscope           = loadIcon("stethoscope")
	iconTrash                 = loadIcon("trash")
	iconUploadSimple          = loadIcon("upload-simple")
	iconUser                  = loadIcon("user")
	iconWarning               = loadIcon("warning")
	iconWarningCircle         = loadIcon("warning-circle")
	iconX                     = loadIcon("x")
	iconXCircle               = loadIcon("x-circle")
)
