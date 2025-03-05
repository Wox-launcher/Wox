#include "windows_window_manager_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>
#include <windows.h>

#include <map>
#include <memory>
#include <string>

namespace {

class WindowsWindowManagerPlugin : public flutter::Plugin {
public:
  static void RegisterWithRegistrar(flutter::PluginRegistrarWindows *registrar);

  WindowsWindowManagerPlugin(flutter::PluginRegistrarWindows *registrar);

  virtual ~WindowsWindowManagerPlugin();

private:
  flutter::PluginRegistrarWindows *registrar_;
  std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> channel_;

  void HandleMethodCall(
      const flutter::MethodCall<flutter::EncodableValue> &method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);

  HWND GetWindow();
};

void WindowsWindowManagerPlugin::RegisterWithRegistrar(
    flutter::PluginRegistrarWindows *registrar) {
  auto plugin = std::make_unique<WindowsWindowManagerPlugin>(registrar);
  registrar->AddPlugin(std::move(plugin));
}

WindowsWindowManagerPlugin::WindowsWindowManagerPlugin(
    flutter::PluginRegistrarWindows *registrar)
    : registrar_(registrar) {
  channel_ = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      registrar_->messenger(), "com.wox.window_manager",
      &flutter::StandardMethodCodec::GetInstance());

  channel_->SetMethodCallHandler([this](const auto &call, auto result) {
    HandleMethodCall(call, std::move(result));
  });
}

WindowsWindowManagerPlugin::~WindowsWindowManagerPlugin() {}

HWND WindowsWindowManagerPlugin::GetWindow() {
  return ::GetAncestor(registrar_->GetView()->GetNativeWindow(), GA_ROOT);
}

void WindowsWindowManagerPlugin::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue> &method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  const std::string &method_name = method_call.method_name();
  HWND window = GetWindow();

  if (method_name == "ensureInitialized") {
    result->Success();
  } else if (method_name == "setSize") {
    const auto *arguments =
        std::get_if<flutter::EncodableMap>(method_call.arguments());
    if (arguments) {
      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));
      if (width_it != arguments->end() && height_it != arguments->end()) {
        double width = std::get<double>(width_it->second);
        double height = std::get<double>(height_it->second);

        RECT rect;
        GetWindowRect(window, &rect);
        SetWindowPos(window, nullptr, rect.left, rect.top,
                     static_cast<int>(width), static_cast<int>(height),
                     SWP_NOZORDER);
        result->Success();
      } else {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setSize");
      }
    } else {
      result->Error("INVALID_ARGUMENTS", "Invalid arguments for setSize");
    }
  } else if (method_name == "getPosition") {
    RECT rect;
    GetWindowRect(window, &rect);
    flutter::EncodableMap position;
    position[flutter::EncodableValue("x")] =
        flutter::EncodableValue(static_cast<double>(rect.left));
    position[flutter::EncodableValue("y")] =
        flutter::EncodableValue(static_cast<double>(rect.top));
    result->Success(flutter::EncodableValue(position));
  } else if (method_name == "setPosition") {
    const auto *arguments =
        std::get_if<flutter::EncodableMap>(method_call.arguments());
    if (arguments) {
      auto x_it = arguments->find(flutter::EncodableValue("x"));
      auto y_it = arguments->find(flutter::EncodableValue("y"));
      if (x_it != arguments->end() && y_it != arguments->end()) {
        double x = std::get<double>(x_it->second);
        double y = std::get<double>(y_it->second);

        RECT rect;
        GetWindowRect(window, &rect);
        int width = rect.right - rect.left;
        int height = rect.bottom - rect.top;
        SetWindowPos(window, nullptr, static_cast<int>(x), static_cast<int>(y),
                     width, height, SWP_NOZORDER);
        result->Success();
      } else {
        result->Error("INVALID_ARGUMENTS", "Invalid arguments for setPosition");
      }
    } else {
      result->Error("INVALID_ARGUMENTS", "Invalid arguments for setPosition");
    }
  } else if (method_name == "center") {
    RECT rect;
    GetWindowRect(window, &rect);
    int width = rect.right - rect.left;
    int height = rect.bottom - rect.top;

    int screenWidth = GetSystemMetrics(SM_CXSCREEN);
    int screenHeight = GetSystemMetrics(SM_CYSCREEN);

    int x = (screenWidth - width) / 2;
    int y = (screenHeight - height) / 2;

    SetWindowPos(window, nullptr, x, y, width, height, SWP_NOZORDER);
    result->Success();
  } else if (method_name == "show") {
    ShowWindow(window, SW_SHOW);
    SetForegroundWindow(window);
    result->Success();
  } else if (method_name == "hide") {
    ShowWindow(window, SW_HIDE);
    result->Success();
  } else if (method_name == "focus") {
    ShowWindow(window, SW_SHOW);
    SetForegroundWindow(window);
    result->Success();
  } else if (method_name == "isVisible") {
    bool is_visible = IsWindowVisible(window);
    result->Success(flutter::EncodableValue(is_visible));
  } else if (method_name == "setAlwaysOnTop") {
    const auto *arguments = std::get_if<bool>(method_call.arguments());
    if (arguments) {
      bool always_on_top = *arguments;
      HWND insert_after = always_on_top ? HWND_TOPMOST : HWND_NOTOPMOST;
      RECT rect;
      GetWindowRect(window, &rect);
      SetWindowPos(window, insert_after, rect.left, rect.top,
                   rect.right - rect.left, rect.bottom - rect.top,
                   SWP_NOMOVE | SWP_NOSIZE);
      result->Success();
    } else {
      result->Error("INVALID_ARGUMENTS",
                    "Invalid arguments for setAlwaysOnTop");
    }
  } else if (method_name == "waitUntilReadyToShow") {
    const auto *arguments =
        std::get_if<flutter::EncodableMap>(method_call.arguments());
    if (arguments) {
      auto width_it = arguments->find(flutter::EncodableValue("width"));
      auto height_it = arguments->find(flutter::EncodableValue("height"));
      auto center_it = arguments->find(flutter::EncodableValue("center"));
      auto always_on_top_it =
          arguments->find(flutter::EncodableValue("alwaysOnTop"));
      auto title_bar_style_it =
          arguments->find(flutter::EncodableValue("titleBarStyle"));
      auto window_button_visibility_it =
          arguments->find(flutter::EncodableValue("windowButtonVisibility"));

      if (width_it != arguments->end() && height_it != arguments->end()) {
        double width = std::get<double>(width_it->second);
        double height = std::get<double>(height_it->second);
        RECT rect;
        GetWindowRect(window, &rect);
        SetWindowPos(window, nullptr, rect.left, rect.top,
                     static_cast<int>(width), static_cast<int>(height),
                     SWP_NOZORDER);
      }

      if (center_it != arguments->end() && std::get<bool>(center_it->second)) {
        RECT rect;
        GetWindowRect(window, &rect);
        int width = rect.right - rect.left;
        int height = rect.bottom - rect.top;

        int screenWidth = GetSystemMetrics(SM_CXSCREEN);
        int screenHeight = GetSystemMetrics(SM_CYSCREEN);

        int x = (screenWidth - width) / 2;
        int y = (screenHeight - height) / 2;

        SetWindowPos(window, nullptr, x, y, width, height, SWP_NOZORDER);
      }

      if (always_on_top_it != arguments->end() &&
          std::get<bool>(always_on_top_it->second)) {
        RECT rect;
        GetWindowRect(window, &rect);
        SetWindowPos(window, HWND_TOPMOST, rect.left, rect.top,
                     rect.right - rect.left, rect.bottom - rect.top,
                     SWP_NOMOVE | SWP_NOSIZE);
      }

      result->Success();
    } else {
      result->Error("INVALID_ARGUMENTS",
                    "Invalid arguments for waitUntilReadyToShow");
    }
  } else {
    result->NotImplemented();
  }
}

} // namespace

void WindowsWindowManagerPluginRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar) {
  WindowsWindowManagerPlugin::RegisterWithRegistrar(
      flutter::PluginRegistrarManager::GetInstance()
          ->GetRegistrar<flutter::PluginRegistrarWindows>(registrar));
}