#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <shlwapi.h>
#include <shobjidl.h>
#include <wrl/client.h>

#include <algorithm>
#include <string>

#include "native_windows.h"

using Microsoft::WRL::ComPtr;

struct WoxWindowsFilePreview {
  HWND parent = nullptr;
  HWND host = nullptr;
  std::wstring path;
  ComPtr<IPreviewHandler> handler;

  ~WoxWindowsFilePreview() {
    if (handler) {
      handler->Unload();
      handler.Reset();
    }
    if (host != nullptr) {
      DestroyWindow(host);
      host = nullptr;
    }
  }
};

namespace {

constexpr wchar_t kPreviewHandlerShellExtension[] = L"{8895b1c6-b41f-4c1c-a562-0d564250836f}";
constexpr wchar_t kPreviewHandlerWindowClassName[] = L"WoxGoFilePreviewHostWindow";

std::wstring utf16_from_utf8(const char *value) {
  if (value == nullptr || *value == '\0') {
    return {};
  }
  const int length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value, -1, nullptr, 0);
  if (length <= 1) {
    return {};
  }
  std::wstring result(static_cast<size_t>(length), L'\0');
  if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value, -1, result.data(), length) == 0) {
    return {};
  }
  result.pop_back();
  return result;
}

std::wstring file_extension(const std::wstring &path) {
  const size_t slash = path.find_last_of(L"\\/");
  const size_t dot = path.find_last_of(L'.');
  if (dot == std::wstring::npos || (slash != std::wstring::npos && dot < slash)) {
    return {};
  }
  return path.substr(dot);
}

HRESULT resolve_preview_handler(const std::wstring &path, CLSID *clsid) {
  if (clsid == nullptr) {
    return E_POINTER;
  }
  const std::wstring extension = file_extension(path);
  if (extension.empty()) {
    return HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND);
  }

  DWORD length = 0;
  HRESULT result = AssocQueryStringW(ASSOCF_INIT_DEFAULTTOSTAR, ASSOCSTR_SHELLEXTENSION, extension.c_str(), kPreviewHandlerShellExtension, nullptr, &length);
  if (result != S_FALSE && result != HRESULT_FROM_WIN32(ERROR_INSUFFICIENT_BUFFER)) {
    return result;
  }
  if (length == 0) {
    return HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND);
  }

  std::wstring value(length, L'\0');
  result = AssocQueryStringW(ASSOCF_INIT_DEFAULTTOSTAR, ASSOCSTR_SHELLEXTENSION, extension.c_str(), kPreviewHandlerShellExtension, value.data(), &length);
  if (FAILED(result)) {
    return result;
  }
  const size_t null_position = value.find(L'\0');
  if (null_position != std::wstring::npos) {
    value.resize(null_position);
  }
  if (value.empty()) {
    return HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND);
  }
  return CLSIDFromString(value.c_str(), clsid);
}

bool register_preview_handler_window_class() {
  static bool registered = false;
  if (registered) {
    return true;
  }
  WNDCLASSW window_class = {};
  window_class.lpszClassName = kPreviewHandlerWindowClassName;
  window_class.lpfnWndProc = &DefWindowProcW;
  window_class.hInstance = GetModuleHandleW(nullptr);
  window_class.hbrBackground = reinterpret_cast<HBRUSH>(COLOR_WINDOW + 1);
  if (RegisterClassW(&window_class) == 0 && GetLastError() != ERROR_CLASS_ALREADY_EXISTS) {
    return false;
  }
  registered = true;
  return true;
}

RECT preview_rect(int32_t x, int32_t y, int32_t width, int32_t height) {
  const LONG safe_width = std::max<LONG>(1, width);
  const LONG safe_height = std::max<LONG>(1, height);
  return RECT{x, y, x + safe_width, y + safe_height};
}

HRESULT initialize_handler(IUnknown *unknown, const std::wstring &path) {
  ComPtr<IInitializeWithFile> with_file;
  HRESULT result = unknown->QueryInterface(IID_PPV_ARGS(&with_file));
  if (SUCCEEDED(result) && with_file) {
    return with_file->Initialize(path.c_str(), STGM_READ);
  }

  ComPtr<IInitializeWithStream> with_stream;
  result = unknown->QueryInterface(IID_PPV_ARGS(&with_stream));
  if (SUCCEEDED(result) && with_stream) {
    ComPtr<IStream> stream;
    result = SHCreateStreamOnFileEx(path.c_str(), STGM_READ, FILE_ATTRIBUTE_NORMAL, FALSE, nullptr, &stream);
    if (FAILED(result)) {
      return result;
    }
    return with_stream->Initialize(stream.Get(), STGM_READ);
  }
  return E_NOINTERFACE;
}

HRESULT show_preview(WoxWindowsFilePreview *preview, const RECT &bounds) {
  if (preview == nullptr || preview->host == nullptr || !preview->handler) {
    return E_INVALIDARG;
  }
  const int width = std::max<LONG>(1, bounds.right - bounds.left);
  const int height = std::max<LONG>(1, bounds.bottom - bounds.top);
  MoveWindow(preview->host, bounds.left, bounds.top, width, height, TRUE);
  RECT child_rect{0, 0, width, height};
  HRESULT result = preview->handler->SetRect(&child_rect);
  if (FAILED(result)) {
    return result;
  }
  ShowWindow(preview->host, SW_SHOWNOACTIVATE);
  UpdateWindow(preview->host);
  return S_OK;
}

}  // namespace

extern "C" int32_t wox_windows_file_preview_create(uintptr_t owner, const char *path, int32_t x, int32_t y, int32_t width, int32_t height, WoxWindowsFilePreview **preview) {
  if (owner == 0 || path == nullptr || preview == nullptr || width <= 0 || height <= 0) {
    return static_cast<int32_t>(E_INVALIDARG);
  }
  *preview = nullptr;
  if (!register_preview_handler_window_class()) {
    return static_cast<int32_t>(HRESULT_FROM_WIN32(GetLastError()));
  }

  auto file_path = utf16_from_utf8(path);
  if (file_path.empty()) {
    return static_cast<int32_t>(E_INVALIDARG);
  }

  auto instance = new WoxWindowsFilePreview();
  instance->parent = reinterpret_cast<HWND>(owner);
  instance->path = std::move(file_path);
  const RECT bounds = preview_rect(x, y, width, height);
  instance->host = CreateWindowExW(0, kPreviewHandlerWindowClassName, L"", WS_CHILD | WS_CLIPCHILDREN | WS_CLIPSIBLINGS, bounds.left, bounds.top, bounds.right - bounds.left, bounds.bottom - bounds.top, instance->parent, nullptr, GetModuleHandleW(nullptr), nullptr);
  if (instance->host == nullptr) {
    delete instance;
    return static_cast<int32_t>(HRESULT_FROM_WIN32(GetLastError()));
  }

  CLSID clsid{};
  HRESULT result = resolve_preview_handler(instance->path, &clsid);
  if (SUCCEEDED(result)) {
    ComPtr<IUnknown> unknown;
    result = CoCreateInstance(clsid, nullptr, CLSCTX_INPROC_SERVER | CLSCTX_LOCAL_SERVER, IID_PPV_ARGS(&unknown));
    if (SUCCEEDED(result) && unknown) {
      result = initialize_handler(unknown.Get(), instance->path);
      if (SUCCEEDED(result)) {
        result = unknown.As(&instance->handler);
      }
    }
  }
  if (SUCCEEDED(result) && instance->handler) {
    RECT child_rect{0, 0, bounds.right - bounds.left, bounds.bottom - bounds.top};
    result = instance->handler->SetWindow(instance->host, &child_rect);
  }
  if (SUCCEEDED(result) && instance->handler) {
    RECT child_rect{0, 0, bounds.right - bounds.left, bounds.bottom - bounds.top};
    result = instance->handler->SetRect(&child_rect);
  }
  if (SUCCEEDED(result) && instance->handler) {
    result = instance->handler->DoPreview();
  }
  if (FAILED(result)) {
    delete instance;
    return static_cast<int32_t>(result);
  }
  ShowWindow(instance->host, SW_SHOWNOACTIVATE);
  UpdateWindow(instance->host);
  *preview = instance;
  return static_cast<int32_t>(S_OK);
}

extern "C" int32_t wox_windows_file_preview_show(WoxWindowsFilePreview *preview, int32_t x, int32_t y, int32_t width, int32_t height) {
  return static_cast<int32_t>(show_preview(preview, preview_rect(x, y, width, height)));
}

extern "C" int32_t wox_windows_file_preview_hide(WoxWindowsFilePreview *preview) {
  if (preview == nullptr || preview->host == nullptr) {
    return static_cast<int32_t>(E_INVALIDARG);
  }
  ShowWindow(preview->host, SW_HIDE);
  return static_cast<int32_t>(S_OK);
}

extern "C" void wox_windows_file_preview_destroy(WoxWindowsFilePreview *preview) {
  delete preview;
}
