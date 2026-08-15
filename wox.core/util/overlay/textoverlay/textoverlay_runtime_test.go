package textoverlay

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

func TestRuntimeTextWindowGrowsUntilMaximumHeight(t *testing.T) {
	if height := runtimeTextWindowHeight(240, 0, 600); height != 240 {
		t.Fatalf("short content height = %v, want 240", height)
	}
	if height := runtimeTextWindowHeight(720, 0, 600); height != 600 {
		t.Fatalf("long content height = %v, want 600", height)
	}
}

func TestTextOverlayFontSize(t *testing.T) {
	if size := textOverlayFontSize(Options{}); size != DefaultFontSize {
		t.Fatalf("default font size = %v, want %v", size, DefaultFontSize)
	}
	if size := textOverlayFontSize(Options{FontSize: 12}); size != 12 {
		t.Fatalf("custom font size = %v, want 12", size)
	}
}

func TestTextOverlayPadding(t *testing.T) {
	fallback := textOverlayPadding(Options{})
	if fallback.Left != runtimeTextPaddingX || fallback.Top != runtimeTextPaddingY {
		t.Fatalf("default padding = %+v, want %v/%v", fallback, runtimeTextPaddingX, runtimeTextPaddingY)
	}
	custom := textOverlayPadding(Options{Padding: woxwidget.Insets{Left: 11, Top: 8, Right: 11, Bottom: 8}})
	if custom.Left != 11 || custom.Top != 8 {
		t.Fatalf("custom padding = %+v, want 11/8", custom)
	}
}

func TestRuntimeTextTitleBarIsReservedForDialogStyleOverlays(t *testing.T) {
	if runtimeTextUsesTitleBar(Options{Closable: true}) {
		t.Fatal("compact auto-close notification unexpectedly uses a title bar")
	}
	if runtimeTextUsesTitleBar(Options{Closable: true, Window: overlay.WindowOptions{CloseOnEscape: true}}) {
		t.Fatal("untitled closeable overlay unexpectedly uses a title bar")
	}
	if !runtimeTextUsesTitleBar(Options{Title: "Translate", Closable: true, Window: overlay.WindowOptions{CloseOnEscape: true}}) {
		t.Fatal("titled overlay does not use a title bar")
	}
	if !runtimeTextUsesTitleBar(Options{ShowCopyButton: true}) {
		t.Fatal("copyable overlay does not use a title bar")
	}
}

func TestRuntimeTextTitleBarContainsCopyAndCloseControls(t *testing.T) {
	instance := &runtimeTextOverlay{
		options:   Options{Title: "Summarize", Closable: true, ShowCopyButton: true, Window: overlay.WindowOptions{Movable: true}},
		titleIcon: &woxui.Image{},
	}
	titleBar := instance.buildTitleBar(420, woxui.Color{R: 255, G: 255, B: 255, A: 255}).(woxwidget.Stack)
	controls := map[woxwidget.Key]bool{}
	hasTitle := false
	hasIcon := false
	hasCopyTooltip := false
	for _, child := range titleBar.Children {
		switch widget := child.Child.(type) {
		case woxwidget.Stateful:
			controls[widget.Key] = true
			if widget.Key == "text-overlay-copy" {
				hasCopyTooltip = widget.Widget.(woxcomponent.IconButtonProps).OnHoverAt != nil
			}
		case woxwidget.TextBlock:
			hasTitle = widget.Value == "Summarize"
		case woxwidget.Image:
			hasIcon = true
		}
	}
	for _, key := range []woxwidget.Key{"text-overlay-copy", "text-overlay-close"} {
		if !controls[key] {
			t.Fatalf("text overlay title bar missing %q", key)
		}
	}
	if !hasTitle || !hasIcon {
		t.Fatalf("text overlay title bar title/icon = %v/%v, want both", hasTitle, hasIcon)
	}
	if !hasCopyTooltip {
		t.Fatal("text overlay copy button does not expose a tooltip hover trigger")
	}
}

func TestRuntimeTextDefaultFontIsReadable(t *testing.T) {
	if DefaultFontSize != 14 {
		t.Fatalf("default font size = %v, want 14", DefaultFontSize)
	}
}

func TestRuntimeTextCopyTooltipIsAnchoredAboveTitleButton(t *testing.T) {
	x, y := runtimeTextCopyTooltipAnchor(woxui.Rect{X: 100, Y: 200}, woxui.Rect{X: 320, Y: 4, Width: 32, Height: 32})
	if x != 436 || y != 200 {
		t.Fatalf("copy tooltip anchor = (%v, %v), want (436, 200)", x, y)
	}
}
