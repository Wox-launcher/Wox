//go:build linux

package screenshot

import (
	"context"
	"image"
	"slices"
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestLinuxScreenshotObjectCoordinates validates toolkit coordinate space against frozen X11 geometry at fractional and integer scaling.
func TestLinuxScreenshotObjectCoordinates(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		logical := Rect{X: 80, Y: 40, Width: 400, Height: 240}
		physical := Rect{X: logical.X * scale, Y: logical.Y * scale, Width: logical.Width * scale, Height: logical.Height * scale}
		window := linuxScreenshotWindow{frame: physical, client: physical}
		if x, y, matched := linuxATSPICoordinateScale(logical, window, scale, scale); !matched || x != scale || y != scale {
			t.Fatalf("scale %v: logical toolkit mapping = %v,%v,%v", scale, x, y, matched)
		}
		if x, y, matched := linuxATSPICoordinateScale(physical, window, scale, scale); !matched || x != 1 || y != 1 {
			t.Fatalf("scale %v: physical toolkit mapping = %v,%v,%v", scale, x, y, matched)
		}
		moved := logical
		moved.X += 10
		if _, _, matched := linuxATSPICoordinateScale(moved, window, scale, scale); matched {
			t.Fatal("accepted a moved toolkit window")
		}
		candidates, _ := linuxX11ScreenshotObjectSelection([]linuxScreenshotWindow{window}, Rect{Width: 800, Height: 600}, image.NewRGBA(image.Rect(0, 0, int(800*scale), int(600*scale))))
		if !slices.Equal(candidates, []Rect{logical}) {
			t.Fatalf("scale %v: capture mapping = %+v", scale, candidates)
		}
	}
}

// TestLinuxScreenshotObjectClip clips a partially off-root window without mixing logical units and capture pixels.
func TestLinuxScreenshotObjectClip(t *testing.T) {
	windows := []linuxScreenshotWindow{{frame: Rect{X: -100, Y: -80, Width: 500, Height: 400}}}
	candidates, _ := linuxX11ScreenshotObjectSelection(windows, Rect{Width: 800, Height: 600}, image.NewRGBA(image.Rect(0, 0, 1600, 1200)))
	if !slices.Equal(candidates, []Rect{{Width: 200, Height: 160}}) {
		t.Fatalf("clipped window = %+v", candidates)
	}
}

// TestLinuxAccessibleParentDecoding checks the real D-Bus variant layout used for hierarchy traversal and cancellation before bus access.
func TestLinuxAccessibleParentDecoding(t *testing.T) {
	parent := linuxAccessibleRef{Bus: ":1.42", Path: "/org/a11y/atspi/accessible/7"}
	variant := dbus.MakeVariant([]any{parent.Bus, parent.Path})
	var decoded linuxAccessibleRef
	if err := dbus.Store([]any{variant.Value()}, &decoded); err != nil || decoded != parent {
		t.Fatalf("parent = %+v, err = %v", decoded, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if path := linuxATSPIObjectPath(ctx, linuxScreenshotWindow{pid: 42}, Point{}, 1, 1); path != nil {
		t.Fatal("cancelled query returned objects")
	}
}
