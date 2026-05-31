#ifndef RUNNER_RESULT_DRAG_BRIDGE_H_
#define RUNNER_RESULT_DRAG_BRIDGE_H_

#include <flutter/binary_messenger.h>
#include <windows.h>

void RegisterResultDragBridge(flutter::BinaryMessenger *messenger, HWND owner_window);

#endif  // RUNNER_RESULT_DRAG_BRIDGE_H_
