//go:build linux

package screenshot

import (
	"image"
	"image/color"
	"testing"
)

// linuxSelectionTestOutputs models a 100% display below a 150% display with a negative desktop origin.
func linuxSelectionTestOutputs() []linuxDesktopOutput {
	outputs := []linuxDesktopOutput{
		{bounds: Rect{X: 0, Y: 80, Width: 192, Height: 108}, source: image.NewRGBA(image.Rect(0, 0, 192, 108))},
		{bounds: Rect{X: -128, Y: 0, Width: 128, Height: 80}, source: image.NewRGBA(image.Rect(0, 0, 192, 120))},
	}
	for index := range outputs {
		source := outputs[index].source
		outputs[index].capture = func() (image.Image, error) { return source, nil }
	}
	return outputs
}

// newLinuxSelectionTestSession prepares the real selection state without mapping native surfaces.
func newLinuxSelectionTestSession(t *testing.T) *linuxScreenshotSelectionSession {
	t.Helper()
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	session, err := newLinuxScreenshotSelectionSession(ScreenshotOptions{}, linuxSelectionTestOutputs(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

// TestLinuxMultiOutputSelectionPreservesNativePixels selects the secondary output without resampling its pixels.
func TestLinuxMultiOutputSelectionPreservesNativePixels(t *testing.T) {
	session := newLinuxSelectionTestSession(t)
	session.pointer(1, PointerEvent{Kind: PointerEnter, Position: Point{X: 10, Y: 20}})
	session.pointer(1, PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 10, Y: 20}})
	session.pointer(1, PointerEvent{Kind: PointerMove, Position: Point{X: 50, Y: 60}})
	session.pointer(1, PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 50, Y: 60}})
	selected := <-session.result
	if selected.output != 1 || selected.selection != (Rect{X: 10, Y: 20, Width: 40, Height: 40}) || selected.outcome.cancelled {
		t.Fatalf("secondary selection = %+v", selected)
	}
	output := session.outputs[selected.output]
	platform := linuxScreenshotEditorPlatform(ScreenshotOptions{}, output.source, output.bounds, nil, nil, output.capture)
	logical := platform.logicalSelection(selected.selection, session.surfaces[1].state.frameSize)
	if logical != (Rect{X: -118, Y: 20, Width: 40, Height: 40}) {
		t.Fatalf("desktop selection = %+v", logical)
	}
	pixels, err := screenshotEditorPixelSelection(output.source.Bounds(), selected.selection, session.surfaces[1].state.frameSize)
	if err != nil || pixels != image.Rect(15, 30, 75, 90) {
		t.Fatalf("150%% native pixel crop = %v, %v", pixels, err)
	}
	recaptured, err := platform.captureDesktop()
	if err != nil || recaptured.source != output.source {
		t.Fatalf("recapture changed output: %+v, %v", recaptured, err)
	}
}

// TestLinuxMultiOutputDragStaysOnStartingDisplay clips a crossing gesture to its original output.
func TestLinuxMultiOutputDragStaysOnStartingDisplay(t *testing.T) {
	session := newLinuxSelectionTestSession(t)
	session.pointer(1, PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 100, Y: 60}})
	session.pointer(0, PointerEvent{Kind: PointerMove, Position: Point{X: 50, Y: 40}})
	session.pointer(0, PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 50, Y: 40}})
	selected := <-session.result
	if selected.output != 1 || selected.selection != (Rect{X: 100, Y: 60, Width: 28, Height: 20}) {
		t.Fatalf("cross-output drag = %+v", selected)
	}
}

// TestLinuxMultiOutputHoverAndKeyboard keeps color shortcuts with the pointer rather than native keyboard focus.
func TestLinuxMultiOutputHoverAndKeyboard(t *testing.T) {
	session := newLinuxSelectionTestSession(t)
	source := session.outputs[1].source.(*image.RGBA)
	source.SetRGBA(30, 30, color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 255})
	var copied string
	session.surfaces[1].state.writeClipboardText = func(value string) error { copied = value; return nil }
	session.pointer(0, PointerEvent{Kind: PointerEnter, Position: Point{X: 20, Y: 20}})
	session.pointer(1, PointerEvent{Kind: PointerEnter, Position: Point{X: 20, Y: 20}})
	if session.surfaces[0].state.pointerInside || !session.surfaces[1].state.pointerInside || session.active != 1 {
		t.Fatal("hover did not transfer to the secondary display")
	}
	if !session.key(KeyEvent{Down: true, Key: Key("h")}) {
		t.Fatal("color shortcut was not handled")
	}
	selected := <-session.result
	if selected.output != 1 || copied != "#123456" || selected.outcome.copiedColor != copied {
		t.Fatalf("color copied from wrong output: value=%q result=%+v", copied, selected)
	}
}

// TestLinuxMultiOutputClickAndCancel covers full-display fallback and one-shot session cancellation.
func TestLinuxMultiOutputClickAndCancel(t *testing.T) {
	session := newLinuxSelectionTestSession(t)
	session.pointer(1, PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 20, Y: 20}})
	session.pointer(1, PointerEvent{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 21, Y: 20}})
	selected := <-session.result
	if selected.output != 1 || selected.selection != (Rect{Width: 128, Height: 80}) {
		t.Fatalf("full-display click = %+v", selected)
	}
	session = newLinuxSelectionTestSession(t)
	session.key(KeyEvent{Down: true, Key: KeyEscape})
	session.key(KeyEvent{Down: true, Key: KeyEscape})
	session.pointer(1, PointerEvent{Kind: PointerDown, Button: PointerButtonPrimary})
	if len(session.result) != 1 || !(<-session.result).outcome.cancelled {
		t.Fatal("cancellation was lost or completed more than once")
	}
}
