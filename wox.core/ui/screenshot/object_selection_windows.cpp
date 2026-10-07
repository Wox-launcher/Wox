//go:build windows

#include "object_selection_windows.h"
#include <windows.h>
#include <UIAutomation.h>
#include <wrl/client.h>
#include <algorithm>
#include <chrono>
#include <vector>

extern "C" int32_t woxGoWindowsScreenshotCancelled(uintptr_t context);

using Microsoft::WRL::ComPtr;
using SelectorClock = std::chrono::steady_clock;

struct SelectorNode {
  ComPtr<IUIAutomationElement> element;
  std::vector<RECT> ancestors;
  int depth;
};

// Provider calls stay on the dedicated Go worker's MTA and receive a bounded remaining timeout.
static bool selector_time_left(IUIAutomation2 *automation, SelectorClock::time_point deadline, uintptr_t cancellation) {
  auto remaining = std::chrono::duration_cast<std::chrono::milliseconds>(deadline-SelectorClock::now()).count();
  if (remaining <= 0 || woxGoWindowsScreenshotCancelled(cancellation)) return false;
  DWORD timeout = static_cast<DWORD>(std::min<int64_t>(remaining, 50));
  return SUCCEEDED(automation->put_ConnectionTimeout(timeout)) && SUCCEEDED(automation->put_TransactionTimeout(timeout));
}

// Traverse only branches containing the physical desktop point. Cache geometry in batches, never the whole application's tree.
int32_t wox_windows_screenshot_elements(uintptr_t window, int32_t x, int32_t y, uint32_t budget_ms, uintptr_t cancellation,
                                        WoxScreenshotElementRect *rects, int32_t capacity) {
  if (window == 0 || rects == nullptr || capacity <= 0 || budget_ms == 0) return 0;
  HRESULT initialized = CoInitializeEx(nullptr, COINIT_MULTITHREADED);
  if (FAILED(initialized)) return 0;
  int32_t count = 0;
  {
    auto deadline = SelectorClock::now()+std::chrono::milliseconds(budget_ms);
    ComPtr<IUIAutomation2> automation;
    if (SUCCEEDED(CoCreateInstance(CLSID_CUIAutomation8, nullptr, CLSCTX_INPROC_SERVER, IID_PPV_ARGS(&automation))) &&
        selector_time_left(automation.Get(), deadline, cancellation)) {
      ComPtr<IUIAutomationElement> root;
      ComPtr<IUIAutomationCacheRequest> cache;
      ComPtr<IUIAutomationCondition> condition;
      if (SUCCEEDED(automation->ElementFromHandle(reinterpret_cast<HWND>(window), &root)) && root &&
          selector_time_left(automation.Get(), deadline, cancellation) && SUCCEEDED(automation->CreateCacheRequest(&cache)) &&
          SUCCEEDED(cache->AddProperty(UIA_BoundingRectanglePropertyId)) && SUCCEEDED(cache->AddProperty(UIA_IsOffscreenPropertyId)) &&
          SUCCEEDED(cache->put_TreeScope(TreeScope_Element)) && SUCCEEDED(automation->get_ControlViewCondition(&condition))) {
        std::vector<SelectorNode> pending;
        pending.push_back({root, {}, 0});
        std::vector<RECT> best;
        int best_depth = -1;
        int64_t best_area = INT64_MAX;
        int visited = 0;
        POINT point = {x, y};
        while (!pending.empty() && visited++ < 512 && selector_time_left(automation.Get(), deadline, cancellation)) {
          SelectorNode node = std::move(pending.back()); pending.pop_back();
          RECT frame = {};
          BOOL offscreen = FALSE;
          HRESULT geometry = node.depth == 0 ? node.element->get_CurrentBoundingRectangle(&frame) : node.element->get_CachedBoundingRectangle(&frame);
          if (node.depth != 0) node.element->get_CachedIsOffscreen(&offscreen);
          if (offscreen || FAILED(geometry)) continue;
          // Virtual grouping elements can have no rectangle while still exposing visible child controls.
          if (frame.right > frame.left && frame.bottom > frame.top && !PtInRect(&frame, point)) continue;
          if (frame.right-frame.left >= 2 && frame.bottom-frame.top >= 2) {
            if (node.ancestors.empty() || !EqualRect(&node.ancestors.back(), &frame)) node.ancestors.push_back(frame);
            int64_t area = static_cast<int64_t>(frame.right-frame.left)*(frame.bottom-frame.top);
            if (node.depth > best_depth || (node.depth == best_depth && area < best_area)) {
              best = node.ancestors; best_depth = node.depth; best_area = area;
            }
          }
          if (node.depth >= 64 || !selector_time_left(automation.Get(), deadline, cancellation)) continue;
          ComPtr<IUIAutomationElementArray> children;
          if (FAILED(node.element->FindAllBuildCache(TreeScope_Children, condition.Get(), cache.Get(), &children)) || !children) continue;
          int length = 0;
          if (FAILED(children->get_Length(&length))) continue;
          // Bound decoding as well as provider IPC. Pending native references are released before returning to Go.
          for (int i = length-1; i >= 0 && visited+static_cast<int>(pending.size()) < 512 && SelectorClock::now() < deadline; --i) {
            ComPtr<IUIAutomationElement> child;
            if (SUCCEEDED(children->GetElement(i, &child)) && child) pending.push_back({child, node.ancestors, node.depth+1});
          }
        }
        std::reverse(best.begin(), best.end());
        for (const RECT &frame : best) {
          if (count >= capacity) break;
          rects[count++] = {frame.left, frame.top, frame.right, frame.bottom};
        }
      }
    }
  }
  CoUninitialize();
  return count;
}
