//go:build linux

package clipboard

// hyprlandClipboard uses the generic Wayland clipboard path. Hyprland does not
// provide a usable portal RemoteDesktop clipboard session, so reads go through
// ext-data-control-v1 and writes use wl-copy.
type hyprlandClipboard struct {
	waylandClipboard
}

func newHyprlandClipboard() hyprlandClipboard {
	return hyprlandClipboard{}
}

func (hyprlandClipboard) name() string {
	return "hyprland"
}
