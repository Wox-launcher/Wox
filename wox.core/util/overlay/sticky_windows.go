//go:build windows

package overlay

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"wox/util"
	"wox/util/windowhook"
)

// stickyAttachment is whatever managed to glue the overlay to its target. Both injected
// paths expose the same two operations, so the runtime never branches on which one won.
type stickyAttachment interface {
	PublishStickyOffset(overlayHWND uintptr)
	Detach() bool
}

// startNativeStickyTracking glues the overlay to its target window.
//
// Only injection is used. An in-process subclass moves the overlay inside the target's
// own move message, which is the one way to stay exactly in step with a drag. Windows
// refuses to inject across bitness, so a 32-bit target is hooked through a helper process
// built for it. Anything that merely observes the target from outside, including
// out-of-context WinEvents, trails the drag badly enough to feel worse than the polling
// fallback, so there is no middle tier: injection or polling.
func (instance *runtimeOverlay) startNativeStickyTracking() bool {
	pid := instance.options.StickyWindowPid
	windowID := instance.options.StickyWindowId
	overlayHWND := instance.window.WindowsHandle()

	target, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || target == 0 {
		util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky has no usable window id, using polling: pid=%d windowId=%q", pid, windowID))
		return false
	}

	attachment, route := instance.attachStickyInjection(uintptr(target), pid, overlayHWND)
	if attachment == nil {
		// Falling back here means the 100ms polling loop drives the overlay, which is
		// the visible difference between a glued overlay and a lagging one.
		util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky injection failed, using polling: pid=%d windowId=%q", pid, windowID))
		return false
	}

	util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky attached via %s: pid=%d windowId=%q", route, pid, windowID))
	instance.stickyPublish = func() { attachment.PublishStickyOffset(instance.window.WindowsHandle()) }
	instance.stickyDetach = func() { attachment.Detach() }
	return true
}

// attachStickyInjection picks the injection mechanism the target's bitness allows.
func (instance *runtimeOverlay) attachStickyInjection(target uintptr, pid int, overlayHWND uintptr) (stickyAttachment, string) {
	if windowhook.NeedsBitnessHelper(pid) {
		if hook := windowhook.AttachStickyViaHelper(target, pid, overlayHWND); hook != nil {
			return hook, "helper injection"
		}
		return nil, ""
	}
	if hook := windowhook.AttachSticky(strconv.FormatUint(uint64(target), 10), pid, overlayHWND); hook != nil {
		return hook, "in-process injection"
	}
	return nil, ""
}
