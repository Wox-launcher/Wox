//go:build linux && cgo

package selection

import (
	"context"
	"fmt"
	"strings"

	"wox/util/clipboard"
	"wox/util/screen"
)

// getSelectedLinuxHyprland requests a fresh copy from the focused surface.
// PRIMARY can retain an unrelated application's selection indefinitely.
func getSelectedLinuxHyprland(ctx context.Context) (Selection, error) {
	if err := ctx.Err(); err != nil {
		return Selection{}, err
	}
	return selectionFromHyprlandCopy(clipboard.CaptureNextWaylandCopy, sendHyprlandSelectionCopy)
}

// selectionFromHyprlandCopy converts only the newly observed clipboard offer.
func selectionFromHyprlandCopy(capture func(func() error) (clipboard.Data, error), trigger func() error) (Selection, error) {
	data, err := capture(trigger)
	if err != nil {
		return Selection{}, err
	}
	switch selected := data.(type) {
	case *clipboard.TextData:
		return Selection{Type: SelectionTypeText, Text: selected.Text}, nil
	case *clipboard.FilePathData:
		return Selection{Type: SelectionTypeFile, FilePaths: selected.FilePaths}, nil
	default:
		return Selection{}, noSelection
	}
}

// sendHyprlandSelectionCopy uses explicit modifiers, like Omarchy's universal copy.
func sendHyprlandSelectionCopy() error {
	const script = `local w = hl.get_active_window()
local key = "C"
if w then
  for _, tag in ipairs(w.tags or {}) do
    if tag:gsub("%*$", "") == "terminal" then key = "Insert"; break end
  end
end
hl.dispatch(hl.dsp.send_key_state({ mods = "CTRL", key = key, state = "down" }))
hl.timer(function() hl.dispatch(hl.dsp.send_key_state({ mods = "CTRL", key = key, state = "up" })) end, { timeout = 50, type = "oneshot" })
return "ready"`
	response, err := screen.ReadHyprlandIPC("/repl "+script, 4096)
	if err != nil {
		return fmt.Errorf("send Hyprland copy shortcut: %w", err)
	}
	if strings.TrimSpace(string(response)) != "ready" {
		return fmt.Errorf("send Hyprland copy shortcut: %s", strings.TrimSpace(string(response)))
	}
	return nil
}
