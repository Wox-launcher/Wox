//go:build windows

#include "native_windows.h"

#include <UIAutomationCore.h>
#include <oleauto.h>
#include <windows.h>

#include <algorithm>
#include <atomic>
#include <cstdint>
#include <memory>
#include <mutex>
#include <string>
#include <unordered_map>
#include <utility>
#include <vector>

extern "C" int32_t woxGoWindowsAccessibilityAction(uintptr_t owner, uint64_t node_id, const char *action, const char *value);

namespace {

constexpr uint32_t state_enabled = 1u << 0;
constexpr uint32_t state_focusable = 1u << 1;
constexpr uint32_t state_focused = 1u << 2;
constexpr uint32_t state_selected = 1u << 3;
constexpr uint32_t state_checked = 1u << 4;
constexpr uint32_t state_expanded = 1u << 5;
constexpr uint32_t state_read_only = 1u << 6;
constexpr uint32_t state_protected = 1u << 7;
constexpr uint32_t state_hidden = 1u << 8;

constexpr uint32_t action_focus = 1u << 0;
constexpr uint32_t action_activate = 1u << 1;
constexpr uint32_t action_set_value = 1u << 2;
constexpr uint32_t action_toggle = 1u << 3;

struct Node {
  uint64_t id = 0;
  uint64_t parent_id = 0;
  std::vector<uint64_t> children;
  std::wstring automation_id;
  std::string role;
  std::wstring label;
  std::wstring description;
  std::wstring value;
  float x = 0;
  float y = 0;
  float width = 0;
  float height = 0;
  uint32_t state_flags = 0;
  uint32_t action_flags = 0;
  int32_t live_region = 0;
};

struct Snapshot {
  uint64_t generation = 0;
  std::unordered_map<uint64_t, Node> nodes;
  std::vector<uint64_t> roots;
};

struct TreeState {
  std::mutex mutex;
  Snapshot current;
  Snapshot pending;
  bool building = false;
};

std::mutex states_mutex;
std::unordered_map<HWND, std::shared_ptr<TreeState>> states;

std::wstring utf8_to_wide(const char *value) {
  if (value == nullptr || value[0] == '\0') {
    return {};
  }
  int length = MultiByteToWideChar(CP_UTF8, 0, value, -1, nullptr, 0);
  if (length <= 1) {
    return {};
  }
  std::wstring result(static_cast<size_t>(length), L'\0');
  MultiByteToWideChar(CP_UTF8, 0, value, -1, result.data(), length);
  result.resize(static_cast<size_t>(length - 1));
  return result;
}

std::string wide_to_utf8(const wchar_t *value) {
  if (value == nullptr || value[0] == L'\0') {
    return {};
  }
  int length = WideCharToMultiByte(CP_UTF8, 0, value, -1, nullptr, 0, nullptr, nullptr);
  if (length <= 1) {
    return {};
  }
  std::string result(static_cast<size_t>(length), '\0');
  WideCharToMultiByte(CP_UTF8, 0, value, -1, result.data(), length, nullptr, nullptr);
  result.resize(static_cast<size_t>(length - 1));
  return result;
}

std::shared_ptr<TreeState> state_for(HWND hwnd, bool create) {
  std::lock_guard<std::mutex> lock(states_mutex);
  auto found = states.find(hwnd);
  if (found != states.end()) {
    return found->second;
  }
  if (!create) {
    return {};
  }
  auto state = std::make_shared<TreeState>();
  states.emplace(hwnd, state);
  return state;
}

CONTROLTYPEID control_type(const std::string &role) {
  if (role == "window" || role == "dialog") return UIA_WindowControlTypeId;
  if (role == "group") return UIA_GroupControlTypeId;
  if (role == "text") return UIA_TextControlTypeId;
  if (role == "heading") return UIA_HeaderControlTypeId;
  if (role == "button") return UIA_ButtonControlTypeId;
  if (role == "text_field") return UIA_EditControlTypeId;
  if (role == "checkbox") return UIA_CheckBoxControlTypeId;
  if (role == "radio_button") return UIA_RadioButtonControlTypeId;
  if (role == "list") return UIA_ListControlTypeId;
  if (role == "list_item") return UIA_ListItemControlTypeId;
  if (role == "image") return UIA_ImageControlTypeId;
  if (role == "progress_bar") return UIA_ProgressBarControlTypeId;
  if (role == "link") return UIA_HyperlinkControlTypeId;
  if (role == "menu") return UIA_MenuControlTypeId;
  if (role == "menu_item") return UIA_MenuItemControlTypeId;
  if (role == "web_view") return UIA_DocumentControlTypeId;
  return UIA_PaneControlTypeId;
}

class Provider final : public IRawElementProviderSimple,
                       public IRawElementProviderFragment,
                       public IRawElementProviderFragmentRoot,
                       public IInvokeProvider,
                       public IValueProvider,
                       public IToggleProvider {
 public:
  Provider(HWND hwnd, std::shared_ptr<TreeState> state, uint64_t node_id)
      : hwnd_(hwnd), state_(std::move(state)), node_id_(node_id) {}

  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID iid, void **object) override {
    if (object == nullptr) return E_POINTER;
    *object = nullptr;
    if (iid == __uuidof(IUnknown) || iid == __uuidof(IRawElementProviderSimple)) {
      *object = static_cast<IRawElementProviderSimple *>(this);
    } else if (iid == __uuidof(IRawElementProviderFragment)) {
      *object = static_cast<IRawElementProviderFragment *>(this);
    } else if (iid == __uuidof(IRawElementProviderFragmentRoot) && node_id_ == 0) {
      *object = static_cast<IRawElementProviderFragmentRoot *>(this);
    } else if (iid == __uuidof(IInvokeProvider) && has_action(action_activate)) {
      *object = static_cast<IInvokeProvider *>(this);
    } else if (iid == __uuidof(IValueProvider) && has_action(action_set_value)) {
      *object = static_cast<IValueProvider *>(this);
    } else if (iid == __uuidof(IToggleProvider) && has_action(action_toggle)) {
      *object = static_cast<IToggleProvider *>(this);
    } else {
      return E_NOINTERFACE;
    }
    AddRef();
    return S_OK;
  }

  ULONG STDMETHODCALLTYPE AddRef() override { return ++references_; }

  ULONG STDMETHODCALLTYPE Release() override {
    ULONG remaining = --references_;
    if (remaining == 0) delete this;
    return remaining;
  }

  HRESULT STDMETHODCALLTYPE get_ProviderOptions(ProviderOptions *options) override {
    if (options == nullptr) return E_POINTER;
    *options = static_cast<ProviderOptions>(ProviderOptions_ServerSideProvider | ProviderOptions_UseComThreading);
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GetPatternProvider(PATTERNID pattern_id, IUnknown **provider) override {
    if (provider == nullptr) return E_POINTER;
    *provider = nullptr;
    if (pattern_id == UIA_InvokePatternId) return QueryInterface(__uuidof(IInvokeProvider), reinterpret_cast<void **>(provider));
    if (pattern_id == UIA_ValuePatternId) return QueryInterface(__uuidof(IValueProvider), reinterpret_cast<void **>(provider));
    if (pattern_id == UIA_TogglePatternId) return QueryInterface(__uuidof(IToggleProvider), reinterpret_cast<void **>(provider));
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GetPropertyValue(PROPERTYID property_id, VARIANT *result) override {
    if (result == nullptr) return E_POINTER;
    VariantInit(result);
    Node node;
    if (!node_copy(node)) return UIA_E_ELEMENTNOTAVAILABLE;
    if (property_id == UIA_ControlTypePropertyId) {
      result->vt = VT_I4;
      result->lVal = control_type(node.role);
    } else if (property_id == UIA_NamePropertyId) {
      result->vt = VT_BSTR;
      result->bstrVal = SysAllocString(node.label.c_str());
    } else if (property_id == UIA_AutomationIdPropertyId) {
      result->vt = VT_BSTR;
      result->bstrVal = SysAllocString(node.automation_id.c_str());
    } else if (property_id == UIA_HelpTextPropertyId) {
      result->vt = VT_BSTR;
      result->bstrVal = SysAllocString(node.description.c_str());
    } else if (property_id == UIA_IsEnabledPropertyId) {
      result->vt = VT_BOOL;
      result->boolVal = (node.state_flags & state_enabled) != 0 ? VARIANT_TRUE : VARIANT_FALSE;
    } else if (property_id == UIA_IsKeyboardFocusablePropertyId) {
      result->vt = VT_BOOL;
      result->boolVal = (node.state_flags & state_focusable) != 0 ? VARIANT_TRUE : VARIANT_FALSE;
    } else if (property_id == UIA_HasKeyboardFocusPropertyId) {
      result->vt = VT_BOOL;
      result->boolVal = (node.state_flags & state_focused) != 0 ? VARIANT_TRUE : VARIANT_FALSE;
    } else if (property_id == UIA_IsPasswordPropertyId) {
      result->vt = VT_BOOL;
      result->boolVal = (node.state_flags & state_protected) != 0 ? VARIANT_TRUE : VARIANT_FALSE;
    } else if (property_id == UIA_IsOffscreenPropertyId) {
      result->vt = VT_BOOL;
      result->boolVal = (node.state_flags & state_hidden) != 0 ? VARIANT_TRUE : VARIANT_FALSE;
    } else if (property_id == UIA_LiveSettingPropertyId) {
      result->vt = VT_I4;
      result->lVal = node.live_region == 2 ? Assertive : (node.live_region == 1 ? Polite : Off);
    }
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE get_HostRawElementProvider(IRawElementProviderSimple **provider) override {
    if (provider == nullptr) return E_POINTER;
    *provider = nullptr;
    return node_id_ == 0 ? UiaHostProviderFromHwnd(hwnd_, provider) : S_OK;
  }

  HRESULT STDMETHODCALLTYPE Navigate(NavigateDirection direction, IRawElementProviderFragment **provider) override {
    if (provider == nullptr) return E_POINTER;
    *provider = nullptr;
    uint64_t target = 0;
    {
      std::lock_guard<std::mutex> lock(state_->mutex);
      const Snapshot &snapshot = state_->current;
      if (direction == NavigateDirection_Parent) {
        if (node_id_ == 0) return S_OK;
        auto found = snapshot.nodes.find(node_id_);
        if (found == snapshot.nodes.end()) return UIA_E_ELEMENTNOTAVAILABLE;
        target = found->second.parent_id;
      } else {
        std::vector<uint64_t> siblings;
        if (node_id_ == 0) {
          siblings = snapshot.roots;
        } else {
          auto found = snapshot.nodes.find(node_id_);
          if (found == snapshot.nodes.end()) return UIA_E_ELEMENTNOTAVAILABLE;
          if (direction == NavigateDirection_FirstChild || direction == NavigateDirection_LastChild) {
            siblings = found->second.children;
          } else if (found->second.parent_id == 0) {
            siblings = snapshot.roots;
          } else {
            auto parent = snapshot.nodes.find(found->second.parent_id);
            if (parent != snapshot.nodes.end()) siblings = parent->second.children;
          }
        }
        if (direction == NavigateDirection_FirstChild && !siblings.empty()) target = siblings.front();
        if (direction == NavigateDirection_LastChild && !siblings.empty()) target = siblings.back();
        if ((direction == NavigateDirection_NextSibling || direction == NavigateDirection_PreviousSibling) && node_id_ != 0) {
          auto current = std::find(siblings.begin(), siblings.end(), node_id_);
          if (current != siblings.end()) {
            if (direction == NavigateDirection_NextSibling && ++current != siblings.end()) target = *current;
            if (direction == NavigateDirection_PreviousSibling && current != siblings.begin()) target = *(current - 1);
          }
        }
      }
    }
    if (direction == NavigateDirection_Parent && node_id_ != 0 && target == 0) {
      *provider = new Provider(hwnd_, state_, 0);
    } else if (target != 0) {
      *provider = new Provider(hwnd_, state_, target);
    }
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GetRuntimeId(SAFEARRAY **runtime_id) override {
    if (runtime_id == nullptr) return E_POINTER;
    int values[] = {UiaAppendRuntimeId, static_cast<int>(node_id_ & 0x7fffffff), static_cast<int>((node_id_ >> 31) & 0x7fffffff)};
    SAFEARRAY *array = SafeArrayCreateVector(VT_I4, 0, 3);
    if (array == nullptr) return E_OUTOFMEMORY;
    for (LONG index = 0; index < 3; ++index) SafeArrayPutElement(array, &index, &values[index]);
    *runtime_id = array;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE get_BoundingRectangle(UiaRect *rectangle) override {
    if (rectangle == nullptr) return E_POINTER;
    Node node;
    if (!node_copy(node)) return UIA_E_ELEMENTNOTAVAILABLE;
    *rectangle = screen_bounds(node);
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GetEmbeddedFragmentRoots(SAFEARRAY **roots) override {
    if (roots == nullptr) return E_POINTER;
    *roots = nullptr;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE SetFocus() override {
    return call_action("focus", "") ? S_OK : UIA_E_NOTSUPPORTED;
  }

  HRESULT STDMETHODCALLTYPE get_FragmentRoot(IRawElementProviderFragmentRoot **root) override {
    if (root == nullptr) return E_POINTER;
    *root = new Provider(hwnd_, state_, 0);
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE ElementProviderFromPoint(double x, double y, IRawElementProviderFragment **provider) override {
    if (provider == nullptr) return E_POINTER;
    *provider = nullptr;
    uint64_t hit = 0;
    {
      std::lock_guard<std::mutex> lock(state_->mutex);
      for (const auto &entry : state_->current.nodes) {
        const Node &node = entry.second;
        UiaRect bounds = screen_bounds(node);
        if ((node.state_flags & state_hidden) == 0 && x >= bounds.left && y >= bounds.top && x <= bounds.left + bounds.width && y <= bounds.top + bounds.height) {
          hit = node.id;
        }
      }
    }
    *provider = new Provider(hwnd_, state_, hit);
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE GetFocus(IRawElementProviderFragment **provider) override {
    if (provider == nullptr) return E_POINTER;
    *provider = nullptr;
    uint64_t focused = 0;
    {
      std::lock_guard<std::mutex> lock(state_->mutex);
      for (const auto &entry : state_->current.nodes) {
        if ((entry.second.state_flags & state_focused) != 0) {
          focused = entry.first;
          break;
        }
      }
    }
    *provider = new Provider(hwnd_, state_, focused);
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Invoke() override {
    return call_action("activate", "") ? S_OK : UIA_E_NOTSUPPORTED;
  }

  HRESULT STDMETHODCALLTYPE SetValue(LPCWSTR value) override {
    return call_action("set_value", wide_to_utf8(value)) ? S_OK : UIA_E_NOTSUPPORTED;
  }

  HRESULT STDMETHODCALLTYPE get_Value(BSTR *value) override {
    if (value == nullptr) return E_POINTER;
    Node node;
    if (!node_copy(node)) return UIA_E_ELEMENTNOTAVAILABLE;
    *value = SysAllocString((node.state_flags & state_protected) != 0 ? L"" : node.value.c_str());
    return *value != nullptr ? S_OK : E_OUTOFMEMORY;
  }

  HRESULT STDMETHODCALLTYPE get_IsReadOnly(BOOL *read_only) override {
    if (read_only == nullptr) return E_POINTER;
    Node node;
    if (!node_copy(node)) return UIA_E_ELEMENTNOTAVAILABLE;
    *read_only = (node.state_flags & state_read_only) != 0 ? TRUE : FALSE;
    return S_OK;
  }

  HRESULT STDMETHODCALLTYPE Toggle() override {
    return call_action("toggle", "") ? S_OK : UIA_E_NOTSUPPORTED;
  }

  HRESULT STDMETHODCALLTYPE get_ToggleState(ToggleState *toggle_state) override {
    if (toggle_state == nullptr) return E_POINTER;
    Node node;
    if (!node_copy(node)) return UIA_E_ELEMENTNOTAVAILABLE;
    *toggle_state = (node.state_flags & state_checked) != 0 ? ToggleState_On : ToggleState_Off;
    return S_OK;
  }

 private:
  ~Provider() = default;

  bool node_copy(Node &node) const {
    if (node_id_ == 0) {
      node.id = 0;
      node.role = "window";
      node.label = L"Wox";
      node.state_flags = state_enabled | state_focusable;
      std::lock_guard<std::mutex> lock(state_->mutex);
      node.children = state_->current.roots;
      return true;
    }
    std::lock_guard<std::mutex> lock(state_->mutex);
    auto found = state_->current.nodes.find(node_id_);
    if (found == state_->current.nodes.end()) return false;
    node = found->second;
    return true;
  }

  bool has_action(uint32_t flag) const {
    Node node;
    return node_copy(node) && (node.action_flags & flag) != 0;
  }

  bool call_action(const char *action, const std::string &value) const {
    Node node;
    if (!node_copy(node) || node_id_ == 0) return false;
    return woxGoWindowsAccessibilityAction(reinterpret_cast<uintptr_t>(hwnd_), node_id_, action, value.c_str()) != 0;
  }

  UiaRect screen_bounds(const Node &node) const {
    RECT client{};
    GetClientRect(hwnd_, &client);
    POINT origin{0, 0};
    ClientToScreen(hwnd_, &origin);
    UINT dpi = GetDpiForWindow(hwnd_);
    double scale = dpi == 0 ? 1.0 : static_cast<double>(dpi) / 96.0;
    if (node.id == 0) {
      return UiaRect{static_cast<double>(origin.x), static_cast<double>(origin.y), static_cast<double>(client.right - client.left), static_cast<double>(client.bottom - client.top)};
    }
    return UiaRect{origin.x + node.x * scale, origin.y + node.y * scale, node.width * scale, node.height * scale};
  }

  std::atomic<ULONG> references_{1};
  HWND hwnd_;
  std::shared_ptr<TreeState> state_;
  uint64_t node_id_;
};

}  // namespace

extern "C" int32_t wox_windows_accessibility_begin(uintptr_t owner, uint64_t generation) {
  HWND hwnd = reinterpret_cast<HWND>(owner);
  auto state = state_for(hwnd, true);
  std::lock_guard<std::mutex> lock(state->mutex);
  state->pending = Snapshot{};
  state->pending.generation = generation;
  state->building = true;
  return 0;
}

extern "C" int32_t wox_windows_accessibility_add_node(uintptr_t owner, uint64_t id, uint64_t parent_id, const uint64_t *children, int32_t child_count, const char *automation_id, const char *role, const char *label, const char *description, const char *value, float x, float y, float width, float height, uint32_t state_flags, uint32_t action_flags, int32_t live_region) {
  auto state = state_for(reinterpret_cast<HWND>(owner), false);
  if (!state || id == 0 || child_count < 0) return -1;
  Node node;
  node.id = id;
  node.parent_id = parent_id;
  if (children != nullptr) node.children.assign(children, children + child_count);
  node.automation_id = utf8_to_wide(automation_id);
  node.role = role != nullptr ? role : "group";
  node.label = utf8_to_wide(label);
  node.description = utf8_to_wide(description);
  node.value = utf8_to_wide(value);
  node.x = x;
  node.y = y;
  node.width = width;
  node.height = height;
  node.state_flags = state_flags;
  node.action_flags = action_flags;
  node.live_region = live_region;
  std::lock_guard<std::mutex> lock(state->mutex);
  if (!state->building) return -1;
  state->pending.nodes[id] = std::move(node);
  if (parent_id == 0) state->pending.roots.push_back(id);
  return 0;
}

extern "C" int32_t wox_windows_accessibility_end(uintptr_t owner) {
  HWND hwnd = reinterpret_cast<HWND>(owner);
  auto state = state_for(hwnd, false);
  if (!state) return -1;
  uint64_t focused = 0;
  {
    std::lock_guard<std::mutex> lock(state->mutex);
    if (!state->building) return -1;
    state->current = std::move(state->pending);
    state->building = false;
    for (const auto &entry : state->current.nodes) {
      if ((entry.second.state_flags & state_focused) != 0) {
        focused = entry.first;
        break;
      }
    }
  }
  Provider *root = new Provider(hwnd, state, 0);
  UiaRaiseStructureChangedEvent(root, StructureChangeType_ChildrenInvalidated, nullptr, 0);
  root->Release();
  if (focused != 0) {
    Provider *focus = new Provider(hwnd, state, focused);
    UiaRaiseAutomationEvent(focus, UIA_AutomationFocusChangedEventId);
    focus->Release();
  }
  return 0;
}

extern "C" uintptr_t wox_windows_accessibility_get_object(uintptr_t owner, uintptr_t wparam, uintptr_t lparam) {
  HWND hwnd = reinterpret_cast<HWND>(owner);
  if (static_cast<LONG>(lparam) != UiaRootObjectId) return 0;
  auto state = state_for(hwnd, false);
  if (!state) return 0;
  Provider *root = new Provider(hwnd, state, 0);
  LRESULT result = UiaReturnRawElementProvider(hwnd, static_cast<WPARAM>(wparam), static_cast<LPARAM>(lparam), root);
  root->Release();
  return static_cast<uintptr_t>(result);
}

extern "C" void wox_windows_accessibility_remove(uintptr_t owner) {
  std::lock_guard<std::mutex> lock(states_mutex);
  states.erase(reinterpret_cast<HWND>(owner));
}
