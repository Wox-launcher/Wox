#ifndef WOX_WINDOW_THREAD_INPUT_WINDOWS_H
#define WOX_WINDOW_THREAD_INPUT_WINDOWS_H

#include <stdint.h>

typedef enum {
  WOX_THREAD_INPUT_CURRENT_TO_QUEUES,
  WOX_THREAD_INPUT_QUEUES_TO_CURRENT
} WoxWindowThreadInputDirection;

// Only successful input-queue attachments are detached; all work stays on the calling OS thread.
typedef struct {
  uint32_t current_thread;
  uint32_t foreground_thread;
  uint32_t target_thread;
  WoxWindowThreadInputDirection direction;
  int32_t foreground_attached;
  int32_t target_attached;
} WoxWindowThreadInputAttachment;

WoxWindowThreadInputAttachment wox_window_attach_thread_input(uintptr_t target, WoxWindowThreadInputDirection direction);
void wox_window_detach_thread_input(WoxWindowThreadInputAttachment *attachment);

#endif
