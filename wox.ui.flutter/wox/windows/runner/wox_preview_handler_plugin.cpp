#include "wox_preview_handler_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>
#include <propsys.h>
#include <shlwapi.h>
#include <shobjidl.h>
#include <windows.h>
#include <wrl/client.h>

#include <algorithm>
#include <cmath>
#include <format>
#include <memory>
#include <optional>
#include <string>
#include <unordered_map>
#include <utility>

namespace {

using Microsoft::WRL::ComPtr;

constexpr auto kChannelName = "com.wox.preview_handler";
constexpr auto kMethodCreate = "create";
constexpr auto kMethodSetBounds = "setBounds";
constexpr auto kMethodDispose = "dispose";
constexpr auto kMethodOnEscapePressed = "onEscapePressed";
constexpr auto kErrorInvalidArguments = "invalid_arguments";
constexpr auto kErrorInvalidId = "invalid_id";
constexpr auto kErrorCreateFailed = "create_failed";
constexpr auto kPreviewHandlerShellExtension = L"{8895b1c6-b41f-4c1c-a562-0d564250836f}";
constexpr auto kPreviewHandlerWindowClassName = L"WoxPreviewHandlerHostWindow";

// Converts Dart's UTF-8 file paths into the UTF-16 paths required by shell COM
// initialization interfaces.
std::wstring Utf16FromUtf8(const std::string& utf8_string) {
  if (utf8_string.empty()) {
    return std::wstring();
  }

  const int target_length = ::MultiByteToWideChar(
      CP_UTF8, MB_ERR_INVALID_CHARS, utf8_string.data(),
      static_cast<int>(utf8_string.size()), nullptr, 0);
  if (target_length <= 0) {
    return std::wstring();
  }

  std::wstring utf16_string(target_length, L'\0');
  const int converted_length = ::MultiByteToWideChar(
      CP_UTF8, MB_ERR_INVALID_CHARS, utf8_string.data(),
      static_cast<int>(utf8_string.size()), utf16_string.data(),
      target_length);
  if (converted_length != target_length) {
    return std::wstring();
  }

  return utf16_string;
}

// Keeps HRESULT details visible to Flutter without depending on localized
// system error strings from the native side.
std::string HResultMessage(const char* action, HRESULT hr) {
  return std::format("{} failed with HRESULT 0x{:08X}", action,
                     static_cast<unsigned int>(hr));
}

// Reads a numeric method-channel argument and rounds Flutter double values into
// native pixel bounds.
int GetIntArgument(const flutter::EncodableMap& map, const char* key,
                   int fallback = 0) {
  const auto it = map.find(flutter::EncodableValue(key));
  if (it == map.end()) {
    return fallback;
  }

  if (const auto value = std::get_if<int>(&it->second)) {
    return *value;
  }
  if (const auto value = std::get_if<int64_t>(&it->second)) {
    return static_cast<int>(*value);
  }
  if (const auto value = std::get_if<double>(&it->second)) {
    return static_cast<int>(std::round(*value));
  }
  return fallback;
}

// Reads a UTF-8 method-channel argument from the StandardMethodCodec map.
std::optional<std::string> GetStringArgument(const flutter::EncodableMap& map,
                                             const char* key) {
  const auto it = map.find(flutter::EncodableValue(key));
  if (it == map.end()) {
    return std::nullopt;
  }

  if (const auto value = std::get_if<std::string>(&it->second)) {
    return *value;
  }
  return std::nullopt;
}

// Reads the native instance id sent back from Dart.
std::optional<int64_t> GetInt64Argument(const flutter::EncodableMap& map,
                                        const char* key) {
  const auto it = map.find(flutter::EncodableValue(key));
  if (it == map.end()) {
    return std::nullopt;
  }

  if (const auto value = std::get_if<int64_t>(&it->second)) {
    return *value;
  }
  if (const auto value = std::get_if<int>(&it->second)) {
    return static_cast<int64_t>(*value);
  }
  return std::nullopt;
}

// Extracts the extension because AssocQueryString resolves preview handlers
// from file-type associations rather than arbitrary file paths.
std::wstring GetFileExtension(const std::wstring& file_path) {
  const auto slash = file_path.find_last_of(L"\\/");
  const auto dot = file_path.find_last_of(L'.');
  if (dot == std::wstring::npos ||
      (slash != std::wstring::npos && dot < slash)) {
    return std::wstring();
  }
  return file_path.substr(dot);
}

// Finds the shell preview handler CLSID registered for the file extension, using
// the same association mechanism as Windows Explorer's preview pane.
HRESULT ResolvePreviewHandlerClsid(const std::wstring& file_path,
                                   CLSID* clsid) {
  if (clsid == nullptr) {
    return E_POINTER;
  }

  const auto extension = GetFileExtension(file_path);
  if (extension.empty()) {
    return HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND);
  }

  DWORD char_count = 0;
  auto hr = ::AssocQueryStringW(
      ASSOCF_INIT_DEFAULTTOSTAR, ASSOCSTR_SHELLEXTENSION, extension.c_str(),
      kPreviewHandlerShellExtension, nullptr, &char_count);
  if (hr != S_FALSE &&
      hr != HRESULT_FROM_WIN32(ERROR_INSUFFICIENT_BUFFER)) {
    return hr;
  }

  std::wstring clsid_text(char_count, L'\0');
  hr = ::AssocQueryStringW(
      ASSOCF_INIT_DEFAULTTOSTAR, ASSOCSTR_SHELLEXTENSION, extension.c_str(),
      kPreviewHandlerShellExtension, clsid_text.data(), &char_count);
  if (FAILED(hr)) {
    return hr;
  }

  const auto null_pos = clsid_text.find(L'\0');
  if (null_pos != std::wstring::npos) {
    clsid_text.resize(null_pos);
  }
  if (clsid_text.empty()) {
    return HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND);
  }

  return ::CLSIDFromString(clsid_text.c_str(), clsid);
}

// Registers the lightweight HWND that Office/WPS preview handlers render into.
bool RegisterPreviewHandlerWindowClass() {
  static bool registered = false;
  if (registered) {
    return true;
  }

  WNDCLASSW window_class = {};
  window_class.lpszClassName = kPreviewHandlerWindowClassName;
  window_class.lpfnWndProc = &DefWindowProcW;
  window_class.hInstance = ::GetModuleHandleW(nullptr);
  window_class.hbrBackground = reinterpret_cast<HBRUSH>(COLOR_WINDOW + 1);

  if (::RegisterClassW(&window_class) == 0 &&
      ::GetLastError() != ERROR_CLASS_ALREADY_EXISTS) {
    return false;
  }

  registered = true;
  return true;
}

// Owns one shell preview handler and the child HWND it renders into.
class PreviewHandlerInstance {
 public:
  PreviewHandlerInstance(HWND parent, const std::wstring& file_path)
      : parent_(parent), file_path_(file_path) {}

  ~PreviewHandlerInstance() { Dispose(); }

  // Creates and starts the COM preview handler for the target file.
  bool Create(const RECT& bounds, std::string* error) {
    if (!RegisterPreviewHandlerWindowClass()) {
      if (error != nullptr) {
        *error = "Failed to register preview handler host window";
      }
      return false;
    }

    const int width = std::max(1, static_cast<int>(bounds.right - bounds.left));
    const int height = std::max(1, static_cast<int>(bounds.bottom - bounds.top));
    // Keep the host hidden until the slow Office handler is fully attached so
    // it cannot flash at stale startup bounds and then move into place.
    host_window_ = ::CreateWindowExW(
        0, kPreviewHandlerWindowClassName, L"",
        WS_CHILD | WS_CLIPCHILDREN | WS_CLIPSIBLINGS,
        bounds.left, bounds.top, width, height, parent_, nullptr,
        ::GetModuleHandleW(nullptr), nullptr);
    if (host_window_ == nullptr) {
      if (error != nullptr) {
        *error = "Failed to create preview handler host window";
      }
      return false;
    }

    CLSID clsid{};
    auto hr = ResolvePreviewHandlerClsid(file_path_, &clsid);
    if (FAILED(hr)) {
      if (error != nullptr) {
        *error = HResultMessage("Resolving preview handler", hr);
      }
      return false;
    }

    ComPtr<IUnknown> unknown;
    hr = ::CoCreateInstance(clsid, nullptr,
                            CLSCTX_INPROC_SERVER | CLSCTX_LOCAL_SERVER,
                            IID_PPV_ARGS(&unknown));
    if (FAILED(hr) || !unknown) {
      if (error != nullptr) {
        *error = HResultMessage("Creating preview handler", hr);
      }
      return false;
    }

    hr = InitializeHandler(unknown.Get());
    if (FAILED(hr)) {
      if (error != nullptr) {
        *error = HResultMessage("Initializing preview handler", hr);
      }
      return false;
    }

    hr = unknown.As(&preview_handler_);
    if (FAILED(hr) || !preview_handler_) {
      if (error != nullptr) {
        *error = HResultMessage("Querying IPreviewHandler", hr);
      }
      return false;
    }

    RECT preview_rect{0, 0, width, height};
    hr = preview_handler_->SetWindow(host_window_, &preview_rect);
    if (FAILED(hr)) {
      if (error != nullptr) {
        *error = HResultMessage("Attaching preview handler", hr);
      }
      return false;
    }

    hr = preview_handler_->DoPreview();
    if (FAILED(hr)) {
      if (error != nullptr) {
        *error = HResultMessage("Starting preview handler", hr);
      }
      return false;
    }

    ::ShowWindow(host_window_, SW_SHOWNOACTIVATE);
    ::UpdateWindow(host_window_);
    return true;
  }

  // Moves the native preview HWND whenever the Flutter preview area changes.
  void SetBounds(const RECT& bounds) {
    if (host_window_ == nullptr) {
      return;
    }

    const int width = std::max(1, static_cast<int>(bounds.right - bounds.left));
    const int height = std::max(1, static_cast<int>(bounds.bottom - bounds.top));
    ::MoveWindow(host_window_, bounds.left, bounds.top, width, height, TRUE);
    if (preview_handler_) {
      RECT preview_rect{0, 0, width, height};
      preview_handler_->SetRect(&preview_rect);
    }
  }

  bool ContainsWindow(HWND window) const {
    return host_window_ != nullptr &&
           (window == host_window_ || ::IsChild(host_window_, window));
  }

  // Unloads the COM handler before the child HWND is destroyed.
  void Dispose() {
    if (preview_handler_) {
      preview_handler_->Unload();
      preview_handler_.Reset();
    }
    if (host_window_ != nullptr) {
      ::DestroyWindow(host_window_);
      host_window_ = nullptr;
    }
  }

 private:
  // Most handlers support IInitializeWithFile; IInitializeWithStream keeps the
  // bridge compatible with handlers that only accept streams.
  HRESULT InitializeHandler(IUnknown* unknown) {
    ComPtr<IInitializeWithFile> initialize_with_file;
    auto hr = unknown->QueryInterface(IID_PPV_ARGS(&initialize_with_file));
    if (SUCCEEDED(hr) && initialize_with_file) {
      return initialize_with_file->Initialize(file_path_.c_str(), STGM_READ);
    }

    ComPtr<IInitializeWithStream> initialize_with_stream;
    hr = unknown->QueryInterface(IID_PPV_ARGS(&initialize_with_stream));
    if (SUCCEEDED(hr) && initialize_with_stream) {
      ComPtr<IStream> stream;
      hr = ::SHCreateStreamOnFileEx(file_path_.c_str(), STGM_READ,
                                    FILE_ATTRIBUTE_NORMAL, FALSE, nullptr,
                                    &stream);
      if (FAILED(hr)) {
        return hr;
      }
      return initialize_with_stream->Initialize(stream.Get(), STGM_READ);
    }

    return E_NOINTERFACE;
  }

  HWND parent_ = nullptr;
  HWND host_window_ = nullptr;
  std::wstring file_path_;
  ComPtr<IPreviewHandler> preview_handler_;
};

// Bridges Dart method calls to native preview-handler instances.
class WoxPreviewHandlerPlugin : public flutter::Plugin {
 public:
  // Registers the method channel against the current Flutter view HWND.
  static void RegisterWithRegistrar(flutter::PluginRegistrarWindows* registrar) {
    auto channel =
        std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
            registrar->messenger(), kChannelName,
            &flutter::StandardMethodCodec::GetInstance());

    HWND parent_window = nullptr;
    if (registrar->GetView() != nullptr) {
      parent_window = registrar->GetView()->GetNativeWindow();
    }

    auto plugin = std::make_unique<WoxPreviewHandlerPlugin>(parent_window,
                                                            std::move(channel));
    plugin->channel_->SetMethodCallHandler(
        [plugin_pointer = plugin.get()](const auto& call, auto result) {
          plugin_pointer->HandleMethodCall(call, std::move(result));
        });

    registrar->AddPlugin(std::move(plugin));
  }

  explicit WoxPreviewHandlerPlugin(
      HWND parent_window,
      std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> channel)
      : parent_window_(parent_window), channel_(std::move(channel)) {
    active_plugin_ = this;
  }

  ~WoxPreviewHandlerPlugin() override {
    instances_.clear();
    UninstallKeyboardHookIfIdle();
    if (active_plugin_ == this) {
      active_plugin_ = nullptr;
    }
  }

 private:
  static WoxPreviewHandlerPlugin* active_plugin_;

  HWND parent_window_ = nullptr;
  HHOOK keyboard_hook_ = nullptr;
  bool escape_press_pending_ = false;
  int64_t next_id_ = 1;
  std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> channel_;
  std::unordered_map<int64_t, std::unique_ptr<PreviewHandlerInstance>>
      instances_;

  // Dispatches small lifecycle commands from the Dart wrapper.
  void HandleMethodCall(
      const flutter::MethodCall<flutter::EncodableValue>& method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
    if (method_call.method_name() == kMethodCreate) {
      return Create(method_call, std::move(result));
    }
    if (method_call.method_name() == kMethodSetBounds) {
      return SetBounds(method_call, std::move(result));
    }
    if (method_call.method_name() == kMethodDispose) {
      return Dispose(method_call, std::move(result));
    }

    result->NotImplemented();
  }

  void Create(
      const flutter::MethodCall<flutter::EncodableValue>& method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
    if (parent_window_ == nullptr) {
      return result->Error(kErrorCreateFailed,
                           "Flutter view window is unavailable");
    }
    const auto* map = std::get_if<flutter::EncodableMap>(
        method_call.arguments());
    if (map == nullptr) {
      return result->Error(kErrorInvalidArguments);
    }

    const auto file_path = GetStringArgument(*map, "filePath");
    if (!file_path || file_path->empty()) {
      return result->Error(kErrorInvalidArguments, "filePath is required");
    }

    const auto utf16_file_path = Utf16FromUtf8(*file_path);
    if (utf16_file_path.empty()) {
      return result->Error(kErrorInvalidArguments, "filePath is invalid");
    }

    RECT bounds = BoundsFromArguments(*map);
    auto instance =
        std::make_unique<PreviewHandlerInstance>(parent_window_,
                                                 utf16_file_path);
    std::string error;
    if (!instance->Create(bounds, &error)) {
      return result->Error(kErrorCreateFailed, error);
    }

    const auto id = next_id_++;
    instances_[id] = std::move(instance);
    InstallKeyboardHookIfNeeded();
    result->Success(flutter::EncodableValue(flutter::EncodableMap{
        {flutter::EncodableValue("id"), flutter::EncodableValue(id)},
    }));
  }

  void SetBounds(
      const flutter::MethodCall<flutter::EncodableValue>& method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
    const auto* map = std::get_if<flutter::EncodableMap>(
        method_call.arguments());
    if (map == nullptr) {
      return result->Error(kErrorInvalidArguments);
    }

    const auto id = GetInt64Argument(*map, "id");
    if (!id) {
      return result->Error(kErrorInvalidArguments, "id is required");
    }

    const auto it = instances_.find(*id);
    if (it == instances_.end()) {
      return result->Error(kErrorInvalidId);
    }

    it->second->SetBounds(BoundsFromArguments(*map));
    result->Success();
  }

  void Dispose(
      const flutter::MethodCall<flutter::EncodableValue>& method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
    const auto* map = std::get_if<flutter::EncodableMap>(
        method_call.arguments());
    if (map == nullptr) {
      return result->Error(kErrorInvalidArguments);
    }

    const auto id = GetInt64Argument(*map, "id");
    if (!id) {
      return result->Error(kErrorInvalidArguments, "id is required");
    }

    const auto it = instances_.find(*id);
    if (it != instances_.end()) {
      instances_.erase(it);
    }
    UninstallKeyboardHookIfIdle();
    result->Success();
  }

  // Converts Flutter physical pixel bounds into the Win32 RECT used by the host.
  RECT BoundsFromArguments(const flutter::EncodableMap& map) const {
    const int x = GetIntArgument(map, "x");
    const int y = GetIntArgument(map, "y");
    const int width = std::max(1, GetIntArgument(map, "width", 1));
    const int height = std::max(1, GetIntArgument(map, "height", 1));
    return RECT{x, y, x + width, y + height};
  }

  // Office preview handlers can own keyboard focus through native child HWNDs,
  // so Flutter's focus tree may never receive Escape. Observe Escape only while
  // focus is inside one of our preview hosts and forward that intent to Dart.
  void InstallKeyboardHookIfNeeded() {
    if (keyboard_hook_ != nullptr) {
      return;
    }

    keyboard_hook_ = ::SetWindowsHookExW(WH_KEYBOARD_LL, LowLevelKeyboardProc,
                                         ::GetModuleHandleW(nullptr), 0);
  }

  void UninstallKeyboardHookIfIdle() {
    if (!instances_.empty()) {
      return;
    }

    escape_press_pending_ = false;
    if (keyboard_hook_ != nullptr) {
      ::UnhookWindowsHookEx(keyboard_hook_);
      keyboard_hook_ = nullptr;
    }
  }

  static LRESULT CALLBACK LowLevelKeyboardProc(int code, WPARAM wparam,
                                               LPARAM lparam) {
    if (code == HC_ACTION && active_plugin_ != nullptr) {
      active_plugin_->HandleLowLevelKeyboard(
          wparam, reinterpret_cast<KBDLLHOOKSTRUCT*>(lparam));
    }

    return ::CallNextHookEx(nullptr, code, wparam, lparam);
  }

  void HandleLowLevelKeyboard(WPARAM wparam,
                              const KBDLLHOOKSTRUCT* keyboard) {
    if (keyboard == nullptr || keyboard->vkCode != VK_ESCAPE) {
      return;
    }

    const bool is_key_up = wparam == WM_KEYUP || wparam == WM_SYSKEYUP ||
                           (keyboard->flags & LLKHF_UP) != 0;
    if (is_key_up) {
      escape_press_pending_ = false;
      return;
    }

    const bool is_key_down = wparam == WM_KEYDOWN || wparam == WM_SYSKEYDOWN;
    if (!is_key_down || escape_press_pending_ ||
        !IsFocusInsideAnyPreviewHost()) {
      return;
    }

    escape_press_pending_ = true;
    NotifyEscapePressed();
  }

  bool IsFocusInsideAnyPreviewHost() const {
    HWND focus_window = CurrentFocusWindow();
    if (focus_window == nullptr) {
      return false;
    }

    for (const auto& entry : instances_) {
      if (entry.second->ContainsWindow(focus_window)) {
        return true;
      }
    }
    return false;
  }

  HWND CurrentFocusWindow() const {
    GUITHREADINFO gui_thread_info{};
    gui_thread_info.cbSize = sizeof(gui_thread_info);
    if (::GetGUIThreadInfo(0, &gui_thread_info) &&
        gui_thread_info.hwndFocus != nullptr) {
      return gui_thread_info.hwndFocus;
    }

    return ::GetFocus();
  }

  void NotifyEscapePressed() {
    if (channel_ == nullptr) {
      return;
    }

    channel_->InvokeMethod(
        kMethodOnEscapePressed,
        std::make_unique<flutter::EncodableValue>(flutter::EncodableMap{}));
  }
};

WoxPreviewHandlerPlugin* WoxPreviewHandlerPlugin::active_plugin_ = nullptr;

}  // namespace

void RegisterWoxPreviewHandlerPlugin(FlutterDesktopPluginRegistrarRef registrar) {
  WoxPreviewHandlerPlugin::RegisterWithRegistrar(
      flutter::PluginRegistrarManager::GetInstance()
          ->GetRegistrar<flutter::PluginRegistrarWindows>(registrar));
}
