//go:build linux

package screenshot

/*
#cgo pkg-config: gtk+-3.0 x11
#include "object_selection_linux_x11.h"
*/
import "C"

import (
	"context"
	"image"
	"math"

	"github.com/godbus/dbus/v5"
	"wox/util"
)

// linuxScreenshotWindow preserves both client and decorated frames so toolkit coordinates can be checked against X11.
type linuxScreenshotWindow struct {
	pid           int
	frame, client Rect
}

type linuxAccessibleRef struct {
	Bus  string
	Path dbus.ObjectPath
}

type linuxAccessibleRect struct {
	X, Y, Width, Height int32
}

// captureLinuxScreenshotWindows freezes EWMH stacking before GTK shows the selection surface.
func captureLinuxScreenshotWindows() []linuxScreenshotWindow {
	if util.IsLinuxWaylandSession() {
		return nil
	}
	var windows [512]C.WoxScreenshotX11Window
	var count C.int32_t
	if err := Call(func() { count = C.wox_screenshot_x11_windows(&windows[0], C.int32_t(len(windows))) }); err != nil {
		return nil
	}
	result := make([]linuxScreenshotWindow, 0, int(count))
	for i := 0; i < int(count); i++ {
		frame := windows[i]
		result = append(result, linuxScreenshotWindow{pid: int(frame.pid),
			frame:  Rect{X: float32(frame.x), Y: float32(frame.y), Width: float32(frame.width), Height: float32(frame.height)},
			client: Rect{X: float32(frame.client_x), Y: float32(frame.client_y), Width: float32(frame.client_width), Height: float32(frame.client_height)}})
	}
	return result
}

// linuxX11ScreenshotObjectSelection maps physical root pixels through the actual captured image, including non-100% scaling.
func linuxX11ScreenshotObjectSelection(windows []linuxScreenshotWindow, bounds Rect, source image.Image) ([]Rect, screenshotObjectQuery) {
	if source == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return nil, nil
	}
	scaleX, scaleY := float32(source.Bounds().Dx())/bounds.Width, float32(source.Bounds().Dy())/bounds.Height
	candidates := make([]Rect, 0, len(windows))
	for _, window := range windows {
		frame := window.frame
		left, top := max(float32(0), frame.X/scaleX), max(float32(0), frame.Y/scaleY)
		right, bottom := min(bounds.Width, (frame.X+frame.Width)/scaleX), min(bounds.Height, (frame.Y+frame.Height)/scaleY)
		if right-left >= 2 && bottom-top >= 2 {
			candidates = append(candidates, Rect{X: left, Y: top, Width: right - left, Height: bottom - top})
		}
	}
	return candidates, func(ctx context.Context, point Point) []Rect {
		physical := Point{X: point.X * scaleX, Y: point.Y * scaleY}
		for _, window := range windows {
			if screenshotEditorRectContains(window.frame, physical) {
				return linuxATSPIObjectPath(ctx, window, physical, scaleX, scaleY)
			}
		}
		return nil
	}
}

// linuxATSPIObjectPath opens the optional accessibility bus only on the query worker and never enables desktop accessibility settings.
func linuxATSPIObjectPath(ctx context.Context, window linuxScreenshotWindow, physical Point, scaleX, scaleY float32) []Rect {
	if ctx.Err() != nil || window.pid <= 0 {
		return nil
	}
	session, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return nil
	}
	defer session.Close()
	var address string
	if session.Object("org.a11y.Bus", "/org/a11y/bus").CallWithContext(ctx, "org.a11y.Bus.GetAddress", 0).Store(&address) != nil || address == "" {
		return nil
	}
	connection, err := dbus.Connect(address, dbus.WithContext(ctx))
	if err != nil {
		return nil
	}
	defer connection.Close()
	registry := linuxAccessibleRef{Bus: "org.a11y.atspi.Registry", Path: "/org/a11y/atspi/accessible/root"}
	apps := linuxATSPIChildren(ctx, connection, registry)
	for _, app := range apps {
		var pid uint32
		if connection.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.GetConnectionUnixProcessID", 0, app.Bus).Store(&pid) != nil || int(pid) != window.pid {
			continue
		}
		for _, root := range linuxATSPIChildren(ctx, connection, app) {
			frame, ok := linuxATSPIFrame(ctx, connection, root)
			if !ok {
				continue
			}
			// Toolkits can expose either physical or logical screen coordinates. Accept only a frame matching the frozen X11 client/window.
			coordinateScaleX, coordinateScaleY, matched := linuxATSPICoordinateScale(frame, window, scaleX, scaleY)
			if !matched {
				continue
			}
			point := Point{X: physical.X / coordinateScaleX, Y: physical.Y / coordinateScaleY}
			var hit linuxAccessibleRef
			if connection.Object(root.Bus, root.Path).CallWithContext(ctx, "org.a11y.atspi.Component.GetAccessibleAtPoint", 0,
				int32(math.Round(float64(point.X))), int32(math.Round(float64(point.Y))), uint32(0)).Store(&hit) != nil {
				return nil
			}
			visited := map[linuxAccessibleRef]bool{root: true}
			for depth := 0; depth < 64 && hit.Bus != "" && hit.Path.IsValid() && !visited[hit] && ctx.Err() == nil; depth++ {
				visited[hit] = true
				var child linuxAccessibleRef
				if connection.Object(hit.Bus, hit.Path).CallWithContext(ctx, "org.a11y.atspi.Component.GetAccessibleAtPoint", 0,
					int32(math.Round(float64(point.X))), int32(math.Round(float64(point.Y))), uint32(0)).Store(&child) != nil || child.Bus == "" || child.Path == "/org/a11y/atspi/null" || visited[child] {
					break
				}
				hit = child
			}
			path := make([]Rect, 0, 16)
			seen := map[linuxAccessibleRef]bool{}
			for depth := 0; depth < 64 && ctx.Err() == nil && hit.Bus != "" && hit.Path != "/org/a11y/atspi/null" && !seen[hit]; depth++ {
				seen[hit] = true
				if rect, ok := linuxATSPIFrame(ctx, connection, hit); ok {
					rect.X *= coordinateScaleX / scaleX
					rect.Width *= coordinateScaleX / scaleX
					rect.Y *= coordinateScaleY / scaleY
					rect.Height *= coordinateScaleY / scaleY
					path = append(path, rect)
				}
				if hit == root {
					current, valid := linuxATSPIFrame(ctx, connection, root)
					sx, sy, matches := linuxATSPICoordinateScale(current, window, scaleX, scaleY)
					if valid && matches && sx == coordinateScaleX && sy == coordinateScaleY {
						return path
					}
					return nil
				}
				var parent dbus.Variant
				if connection.Object(hit.Bus, hit.Path).CallWithContext(ctx, "org.freedesktop.DBus.Properties.Get", 0, "org.a11y.atspi.Accessible", "Parent").Store(&parent) != nil || dbus.Store([]any{parent.Value()}, &hit) != nil {
					break
				}
			}
			return nil
		}
	}
	return nil
}

// linuxATSPIChildren bounds application/window enumeration and uses the request deadline for every remote call.
func linuxATSPIChildren(ctx context.Context, connection *dbus.Conn, ref linuxAccessibleRef) []linuxAccessibleRef {
	var children []linuxAccessibleRef
	if ref.Bus != "" && ref.Path.IsValid() {
		_ = connection.Object(ref.Bus, ref.Path).CallWithContext(ctx, "org.a11y.atspi.Accessible.GetChildren", 0).Store(&children)
	}
	if len(children) > 512 {
		children = children[:512]
	}
	return children
}

// linuxATSPIFrame reads screen geometry without retrieving accessible names or text.
func linuxATSPIFrame(ctx context.Context, connection *dbus.Conn, ref linuxAccessibleRef) (Rect, bool) {
	var frame linuxAccessibleRect
	if ref.Bus == "" || !ref.Path.IsValid() || connection.Object(ref.Bus, ref.Path).CallWithContext(ctx, "org.a11y.atspi.Component.GetExtents", 0, uint32(0)).Store(&frame) != nil {
		return Rect{}, false
	}
	return Rect{X: float32(frame.X), Y: float32(frame.Y), Width: float32(frame.Width), Height: float32(frame.Height)}, frame.Width >= 2 && frame.Height >= 2
}

// linuxATSPICoordinateScale rejects mismatched toolkit geometry rather than guessing a display's DPI from the pointer.
func linuxATSPICoordinateScale(frame Rect, window linuxScreenshotWindow, scaleX, scaleY float32) (float32, float32, bool) {
	for _, scale := range []Point{{X: 1, Y: 1}, {X: scaleX, Y: scaleY}} {
		physical := Rect{X: frame.X * scale.X, Y: frame.Y * scale.Y, Width: frame.Width * scale.X, Height: frame.Height * scale.Y}
		for _, expected := range []Rect{window.frame, window.client} {
			if math.Abs(float64(physical.X-expected.X)) <= 2 && math.Abs(float64(physical.Y-expected.Y)) <= 2 &&
				math.Abs(float64(physical.Width-expected.Width)) <= 2 && math.Abs(float64(physical.Height-expected.Height)) <= 2 {
				return scale.X, scale.Y, true
			}
		}
	}
	return 0, 0, false
}
