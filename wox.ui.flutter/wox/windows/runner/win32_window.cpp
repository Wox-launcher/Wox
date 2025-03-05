#include "win32_window.h"

#include <dwmapi.h>
#include <flutter_windows.h>
#include <windowsx.h>

#include "resource.h"

namespace {

/// Window attribute that enables dark mode window decorations.
///
/// Redefined in case the developer's machine has a Windows SDK older than
/// version 10.0.22000.0.
/// See: https://docs.microsoft.com/windows/win32/api/dwmapi/ne-dwmapi-dwmwindowattribute
#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

// Round corners on Windows 11 (or newer) 
#ifndef DWMWA_WINDOW_CORNER_PREFERENCE
#define DWMWA_WINDOW_CORNER_PREFERENCE 33
#endif

// Windows 11 backdrop types
#ifndef DWMWA_SYSTEMBACKDROP_TYPE
#define DWMWA_SYSTEMBACKDROP_TYPE 38
#endif

#ifndef DWMSBT_NONE
// DWM System Backdrop types
typedef enum {
  DWMSBT_NONE = 0,          // None
  DWMSBT_MAINWINDOW = 1,    // Use the Backdrop material
  DWMSBT_TRANSIENTWINDOW = 2, // Use the Acrylic material
  DWMSBT_TABBEDWINDOW = 3   // Use the Mica material
} DWMSBT;
#endif

// DWM_WINDOW_CORNER_PREFERENCE enum values (for Windows 11)
#ifndef DWMWCP_DEFAULT
typedef enum {
  DWMWCP_DEFAULT = 0,
  DWMWCP_DONOTROUND = 1,
  DWMWCP_ROUND = 2,
  DWMWCP_ROUNDSMALL = 3
} MY_DWM_WINDOW_CORNER_PREFERENCE;
#endif

constexpr const wchar_t kWindowClassName[] = L"FLUTTER_RUNNER_WIN32_WINDOW";

// The number of Win32Window objects that currently exist.
static int g_active_window_count = 0;

using EnableNonClientDpiScaling = BOOL __stdcall(HWND hwnd);

// Scale helper to convert logical scaler values to physical using passed in
// scale factor
int Scale(int source, double scale_factor) {
  return static_cast<int>(source * scale_factor);
}

// Dynamically loads the |EnableNonClientDpiScaling| from the User32 module.
// This API is only needed for PerMonitor V1 awareness mode.
void EnableFullDpiSupportIfAvailable(HWND hwnd) {
  HMODULE user32_module = LoadLibraryA("User32.dll");
  if (!user32_module) {
    return;
  }
  auto enable_non_client_dpi_scaling =
      reinterpret_cast<EnableNonClientDpiScaling*>(
          GetProcAddress(user32_module, "EnableNonClientDpiScaling"));
  if (enable_non_client_dpi_scaling != nullptr) {
    enable_non_client_dpi_scaling(hwnd);
  }
  FreeLibrary(user32_module);
}

// Enable acrylic effect (Windows 11)
void EnableAcrylicEffect(HWND hwnd) {
  // Get Windows build number
  DWORD buildNumber = 0;
  HMODULE hNtdll = GetModuleHandleW(L"ntdll.dll");
  if (hNtdll) {
    typedef void (WINAPI *RtlGetNtVersionNumbersFunc)(DWORD*, DWORD*, DWORD*);
    RtlGetNtVersionNumbersFunc RtlGetNtVersionNumbers = (RtlGetNtVersionNumbersFunc)GetProcAddress(hNtdll, "RtlGetNtVersionNumbers");
    if (RtlGetNtVersionNumbers) {
      DWORD major, minor, buildNum;
      RtlGetNtVersionNumbers(&major, &minor, &buildNum);
      buildNumber = buildNum & 0x0FFFFFFF; // Remove the build revision
    }
  }

  // Windows 11 (build 22000 and higher) - Use official API
  if (buildNumber >= 22000) {
    // Use SystemBackdrop API with Acrylic effect (true blur with frosted glass look)
    int backdropType = DWMSBT_TRANSIENTWINDOW; // Acrylic (frosted glass effect)
    
    // 应用毛玻璃效果
    DwmSetWindowAttribute(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, &backdropType, sizeof(backdropType));
  }
  // 其他Windows版本不需要支持毛玻璃效果
}

}  // namespace

// Manages the Win32Window's window class registration.
class WindowClassRegistrar {
 public:
  ~WindowClassRegistrar() = default;

  // Returns the singleton registrar instance.
  static WindowClassRegistrar* GetInstance() {
    if (!instance_) {
      instance_ = new WindowClassRegistrar();
    }
    return instance_;
  }

  // Returns the name of the window class, registering the class if it hasn't
  // previously been registered.
  const wchar_t* GetWindowClass();

  // Unregisters the window class. Should only be called if there are no
  // instances of the window.
  void UnregisterWindowClass();

 private:
  WindowClassRegistrar() = default;

  static WindowClassRegistrar* instance_;

  bool class_registered_ = false;
};

WindowClassRegistrar* WindowClassRegistrar::instance_ = nullptr;

const wchar_t* WindowClassRegistrar::GetWindowClass() {
  if (!class_registered_) {
    WNDCLASS window_class{};
    window_class.hCursor = LoadCursor(nullptr, IDC_ARROW);
    window_class.lpszClassName = kWindowClassName;
    window_class.style = CS_HREDRAW | CS_VREDRAW;
    window_class.cbClsExtra = 0;
    window_class.cbWndExtra = 0;
    window_class.hInstance = GetModuleHandle(nullptr);
    window_class.hIcon =
        LoadIcon(window_class.hInstance, MAKEINTRESOURCE(IDI_APP_ICON));
    window_class.hbrBackground = 0;
    window_class.lpszMenuName = nullptr;
    window_class.lpfnWndProc = Win32Window::WndProc;
    RegisterClass(&window_class);
    class_registered_ = true;
  }
  return kWindowClassName;
}

void WindowClassRegistrar::UnregisterWindowClass() {
  UnregisterClass(kWindowClassName, nullptr);
  class_registered_ = false;
}

Win32Window::Win32Window() {
  ++g_active_window_count;
}

Win32Window::~Win32Window() {
  --g_active_window_count;
  Destroy();
}

bool Win32Window::Create(const std::wstring& title,
                         const Point& origin,
                         const Size& size) {
  Destroy();

  const wchar_t* window_class =
      WindowClassRegistrar::GetInstance()->GetWindowClass();

  const POINT target_point = {static_cast<LONG>(origin.x),
                              static_cast<LONG>(origin.y)};
  HMONITOR monitor = MonitorFromPoint(target_point, MONITOR_DEFAULTTONEAREST);
  UINT dpi = FlutterDesktopGetDpiForMonitor(monitor);
  double scale_factor = dpi / 96.0;

  // Update window style for better support of DWM effects
  // We need a proper window with caption area for DWM to apply effects correctly
  // but we use WS_POPUP for borderless look
  DWORD dwStyle = WS_POPUP | WS_THICKFRAME;
  DWORD dwExStyle = WS_EX_APPWINDOW;

  HWND window = CreateWindowEx(
      dwExStyle,
      window_class, title.c_str(), dwStyle,
      Scale(origin.x, scale_factor), Scale(origin.y, scale_factor),
      Scale(size.width, scale_factor), Scale(size.height, scale_factor),
      nullptr, nullptr, GetModuleHandle(nullptr), this);

  if (!window) {
    return false;
  }

  // Important: Do NOT set a NULL_BRUSH as it breaks the acrylic effect
  // Do NOT set window to layered with SetLayeredWindowAttributes as it
  // will break the DWM acrylic/blur effect

  // Set rounded corners (Windows 11 feature)
  DWORD buildNumber = 0;
  HMODULE hNtdll = GetModuleHandleW(L"ntdll.dll");
  if (hNtdll) {
    typedef void (WINAPI *RtlGetNtVersionNumbersFunc)(DWORD*, DWORD*, DWORD*);
    RtlGetNtVersionNumbersFunc RtlGetNtVersionNumbers = (RtlGetNtVersionNumbersFunc)GetProcAddress(hNtdll, "RtlGetNtVersionNumbers");
    if (RtlGetNtVersionNumbers) {
      DWORD major, minor, buildNum;
      RtlGetNtVersionNumbers(&major, &minor, &buildNum);
      buildNumber = buildNum & 0x0FFFFFFF; // Remove the build revision
    }
  }

  // Apply rounded corners for Windows 11 (build 22000 and higher)
  if (buildNumber >= 22000) {
    // DWMWCP_ROUND = 2
    UINT cornerPreference = 2;
    DwmSetWindowAttribute(window, DWMWA_WINDOW_CORNER_PREFERENCE, &cornerPreference, sizeof(cornerPreference));
  }

  // Apply acrylic effect for Windows 10 and 11
  EnableAcrylicEffect(window);

  // disable animations
  BOOL disableAnimations = TRUE;
  DwmSetWindowAttribute(window, DWMWA_TRANSITIONS_FORCEDISABLED, &disableAnimations, sizeof(disableAnimations));

  // force light theme regardless of system theme setting
  BOOL forceDarkTheme = FALSE;
  DwmSetWindowAttribute(window, DWMWA_USE_IMMERSIVE_DARK_MODE, &forceDarkTheme, sizeof(forceDarkTheme));

  return OnCreate();
}

bool Win32Window::Show() {
  if (window_handle_) {
    ShowWindow(window_handle_, SW_SHOWNOACTIVATE);
    UpdateWindow(window_handle_);
    return true;
  }
  return false;
}

// static
LRESULT CALLBACK Win32Window::WndProc(HWND const window,
                                      UINT const message,
                                      WPARAM const wparam,
                                      LPARAM const lparam) noexcept {
  if (message == WM_NCCREATE) {
    auto window_struct = reinterpret_cast<CREATESTRUCT*>(lparam);
    SetWindowLongPtr(window, GWLP_USERDATA,
                     reinterpret_cast<LONG_PTR>(window_struct->lpCreateParams));

    auto that = static_cast<Win32Window*>(window_struct->lpCreateParams);
    EnableFullDpiSupportIfAvailable(window);
    that->window_handle_ = window;
  } else if (Win32Window* that = GetThisFromHandle(window)) {
    return that->MessageHandler(window, message, wparam, lparam);
  }

  return DefWindowProc(window, message, wparam, lparam);
}

LRESULT
Win32Window::MessageHandler(HWND hwnd,
                            UINT const message,
                            WPARAM const wparam,
                            LPARAM const lparam) noexcept {
  switch (message) {
    case WM_DESTROY:
      window_handle_ = nullptr;
      Destroy();
      if (quit_on_close_) {
        PostQuitMessage(0);
      }
      return 0;

    case WM_DPICHANGED: {
      auto newRectSize = reinterpret_cast<RECT*>(lparam);
      LONG newWidth = newRectSize->right - newRectSize->left;
      LONG newHeight = newRectSize->bottom - newRectSize->top;

      SetWindowPos(hwnd, nullptr, newRectSize->left, newRectSize->top, newWidth,
                   newHeight, SWP_NOZORDER | SWP_NOACTIVATE);

      return 0;
    }
    
    // For acrylic effect, we need to leave WM_ERASEBKGND to default handler
    // Do NOT return 1 here as it breaks the DWM backdrop effect
    
    // Remove the WM_PAINT custom handler - let the system handle it for acrylic
      
    case WM_NCCALCSIZE:
      // Keep default non-client area calculation for DWM to work
      break;
      
    case WM_SIZE: {
      RECT rect;
      GetClientRect(hwnd, &rect);
      if (child_content_ != nullptr) {
        // Size and position the child window.
        MoveWindow(child_content_, rect.left, rect.top, rect.right - rect.left,
                   rect.bottom - rect.top, TRUE);
      }
      return 0;
    }

    case WM_ACTIVATE:
      if (child_content_ != nullptr) {
        SetFocus(child_content_);
      }
      return 0;
  }

  return DefWindowProc(window_handle_, message, wparam, lparam);
}

void Win32Window::Destroy() {
  OnDestroy();

  if (window_handle_) {
    DestroyWindow(window_handle_);
    window_handle_ = nullptr;
  }
  if (g_active_window_count == 0) {
    WindowClassRegistrar::GetInstance()->UnregisterWindowClass();
  }
}

Win32Window* Win32Window::GetThisFromHandle(HWND const window) noexcept {
  return reinterpret_cast<Win32Window*>(
      GetWindowLongPtr(window, GWLP_USERDATA));
}

void Win32Window::SetChildContent(HWND content) {
  child_content_ = content;
  SetParent(content, window_handle_);
  RECT frame = GetClientArea();

  MoveWindow(content, frame.left, frame.top, frame.right - frame.left,
             frame.bottom - frame.top, true);

  SetFocus(child_content_);
}

RECT Win32Window::GetClientArea() {
  RECT frame;
  GetClientRect(window_handle_, &frame);
  return frame;
}

HWND Win32Window::GetHandle() {
  return window_handle_;
}

void Win32Window::SetQuitOnClose(bool quit_on_close) {
  quit_on_close_ = quit_on_close;
}

bool Win32Window::OnCreate() {
  // No-op; provided for subclasses.
  return true;
}

void Win32Window::OnDestroy() {
  // No-op; provided for subclasses.
}
