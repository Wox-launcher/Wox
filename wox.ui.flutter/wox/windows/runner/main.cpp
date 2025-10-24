#include <flutter/dart_project.h>
#include <windows.h>

#include "flutter_window.h"
#include "utils.h"

#include <protocol_handler_windows/protocol_handler_windows_plugin_c_api.h>

int APIENTRY wWinMain(_In_ HINSTANCE instance, _In_opt_ HINSTANCE prev,
                      _In_ wchar_t *command_line, _In_ int show_command)
{

  // Ensure we find an existing Wox UI window by its title
  HWND hwnd = ::FindWindow(L"FLUTTER_RUNNER_WIN32_WINDOW", L"wox-ui");
  if (hwnd != NULL)
  {
    DispatchToProtocolHandler(hwnd);

    ::ShowWindow(hwnd, SW_NORMAL);
    ::SetForegroundWindow(hwnd);
    return EXIT_FAILURE;
  }

  // Attach to console when present (e.g., 'flutter run') or create a
  // new console when running with a debugger.
  if (!::AttachConsole(ATTACH_PARENT_PROCESS) && ::IsDebuggerPresent())
  {
    CreateAndAttachConsole();
  }

  // Initialize COM, so that it is available for use in the library and/or
  // plugins.
  ::CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);

  flutter::DartProject project(L"data");

  std::vector<std::string> command_line_arguments =
      GetCommandLineArguments();

  project.set_dart_entrypoint_arguments(std::move(command_line_arguments));

  FlutterWindow window(project);
  Win32Window::Point origin(10, 10);
  Win32Window::Size size(1280, 720);
  if (!window.Create(L"wox-ui", origin, size))
  {
    return EXIT_FAILURE;
  }

  // Set window styles to ensure no title bar, no resize, no maximize/minimize buttons
  HWND window_handle = window.GetHandle();
  if (window_handle != NULL)
  {
    // Remove any system menu
    SetMenu(window_handle, NULL);

    // Make sure the window cannot be resized
    LONG style = GetWindowLong(window_handle, GWL_STYLE);
    style &= ~(WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_SYSMENU);
    SetWindowLong(window_handle, GWL_STYLE, style);

    // Update the window
    SetWindowPos(window_handle, NULL, 0, 0, 0, 0,
                 SWP_FRAMECHANGED | SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOOWNERZORDER);
  }

  window.SetQuitOnClose(true);

  ::MSG msg = {};
  while (::GetMessage(&msg, nullptr, 0, 0) > 0)
  {
    // Send keyboard events directly to Flutter (Windows-specific workaround)
    // This bypasses Flutter's broken child window keyboard event handling
    if (msg.message == WM_KEYDOWN || msg.message == WM_SYSKEYDOWN ||
        msg.message == WM_KEYUP || msg.message == WM_SYSKEYUP)
    {
      char keyName[256];
      GetKeyNameTextA(static_cast<LONG>(msg.lParam), keyName, sizeof(keyName));
      char hwndStr[32];
      sprintf_s(hwndStr, "%p", msg.hwnd);

      std::string eventType = (msg.message == WM_KEYDOWN || msg.message == WM_SYSKEYDOWN) ? "WM_KEYDOWN" : "WM_KEYUP";
      std::string logMsg = "[KEYLOG][MSGLOOP] " + eventType + ": vk=" + std::to_string(msg.wParam) +
                           " (" + keyName + ") hwnd=" + hwndStr;
      window.Log(logMsg);

      // Send to Flutter via our custom channel
      window.SendKeyboardEvent(msg.message, msg.wParam, msg.lParam);
    }

    // prevent the error/beep sound when alt+number/letter is pressed
    // Also prevent WM_CHAR generation for Alt combinations
    if (msg.message == WM_SYSKEYDOWN || msg.message == WM_SYSKEYUP)
    {
      // Don't call TranslateMessage for SYSKEYDOWN/UP to prevent WM_CHAR generation
      // This prevents Alt+J from typing 'j' into the text field
      ::DispatchMessage(&msg);
      window.Log("[KEYLOG][MSGLOOP] Dispatched SYSKEY without TranslateMessage");
      continue;
    }

    ::TranslateMessage(&msg);
    ::DispatchMessage(&msg);
  }

  ::CoUninitialize();
  return EXIT_SUCCESS;
}
