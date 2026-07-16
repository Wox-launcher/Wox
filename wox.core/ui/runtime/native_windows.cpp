//go:build windows

#define WIN32_LEAN_AND_MEAN
#include <windows.h>

#include <shobjidl.h>

#include <cstdlib>
#include <cstring>
#include <atomic>
#include <cstdarg>
#include <cstdio>
#include <memory>
#include <string>
#include <unordered_map>
#include <vector>

#include "native_windows.h"

extern "C" int32_t woxGoWindowsWebViewEscape(uintptr_t owner);

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

template <typename Function>
static Function webview_method(IUnknown *object, size_t index) {
  return reinterpret_cast<Function>((*reinterpret_cast<void ***>(object))[index]);
}

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
  IUnknown *controller = nullptr;
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
  std::wstring loaded_content_key;
};

struct WoxWindowsWebView;

class WoxEnvironmentCompletedHandler final : public IUnknown {
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

class WoxControllerCompletedHandler final : public IUnknown {
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

class WoxScriptCompletedHandler final : public IUnknown {
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

class WoxWebMessageHandler final : public IUnknown {
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

struct WoxWindowsWebView {
  using CreateEnvironment = HRESULT(STDAPICALLTYPE *)(const wchar_t *, const wchar_t *, IUnknown *, IUnknown *);

  explicit WoxWindowsWebView(HWND owner_window) : owner(owner_window) {}

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
      error = FAILED(result) ? result : E_FAIL;
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    environment = created_environment;
    webview_add_ref(environment);
    if (active != nullptr) {
      create_controller(active);
    }
    InvalidateRect(owner, nullptr, FALSE);
  }

  void create_controller(WoxWindowsWebViewSession *session) {
    if (environment == nullptr || session == nullptr || session->controller != nullptr || session->controller_pending || session->retired) {
      return;
    }
    session->controller_pending = true;
    auto *handler = new WoxControllerCompletedHandler(this, session);
    using CreateController = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, HWND, IUnknown *);
    HRESULT result = webview_method<CreateController>(environment, 3)(environment, owner, handler);
    webview_debug("create controller returned 0x%08X session=%p", static_cast<unsigned int>(result), session);
    handler->Release();
    if (FAILED(result)) {
      session->controller_pending = false;
      error = result;
      InvalidateRect(owner, nullptr, FALSE);
    }
  }

  void controller_completed(WoxWindowsWebViewSession *session, HRESULT result, IUnknown *created_controller) {
    webview_debug("controller completed 0x%08X session=%p controller=%p", static_cast<unsigned int>(result), session, created_controller);
    session->controller_pending = false;
    if (closing || session->retired) {
      if (created_controller != nullptr) {
        using Close = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
        webview_method<Close>(created_controller, 24)(created_controller);
      }
      return;
    }
    if (FAILED(result) || created_controller == nullptr) {
      error = FAILED(result) ? result : E_FAIL;
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    session->controller = created_controller;
    webview_add_ref(session->controller);
    using GetCore = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown **);
    result = webview_method<GetCore>(session->controller, 25)(session->controller, &session->core);
    webview_debug("get core returned 0x%08X core=%p", static_cast<unsigned int>(result), session->core);
    if (FAILED(result) || session->core == nullptr) {
      error = FAILED(result) ? result : E_FAIL;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    register_message_handler(session);
    if (!session->retired) {
      configure_script(session);
    }
  }

  void register_message_handler(WoxWindowsWebViewSession *session) {
    auto *handler = new WoxWebMessageHandler(this);
    using AddWebMessageHandler = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, IUnknown *, int64_t *);
    HRESULT result = webview_method<AddWebMessageHandler>(session->core, 34)(session->core, handler, &session->web_message_token);
    webview_debug("add web message handler returned 0x%08X token=%lld", static_cast<unsigned int>(result), static_cast<long long>(session->web_message_token));
    handler->Release();
    if (FAILED(result)) {
      error = result;
      dispose_session(session);
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    session->web_message_registered = true;
  }

  void web_message_received(IUnknown *args) {
    if (closing || args == nullptr) {
      return;
    }
    wchar_t *message = nullptr;
    using TryGetString = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, wchar_t **);
    HRESULT result = webview_method<TryGetString>(args, 5)(args, &message);
    webview_debug("web message received 0x%08X value=%ls", static_cast<unsigned int>(result), message != nullptr ? message : L"");
    if (SUCCEEDED(result) && message != nullptr && wcscmp(message, L"wox-unhandled-escape") == 0) {
      woxGoWindowsWebViewEscape(reinterpret_cast<uintptr_t>(owner));
    }
    if (message != nullptr) {
      CoTaskMemFree(message);
    }
  }

  void configure_script(WoxWindowsWebViewSession *session) {
    std::wstring script = L"(()=>{const c=" + javascript_string(session->signature) +
                          L";if(c){let s=document.getElementById('wox-webview-preview-style');if(!s){s=document.createElement('style');s.id='wox-webview-preview-style';(document.head||document.documentElement).appendChild(s)}s.textContent=c}"
                          L"if(window.__woxUnhandledEscapeInstalled__)return;window.__woxUnhandledEscapeInstalled__=true;document.addEventListener('keydown',e=>{if(e.key!=='Escape'||e.repeat)return;setTimeout(()=>{if(e.defaultPrevented||e.cancelBubble)return;window.chrome.webview.postMessage('wox-unhandled-escape')},0)},true)})()";
    session->script_pending = true;
    auto *handler = new WoxScriptCompletedHandler(this, session);
    using AddScript = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, const wchar_t *, IUnknown *);
    HRESULT result = webview_method<AddScript>(session->core, 27)(session->core, script.c_str(), handler);
    webview_debug("add startup script returned 0x%08X", static_cast<unsigned int>(result));
    handler->Release();
    if (FAILED(result)) {
      session->script_pending = false;
      error = result;
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
      error = result;
      InvalidateRect(owner, nullptr, FALSE);
      return;
    }
    session->script_ready = true;
    apply_session(session);
  }

  void apply_session(WoxWindowsWebViewSession *session) {
    if (session == nullptr || session->controller == nullptr || session->core == nullptr || !session->script_ready || session->retired) {
      return;
    }
    using PutBounds = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, RECT);
    using PutVisible = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, BOOL);
    HRESULT result = webview_method<PutBounds>(session->controller, 6)(session->controller, session->bounds);
    if (SUCCEEDED(result)) {
      result = webview_method<PutVisible>(session->controller, 4)(session->controller, session->visible ? TRUE : FALSE);
    }
    webview_debug("apply bounds/visibility returned 0x%08X visible=%d", static_cast<unsigned int>(result), session->visible ? 1 : 0);
    if (FAILED(result)) {
      error = result;
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
    } else {
      error = result;
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
    if (session->controller != nullptr) {
      using PutVisible = HRESULT(STDMETHODCALLTYPE *)(IUnknown *, BOOL);
      using Close = HRESULT(STDMETHODCALLTYPE *)(IUnknown *);
      webview_method<PutVisible>(session->controller, 4)(session->controller, FALSE);
      webview_method<Close>(session->controller, 24)(session->controller);
    }
    webview_release(session->core);
    webview_release(session->controller);
  }

  HRESULT show(const char *url, const char *html, const char *inject_css, bool cache_disabled, const char *cache_key, RECT bounds) {
    if (closing) {
      return E_FAIL;
    }
    if (FAILED(error)) {
      return error;
    }
    std::wstring wide_url = utf8_to_wide(url);
    std::wstring wide_html = utf8_to_wide(html);
    std::wstring signature = utf8_to_wide(inject_css);
    std::wstring content_key = (wide_html.empty() ? L"url|" + wide_url : L"html|" + wide_html);
    std::string key = cache_key != nullptr ? cache_key : "";
    bool use_cache = !cache_disabled && !key.empty();
    WoxWindowsWebViewSession *session = nullptr;
    if (use_cache) {
      auto cached = cache.find(key);
      if (cached != cache.end() && !cached->second->retired && cached->second->signature == signature) {
        session = cached->second;
      } else {
        if (cached != cache.end()) {
          dispose_session(cached->second);
        }
        session = new_session(key, signature, false);
        cache[key] = session;
      }
    } else if (active != nullptr && active->transient && !active->retired && active->signature == signature && active->content_key == content_key) {
      session = active;
    } else {
      session = new_session({}, signature, true);
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
    session->content_key = std::move(content_key);
    session->bounds = bounds;
    session->visible = true;
    if (environment != nullptr) {
      create_controller(session);
      apply_session(session);
    }
    return error;
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
    webview_release(environment);
  }

  WoxWindowsWebViewSession *new_session(std::string key, std::wstring signature, bool transient) {
    auto session = std::make_unique<WoxWindowsWebViewSession>();
    session->cache_key = std::move(key);
    session->signature = std::move(signature);
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
  HMODULE loader = nullptr;
  IUnknown *environment = nullptr;
  std::vector<std::unique_ptr<WoxWindowsWebViewSession>> sessions;
  std::unordered_map<std::string, WoxWindowsWebViewSession *> cache;
  WoxWindowsWebViewSession *active = nullptr;
  HRESULT error = S_OK;
  bool closing = false;
};

static HRESULT callback_query_interface(IUnknown *self, void **object) {
  if (object == nullptr) {
    return E_POINTER;
  }
  *object = self;
  webview_add_ref(self);
  return S_OK;
}

WoxEnvironmentCompletedHandler::WoxEnvironmentCompletedHandler(WoxWindowsWebView *owner) : owner_(owner) { owner_->retain(); }
HRESULT WoxEnvironmentCompletedHandler::QueryInterface(REFIID, void **object) { return callback_query_interface(this, object); }
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
HRESULT WoxControllerCompletedHandler::QueryInterface(REFIID, void **object) { return callback_query_interface(this, object); }
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
HRESULT WoxScriptCompletedHandler::QueryInterface(REFIID, void **object) { return callback_query_interface(this, object); }
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
HRESULT WoxWebMessageHandler::QueryInterface(REFIID, void **object) { return callback_query_interface(this, object); }
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

extern "C" int32_t wox_windows_webview_create(uintptr_t owner, WoxWindowsWebView **webview) {
  if (owner == 0 || webview == nullptr) {
    return E_INVALIDARG;
  }
  *webview = new WoxWindowsWebView(reinterpret_cast<HWND>(owner));
  HRESULT result = (*webview)->initialize();
  if (FAILED(result)) {
    (*webview)->release();
    *webview = nullptr;
  }
  return result;
}

extern "C" int32_t wox_windows_webview_show(WoxWindowsWebView *webview, const char *url, const char *html, const char *inject_css, int32_t cache_disabled, const char *cache_key, int32_t x, int32_t y, int32_t width, int32_t height) {
  if (webview == nullptr || url == nullptr || html == nullptr || inject_css == nullptr || cache_key == nullptr || width <= 0 || height <= 0) {
    return E_INVALIDARG;
  }
  RECT bounds = {x, y, x + width, y + height};
  return webview->show(url, html, inject_css, cache_disabled != 0, cache_key, bounds);
}

extern "C" int32_t wox_windows_webview_hide(WoxWindowsWebView *webview) {
  return webview != nullptr ? webview->hide() : E_INVALIDARG;
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
