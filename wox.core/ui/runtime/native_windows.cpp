//go:build windows

#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <shobjidl.h>
#include <shlobj.h>
#include <shellapi.h>

#include <cstdlib>
#include <cstring>
#include <algorithm>
#include <atomic>
#include <cstdarg>
#include <cstdio>
#include <memory>
#include <string>
#include <unordered_map>
#include <utility>
#include <vector>

#include "native_windows.h"
#include "renderer_windows.h"

extern "C" int32_t woxGoWindowsWebViewEscape(uintptr_t owner);
extern "C" void woxGoWindowsWebViewEscapeDiagnostic(uintptr_t owner, const char *detail);
extern "C" int32_t woxGoWindowsWebViewActionPanel(uintptr_t owner);
extern "C" void woxGoWindowsWebViewNavigationChanged(uintptr_t owner, const char *url, int32_t can_go_back, int32_t can_go_forward);

static void webview_debug(const char *format, ...) {
  static const bool enabled = [] {
    char value[8] = {};
    return GetEnvironmentVariableA("WOX_UI_WEBVIEW_DEBUG", value, sizeof(value)) > 0 && value[0] == '1';
  }();
  if (!enabled) {
    return;
  }
  std::fprintf(stderr, "[wox-webview] ");
  va_list arguments;
  va_start(arguments, format);
  std::vfprintf(stderr, format, arguments);
  va_end(arguments);
  std::fprintf(stderr, "\n");
  std::fflush(stderr);
}

static std::string wide_to_utf8(const wchar_t *value) {
  if (value == nullptr || *value == L'\0') {
    return {};
  }
  const int length = WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, value, -1, nullptr, 0, nullptr, nullptr);
  if (length <= 1) {
    return {};
  }
  std::string result(static_cast<size_t>(length), '\0');
  if (WideCharToMultiByte(CP_UTF8, WC_ERR_INVALID_CHARS, value, -1, result.data(), length, nullptr, nullptr) == 0) {
    return {};
  }
  result.pop_back();
  return result;
}

extern "C" int32_t wox_windows_pick_file(uintptr_t owner, int32_t directory, char **path) {
  if (owner == 0 || path == nullptr) {
    return E_INVALIDARG;
  }
  *path = nullptr;

  IFileOpenDialog *dialog = nullptr;
  HRESULT result = CoCreateInstance(CLSID_FileOpenDialog, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&dialog));
  if (FAILED(result)) {
    return result;
  }

  FILEOPENDIALOGOPTIONS options = 0;
  result = dialog->GetOptions(&options);
  if (SUCCEEDED(result)) {
    options |= FOS_FORCEFILESYSTEM | FOS_NOCHANGEDIR;
    if (directory != 0) {
      options |= FOS_PICKFOLDERS;
    }
    result = dialog->SetOptions(options);
  }
  if (SUCCEEDED(result)) {
    result = dialog->Show(reinterpret_cast<HWND>(owner));
  }
  if (result == HRESULT_FROM_WIN32(ERROR_CANCELLED)) {
    dialog->Release();
    return 1;
  }

  IShellItem *item = nullptr;
  if (SUCCEEDED(result)) {
    result = dialog->GetResult(&item);
  }
  PWSTR native_path = nullptr;
  if (SUCCEEDED(result)) {
    result = item->GetDisplayName(SIGDN_FILESYSPATH, &native_path);
  }
  std::string utf8_path;
  if (SUCCEEDED(result)) {
    utf8_path = wide_to_utf8(native_path);
    if (utf8_path.empty()) {
      result = E_FAIL;
    }
  }
  if (native_path != nullptr) {
    CoTaskMemFree(native_path);
  }
  if (item != nullptr) {
    item->Release();
  }
  dialog->Release();
  if (FAILED(result)) {
    return result;
  }

  *path = static_cast<char *>(std::malloc(utf8_path.size() + 1));
  if (*path == nullptr) {
    return E_OUTOFMEMORY;
  }
  std::memcpy(*path, utf8_path.c_str(), utf8_path.size() + 1);
  return 0;
}

extern "C" void wox_windows_free_string(char *value) {
  std::free(value);
}

static std::wstring utf8_to_wide(const char *value);

namespace {

static HGLOBAL create_file_drop_global(const std::vector<std::wstring> &files) {
  size_t path_char_count = 1;
  for (const auto &file : files) {
    path_char_count += file.size() + 1;
  }

  const SIZE_T data_size = sizeof(DROPFILES) + path_char_count * sizeof(wchar_t);
  HGLOBAL memory = ::GlobalAlloc(GMEM_MOVEABLE | GMEM_ZEROINIT, data_size);
  if (memory == nullptr) {
    return nullptr;
  }

  auto *drop_files = static_cast<DROPFILES *>(::GlobalLock(memory));
  if (drop_files == nullptr) {
    ::GlobalFree(memory);
    return nullptr;
  }

  drop_files->pFiles = sizeof(DROPFILES);
  drop_files->fWide = TRUE;
  auto *cursor = reinterpret_cast<wchar_t *>(reinterpret_cast<BYTE *>(drop_files) + sizeof(DROPFILES));
  for (const auto &file : files) {
    std::copy(file.begin(), file.end(), cursor);
    cursor += file.size();
    *cursor++ = L'\0';
  }
  *cursor = L'\0';
  ::GlobalUnlock(memory);
  return memory;
}

static HGLOBAL duplicate_global_memory(HGLOBAL source) {
  const SIZE_T size = ::GlobalSize(source);
  if (size == 0) {
    return nullptr;
  }

  HGLOBAL target = ::GlobalAlloc(GMEM_MOVEABLE, size);
  if (target == nullptr) {
    return nullptr;
  }

  void *source_ptr = ::GlobalLock(source);
  void *target_ptr = ::GlobalLock(target);
  if (source_ptr == nullptr || target_ptr == nullptr) {
    if (source_ptr != nullptr) {
      ::GlobalUnlock(source);
    }
    if (target_ptr != nullptr) {
      ::GlobalUnlock(target);
    }
    ::GlobalFree(target);
    return nullptr;
  }

  std::memcpy(target_ptr, source_ptr, size);
  ::GlobalUnlock(source);
  ::GlobalUnlock(target);
  return target;
}

static bool is_file_format(const FORMATETC *format) {
  return format != nullptr && format->cfFormat == CF_HDROP && (format->tymed & TYMED_HGLOBAL) != 0 && format->dwAspect == DVASPECT_CONTENT;
}

class WoxFormatEtcEnumerator final : public IEnumFORMATETC {
 public:
  explicit WoxFormatEtcEnumerator(const FORMATETC &format) : format_(format) {}

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
    if (object == nullptr) {
      return E_POINTER;
    }
    if (riid == IID_IUnknown || riid == IID_IEnumFORMATETC) {
      *object = static_cast<IEnumFORMATETC *>(this);
      AddRef();
      return S_OK;
    }
    *object = nullptr;
    return E_NOINTERFACE;
  }

  ULONG STDMETHODCALLTYPE AddRef() override { return ++references_; }

  ULONG STDMETHODCALLTYPE Release() override {
    ULONG count = --references_;
    if (count == 0) {
      delete this;
    }
    return count;
  }

  HRESULT STDMETHODCALLTYPE Next(ULONG count, FORMATETC *formats, ULONG *fetched) override {
    if (formats == nullptr) {
      return E_POINTER;
    }
    if (fetched != nullptr) {
      *fetched = 0;
    }
    if (index_ > 0 || count == 0) {
      return S_FALSE;
    }

    formats[0] = format_;
    index_ = 1;
    if (fetched != nullptr) {
      *fetched = 1;
    }
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Skip(ULONG count) override {
    if (count == 0 || index_ > 0) {
      return S_FALSE;
    }
    index_ = 1;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Reset() override {
    index_ = 0;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Clone(IEnumFORMATETC **result) override {
    if (result == nullptr) {
      return E_POINTER;
    }
    auto *clone = new WoxFormatEtcEnumerator(format_);
    clone->index_ = index_;
    *result = clone;
    return S_OK;
  }

 private:
  std::atomic<ULONG> references_{1};
  FORMATETC format_{};
  ULONG index_ = 0;
};

class WoxFileDataObject final : public IDataObject {
 public:
  explicit WoxFileDataObject(HGLOBAL hdrop) : hdrop_(hdrop) {
    format_.cfFormat = CF_HDROP;
    format_.dwAspect = DVASPECT_CONTENT;
    format_.lindex = -1;
    format_.tymed = TYMED_HGLOBAL;
  }

  ~WoxFileDataObject() {
    if (hdrop_ != nullptr) {
      ::GlobalFree(hdrop_);
    }
  }

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
    if (object == nullptr) {
      return E_POINTER;
    }
    if (riid == IID_IUnknown || riid == IID_IDataObject) {
      *object = static_cast<IDataObject *>(this);
      AddRef();
      return S_OK;
    }
    *object = nullptr;
    return E_NOINTERFACE;
  }

  ULONG STDMETHODCALLTYPE AddRef() override { return ++references_; }

  ULONG STDMETHODCALLTYPE Release() override {
    ULONG count = --references_;
    if (count == 0) {
      delete this;
    }
    return count;
  }

  HRESULT STDMETHODCALLTYPE GetData(FORMATETC *format, STGMEDIUM *medium) override {
    if (!is_file_format(format) || medium == nullptr) {
      return DV_E_FORMATETC;
    }
    HGLOBAL copy = duplicate_global_memory(hdrop_);
    if (copy == nullptr) {
      return STG_E_MEDIUMFULL;
    }
    medium->tymed = TYMED_HGLOBAL;
    medium->hGlobal = copy;
    medium->pUnkForRelease = nullptr;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GetDataHere(FORMATETC *, STGMEDIUM *) override { return DATA_E_FORMATETC; }

  HRESULT STDMETHODCALLTYPE QueryGetData(FORMATETC *format) override { return is_file_format(format) ? S_OK : DV_E_FORMATETC; }

  HRESULT STDMETHODCALLTYPE GetCanonicalFormatEtc(FORMATETC *, FORMATETC *format) override {
    if (format != nullptr) {
      format->ptd = nullptr;
    }
    return DATA_S_SAMEFORMATETC;
  }

  HRESULT STDMETHODCALLTYPE SetData(FORMATETC *, STGMEDIUM *, BOOL) override { return E_NOTIMPL; }

  HRESULT STDMETHODCALLTYPE EnumFormatEtc(DWORD direction, IEnumFORMATETC **result) override {
    if (result == nullptr) {
      return E_POINTER;
    }
    if (direction != DATADIR_GET) {
      *result = nullptr;
      return E_NOTIMPL;
    }
    *result = new WoxFormatEtcEnumerator(format_);
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE DAdvise(FORMATETC *, DWORD, IAdviseSink *, DWORD *) override { return OLE_E_ADVISENOTSUPPORTED; }

  HRESULT STDMETHODCALLTYPE DUnadvise(DWORD) override { return OLE_E_ADVISENOTSUPPORTED; }

  HRESULT STDMETHODCALLTYPE EnumDAdvise(IEnumSTATDATA **) override { return OLE_E_ADVISENOTSUPPORTED; }

 private:
  std::atomic<ULONG> references_{1};
  HGLOBAL hdrop_ = nullptr;
  FORMATETC format_{};
};

class WoxFileDragSource final : public IDropSource {
 public:
  explicit WoxFileDragSource(HWND owner) : owner_(owner) {}

  bool released_in_source() const { return released_in_source_; }

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID riid, void **object) override {
    if (object == nullptr) {
      return E_POINTER;
    }
    if (riid == IID_IUnknown || riid == IID_IDropSource) {
      *object = static_cast<IDropSource *>(this);
      AddRef();
      return S_OK;
    }
    *object = nullptr;
    return E_NOINTERFACE;
  }

  ULONG STDMETHODCALLTYPE AddRef() override { return ++references_; }

  ULONG STDMETHODCALLTYPE Release() override {
    ULONG count = --references_;
    if (count == 0) {
      delete this;
    }
    return count;
  }

  HRESULT STDMETHODCALLTYPE QueryContinueDrag(BOOL escape_pressed, DWORD key_state) override {
    if (escape_pressed) {
      return DRAGDROP_S_CANCEL;
    }
    if ((key_state & MK_LBUTTON) == 0) {
      POINT cursor{};
      RECT source_rect{};
      if (owner_ != nullptr && ::GetCursorPos(&cursor) && ::GetWindowRect(owner_, &source_rect) && ::PtInRect(&source_rect, cursor)) {
        released_in_source_ = true;
        return DRAGDROP_S_CANCEL;
      }
      if (owner_ != nullptr) {
        // Hide before the target processes the drop so a late overwrite or permission dialog owns the foreground UI.
        ::ShowWindow(owner_, SW_HIDE);
      }
      return DRAGDROP_S_DROP;
    }
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GiveFeedback(DWORD) override { return DRAGDROP_S_USEDEFAULTCURSORS; }

 private:
  std::atomic<ULONG> references_{1};
  HWND owner_ = nullptr;
  bool released_in_source_ = false;
};

}  // namespace

extern "C" int32_t wox_windows_start_file_drag(uintptr_t owner, const char *const *paths, int32_t path_count) {
  if (paths == nullptr || path_count <= 0) {
    return -1;
  }

  std::vector<std::wstring> files;
  files.reserve(static_cast<size_t>(path_count));
  for (int32_t index = 0; index < path_count; ++index) {
    std::wstring path = utf8_to_wide(paths[index]);
    if (path.empty() || ::GetFileAttributesW(path.c_str()) == INVALID_FILE_ATTRIBUTES) {
      continue;
    }
    files.push_back(std::move(path));
  }
  if (files.empty()) {
    return -1;
  }

  HGLOBAL hdrop = create_file_drop_global(files);
  if (hdrop == nullptr) {
    return -1;
  }

  auto *data_object = new WoxFileDataObject(hdrop);
  auto *drop_source = new WoxFileDragSource(reinterpret_cast<HWND>(owner));
  if (owner != 0) {
    ::ReleaseCapture();
  }
  DWORD effect = DROPEFFECT_NONE;
  HRESULT result = ::DoDragDrop(data_object, drop_source, DROPEFFECT_COPY, &effect);
  bool released_in_source = drop_source->released_in_source();
  data_object->Release();
  drop_source->Release();

  if (result == DRAGDROP_S_DROP && (effect & DROPEFFECT_COPY) != 0) {
    return 0;
  }
  if (released_in_source) {
    return 2;
  }
  if (result == DRAGDROP_S_CANCEL) {
    return 1;
  }
  return -1;
}

template <typename Function>
static Function webview_method(IUnknown *object, size_t index) {
  return reinterpret_cast<Function>((*reinterpret_cast<void ***>(object))[index]);
}

// These slots include inherited methods from the corresponding WebView2 interfaces.
static constexpr size_t kEnvironment3CreateCompositionControllerMethod = 9;
static constexpr size_t kCompositionControllerPutRootVisualTargetMethod = 4;
static constexpr size_t kCompositionControllerSendMouseInputMethod = 5;
static constexpr size_t kControllerMoveFocusMethod = 12;
static constexpr size_t kSettings2PutUserAgentMethod = 22;

static const GUID kCoreWebView2Environment3Iid = {0x80a22ae3, 0xbe7c, 0x4ce2, {0xaf, 0xe1, 0x5a, 0x50, 0x05, 0x6c, 0xde, 0xeb}};
static const GUID kCoreWebView2ControllerIid = {0x4d00c0d1, 0x9434, 0x4eb6, {0x80, 0x78, 0x86, 0x97, 0xa5, 0x60, 0x33, 0x4f}};
static const GUID kCoreWebView2CompositionControllerIid = {0x3df9b733, 0xb9ae, 0x4a15, {0x86, 0xb4, 0xeb, 0x9e, 0xe9, 0x82, 0x64, 0x69}};
static const GUID kCoreWebView2Settings2Iid = {0xee9a0f68, 0xf46c, 0x4e32, {0xac, 0x23, 0xef, 0x8c, 0xac, 0x22, 0x4d, 0x2a}};

static void webview_add_ref(IUnknown *object) {
  if (object != nullptr) {
    webview_method<ULONG(STDMETHODCALLTYPE *)(IUnknown *)>(object, 1)(object);
  }
}

static void webview_release(IUnknown *&object) {
  if (object != nullptr) {
    webview_method<ULONG(STDMETHODCALLTYPE *)(IUnknown *)>(object, 2)(object);
    object = nullptr;
  }
}

static std::wstring utf8_to_wide(const char *value) {
  if (value == nullptr || value[0] == '\0') {
    return {};
  }
  int length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, value, -1, nullptr, 0);
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

static std::wstring javascript_string(const std::wstring &value) {
  std::wstring result = L"\"";
  for (wchar_t character : value) {
    switch (character) {
    case L'\\':
      result += L"\\\\";
      break;
    case L'\"':
      result += L"\\\"";
      break;
    case L'\n':
      result += L"\\n";
      break;
    case L'\r':
      result += L"\\r";
      break;
    case L'\t':
      result += L"\\t";
      break;
    default:
      if (character < 0x20) {
        wchar_t escape[7] = {};
        swprintf(escape, 7, L"\\u%04x", static_cast<unsigned int>(character));
        result += escape;
      } else {
        result += character;
      }
      break;
    }
  }
  result += L'\"';
  return result;
}

static HMODULE load_webview2_loader() {
  wchar_t configured[MAX_PATH] = {};
  DWORD configured_length = GetEnvironmentVariableW(L"WOX_WEBVIEW2_LOADER_PATH", configured, MAX_PATH);
  if (configured_length > 0 && configured_length < MAX_PATH) {
    HMODULE library = LoadLibraryW(configured);
    if (library != nullptr) {
      return library;
    }
  }
  HMODULE library = LoadLibraryW(L"WebView2Loader.dll");
  if (library != nullptr) {
    return library;
  }
  wchar_t executable[MAX_PATH] = {};
  DWORD length = GetModuleFileNameW(nullptr, executable, MAX_PATH);
  if (length == 0 || length >= MAX_PATH) {
    return nullptr;
  }
  wchar_t *separator = wcsrchr(executable, L'\\');
  if (separator == nullptr) {
    return nullptr;
  }
  *(separator + 1) = L'\0';
  if (wcslen(executable) + wcslen(L"WebView2Loader.dll") >= MAX_PATH) {
    return nullptr;
  }
  wcscat(executable, L"WebView2Loader.dll");
  return LoadLibraryW(executable);
}

static std::wstring webview_user_data_folder() {
  wchar_t local_app_data[MAX_PATH] = {};
  DWORD length = GetEnvironmentVariableW(L"LOCALAPPDATA", local_app_data, MAX_PATH);
  if (length == 0 || length >= MAX_PATH) {
    return {};
  }
  std::wstring parent = std::wstring(local_app_data) + L"\\Wox";
  CreateDirectoryW(parent.c_str(), nullptr);
  std::wstring folder = parent + L"\\GoUIWebView2";
  CreateDirectoryW(folder.c_str(), nullptr);
  return folder;
}

struct WoxWindowsWebViewSession {
  std::string cache_key;
  std::wstring signature;
  std::wstring content_key;
  std::wstring url;
  std::wstring html;
  std::wstring user_agent;
  IUnknown *controller = nullptr;
  IUnknown *composition_controller = nullptr;
  void *composition_visual = nullptr;
  IUnknown *core = nullptr;
  RECT bounds = {};
  bool transient = false;
  bool controller_pending = false;
  bool script_pending = false;
  bool script_ready = false;
  bool visible = false;
  bool retired = false;
  int64_t web_message_token = 0;
  bool web_message_registered = false;
  int64_t history_token = 0;
  bool history_registered = false;
  int64_t source_token = 0;
  bool source_registered = false;
  int64_t accelerator_token = 0;
  bool accelerator_registered = false;
  std::wstring loaded_content_key;
  HRESULT error = S_OK;
};

struct WoxWindowsWebView;

struct WoxWebViewEnvironmentCompletedCallback : public IUnknown {
  virtual HRESULT STDMETHODCALLTYPE Invoke(HRESULT error, IUnknown *environment) = 0;
};
__CRT_UUID_DECL(WoxWebViewEnvironmentCompletedCallback, 0x4e8a3389, 0xc9d8, 0x4bd2, 0xb6, 0xb5, 0x12, 0x4f, 0xee, 0x6c, 0xc1, 0x4d)

struct WoxWebViewControllerCompletedCallback : public IUnknown {
  virtual HRESULT STDMETHODCALLTYPE Invoke(HRESULT error, IUnknown *controller) = 0;
};
__CRT_UUID_DECL(WoxWebViewControllerCompletedCallback, 0x02fab84b, 0x1428, 0x4fb7, 0xad, 0x45, 0x1b, 0x2e, 0x64, 0x73, 0x61, 0x84)

struct WoxWebViewScriptCompletedCallback : public IUnknown {
  virtual HRESULT STDMETHODCALLTYPE Invoke(HRESULT error, const wchar_t *script_id) = 0;
};
__CRT_UUID_DECL(WoxWebViewScriptCompletedCallback, 0xb99369f3, 0x9b11, 0x47b5, 0xbc, 0x6f, 0x8e, 0x78, 0x95, 0xfc, 0xea, 0x17)

struct WoxWebViewMessageCallback : public IUnknown {
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args) = 0;
};
__CRT_UUID_DECL(WoxWebViewMessageCallback, 0x57213f19, 0x00e6, 0x49fa, 0x8e, 0x07, 0x89, 0x8e, 0xa0, 0x1e, 0xcb, 0xd2)

struct WoxWebViewHistoryChangedCallback : public IUnknown {
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args) = 0;
};
__CRT_UUID_DECL(WoxWebViewHistoryChangedCallback, 0xc79a420c, 0xefd9, 0x4058, 0x92, 0x95, 0x3e, 0x8b, 0x4b, 0xc8, 0x6a, 0xfe)

struct WoxWebViewSourceChangedCallback : public IUnknown {
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args) = 0;
};
__CRT_UUID_DECL(WoxWebViewSourceChangedCallback, 0x3c067f9f, 0x5388, 0x4772, 0x8b, 0x48, 0x67, 0xed, 0x72, 0xc6, 0xbe, 0xcd)

struct WoxWebViewAcceleratorKeyPressedCallback : public IUnknown {
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args) = 0;
};
__CRT_UUID_DECL(WoxWebViewAcceleratorKeyPressedCallback, 0xb29c7e28, 0xfa79, 0x41a8, 0x8e, 0x44, 0x65, 0x81, 0x1c, 0x76, 0xdc, 0xb2)

class WoxEnvironmentCompletedHandler final : public WoxWebViewEnvironmentCompletedCallback {
public:
  explicit WoxEnvironmentCompletedHandler(WoxWindowsWebView *owner);
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **object) override;
  ULONG STDMETHODCALLTYPE AddRef() override;
  ULONG STDMETHODCALLTYPE Release() override;
  virtual HRESULT STDMETHODCALLTYPE Invoke(HRESULT error, IUnknown *environment);

private:
  ~WoxEnvironmentCompletedHandler() = default;
  std::atomic<ULONG> references_{1};
  WoxWindowsWebView *owner_;
};

class WoxControllerCompletedHandler final : public WoxWebViewControllerCompletedCallback {
public:
  WoxControllerCompletedHandler(WoxWindowsWebView *owner, WoxWindowsWebViewSession *session);
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **object) override;
  ULONG STDMETHODCALLTYPE AddRef() override;
  ULONG STDMETHODCALLTYPE Release() override;
  virtual HRESULT STDMETHODCALLTYPE Invoke(HRESULT error, IUnknown *controller);

private:
  ~WoxControllerCompletedHandler() = default;
  std::atomic<ULONG> references_{1};
  WoxWindowsWebView *owner_;
  WoxWindowsWebViewSession *session_;
};

class WoxScriptCompletedHandler final : public WoxWebViewScriptCompletedCallback {
public:
  WoxScriptCompletedHandler(WoxWindowsWebView *owner, WoxWindowsWebViewSession *session);
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **object) override;
  ULONG STDMETHODCALLTYPE AddRef() override;
  ULONG STDMETHODCALLTYPE Release() override;
  virtual HRESULT STDMETHODCALLTYPE Invoke(HRESULT error, const wchar_t *script_id);

private:
  ~WoxScriptCompletedHandler() = default;
  std::atomic<ULONG> references_{1};
  WoxWindowsWebView *owner_;
  WoxWindowsWebViewSession *session_;
};

class WoxWebMessageHandler final : public WoxWebViewMessageCallback {
public:
  explicit WoxWebMessageHandler(WoxWindowsWebView *owner);
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **object) override;
  ULONG STDMETHODCALLTYPE AddRef() override;
  ULONG STDMETHODCALLTYPE Release() override;
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args);

private:
  ~WoxWebMessageHandler() = default;
  std::atomic<ULONG> references_{1};
  WoxWindowsWebView *owner_;
};

class WoxHistoryChangedHandler final : public WoxWebViewHistoryChangedCallback {
public:
  explicit WoxHistoryChangedHandler(WoxWindowsWebView *owner);
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **object) override;
  ULONG STDMETHODCALLTYPE AddRef() override;
  ULONG STDMETHODCALLTYPE Release() override;
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args);

private:
  ~WoxHistoryChangedHandler() = default;
  std::atomic<ULONG> references_{1};
  WoxWindowsWebView *owner_;
};

class WoxSourceChangedHandler final : public WoxWebViewSourceChangedCallback {
public:
  explicit WoxSourceChangedHandler(WoxWindowsWebView *owner);
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **object) override;
  ULONG STDMETHODCALLTYPE AddRef() override;
  ULONG STDMETHODCALLTYPE Release() override;
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args);

private:
  ~WoxSourceChangedHandler() = default;
  std::atomic<ULONG> references_{1};
  WoxWindowsWebView *owner_;
};

class WoxAcceleratorKeyPressedHandler final : public WoxWebViewAcceleratorKeyPressedCallback {
public:
  explicit WoxAcceleratorKeyPressedHandler(WoxWindowsWebView *owner);
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **object) override;
  ULONG STDMETHODCALLTYPE AddRef() override;
  ULONG STDMETHODCALLTYPE Release() override;
  virtual HRESULT STDMETHODCALLTYPE Invoke(IUnknown *sender, IUnknown *args);

private:
  ~WoxAcceleratorKeyPressedHandler() = default;
  std::atomic<ULONG> references_{1};
  WoxWindowsWebView *owner_;
};

struct WoxWindowsWebView {
  using CreateEnvironment = HRESULT(STDAPICALLTYPE *)(const wchar_t *, const wchar_t *, IUnknown *, IUnknown *);

  WoxWindowsWebView(HWND owner_window, WoxRenderer *renderer_handle) : owner(owner_window), renderer(renderer_handle) {}

  void retain() { references.fetch_add(1, std::memory_order_relaxed); }

  void release() {
    if (references.fetch_sub(1, std::memory_order_acq_rel) == 1) {
      delete this;
    }
  }

  HRESULT initialize() {
    loader = load_webview2_loader();
    if (loader == nullptr) {
      webview_debug("loader missing");
      return HRESULT_FROM_WIN32(ERROR_MOD_NOT_FOUND);
    }
    FARPROC procedure = GetProcAddress(loader, "CreateCoreWebView2EnvironmentWithOptions");
    if (procedure == nullptr) {
      return HRESULT_FROM_WIN32(ERROR_PROC_NOT_FOUND);
    }
    CreateEnvironment create_environment = nullptr;
    static_assert(sizeof(create_environment) == sizeof(procedure));
    std::memcpy(&create_environment, &procedure, sizeof(create_environment));
    std::wstring user_data = webview_user_data_folder();
    auto *handler = new WoxEnvironmentCompletedHandler(this);
    HRESULT result = create_environment(nullptr, user_data.empty() ? nullptr : user_data.c_str(), nullptr, handler);
    webview_debug("create environment returned 0x%08X", static_cast<unsigned int>(result));
    handler->Release();
    return result;
  }

  void environment_completed(HRESULT result, IUnknown *created_environment) {
    webview_debug("environment completed 0x%08X environment=%p", static_cast<unsigned int>(result), created_environment);
    if (closing) {
      return;
    }
    if (FAILED(result) || created_environment == nullptr) {
      fatal_error = FAILED(result) ? result : E_FAIL;
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    environment = created_environment;
    webview_add_ref(environment);
    result = environment->QueryInterface(kCoreWebView2Environment3Iid, reinterpret_cast<void **>(&environment3));
    if (FAILED(result) || environment3 == nullptr) {
      fatal_error = FAILED(result) ? result : E_NOINTERFACE;
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    if (active != nullptr) {
      create_controller(active);
    }
    InvalidateRect(owner, nullptr, FALSE);
  }

  void create_controller(WoxWindowsWebViewSession *session) {
    if (environment3 == nullptr || session == nullptr || session->controller != nullptr || session->controller_pending || session->retired) {
      return;
    }
    session->controller_pending = true;
    auto *handler = new WoxControllerCompletedHandler(this, session);
    using CreateController = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, HWND, IUnknown *);
    HRESULT result = webview_method<CreateController>(environment3, kEnvironment3CreateCompositionControllerMethod)(environment3, owner, handler);
    webview_debug("create composition controller returned 0x%08X session=%p", static_cast<unsigned int>(result), session);
    handler->Release();
    if (FAILED(result)) {
      session->controller_pending = false;
      session->error = result;
      InvalidateRect(owner, nullptr, FALSE);
    }
  }

  void controller_completed(WoxWindowsWebViewSession *session, HRESULT result, IUnknown *created_controller) {
    webview_debug("controller completed 0x%08X session=%p controller=%p", static_cast<unsigned int>(result), session, created_controller);
    session->controller_pending = false;
    if (closing || session->retired) {
      if (created_controller != nullptr) {
        IUnknown *controller = nullptr;
        using Close = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
        if (SUCCEEDED(created_controller->QueryInterface(kCoreWebView2ControllerIid, reinterpret_cast<void **>(&controller))) && controller != nullptr) {
          webview_method<Close>(controller, 24)(controller);
          controller->Release();
        }
      }
      return;
    }
    if (FAILED(result) || created_controller == nullptr) {
      session->error = FAILED(result) ? result : E_FAIL;
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    result = created_controller->QueryInterface(kCoreWebView2CompositionControllerIid, reinterpret_cast<void **>(&session->composition_controller));
    if (FAILED(result) || session->composition_controller == nullptr) {
      session->error = FAILED(result) ? result : E_NOINTERFACE;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    result = session->composition_controller->QueryInterface(kCoreWebView2ControllerIid, reinterpret_cast<void **>(&session->controller));
    if (FAILED(result) || session->controller == nullptr) {
      session->error = FAILED(result) ? result : E_NOINTERFACE;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    void *root_visual_target = nullptr;
    result = wox_renderer_create_webview_visual(renderer, &session->composition_visual, &root_visual_target);
    if (SUCCEEDED(result)) {
      using PutRootVisualTarget = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown *);
      result = webview_method<PutRootVisualTarget>(session->composition_controller, kCompositionControllerPutRootVisualTargetMethod)(session->composition_controller, static_cast<IUnknown *>(root_visual_target));
    }
    if (FAILED(result)) {
      session->error = result;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    using GetCore = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown **);
    result = webview_method<GetCore>(session->controller, 25)(session->controller, &session->core);
    webview_debug("get core returned 0x%08X core=%p", static_cast<unsigned int>(result), session->core);
    if (FAILED(result) || session->core == nullptr) {
      session->error = FAILED(result) ? result : E_FAIL;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    result = configure_user_agent(session);
    if (FAILED(result)) {
      session->error = result;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    register_message_handler(session);
    register_navigation_handlers(session);
    register_accelerator_handler(session);
    if (!session->retired) {
      configure_script(session);
    }
  }

  // configure_user_agent applies only explicit overrides so WebView2 can keep its installed desktop identity by default.
  HRESULT configure_user_agent(WoxWindowsWebViewSession *session) {
    if (session == nullptr || session->core == nullptr || session->user_agent.empty()) {
      return S_OK;
    }
    IUnknown *settings = nullptr;
    using GetSettings = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown **);
    HRESULT result = webview_method<GetSettings>(session->core, 3)(session->core, &settings);
    if (FAILED(result) || settings == nullptr) {
      webview_release(settings);
      return FAILED(result) ? result : E_NOINTERFACE;
    }
    IUnknown *settings2 = nullptr;
    result = settings->QueryInterface(kCoreWebView2Settings2Iid, reinterpret_cast<void **>(&settings2));
    webview_release(settings);
    if (FAILED(result) || settings2 == nullptr) {
      webview_release(settings2);
      return FAILED(result) ? result : E_NOINTERFACE;
    }
    using PutUserAgent = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, const wchar_t *);
    result = webview_method<PutUserAgent>(settings2, kSettings2PutUserAgentMethod)(settings2, session->user_agent.c_str());
    webview_release(settings2);
    return result;
  }

  void register_message_handler(WoxWindowsWebViewSession *session) {
    auto *handler = new WoxWebMessageHandler(this);
    using AddWebMessageHandler = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown *, int64_t *);
    HRESULT result = webview_method<AddWebMessageHandler>(session->core, 34)(session->core, handler, &session->web_message_token);
    webview_debug("add web message handler returned 0x%08X token=%lld", static_cast<unsigned int>(result), static_cast<long long>(session->web_message_token));
    handler->Release();
    if (FAILED(result)) {
      session->error = result;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    session->web_message_registered = true;
  }

  void register_navigation_handlers(WoxWindowsWebViewSession *session) {
    if (session == nullptr || session->core == nullptr || session->retired) {
      return;
    }
    using AddHandler = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown *, int64_t *);
    if (!session->history_registered) {
      auto *handler = new WoxHistoryChangedHandler(this);
      HRESULT result = webview_method<AddHandler>(session->core, 13)(session->core, handler, &session->history_token);
      webview_debug("add history changed handler returned 0x%08X", static_cast<unsigned int>(result));
      handler->Release();
      if (SUCCEEDED(result)) {
        session->history_registered = true;
      }
    }
    if (!session->source_registered) {
      auto *handler = new WoxSourceChangedHandler(this);
      HRESULT result = webview_method<AddHandler>(session->core, 11)(session->core, handler, &session->source_token);
      webview_debug("add source changed handler returned 0x%08X", static_cast<unsigned int>(result));
      handler->Release();
      if (SUCCEEDED(result)) {
        session->source_registered = true;
      }
    }
    notify_navigation_changed(session);
  }

  void register_accelerator_handler(WoxWindowsWebViewSession *session) {
    if (session == nullptr || session->controller == nullptr || session->retired || session->accelerator_registered) {
      return;
    }
    auto *handler = new WoxAcceleratorKeyPressedHandler(this);
    using AddHandler = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown *, int64_t *);
    HRESULT result = webview_method<AddHandler>(session->controller, 19)(session->controller, handler, &session->accelerator_token);
    webview_debug("add accelerator handler returned 0x%08X", static_cast<unsigned int>(result));
    handler->Release();
    if (SUCCEEDED(result)) {
      session->accelerator_registered = true;
    }
  }

  void accelerator_key_pressed(IUnknown *args) {
    if (closing || args == nullptr) {
      return;
    }
    int32_t kind = 0;
    uint32_t virtual_key = 0;
    using GetInt32 = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, int32_t *);
    using GetUInt32 = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, uint32_t *);
    HRESULT kind_result = webview_method<GetInt32>(args, 3)(args, &kind);
    HRESULT key_result = webview_method<GetUInt32>(args, 4)(args, &virtual_key);
    bool primary_j = SUCCEEDED(kind_result) && SUCCEEDED(key_result) && kind == 0 && virtual_key == 'J' &&
                     (GetKeyState(VK_CONTROL) & 0x8000) != 0 && (GetKeyState(VK_MENU) & 0x8000) == 0 && (GetKeyState(VK_SHIFT) & 0x8000) == 0;
    if (!primary_j) {
      return;
    }
    using PutHandled = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, BOOL);
    webview_method<PutHandled>(args, 8)(args, TRUE);
    SetFocus(owner);
    woxGoWindowsWebViewActionPanel(reinterpret_cast<uintptr_t>(owner));
  }

  void notify_navigation_changed(WoxWindowsWebViewSession *session) {
    if (closing || session == nullptr || session->core == nullptr || session != active) {
      return;
    }
    wchar_t *source = nullptr;
    using GetSource = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, wchar_t **);
    webview_method<GetSource>(session->core, 4)(session->core, &source);
    BOOL can_go_back = FALSE;
    BOOL can_go_forward = FALSE;
    using GetBool = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, BOOL *);
    webview_method<GetBool>(session->core, 38)(session->core, &can_go_back);
    webview_method<GetBool>(session->core, 39)(session->core, &can_go_forward);
    std::string utf8 = wide_to_utf8(source != nullptr ? source : L"");
    if (source != nullptr) {
      CoTaskMemFree(source);
    }
    woxGoWindowsWebViewNavigationChanged(reinterpret_cast<uintptr_t>(owner), utf8.c_str(), can_go_back ? 1 : 0, can_go_forward ? 1 : 0);
  }

  HRESULT go_back() {
    if (active == nullptr || active->core == nullptr) {
      return E_FAIL;
    }
    using GoBack = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
    return webview_method<GoBack>(active->core, 40)(active->core);
  }

  HRESULT go_forward() {
    if (active == nullptr || active->core == nullptr) {
      return E_FAIL;
    }
    using GoForward = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
    return webview_method<GoForward>(active->core, 41)(active->core);
  }

  HRESULT reload() {
    if (active == nullptr || active->core == nullptr) {
      return E_FAIL;
    }
    using Reload = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
    return webview_method<Reload>(active->core, 31)(active->core);
  }

  HRESULT open_dev_tools() {
    if (active == nullptr || active->core == nullptr) {
      return E_FAIL;
    }
    // OpenDevToolsWindow is slot 51 on the stable base ICoreWebView2 interface.
    using OpenDevToolsWindow = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
    return webview_method<OpenDevToolsWindow>(active->core, 51)(active->core);
  }

  HRESULT open_in_browser() {
    if (active == nullptr || active->core == nullptr) {
      return E_FAIL;
    }
    wchar_t *source = nullptr;
    using GetSource = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, wchar_t **);
    HRESULT result = webview_method<GetSource>(active->core, 4)(active->core, &source);
    if (FAILED(result) || source == nullptr) {
      if (source != nullptr) {
        CoTaskMemFree(source);
      }
      return FAILED(result) ? result : E_FAIL;
    }
    std::wstring url(source);
    CoTaskMemFree(source);
    if (url.rfind(L"http://", 0) != 0 && url.rfind(L"https://", 0) != 0) {
      return E_FAIL;
    }
    HINSTANCE launched = ShellExecuteW(owner, L"open", url.c_str(), nullptr, nullptr, SW_SHOWNORMAL);
    return reinterpret_cast<intptr_t>(launched) > 32 ? S_OK : E_FAIL;
  }

  HRESULT navigation_state(char **url, int32_t *can_go_back, int32_t *can_go_forward) {
    if (url == nullptr || can_go_back == nullptr || can_go_forward == nullptr) {
      return E_INVALIDARG;
    }
    *url = nullptr;
    *can_go_back = 0;
    *can_go_forward = 0;
    if (active == nullptr || active->core == nullptr) {
      return E_FAIL;
    }
    wchar_t *source = nullptr;
    using GetSource = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, wchar_t **);
    HRESULT result = webview_method<GetSource>(active->core, 4)(active->core, &source);
    if (FAILED(result)) {
      return result;
    }
    std::string utf8 = wide_to_utf8(source != nullptr ? source : L"");
    if (source != nullptr) {
      CoTaskMemFree(source);
    }
    *url = static_cast<char *>(std::malloc(utf8.size() + 1));
    if (*url == nullptr) {
      return E_OUTOFMEMORY;
    }
    std::memcpy(*url, utf8.c_str(), utf8.size() + 1);
    BOOL back = FALSE;
    BOOL forward = FALSE;
    using GetBool = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, BOOL *);
    webview_method<GetBool>(active->core, 38)(active->core, &back);
    webview_method<GetBool>(active->core, 39)(active->core, &forward);
    *can_go_back = back ? 1 : 0;
    *can_go_forward = forward ? 1 : 0;
    return S_OK;
  }

  HRESULT pointer(int32_t kind, POINT point, int32_t button, int32_t scroll_x, int32_t scroll_y, uint32_t modifiers) {
    if (active == nullptr || active->controller == nullptr || active->composition_controller == nullptr || !active->visible) {
      return E_FAIL;
    }
    uint32_t virtual_keys = 0;
    if ((modifiers & 1) != 0) virtual_keys |= 0x0004;
    if ((modifiers & 2) != 0) virtual_keys |= 0x0008;
    if ((GetKeyState(VK_LBUTTON) & 0x8000) != 0) virtual_keys |= 0x0001;
    if ((GetKeyState(VK_RBUTTON) & 0x8000) != 0) virtual_keys |= 0x0002;
    if ((GetKeyState(VK_MBUTTON) & 0x8000) != 0) virtual_keys |= 0x0010;

    uint32_t event_kind = 0x0200;
    uint32_t mouse_data = 0;
    if (kind == 2) {
      event_kind = 0x02A3;
      virtual_keys = 0;
      point = {};
    } else if (kind == 3 || kind == 4) {
      const bool down = kind == 3;
      if (button == 1) event_kind = down ? 0x0201 : 0x0202;
      if (button == 2) event_kind = down ? 0x0204 : 0x0205;
      if (button == 3) event_kind = down ? 0x0207 : 0x0208;
      if (down) {
        // Focus belongs to ICoreWebView2Controller; the composition interface only accepts visual and pointer input.
        using MoveFocus = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, int32_t);
        webview_method<MoveFocus>(active->controller, kControllerMoveFocusMethod)(active->controller, 0);
      }
    } else if (kind == 5) {
      event_kind = scroll_x != 0 ? 0x020E : 0x020A;
      const int32_t wheel_delta = scroll_x != 0 ? scroll_x : scroll_y;
      // SendMouseInput takes the signed wheel delta itself, not WM_MOUSEWHEEL's packed wParam.
      mouse_data = static_cast<uint32_t>(wheel_delta);
    }
    using SendMouseInput = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, uint32_t, uint32_t, uint32_t, POINT);
    return webview_method<SendMouseInput>(active->composition_controller, kCompositionControllerSendMouseInputMethod)(active->composition_controller, event_kind, virtual_keys, mouse_data, point);
  }

  void web_message_received(IUnknown *args) {
    if (closing || args == nullptr) {
      return;
    }
    wchar_t *message = nullptr;
    using TryGetString = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, wchar_t **);
    HRESULT result = webview_method<TryGetString>(args, 5)(args, &message);
    webview_debug("web message received 0x%08X value=%ls", static_cast<unsigned int>(result), message != nullptr ? message : L"");
    constexpr wchar_t escape_diagnostic_prefix[] = L"wox-escape-diagnostic:";
    constexpr size_t escape_diagnostic_prefix_length = (sizeof(escape_diagnostic_prefix) / sizeof(wchar_t)) - 1;
    if (SUCCEEDED(result) && message != nullptr && wcsncmp(message, escape_diagnostic_prefix, escape_diagnostic_prefix_length) == 0) {
      std::string detail = wide_to_utf8(message + escape_diagnostic_prefix_length);
      woxGoWindowsWebViewEscapeDiagnostic(reinterpret_cast<uintptr_t>(owner), detail.c_str());
    } else if (SUCCEEDED(result) && message != nullptr && wcscmp(message, L"wox-unhandled-escape") == 0) {
      // The Go Host can choose the next logical focus owner only after the native WebView releases keyboard focus.
      SetFocus(owner);
      woxGoWindowsWebViewEscapeDiagnostic(reinterpret_cast<uintptr_t>(owner), GetFocus() == owner ? "native-focus-restored" : "native-focus-missing");
      woxGoWindowsWebViewEscape(reinterpret_cast<uintptr_t>(owner));
    }
    if (message != nullptr) {
      CoTaskMemFree(message);
    }
  }

  void configure_script(WoxWindowsWebViewSession *session) {
    // WebView2 runs document-created scripts before HTML parsing, so attach CSS as soon as the parser creates a root node.
    // Global page routers may always prevent Escape, so only an observable page transition claims it.
    std::wstring script = L"(()=>{const c=" + javascript_string(session->signature) +
                          L";if(c){const apply=()=>{const root=document.head||document.documentElement;if(!root)return false;let s=document.getElementById('wox-webview-preview-style');if(!s){s=document.createElement('style');s.id='wox-webview-preview-style';root.appendChild(s)}s.textContent=c;return true};if(!apply()){const observer=new MutationObserver(()=>{if(apply())observer.disconnect()});observer.observe(document,{childList:true})}}"
                          L"if(window.__woxUnhandledEscapeInstalled__)return;window.__woxUnhandledEscapeInstalled__=true;document.addEventListener('keydown',e=>{if(e.key!=='Escape'||e.repeat)return;const f=document.activeElement;const d=n=>!n?'none':(n.tagName||'node').toLowerCase()+(n.type?'[type='+n.type+']':'');let m=false;const o=new MutationObserver(()=>{m=true});if(document.documentElement)o.observe(document.documentElement,{attributes:true,childList:true,characterData:true,subtree:true});setTimeout(()=>{o.disconnect();const a=document.activeElement;const r=(f&&f!==a)?'page-focus-changed':m?'page-dom-changed':e.defaultPrevented?'page-prevented-no-change-forwarded':'page-forwarded';window.chrome.webview.postMessage('wox-escape-diagnostic:'+r+' before='+d(f)+' after='+d(a));if(r==='page-forwarded'||r==='page-prevented-no-change-forwarded')window.chrome.webview.postMessage('wox-unhandled-escape')},0)},true)})()";
    session->script_pending = true;
    auto *handler = new WoxScriptCompletedHandler(this, session);
    using AddScript = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, const wchar_t *, IUnknown *);
    HRESULT result = webview_method<AddScript>(session->core, 27)(session->core, script.c_str(), handler);
    webview_debug("add startup script returned 0x%08X", static_cast<unsigned int>(result));
    handler->Release();
    if (FAILED(result)) {
      session->script_pending = false;
      session->error = result;
      InvalidateRect(owner, nullptr, FALSE);
    }
  }

  void script_completed(WoxWindowsWebViewSession *session, HRESULT result) {
    webview_debug("startup script completed 0x%08X session=%p", static_cast<unsigned int>(result), session);
    session->script_pending = false;
    if (closing || session->retired) {
      return;
    }
    if (FAILED(result)) {
      session->error = result;
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    session->script_ready = true;
    apply_session(session);
  }

  void apply_session(WoxWindowsWebViewSession *session) {
    if (session == nullptr || session->controller == nullptr || session->core == nullptr || !session->script_ready || session->retired || FAILED(session->error)) {
      return;
    }
    using PutBounds = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, RECT);
    using PutVisible = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, BOOL);
    RECT local_bounds = {0, 0, session->bounds.right - session->bounds.left, session->bounds.bottom - session->bounds.top};
    HRESULT result = webview_method<PutBounds>(session->controller, 6)(session->controller, local_bounds);
    if (SUCCEEDED(result)) {
      result = wox_renderer_set_webview_visual_bounds(renderer, session->composition_visual, static_cast<float>(session->bounds.left), static_cast<float>(session->bounds.top), static_cast<float>(local_bounds.right), static_cast<float>(local_bounds.bottom));
    }
    if (SUCCEEDED(result)) {
      result = webview_method<PutVisible>(session->controller, 4)(session->controller, session->visible ? TRUE : FALSE);
    }
    webview_debug("apply bounds/visibility returned 0x%08X visible=%d", static_cast<unsigned int>(result), session->visible ? 1 : 0);
    if (FAILED(result)) {
      session->error = result;
      return;
    }
    if (!session->visible || session->loaded_content_key == session->content_key) {
      return;
    }
    using Navigate = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, const wchar_t *);
    if (!session->html.empty()) {
      result = webview_method<Navigate>(session->core, 6)(session->core, session->html.c_str());
      webview_debug("navigate HTML returned 0x%08X chars=%zu", static_cast<unsigned int>(result), session->html.size());
    } else {
      result = webview_method<Navigate>(session->core, 5)(session->core, session->url.c_str());
      webview_debug("navigate URL returned 0x%08X", static_cast<unsigned int>(result));
    }
    if (SUCCEEDED(result)) {
      session->loaded_content_key = session->content_key;
      notify_navigation_changed(session);
    } else {
      session->error = result;
    }
  }

  void set_visible(WoxWindowsWebViewSession *session, bool visible) {
    if (session == nullptr || session->retired) {
      return;
    }
    session->visible = visible;
    apply_session(session);
  }

  void dispose_session(WoxWindowsWebViewSession *session) {
    if (session == nullptr || session->retired) {
      return;
    }
    session->retired = true;
    session->visible = false;
    if (session->core != nullptr) {
      using RemoveHandler = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, int64_t);
      if (session->history_registered) {
        webview_method<RemoveHandler>(session->core, 14)(session->core, session->history_token);
        session->history_registered = false;
      }
      if (session->source_registered) {
        webview_method<RemoveHandler>(session->core, 12)(session->core, session->source_token);
        session->source_registered = false;
      }
      if (session->web_message_registered) {
        webview_method<RemoveHandler>(session->core, 35)(session->core, session->web_message_token);
        session->web_message_registered = false;
      }
    }
    if (session->controller != nullptr) {
      using PutVisible = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, BOOL);
      using Close = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
      if (session->accelerator_registered) {
        using RemoveHandler = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, int64_t);
        webview_method<RemoveHandler>(session->controller, 20)(session->controller, session->accelerator_token);
        session->accelerator_registered = false;
      }
      webview_method<PutVisible>(session->controller, 4)(session->controller, FALSE);
      webview_method<Close>(session->controller, 24)(session->controller);
    }
    if (session->composition_controller != nullptr) {
      using PutRootVisualTarget = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown *);
      webview_method<PutRootVisualTarget>(session->composition_controller, kCompositionControllerPutRootVisualTargetMethod)(session->composition_controller, nullptr);
    }
    if (session->composition_visual != nullptr) {
      wox_renderer_remove_webview_visual(renderer, session->composition_visual);
      session->composition_visual = nullptr;
    }
    webview_release(session->core);
    webview_release(session->controller);
    webview_release(session->composition_controller);
  }

  HRESULT show(const char *url, const char *html, const char *inject_css, const char *user_agent, bool cache_disabled, const char *cache_key, RECT bounds) {
    if (closing) {
      return E_FAIL;
    }
    if (FAILED(fatal_error)) {
      return fatal_error;
    }
    std::wstring wide_url = utf8_to_wide(url);
    std::wstring wide_html = utf8_to_wide(html);
    std::wstring signature = utf8_to_wide(inject_css);
    std::wstring wide_user_agent = utf8_to_wide(user_agent);
    std::wstring content_key = (wide_html.empty() ? L"url|" + wide_url : L"html|" + wide_html);
    std::string key = cache_key != nullptr ? cache_key : "";
    bool use_cache = !cache_disabled && !key.empty();
    WoxWindowsWebViewSession *session = nullptr;
    if (use_cache) {
      auto cached = cache.find(key);
      if (cached != cache.end() && !cached->second->retired && cached->second->signature == signature && cached->second->user_agent == wide_user_agent) {
        session = cached->second;
      } else {
        if (cached != cache.end()) {
          dispose_session(cached->second);
        }
        session = new_session(key, signature, wide_user_agent, false);
        cache[key] = session;
      }
    } else if (active != nullptr && active->transient && !active->retired && active->signature == signature && active->user_agent == wide_user_agent && active->content_key == content_key) {
      session = active;
    } else {
      session = new_session({}, signature, wide_user_agent, true);
    }
    if (active != session) {
      if (active != nullptr) {
        if (active->transient) {
          dispose_session(active);
        } else {
          set_visible(active, false);
        }
      }
      active = session;
    }
    session->url = std::move(wide_url);
    session->html = std::move(wide_html);
    if (session->content_key != content_key) {
      // A changed document gets a fresh navigation attempt even if the previous one failed.
      session->error = S_OK;
    }
    session->content_key = std::move(content_key);
    session->bounds = bounds;
    session->visible = true;
    if (environment != nullptr) {
      create_controller(session);
      apply_session(session);
    }
    return session->error;
  }

  HRESULT hide() {
    if (active == nullptr) {
      return S_OK;
    }
    if (active->transient) {
      dispose_session(active);
    } else {
      set_visible(active, false);
    }
    active = nullptr;
    return S_OK;
  }

  void close() {
    if (closing) {
      return;
    }
    closing = true;
    active = nullptr;
    for (const auto &session : sessions) {
      dispose_session(session.get());
    }
    cache.clear();
    webview_release(environment3);
    webview_release(environment);
  }

  WoxWindowsWebViewSession *new_session(std::string key, std::wstring signature, std::wstring user_agent, bool transient) {
    auto session = std::make_unique<WoxWindowsWebViewSession>();
    session->cache_key = std::move(key);
    session->signature = std::move(signature);
    session->user_agent = std::move(user_agent);
    session->transient = transient;
    WoxWindowsWebViewSession *value = session.get();
    sessions.push_back(std::move(session));
    return value;
  }

  ~WoxWindowsWebView() {
    close();
    if (loader != nullptr) {
      FreeLibrary(loader);
    }
  }

  std::atomic<ULONG> references{1};
  HWND owner;
  WoxRenderer *renderer;
  HMODULE loader = nullptr;
  IUnknown *environment = nullptr;
  IUnknown *environment3 = nullptr;
  std::vector<std::unique_ptr<WoxWindowsWebViewSession>> sessions;
  std::unordered_map<std::string, WoxWindowsWebViewSession *> cache;
  WoxWindowsWebViewSession *active = nullptr;
  HRESULT fatal_error = S_OK;
  bool closing = false;
};

static HRESULT callback_query_interface(IUnknown *self, REFIID iid, REFIID supported_iid, void **object) {
  if (object == nullptr) {
    return E_POINTER;
  }
  *object = nullptr;
  if (!IsEqualIID(iid, IID_IUnknown) && !IsEqualIID(iid, supported_iid)) {
    return E_NOINTERFACE;
  }
  *object = self;
  webview_add_ref(self);
  return S_OK;
}

WoxEnvironmentCompletedHandler::WoxEnvironmentCompletedHandler(WoxWindowsWebView *owner) : owner_(owner) { owner_->retain(); }
HRESULT WoxEnvironmentCompletedHandler::QueryInterface(REFIID iid, void **object) { return callback_query_interface(this, iid, __uuidof(WoxWebViewEnvironmentCompletedCallback), object); }
ULONG WoxEnvironmentCompletedHandler::AddRef() { return references_.fetch_add(1) + 1; }
ULONG WoxEnvironmentCompletedHandler::Release() {
  ULONG remaining = references_.fetch_sub(1) - 1;
  if (remaining == 0) {
    owner_->release();
    delete this;
  }
  return remaining;
}
HRESULT WoxEnvironmentCompletedHandler::Invoke(HRESULT error, IUnknown *environment) {
  owner_->environment_completed(error, environment);
  return S_OK;
}

WoxControllerCompletedHandler::WoxControllerCompletedHandler(WoxWindowsWebView *owner, WoxWindowsWebViewSession *session) : owner_(owner), session_(session) { owner_->retain(); }
HRESULT WoxControllerCompletedHandler::QueryInterface(REFIID iid, void **object) { return callback_query_interface(this, iid, __uuidof(WoxWebViewControllerCompletedCallback), object); }
ULONG WoxControllerCompletedHandler::AddRef() { return references_.fetch_add(1) + 1; }
ULONG WoxControllerCompletedHandler::Release() {
  ULONG remaining = references_.fetch_sub(1) - 1;
  if (remaining == 0) {
    owner_->release();
    delete this;
  }
  return remaining;
}
HRESULT WoxControllerCompletedHandler::Invoke(HRESULT error, IUnknown *controller) {
  owner_->controller_completed(session_, error, controller);
  return S_OK;
}

WoxScriptCompletedHandler::WoxScriptCompletedHandler(WoxWindowsWebView *owner, WoxWindowsWebViewSession *session) : owner_(owner), session_(session) { owner_->retain(); }
HRESULT WoxScriptCompletedHandler::QueryInterface(REFIID iid, void **object) { return callback_query_interface(this, iid, __uuidof(WoxWebViewScriptCompletedCallback), object); }
ULONG WoxScriptCompletedHandler::AddRef() { return references_.fetch_add(1) + 1; }
ULONG WoxScriptCompletedHandler::Release() {
  ULONG remaining = references_.fetch_sub(1) - 1;
  if (remaining == 0) {
    owner_->release();
    delete this;
  }
  return remaining;
}
HRESULT WoxScriptCompletedHandler::Invoke(HRESULT error, const wchar_t *) {
  owner_->script_completed(session_, error);
  return S_OK;
}

WoxWebMessageHandler::WoxWebMessageHandler(WoxWindowsWebView *owner) : owner_(owner) { owner_->retain(); }
HRESULT WoxWebMessageHandler::QueryInterface(REFIID iid, void **object) { return callback_query_interface(this, iid, __uuidof(WoxWebViewMessageCallback), object); }
ULONG WoxWebMessageHandler::AddRef() { return references_.fetch_add(1) + 1; }
ULONG WoxWebMessageHandler::Release() {
  ULONG remaining = references_.fetch_sub(1) - 1;
  if (remaining == 0) {
    owner_->release();
    delete this;
  }
  return remaining;
}
HRESULT WoxWebMessageHandler::Invoke(IUnknown *, IUnknown *args) {
  owner_->web_message_received(args);
  return S_OK;
}

WoxHistoryChangedHandler::WoxHistoryChangedHandler(WoxWindowsWebView *owner) : owner_(owner) { owner_->retain(); }
HRESULT WoxHistoryChangedHandler::QueryInterface(REFIID iid, void **object) { return callback_query_interface(this, iid, __uuidof(WoxWebViewHistoryChangedCallback), object); }
ULONG WoxHistoryChangedHandler::AddRef() { return references_.fetch_add(1) + 1; }
ULONG WoxHistoryChangedHandler::Release() {
  ULONG remaining = references_.fetch_sub(1) - 1;
  if (remaining == 0) {
    owner_->release();
    delete this;
  }
  return remaining;
}
HRESULT WoxHistoryChangedHandler::Invoke(IUnknown *, IUnknown *) {
  if (owner_->active != nullptr) {
    owner_->notify_navigation_changed(owner_->active);
  }
  return S_OK;
}

WoxSourceChangedHandler::WoxSourceChangedHandler(WoxWindowsWebView *owner) : owner_(owner) { owner_->retain(); }
HRESULT WoxSourceChangedHandler::QueryInterface(REFIID iid, void **object) { return callback_query_interface(this, iid, __uuidof(WoxWebViewSourceChangedCallback), object); }
ULONG WoxSourceChangedHandler::AddRef() { return references_.fetch_add(1) + 1; }
ULONG WoxSourceChangedHandler::Release() {
  ULONG remaining = references_.fetch_sub(1) - 1;
  if (remaining == 0) {
    owner_->release();
    delete this;
  }
  return remaining;
}
HRESULT WoxSourceChangedHandler::Invoke(IUnknown *, IUnknown *) {
  if (owner_->active != nullptr) {
    owner_->notify_navigation_changed(owner_->active);
  }
  return S_OK;
}

WoxAcceleratorKeyPressedHandler::WoxAcceleratorKeyPressedHandler(WoxWindowsWebView *owner) : owner_(owner) { owner_->retain(); }
HRESULT WoxAcceleratorKeyPressedHandler::QueryInterface(REFIID iid, void **object) { return callback_query_interface(this, iid, __uuidof(WoxWebViewAcceleratorKeyPressedCallback), object); }
ULONG WoxAcceleratorKeyPressedHandler::AddRef() { return references_.fetch_add(1) + 1; }
ULONG WoxAcceleratorKeyPressedHandler::Release() {
  ULONG remaining = references_.fetch_sub(1) - 1;
  if (remaining == 0) {
    owner_->release();
    delete this;
  }
  return remaining;
}
HRESULT WoxAcceleratorKeyPressedHandler::Invoke(IUnknown *, IUnknown *args) {
  owner_->accelerator_key_pressed(args);
  return S_OK;
}

extern "C" int32_t wox_windows_webview_create(uintptr_t owner, WoxRenderer *renderer, WoxWindowsWebView **webview) {
  if (owner == 0 || renderer == nullptr || webview == nullptr) {
    return E_INVALIDARG;
  }
  *webview = new WoxWindowsWebView(reinterpret_cast<HWND>(owner), renderer);
  HRESULT result = (*webview)->initialize();
  if (FAILED(result)) {
    (*webview)->release();
    *webview = nullptr;
  }
  return result;
}

extern "C" int32_t wox_windows_webview_show(WoxWindowsWebView *webview, const char *url, const char *html, const char *inject_css, const char *user_agent, int32_t cache_disabled, const char *cache_key, int32_t x, int32_t y, int32_t width, int32_t height) {
  if (webview == nullptr || url == nullptr || html == nullptr || inject_css == nullptr || user_agent == nullptr || cache_key == nullptr || width <= 0 || height <= 0) {
    return E_INVALIDARG;
  }
  RECT bounds = {x, y, x + width, y + height};
  return webview->show(url, html, inject_css, user_agent, cache_disabled != 0, cache_key, bounds);
}

extern "C" int32_t wox_windows_webview_hide(WoxWindowsWebView *webview) {
  return webview != nullptr ? webview->hide() : E_INVALIDARG;
}

extern "C" int32_t wox_windows_webview_go_back(WoxWindowsWebView *webview) {
  return webview != nullptr ? webview->go_back() : E_INVALIDARG;
}

extern "C" int32_t wox_windows_webview_go_forward(WoxWindowsWebView *webview) {
  return webview != nullptr ? webview->go_forward() : E_INVALIDARG;
}

extern "C" int32_t wox_windows_webview_reload(WoxWindowsWebView *webview) {
  return webview != nullptr ? webview->reload() : E_INVALIDARG;
}

extern "C" int32_t wox_windows_webview_open_dev_tools(WoxWindowsWebView *webview) {
  return webview != nullptr ? webview->open_dev_tools() : E_INVALIDARG;
}

extern "C" int32_t wox_windows_webview_open_in_browser(WoxWindowsWebView *webview) {
  return webview != nullptr ? webview->open_in_browser() : E_INVALIDARG;
}

extern "C" int32_t wox_windows_webview_navigation_state(WoxWindowsWebView *webview, char **url, int32_t *can_go_back, int32_t *can_go_forward) {
  return webview != nullptr ? webview->navigation_state(url, can_go_back, can_go_forward) : E_INVALIDARG;
}

extern "C" int32_t wox_windows_webview_pointer(WoxWindowsWebView *webview, int32_t kind, int32_t x, int32_t y, int32_t button, int32_t scroll_x, int32_t scroll_y, uint32_t modifiers) {
  if (webview == nullptr) {
    return E_INVALIDARG;
  }
  return webview->pointer(kind, POINT{x, y}, button, scroll_x, scroll_y, modifiers);
}

extern "C" void wox_windows_webview_destroy(WoxWindowsWebView *webview) {
  if (webview != nullptr) {
    webview->close();
    webview->release();
  }
}

static HRESULT last_error_result() {
  const DWORD error = GetLastError();
  return HRESULT_FROM_WIN32(error == ERROR_SUCCESS ? ERROR_GEN_FAILURE : error);
}

static bool open_clipboard_with_retry(HWND owner) {
  for (int attempt = 0; attempt < 10; ++attempt) {
    if (OpenClipboard(owner) != FALSE) {
      return true;
    }
    Sleep(10);
  }
  return false;
}

extern "C" int32_t wox_windows_write_clipboard_text(uintptr_t owner, const char *text) {
  if (owner == 0 || text == nullptr) {
    return E_INVALIDARG;
  }
  const int wide_length = MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, nullptr, 0);
  if (wide_length <= 0 || static_cast<size_t>(wide_length) > SIZE_MAX / sizeof(wchar_t)) {
    return E_INVALIDARG;
  }
  const size_t byte_count = static_cast<size_t>(wide_length) * sizeof(wchar_t);
  HGLOBAL handle = GlobalAlloc(GMEM_MOVEABLE, byte_count);
  if (handle == nullptr) {
    return E_OUTOFMEMORY;
  }
  auto *memory = static_cast<wchar_t *>(GlobalLock(handle));
  if (memory == nullptr) {
    GlobalFree(handle);
    return last_error_result();
  }
  if (MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, text, -1, memory, wide_length) == 0) {
    GlobalUnlock(handle);
    GlobalFree(handle);
    return E_INVALIDARG;
  }
  GlobalUnlock(handle);

  if (!open_clipboard_with_retry(reinterpret_cast<HWND>(owner))) {
    GlobalFree(handle);
    return last_error_result();
  }
  if (EmptyClipboard() == FALSE) {
    const HRESULT result = last_error_result();
    CloseClipboard();
    GlobalFree(handle);
    return result;
  }
  if (SetClipboardData(CF_UNICODETEXT, handle) == nullptr) {
    const HRESULT result = last_error_result();
    CloseClipboard();
    GlobalFree(handle);
    return result;
  }
  CloseClipboard();
  return S_OK;
}

static void publish_png_clipboard_format(const uint8_t *png, uint32_t png_size) {
  if (png == nullptr || png_size == 0) {
    return;
  }
  const UINT png_format = RegisterClipboardFormatW(L"PNG");
  if (png_format == 0) {
    return;
  }
  HGLOBAL handle = GlobalAlloc(GMEM_MOVEABLE, png_size);
  if (handle == nullptr) {
    return;
  }
  void *memory = GlobalLock(handle);
  if (memory == nullptr) {
    GlobalFree(handle);
    return;
  }
  std::memcpy(memory, png, png_size);
  GlobalUnlock(handle);
  if (SetClipboardData(png_format, handle) == nullptr) {
    GlobalFree(handle);
  }
}

extern "C" int32_t wox_windows_write_clipboard_image(uintptr_t owner, const uint8_t *pixels, uint32_t width, uint32_t height, uint32_t row_stride, const uint8_t *png, uint32_t png_size) {
  if (owner == 0 || pixels == nullptr || width == 0 || height == 0 || width > UINT32_MAX / 4 || row_stride < width * 4) {
    return E_INVALIDARG;
  }
  const size_t output_stride = static_cast<size_t>(width) * 4;
  if (height > (SIZE_MAX - sizeof(BITMAPINFOHEADER)) / output_stride) {
    return E_INVALIDARG;
  }
  const size_t pixel_size = output_stride * height;
  const size_t allocation_size = sizeof(BITMAPINFOHEADER) + pixel_size;

  if (!open_clipboard_with_retry(reinterpret_cast<HWND>(owner))) {
    return last_error_result();
  }
  if (EmptyClipboard() == FALSE) {
    const HRESULT result = last_error_result();
    CloseClipboard();
    return result;
  }

  publish_png_clipboard_format(png, png_size);
  HGLOBAL dib_handle = GlobalAlloc(GMEM_MOVEABLE | GMEM_ZEROINIT, allocation_size);
  if (dib_handle == nullptr) {
    const HRESULT result = E_OUTOFMEMORY;
    CloseClipboard();
    return result;
  }
  auto *header = static_cast<BITMAPINFOHEADER *>(GlobalLock(dib_handle));
  if (header == nullptr) {
    const HRESULT result = last_error_result();
    GlobalFree(dib_handle);
    CloseClipboard();
    return result;
  }
  header->biSize = sizeof(BITMAPINFOHEADER);
  header->biWidth = static_cast<LONG>(width);
  header->biHeight = static_cast<LONG>(height);
  header->biPlanes = 1;
  header->biBitCount = 32;
  header->biCompression = BI_RGB;
  header->biSizeImage = static_cast<DWORD>(pixel_size);
  uint8_t *output = reinterpret_cast<uint8_t *>(header + 1);
  for (uint32_t y = 0; y < height; ++y) {
    const uint8_t *source_row = pixels + static_cast<size_t>(height - 1 - y) * row_stride;
    uint8_t *output_row = output + static_cast<size_t>(y) * output_stride;
    for (uint32_t x = 0; x < width; ++x) {
      output_row[x * 4] = source_row[x * 4 + 2];
      output_row[x * 4 + 1] = source_row[x * 4 + 1];
      output_row[x * 4 + 2] = source_row[x * 4];
      output_row[x * 4 + 3] = source_row[x * 4 + 3];
    }
  }
  GlobalUnlock(dib_handle);
  if (SetClipboardData(CF_DIB, dib_handle) == nullptr) {
    const HRESULT result = last_error_result();
    GlobalFree(dib_handle);
    CloseClipboard();
    return result;
  }
  CloseClipboard();
  return S_OK;
}
