//go:build windows

package window

/*
#cgo LDFLAGS: -luser32
#include "window_thread_input_windows.h"
*/
import "C"

// ThreadInputDirection keeps attachment policy at the window activation call site.
type ThreadInputDirection int

const (
	CurrentThreadToQueues ThreadInputDirection = C.WOX_THREAD_INPUT_CURRENT_TO_QUEUES
	QueuesToCurrentThread ThreadInputDirection = C.WOX_THREAD_INPUT_QUEUES_TO_CURRENT
)

// ThreadInputAttachment owns temporary input-queue links, without window activation or focus policy.
type ThreadInputAttachment struct {
	native C.WoxWindowThreadInputAttachment
}

// AttachThreadInput joins the foreground queue and, when nonzero, the target window queue.
// The caller must keep the same OS thread through Close; a zero target skips target-thread attachment.
func AttachThreadInput(target uintptr, direction ThreadInputDirection) *ThreadInputAttachment {
	return &ThreadInputAttachment{native: C.wox_window_attach_thread_input(C.uintptr_t(target), C.WoxWindowThreadInputDirection(direction))}
}

// Close detaches successful links once on the same thread that created them.
func (attachment *ThreadInputAttachment) Close() {
	if attachment != nil {
		C.wox_window_detach_thread_input(&attachment.native)
	}
}
