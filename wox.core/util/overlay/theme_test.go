package overlay

import (
	"runtime"
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
)

func TestThemeBackgroundUsesAppBackgroundColor(t *testing.T) {
	fallback := woxui.Color{R: 24, G: 29, B: 38, A: 242}
	if got := ThemeBackground(fallback); got != fallback {
		t.Fatalf("empty provider = %#v, want fallback %#v", got, fallback)
	}

	SetThemeProvider(func() common.Theme {
		return common.Theme{AppBackgroundColor: "rgba(22, 22, 26, 0.52)"}
	})
	defer SetThemeProvider(nil)
	got := ThemeBackground(fallback)
	want := woxui.Color{R: 22, G: 22, B: 26, A: 132}
	if got != want {
		t.Fatalf("theme background = %#v, want %#v", got, want)
	}
	chrome := CurrentThemeChrome()
	if chrome.Background != SurfaceFill(runtime.GOOS, want, false) {
		t.Fatalf("chrome background = %#v, want SurfaceFill of theme AppBackgroundColor", chrome.Background)
	}
	if runtime.GOOS == "linux" && chrome.Background.A != 255 {
		t.Fatalf("linux chrome background = %#v, want opaque because Linux has no acrylic", chrome.Background)
	}
}

func TestCurrentThemeChromeFollowsLightAppearance(t *testing.T) {
	SetThemeProvider(func() common.Theme {
		return common.Theme{AppBackgroundColor: "#F5F5F5", ToolbarFontColor: "#1C1C1E"}
	})
	defer SetThemeProvider(nil)
	chrome := CurrentThemeChrome()
	if !chrome.Light || chrome.Background != (woxui.Color{R: 0xF5, G: 0xF5, B: 0xF5, A: 255}) {
		t.Fatalf("light chrome = %#v, want a light AppBackgroundColor", chrome)
	}
	if chrome.Foreground != (woxui.Color{R: 0x1C, G: 0x1C, B: 0x1E, A: 255}) {
		t.Fatalf("light foreground = %#v, want toolbar text", chrome.Foreground)
	}
}

func TestApplyThemeAppearanceFollowsLightTheme(t *testing.T) {
	SetThemeProvider(func() common.Theme {
		return common.Theme{AppBackgroundColor: "#F5F5F5"}
	})
	defer SetThemeProvider(nil)
	options := WindowOptions{}
	ApplyThemeAppearance(&options)
	if !options.LightAppearance || !options.FollowsThemeAppearance {
		t.Fatalf("appearance = light %v follow %v, want the light Wox theme", options.LightAppearance, options.FollowsThemeAppearance)
	}
}

func TestColorIsDark(t *testing.T) {
	if !ColorIsDark(woxui.Color{R: 22, G: 22, B: 26, A: 133}) {
		t.Fatal("dark wash should request the dark window appearance")
	}
	if ColorIsDark(woxui.Color{R: 245, G: 245, B: 247, A: 200}) {
		t.Fatal("light wash should request the light window appearance")
	}
}

func TestParseThemeCSSColor(t *testing.T) {
	if color := parseThemeCSSColor("#1F242E", woxui.Color{R: 1, G: 2, B: 3, A: 4}); color.R != 0x1F || color.G != 0x24 || color.B != 0x2E || color.A != 255 {
		t.Fatalf("hex color = %+v, want #1F242E", color)
	}
	if color := parseThemeCSSColor("rgba(31, 36, 46, 0.5)", woxui.Color{}); color.R != 31 || color.G != 36 || color.B != 46 || color.A != 127 {
		t.Fatalf("rgba color = %+v, want rgba(31,36,46,0.5)", color)
	}
	fallback := woxui.Color{R: 9, G: 8, B: 7, A: 6}
	if color := parseThemeCSSColor("not-a-color", fallback); color != fallback {
		t.Fatalf("invalid color = %+v, want fallback %+v", color, fallback)
	}
}
