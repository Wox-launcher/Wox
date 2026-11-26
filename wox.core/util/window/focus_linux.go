package window

import (
	"context"
	"wox/util"
)

// focusCapturedWindow selects the foreground protocol of the current session.
func focusCapturedWindow(ctx context.Context, id string, pid int) error {
	switch linuxForegroundBackend(util.IsLinuxWaylandSession(), util.IsHyprlandSession(), util.IsKDEDesktopSession()) {
	case linuxForegroundKDE:
		return FocusKDEWindow(ctx, id, pid)
	case linuxForegroundHyprland:
		return focusCapturedHyprlandWindow(ctx, id, pid)
	case linuxForegroundX11:
		return focusCapturedX11Window(ctx, id, pid)
	default:
		return ErrWindowManagementUnsupported
	}
}
