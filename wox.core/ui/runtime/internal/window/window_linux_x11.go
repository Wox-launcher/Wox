//go:build linux

package window

// linuxX11WindowPolicy leaves activation, taskbar hints, and focus restoration to
// native_linux_x11.c; X11 windows must never negotiate Wayland layer-shell.
func linuxX11WindowPolicy() linuxWindowPolicy {
	return linuxWindowPolicy{name: "x11"}
}
