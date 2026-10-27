//go:build linux

package screenshot

import (
	"context"
	"encoding/json"
	"image"
	"sort"

	"wox/util"
	utilscreen "wox/util/screen"
)

const linuxHyprlandClientsIPCLimit int64 = 4 << 20

// hyprlandScreenshotClient is the geometry Hyprland reports for one toplevel.
// at/size are global logical pixels, the same space as the portal capture bounds.
type hyprlandScreenshotClient struct {
	At        [2]float64 `json:"at"`
	Size      [2]float64 `json:"size"`
	Workspace struct {
		ID int `json:"id"`
	} `json:"workspace"`
	Mapped         bool `json:"mapped"`
	Hidden         bool `json:"hidden"`
	Floating       bool `json:"floating"`
	Pinned         bool `json:"pinned"`
	Fullscreen     int  `json:"fullscreen"`
	FocusHistoryID int  `json:"focusHistoryID"`
	PID            int  `json:"pid"`
}

type hyprlandScreenshotRank struct {
	window  linuxScreenshotWindow
	special bool
	full    bool
	front   bool
	focus   int
	area    float32
}

// captureLinuxHyprlandScreenshotWindows freezes visible toplevels before the selection overlay is shown.
func captureLinuxHyprlandScreenshotWindows() []linuxScreenshotWindow {
	monitors, err := utilscreen.GetHyprlandMonitors()
	if err != nil {
		util.GetLogger().Debug(context.Background(), "screenshot hyprland monitors: "+err.Error())
		return nil
	}
	payload, err := utilscreen.ReadHyprlandIPC("j/clients", linuxHyprlandClientsIPCLimit)
	if err != nil {
		util.GetLogger().Debug(context.Background(), "screenshot hyprland clients: "+err.Error())
		return nil
	}
	return parseLinuxHyprlandScreenshotWindows(payload, monitors)
}

// parseLinuxHyprlandScreenshotWindows keeps windows that are actually on screen, front-most first.
// Hyprland's client list has no stacking index. A shown special workspace sits above the
// normal workspace, fullscreen and floating sit above tiled windows, and a lower
// focusHistoryID was focused more recently. Smaller area breaks the remaining ties so a
// dialog is asked about before the window it covers.
func parseLinuxHyprlandScreenshotWindows(payload []byte, monitors []utilscreen.HyprlandMonitor) []linuxScreenshotWindow {
	var clients []hyprlandScreenshotClient
	if err := json.Unmarshal(payload, &clients); err != nil {
		return nil
	}
	shown := map[int]bool{}
	special := map[int]bool{}
	for _, monitor := range monitors {
		if monitor.ActiveWorkspace.ID != 0 {
			shown[monitor.ActiveWorkspace.ID] = true
		}
		if monitor.SpecialWorkspace.ID != 0 {
			shown[monitor.SpecialWorkspace.ID] = true
			special[monitor.SpecialWorkspace.ID] = true
		}
	}
	ranked := make([]hyprlandScreenshotRank, 0, len(clients))
	for _, client := range clients {
		if !client.Mapped || client.Hidden || client.Size[0] < 2 || client.Size[1] < 2 {
			continue
		}
		if !client.Pinned && !shown[client.Workspace.ID] {
			continue
		}
		frame := Rect{
			X:      float32(client.At[0]),
			Y:      float32(client.At[1]),
			Width:  float32(client.Size[0]),
			Height: float32(client.Size[1]),
		}
		ranked = append(ranked, hyprlandScreenshotRank{
			window:  linuxScreenshotWindow{pid: client.PID, frame: frame, client: frame},
			special: special[client.Workspace.ID],
			full:    client.Fullscreen != 0,
			front:   client.Floating || client.Pinned,
			focus:   client.FocusHistoryID,
			area:    frame.Width * frame.Height,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.special != b.special {
			return a.special
		}
		if a.full != b.full {
			return a.full
		}
		if a.front != b.front {
			return a.front
		}
		if a.focus != b.focus {
			return a.focus < b.focus
		}
		return a.area < b.area
	})
	windows := make([]linuxScreenshotWindow, 0, len(ranked))
	for _, rank := range ranked {
		windows = append(windows, rank.window)
	}
	return windows
}

// linuxHyprlandScreenshotObjectSelection maps compositor rectangles into editor coordinates.
// AT-SPI geometry is accepted only when it matches the frozen window; otherwise the window stands.
func linuxHyprlandScreenshotObjectSelection(windows []linuxScreenshotWindow, bounds Rect, source image.Image) ([]Rect, screenshotObjectQuery) {
	if source == nil || bounds.Width < 2 || bounds.Height < 2 || len(windows) == 0 {
		return nil, nil
	}
	candidates := make([]Rect, 0, len(windows))
	kept := make([]linuxScreenshotWindow, 0, len(windows))
	for _, window := range windows {
		rect, ok := linuxHyprlandEditorRect(window.frame, bounds)
		if !ok {
			continue
		}
		candidates = append(candidates, rect)
		kept = append(kept, window)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	return candidates, func(ctx context.Context, point Point) []Rect {
		global := Point{X: bounds.X + point.X, Y: bounds.Y + point.Y}
		for index, window := range kept {
			if !screenshotEditorRectContains(candidates[index], point) {
				continue
			}
			path := linuxATSPIObjectPath(ctx, window, global, 1, 1)
			for i := range path {
				path[i].X -= bounds.X
				path[i].Y -= bounds.Y
			}
			return path
		}
		return nil
	}
}

// linuxHyprlandEditorRect clips a global logical window to the capture and returns editor coordinates.
func linuxHyprlandEditorRect(frame, bounds Rect) (Rect, bool) {
	left := max(frame.X, bounds.X)
	top := max(frame.Y, bounds.Y)
	right := min(frame.X+frame.Width, bounds.X+bounds.Width)
	bottom := min(frame.Y+frame.Height, bounds.Y+bounds.Height)
	if right-left < 2 || bottom-top < 2 {
		return Rect{}, false
	}
	return Rect{X: left - bounds.X, Y: top - bounds.Y, Width: right - left, Height: bottom - top}, true
}
