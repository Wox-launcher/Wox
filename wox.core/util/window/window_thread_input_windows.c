//go:build windows

#include "window_thread_input_windows.h"
#include <windows.h>

// Keep the caller's ordered thread pair identical for attachment and cleanup.
static BOOL set_thread_input_link(uint32_t current, uint32_t other, WoxWindowThreadInputDirection direction, BOOL attach) {
  if (direction == WOX_THREAD_INPUT_QUEUES_TO_CURRENT) {
    return AttachThreadInput(other, current, attach);
  }
  return AttachThreadInput(current, other, attach);
}

// A zero target joins only the foreground queue; the caller chooses the attachment direction.
WoxWindowThreadInputAttachment wox_window_attach_thread_input(uintptr_t target, WoxWindowThreadInputDirection direction) {
  HWND foreground = GetForegroundWindow();
  WoxWindowThreadInputAttachment attachment = {GetCurrentThreadId(), 0, 0, direction, FALSE, FALSE};
  attachment.foreground_thread = foreground ? GetWindowThreadProcessId(foreground, NULL) : 0;
  attachment.target_thread = target ? GetWindowThreadProcessId((HWND)target, NULL) : 0;
  if (attachment.foreground_thread != 0 && attachment.foreground_thread != attachment.current_thread) {
    attachment.foreground_attached = set_thread_input_link(attachment.current_thread, attachment.foreground_thread, direction, TRUE);
  }
  if (attachment.target_thread != 0 && attachment.target_thread != attachment.current_thread &&
      attachment.target_thread != attachment.foreground_thread) {
    attachment.target_attached = set_thread_input_link(attachment.current_thread, attachment.target_thread, direction, TRUE);
  }
  return attachment;
}

// Reverse the successful attachments once, including partial failures and repeated cleanup.
void wox_window_detach_thread_input(WoxWindowThreadInputAttachment *attachment) {
  if (attachment == NULL) {
    return;
  }
  if (attachment->target_attached) {
    set_thread_input_link(attachment->current_thread, attachment->target_thread, attachment->direction, FALSE);
    attachment->target_attached = FALSE;
  }
  if (attachment->foreground_attached) {
    set_thread_input_link(attachment->current_thread, attachment->foreground_thread, attachment->direction, FALSE);
    attachment->foreground_attached = FALSE;
  }
}
