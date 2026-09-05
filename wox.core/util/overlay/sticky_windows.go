//go:build windows

package overlay

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	woxui "wox/ui/runtime"
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
// A native subclass moves the overlay inside the target's
// own move message, which is the one way to stay exactly in step with a drag. Windows
// refuses to inject across bitness, so a 32-bit target is hooked through a helper process
// built for it. Anything that merely observes the target from outside, including
// out-of-context WinEvents, trails the drag badly enough to feel worse than the polling
// fallback. Own-process windows install the same subclass directly on the UI thread.
func (instance *runtimeOverlay) startNativeStickyTracking() bool {
	pid := instance.options.StickyWindowPid
	windowID := instance.options.StickyWindowId
	overlayHWND := instance.window.WindowsHandle()

	target, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || target == 0 {
		util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky has no usable window id, using polling: pid=%d windowId=%q", pid, windowID))
		return false
	}
	if pid == os.Getpid() {
		// Layout runs on the UI thread, so our picker can reuse the native subclass
		// directly without injection, IPC waits, or the 100ms polling delay.
		attachment := windowhook.AttachSticky(windowID, pid, overlayHWND)
		if attachment == nil {
			return false
		}
		instance.stickyPublish = func() { attachment.PublishStickyOffset(instance.window.WindowsHandle()) }
		instance.stickyDetach = func() { attachment.Detach() }
		util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky attached via direct subclass: pid=%d windowId=%q", pid, windowID))
		return true
	}

	attachment, route := instance.attachStickyInjection(uintptr(target), pid, overlayHWND)
	if attachment == nil {
		// Falling back here means the 100ms polling loop drives the overlay, which is
		// the visible difference between a glued overlay and a lagging one.
		util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky injection failed, using polling: pid=%d windowId=%q", pid, windowID))
		stop := instance.stickyStop
		go func() {
			attempt := 0
			attachment := retryStickyAttachment(stop, func() stickyAttachment {
				attempt++
				started := time.Now()
				util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky retry begin: attempt=%d pid=%d windowId=%q overlay=0x%X", attempt, pid, windowID, overlayHWND))
				var retried stickyAttachment
				retried, route = instance.attachStickyInjection(uintptr(target), pid, overlayHWND)
				util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky retry end: attempt=%d pid=%d windowId=%q attached=%t elapsedMs=%d", attempt, pid, windowID, retried != nil, time.Since(started).Milliseconds()))
				return retried
			})
			if attachment == nil {
				reason := "exhausted"
				select {
				case <-stop:
					reason = "cancelled"
				default:
				}
				util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky retry stopped: reason=%s attempts=%d pid=%d windowId=%q", reason, attempt, pid, windowID))
				return
			}
			accepted := false
			_ = woxui.Call(func() {
				// A closed or retargeted overlay must not inherit a late attachment.
				if runtimeOverlayByID(instance.id) != instance || instance.stickyStop != stop {
					return
				}
				instance.stickyPublish = func() { attachment.PublishStickyOffset(overlayHWND) }
				instance.stickyDetach = func() { attachment.Detach() }
				instance.applyLayout(false)
				if instance.shown {
					close(stop)
					instance.stickyStop = nil
				}
				accepted = true
				util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky attached after retry via %s: pid=%d windowId=%q", route, pid, windowID))
			})
			if !accepted {
				attachment.Detach()
			}
		}()
		return false
	}

	util.GetLogger().Info(context.Background(), fmt.Sprintf("overlay sticky attached via %s: pid=%d windowId=%q", route, pid, windowID))
	instance.stickyPublish = func() { attachment.PublishStickyOffset(instance.window.WindowsHandle()) }
	instance.stickyDetach = func() { attachment.Detach() }
	return true
}

// retryStickyAttachment gives a newly opened dialog time to pump messages without
// blocking Wox's UI thread or retrying a permanently unavailable hook indefinitely.
func retryStickyAttachment(stop <-chan struct{}, attach func() stickyAttachment) stickyAttachment {
	for _, delay := range []time.Duration{150 * time.Millisecond, 300 * time.Millisecond, 600 * time.Millisecond} {
		timer := time.NewTimer(delay)
		select {
		case <-stop:
			timer.Stop()
			return nil
		case <-timer.C:
		}
		select {
		case <-stop:
			return nil
		default:
		}
		attachment := attach()
		select {
		case <-stop:
			if attachment != nil {
				attachment.Detach()
			}
			return nil
		default:
		}
		if attachment != nil {
			return attachment
		}
	}
	return nil
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
