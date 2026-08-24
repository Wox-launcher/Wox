package component

import (
	"math"
	"sync"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	woxsvg "wox/util/svg"
)

type svgIconCacheKey struct {
	name       string
	rasterSize int
	color      woxui.Color
}

var svgIconCache sync.Map

// svgIcon renders and caches a categorized common SVG for pure component views.
func svgIcon(name string, size float32, color woxui.Color) woxwidget.Widget {
	image := svgIconImage(name, size, color)
	if image == nil {
		return woxwidget.Painter{Width: size, Height: size}
	}
	return woxwidget.Image{Source: image, Width: size, Height: size}
}

// svgIconImage returns the cached raster used by static and animated icon widgets.
func svgIconImage(name string, size float32, color woxui.Color) *woxui.Image {
	source := common.UIIcon(name)
	if source.ImageType != "svg" || source.ImageData == "" {
		return nil
	}
	return svgSourceImage(name, source.ImageData, size, color)
}

func svgSourceImage(name, source string, size float32, color woxui.Color) *woxui.Image {
	rasterSize := max(32, int(math.Ceil(float64(size*2))))
	key := svgIconCacheKey{name: name, rasterSize: rasterSize, color: color}
	if cached, ok := svgIconCache.Load(key); ok {
		return cached.(*woxui.Image)
	}
	rgba, err := woxsvg.Render(source, rasterSize, rasterSize)
	if err != nil {
		return nil
	}
	for index := 0; index < len(rgba.Pix); index += 4 {
		alpha := uint8((uint16(rgba.Pix[index+3])*uint16(color.A) + 127) / 255)
		rgba.Pix[index] = uint8((uint16(color.R)*uint16(alpha) + 127) / 255)
		rgba.Pix[index+1] = uint8((uint16(color.G)*uint16(alpha) + 127) / 255)
		rgba.Pix[index+2] = uint8((uint16(color.B)*uint16(alpha) + 127) / 255)
		rgba.Pix[index+3] = alpha
	}
	image, err := woxui.NewImage(rgba)
	if err != nil {
		return nil
	}
	actual, _ := svgIconCache.LoadOrStore(key, image)
	return actual.(*woxui.Image)
}

// CloseGlyph returns the shared SVG close icon.
func CloseGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return svgIcon("control.close", size, color)
}

// SearchGlyph returns the shared SVG search icon.
func SearchGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 18
	}
	return svgIcon("control.search", size, color)
}

// WindowsGlyph returns the four-pane Windows logo used by simulated desktop chrome.
func WindowsGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("system.windows", size, color)
}

// BrowserGlyph returns the compact browser icon used by simulated pinned apps.
func BrowserGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("system.browser", size, color)
}

// CodeGlyph returns the compact editor icon used by simulated pinned apps.
func CodeGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("system.code", size, color)
}

// WifiGlyph returns the compact wireless indicator used by system chrome.
func WifiGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("system.wifi", size, color)
}

// VolumeGlyph returns the compact speaker indicator used by system chrome.
func VolumeGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("system.volume", size, color)
}

// BluetoothGlyph returns the compact Bluetooth indicator used by system chrome.
func BluetoothGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("system.bluetooth", size, color)
}

// AppsGlyph returns the launcher grid icon used by simulated Windows chrome.
func AppsGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("usage.apps", size, color)
}

// FolderGlyph returns the outlined folder icon used by simulated pinned apps.
func FolderGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.folder-open", size, color)
}

// AccessibilityGlyph returns the monochrome Universal Access figure used by permission rows.
func AccessibilityGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 20
	}
	return svgIcon("control.accessibility", size, color)
}

// DiskAccessGlyph returns the monochrome internal-drive icon used by permission rows.
func DiskAccessGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 20
	}
	return svgIcon("control.internal-drive", size, color)
}

// AddGlyph returns the shared add icon.
func AddGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.add", size, color)
}

// DeleteGlyph returns the shared delete icon.
func DeleteGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.delete", size, color)
}

// ChatBubbleGlyph returns Flutter's filled conversation icon.
func ChatBubbleGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.chat-bubble", size, color)
}

// FullscreenGlyph returns the shared enter or exit fullscreen icon.
func FullscreenGlyph(size float32, color woxui.Color, fullscreen bool) woxwidget.Widget {
	if size <= 0 {
		size = 18
	}
	name := "control.fullscreen"
	if fullscreen {
		name = "control.fullscreen-exit"
	}
	return svgIcon(name, size, color)
}

// MenuGlyph returns the shared SVG menu icon.
func MenuGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 18
	}
	return svgIcon("control.menu", size, color)
}

// SidebarGlyph returns the shared sidebar visibility icon.
func SidebarGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 18
	}
	return svgIcon("control.sidebar", size, color)
}

// ChevronGlyph returns the shared SVG disclosure icon.
func ChevronGlyph(size float32, color woxui.Color, expanded bool) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	name := "control.chevron-down"
	if expanded {
		name = "control.chevron-up"
	}
	return svgIcon(name, size, color)
}

// CopyGlyph returns the shared SVG copy icon.
func CopyGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 14
	}
	return svgIcon("control.copy", size, color)
}

// KeyboardGlyph returns the shared keyboard icon used by shortcut overviews.
func KeyboardGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 20
	}
	return svgIcon("control.keyboard", size, color)
}

// EditGlyph returns the shared SVG edit icon.
func EditGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 14
	}
	return svgIcon("control.edit", size, color)
}

// RefreshGlyph returns the shared SVG refresh icon.
func RefreshGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 14
	}
	return svgIcon("control.refresh", size, color)
}

// ArrowLeftGlyph returns the shared SVG back-navigation icon.
func ArrowLeftGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return svgIcon("control.arrow-left", size, color)
}

// ArrowRightGlyph returns the shared SVG forward-navigation icon.
func ArrowRightGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return svgIcon("control.arrow-right", size, color)
}

// ExternalGlyph returns the shared SVG open-in-browser icon.
func ExternalGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 14
	}
	return svgIcon("control.external", size, color)
}

// DebugGlyph returns the shared SVG debug icon.
func DebugGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return svgIcon("settings.debug", size, color)
}

// ClockGlyph returns the shared SVG clock icon.
func ClockGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	return svgIcon("control.clock", size, color)
}

// FilterListGlyph returns the shared SVG compact filter-list icon.
func FilterListGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 15
	}
	return svgIcon("control.filter-list", size, color)
}

// CheckGlyph returns the shared SVG check icon.
func CheckGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 12
	}
	return svgIcon("control.check", size, color)
}

// CheckCircleGlyph returns the shared successful-status icon.
func CheckCircleGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.check-circle", size, color)
}

// ErrorGlyph returns the shared failed-status icon.
func ErrorGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.error", size, color)
}

// ToolGlyph returns Flutter's outlined tool-call icon.
func ToolGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.build", size, color)
}

// ArticleGlyph returns Flutter's outlined page-fetch icon.
func ArticleGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.article", size, color)
}

// TerminalGlyph returns Flutter's generic tool-activity icon.
func TerminalGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.terminal", size, color)
}

// PlayArrowGlyph returns Flutter's streaming tool-status icon.
func PlayArrowGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.play-arrow", size, color)
}

// HourglassGlyph returns Flutter's pending tool-status icon.
func HourglassGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.hourglass-empty", size, color)
}

// ModelTrainingGlyph returns Flutter's rounded AI model icon.
func ModelTrainingGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.model-training", size, color)
}

// KeyboardArrowDownGlyph returns Flutter's compact model-selector arrow.
func KeyboardArrowDownGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.arrow-down", size, color)
}

// KeyboardArrowRightGlyph returns Flutter's collapsed disclosure arrow.
func KeyboardArrowRightGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.arrow-right", size, color)
}

// ExtensionGlyph returns Flutter's rounded skill icon.
func ExtensionGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("usage.extension", size, color)
}

// SparklesGlyph returns the shared AI refinement icon.
func SparklesGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.sparkles", size, color)
}

// WaveformGlyph returns the shared transcript waveform icon.
func WaveformGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.waveform", size, color)
}

// MultitrackAudioGlyph returns the shared diagnostic audio icon.
func MultitrackAudioGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.multitrack-audio", size, color)
}

// PlayCircleGlyph returns the shared audio playback icon.
func PlayCircleGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.play-circle", size, color)
}

// PauseGlyph returns the shared audio pause icon.
func PauseGlyph(size float32, color woxui.Color) woxwidget.Widget {
	return svgIcon("control.pause", size, color)
}

// PinGlyph returns the shared thumbtack used to keep a window above others.
func PinGlyph(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 15
	}
	return svgIcon("control.pin", size, color)
}

var formatGlyphNames = map[string]string{
	"block":     "control.format-heading",
	"bold":      "control.format-bold",
	"italic":    "control.format-italic",
	"underline": "control.format-underline",
	"strike":    "control.format-strikethrough",
	"code":      "control.code",
	"link":      "control.link",
	"bullet":    "control.list",
	"ordered":   "control.format-ordered",
	"task":      "control.checkbox.unchecked",
	"quote":     "control.format-quote",
	"divider":   "control.format-divider",
	"more":      "control.more-horizontal",
}

// FormatGlyph returns the shared SVG used by compact editor format bars.
func FormatGlyph(kind string, size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	name := formatGlyphNames[kind]
	if name == "" {
		return woxwidget.Painter{Width: size, Height: size}
	}
	return svgIcon(name, size, color)
}
