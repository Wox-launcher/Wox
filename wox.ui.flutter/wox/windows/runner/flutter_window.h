#ifndef RUNNER_FLUTTER_WINDOW_H_
#define RUNNER_FLUTTER_WINDOW_H_

#include <flutter/dart_project.h>
#include <flutter/flutter_view_controller.h>
#include <flutter/method_channel.h>
#include <flutter/standard_method_codec.h>

#include <memory>

#include "win32_window.h"

// A window that does nothing but host a Flutter view.
class FlutterWindow : public Win32Window
{
public:
  // Creates a new FlutterWindow hosting a Flutter view running |project|.
  explicit FlutterWindow(const flutter::DartProject &project);
  virtual ~FlutterWindow();

  // Log message to console and Flutter
  void Log(const std::string &message);

  // Send keyboard event to Flutter (Windows-specific workaround)
  void SendKeyboardEvent(UINT message, WPARAM wparam, LPARAM lparam);

protected:
  // Win32Window:
  bool OnCreate() override;
  void OnDestroy() override;
  LRESULT MessageHandler(HWND window, UINT const message, WPARAM const wparam,
                         LPARAM const lparam) noexcept override;

private:
  // The project to run.
  flutter::DartProject project_;

  // The Flutter instance hosted by this window.
  std::unique_ptr<flutter::FlutterViewController> flutter_controller_;

  // Window manager method channel
  std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> window_manager_channel_;

  // Original window procedure
  WNDPROC original_window_proc_;

  // Previous active window handle
  HWND previous_active_window_;

  // Suppress transient WM_ACTIVATE/WA_INACTIVE blur events between show() and focus().
  // show() sets this to true; focus() and hide() clear it.
  bool suppress_blur_ = false;

  // Save/restore the previously focused window (Windows focus rules require explicit restore)
  void SavePreviousActiveWindow(HWND selfHwnd);
  void RestorePreviousActiveWindow(HWND selfHwnd);

  // Get the DPI scaling factor for the window
  float GetDpiScale(HWND hwnd);

  // Send window event to Flutter
  void SendWindowEvent(const std::string &eventName);

  // Handle method calls from Flutter
  void HandleWindowManagerMethodCall(
      const flutter::MethodCall<flutter::EncodableValue> &method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);

  // Dismiss the Windows Start Menu if it is currently open.
  // SetForegroundWindow requires no menus to be active.
  void DismissStartMenuIfOpen();

  // Static window procedure for handling window events
  static LRESULT CALLBACK WindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);
};

#endif // RUNNER_FLUTTER_WINDOW_H_
