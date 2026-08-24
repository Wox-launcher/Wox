package imageoverlay

import (
	"path/filepath"
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
	"wox/util/screen"
)

func TestImageOverlayTitleUsesSourceName(t *testing.T) {
	if title := imageOverlayTitle(common.NewWoxImageAbsolutePath(filepath.Join("screenshots", "capture.png"))); title != "capture.png" {
		t.Fatalf("file title = %q, want capture.png", title)
	}
	if title := imageOverlayTitle(common.NewWoxImageUrl("https://example.com/images/photo.jpg")); title != "photo.jpg" {
		t.Fatalf("URL title = %q, want photo.jpg", title)
	}
}

func TestParseImageOverlayCSSColor(t *testing.T) {
	if color := parseImageOverlayCSSColor("#1F242E", woxui.Color{R: 1, G: 2, B: 3, A: 4}); color.R != 0x1F || color.G != 0x24 || color.B != 0x2E || color.A != 255 {
		t.Fatalf("hex color = %+v, want #1F242E", color)
	}
	if color := parseImageOverlayCSSColor("rgba(31, 36, 46, 0.5)", woxui.Color{}); color.R != 31 || color.G != 36 || color.B != 46 || color.A != 127 {
		t.Fatalf("rgba color = %+v, want rgba(31,36,46,0.5)", color)
	}
	fallback := woxui.Color{R: 9, G: 8, B: 7, A: 6}
	if color := parseImageOverlayCSSColor("not-a-color", fallback); color != fallback {
		t.Fatalf("invalid color = %+v, want fallback %+v", color, fallback)
	}
}

func TestImageOverlayThemeColorsFollowsProvider(t *testing.T) {
	SetThemeProvider(func() common.Theme {
		return common.Theme{
			AppBackgroundColor:             "#112233",
			ActionContainerBackgroundColor: "#112233",
			ActionItemFontColor:            "#445566",
			PreviewSplitLineColor:          "rgba(1, 2, 3, 0.25)",
		}
	})
	defer SetThemeProvider(nil)
	colors := imageOverlayThemeColors()
	if colors.Background != (woxui.Color{R: 0x11, G: 0x22, B: 0x33, A: 255}) {
		t.Fatalf("background = %+v, want #112233", colors.Background)
	}
	if colors.Foreground != (woxui.Color{R: 0x44, G: 0x55, B: 0x66, A: 255}) {
		t.Fatalf("foreground = %+v, want #445566", colors.Foreground)
	}
	if colors.Border.A != 63 {
		t.Fatalf("border alpha = %v, want 63", colors.Border.A)
	}
	if !colors.Dark {
		t.Fatal("dark background should report a dark theme")
	}
	SetThemeProvider(func() common.Theme {
		return common.Theme{
			AppBackgroundColor:             "#F5F5F5",
			ActionContainerBackgroundColor: "#112233",
		}
	})
	light := imageOverlayThemeColors()
	if light.Dark {
		t.Fatal("light AppBackgroundColor should request a light overlay appearance")
	}
	if light.Background != (woxui.Color{R: 0xF5, G: 0xF5, B: 0xF5, A: 255}) {
		t.Fatalf("light background = %+v, want AppBackgroundColor #F5F5F5", light.Background)
	}
	SetThemeProvider(nil)
	fallback := imageOverlayThemeColors()
	if fallback.Background != (woxui.Color{R: 24, G: 24, B: 26, A: 255}) {
		t.Fatalf("fallback background = %+v, want dark default", fallback.Background)
	}
}

func TestImageOverlayColorIsDark(t *testing.T) {
	if !imageOverlayColorIsDark(woxui.Color{R: 24, G: 24, B: 26, A: 255}) {
		t.Fatal("near-black color should be dark")
	}
	if imageOverlayColorIsDark(woxui.Color{R: 236, G: 240, B: 240, A: 255}) {
		t.Fatal("near-white color should not be dark")
	}
}

func TestImageOverlayMacOSTitleOmitsIcon(t *testing.T) {
	chrome := buildImageOverlayChrome(imageOverlayTitleBarProps{
		Width: 400, Height: 280, Title: "shot.png", Platform: "darwin",
		Logo:   &woxui.Image{},
		Colors: ThemeColors{Foreground: woxui.Color{A: 255}},
	}, "", "", nil, nil).(woxwidget.Stack)
	title, ok := chrome.Children[3].Child.(woxwidget.Align)
	if !ok {
		t.Fatalf("macOS title = %T, want Align", chrome.Children[3].Child)
	}
	text, ok := title.Child.(woxwidget.TextBlock)
	if !ok || text.Value != "shot.png" {
		t.Fatalf("macOS title child = %#v, want title text only", title.Child)
	}
}

func TestImageOverlayChromePaintsThemeBackground(t *testing.T) {
	background := woxui.Color{R: 245, G: 245, B: 245, A: 255}
	chrome := buildImageOverlayChrome(imageOverlayTitleBarProps{
		Width: 400, Height: 280, Title: "shot.png", Platform: "darwin",
		Colors: ThemeColors{Background: background, Foreground: woxui.Color{A: 255}, Toolbar: woxui.Color{A: 255}},
	}, "", "", nil, nil).(woxwidget.Stack)
	panel, ok := chrome.Children[0].Child.(woxwidget.Container)
	if !ok {
		t.Fatalf("chrome panel = %T, want Container", chrome.Children[0].Child)
	}
	if panel.Color != background {
		t.Fatalf("chrome fill = %+v, want theme AppBackgroundColor %+v", panel.Color, background)
	}
}

func TestImageOverlayChromeLeavesWindowsTitleBarTransparent(t *testing.T) {
	chrome := buildImageOverlayChrome(imageOverlayTitleBarProps{
		Width: 400, Height: 280, Title: "shot.png", Platform: "windows",
		Colors: ThemeColors{Background: woxui.Color{A: 255}, Foreground: woxui.Color{A: 255}, Toolbar: woxui.Color{A: 255}},
	}, "", "", nil, nil).(woxwidget.Stack)
	background := chrome.Children[0]
	panel, ok := background.Child.(woxwidget.Container)
	if !ok {
		t.Fatalf("Windows chrome panel = %T, want Container", background.Child)
	}
	if background.Top != imageOverlayTitleBarHeight || panel.Height != 280-imageOverlayTitleBarHeight {
		t.Fatalf("Windows background = top %v height %v, want title bar left transparent", background.Top, panel.Height)
	}
}

func TestImageOverlayChromeOmitsWidgetWindowOutline(t *testing.T) {
	for _, platform := range []string{"windows", "darwin", "linux"} {
		chrome := buildImageOverlayChrome(imageOverlayTitleBarProps{
			Width: 400, Height: 280, Title: "clipboard", Platform: platform,
			Colors: ThemeColors{Background: woxui.Color{A: 255}, Border: woxui.Color{A: 30}},
		}, "", "", nil, nil).(woxwidget.Stack)
		panel, ok := chrome.Children[0].Child.(woxwidget.Container)
		if !ok {
			t.Fatalf("%s panel = %T, want Container", platform, chrome.Children[0].Child)
		}
		if panel.Radius != 0 || panel.BorderWidth != 0 || panel.BorderColor.A != 0 {
			t.Fatalf("%s panel chrome = radius %v border %v/%#v, want platform window corners only", platform, panel.Radius, panel.BorderWidth, panel.BorderColor)
		}
	}
}

func TestImageOverlayDefaultPositionUsesMouseScreenCenter(t *testing.T) {
	x, y, ok := imageOverlayDefaultPosition(screen.Size{X: -1920, Y: 0, Width: 1920, Height: 1080})
	if !ok || x != -960 || y != 540 {
		t.Fatalf("secondary screen center = (%v, %v, %v), want (-960, 540, true)", x, y, ok)
	}
	x, y, ok = imageOverlayDefaultPosition(screen.Size{X: 0, Y: 0, Width: 1440, Height: 900})
	if !ok || x != 720 || y != 450 {
		t.Fatalf("primary screen center = (%v, %v, %v), want (720, 450, true)", x, y, ok)
	}
	if _, _, ok = imageOverlayDefaultPosition(screen.Size{}); ok {
		t.Fatal("empty screen should not produce a default position")
	}
}

func TestNormalizeImageOverlayOptionsKeepsExplicitPinPosition(t *testing.T) {
	opts := normalizeImageOverlayOptions(Options{
		AbsolutePosition: true,
		Anchor:           overlay.AnchorTopLeft,
		OffsetX:          100,
		OffsetY:          200,
	})
	if !opts.AbsolutePosition || opts.Anchor != overlay.AnchorTopLeft || opts.OffsetX != 100 || opts.OffsetY != 200 {
		t.Fatalf("explicit pin position was rewritten: %+v", opts)
	}
}

func TestFitImageOverlaySizeToScreenCapsToPointerDisplay(t *testing.T) {
	width, height := fitImageOverlaySizeToScreen(4000, 2000, screen.Size{Width: 1000, Height: 800})
	if width != 860 || height != 430 {
		t.Fatalf("fitted size = %vx%v, want 860x430", width, height)
	}
	width, height = fitImageOverlaySizeToScreen(400, 300, screen.Size{Width: 1000, Height: 800})
	if width != 400 || height != 300 {
		t.Fatalf("small image size = %vx%v, want original 400x300", width, height)
	}
}
