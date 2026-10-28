//go:build linux

package screenshot

// prepareLinuxHyprlandRecordingToolbar releases the screenshot's exclusive seat
// ownership. Hyprland otherwise routes border and desktop clicks to the toolbar,
// even outside its bounds. On-demand focus keeps the toolbar keyboard-capable.
func prepareLinuxHyprlandRecordingToolbar(window *Window) error {
	return window.SetExclusiveKeyboard(false)
}
