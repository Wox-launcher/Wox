//go:build windows

package overlay

import "wox/util/windowhook"

// startNativeStickyTracking restores the injected target-thread move signal used by Windows overlays.
func (instance *runtimeOverlay) startNativeStickyTracking() bool {
	hook := windowhook.AttachSticky(instance.options.StickyWindowId, instance.options.StickyWindowPid, instance.window.WindowsHandle())
	if hook == nil {
		return false
	}
	instance.stickyDetach = func() { hook.Detach() }
	return true
}
