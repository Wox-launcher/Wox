#ifndef RUNNER_FLUTTER_WINDOW_H_
#define RUNNER_FLUTTER_WINDOW_H_

#include <flutter/dart_project.h>
#include <flutter/flutter_view_controller.h>
#include <flutter/method_channel.h>
#include <flutter/standard_method_codec.h>

#include <memory>
#include <optional>
#include <string>
#include <unordered_set>
#include <vector>
#include <uiautomation.h>

#include "win32_window.h"

// A window that does nothing but host a Flutter view.
class FlutterWindow : public Win32Window
{
public:
  // Creates a new FlutterWindow hosting a Flutter view running |project|.
  explicit FlutterWindow(const flutter::DartProject &project);
  virtual ~FlutterWindow();

  // Log message to console and Flutter
  void Log(const std::string &message);

protected:
  // Win32Window:
  bool OnCreate() override;
  void OnDestroy() override;
  LRESULT MessageHandler(HWND window, UINT const message, WPARAM const wparam,
                         LPARAM const lparam) noexcept override;

private:
  // The project to run.
  flutter::DartProject project_;

  // The Flutter instance hosted by this window.
  std::unique_ptr<flutter::FlutterViewController> flutter_controller_;

  // Window manager method channel
  std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> window_manager_channel_;

  // Original window procedure
  WNDPROC original_window_proc_;

  // Original child window procedure for the Flutter view hwnd.
  WNDPROC original_child_window_proc_ = nullptr;

  // Flutter view child window handle.
  HWND child_window_ = nullptr;

  // Previous active window handle
  HWND previous_active_window_;

  // Only restore the saved foreground window when Wox has stayed focused since
  // the last show/focus request.
  bool restore_previous_window_on_hide_ = false;

  // Guard transient WM_ACTIVATE/WA_INACTIVE blur events between show() and focus().
  // show() sets this to true; focus() and hide() clear it.
  bool blur_guard_active_ = false;

  // Extra blur grace period (GetTickCount64 deadline) after show/focus to absorb
  // short-lived foreground steals from other apps. see issue #4346
  ULONGLONG blur_guard_until_tick_ = 0;

  struct ScreenshotPresentationState
  {
    bool active = false;
    bool prepared = false;
    double workspace_scale = 1.0;
    RECT native_workspace_bounds{0, 0, 0, 0};
  } screenshot_presentation_state_;

  struct ScrollingCaptureOverlayState
  {
    bool active = false;
    HWND overlay_window = nullptr;
    HHOOK mouse_hook = nullptr;
    RECT selection_bounds{0, 0, 0, 0};
  } scrolling_capture_overlay_state_;

  struct ScreenshotSelectionOverlayState
  {
    bool active = false;
    bool dragging = false;
    bool completed = false;
    bool dim_region_dirty = false;
    bool dim_region_update_posted = false;
    // The input window is also the dimming surface. A low-level mouse hook drives the fast drag path
    // because the layered full-screen HWND can receive its first button messages late while DWM is
    // still presenting the mask; the HWND input handlers remain as a fallback when the hook is unavailable.
    HWND input_window = nullptr;
    std::vector<HWND> border_windows;
    HHOOK mouse_hook = nullptr;
    POINT drag_start{0, 0};
    RECT workspace_bounds{0, 0, 0, 0};
    RECT selection_bounds{0, 0, 0, 0};
    bool has_pending_hover_selection = false;
    RECT pending_hover_selection_bounds{0, 0, 0, 0};
    bool has_hover_selection = false;
    RECT hover_selection_bounds{0, 0, 0, 0};
    std::string hover_selection_source;
    HWND hover_selection_root_window = nullptr;
    std::vector<RECT> hover_candidate_bounds;
    HWND hover_candidate_root_window = nullptr;
    bool has_last_hover_probe_point = false;
    POINT last_hover_probe_point{0, 0};
    ULONGLONG last_hover_probe_tick = 0;
    bool has_pending_hover_move = false;
    POINT pending_hover_move_point{0, 0};
    bool hover_move_message_posted = false;
    bool has_pending_hover_probe = false;
    POINT pending_hover_probe_point{0, 0};
    HWND pending_hover_probe_root_window = nullptr;
    ULONGLONG hover_probe_revision = 0;
    ULONGLONG pending_hover_probe_revision = 0;
    bool hover_probe_timer_active = false;
    bool hover_display_sized_uia_rejected = false;
    ULONGLONG last_hover_probe_slow_tick = 0;
    std::string last_hover_debug_signature;
    ULONGLONG last_hover_debug_tick = 0;
    ULONGLONG started_tick = 0;
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> pending_result;
  } screenshot_selection_overlay_state_;

  enum class ScreenshotImagePayloadMode
  {
    kNone,
    kBase64,
    kFilePath,
  };

  struct DisplaySnapshotPayloadAsyncResult
  {
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result;
    bool success = false;
    flutter::EncodableList snapshots;
    std::string error;
    size_t display_count = 0;
    int payload_count = 0;
    ScreenshotImagePayloadMode payload_mode = ScreenshotImagePayloadMode::kNone;
    ULONGLONG elapsed_ms = 0;
  };

  struct CachedDisplayCapture
  {
    std::wstring display_id;
    RECT monitor_bounds{0, 0, 0, 0};
    double scale = 1.0;
    int rotation = 0;
    HBITMAP bitmap = nullptr;
  };

  std::vector<CachedDisplayCapture> cached_display_captures_;
  IUIAutomation *screenshot_uia_automation_ = nullptr;

  // Save/restore the previously focused window (Windows focus rules require explicit restore)
  void SavePreviousActiveWindow(HWND selfHwnd);
  void RestorePreviousActiveWindow(HWND selfHwnd);
  HWND NormalizeToRootWindow(HWND hwnd) const;
  bool ShouldSuppressBlurForActivatedWindow(HWND selfHwnd, HWND activatedHwnd);

  // Get the DPI scaling factor for the window
  float GetDpiScale(HWND hwnd);

  // Sync the hosted Flutter child window with the root client area.
  void SyncFlutterChildWindowToClientArea(HWND hwnd, const char *source, bool engine_handled);
  void FocusFlutterViewOrRoot(HWND hwnd);

  // Helpers for logging native geometry.
  std::string RectToString(const RECT &rect) const;
  RECT GetWindowRectSafe(HWND hwnd) const;
  static void ReleaseDisplayCaptures(std::vector<CachedDisplayCapture> *captures);
  void ClearCachedDisplayCaptures();
  bool CaptureDisplaySnapshots(std::vector<CachedDisplayCapture> *captures_out, std::string *error_out, const std::optional<RECT> &logical_selection = std::nullopt);
  bool BuildDisplaySnapshotPayloads(const std::vector<CachedDisplayCapture> &captures, ScreenshotImagePayloadMode payload_mode, flutter::EncodableList *snapshots_out, std::string *error_out);
  static bool BuildDisplaySnapshotPayloadsCore(const std::vector<CachedDisplayCapture> &captures, ScreenshotImagePayloadMode payload_mode, flutter::EncodableList *snapshots_out, std::string *error_out, int *payload_count_out, ULONGLONG *elapsed_ms_out);
  static const char *ScreenshotImagePayloadModeName(ScreenshotImagePayloadMode payload_mode);
  bool CloneDisplayCaptures(const std::vector<CachedDisplayCapture> &captures, std::vector<CachedDisplayCapture> *captures_out, std::string *error_out);
  void BuildDisplaySnapshotPayloadsAsync(std::vector<CachedDisplayCapture> captures, std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);
  void CompleteDisplaySnapshotPayloadAsyncResult(DisplaySnapshotPayloadAsyncResult *payload_result);
  const CachedDisplayCapture *FindCachedDisplayCapture(const std::string &display_id) const;
  bool CachedDisplayCapturesMatch(const std::vector<std::string> &display_ids) const;
  void PrepareCaptureWorkspace(HWND hwnd, const RECT &native_workspace_bounds);
  void RevealPreparedCaptureWorkspace(HWND hwnd);
  flutter::EncodableMap BuildCaptureWorkspaceResponse(const RECT &native_workspace_bounds) const;
  bool BeginScreenshotSelectionOverlay(HWND hwnd, const RECT &workspace_bounds, std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result, std::string *error_out);
  void LayoutScreenshotSelectionOverlay();
  void UpdateScreenshotSelectionHover(const POINT &point);
  void UpdateScreenshotSelectionHoverFromHook(const POINT &point);
  void ScheduleScreenshotHoverProbe(const POINT &point, HWND root_window);
  void CancelScreenshotHoverProbeTimer();
  void HandleScreenshotHoverProbeTimer();
  void EmitScreenshotSelectionDisplayHint(const POINT &point);
  void ApplyScreenshotHoverSelection(const POINT &point, bool has_hover_selection, const RECT &hover_selection, const std::string &hover_source, HWND root_window, const std::vector<RECT> *candidate_bounds, bool update_candidate_bounds, ULONGLONG elapsed_ms);
  void ScheduleScreenshotSelectionDimRegionUpdate();
  void FlushScreenshotSelectionDimRegionUpdate();
  void CancelScreenshotSelectionDimRegionUpdate();
  void ApplyScreenshotSelectionDimRegion();
  void InvalidateScreenshotSelectionDimChange(const RECT &old_selection_bounds, const RECT &new_selection_bounds);
  void UpdateScreenshotSelectionOverlay(const RECT &selection_bounds);
  void BeginScreenshotSelectionPointerDown(const POINT &point);
  void CompleteScreenshotSelectionPointerUp(const POINT &point);
  void CompleteScreenshotSelectionOverlay(bool cancelled);
  void DismissNativeSelectionOverlays();
  void DestroyScreenshotSelectionOverlayWindows();
  void MoveSelectionOverlayWindow(HWND hwnd, const RECT &bounds, bool activate = false);
  void LogScreenshotHoverDebug(const std::string &signature, const std::string &message);
  bool TryPickSmallestHoverCandidate(const POINT &point, const std::vector<RECT> &candidate_bounds, RECT *selection_out) const;
  bool TryPickCachedHoverSelection(const POINT &point, RECT *selection_out, std::string *source_out) const;
  bool TryResolveUiaHoverSelection(const POINT &point, RECT *selection_out, std::string *source_out, std::vector<RECT> *candidate_bounds_out);
  bool TryResolveWindowHoverSelection(const POINT &point, RECT *selection_out, std::string *source_out, std::vector<RECT> *candidate_bounds_out);
  bool TryGetUiaElementBounds(IUIAutomationElement *element, RECT *bounds_out);
  bool IsChromeLikeScreenshotHoverUiaElement(IUIAutomationElement *element, const RECT &bounds, HWND native_window);
  void AddHoverCandidateRect(const RECT &candidate, std::vector<RECT> *candidate_bounds_out) const;
  bool TryFindDeepestUiaElementBounds(IUIAutomationTreeWalker *walker, IUIAutomationElement *element, const POINT &point, int depth, RECT *bounds_out, std::vector<RECT> *candidate_bounds_out);
  bool NormalizeHoverSelectionRect(const POINT &point, const RECT &candidate, RECT *selection_out, bool allow_display_sized_candidate = false) const;
  // Resolve the physical display rectangle even before screenshot captures have been cached.
  RECT DisplayBoundsForPoint(POINT point) const;
  bool IsScreenshotOverlayWindow(HWND hwnd);
  bool IsSelectableScreenshotHoverWindow(HWND hwnd);
  HWND FindUnderlyingWindowAtPoint(const POINT &point);
  HWND ResolveScreenshotHoverRootWindowAtPoint(const POINT &point);
  IUIAutomation *EnsureScreenshotUiaAutomation();
  const CachedDisplayCapture *PreferredDisplayCaptureForSelection(const RECT &selection_bounds) const;
  const CachedDisplayCapture *DisplayCaptureForPoint(POINT point) const;
  void BeginScrollingCaptureOverlay(HWND hwnd, const RECT &workspace_bounds, const RECT &selection_bounds, const RECT &controls_bounds);
  void DismissScrollingCaptureOverlay();
  void MoveScrollingCaptureControlsWindow(HWND hwnd, const RECT &controls_bounds);
  void SetScrollingCaptureControlsBackdrop(HWND hwnd, bool compact);
  HRGN CreateScrollingCaptureControlsRegion(int width, int height) const;
  void ApplyScrollingCaptureControlsRegion(HWND hwnd);
  void ClearScrollingCaptureControlsRegion();
  void PaintScreenshotSelectionOverlay(HWND hwnd);
  void PaintScrollingCaptureOverlay(HWND hwnd);
  void EmitScrollingCaptureWheelEvent(int wheel_delta);
  bool IsPointInScrollingCaptureSelection(POINT point) const;

  // Send window event to Flutter
  void SendWindowEvent(const std::string &eventName);

  // Handle method calls from Flutter
  void HandleWindowManagerMethodCall(
      const flutter::MethodCall<flutter::EncodableValue> &method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);

  // Dismiss the Windows Start Menu if it is currently open.
  // SetForegroundWindow requires no menus to be active.
  void DismissStartMenuIfOpen();

  // Static window procedure for handling window events
  static LRESULT CALLBACK WindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);

  // Static child window procedure for observing the Flutter view hwnd.
  static LRESULT CALLBACK ChildWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);

  // Static overlay procedure for the passive scrolling screenshot mask.
  static LRESULT CALLBACK ScrollingCaptureOverlayWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);

  // Static procedure for native screenshot region selection input.
  static LRESULT CALLBACK ScreenshotSelectionInputWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);

  // Static procedure for passive native screenshot dim/border windows.
  static LRESULT CALLBACK ScreenshotSelectionPassiveWindowProc(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);

  // Static low-level mouse hook for native screenshot region selection.
  static LRESULT CALLBACK ScreenshotSelectionMouseHookProc(int code, WPARAM wparam, LPARAM lparam);

  // Static low-level mouse hook for native scrolling screenshot wheel observation.
  static LRESULT CALLBACK ScrollingCaptureMouseHookProc(int code, WPARAM wparam, LPARAM lparam);

  // Track non-repeat keydowns that reached the Flutter child window. If the
  // matching keyup later lands on the root window and Flutter ignores it, we
  // use this set to decide whether the release should be sent back to the
  // child window.
  void TrackChildKeyDown(UINT message, WPARAM wparam, LPARAM lparam);
  void ClearTrackedChildKeyDown(UINT message, WPARAM wparam, LPARAM lparam);
  bool HasTrackedChildKeyDown(UINT message, WPARAM wparam, LPARAM lparam) const;
  bool RerouteIgnoredRootKeyUp(HWND hwnd, UINT message, WPARAM wparam, LPARAM lparam);
  // Sends a synthetic WM_KEYUP/WM_SYSKEYUP to the child window for every
  // keydown that was tracked but has not yet received a matching keyup.
  // When skipPhysicallyHeld is true (default) the flush is skipped for keys
  // that are still physically depressed according to GetAsyncKeyState; this
  // is appropriate for the show/capture paths where WM_SETFOCUS will re-sync
  // modifier state.  When skipPhysicallyHeld is false (hide path) every
  // pending keydown is flushed unconditionally, because after SW_HIDE the real
  // keyup will be delivered to whichever window gains focus next — not Flutter
  // — leaving HardwareKeyboard in a permanently "pressed" state.
  void FlushPendingChildKeyUps(bool skipPhysicallyHeld = true);
  static uint64_t MakeKeyboardMessageSignature(UINT message, WPARAM wparam, LPARAM lparam);

  std::unordered_set<uint64_t> pending_child_keydowns_;
};

#endif // RUNNER_FLUTTER_WINDOW_H_
