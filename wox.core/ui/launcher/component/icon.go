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
	rasterSize := max(32, int(math.Ceil(float64(size*2))))
	key := svgIconCacheKey{name: name, rasterSize: rasterSize, color: color}
	if cached, ok := svgIconCache.Load(key); ok {
		return cached.(*woxui.Image)
	}
	source := common.UIIcon(name)
	if source.ImageType != "svg" || source.ImageData == "" {
		return nil
	}
	rgba, err := woxsvg.Render(source.ImageData, rasterSize, rasterSize)
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
