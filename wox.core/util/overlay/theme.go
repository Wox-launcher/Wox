package overlay

import (
	"encoding/hex"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"wox/common"
	woxui "wox/ui/runtime"
)

var (
	themeMu       sync.Mutex
	themeProvider func() common.Theme
)

// SetThemeProvider registers the Wox theme source used by overlays that paint
// AppBackgroundColor over native window material.
func SetThemeProvider(provider func() common.Theme) {
	themeMu.Lock()
	themeProvider = provider
	themeMu.Unlock()
}

// ThemeChrome is the window wash and readable text color shared by overlays.
type ThemeChrome struct {
	Background woxui.Color
	Foreground woxui.Color
	Light      bool
}

var (
	themeFallbackBackground = woxui.Color{R: 24, G: 29, B: 38, A: 242}
	themeFallbackDarkText   = woxui.Color{R: 246, G: 246, B: 246, A: 255}
	themeFallbackLightText  = woxui.Color{R: 28, G: 28, B: 30, A: 255}
)

// ThemeBackground is the painted wash used by launcher and WebView windows.
// Notes and text overlays use the same color so text stays readable over
// native acrylic or vibrancy instead of sitting on bare window material.
func ThemeBackground(fallback woxui.Color) woxui.Color {
	themeMu.Lock()
	provider := themeProvider
	themeMu.Unlock()
	if provider == nil {
		return fallback
	}
	return parseThemeCSSColor(provider().AppBackgroundColor, fallback)
}

// CurrentThemeChrome resolves AppBackgroundColor and a matching foreground.
func CurrentThemeChrome() ThemeChrome {
	background := ThemeBackground(themeFallbackBackground)
	fallbackText := themeFallbackDarkText
	if !ColorIsDark(background) {
		fallbackText = themeFallbackLightText
	}
	foreground := fallbackText
	themeMu.Lock()
	provider := themeProvider
	themeMu.Unlock()
	if provider != nil {
		theme := provider()
		foreground = parseThemeCSSColor(theme.ActionItemFontColor, parseThemeCSSColor(theme.ToolbarFontColor, fallbackText))
	}
	light := !ColorIsDark(background)
	return ThemeChrome{Background: SurfaceFill(runtime.GOOS, background, light), Foreground: foreground, Light: light}
}

// ApplyThemeAppearance makes an overlay follow the current Wox light/dark material.
func ApplyThemeAppearance(options *WindowOptions) {
	if options == nil {
		return
	}
	options.LightAppearance = CurrentThemeChrome().Light
	options.FollowsThemeAppearance = true
}

// ColorIsDark reports whether a fill should request the dark window appearance.
func ColorIsDark(color woxui.Color) bool {
	linear := func(value uint8) float64 {
		channel := float64(value) / 255
		if channel <= 0.03928 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(color.R)+0.7152*linear(color.G)+0.0722*linear(color.B) < 0.5
}

func parseThemeCSSColor(value string, fallback woxui.Color) woxui.Color {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		raw := strings.TrimPrefix(value, "#")
		if len(raw) == 6 || len(raw) == 8 {
			decoded, err := hex.DecodeString(raw)
			if err == nil {
				color := woxui.Color{R: decoded[0], G: decoded[1], B: decoded[2], A: 255}
				if len(decoded) == 4 {
					color.A = decoded[3]
				}
				return color
			}
		}
	}
	if (strings.HasPrefix(value, "rgb(") || strings.HasPrefix(value, "rgba(")) && strings.HasSuffix(value, ")") {
		start := strings.IndexByte(value, '(')
		parts := strings.Split(value[start+1:len(value)-1], ",")
		if len(parts) == 3 || len(parts) == 4 {
			channels := make([]float64, len(parts))
			valid := true
			for index, part := range parts {
				channel, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
				if err != nil {
					valid = false
					break
				}
				channels[index] = channel
			}
			if valid {
				alpha := float64(255)
				if len(channels) == 4 {
					alpha = channels[3]
					if alpha <= 1 {
						alpha = math.Floor(alpha * 255)
					}
				}
				return woxui.Color{
					R: themeColorByte(channels[0]),
					G: themeColorByte(channels[1]),
					B: themeColorByte(channels[2]),
					A: themeColorByte(alpha),
				}
			}
		}
	}
	return fallback
}

func themeColorByte(value float64) uint8 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(value)
}
