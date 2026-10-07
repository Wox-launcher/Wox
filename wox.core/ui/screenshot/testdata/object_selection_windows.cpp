#include "../object_selection_windows.cpp"
#include <cassert>

static int cancellation_checks = 0;
static int cancel_after = INT_MAX;

extern "C" int32_t woxGoWindowsScreenshotCancelled(uintptr_t) {
  return ++cancellation_checks > cancel_after;
}

// A cached empty batch exercises interrupted decoding without COM providers or visible windows.
class EmptyBatch final : public IUIAutomationElementArray {
  ULONG references = 1;
  bool *destroyed;
public:
  explicit EmptyBatch(bool *destroyed) : destroyed(destroyed) {}
  HRESULT STDMETHODCALLTYPE QueryInterface(REFIID, void **) override { return E_NOINTERFACE; }
  ULONG STDMETHODCALLTYPE AddRef() override { return ++references; }
  ULONG STDMETHODCALLTYPE Release() override {
    ULONG remaining = --references;
    if (!remaining) { *destroyed = true; delete this; }
    return remaining;
  }
  HRESULT STDMETHODCALLTYPE get_Length(int *length) override { *length = 100; return S_OK; }
  HRESULT STDMETHODCALLTYPE GetElement(int, IUIAutomationElement **element) override { *element = nullptr; return S_OK; }
};

// Cached geometry must resolve wide lists locally, including negative desktop origins and pixel-sized targets.
static void test_cached_geometry() {
  WoxScreenshotObjectSelector selector;
  selector.window = 1;
  selector.nodes.emplace_back();
  selector.nodes[0].frame = {-1600, -800, 1600, 1600};
  selector.nodes[0].expanded = true;
  for (int i = 0; i < 1000; ++i) {
    selector.nodes.emplace_back();
    selector.nodes.back().frame = {-1200, -700+i*2, -1000, -698+i*2};
    selector.nodes.back().expanded = true;
    selector.nodes[0].children.push_back(selector.nodes.size()-1);
  }
  WoxScreenshotElementRect rects[48];
  // No automation object exists: these repeated hits would fail if they attempted provider IPC.
  for (int i : {10, 600, 999, 600}) {
    int count = wox_windows_screenshot_elements(&selector, 1, -1100, -699+i*2, 168, 0, 0, rects, 48);
    assert(count == 2 && rects[0].left == -1200 && rects[0].top == -700+i*2);
    assert(rects[1].left == -1600);
  }
  assert(wox_windows_screenshot_elements(&selector, 1, 1500, 1500, 168, 0, 0, rects, 48) == 1);
  assert(wox_windows_screenshot_elements(&selector, 1, 2000, 1500, 168, 0, 0, rects, 48) == 0);
  selector.refined = true;
  assert(wox_windows_screenshot_elements(&selector, 1, -1100, -699, 168, 1, 0, rects, 48) == 2);
  cancel_after = cancellation_checks;
  assert(wox_windows_screenshot_elements(&selector, 1, -1100, -699, 168, 0, 0, rects, 48) == 0);
  cancel_after = INT_MAX;
  wox_windows_screenshot_selector_reset(&selector);
  assert(selector.nodes.empty() && selector.window == 0 && !selector.refined);
}

// Decoding resumes from the retained batch and releases it on completion or invalidation.
static void test_cancelled_batch() {
  WoxScreenshotObjectSelector selector;
  selector.nodes.emplace_back();
  bool destroyed = false;
  selector.nodes[0].batch.Attach(new EmptyBatch(&destroyed));
  selector.nodes[0].child_count = 100;
  cancel_after = cancellation_checks+3;
  auto deadline = SelectorClock::now()+std::chrono::seconds(1);
  assert(!selector_expand(&selector, 0, deadline, 0));
  assert(selector.nodes[0].next_child == 3 && !selector.nodes[0].expanded && !destroyed);
  cancel_after = INT_MAX;
  assert(selector_expand(&selector, 0, deadline, 0));
  assert(selector.nodes[0].next_child == 100 && selector.nodes[0].expanded && destroyed);
  destroyed = false;
  selector.nodes[0].batch.Attach(new EmptyBatch(&destroyed));
  wox_windows_screenshot_selector_reset(&selector);
  assert(destroyed && selector.nodes.empty());
}

// Reaching the node cap releases the remaining sibling batch instead of growing the capture cache.
static void test_cache_limit() {
  WoxScreenshotObjectSelector selector;
  selector.nodes.resize(selector_node_limit);
  bool destroyed = false;
  selector.nodes[0].batch.Attach(new EmptyBatch(&destroyed));
  selector.nodes[0].child_count = 100;
  assert(selector_expand(&selector, 0, SelectorClock::now()+std::chrono::seconds(1), 0));
  assert(selector.nodes.size() == selector_node_limit && destroyed && selector.nodes[0].expanded);
}

int main() {
  test_cached_geometry();
  test_cancelled_batch();
  test_cache_limit();
  return 0;
}
