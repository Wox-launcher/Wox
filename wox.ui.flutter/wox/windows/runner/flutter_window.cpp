#include "flutter_window.h"

#include <optional>
#include <thread>
#include <flutter/plugin_registrar_windows.h>
#include <windows.h>

#include "flutter/generated_plugin_registrant.h"

// Store window instance for window procedure
FlutterWindow *g_window_instance = nullptr;

// Global log function
void LogMessage(const std::string &message)
{
  if (g_window_instance)
  {
    g_window_instance->Log(message);
  }
}

FlutterWindow::FlutterWindow(const flutter::DartProject &project)
    : project_(project),
      original_window_proc_(nullptr),
      previous_active_window_(nullptr)
{
  g_window_instance = this;
}

FlutterWindow::~FlutterWindow()
{
  // Clear global instance
  if (g_window_instance == this)
  {
    g_window_instance = nullptr;
  }
}

void FlutterWindow::Log(const std::string &message)
{
  if (window_manager_channel_)
  {
    window_manager_channel_->InvokeMethod("log", std::make_unique<flutter::EncodableValue>(message));
  }
}

// Send keyboard event to Flutter (Windows-specific workaround)
void FlutterWindow::SendKeyboardEvent(UINT message, WPARAM wparam, LPARAM lparam)
{
  if (!window_manager_channel_)
  {
    return;
  }

  // Determine event type
  std::string eventType;
  if (message == WM_KEYDOWN || message == WM_SYSKEYDOWN)
  {
    eventType = "keydown";
  }
  else if (message == WM_KEYUP || message == WM_SYSKEYUP)
  {
    eventType = "keyup";
  }
  else
  {
    return; // Not a keyboard event we care about
  }

  // Get modifier key states using GetAsyncKeyState
  bool isShiftPressed = (GetAsyncKeyState(VK_SHIFT) & 0x8000) != 0;
  bool isControlPressed = (GetAsyncKeyState(VK_CONTROL) & 0x8000) != 0;
  bool isAltPressed = (GetAsyncKeyState(VK_MENU) & 0x8000) != 0;
  bool isMetaPressed = (GetAsyncKeyState(VK_LWIN) & 0x8000) != 0 || (GetAsyncKeyState(VK_RWIN) & 0x8000) != 0;

  // Build event data
  flutter::EncodableMap eventData;
  eventData[flutter::EncodableValue("type")] = flutter::EncodableValue(eventType);
  eventData[flutter::EncodableValue("keyCode")] = flutter::EncodableValue(static_cast<int>(wparam));
  eventData[flutter::EncodableValue("scanCode")] = flutter::EncodableValue(static_cast<int>((lparam >> 16) & 0xFF));
  eventData[flutter::EncodableValue("repeatCount")] = flutter::EncodableValue(static_cast<int>(lparam & 0xFFFF));
  eventData[flutter::EncodableValue("isExtended")] = flutter::EncodableValue(((lparam >> 24) & 1) == 1);

  // Add modifier key states
  eventData[flutter::EncodableValue("isShiftPressed")] = flutter::EncodableValue(isShiftPressed);
  eventData[flutter::EncodableValue("isControlPressed")] = flutter::EncodableValue(isControlPressed);
  eventData[flutter::EncodableValue("isAltPressed")] = flutter::EncodableValue(isAltPressed);
  eventData[flutter::EncodableValue("isMetaPressed")] = flutter::EncodableValue(isMetaPressed);

  // Send to Flutter
  window_manager_channel_->InvokeMethod("onKeyboardEvent", std::make_unique<flutter::EncodableValue>(eventData));
}

// Get the DPI scaling factor for the window
float FlutterWindow::GetDpiScale(HWND hwnd)
{
  // Default DPI is 96
  float dpiScale = 1.0f;

  // Try to use GetDpiForWindow which is available on Windows 10 1607 and later
  HMODULE user32 = GetModuleHandle(TEXT("user32.dll"));
  if (user32)
  {
    typedef UINT(WINAPI * GetDpiForWindowFunc)(HWND);
    GetDpiForWindowFunc getDpiForWindow =
        reinterpret_cast<GetDpiForWindowFunc>(GetProcAddress(user32, "GetDpiForWindow"));

    if (getDpiForWindow)
    {
      UINT dpi = getDpiForWindow(hwnd);
      dpiScale = dpi / 96.0f;
    }
    else
    {
      // Fallback for older Windows versions
      HDC hdc = GetDC(hwnd);
      if (hdc)
      {
        int dpiX = GetDeviceCaps(hdc, LOGPIXELSX);
        dpiScale = dpiX / 96.0f;
        ReleaseDC(hwnd, hdc);
      }
    }
  }

  return dpiScale;
}

bool FlutterWindow::OnCreate()
{
  if (!Win32Window::OnCreate())
  {
    return false;
  }

  RECT frame = GetClientArea();

  // The size here must match the window dimensions to avoid unnecessary surface
  // creation / destruction in the startup path.
  flutter_controller_ = std::make_unique<flutter::FlutterViewController>(frame.right - frame.left, frame.bottom - frame.top, project_);
  // Ensure that basic setup of the controller was successful.
  if (!flutter_controller_->engine() || !flutter_controller_->view())
  {
    return false;
  }
  RegisterPlugins(flutter_controller_->engine());

  // Set up window manager method channel
  window_manager_channel_ = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      flutter_controller_->engine()->messenger(), "com.wox.windows_window_manager",
      &flutter::StandardMethodCodec::GetInstance());

  window_manager_channel_->SetMethodCallHandler(
      [this](const auto &call, auto result)
      {
        HandleWindowManagerMethodCall(call, std::move(result));
      });

  // Replace the window procedure to capture window events
  HWND hwnd = GetHandle();
  if (hwnd != nullptr)
  {
    original_window_proc_ = reinterpret_cast<WNDPROC>(GetWindowLongPtr(hwnd, GWLP_WNDPROC));
    SetWindowLongPtr(hwnd, GWLP_WNDPROC, reinterpret_cast<LONG_PTR>(WindowProc));
  }

  SetChildContent(flutter_controller_->view()->GetNativeWindow());

  flutter_controller_->engine()->SetNextFrameCallback([&]()
                                                      {
                                                        // hidden-at-launch
                                                        // this->Show();
                                                      });

  // Flutter can complete the first frame before the "show window" callback is
  // registered. The following call ensures a frame is pending to ensure the
  // window is shown. It is a no-op if the first frame hasn't completed yet.
  flutter_controller_->ForceRedraw();

  return true;
}

void FlutterWindow::OnDestroy()
{
  // Restore original window procedure
  HWND hwnd = GetHandle();
  if (hwnd != nullptr && original_window_proc_ != nullptr)
  {
    SetWindowLongPtr(hwnd, GWLP_WNDPROC, reinterpret_cast<LONG_PTR>(original_window_proc_));
  }

  if (flutter_controller_)
  {
    flutter_controller_ = nullptr;
  }

  Win32Window::OnDestroy();
}

LRESULT
FlutterWindow::MessageHandler(HWND hwnd, UINT const message, WPARAM const wparam, LPARAM const lparam) noexcept
{
  // Log keyboard events BEFORE Flutter handles them
  switch (message)
  {
  case WM_KEYDOWN:
  case WM_SYSKEYDOWN:
  {
    char keyName[256];
    GetKeyNameTextA(static_cast<LONG>(lparam), keyName, sizeof(keyName));
    HWND foreground = GetForegroundWindow();
    bool isForeground = (foreground == hwnd);
    char hwndStr[32];
    sprintf_s(hwndStr, "%p", hwnd);
    char parentStr[32];
    HWND parent = GetParent(hwnd);
    sprintf_s(parentStr, "%p", parent);
    std::string logMsg = "[KEYLOG][NATIVE] WM_KEYDOWN: vk=" + std::to_string(wparam) +
                         " (" + keyName + ")" +
                         " repeat=" + std::to_string((lparam >> 30) & 1) +
                         " scancode=" + std::to_string((lparam >> 16) & 0xFF) +
                         " hwnd=" + hwndStr +
                         " parent=" + parentStr +
                         " isForeground=" + (isForeground ? "true" : "false");
    Log(logMsg);
  }
  break;

  case WM_KEYUP:
  case WM_SYSKEYUP:
  {
    char keyName[256];
    GetKeyNameTextA(static_cast<LONG>(lparam), keyName, sizeof(keyName));
    HWND foreground = GetForegroundWindow();
    bool isForeground = (foreground == hwnd);
    char hwndStr[32];
    sprintf_s(hwndStr, "%p", hwnd);
    char parentStr[32];
    HWND parent = GetParent(hwnd);
    sprintf_s(parentStr, "%p", parent);
    std::string logMsg = "[KEYLOG][NATIVE] WM_KEYUP: vk=" + std::to_string(wparam) +
                         " (" + keyName + ")" +
                         " scancode=" + std::to_string((lparam >> 16) & 0xFF) +
                         " hwnd=" + hwndStr +
                         " parent=" + parentStr +
                         " isForeground=" + (isForeground ? "true" : "false");
    Log(logMsg);
  }
  break;
  }

  // Give Flutter, including plugins, an opportunity to handle window messages.
  if (flutter_controller_)
  {
    std::optional<LRESULT> result = flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam, lparam);

    // Log Flutter's handling result for keyboard events
    if (message == WM_KEYDOWN || message == WM_SYSKEYDOWN)
    {
      if (result)
      {
        Log("[KEYLOG][NATIVE] Flutter consumed WM_KEYDOWN vk=" + std::to_string(wparam) + ", result=" + std::to_string(*result));
      }
      else
      {
        Log("[KEYLOG][NATIVE] Flutter did NOT consume WM_KEYDOWN vk=" + std::to_string(wparam));
      }
    }
    else if (message == WM_KEYUP || message == WM_SYSKEYUP)
    {
      if (result)
      {
        Log("[KEYLOG][NATIVE] Flutter consumed WM_KEYUP vk=" + std::to_string(wparam) + ", result=" + std::to_string(*result));
      }
      else
      {
        Log("[KEYLOG][NATIVE] Flutter did NOT consume WM_KEYUP vk=" + std::to_string(wparam));
      }
    }

    if (result)
    {
      return *result;
    }
  }

  switch (message)
  {
  case WM_FONTCHANGE:
    flutter_controller_->engine()->ReloadSystemFonts();
    break;
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}

void FlutterWindow::SendWindowEvent(const std::string &eventName)
{
  if (window_manager_channel_)
  {
    window_manager_channel_->InvokeMethod(eventName, std::make_unique<flutter::EncodableValue>(flutter::EncodableMap()));
  }
}

LRESULT CALLBACK FlutterWindow::WindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam)
{
  // If window instance is not available, use default window procedure
  if (g_window_instance == nullptr || g_window_instance->original_window_proc_ == nullptr)
  {
    return DefWindowProc(hwnd, message, wparam, lparam);
  }

  // Log keyboard events in WindowProc
  if (message == WM_KEYDOWN || message == WM_SYSKEYDOWN)
  {
    char keyName[256];
    GetKeyNameTextA(static_cast<LONG>(lparam), keyName, sizeof(keyName));
    char hwndStr[32];
    sprintf_s(hwndStr, "%p", hwnd);
    std::string logMsg = "[KEYLOG][WINDOWPROC] WM_KEYDOWN: vk=" + std::to_string(wparam) +
                         " (" + keyName + ") hwnd=" + hwndStr;
    g_window_instance->Log(logMsg);
  }
  else if (message == WM_KEYUP || message == WM_SYSKEYUP)
  {
    char keyName[256];
    GetKeyNameTextA(static_cast<LONG>(lparam), keyName, sizeof(keyName));
    char hwndStr[32];
    sprintf_s(hwndStr, "%p", hwnd);
    std::string logMsg = "[KEYLOG][WINDOWPROC] WM_KEYUP: vk=" + std::to_string(wparam) +
                         " (" + keyName + ") hwnd=" + hwndStr;
    g_window_instance->Log(logMsg);
  }

  // Handle window messages and send events to Flutter
  switch (message)
  {
  case WM_ACTIVATE:
    if (LOWORD(wparam) == WA_ACTIVE || LOWORD(wparam) == WA_CLICKACTIVE)
    {
      // g_window_instance->SendWindowEvent("onWindowFocus");
    }
    else
    {
      g_window_instance->SendWindowEvent("onWindowBlur");
    }
    break;
  }

  // Call the original window procedure
  return CallWindowProc(g_window_instance->original_window_proc_, hwnd, message, wparam, lparam);
}

void FlutterWindow::HandleWindowManagerMethodCall(
    const flutter::MethodCall<flutter::EncodableValue> &method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result)
{
  const std::string &method_name = method_call.method_name();
  HWND hwnd = GetHandle();

  if (hwnd == nullptr)
  {
    result->Error("WINDOW_ERROR", "Failed to get window handle");
    return;
  }

  try
  {
    if (method_name == "setSize")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments)
      {
        auto width_it = arguments->find(flutter::EncodableValue("width"));
        auto height_it = arguments->find(flutter::EncodableValue("height"));
        if (width_it != arguments->end() && height_it != arguments->end())
        {
          double width = std::get<double>(width_it->second);
          double height = std::get<double>(height_it->second);

          // Get DPI scale factor
          float dpiScale = GetDpiScale(hwnd);

          // Apply DPI scaling to get physical pixels
          int scaledWidth = static_cast<int>(width * dpiScale);
          int scaledHeight = static_cast<int>(height * dpiScale);

          RECT rect;
          GetWindowRect(hwnd, &rect);
          SetWindowPos(hwnd, nullptr, rect.left, rect.top, scaledWidth, scaledHeight, SWP_NOZORDER);
          result->Success();
        }
        else
        {
          result->Error("INVALID_ARGUMENTS", "Invalid arguments for setSize");
        }
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setSize");
      }
    }
    else if (method_name == "getPosition")
    {
      RECT rect;
      GetWindowRect(hwnd, &rect);

      // Get DPI scale factor
      float dpiScale = GetDpiScale(hwnd);

      // Apply DPI scaling to logical pixels (physical to logical)
      double scaledX = static_cast<double>(rect.left) / dpiScale;
      double scaledY = static_cast<double>(rect.top) / dpiScale;

      flutter::EncodableMap position;
      position[flutter::EncodableValue("x")] = flutter::EncodableValue(scaledX);
      position[flutter::EncodableValue("y")] = flutter::EncodableValue(scaledY);
      result->Success(flutter::EncodableValue(position));
    }
    else if (method_name == "setPosition")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (arguments)
      {
        auto x_it = arguments->find(flutter::EncodableValue("x"));
        auto y_it = arguments->find(flutter::EncodableValue("y"));
        if (x_it != arguments->end() && y_it != arguments->end())
        {
          double x = std::get<double>(x_it->second);
          double y = std::get<double>(y_it->second);

          // COORDINATE SYSTEM EXPLANATION:
          //
          // Backend (Go) provides LOGICAL coordinates (DPI-adjusted):
          //   - These are platform-independent coordinates
          //   - Example: (5680, 1234) on a 1920x1080 monitor at 100% DPI
          //
          // Frontend (Flutter/Windows) needs PHYSICAL coordinates (device pixels):
          //   - SetWindowPos API requires physical pixel coordinates
          //   - Example: Same position on 100% DPI = (5680, 1234) physical
          //   - Example: Same position on 225% DPI = (12780, 2776) physical
          //
          // The challenge in multi-monitor setups:
          //   - Different monitors can have different DPI settings
          //   - We must find which monitor contains the logical point
          //   - Then use THAT monitor's DPI to convert logical → physical
          //
          // Example scenario:
          //   Monitor 1: 5120x2880 @ 225% DPI → 2275x1280 logical, offset (0,0)
          //   Monitor 2: 1920x1080 @ 100% DPI → 1920x1080 logical, offset (5120,1080) physical
          //
          //   Logical point (5680, 1234) is on Monitor 2:
          //   - Physical bounds of Monitor 2: [5120,1080] to [7040,2160]
          //   - Logical bounds of Monitor 2: [5120,1080] to [7040,2160] (100% DPI, no scaling)
          //   - Convert: (5680, 1234) * 1.0 = (5680, 1234) physical ✓
          //
          //   If we mistakenly used Monitor 1's DPI (225%):
          //   - Convert: (5680, 1234) * 2.25 = (12780, 2776) physical ✗
          //   - This would place the window far outside Monitor 2!

          struct MonitorFindData
          {
            LONG targetX, targetY;
            HMONITOR foundMonitor;
            UINT foundDpi;
          } findData = {static_cast<LONG>(x), static_cast<LONG>(y), nullptr, 96};

          // Enumerate all monitors to find which one contains our logical point
          EnumDisplayMonitors(nullptr, nullptr, [](HMONITOR hMon, HDC, LPRECT, LPARAM lParam) -> BOOL
                              {
                                auto *data = reinterpret_cast<MonitorFindData *>(lParam);
                                MONITORINFO mi = {sizeof(mi)};
                                if (GetMonitorInfo(hMon, &mi))
                                {
                                  // GetMonitorInfo returns PHYSICAL coordinates
                                  // Example: Monitor 2 at physical [5120,1080] to [7040,2160]

                                  UINT dpi = FlutterDesktopGetDpiForMonitor(hMon);
                                  float scale = dpi / 96.0f;

                                  // Convert this monitor's physical bounds to logical coordinates
                                  // Example: Monitor 2 @ 100% DPI (scale=1.0)
                                  //   Physical [5120,1080,7040,2160] → Logical [5120,1080,7040,2160]
                                  // Example: Monitor 1 @ 225% DPI (scale=2.25)
                                  //   Physical [0,0,5120,2880] → Logical [0,0,2275,1280]
                                  LONG logLeft = static_cast<LONG>(mi.rcMonitor.left / scale);
                                  LONG logTop = static_cast<LONG>(mi.rcMonitor.top / scale);
                                  LONG logRight = static_cast<LONG>(mi.rcMonitor.right / scale);
                                  LONG logBottom = static_cast<LONG>(mi.rcMonitor.bottom / scale);

                                  // Check if our target logical point is within this monitor's logical bounds
                                  // Example: Point (5680,1234) is in Monitor 2's bounds [5120,1080,7040,2160] ✓
                                  if (data->targetX >= logLeft && data->targetX < logRight &&
                                      data->targetY >= logTop && data->targetY < logBottom)
                                  {
                                    data->foundMonitor = hMon;
                                    data->foundDpi = dpi;
                                    return FALSE; // Found the correct monitor, stop enumeration
                                  }
                                }
                                return TRUE; // Not this monitor, continue searching
                              },
                              reinterpret_cast<LPARAM>(&findData));

          // Fallback to primary monitor if logical point is not in any monitor
          // (This shouldn't happen in normal cases, but provides safety)
          if (findData.foundMonitor == nullptr)
          {
            findData.foundMonitor = MonitorFromPoint({0, 0}, MONITOR_DEFAULTTOPRIMARY);
            findData.foundDpi = FlutterDesktopGetDpiForMonitor(findData.foundMonitor);
          }

          // Now convert logical coordinates to physical using the correct monitor's DPI
          // Example: Monitor 2 @ 100% DPI (scale=1.0)
          //   Logical (5680, 1234) * 1.0 = Physical (5680, 1234)
          // Example: Monitor 1 @ 225% DPI (scale=2.25)
          //   Logical (738, 182) * 2.25 = Physical (1660, 409)
          float dpiScale = findData.foundDpi / 96.0f;
          int scaledX = static_cast<int>(x * dpiScale);
          int scaledY = static_cast<int>(y * dpiScale);

          RECT rect;
          GetWindowRect(hwnd, &rect);
          int width = rect.right - rect.left;
          int height = rect.bottom - rect.top;
          SetWindowPos(hwnd, nullptr, scaledX, scaledY, width, height, SWP_NOZORDER | SWP_NOSIZE);
          result->Success();
        }
        else
        {
          result->Error("INVALID_ARGUMENTS", "Invalid arguments for setPosition");
        }
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setPosition");
      }
    }
    else if (method_name == "center")
    {
      const auto *arguments = std::get_if<flutter::EncodableMap>(method_call.arguments());
      if (!arguments)
      {
        result->Error("INVALID_ARGUMENTS", "Arguments must be provided for center");
        return;
      }

      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));

      if (width_it == arguments->end() || height_it == arguments->end())
      {
        result->Error("INVALID_ARGUMENTS", "Both width and height must be provided for center");
        return;
      }

      double width = std::get<double>(width_it->second);
      double height = std::get<double>(height_it->second);

      // Get DPI scale factor
      float dpiScale = GetDpiScale(hwnd);

      // Apply DPI scaling to get physical pixels
      int scaledWidth = static_cast<int>(width * dpiScale);
      int scaledHeight = static_cast<int>(height * dpiScale);

      // Get system metrics for the primary monitor
      int screenWidth = GetSystemMetrics(SM_CXSCREEN);
      int screenHeight = GetSystemMetrics(SM_CYSCREEN);

      // Calculate center position
      int x = (screenWidth - scaledWidth) / 2;
      int y = (screenHeight - scaledHeight) / 2;

      Log("Center: window to " + std::to_string(x) + "," + std::to_string(y) + " with " + std::to_string(scaledWidth) + "," + std::to_string(scaledHeight));
      SetWindowPos(hwnd, nullptr, x, y, scaledWidth, scaledHeight, SWP_NOZORDER);
      result->Success();
    }
    else if (method_name == "show")
    {
      ShowWindow(hwnd, SW_SHOW);
      result->Success();
    }
    else if (method_name == "hide")
    {
      Log("[KEYLOG][NATIVE] Hide called, using ShowWindow(SW_HIDE)");

      // Use ShowWindow to properly hide the window
      // This should properly reset Flutter's internal state
      ShowWindow(hwnd, SW_HIDE);

      result->Success();
    }
    else if (method_name == "focus")
    {
      // 1. Use AttachThreadInput to try to set foreground window
      HWND fg = GetForegroundWindow();
      DWORD curTid = GetCurrentThreadId();
      DWORD fgTid = 0;
      if (fg)
      {
        GetWindowThreadProcessId(fg, &fgTid);
      }

      bool attached = false;
      if (fg && fgTid != curTid)
      {
        attached = AttachThreadInput(fgTid, curTid, TRUE);
      }

      SetForegroundWindow(hwnd);
      SetFocus(hwnd);
      BringWindowToTop(hwnd);

      if (attached)
      {
        AttachThreadInput(fgTid, curTid, FALSE);
      }

      if (GetForegroundWindow() == hwnd)
      {
        Log("Focus: use attach thread input");
        result->Success();
        return;
      }

      // 2. Fallback: legacy Alt key injection trick
      // alt has a side effect of showing the system start menu if user remapped keys using AutoHotkey or PowerToys (E.g. alt <-> win)
      // so we only use it in the last try
      INPUT pInputs[2];
      ZeroMemory(pInputs, sizeof(INPUT));

      pInputs[0].type = INPUT_KEYBOARD;
      pInputs[0].ki.wVk = VK_MENU; // Alt down
      pInputs[0].ki.dwFlags = 0;

      pInputs[1].type = INPUT_KEYBOARD;
      pInputs[1].ki.wVk = VK_MENU; // Alt up
      pInputs[1].ki.dwFlags = KEYEVENTF_KEYUP;

      SendInput(2, pInputs, sizeof(INPUT));
      SetForegroundWindow(hwnd);

      Log("Focus: use Alt key injection");
      result->Success();
    }
    else if (method_name == "isVisible")
    {
      // Use IsWindowVisible to check if window is actually visible
      bool is_visible = IsWindowVisible(hwnd) != 0;
      result->Success(flutter::EncodableValue(is_visible));
    }
    else if (method_name == "setAlwaysOnTop")
    {
      const auto *arguments = std::get_if<bool>(method_call.arguments());
      if (arguments)
      {
        bool always_on_top = *arguments;
        SetWindowPos(hwnd, always_on_top ? HWND_TOPMOST : HWND_NOTOPMOST, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE);
        result->Success();
      }
      else
      {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setAlwaysOnTop");
      }
    }
    else if (method_name == "startDragging")
    {
      ReleaseCapture();
      SendMessage(hwnd, WM_NCLBUTTONDOWN, HTCAPTION, 0);
      result->Success();
    }
    else if (method_name == "waitUntilReadyToShow")
    {
      result->Success();
    }
    else
    {
      result->NotImplemented();
    }
  }
  catch (const std::exception &e)
  {
    result->Error("EXCEPTION", std::string("Exception: ") + e.what());
  }
  catch (...)
  {
    result->Error("EXCEPTION", "Unknown exception occurred");
  }
}
