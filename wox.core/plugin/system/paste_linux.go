package system

import (
	"context"
	"wox/plugin"
	"wox/util"
	"wox/util/keyboard"
)

// simulatePasteAfterFocus chooses the input backend before confirming focus.
func simulatePasteAfterFocus(ctx context.Context, target plugin.QueryEnv, focus func() error) error {
	if util.IsKDEWayland() {
		return pasteToKDEWindow(ctx, target, focus)
	}
	if !util.IsLinuxWaylandSession() {
		return keyboard.SimulateX11PasteAfterFocus(focus)
	}
	return keyboard.SimulateWaylandPasteAfterFocus(focus)
}
