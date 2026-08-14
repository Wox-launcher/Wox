package imageoverlay

import (
	"path/filepath"
	"testing"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

func TestImageOverlayTitleUsesSourceName(t *testing.T) {
	if title := imageOverlayTitle(common.NewWoxImageAbsolutePath(filepath.Join("screenshots", "capture.png"))); title != "capture.png" {
		t.Fatalf("file title = %q, want capture.png", title)
	}
	if title := imageOverlayTitle(common.NewWoxImageUrl("https://example.com/images/photo.jpg")); title != "photo.jpg" {
		t.Fatalf("URL title = %q, want photo.jpg", title)
	}
}

func TestImageOverlayWindowsCloseUsesWebViewTitleBarStyle(t *testing.T) {
	button := overlay.TitleBarCloseButton(true, "image-overlay-close", woxui.Color{R: 245, G: 245, B: 245, A: 255}, func() {}).(woxwidget.Stateful)
	props := button.Widget.(woxcomponent.IconButtonProps)
	if props.Width != 46 || props.Height != imageOverlayTitleBarHeight || props.Radius != 0 {
		t.Fatalf("Windows close button geometry = %vx%v radius %v, want 46x40 radius 0", props.Width, props.Height, props.Radius)
	}
	if props.HoverBackground != (woxui.Color{R: 232, G: 17, B: 35, A: 255}) {
		t.Fatalf("Windows close hover background = %#v, want WebView danger red", props.HoverBackground)
	}
}
