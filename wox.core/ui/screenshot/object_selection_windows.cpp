//go:build windows

#include "object_selection_windows.h"
#include <windows.h>
#include <UIAutomation.h>
#include <wrl/client.h>
#include <algorithm>
#include <chrono>
#include <deque>
#include <vector>

extern "C" int32_t woxGoWindowsScreenshotCancelled(uintptr_t context);

using Microsoft::WRL::ComPtr;
using SelectorClock = std::chrono::steady_clock;
static constexpr size_t selector_node_limit = 4096;

// Expanded nodes retain only geometry. Unloaded nodes and partially decoded batches
// keep provider references until expansion finishes or the capture session ends.
struct SelectorNode {
  ComPtr<IUIAutomationElement> element;
  RECT frame = {};
  std::vector<size_t> children;
  ComPtr<IUIAutomationElementArray> batch;
  int next_child = 0;
  int child_count = 0;
  bool expanded = false;
};

// Cache only the current frozen window, bounding both geometry and native references.
struct WoxScreenshotObjectSelector {
  ComPtr<IUIAutomation2> automation;
  ComPtr<IUIAutomationCacheRequest> cache;
  ComPtr<IUIAutomationCondition> condition;
  uintptr_t window = 0;
  bool refined = false;
  std::deque<SelectorNode> nodes;
};

// Cancellation preserves completed cache work, so successive pointer events do not restart provider traversal.
static bool selector_active(SelectorClock::time_point deadline, uintptr_t cancellation) {
  return SelectorClock::now() < deadline && !woxGoWindowsScreenshotCancelled(cancellation);
}

// Set provider timeouts only before IPC; cached geometry needs no provider calls.
static bool selector_time_left(IUIAutomation2 *automation, SelectorClock::time_point deadline, uintptr_t cancellation) {
  auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline-SelectorClock::now()).count();
  if (remaining <= 0 || woxGoWindowsScreenshotCancelled(cancellation)) return false;
  DWORD timeout = static_cast<DWORD>(std::min<int64_t>(remaining, 50));
  return SUCCEEDED(automation->put_ConnectionTimeout(timeout)) && SUCCEEDED(automation->put_TransactionTimeout(timeout));
}

// Initialize once on the worker MTA, which owns all provider objects through destruction.
WoxScreenshotObjectSelector *wox_windows_screenshot_selector_create(void) {
  if (FAILED(CoInitializeEx(nullptr, COINIT_MULTITHREADED))) return nullptr;
  auto selector = new WoxScreenshotObjectSelector;
  if (FAILED(CoCreateInstance(CLSID_CUIAutomation8, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&selector->automation))) ||
      FAILED(selector->automation->CreateCacheRequest(&selector->cache)) ||
      FAILED(selector->cache->AddProperty(UIA_BoundingRectanglePropertyId)) ||
      FAILED(selector->cache->AddProperty(UIA_IsOffscreenPropertyId)) ||
      FAILED(selector->cache->put_TreeScope(TreeScope_Element)) ||
      FAILED(selector->automation->get_ControlViewCondition(&selector->condition))) {
    delete selector;
    CoUninitialize();
    return nullptr;
  }
  return selector;
}

// Reset before accepting geometry from a different or moved frozen window.
void wox_windows_screenshot_selector_reset(WoxScreenshotObjectSelector *selector) {
  if (selector == nullptr) return;
  selector->nodes.clear();
  selector->window = 0;
  selector->refined = false;
}

// Release native references before ending the worker's COM apartment.
void wox_windows_screenshot_selector_destroy(WoxScreenshotObjectSelector *selector) {
  if (selector == nullptr) return;
  delete selector;
  CoUninitialize();
}

// Load each sibling batch once, retaining decoding progress across cancellation.
// Publish children only when the batch is complete so interruption cannot change overlap precedence.
static bool selector_expand(WoxScreenshotObjectSelector *selector, size_t index,
                            SelectorClock::time_point deadline, uintptr_t cancellation) {
  SelectorNode &node = selector->nodes[index];
  if (node.expanded) return true;
  if (!node.batch) {
    if (!selector_time_left(selector->automation.Get(), deadline, cancellation)) return false;
    if (FAILED(node.element->FindAllBuildCache(TreeScope_Children, selector->condition.Get(), selector->cache.Get(), &node.batch))) return false;
    node.child_count = 0;
    if (node.batch && FAILED(node.batch->get_Length(&node.child_count))) {
      node.batch.Reset();
      return false;
    }
  }
  while (node.next_child < node.child_count && selector->nodes.size() < selector_node_limit) {
    if (!selector_active(deadline, cancellation)) return false;
    ComPtr<IUIAutomationElement> child;
    if (SUCCEEDED(node.batch->GetElement(node.next_child++, &child)) && child) {
      RECT frame = {};
      BOOL offscreen = FALSE;
      if (FAILED(child->get_CachedBoundingRectangle(&frame))) continue;
      child->get_CachedIsOffscreen(&offscreen);
      if (offscreen) continue;
      size_t child_index = selector->nodes.size();
      selector->nodes.emplace_back();
      selector->nodes.back().element = std::move(child);
      selector->nodes.back().frame = frame;
      node.children.push_back(child_index);
    }
  }
  node.expanded = true;
  node.batch.Reset();
  node.element.Reset();
  return true;
}

// Traverse only branches containing the physical desktop point. Cached branches avoid repeated UIA IPC.
int32_t wox_windows_screenshot_elements(WoxScreenshotObjectSelector *selector, uintptr_t window, int32_t x, int32_t y,
                                        uint32_t budget_ms, int32_t refinement, uintptr_t cancellation,
                                        WoxScreenshotElementRect *rects, int32_t capacity) {
  if (selector == nullptr || window == 0 || rects == nullptr || capacity <= 0 || budget_ms == 0) return 0;
  auto deadline = SelectorClock::now()+std::chrono::milliseconds(budget_ms);
  if (selector->window != window) wox_windows_screenshot_selector_reset(selector);
  if (refinement && !selector->refined && selector_active(deadline, cancellation)) {
    // Chromium/Electron can publish descendants after the first UIA request. Refresh
    // once after dwell, then reuse that tree instead of repeating IPC on every movement.
    selector->nodes.clear();
    selector->refined = true;
  }
  if (selector->nodes.empty()) {
    if (!selector_time_left(selector->automation.Get(), deadline, cancellation)) return 0;
    ComPtr<IUIAutomationElement> root;
    RECT frame = {};
    if (FAILED(selector->automation->ElementFromHandleBuildCache(reinterpret_cast<HWND>(window), selector->cache.Get(), &root)) ||
        !root || FAILED(root->get_CachedBoundingRectangle(&frame))) return 0;
    selector->nodes.emplace_back();
    selector->nodes.back().element = std::move(root);
    selector->nodes.back().frame = frame;
    selector->window = window;
  }
  struct PendingNode {
    size_t index;
    std::vector<RECT> ancestors;
    int depth;
  };
  std::vector<PendingNode> pending;
  pending.push_back({0, {}, 0});
  std::vector<RECT> best;
  int best_depth = -1;
  int64_t best_area = INT64_MAX;
  int visited = 0;
  POINT point = {x, y};
  while (!pending.empty() && visited++ < 512 && selector_active(deadline, cancellation)) {
    PendingNode current = std::move(pending.back()); pending.pop_back();
    SelectorNode &node = selector->nodes[current.index];
    RECT frame = node.frame;
    // Virtual grouping elements can have no rectangle while still exposing visible child controls.
    if (frame.right > frame.left && frame.bottom > frame.top && !PtInRect(&frame, point)) continue;
    if (frame.right-frame.left >= 2 && frame.bottom-frame.top >= 2) {
      if (current.ancestors.empty() || !EqualRect(&current.ancestors.back(), &frame)) current.ancestors.push_back(frame);
      int64_t area = static_cast<int64_t>(frame.right-frame.left)*(frame.bottom-frame.top);
      if (current.depth > best_depth || (current.depth == best_depth && area < best_area)) {
        best = current.ancestors; best_depth = current.depth; best_area = area;
      }
    }
    if (current.depth >= 64 || !selector_expand(selector, current.index, deadline, cancellation)) continue;
    for (auto child = node.children.rbegin(); child != node.children.rend() && visited+static_cast<int>(pending.size()) < 512; ++child) {
      // Reject cached siblings before queueing them so wide lists do not consume the traversal budget.
      RECT bounds = selector->nodes[*child].frame;
      if (bounds.right > bounds.left && bounds.bottom > bounds.top && !PtInRect(&bounds, point)) continue;
      pending.push_back({*child, current.ancestors, current.depth+1});
    }
  }
  std::reverse(best.begin(), best.end());
  int32_t count = 0;
  for (const RECT &frame : best) {
    if (count >= capacity) break;
    rects[count++] = {frame.left, frame.top, frame.right, frame.bottom};
  }
  return count;
}
