import 'dart:async';
import 'dart:math' as math;
import 'dart:io';
import 'dart:convert';
import 'dart:ui';

import 'package:extended_text_field/extended_text_field.dart';
import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controllers/query_box_text_editing_controller.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_base_list_controller.dart';
import 'package:wox/controllers/wox_grid_controller.dart';
import 'package:wox/controllers/wox_list_controller.dart';
import 'package:wox/controllers/wox_screenshot_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/screenshot_session.dart';
import 'package:wox/models/doctor_check_result.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/windows/windows_window_manager.dart';
import 'package:wox/utils/windows/linux_window_manager.dart';
import 'package:wox/utils/windows/window_manager.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_launch_mode_enum.dart';
import 'package:wox/enums/wox_start_page_enum.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';

import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/enums/wox_msg_type_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_refinement_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_result_action_type_enum.dart';
import 'package:wox/enums/wox_result_tail_text_category_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';
import 'package:wox/enums/wox_show_source_enum.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/picker.dart';
import 'package:wox/utils/result_drag_platform_bridge.dart';
import 'package:wox/utils/screenshot/screenshot_platform_bridge.dart';
import 'package:wox/utils/wox_hotkey_recording_bus.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_platform_hotkey_util.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_system_wallpaper_util.dart';
import 'package:wox/utils/wox_time_tracker.dart';
import 'package:wox/utils/webview/wox_webview_util.dart';

import 'package:wox/utils/wox_websocket_msg_util.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/utils/window_flicker_detector.dart';
import 'package:wox/utils/color_util.dart';

class WoxLauncherController extends GetxController {
  static const int _slowLauncherActivationWarningThresholdMs = 50;
  static const String _onReceivedTailTooltip = "onReceivedQueryResults elapsed since Flutter query request";
  static const String _queryActionIconRefType = "iconref";
  static const String localActionTogglePreviewFullscreenId = "__local_toggle_preview_fullscreen__";
  static const String localActionPreviewSearchId = "__local_preview_search__";
  static const String localActionOpenUpdateId = "__local_open_update__";
  static const String localActionOpenDoctorId = "__local_open_doctor__";
  static const String localActionOpenWebViewInspectorId = "__local_open_webview_inspector__";
  static const String localActionWebViewRefreshId = "__local_webview_refresh__";
  static const String localActionWebViewBackId = "__local_webview_back__";
  static const String localActionWebViewForwardId = "__local_webview_forward__";
  static const String localActionWebViewClearStateId = "__local_webview_clear_state__";
  static const String localActionLoadFilePreviewId = "__local_load_file_preview__";

  int _captureDevLauncherVisibleActivationCost(ShowAppParams params) {
    if (!Env.isDev || params.activationStartedAt <= 0) {
      return -1;
    }

    // The dev warning is about when the launcher becomes visible, not when the
    // later focus/input-ready work finishes. Capture this immediately after
    // windowManager.show() so the toolbar value matches the user's visual boundary.
    return DateTime.now().millisecondsSinceEpoch - params.activationStartedAt;
  }

  Future<void> _showDevLauncherActivationWarningIfSlow(String traceId, int visibleActivationCost) async {
    if (!Env.isDev || visibleActivationCost < 0 || visibleActivationCost <= _slowLauncherActivationWarningThresholdMs) {
      return;
    }

    final toolbarMsgId = "dev-launcher-activation-${DateTime.now().microsecondsSinceEpoch}";
    // Development-only activation diagnostics must be emitted from Flutter after
    // native show finishes. The Go websocket response only proves that the
    // request was accepted, and focus timing is tracked separately from visibility.
    await showToolbarMsg(
      traceId,
      ToolbarMsg(id: toolbarMsgId, title: "Dev: hotkey activation took ${visibleActivationCost}ms (>${_slowLauncherActivationWarningThresholdMs}ms)", displaySeconds: 3),
    );

    Future.delayed(const Duration(seconds: 3), () {
      if (toolbarMsg.value.id == toolbarMsgId) {
        unawaited(clearToolbarMsg(traceId, toolbarMsgId));
      }
    });
  }

  //query related variables
  final currentQuery = PlainQuery.empty().obs;
  // is current query returned results or finished without results
  bool isCurrentQueryReturned = false;
  final launcherFocusNode = FocusNode();
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = QueryBoxTextEditingController(
    selectedTextStyle: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxTextSelectionColor)),
    enableSelectedTextStyle: false,
  );
  // Bug fix: launcher show uses a delayed focus retry for Windows, but that
  // retry must not re-apply SelectAll after the user has already typed into the
  // newly visible query box. This token scopes each visible-launcher focus
  // sequence so stale retries from an older show/hide cycle cannot touch input.
  int _visibleLauncherFocusToken = 0;
  final queryBoxScrollController = ScrollController(initialScrollOffset: 0.0);
  // Stores the current editable text width so query box height can follow visual wrapping.
  double queryBoxTextWrapWidth = 0;
  final queryBoxLineCount = 1.obs;
  final queryBoxTextFieldKey = GlobalKey<ExtendedTextFieldState>();
  // Bug fix: Linux can sometimes surface Enter only as KeyRepeatEvent after the platform/IME
  // consumed the original KeyDownEvent. The query-box view uses this per-press state to
  // execute once from the repeat fallback and swallow later repeats without widening the
  // workaround to other platforms or other focus handlers.
  bool _hasHandledLinuxQueryBoxSubmitKey = false;

  // Query refinements are query-response scoped controls. They are kept
  // separate from result rows so late responses can be guarded by query id and
  // stale plugin filters cannot leak into the next trigger keyword.
  final queryRefinements = <WoxQueryRefinement>[].obs;
  final queryRefinementValues = <String, List<String>>{}.obs;
  final isQueryRefinementBarExpanded = false.obs;
  String queryRefinementScopeKey = "";

  //preview related variables
  final currentPreview = WoxPreview.empty().obs;
  final isShowPreviewPanel = false.obs;
  final terminalFindTrigger = 0.obs;
  final isPreviewFullscreen = false.obs;
  final isManualFilePreviewLoadAvailable = false.obs;
  String _manualFilePreviewLoadAvailabilityKey = "";
  final _manualFilePreviewLoadRequests = StreamController<String>.broadcast();
  final Map<String, StreamController<Map<String, dynamic>>> terminalChunkControllers = {};
  final Map<String, StreamController<Map<String, dynamic>>> terminalStateControllers = {};
  static const double defaultResultPreviewRatio = 0.4;
  double lastResultPreviewRatioBeforePreviewFullscreen = defaultResultPreviewRatio;
  double preferredResultPreviewRatio = defaultResultPreviewRatio;

  /// The ratio of result panel width to total width, value range: 0.0-1.0
  /// e.g., 0.3 means result panel takes 30% width, preview panel takes 70%
  final resultPreviewRatio = defaultResultPreviewRatio.obs;

  // result related variables
  late final WoxListController<WoxQueryResult> resultListViewController;
  late final WoxGridController<WoxQueryResult> resultGridViewController;
  WoxBaseListController<WoxQueryResult> get activeResultViewController => isInGridMode() ? resultGridViewController : resultListViewController;

  // action related variables
  late final WoxListController<WoxResultAction> actionListViewController;
  final isShowActionPanel = false.obs;

  // form action related variables
  final isShowFormActionPanel = false.obs;
  final activeFormAction = Rxn<WoxResultAction>();
  final activeFormResultId = "".obs;
  final formActionValues = <String, String>{}.obs;

  /// Grace window before dropping stale visible results for a new query.
  /// This avoids immediate clear/fill flashes when the next snapshot arrives quickly.
  Timer clearQueryResultsTimer = Timer(const Duration(), () => {});
  static const staleVisibleResultsDuration = Duration(milliseconds: 80);
  static const Size _onboardingWindowSize = Size(1040, 800);
  final windowFlickerDetector = WindowFlickerDetector();
  Size? ongoingResizeTargetSize;
  int resizeRequestToken = 0;

  /// True while a fixed-size management window is being staged before its Flutter route mounts.
  bool isManagementWindowTransitionActive = false;
  bool isShowingPendingResultPlaceholder = false;
  double? pendingResultPlaceholderHeight;
  double? committedWindowHeight;

  /// This flag is used to control whether the user can arrow up to show history when the app is first shown.
  var canArrowUpHistory = true;
  final latestQueryHistories = <QueryHistory>[]; // the latest query histories
  var currentQueryHistoryIndex = 0; //  query history index, used to navigate query history

  /// Pending preserved index for query refresh
  int? pendingPreservedIndex;

  var lastLaunchMode = WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code;
  var lastStartPage = WoxStartPageEnum.WOX_START_PAGE_MRU.code;
  final isInSettingView = false.obs;
  // Onboarding shares the settings-sized management window but stays separate
  // from isInSettingView so build routing and tests can distinguish the guide
  // while backend notification routing still treats it as a management surface.
  final isInOnboardingView = false.obs;

  // UI Control Flags
  final isQueryBoxAtBottom = false.obs;
  final isQueryBoxVisible = true.obs;
  final isToolbarHiddenForce = false.obs;
  double forceWindowWidth = 0;
  int forceMaxResultCount = 0;
  var forceHideOnBlur = false;
  // Used to store the current main query before a temporary query source (tray query / selection query / query hotkey query)
  // overwrites it, so that we can restore the main query after hiding.
  PlainQuery? queryBeforeTemporaryQuery;
  String? queryBeforeTemporaryQuerySource;
  double? windowHeightBeforeTemporaryQuery;
  // After restoring the preserved main query we create a new queryId, so the
  // original temporary-query snapshot above is no longer enough to identify
  // which follow-up ShowApp should reuse the old expanded height. These fields
  // keep that one-shot "restored query is still waiting for results" context
  // until the matching query results arrive.
  String? pendingRestoredQueryId;
  double? pendingRestoredQueryWindowHeight;

  var positionBeforeOpenSetting = const Offset(0, 0);
  // Whether settings was opened when window was hidden (e.g., from tray)
  bool isSettingOpenedFromHidden = false;

  // Performance metrics: Map<traceId, startTime>
  final Map<String, int> queryStartTimeMap = {};
  // UI-only onReceivedQueryResults metric per result, kept so later backend
  // snapshots can preserve each row's first UI receive boundary.
  final Map<String, int> queryOnReceivedElapsedByResultKey = {};

  /// The icon at end of query box.
  final queryIcon = QueryIconInfo.empty().obs;
  final glanceItems = <GlanceItem>[].obs;
  final attentionUnreadCount = 0.obs;
  Timer? glanceRefreshTimer;
  Timer? hiddenCacheClearTimer;
  QueryContext backendQueryContext = QueryContext.empty();
  String backendQueryContextQueryId = "";
  final queryCompletionHint = Rxn<QueryCompletionHint>();

  /// The result of the doctor check.
  var doctorCheckPassed = true;
  final doctorCheckInfo = DoctorCheckInfo.empty().obs;
  String lastAppliedDoctorToolbarMessage = '';

  // toolbar related variables
  final toolbar = ToolbarInfo.empty().obs;
  final toolbarMsg = ToolbarMsg.empty().obs;
  // Feature: bug aware mode is launcher-wide diagnostic state, not a plugin
  // toolbar message. Keeping it separate preserves ShowToolbarMsg ownership
  // while allowing monitoring mode to keep a persistent toolbar indicator.
  final isBugAwareModeEnabled = false.obs;
  // store i18n key instead of literal text
  final toolbarCopyText = 'toolbar_copy'.obs;

  // quick select related variables
  final isQuickSelectMode = false.obs;
  Timer? quickSelectTimer;
  final quickSelectDelay = 300; // delay to show number labels
  bool isQuickSelectKeyPressed = false;

  // grid layout related variables
  final isGridLayout = false.obs;
  final gridLayoutParams = GridLayoutParams.empty().obs;

  // loading animation related variables
  final isLoading = false.obs;
  Timer? loadingTimer;
  final loadingDelay = const Duration(milliseconds: 500);

  bool get shouldShowGlance {
    final setting = WoxSettingUtil.instance.currentSetting;
    return setting.enableGlance && isGlobalInputQuery(currentQuery.value) && queryIcon.value.icon.imageData.isEmpty && glanceItems.isNotEmpty && !isLoading.value;
  }

  bool get shouldShowAttentionBadge {
    return attentionUnreadCount.value > 0 && isGlobalInputQuery(currentQuery.value) && !isLoading.value;
  }

  bool shouldShowGlanceIcon(GlanceItem item) {
    final setting = WoxSettingUtil.instance.currentSetting;
    // Keep the render decision in one place so layout width and widget content
    // stay synchronized when users switch Glance into text-only mode.
    return !setting.hideGlanceIcon && item.icon.imageData.isNotEmpty;
  }

  List<GlanceRef> selectedGlanceRefs() {
    final setting = WoxSettingUtil.instance.currentSetting;
    final refs = <GlanceRef>[];
    if (!setting.primaryGlance.isEmpty) {
      refs.add(setting.primaryGlance);
    }
    return refs;
  }

  Future<void> refreshGlance(String traceId, String reason, {String pluginId = "", List<String> ids = const []}) async {
    glanceRefreshTimer?.cancel();
    final queryId = currentQuery.value.queryId;
    final setting = WoxSettingUtil.instance.currentSetting;
    if (!setting.enableGlance || !isGlobalInputQuery(currentQuery.value)) {
      // Global Glance must not leak into plugin contexts because the query-box
      // accessory is already reserved for plugin identity there.
      glanceItems.clear();
      return;
    }

    var refs = selectedGlanceRefs();
    if (pluginId.isNotEmpty) {
      refs = refs.where((ref) => ref.pluginId == pluginId && (ids.isEmpty || ids.contains(ref.glanceId))).toList();
    }
    if (refs.isEmpty) {
      glanceItems.clear();
      return;
    }

    try {
      final items = await WoxApi.instance.getGlanceItems(traceId, refs, reason);
      if (currentQuery.value.queryId != queryId || !isGlobalInputQuery(currentQuery.value)) {
        // Bug fix: Glance refreshes are HTTP requests and can finish after the
        // user has already cleared or changed the search. Ignore stale replies
        // so an older query cannot hide or restore the accessory for the current
        // launcher state.
        return;
      }
      final byKey = {for (final item in items) '${item.pluginId}\x00${item.id}': item};
      // Preserve slot order from settings instead of plugin response order so the
      // primary item remains visually stable across refreshes.
      glanceItems.assignAll(refs.map((ref) => byKey[ref.key]).whereType<GlanceItem>().where((item) => !item.isEmpty).toList());
      scheduleNextGlanceRefresh(traceId);
    } catch (e) {
      Logger.instance.error(traceId, "refresh glance failed: $e");
      if (currentQuery.value.queryId == queryId && isGlobalInputQuery(currentQuery.value)) {
        // Bug fix: only the active query can clear the visible Glance. A failed
        // request from an older query should not remove the accessory after the
        // user has already returned to the empty global search box.
        glanceItems.clear();
      }
    }
  }

  void scheduleNextGlanceRefresh(String traceId) {
    final interval = resolveSelectedGlanceRefreshInterval();
    if (interval == null || interval <= Duration.zero) {
      return;
    }
    glanceRefreshTimer = Timer(interval, () {
      unawaited(refreshGlance(const UuidV4().generate(), "interval"));
    });
  }

  // scheduleHiddenCacheClear clears Flutter caches again after hide animations settle.
  void scheduleHiddenCacheClear(String traceId) {
    hiddenCacheClearTimer?.cancel();
    hiddenCacheClearTimer = Timer(const Duration(seconds: 10), () {
      unawaited(clearHiddenCaches());
    });
  }

  Future<void> clearHiddenCaches() async {
    hiddenCacheClearTimer = null;
    if (await windowManager.isVisible()) {
      return;
    }

    PaintingBinding.instance.imageCache.clear();
    PaintingBinding.instance.imageCache.clearLiveImages();
  }

  Duration? resolveSelectedGlanceRefreshInterval() {
    final refs = selectedGlanceRefs();
    if (refs.isEmpty || !Get.isRegistered<WoxSettingController>()) {
      return null;
    }
    final settingController = Get.find<WoxSettingController>();
    int? minIntervalMs;
    for (final ref in refs) {
      for (final plugin in settingController.installedPlugins) {
        if (plugin.id != ref.pluginId) {
          continue;
        }
        for (final glance in plugin.glances) {
          if (glance.id == ref.glanceId && glance.refreshIntervalMs > 0) {
            minIntervalMs = minIntervalMs == null ? glance.refreshIntervalMs : math.min(minIntervalMs, glance.refreshIntervalMs);
          }
        }
      }
    }
    if (minIntervalMs == null) {
      return null;
    }
    // Bug fix: the previous 60-second floor ignored metadata for live Glance
    // metrics such as CPU and memory. Trust provider intervals so 3-second
    // system metrics can refresh at their declared cadence.
    return Duration(milliseconds: minIntervalMs);
  }

  Future<void> executeGlanceDefaultAction(String traceId, GlanceItem item) async {
    final action = item.action;
    if (action == null) {
      return;
    }
    await WoxApi.instance.executeGlanceAction(traceId, item.pluginId, item.id, action.id);
    if (!action.preventHideAfterAction) {
      hideApp(traceId);
    }
  }

  /// Reset controller state for integration testing without full disposal.
  /// This clears pending timers, hides panels, and resets query state.
  Future<void> resetForIntegrationTest() async {
    hiddenCacheClearTimer?.cancel();
    await Get.find<WoxScreenshotController>().resetForIntegrationTest();

    // Avoid focus restoration during smoke teardown. The regular hide helpers
    // call focusQueryBox(), which can outlive the test and touch a disposed
    // FocusManager after Get.reset() starts tearing down the app.
    isShowActionPanel.value = false;
    actionListViewController.clearFilter(const UuidV4().generate());
    activeFormAction.value = null;
    activeFormResultId.value = "";
    formActionValues.clear();
    isShowFormActionPanel.value = false;

    if (isInSettingView.value) {
      await exitSetting(const UuidV4().generate());
    }
    if (isInOnboardingView.value) {
      // Test teardown should not mark the guide as completed; it only clears
      // the transient management-view state so the next smoke test starts from
      // a clean launcher/window lifecycle.
      isInOnboardingView.value = false;
      await WoxApi.instance.onOnboarding(const UuidV4().generate(), false);
    }

    queryBoxTextFieldController.clear();
    onQueryBoxTextChanged('');
    queryBeforeTemporaryQuery = null;
    queryBeforeTemporaryQuerySource = null;
    windowHeightBeforeTemporaryQuery = null;
    pendingRestoredQueryId = null;
    pendingRestoredQueryWindowHeight = null;
    isGridLayout.value = false;
    clearQueryRefinements(const UuidV4().generate());
    cancelPendingResultTransitions();
    quickSelectTimer?.cancel();
    isQuickSelectMode.value = false;
    loadingTimer?.cancel();
    isLoading.value = false;
  }

  @override
  void onInit() {
    super.onInit();

    // On Linux, the IME (IBus/Fcitx) can consume the first keydown after focus gain.
    // Escape still needs a focus-node fallback because the shared query-box handler only reacts
    // to KeyDown for hide, while Enter fallback now lives in the query-box view's KeyRepeat path
    // so the workaround stays attached to the multiline input that would otherwise insert newline.
    if (Platform.isLinux) {
      queryBoxFocusNode.onKeyEvent = (node, event) {
        if ((event is KeyDownEvent || event is KeyRepeatEvent) && event.logicalKey == LogicalKeyboardKey.escape && !WoxHotkey.isAnyModifierPressed()) {
          hideApp(const UuidV4().generate());
          return KeyEventResult.handled;
        }
        return KeyEventResult.ignored;
      };
    }

    resultListViewController = Get.put(
      WoxListController<WoxQueryResult>(
        onItemExecuted: (traceId, item) {
          executeDefaultAction(traceId);
        },
        onItemActive: onResultItemActivated,
        onItemsEmpty: onResultItemsEmpty,
        itemHeightGetter: () => WoxThemeUtil.instance.getResultItemHeight(),
      ),
      tag: 'result',
    );

    resultGridViewController = Get.put(
      WoxGridController(
        onItemExecuted: (traceId, item) {
          executeDefaultAction(traceId);
        },
        onItemActive: onResultItemActivated,
        onItemsEmpty: onResultItemsEmpty,
      ),
      tag: 'grid',
    );

    actionListViewController = Get.put(
      WoxListController<WoxResultAction>(
        onItemExecuted: (traceId, item) {
          executeDefaultAction(traceId);
        },
        onFilterBoxEscPressed: hideActionPanel,
        itemHeightGetter: () => WoxThemeUtil.instance.getActionItemHeight(),
      ),
      tag: 'action',
    );

    // Add focus listener to query box
    queryBoxFocusNode.addListener(() {
      updateQueryBoxSelectedTextStyle();
      if (queryBoxFocusNode.hasFocus) {
        // Reset Linux fallback state whenever focus returns because the launcher can hide before
        // Flutter delivers the matching KeyUpEvent for Enter.
        _hasHandledLinuxQueryBoxSubmitKey = false;
        var traceId = const UuidV4().generate();
        final screenshotController = Get.find<WoxScreenshotController>();
        if (screenshotController.isSessionActive.value) {
          // Screenshot annotation reuses the same top-level window as the launcher. Letting the
          // hidden query box reclaim focus during an active screenshot session reroutes keyboard and
          // IME side effects back into launcher code, which can make the screenshot editor dismiss
          // itself before the user finishes annotating. Drop that stale focus immediately instead of
          // treating it like a real launcher activation.
          queryBoxFocusNode.unfocus();
          return;
        }
        if (isShowFormActionPanel.value) {
          Logger.instance.debug(traceId, "query box gained focus while form action panel is visible, ignore auto hide");
          return;
        }
        hideActionPanel(traceId);
        hideFormActionPanel(traceId, reason: "query box gained focus");

        // Call API when query box gains focus
        WoxApi.instance.onQueryBoxFocus(traceId);
      }
    });
    queryBoxTextFieldController.addListener(syncQueryBoxCompletionHint);

    // Add scroll listener to update quick select numbers when scrolling
    resultListViewController.scrollController.addListener(() {
      if (isQuickSelectMode.value) {
        updateQuickSelectNumbers(const UuidV4().generate());
      }
    });
    resultGridViewController.scrollController.addListener(() {
      if (isQuickSelectMode.value) {
        updateQuickSelectNumbers(const UuidV4().generate());
      }
    });

    // Initialize doctor check info
    doctorCheckInfo.value = DoctorCheckInfo.empty();
  }

  void updateQueryBoxSelectedTextStyle() {
    queryBoxTextFieldController.updateSelectedTextStyle(
      style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxTextSelectionColor)),
      enabled: queryBoxFocusNode.hasFocus,
    );
    syncQueryBoxCompletionHint();
  }

  bool isQueryBoxComposing() {
    final composing = queryBoxTextFieldController.value.composing;
    return composing.start >= 0 && composing.end >= 0;
  }

  bool isQueryBoxCursorAtEnd() {
    final selection = queryBoxTextFieldController.selection;
    return selection.isValid && selection.isCollapsed && selection.extentOffset == queryBoxTextFieldController.text.length;
  }

  bool isQueryCompletionHintEnabled() {
    return WoxSettingUtil.instance.currentSetting.enableQueryCompletionHint;
  }

  bool isQueryCompletionHintValid(QueryCompletionHint hint) {
    final currentText = queryBoxTextFieldController.text;
    return isQueryCompletionHintEnabled() &&
        currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code &&
        currentText == hint.inputPrefix &&
        hint.suffix.isNotEmpty &&
        hint.completionText.startsWith(currentText) &&
        isQueryBoxCursorAtEnd() &&
        !isQueryBoxComposing();
  }

  void syncQueryBoxCompletionHint() {
    final hint = queryCompletionHint.value;
    final suffix = hint != null && isQueryCompletionHintValid(hint) ? hint.suffix : "";
    queryBoxTextFieldController.updateCompletionHint(
      suffix: suffix,
      style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxFontColor).withValues(alpha: 0.38)),
    );
  }

  void clearQueryCompletionHint() {
    queryCompletionHint.value = null;
    syncQueryBoxCompletionHint();
  }

  bool applyQueryCompletionHintForQueryId(String traceId, String queryId, QueryCompletionHint? hint) {
    if (currentQuery.value.queryId != queryId) {
      Logger.instance.debug(traceId, "ignore stale query completion hint: response queryId=$queryId, current queryId=${currentQuery.value.queryId}");
      return false;
    }

    if (hint == null || !isQueryCompletionHintValid(hint)) {
      clearQueryCompletionHint();
      return false;
    }

    queryCompletionHint.value = hint;
    syncQueryBoxCompletionHint();
    return true;
  }

  bool reuseQueryCompletionHintForText(String value) {
    if (!isQueryCompletionHintEnabled()) {
      clearQueryCompletionHint();
      return false;
    }

    final hint = queryCompletionHint.value;
    if (hint == null || value.length <= hint.inputPrefix.length || !value.startsWith(hint.inputPrefix) || !hint.completionText.startsWith(value)) {
      return false;
    }

    final suffix = hint.completionText.substring(value.length);
    if (suffix.isEmpty) {
      clearQueryCompletionHint();
      return true;
    }

    queryCompletionHint.value = QueryCompletionHint(inputPrefix: value, completionText: hint.completionText, suffix: suffix, source: hint.source, score: hint.score);
    syncQueryBoxCompletionHint();
    return true;
  }

  void markLinuxQueryBoxSubmitKeyHandled() {
    _hasHandledLinuxQueryBoxSubmitKey = true;
  }

  void resetLinuxQueryBoxSubmitKeyHandling() {
    _hasHandledLinuxQueryBoxSubmitKey = false;
  }

  bool shouldExecuteLinuxQueryBoxSubmitFromRepeat() {
    if (_hasHandledLinuxQueryBoxSubmitKey) {
      return false;
    }

    _hasHandledLinuxQueryBoxSubmitKey = true;
    return true;
  }

  bool isGlobalInputQuery(PlainQuery query) {
    if (query.queryType != WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      return false;
    }
    if (backendQueryContextQueryId == query.queryId) {
      return backendQueryContext.isGlobalQuery;
    }
    final normalizedQuery = expandQueryShortcutForLocalClassification(query.queryText);
    if (!normalizedQuery.contains(" ")) {
      return true;
    }
    if (!hasLocalPluginTriggerMetadata()) {
      return false;
    }
    return !isKnownPluginInputQueryText(normalizedQuery);
  }

  bool isLikelyPluginInputQuery(PlainQuery query) {
    if (query.queryType != WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      return false;
    }
    final normalizedQuery = expandQueryShortcutForLocalClassification(query.queryText);
    if (!normalizedQuery.contains(" ")) {
      return false;
    }
    if (!hasLocalPluginTriggerMetadata()) {
      // Keep plugin-looking queries in an unknown state until plugin metadata is
      // loaded. Core's QueryContext will correct the surface once a response
      // arrives, and this avoids clearing a plugin icon on incomplete local data.
      return true;
    }
    return isKnownPluginInputQueryText(normalizedQuery);
  }

  String getQueryRefinementScopeKey(PlainQuery query) {
    if (query.queryType != WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      return "";
    }

    final normalizedQuery = expandQueryShortcutForLocalClassification(query.queryText).trimLeft();
    if (normalizedQuery.isEmpty || !RegExp(r'\s').hasMatch(normalizedQuery)) {
      return "";
    }

    return normalizedQuery.split(RegExp(r'\s+')).first;
  }

  bool shouldPreserveQueryRefinementsForTextChange(PlainQuery currentQueryValue, String nextText) {
    final currentScope = getQueryRefinementScopeKey(currentQueryValue);
    if (currentScope.isEmpty) {
      return false;
    }

    final nextScope = getQueryRefinementScopeKey(
      PlainQuery(queryId: "", queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: nextText, querySelection: Selection.empty()),
    );
    return currentScope == nextScope;
  }

  // Hidden context follows the same trigger scope as refinements but is not rendered in the UI.
  bool shouldPreserveQueryContextDataForTextChange(PlainQuery currentQueryValue, String nextText) {
    if (currentQueryValue.contextData.isEmpty) {
      return false;
    }
    return shouldPreserveQueryRefinementsForTextChange(currentQueryValue, nextText);
  }

  Map<String, List<String>> cloneQueryRefinementValues(Map<String, List<String>> values) {
    return values.map((key, value) => MapEntry(key, List<String>.from(value)));
  }

  Map<String, String> cloneQueryRefinementPayload(Map<String, String> values) {
    return Map<String, String>.from(values);
  }

  // Clone query context before assigning it to a new PlainQuery so later edits cannot mutate previous query snapshots.
  Map<String, String> cloneQueryContextData(Map<String, String> values) {
    return Map<String, String>.from(values);
  }

  List<String> decodeQueryRefinementValue(String value) {
    if (value.isEmpty) {
      return const <String>[];
    }
    return value.split(',').map((part) => part.trim()).where((part) => part.isNotEmpty).toList();
  }

  Map<String, List<String>> decodeQueryRefinementPayload(Map<String, String> payload) {
    return payload.map((key, value) => MapEntry(key, decodeQueryRefinementValue(value)));
  }

  String encodeQueryRefinementValue(List<String> values) {
    return values.where((value) => value.isNotEmpty).join(',');
  }

  Map<String, String> encodeQueryRefinementPayload(Map<String, List<String>> values) {
    final payload = <String, String>{};
    for (final entry in values.entries) {
      // Protocol change: plugins receive refinements as map[string]string for a
      // simpler API. The UI keeps list selections internally for multi-select
      // controls, then encodes multi values as comma-separated strings here.
      final encoded = encodeQueryRefinementValue(entry.value);
      if (encoded.isNotEmpty) {
        payload[entry.key] = encoded;
      }
    }
    return payload;
  }

  void clearQueryRefinements(String traceId) {
    if (queryRefinements.isEmpty && queryRefinementValues.isEmpty && queryRefinementScopeKey.isEmpty && !isQueryRefinementBarExpanded.value) {
      return;
    }

    // Bug fix: refinement controls are owned by the plugin query that returned
    // them. Clearing both definitions and selected values prevents filters from
    // one trigger keyword from silently being sent to another plugin.
    queryRefinements.clear();
    queryRefinementValues.clear();
    isQueryRefinementBarExpanded.value = false;
    queryRefinementScopeKey = "";
    unawaited(resizeHeight(traceId: traceId, reason: "query refinements cleared"));
  }

  void prepareQueryRefinementsOnQueryChanged(String traceId, PlainQuery query) {
    final nextScopeKey = getQueryRefinementScopeKey(query);
    queryRefinementValues.assignAll(cloneQueryRefinementValues(decodeQueryRefinementPayload(query.queryRefinements)));

    if (nextScopeKey.isEmpty || (queryRefinementScopeKey.isNotEmpty && queryRefinementScopeKey != nextScopeKey)) {
      clearQueryRefinements(traceId);
      return;
    }
  }

  List<String> normalizeQueryRefinementSelection(WoxQueryRefinement refinement, List<String> rawValues) {
    final optionValues = refinement.options.map((option) => option.value).where((value) => value.isNotEmpty).toSet();
    final allowAnyValue = optionValues.isEmpty;

    List<String> normalizeValues(List<String> values) {
      final normalized = <String>[];
      for (final value in values) {
        if (value.isEmpty || (!allowAnyValue && !optionValues.contains(value)) || normalized.contains(value)) {
          continue;
        }
        normalized.add(value);
      }
      return normalized;
    }

    var normalized = normalizeValues(rawValues);
    if (normalized.isEmpty) {
      normalized = normalizeValues(refinement.defaultValue);
    }

    if ((refinement.type == WoxQueryRefinementTypeEnum.singleSelect.code || refinement.type == WoxQueryRefinementTypeEnum.sort.code) &&
        normalized.isEmpty &&
        refinement.options.isNotEmpty) {
      normalized = [refinement.options.first.value];
    }

    if (refinement.type == WoxQueryRefinementTypeEnum.singleSelect.code || refinement.type == WoxQueryRefinementTypeEnum.sort.code) {
      return normalized.take(1).toList();
    }

    return normalized;
  }

  Map<String, List<String>> normalizeQueryRefinementValues(List<WoxQueryRefinement> refinements, Map<String, List<String>> selectedValues) {
    final normalized = <String, List<String>>{};
    for (final refinement in refinements) {
      if (refinement.isEmpty) {
        continue;
      }

      // Feature addition: response defaults are materialized into the next
      // query payload so plugins receive a stable object even before the user
      // changes a control manually.
      final selected = normalizeQueryRefinementSelection(refinement, selectedValues[refinement.id] ?? const <String>[]);
      if (selected.isNotEmpty) {
        normalized[refinement.id] = selected;
      }
    }
    return normalized;
  }

  bool applyQueryRefinementsForQueryId(String traceId, String queryId, List<WoxQueryRefinement> refinements) {
    if (currentQuery.value.queryId != queryId) {
      // QueryResponse arrives asynchronously. Guarding by query id prevents a
      // late plugin response from replacing controls for the query the user is
      // currently editing.
      Logger.instance.debug(traceId, "ignore stale query refinements: response queryId=$queryId, current queryId=${currentQuery.value.queryId}");
      return false;
    }

    applyQueryRefinementsForQuery(traceId, currentQuery.value, refinements);
    return true;
  }

  void applyQueryRefinementsForQuery(String traceId, PlainQuery query, List<WoxQueryRefinement> refinements) {
    final validRefinements = refinements.where((refinement) => !refinement.isEmpty).toList();
    if (validRefinements.isEmpty) {
      currentQuery.value = cloneQuery(query, queryRefinements: <String, String>{});
      clearQueryRefinements(traceId);
      return;
    }

    final normalizedValues = normalizeQueryRefinementValues(validRefinements, decodeQueryRefinementPayload(query.queryRefinements));
    queryRefinements.assignAll(validRefinements);
    queryRefinementValues.assignAll(normalizedValues);
    queryRefinementScopeKey = getQueryRefinementScopeKey(query);
    currentQuery.value = cloneQuery(query, queryRefinements: encodeQueryRefinementPayload(normalizedValues));
    unawaited(resizeHeight(traceId: traceId, reason: "query refinements applied"));
  }

  List<String> getQueryRefinementSelectedValues(String refinementId) {
    return List<String>.from(queryRefinementValues[refinementId] ?? const <String>[]);
  }

  void updateQueryRefinementSelection(String traceId, WoxQueryRefinement refinement, List<String> values) {
    final normalizedValues = normalizeQueryRefinementSelection(refinement, values);
    final nextRefinements = cloneQueryRefinementValues(decodeQueryRefinementPayload(currentQuery.value.queryRefinements));
    if (normalizedValues.isEmpty) {
      nextRefinements.remove(refinement.id);
    } else {
      nextRefinements[refinement.id] = normalizedValues;
    }

    // Feature addition: changing a refinement is equivalent to changing the
    // query. Reusing onQueryChanged keeps loading, stale-result clearing, and
    // websocket payload construction on the same path as text edits.
    final nextQuery = cloneQuery(currentQuery.value, queryId: const UuidV4().generate(), queryRefinements: encodeQueryRefinementPayload(nextRefinements));
    onQueryChanged(traceId, nextQuery, "query refinement changed");
  }

  List<String> getNextQueryRefinementHotkeyValues(WoxQueryRefinement refinement) {
    final optionValues = refinement.options.map((option) => option.value).where((value) => value.isNotEmpty).toList();
    final selectedValues = normalizeQueryRefinementSelection(refinement, getQueryRefinementSelectedValues(refinement.id));

    if (refinement.type == WoxQueryRefinementTypeEnum.toggle.code) {
      // Feature addition: toggle hotkeys mirror the visible toggle control,
      // including its fallback value, so keyboard and mouse paths send the same
      // selected value for minimal toggle definitions.
      final toggleValue = refinement.defaultValue.firstWhereOrNull((value) => value.isNotEmpty) ?? (optionValues.isNotEmpty ? optionValues.first : "true");
      if (selectedValues.contains(toggleValue)) {
        return <String>[];
      }
      return [toggleValue];
    }

    if (refinement.type == WoxQueryRefinementTypeEnum.multiSelect.code) {
      final allSelected = optionValues.isNotEmpty && optionValues.every(selectedValues.contains);
      return allSelected ? <String>[] : optionValues;
    }

    if (optionValues.isEmpty) {
      return <String>[];
    }

    final currentValue = selectedValues.isNotEmpty ? selectedValues.first : optionValues.first;
    final currentIndex = optionValues.indexOf(currentValue);
    return [optionValues[(currentIndex + 1) % optionValues.length]];
  }

  bool executeQueryRefinementHotkey(String traceId, HotKey hotkey) {
    if (!shouldShowQueryRefinementAffordance) {
      return false;
    }

    for (final refinement in queryRefinements) {
      final parsed = WoxHotkey.parseHotkeyFromString(refinement.hotkey);
      if (parsed == null || !parsed.isNormalHotkey) {
        continue;
      }

      if (!WoxHotkey.equals(parsed.normalHotkey, hotkey)) {
        continue;
      }

      // Feature addition: refinement hotkeys mutate the selected values without
      // moving focus away from the query box. This keeps filter changes on the
      // same keyboard-first path as result actions while still routing the next
      // query through the normal websocket flow.
      updateQueryRefinementSelection(traceId, refinement, getNextQueryRefinementHotkeyValues(refinement));
      return true;
    }

    return false;
  }

  String get queryRefinementToggleHotkey => WoxPlatformHotkeyUtil.primaryHotkey("f");

  String get queryRefinementToggleHotkeyLabel => WoxPlatformHotkeyUtil.primaryHotkeyLabel("f");

  bool executeQueryRefinementToggleHotkey(String traceId, HotKey hotkey) {
    if (!shouldShowQueryRefinementAffordance) {
      return false;
    }

    final parsed = WoxHotkey.parseHotkeyFromString(queryRefinementToggleHotkey);
    if (parsed == null || !parsed.isNormalHotkey || !WoxHotkey.equals(parsed.normalHotkey, hotkey)) {
      return false;
    }

    toggleQueryRefinementBar(traceId);
    return true;
  }

  void toggleQueryRefinementBar(String traceId) {
    if (!shouldShowQueryRefinementAffordance) {
      return;
    }

    // Visual refinement: filters are collapsed by default so normal launcher
    // scanning stays tight. Toggling only changes chrome visibility; selected
    // refinement values remain in the query payload either way.
    isQueryRefinementBarExpanded.value = !isQueryRefinementBarExpanded.value;
    unawaited(resizeHeight(traceId: traceId, reason: "query refinement bar toggled"));
  }

  bool isSameQueryRefinementSelection(List<String> left, List<String> right) {
    if (left.length != right.length) {
      return false;
    }

    for (final value in left) {
      if (!right.contains(value)) {
        return false;
      }
    }
    return true;
  }

  bool isQueryRefinementDefaultSelection(WoxQueryRefinement refinement) {
    final selectedValues = normalizeQueryRefinementSelection(refinement, getQueryRefinementSelectedValues(refinement.id));
    final defaultValues = normalizeQueryRefinementSelection(refinement, refinement.defaultValue);
    return isSameQueryRefinementSelection(selectedValues, defaultValues);
  }

  List<String> getActiveQueryRefinementLabels({int limit = 2}) {
    final labels = <String>[];
    for (final refinement in queryRefinements) {
      if (isQueryRefinementDefaultSelection(refinement)) {
        continue;
      }

      final selectedValues = normalizeQueryRefinementSelection(refinement, getQueryRefinementSelectedValues(refinement.id));
      for (final selectedValue in selectedValues) {
        final option = refinement.options.firstWhereOrNull((item) => item.value == selectedValue);
        labels.add(option == null ? selectedValue : tr(option.title));
        if (labels.length >= limit) {
          return labels;
        }
      }
    }
    return labels;
  }

  bool get hasActiveQueryRefinements {
    return getActiveQueryRefinementLabels(limit: 1).isNotEmpty;
  }

  String getQueryRefinementAffordanceLabel() {
    final activeLabels = getActiveQueryRefinementLabels(limit: 2);
    if (activeLabels.isEmpty) {
      return tr("ui_query_refinement_filters");
    }

    final hiddenCount = queryRefinements.where((refinement) => !isQueryRefinementDefaultSelection(refinement)).length - activeLabels.length;
    if (hiddenCount > 0) {
      return "${activeLabels.join(", ")} +$hiddenCount";
    }
    return activeLabels.join(", ");
  }

  bool get shouldShowQueryRefinementAffordance {
    return isQueryBoxVisible.value && queryRefinements.isNotEmpty && !isPreviewOnlyLayout;
  }

  bool get shouldShowQueryRefinements {
    return shouldShowQueryRefinementAffordance && isQueryRefinementBarExpanded.value;
  }

  double getQueryRefinementBarHeight() {
    return shouldShowQueryRefinements ? WoxInterfaceSizeUtil.instance.current.queryRefinementBarHeight : 0.0;
  }

  bool hasLocalPluginTriggerMetadata() {
    return Get.isRegistered<WoxSettingController>() && Get.find<WoxSettingController>().installedPlugins.isNotEmpty;
  }

  bool isKnownPluginInputQueryText(String queryText) {
    final triggerKeyword = queryText.split(" ").first;
    if (triggerKeyword.isEmpty || !Get.isRegistered<WoxSettingController>()) {
      return false;
    }
    final settingController = Get.find<WoxSettingController>();
    for (final plugin in settingController.installedPlugins) {
      final triggerKeywords = plugin.setting.triggerKeywords.isNotEmpty ? plugin.setting.triggerKeywords : plugin.triggerKeywords;
      if (triggerKeywords.contains(triggerKeyword)) {
        return true;
      }
    }
    return false;
  }

  String expandQueryShortcutForLocalClassification(String queryText) {
    final shortcuts = List<QueryShortcut>.from(WoxSettingUtil.instance.currentSetting.queryShortcuts);
    shortcuts.sort((a, b) => b.shortcut.length.compareTo(a.shortcut.length));
    for (final shortcut in shortcuts) {
      if (shortcut.disabled) {
        continue;
      }
      if (queryText != shortcut.shortcut && !queryText.startsWith("${shortcut.shortcut} ")) {
        continue;
      }

      // Local classification mirrors core's shortcut boundary rules so Glance
      // does not disappear for ordinary global text that merely shares a prefix.
      if (!shortcut.query.contains("{0}")) {
        return queryText.replaceFirst(shortcut.shortcut, shortcut.query);
      }

      final arguments = queryText.replaceFirst(shortcut.shortcut, "").trimLeft().split(" ");
      var expandedQuery = shortcut.query;
      for (var index = 0; index < arguments.length; index++) {
        expandedQuery = expandedQuery.replaceAll("{$index}", arguments[index]);
      }
      return expandedQuery;
    }
    return queryText;
  }

  bool get isShowDoctorCheckInfo => currentQuery.value.isEmpty && !doctorCheckInfo.value.allPassed;

  bool get shouldShowUpdateActionInToolbar {
    if (currentQuery.value.isEmpty == false || doctorCheckInfo.value.allPassed) {
      return false;
    }

    return doctorCheckInfo.value.results.any((result) => result.isVersionIssue);
  }

  bool get hasVisibleToolbarMsg => toolbarMsg.value.isPersistent;

  bool get hasBugAwareToolbarIndicator => isBugAwareModeEnabled.value;

  bool get isShowToolbar => activeResultViewController.items.isNotEmpty || isShowDoctorCheckInfo || hasVisibleToolbarMsg || hasBugAwareToolbarIndicator;

  bool get isToolbarVisible => isShowToolbar && !isToolbarHiddenForce.value;

  bool get isToolbarShowedWithoutResults => isToolbarVisible && activeResultViewController.items.isEmpty;

  // Launcher chrome is the query box and toolbar shell around result content.
  bool get isLauncherChromeHidden => !isQueryBoxVisible.value && !isToolbarVisible;

  bool get isPreviewOnlyLayout => isLauncherChromeHidden && isShowPreviewPanel.value && resultPreviewRatio.value == 0;

  String? get resolvedToolbarText => hasVisibleToolbarMsg ? toolbarMsg.value.displayText : toolbar.value.text;

  WoxImage? get resolvedToolbarIcon => hasVisibleToolbarMsg ? toolbarMsg.value.icon : toolbar.value.icon;

  int? get resolvedToolbarProgress => hasVisibleToolbarMsg ? toolbarMsg.value.progress : null;

  bool get resolvedToolbarIndeterminate => hasVisibleToolbarMsg && toolbarMsg.value.indeterminate;

  Future<void> loadDiagnosticStatus(String traceId) async {
    try {
      final status = await WoxApi.instance.getDiagnosticStatus(traceId);
      updateDiagnosticStatus(traceId, status["enabled"] == true, shouldResize: false);
    } catch (e) {
      Logger.instance.warn(traceId, "failed to load diagnostic status: $e");
    }
  }

  void updateDiagnosticStatus(String traceId, bool enabled, {bool shouldResize = true}) {
    if (isBugAwareModeEnabled.value == enabled) {
      return;
    }

    // Feature: the backend owns the persistent diagnostic flag and Flutter owns
    // the toolbar affordance. A single update path keeps startup status loading
    // and live websocket changes consistent.
    isBugAwareModeEnabled.value = enabled;
    Logger.instance.info(traceId, "bug aware mode status changed: enabled=$enabled");
    if (shouldResize) {
      unawaited(resizeHeight(traceId: traceId, reason: "bug aware toolbar visibility changed"));
    }
  }

  void updateAttentionUnreadCount(String traceId, int unreadCount) {
    final nextCount = unreadCount < 0 ? 0 : unreadCount;
    if (attentionUnreadCount.value == nextCount) {
      return;
    }

    attentionUnreadCount.value = nextCount;
    Logger.instance.info(traceId, "attention unread count changed: count=$nextCount");
  }

  Future<void> activateAttentionQuery(String traceId) async {
    await onQueryChanged(traceId, PlainQuery.text("attention "), "attention badge clicked", moveCursorToEnd: true);
    await focusQueryBox();
  }

  String get attentionHotkey => WoxPlatformHotkeyUtil.primaryHotkey("u");

  String get attentionHotkeyLabel => WoxPlatformHotkeyUtil.primaryHotkeyLabel("u");

  // executeAttentionHotkey only works while the badge is visible, matching the on-screen affordance.
  bool executeAttentionHotkey(String traceId, HotKey hotkey) {
    if (!shouldShowAttentionBadge) {
      return false;
    }

    final parsed = WoxHotkey.parseHotkeyFromString(attentionHotkey);
    if (parsed == null || !parsed.isNormalHotkey || !WoxHotkey.equals(parsed.normalHotkey, hotkey)) {
      return false;
    }

    unawaited(activateAttentionQuery(traceId));
    return true;
  }

  Future<void> activateBugReportQuery(String traceId) async {
    // Feature: clicking the bug indicator should enter the system plugin using
    // Wox's normal trigger-keyword semantics, which require the trailing space.
    await onQueryChanged(traceId, PlainQuery.text("bugreport "), "bug aware toolbar indicator clicked", moveCursorToEnd: true);
    await focusQueryBox();
  }

  String get previewFullscreenHotkey => WoxPlatformHotkeyUtil.primaryHotkey("b");

  String get previewFullscreenHotkeyLabel => WoxPlatformHotkeyUtil.primaryHotkeyLabel("b");

  String get previewSearchHotkey => WoxPlatformHotkeyUtil.primaryHotkey("shift+f");

  String get previewSearchHotkeyLabel => WoxPlatformHotkeyUtil.primaryHotkeyLabel("shift+f");

  String get previewInspectorHotkey => WoxPlatformHotkeyUtil.primaryHotkey("alt+i");
  String get previewRefreshHotkey => WoxPlatformHotkeyUtil.primaryHotkey("r");
  String get previewBackHotkey => WoxPlatformHotkeyUtil.primaryHotkey("[");
  String get previewForwardHotkey => WoxPlatformHotkeyUtil.primaryHotkey("]");

  String get filePreviewLoadHotkey => WoxPlatformHotkeyUtil.primaryHotkey("l");

  String get filePreviewLoadHotkeyLabel => WoxPlatformHotkeyUtil.primaryHotkeyLabel("l");

  Stream<String> get manualFilePreviewLoadRequests => _manualFilePreviewLoadRequests.stream;

  String get moreActionsHotkey => WoxPlatformHotkeyUtil.primaryHotkey("j");

  String get moreActionsHotkeyLabel => WoxPlatformHotkeyUtil.primaryHotkeyLabel("j");

  String? getVisibleResultQueryId() {
    for (final item in activeResultViewController.items) {
      if (!item.value.isGroup) {
        return item.value.data.queryId;
      }
    }
    return null;
  }

  bool get hasVisibleResultsForCurrentQuery => getVisibleResultQueryId() == currentQuery.value.queryId;

  bool get hasVisibleStaleResultsDuringQueryTransition {
    return activeResultViewController.items.isNotEmpty && !isShowingPendingResultPlaceholder && !hasVisibleResultsForCurrentQuery;
  }

  void resetPendingResultPlaceholder() {
    isShowingPendingResultPlaceholder = false;
    pendingResultPlaceholderHeight = null;
  }

  void cancelPendingResultTransitions() {
    clearQueryResultsTimer.cancel();
    resetPendingResultPlaceholder();
  }

  void clearStaleResultsForLayoutTransition(String traceId) {
    if (isCurrentQueryReturned || currentQuery.value.isEmpty || !hasVisibleStaleResultsDuringQueryTransition) {
      return;
    }

    // Bug fix: the stale-results grace window is only safe while the result
    // layout keeps the same meaning. When metadata switches list/grid before
    // slow plugin results arrive, the old snapshot would be re-rendered in the
    // new layout and look like fresh plugin content. Reuse the pending
    // placeholder path so outdated items disappear immediately while the
    // window height remains stable until real results replace them.
    clearQueryResultsTimer.cancel();
    showPendingResultPlaceholder(traceId);
  }

  // Keep a temporary "empty but still tall" transition state while a new query
  // is waiting for results. We intentionally clear stale items, actions, and
  // preview immediately so the UI no longer exposes outdated content, but we
  // preserve the last committed height to avoid a shrink-then-expand flash
  // during fast typing. This is different from the removed shrink debounce:
  // once real results arrive, resizeHeightForResultUpdate now applies the new
  // height immediately instead of waiting for a settle timer.
  void showPendingResultPlaceholder(String traceId) {
    if (isClosed || currentQuery.value.isEmpty || isCurrentQueryReturned) {
      return;
    }

    // Reuse the most recently committed window height so the placeholder keeps
    // the launcher geometry stable until the next snapshot replaces it.
    pendingResultPlaceholderHeight ??= committedWindowHeight ?? calculateWindowHeight();
    isShowingPendingResultPlaceholder = true;

    Logger.instance.debug(traceId, "show pending result placeholder, preservedHeight=$pendingResultPlaceholderHeight");

    resultListViewController.clearItems();
    resultGridViewController.clearItems();
    actionListViewController.clearItems();
    isShowPreviewPanel.value = false;
    isShowActionPanel.value = false;
    syncPreviewFullscreenState();
    refreshToolbarActionsForCurrentState(traceId);
  }

  Future<void> resizeHeightForResultUpdate({required String traceId, required String reason}) async {
    // Bug fix: result updates used to delay shrink while still applying growth
    // immediately. That made the window height lag behind the current result
    // snapshot, so result-driven resize now always applies the latest target
    // height right away. The pending-result placeholder above still handles the
    // separate cross-query flicker case without reintroducing shrink debounce.
    final targetHeight = calculateWindowHeight();
    await resizeHeight(traceId: traceId, reason: reason, overrideTargetHeight: targetHeight);
  }

  /// Triggered when received query results from the server.
  Future<bool> onReceivedQueryResults(String traceId, String queryId, List<WoxQueryResult> receivedResults, {required bool isFinal, int? backendQueryStartTimestampMs}) async {
    final tracker = WoxTimeTracker.start(traceId, "ui_query_result_apply");
    final totalStartUs = tracker.checkpointUs();
    tracker.setRawString("queryId", queryId);
    tracker.setInt("resultCount", receivedResults.length);
    tracker.setBool("isFinal", isFinal);

    // Cancel loading timer and hide loading animation when results are received
    if (queryId == currentQuery.value.queryId) {
      final stateStartUs = tracker.checkpointUs();
      if (receivedResults.isNotEmpty || isFinal) {
        clearQueryResultsTimer.cancel();
        resetPendingResultPlaceholder();
      }

      if (receivedResults.isNotEmpty || isFinal) {
        isCurrentQueryReturned = true;
      }

      if (receivedResults.isNotEmpty || isFinal) {
        loadingTimer?.cancel();
        isLoading.value = false;
      }
      tracker.setElapsedUs("queryStateUs", stateStartUs);
    } else {
      Logger.instance.error(traceId, "query id is not matched, ignore the results");
      tracker.setBool("staleQuery", true);
      tracker.setElapsedUs("totalUs", totalStartUs);
      tracker.log();
      return false;
    }

    if (receivedResults.isEmpty && !isFinal) {
      Logger.instance.debug(traceId, "ignore non-final empty query results");
      tracker.setBool("ignoredEmptyNonFinal", true);
      tracker.setElapsedUs("totalUs", totalStartUs);
      tracker.log();
      return false;
    }

    if (receivedResults.isEmpty) {
      final emptyApplyStartUs = tracker.checkpointUs();
      // Empty responses must clear stale items from the previous query state,
      // otherwise plugin-scoped toolbar messages cannot be shown without results.
      resultListViewController.clearItems();
      resultGridViewController.clearItems();
      actionListViewController.clearItems();
      isShowPreviewPanel.value = false;
      isShowActionPanel.value = false;
      syncPreviewFullscreenState();
      refreshToolbarActionsForCurrentState(traceId);
      // Bug fix: empty terminal snapshots used to wait until the next frame
      // before shrinking, which still let the old geometry flash once. Resize
      // in the same async flow so the empty state is committed immediately.
      tracker.setElapsedUs("emptyApplyUs", emptyApplyStartUs);
      final resizeStartUs = tracker.checkpointUs();
      await resizeHeightForResultUpdate(traceId: traceId, reason: "empty query results received");
      tracker.setElapsedUs("resizeUs", resizeStartUs);
      tracker.setElapsedUs("totalUs", totalStartUs);
      tracker.log();
      return true;
    }

    // 1. Use silent mode to avoid triggering onItemActive callback during updateItems (which may cause a little performance issue)
    //    Following resetActiveResult in updateActiveResultIndex will trigger the callback
    // 2. We need update items in both list and grid controllers, because metdata query (grid and list layout change relay on this) may after results arrival,
    //    at this point, we don't know which layout this query will use, so we update both
    final appendTailStartUs = tracker.checkpointUs();
    final displayResults = appendOnReceivedPerformanceTailForQuery(traceId, queryId, receivedResults, backendQueryStartTimestampMs: backendQueryStartTimestampMs);
    tracker.setElapsedUs("appendTailUs", appendTailStartUs);
    final listItemStartUs = tracker.checkpointUs();
    final listItems = displayResults.map((e) => WoxListItem.fromQueryResult(e)).toList();
    tracker.setElapsedUs("listItemMapUs", listItemStartUs);

    final listUpdateStartUs = tracker.checkpointUs();
    resultListViewController.updateItems(traceId, listItems, silent: true);
    tracker.setElapsedUs("listUpdateUs", listUpdateStartUs);
    final gridUpdateStartUs = tracker.checkpointUs();
    resultGridViewController.updateItems(traceId, listItems, silent: true);
    tracker.setElapsedUs("gridUpdateUs", gridUpdateStartUs);

    final activeIndexStartUs = tracker.checkpointUs();
    updateActiveResultIndex(traceId);
    tracker.setElapsedUs("activeIndexUs", activeIndexStartUs);
    final toolbarStartUs = tracker.checkpointUs();
    updateDoctorToolbarIfNeeded(traceId);
    tracker.setElapsedUs("toolbarUs", toolbarStartUs);

    final resizeStartUs = tracker.checkpointUs();
    await resizeHeightForResultUpdate(traceId: traceId, reason: "query results updated");
    tracker.setElapsedUs("resizeUs", resizeStartUs);
    tracker.setElapsedUs("totalUs", totalStartUs);
    tracker.log();
    return true;
  }

  // Re-attach the UI-only onReceived tail when a later backend batch replaces the result list.
  List<WoxQueryResult> appendOnReceivedPerformanceTailForQuery(String traceId, String queryId, List<WoxQueryResult> results, {int? backendQueryStartTimestampMs}) {
    final setting = WoxSettingUtil.instance.currentSetting;
    if (!Env.isDev || !setting.showPerformanceTail || !setting.showPerformanceTailUiReceived) {
      return results;
    }

    // Prefer the backend query start so this UI tail is comparable with the
    // backend "response received" tail, which uses queryRun.startTimestamp.
    final queryStartTime = backendQueryStartTimestampMs != null && backendQueryStartTimestampMs > 0 ? backendQueryStartTimestampMs : queryStartTimeMap[traceId];
    if (queryStartTime == null) {
      return results;
    }
    final currentOnReceivedElapsed = DateTime.now().millisecondsSinceEpoch - queryStartTime;

    for (final result in results) {
      final resultKey = getOnReceivedResultKey(queryId, result);
      final onReceivedElapsed = resultKey.isEmpty ? currentOnReceivedElapsed : queryOnReceivedElapsedByResultKey.putIfAbsent(resultKey, () => currentOnReceivedElapsed);
      appendOnReceivedPerformanceTail(result, onReceivedElapsed);
    }
    return results;
  }

  String getOnReceivedResultKey(String queryId, WoxQueryResult result) {
    if (result.id.isEmpty) {
      return "";
    }
    return "$queryId:${result.id}";
  }

  // Appends the onReceived tail once while preserving all backend-provided tails.
  void appendOnReceivedPerformanceTail(WoxQueryResult result, int onReceivedElapsed) {
    if (result.isGroup) {
      return;
    }

    result.tails =
        result.tails.where((tail) => tail.tooltip != _onReceivedTailTooltip).toList()
          ..add(WoxListItemTail.text("${onReceivedElapsed}ms", textCategory: getOnReceivedTailTextCategory(onReceivedElapsed))..tooltip = _onReceivedTailTooltip);
  }

  // This dev metric stops at onReceivedQueryResults, before resize and frame paint.
  String getOnReceivedTailTextCategory(int onReceivedElapsed) {
    return woxListItemTailTextCategoryDefault;
  }

  void updateActiveResultIndex(String traceId) {
    final existingQueryResults = activeResultViewController.items.where((item) => item.value.data.queryId == currentQuery.value.queryId).map((e) => e.value.data).toList();

    // Handle index preservation or reset
    final controller = activeResultViewController;
    if (pendingPreservedIndex != null) {
      // Restore the preserved index
      final targetIndex = pendingPreservedIndex!;
      pendingPreservedIndex = null; // Clear the pending index

      // Ensure the index is within bounds
      if (targetIndex < controller.items.length) {
        // Skip group items - find the next non-group item
        var actualIndex = targetIndex;
        while (actualIndex < controller.items.length && controller.items[actualIndex].value.data.isGroup) {
          actualIndex++;
        }

        // If we found a valid non-group item, use it; otherwise reset to first
        if (actualIndex < controller.items.length) {
          controller.updateActiveIndex(traceId, actualIndex);
          Logger.instance.debug(traceId, "restored active index to: $actualIndex (original: $targetIndex)");
        } else {
          resetActiveResult();
          Logger.instance.debug(traceId, "could not restore index $targetIndex (all remaining items are groups), reset to first");
        }
      } else {
        resetActiveResult();
        Logger.instance.debug(traceId, "could not restore index $targetIndex (out of bounds), reset to first");
      }
    } else {
      // Normal behavior: if current query already has results and active result is not the first one, then do not reset active result and action
      // this will prevent the active result from being reset to the first one when the query results are received
      if (existingQueryResults.isEmpty || controller.activeIndex.value == 0) {
        resetActiveResult();
      }
    }
  }

  void clearDoctorToolbarIfApplied() {
    final currentText = toolbar.value.text ?? '';
    if (lastAppliedDoctorToolbarMessage.isNotEmpty && currentText == lastAppliedDoctorToolbarMessage) {
      toolbar.value = toolbar.value.emptyLeftSide();
    }
    lastAppliedDoctorToolbarMessage = '';
  }

  void updateDoctorToolbarIfNeeded(String traceId) {
    if (hasVisibleToolbarMsg) {
      clearDoctorToolbarIfApplied();
      return;
    }

    if (!currentQuery.value.isEmpty) {
      clearDoctorToolbarIfApplied();
      return;
    }

    if (doctorCheckInfo.value.allPassed) {
      clearDoctorToolbarIfApplied();
      return;
    }

    final currentText = toolbar.value.text ?? '';
    final canOverrideLeft = currentText.isEmpty || currentText == lastAppliedDoctorToolbarMessage;
    if (!canOverrideLeft) {
      return;
    }

    // Doctor warnings are global launcher issues. Keep their action visible even
    // when MRU/results are present so Enter resolves the warning instead of
    // acting on an unrelated result.
    if (activeResultViewController.items.isEmpty) {
      final updateAction = buildUpdateToolbarAction();
      final actions = <ToolbarActionInfo>[];
      if (updateAction != null) {
        actions.add(updateAction);
      }
      actions.add(buildDoctorToolbarAction());

      toolbar.value = ToolbarInfo(text: doctorCheckInfo.value.message, icon: doctorCheckInfo.value.icon, actions: actions);
    } else {
      toolbar.value = toolbar.value.copyWith(text: doctorCheckInfo.value.message, icon: doctorCheckInfo.value.icon, actions: buildToolbarActionsForCurrentState(getActiveResult()));
    }

    lastAppliedDoctorToolbarMessage = doctorCheckInfo.value.message;
  }

  void openUpdateFromToolbar(String traceId) {
    onQueryChanged(traceId, PlainQuery.text("update "), "toolbar go to update");
  }

  void openDoctorFromToolbar(String traceId) {
    onQueryChanged(traceId, PlainQuery.text("doctor "), "toolbar go to doctor");
  }

  ToolbarActionInfo? buildUpdateToolbarAction() {
    if (!shouldShowUpdateActionInToolbar) {
      return null;
    }

    return ToolbarActionInfo(
      name: tr("plugin_doctor_go_to_update"),
      hotkey: WoxPlatformHotkeyUtil.primaryHotkey("u"),
      action: () {
        openUpdateFromToolbar(const UuidV4().generate());
      },
    );
  }

  ToolbarActionInfo buildDoctorToolbarAction() {
    return ToolbarActionInfo(
      name: tr("plugin_doctor_handle"),
      hotkey: WoxPlatformHotkeyUtil.primaryHotkey("enter"),
      action: () {
        openDoctorFromToolbar(const UuidV4().generate());
      },
    );
  }

  List<WoxResultAction> buildLocalActions() {
    final actions = <WoxResultAction>[];

    final updateAction = hasVisibleToolbarMsg ? null : buildUpdateToolbarAction();
    if (updateAction != null) {
      actions.add(
        WoxResultAction.local(
          id: localActionOpenUpdateId,
          name: updateAction.name,
          hotkey: updateAction.hotkey,
          emoji: "⬆️",
          handler: (traceId) {
            openUpdateFromToolbar(traceId);
            return true;
          },
        ),
      );
    }

    if (!hasVisibleToolbarMsg && currentQuery.value.isEmpty && !doctorCheckInfo.value.allPassed) {
      actions.add(
        WoxResultAction.local(
          id: localActionOpenDoctorId,
          name: tr("plugin_doctor_handle"),
          hotkey: WoxPlatformHotkeyUtil.primaryHotkey("enter"),
          icon: doctorCheckInfo.value.icon,
          // The primary-modifier Enter shortcut keeps Doctor handling available
          // without stealing the normal Enter default action from the selected result.
          handler: (traceId) {
            openDoctorFromToolbar(traceId);
            return true;
          },
        ),
      );
    }

    if (!isShowPreviewPanel.value) {
      return actions;
    }

    if (currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_FILE.code && isManualFilePreviewLoadAvailable.value) {
      actions.add(
        WoxResultAction.local(
          id: localActionLoadFilePreviewId,
          name: tr("ui_file_preview_load_full_preview"),
          hotkey: filePreviewLoadHotkey,
          emoji: "👁️",
          handler: requestManualFilePreviewLoad,
        ),
      );
    }

    if (currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code) {
      actions.add(
        WoxResultAction.local(
          id: localActionPreviewSearchId,
          name: tr("ui_action_preview_search"),
          hotkey: previewSearchHotkey,
          emoji: "🔎",
          handler: (_) => triggerTerminalFind(),
        ),
      );
    }

    if ((Platform.isMacOS || Platform.isWindows) && currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_WEBVIEW.code) {
      actions.add(
        WoxResultAction.local(
          id: localActionWebViewRefreshId,
          name: tr("ui_action_webview_refresh"),
          hotkey: previewRefreshHotkey,
          emoji: "🔄",
          handler: (_) {
            unawaited(WoxWebViewUtil.refresh());
            return true;
          },
        ),
      );
      actions.add(
        WoxResultAction.local(
          id: localActionWebViewBackId,
          name: tr("ui_action_webview_go_back"),
          hotkey: previewBackHotkey,
          emoji: "◀️",
          handler: (_) {
            unawaited(WoxWebViewUtil.goBack());
            return true;
          },
        ),
      );
      actions.add(
        WoxResultAction.local(
          id: localActionWebViewForwardId,
          name: tr("ui_action_webview_go_forward"),
          hotkey: previewForwardHotkey,
          emoji: "▶️",
          handler: (_) {
            unawaited(WoxWebViewUtil.goForward());
            return true;
          },
        ),
      );
      actions.add(
        WoxResultAction.local(
          id: localActionOpenWebViewInspectorId,
          name: tr("ui_action_webview_open_inspector"),
          hotkey: previewInspectorHotkey,
          emoji: "🧰",
          handler: (traceId) {
            // Log the native result because macOS WebKit can reject programmatic inspector opening without
            // throwing through MethodChannel. The previous fire-and-forget call made that failure invisible.
            unawaited(
              WoxWebViewUtil.openInspector()
                  .then((opened) => Logger.instance.debug(traceId, "open webview inspector result: $opened"))
                  .catchError((err, stack) => Logger.instance.error(traceId, "open webview inspector failed: $err")),
            );
            return true;
          },
        ),
      );
      actions.add(
        WoxResultAction.local(
          id: localActionWebViewClearStateId,
          name: tr("ui_action_webview_clear_state"),
          hotkey: "",
          emoji: "🧹",
          handler: (_) {
            unawaited(WoxWebViewUtil.clearState());
            return true;
          },
        ),
      );
    }

    if (supportsPreviewFullscreen(currentPreview.value)) {
      actions.add(
        WoxResultAction.local(
          id: localActionTogglePreviewFullscreenId,
          name: isPreviewFullscreen.value ? tr("ui_action_exit_fullscreen") : tr("ui_action_toggle_fullscreen"),
          hotkey: previewFullscreenHotkey,
          emoji: isPreviewFullscreen.value ? "🗗" : "🗖",
          handler: (traceId) => togglePreviewFullscreen(traceId),
        ),
      );
    }

    return actions;
  }

  List<WoxResultAction> buildToolbarMsgActions() {
    if (!hasVisibleToolbarMsg) {
      return [];
    }

    return toolbarMsg.value.actions
        .map(
          (action) => WoxResultAction.local(
            id: 'toolbar-msg:${toolbarMsg.value.id}:${action.id}',
            name: action.name,
            hotkey: action.hotkey,
            icon: action.icon,
            emoji: action.icon == null ? '⏳' : null,
            preventHideAfterAction: action.preventHideAfterAction,
            isDefault: action.isDefault,
            handler: (traceId) {
              WoxWebsocketMsgUtil.instance.sendMessage(
                WoxWebsocketMsg(
                  requestId: const UuidV4().generate(),
                  traceId: traceId,
                  type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
                  method: WoxMsgMethodEnum.WOX_MSG_METHOD_TOOLBAR_MSG_ACTION.code,
                  data: {'toolbarMsgId': toolbarMsg.value.id, 'actionId': action.id},
                ),
              );
              return true;
            },
          ),
        )
        .toList();
  }

  List<WoxResultAction> buildUnifiedActions(String traceId, WoxQueryResult? activeResult) {
    final localActions = buildLocalActions();
    final toolbarMsgActions = buildToolbarMsgActions();
    if (activeResult == null || activeResult.isGroup) {
      return [...localActions, ...toolbarMsgActions];
    }

    final pluginActions = buildResultActionsForCurrentState(activeResult, localActions: localActions, toolbarMsgActions: toolbarMsgActions);
    return [...localActions, ...toolbarMsgActions, ...pluginActions];
  }

  List<WoxResultAction> buildResultActionsForCurrentState(
    WoxQueryResult activeResult, {
    required List<WoxResultAction> localActions,
    required List<WoxResultAction> toolbarMsgActions,
  }) {
    final reservedHotkeys = localActions.where((action) => action.hotkey.isNotEmpty).map((action) => action.hotkey.toLowerCase()).toSet();
    reservedHotkeys.addAll(toolbarMsgActions.where((action) => action.hotkey.isNotEmpty).map((action) => action.hotkey.toLowerCase()));

    return activeResult.actions.map((action) {
      if (action.hotkey.isEmpty) {
        return action.copyWith();
      }
      final conflicted = reservedHotkeys.contains(action.hotkey.toLowerCase());
      return conflicted ? action.copyWith(hotkey: "") : action.copyWith();
    }).toList();
  }

  WoxResultAction? getLocalActionByHotkey(HotKey hotkey, {Set<String>? allowedActionIds}) {
    final localActions = buildLocalActions();
    for (final action in localActions) {
      if (allowedActionIds != null && !allowedActionIds.contains(action.id)) {
        continue;
      }

      final parsed = WoxHotkey.parseHotkeyFromString(action.hotkey);
      if (parsed == null || !parsed.isNormalHotkey) {
        continue;
      }

      if (WoxHotkey.equals(parsed.normalHotkey, hotkey)) {
        return action;
      }
    }

    return null;
  }

  bool executeLocalActionByHotkey(String traceId, HotKey hotkey, {Set<String>? allowedActionIds}) {
    final action = getLocalActionByHotkey(hotkey, allowedActionIds: allowedActionIds);
    if (action == null) {
      return false;
    }

    return action.runLocalAction(traceId);
  }

  // Keeps the load-preview toolbar action owned by the currently mounted
  // deferred preview so stale preview widgets cannot clear a newer action.
  void updateManualFilePreviewLoadAvailability(String traceId, String previewKey, bool available) {
    final normalizedKey = previewKey.trim();
    if (normalizedKey.isEmpty) {
      return;
    }

    if (available) {
      if (isManualFilePreviewLoadAvailable.value && _manualFilePreviewLoadAvailabilityKey == normalizedKey) {
        return;
      }
      _manualFilePreviewLoadAvailabilityKey = normalizedKey;
      isManualFilePreviewLoadAvailable.value = true;
      refreshToolbarActionsForCurrentState(traceId);
      return;
    }

    if (_manualFilePreviewLoadAvailabilityKey != normalizedKey) {
      return;
    }

    _manualFilePreviewLoadAvailabilityKey = "";
    isManualFilePreviewLoadAvailable.value = false;
    refreshToolbarActionsForCurrentState(traceId);
  }

  // Emits a load request back into the visible deferred preview widget.
  bool requestManualFilePreviewLoad(String traceId) {
    if (!isManualFilePreviewLoadAvailable.value) {
      return false;
    }

    _manualFilePreviewLoadRequests.add(traceId);
    return true;
  }

  List<ToolbarActionInfo> buildToolbarActionsForCurrentState(WoxQueryResult? activeResult) {
    final localActions = buildLocalActions();
    final toolbarMsgActions = buildToolbarMsgActions();

    List<WoxResultAction> orderedActions;
    if (activeResult == null || activeResult.isGroup) {
      orderedActions = [...localActions, ...toolbarMsgActions];
    } else {
      final resultActions = buildResultActionsForCurrentState(activeResult, localActions: localActions, toolbarMsgActions: toolbarMsgActions);
      final showLocalActionsOnToolbarRight = hasVisibleToolbarMsg || (currentQuery.value.isEmpty && !doctorCheckInfo.value.allPassed);
      orderedActions = showLocalActionsOnToolbarRight ? [...resultActions, ...localActions, ...toolbarMsgActions] : [...localActions, ...resultActions];
    }

    final toolbarActions = orderedActions.where((action) => action.hotkey.isNotEmpty).map((action) => ToolbarActionInfo(name: tr(action.name), hotkey: action.hotkey)).toList();

    if (orderedActions.isNotEmpty) {
      toolbarActions.add(ToolbarActionInfo(name: tr("toolbar_more_actions"), hotkey: moreActionsHotkey));
    }

    return toolbarActions;
  }

  void refreshActionsForActiveResult(String traceId, {required bool preserveSelection}) {
    final activeResult = getActiveResult();
    if (activeResult == null || activeResult.isGroup) {
      final actions = buildUnifiedActions(traceId, null);
      if (actions.isEmpty) {
        actionListViewController.clearItems();
        refreshToolbarActionsForCurrentState(traceId);
        return;
      }

      final oldActionName = preserveSelection ? getCurrentActionName() : null;
      final actionItems = actions.map((e) => WoxListItem.fromResultAction(e)).toList();
      actionListViewController.updateItems(traceId, actionItems);
      if (actionItems.isNotEmpty) {
        final newActiveIndex = calculatePreservedActionIndex(oldActionName);
        if (newActiveIndex >= 0 && newActiveIndex < actionItems.length && actionListViewController.activeIndex.value != newActiveIndex) {
          actionListViewController.updateActiveIndex(traceId, newActiveIndex);
        }
      }

      updateToolbarWithActions(traceId);
      return;
    }

    final oldActionName = preserveSelection ? getCurrentActionName() : null;
    final actions = buildUnifiedActions(traceId, activeResult);
    final actionItems = actions.map((e) => WoxListItem.fromResultAction(e)).toList();
    actionListViewController.updateItems(traceId, actionItems);

    if (actionItems.isNotEmpty) {
      final newActiveIndex = calculatePreservedActionIndex(oldActionName);
      if (newActiveIndex >= 0 && newActiveIndex < actionItems.length && actionListViewController.activeIndex.value != newActiveIndex) {
        actionListViewController.updateActiveIndex(traceId, newActiveIndex);
      }
    }

    updateToolbarWithActions(traceId);
  }

  Future<void> toggleApp(String traceId, ShowAppParams params) async {
    final screenshotController = Get.find<WoxScreenshotController>();
    if (screenshotController.isSessionActive.value) {
      final wasVisible = await windowManager.isVisible();
      // Screenshot capture owns the shared Wox window. Running the normal launcher toggle while
      // that session is active can hide the screenshot workspace without finishing the pending
      // CaptureScreenshot request, leaving the UI invisible until the backend times out. Treat the
      // hotkey as "cancel screenshot first" so the session-specific restore path can recover safely.
      await screenshotController.cancelSession(traceId, reason: 'launcher_toggle_app');
      if (!wasVisible) {
        final isVisibleAfterCancel = await windowManager.isVisible();
        if (!isVisibleAfterCancel) {
          await showApp(traceId, params);
        }
      }
      return;
    }

    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      if (isInSettingView.value) {
        exitSetting(traceId);
      } else if (isInOnboardingView.value) {
        hideApp(traceId);
      } else {
        hideApp(traceId);
      }
    } else {
      showApp(traceId, params);
    }
  }

  // Native Wayland compositors own top-level placement, so Wox should not send absolute window coordinates there.
  // Detect this from the actual GTK/GDK backend instead of environment variables alone: a Wayland
  // session often exposes DISPLAY for XWayland, and a forced X11 backend may still inherit
  // WAYLAND_DISPLAY. Actual X11 backends keep the precise XRandR placement path enabled.
  bool _isLinuxNativeWaylandBackend(Map<String, dynamic> backendInfo) {
    final backendFields = [
      backendInfo["backend"],
      backendInfo["displayType"],
      backendInfo["windowType"],
      backendInfo["display"],
    ].map((value) => value?.toString().toLowerCase() ?? "");
    final isX11Value = backendInfo["isX11"];
    final isX11 = isX11Value == true || isX11Value.toString().toLowerCase() == "true" || backendFields.any((value) => value.contains("x11"));
    if (isX11) {
      return false;
    }

    final waylandDisplay = backendInfo["waylandDisplayEnv"]?.toString() ?? "";
    return waylandDisplay.isNotEmpty || backendFields.any((value) => value.contains("wayland"));
  }

  Future<void> showApp(String traceId, ShowAppParams params) async {
    hiddenCacheClearTimer?.cancel();

    // Restore image cache capacity so images can be cached again while the
    // launcher is visible. hideApp shrinks it to zero. Values match the
    // desktop-tuned defaults set in main.dart (200 entries / 20 MB).
    PaintingBinding.instance.imageCache.maximumSize = 200;
    PaintingBinding.instance.imageCache.maximumSizeBytes = 20 * 1024 * 1024;

    final screenshotController = Get.find<WoxScreenshotController>();
    if (screenshotController.isSessionActive.value) {
      // Screenshot completion/cancel restore decides whether the shared window should stay visible.
      // Showing the launcher before that cleanup finishes mixes launcher focus/layout state into the
      // screenshot editor and can strand the session. Cancel first, then only continue if the
      // restore path intentionally kept the window hidden.
      await screenshotController.cancelSession(traceId, reason: 'launcher_show_app');
      final isVisibleAfterCancel = await windowManager.isVisible();
      if (isVisibleAfterCancel) {
        return;
      }
    }
    if (isInOnboardingView.value) {
      // Showing the launcher from a hotkey or the final onboarding action must
      // leave the guide state first; otherwise build routing would keep the
      // management page mounted over fresh query results.
      isInOnboardingView.value = false;
      await WoxApi.instance.onOnboarding(traceId, false);
    }
    if (isInSettingView.value) {
      // Bug fix: the native window can become hidden while the settings route is
      // still mounted, for example after a management-view blur or a platform
      // hide outside hideApp(). A later launcher show must clear that stale route;
      // otherwise the window reopens as settings instead of the query UI.
      isSettingOpenedFromHidden = false;
      isInSettingView.value = false;
      final settingController = Get.find<WoxSettingController>();
      settingController.clearSettingSearch();
      settingController.settingFocusNode.unfocus();
      settingController.settingSearchFocusNode.unfocus();
      await WoxApi.instance.onSetting(traceId, false);
    }

    // update some properties to latest for later use
    latestQueryHistories.assignAll(params.queryHistories);
    lastLaunchMode = params.launchMode;
    lastStartPage = params.startPage;
    updateAttentionUnreadCount(traceId, params.attentionUnreadCount);
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      canArrowUpHistory = true;
      if (lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code) {
        //skip the first one, because it's the current query
        currentQueryHistoryIndex = 0;
      } else {
        currentQueryHistoryIndex = -1;
      }
    }

    // Query preservation has two different sources and they solve different cases:
    // 1. Explicit incoming query source: query hotkey, selection query, tray query, and explorer type-to-search inject a
    //    new query for this show action, so fresh mode must preserve that incoming query.
    // 2. Continue-mode fallback: when show source is default, reopening the launcher in continue
    //    mode should keep the existing query already stored in the controller.
    //
    // We must detect input and selection queries separately here because their valid payloads are
    // different:
    // - Input query: only preserve when there is actual text. Empty input should fall back to MRU
    //   or blank start page instead of being treated as an existing query.
    // - Selection query: preserve by query type, not by queryText, because a valid selection query
    //   may carry its real payload in querySelection while queryText stays empty.
    //
    // This split is why show source was introduced: without it, fresh mode cannot distinguish a
    // newly injected query for this show action from stale query state left from the previous show.
    final hasCurrentInputQuery = currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code && currentQuery.value.queryText.isNotEmpty;
    final hasCurrentSelectionQuery = currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code;
    final shouldPreserveIncomingQuery =
        params.showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_QUERY_HOTKEY.code ||
        params.showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_SELECTION.code ||
        params.showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_TRAY_QUERY.code ||
        params.showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_EXPLORER.code;
    final shouldPreserveQueryOnShow =
        shouldPreserveIncomingQuery || (lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code && (hasCurrentInputQuery || hasCurrentSelectionQuery));

    if (lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code) {
      if (!shouldPreserveQueryOnShow) {
        currentQuery.value = PlainQuery.emptyInput();
        queryBoxTextFieldController.clear();
      }
    }

    // Handle start page when the current show action does not carry a query into the launcher.
    if (!shouldPreserveQueryOnShow) {
      pendingRestoredQueryId = null;
      pendingRestoredQueryWindowHeight = null;
      if (lastStartPage == WoxStartPageEnum.WOX_START_PAGE_MRU.code) {
        queryMRU(traceId);
      } else {
        // Blank page - clear results
        await clearQueryResults(traceId);
      }
    }

    resetLayoutState(traceId);
    isQueryBoxVisible.value = !params.hideQueryBox;
    isToolbarHiddenForce.value = params.hideToolbar;
    isQueryBoxAtBottom.value = params.queryBoxAtBottom;
    forceWindowWidth = params.windowWidth > 0 ? params.windowWidth.toDouble() : 0;
    forceMaxResultCount = params.maxResultCount;
    forceHideOnBlur = params.hideOnBlur;

    final targetHeight = calculateInitialShowWindowHeight(shouldPreserveIncomingQuery);
    final targetWidth = forceWindowWidth != 0 ? forceWindowWidth : WoxSettingUtil.instance.currentSetting.appWidth.toDouble();
    final targetSize = Size(targetWidth, targetHeight);
    final targetPosition = resolveShowAppPosition(params, targetWidth, targetHeight);
    final initialShowResizeToken = resizeRequestToken;
    Map<String, dynamic> linuxBackendInfo = <String, dynamic>{};
    var skipAbsolutePosition = false;
    var layerShellPlacement = false;
    var initialShowSizeApplied = false;
    if (Platform.isLinux) {
      Logger.instance.info(
        traceId,
        "linux-window-bounds dart stage=showApp-target position=${targetPosition.dx},${targetPosition.dy} size=${targetWidth}x$targetHeight source=${params.showSource} positionType=${params.position.type}",
      );
    }

    // Linux native Wayland does not allow reliable absolute top-level placement.
    // When the compositor supports wlr-layer-shell (Hyprland/sway), apply the
    // placement before show so the surface is mapped at the right position from
    // the start. Otherwise show first so the compositor chooses the active
    // monitor, then only apply the requested size. Exact centered placement on
    // Wayland without layer-shell needs compositor-specific protocols a regular
    // GTK client cannot do portably.
    if (Platform.isLinux) {
      linuxBackendInfo = await LinuxWindowManager.instance.getBackendInfo();
      skipAbsolutePosition = _isLinuxNativeWaylandBackend(linuxBackendInfo);
      final layerShellSupported = linuxBackendInfo["supportsLayerShell"] == true || linuxBackendInfo["supportsLayerShell"].toString().toLowerCase() == "true";
      layerShellPlacement = skipAbsolutePosition && layerShellSupported;
      Logger.instance.info(traceId, "linux-window-bounds dart stage=backend-info $linuxBackendInfo skipAbsolutePosition=$skipAbsolutePosition layerShell=$layerShellPlacement");
    }

    // Linux Wayland query results can arrive while showApp is still waiting for
    // backend info. Keep that newer resize authoritative instead of applying
    // the stale initial show height afterward.
    final initialShowSizeSuperseded = Platform.isLinux && resizeRequestToken != initialShowResizeToken;
    if (!initialShowSizeSuperseded) {
      if (layerShellPlacement) {
        // wlr-layer-shell lets Wox anchor to the top-left of a chosen output and
        // set margins, restoring exact launcher placement on Hyprland/sway while
        // still keeping the window above normal surfaces and out of the taskbar.
        // Apply placement before show so the compositor maps the surface at the
        // correct position and monitor from the start, avoiding a visible jump.
        await LinuxWindowManager.instance.applyLayerShellPlacement(targetPosition, targetSize);
        initialShowSizeApplied = true;
        Logger.instance.info(
          traceId,
          "linux-window-bounds dart stage=layer-shell-placement positionType=${params.position.type} target=${targetPosition.dx},${targetPosition.dy} ${targetWidth}x$targetHeight",
        );
      } else if (skipAbsolutePosition) {
        await windowManager.show();
        await windowManager.setSize(targetSize);
        initialShowSizeApplied = true;
        final actualSize = await windowManager.getSize();
        Logger.instance.info(
          traceId,
          "linux-window-bounds dart stage=skip-setBounds reason=native-wayland-compositor-placement positionType=${params.position.type} targetSize=${targetWidth}x$targetHeight actualSize=${actualSize.width}x${actualSize.height}",
        );
      } else {
        // Apply position+size together before showing to avoid opening with stale width.
        await windowManager.setBounds(targetPosition, targetSize);
        initialShowSizeApplied = true;
        if (Platform.isLinux) {
          final actualPosition = await windowManager.getPosition();
          final actualSize = await windowManager.getSize();
          Logger.instance.info(
            traceId,
            "linux-window-bounds dart stage=after-setBounds target=${targetPosition.dx},${targetPosition.dy} ${targetWidth}x$targetHeight actual=${actualPosition.dx},${actualPosition.dy} ${actualSize.width}x${actualSize.height}",
          );
        }
      }
    }
    if (initialShowSizeApplied || !Platform.isLinux) {
      committedWindowHeight = targetHeight;
    }

    // Set always-on-top BEFORE show() so the TOPMOST flag is already in place
    // when the window becomes visible, avoiding transient blur on Windows.
    if (!isInSettingView.value) {
      await windowManager.setAlwaysOnTop(true);
    }
    if (Platform.isWindows) {
      Logger.instance.debug(traceId, "windows showApp before native show/focus");
    }
    await windowManager.show();
    final visibleActivationCost = _captureDevLauncherVisibleActivationCost(params);
    await windowManager.focus();
    if (Platform.isWindows) {
      Logger.instance.debug(traceId, "windows showApp after initial native focus");
    }

    // Workaround for Windows DWM Acrylic bug:
    // When resizing a hidden window to its full height, DWM caching fails to compose the transparent alpha channel upon showing.
    // We must manually trigger a resize event *after* the window is fully visible to force DWM recomposition.
    // Note: We cannot achieve this simply by firing SetWindowPos with SWP_FRAMECHANGED in native C++ during WM_SHOWWINDOW.
    // Because of the timing difference between DWM and Flutter's DirectX Swapchain, an empty native window update gets
    // immediately overwritten/ignored by the Flutter engine if the logical window height remains unchanged.
    // Thus, we physically alter the height by 1 pixel via Dart in a delayed manner to strictly ensure a new valid frame is painted.
    if (Platform.isWindows) {
      Future.delayed(const Duration(milliseconds: 25), () {
        if (!isClosed) {
          unawaited(resizeHeight(traceId: traceId, reason: "force DWM recomposition after showing window", forceDwmRecomposition: true));
        }
      });
    }

    unawaited(_focusQueryBoxAfterLauncherShow(traceId: traceId, selectAll: params.selectAll));
    unawaited(_showDevLauncherActivationWarningIfSlow(traceId, visibleActivationCost));

    if (params.isQueryFocus) {
      Logger.instance.debug(traceId, "need to auto focus to chat input on show app (query focus)");
      if (isShowPreviewPanel.value && currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code) {
        final chatController = Get.find<WoxAIChatController>();
        chatController.focusToChatInput(traceId);
        enterPreviewFullscreen(traceId);
      }
    }

    WoxApi.instance.onShow(traceId);
    unawaited(refreshGlance(traceId, "windowShown"));
  }

  void resetLayoutState(String traceId) {
    isQueryBoxAtBottom.value = false;
    isQueryBoxVisible.value = true;
    isToolbarHiddenForce.value = false;
    forceWindowWidth = 0;
    forceMaxResultCount = 0;
    forceHideOnBlur = false;
  }

  Offset resolveShowAppPosition(ShowAppParams params, double targetWidth, double targetHeight) {
    final trayAnchor = params.trayAnchor;
    if (!Platform.isWindows || trayAnchor == null || params.showSource != WoxShowSourceEnum.WOX_SHOW_SOURCE_TRAY_QUERY.code) {
      return Offset(params.position.x.toDouble(), params.position.y.toDouble());
    }

    const double margin = 10;
    final minX = trayAnchor.screenRect.x + margin;
    final maxX = trayAnchor.screenRect.x + trayAnchor.screenRect.width - targetWidth - margin;
    final resolvedX = maxX < minX ? minX : (trayAnchor.windowX.toDouble().clamp(minX, maxX) as num).toDouble();

    final minY = trayAnchor.screenRect.y + margin;
    final rawY = trayAnchor.bottom - targetHeight;
    final unclampedMaxY = trayAnchor.screenRect.y + trayAnchor.screenRect.height - targetHeight - margin;
    final maxY = unclampedMaxY < minY ? minY : unclampedMaxY;
    final resolvedY = (rawY.clamp(minY, maxY) as num).toDouble();

    return Offset(resolvedX, resolvedY);
  }

  int getMaxResultCount() {
    // Allow show-app callers such as tray query and query hotkey to override
    // the global max result count for a specific launcher session.
    if (forceMaxResultCount > 0) {
      return forceMaxResultCount;
    }

    final configuredCount = WoxSettingUtil.instance.currentSetting.maxResultCount;
    return configuredCount > 0 ? configuredCount : MAX_LIST_VIEW_ITEM_COUNT;
  }

  double getMaxResultListViewHeight() {
    return WoxThemeUtil.instance.getResultListViewHeightByCount(getMaxResultCount());
  }

  double getMaxResultContainerHeight() {
    return getMaxResultListViewHeight() +
        WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingTop +
        WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingBottom;
  }

  PlainQuery cloneQuery(PlainQuery query, {String? queryId, Map<String, String>? queryRefinements, Map<String, String>? contextData}) {
    return PlainQuery(
      queryId: queryId ?? query.queryId,
      queryType: query.queryType,
      queryText: query.queryText,
      querySelection: Selection(type: query.querySelection.type, text: query.querySelection.text, filePaths: List<String>.from(query.querySelection.filePaths)),
      queryRefinements: cloneQueryRefinementPayload(queryRefinements ?? query.queryRefinements),
      contextData: cloneQueryContextData(contextData ?? query.contextData),
    );
  }

  bool shouldRestoreQueryAfterHide(String showSource) {
    return showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_TRAY_QUERY.code ||
        showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_QUERY_HOTKEY.code ||
        showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_SELECTION.code ||
        showSource == WoxShowSourceEnum.WOX_SHOW_SOURCE_EXPLORER.code;
  }

  void preserveQueryBeforeTemporaryQuery(String traceId, String showSource) {
    if (queryBeforeTemporaryQuery != null) {
      return;
    }

    final query = currentQuery.value;
    if (query.isEmpty) {
      queryBeforeTemporaryQuery = PlainQuery.emptyInput();
    } else {
      queryBeforeTemporaryQuery = cloneQuery(query);
    }
    queryBeforeTemporaryQuerySource = showSource;
    windowHeightBeforeTemporaryQuery = calculateWindowHeight();

    Logger.instance.debug(traceId, "preserve current query before temporary query($showSource): ${queryBeforeTemporaryQuery!.queryText}");
  }

  Future<void> restoreQueryAfterTemporaryQuery(String traceId) async {
    final preservedQuery = queryBeforeTemporaryQuery;
    final preservedSource = queryBeforeTemporaryQuerySource;
    final preservedWindowHeight = windowHeightBeforeTemporaryQuery;
    queryBeforeTemporaryQuery = null;
    queryBeforeTemporaryQuerySource = null;
    windowHeightBeforeTemporaryQuery = null;
    if (preservedQuery == null) {
      return;
    }

    final restoredQuery = cloneQuery(preservedQuery, queryId: const UuidV4().generate());
    pendingRestoredQueryId = restoredQuery.queryId;
    pendingRestoredQueryWindowHeight = preservedWindowHeight;
    Logger.instance.debug(
      traceId,
      "restore preserved query after temporary query(${preservedSource ?? "unknown"}): ${restoredQuery.queryText}, preservedWindowHeight=$preservedWindowHeight",
    );
    await onQueryChanged(traceId, restoredQuery, "restore query after temporary query");
  }

  Future<void> hideApp(String traceId) async {
    final screenshotController = Get.find<WoxScreenshotController>();
    if (screenshotController.isSessionActive.value) {
      // Hiding the launcher while screenshot capture is active hides the capture workspace too
      // because both flows share the same window. The previous hide path left the screenshot
      // session running in the background, which made later toggles feel broken until the backend
      // CaptureScreenshot call eventually timed out. Cancel the screenshot instead of hiding it.
      await screenshotController.cancelSession(traceId, reason: 'launcher_hide_app');
      return;
    }

    // Bug fix: hide invalidates pending visible-launcher focus retries. Without
    // this guard, a retry scheduled by the previous show cycle can run after the
    // next window transition and unexpectedly re-select query text.
    _visibleLauncherFocusToken++;

    await saveWindowPositionNow(traceId, "before-hide");

    // hide first to avoid the potential delay caused by some heavy operations in onHide callback
    // E.g. on tray query mode, hideActionPanel will call resize height, which may cause a noticeable
    // resize animation if the window is still visible while resizing, so we hide the window first and then do the rest of the operations
    await windowManager.hide();

    //clear query box text if query type is selection or launch mode is fresh
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code || lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code) {
      currentQuery.value = PlainQuery.emptyInput();
      queryBoxTextFieldController.clear();
      await clearQueryResults(traceId);
    }

    hideActionPanel(traceId);
    hideFormActionPanel(traceId, reason: "hide app");
    glanceRefreshTimer?.cancel();
    glanceItems.clear();
    PaintingBinding.instance.imageCache.clear();
    PaintingBinding.instance.imageCache.clearLiveImages();
    // Shrink image cache capacity to zero so no new decoded images can accumulate
    // while the window is hidden. showApp restores the default capacity.
    PaintingBinding.instance.imageCache.maximumSize = 0;
    PaintingBinding.instance.imageCache.maximumSizeBytes = 0;
    scheduleHiddenCacheClear(traceId);

    // Clean up quick select state
    if (isQuickSelectMode.value) {
      deactivateQuickSelectMode(traceId);
    }
    quickSelectTimer?.cancel();
    isQuickSelectKeyPressed = false;
    final wasInSettingView = isInSettingView.value;
    isSettingOpenedFromHidden = false;
    isInSettingView.value = false;
    if (wasInSettingView) {
      // Bug fix: hideApp can close settings through blur/tray paths without
      // calling exitSetting. Clear the per-session search state here too so the
      // next open is not polluted by the hidden route.
      final settingController = Get.find<WoxSettingController>();
      settingController.clearSettingSearch();
      settingController.settingFocusNode.unfocus();
      settingController.settingSearchFocusNode.unfocus();
    }
    await WoxApi.instance.onSetting(traceId, false);
    isInOnboardingView.value = false;
    await WoxApi.instance.onOnboarding(traceId, false);
    resetLayoutState(traceId);

    // Release large reference lists that are lazily reloaded on next use.
    // installedPlugins is kept because launcher trigger-keyword detection
    // (hasLocalPluginTriggerMetadata) depends on it without an async reload.
    Get.find<WoxSettingController>().clearStoreCache();
    Get.find<WoxAIChatController>().clearReferenceDataCache();

    // Release the wallpaper image provider so its decoded bitmap is not held
    // while the window is hidden. The path cache is kept; only the image is
    // released. Skipped while settings is open because the theme editor may
    // still be rendering the wallpaper preview.
    WoxSystemWallpaperUtil.instance.releaseImageCache();

    // Close terminal preview stream controllers and clear per-query metric
    // maps so hidden-state memory does not retain them between sessions.
    for (final controller in terminalChunkControllers.values) {
      controller.close();
    }
    for (final controller in terminalStateControllers.values) {
      controller.close();
    }
    terminalChunkControllers.clear();
    terminalStateControllers.clear();
    queryStartTimeMap.clear();
    queryOnReceivedElapsedByResultKey.clear();

    await WoxApi.instance.onHide(traceId);
    await restoreQueryAfterTemporaryQuery(traceId);
  }

  Future<void> saveWindowPositionNow(String traceId, String reason) async {
    final setting = WoxSettingUtil.instance.currentSetting;
    if (setting.showPosition != WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code) {
      return;
    }

    try {
      Map<String, dynamic> backendInfo = <String, dynamic>{};
      if (Platform.isLinux) {
        backendInfo = await LinuxWindowManager.instance.getBackendInfo();
        if (_isLinuxNativeWaylandBackend(backendInfo)) {
          final layerShellSupported = backendInfo["supportsLayerShell"] == true || backendInfo["supportsLayerShell"].toString().toLowerCase() == "true";
          if (!layerShellSupported) {
            // Native Wayland positions are compositor-owned and gtk_window_get_position may return
            // synthetic or stale coordinates, so persisting them would poison last_location.
            Logger.instance.info(traceId, "linux-window-bounds dart stage=skip-save-last-location reason=$reason backendInfo=$backendInfo note=native-wayland-position-unreliable");
            return;
          }
          // With layer-shell the window is placed via explicit margins, so the
          // GTK-reported coordinates are meaningful and safe to persist.
        }
      }

      final position = await windowManager.getPosition();
      if (Platform.isLinux) {
        Logger.instance.info(traceId, "linux-window-bounds dart stage=save-last-location reason=$reason position=${position.dx},${position.dy} backendInfo=$backendInfo");
      }
      await WoxApi.instance.saveWindowPosition(traceId, position.dx.toInt(), position.dy.toInt());
    } catch (e) {
      Logger.instance.error(traceId, "Failed to save window position: $e");
    }
  }

  void saveWindowPositionIfNeeded({String reason = "delayed"}) {
    final setting = WoxSettingUtil.instance.currentSetting;
    if (setting.showPosition == WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code) {
      // Run in async task with delay to ensure window position is fully updated.
      Future.delayed(const Duration(milliseconds: 500), () async {
        final traceId = const UuidV4().generate();
        await saveWindowPositionNow(traceId, reason);
      });
    }
  }

  Future<void> toggleActionPanel(String traceId) async {
    if (activeResultViewController.items.isEmpty && buildToolbarMsgActions().isEmpty) {
      return;
    }

    if (isShowActionPanel.value) {
      hideActionPanel(traceId);
    } else {
      openActionPanelForActiveResult(traceId);
    }
  }

  bool isActionHotkey(HotKey hotkey) {
    final parsed = WoxHotkey.parseHotkeyFromString(moreActionsHotkey);
    if (parsed == null || !parsed.isNormalHotkey) {
      return false;
    }
    return WoxHotkey.equals(hotkey, parsed.normalHotkey);
  }

  String normalizeToolbarHotkey(String hotkey) {
    return hotkey.replaceAll(" ", "").toLowerCase();
  }

  void hideActionPanel(String traceId) {
    final wasShowActionPanel = isShowActionPanel.value;
    isShowActionPanel.value = false;
    actionListViewController.clearFilter(traceId);
    focusQueryBox();
    if (wasShowActionPanel) {
      resizeHeight(traceId: traceId, reason: "hide action panel");
    }
  }

  void showFormActionPanel(String traceId, WoxResultAction action, String resultId) {
    Logger.instance.debug(traceId, "show form action panel: action=${action.name}, resultId=$resultId, fieldCount=${action.form.length}");
    activeFormAction.value = action;
    activeFormResultId.value = resultId;
    formActionValues.clear();
    for (final item in action.form) {
      final key = (item.value as dynamic).key as String?;
      if (key != null) {
        final defaultValue = (item.value as dynamic).defaultValue as String? ?? "";
        formActionValues[key] = defaultValue;
      }
    }
    isShowFormActionPanel.value = true;
    isShowActionPanel.value = false;
    resizeHeight(traceId: traceId, reason: "show form action panel");
  }

  void hideFormActionPanel(String traceId, {String reason = "unspecified"}) {
    final wasShowFormActionPanel = isShowFormActionPanel.value;
    Logger.instance.debug(
      traceId,
      "hide form action panel: reason=$reason, isShow=${isShowFormActionPanel.value}, activeAction=${activeFormAction.value?.name ?? "null"}, resultId=${activeFormResultId.value}",
    );
    activeFormAction.value = null;
    activeFormResultId.value = "";
    formActionValues.clear();
    isShowFormActionPanel.value = false;
    focusQueryBox();
    if (wasShowFormActionPanel) {
      resizeHeight(traceId: traceId, reason: "hide form action panel: $reason");
    }
  }

  /// Focuses the active launcher keyboard target.
  /// Hidden-query preview mode has no editable query box, so it needs a stable
  /// launcher-level focus node to keep Escape and other global keys reachable.
  Future<void> focusLauncherKeyboardTarget({bool selectAll = false, bool Function()? shouldSelectAll, bool ensureNativeWindowFocus = false}) async {
    if (isQueryBoxVisible.value) {
      // Windows can briefly lose OS foreground after show; the delayed launcher focus retry
      // may need to reclaim native focus before asking Flutter's query box to accept input.
      if (ensureNativeWindowFocus && Platform.isWindows) {
        final isVisible = await windowManager.isVisible();
        if (!isVisible) {
          return;
        }
        await windowManager.focus();
      }
      await focusQueryBox(selectAll: selectAll, shouldSelectAll: shouldSelectAll);
      return;
    }

    final screenshotController = Get.find<WoxScreenshotController>();
    if (screenshotController.isSessionActive.value) {
      return;
    }

    final isVisible = await windowManager.isVisible();
    if (!isVisible) {
      return;
    }

    await windowManager.focus();
    launcherFocusNode.requestFocus();
  }

  Future<void> focusQueryBox({bool selectAll = false, bool Function()? shouldSelectAll}) async {
    final screenshotController = Get.find<WoxScreenshotController>();
    if (screenshotController.isSessionActive.value) {
      // Screenshot capture owns the shared window while the annotation workspace is visible. The
      // previous launcher helpers still tried to refocus the hidden query box from generic cleanup
      // paths, which pulled focus/IME behavior back into the launcher and made some screenshot
      // sessions auto-cancel. Ignore launcher focus requests until the screenshot session ends.
      return;
    }

    // only focus when window is visible
    // otherwise it will gain focus but not visible, causing some issues on windows
    // e.g. active window snapshot is wrong
    final isVisible = await windowManager.isVisible();
    if (!isVisible) {
      return;
    }

    if (!isQueryBoxVisible.value) {
      // Hidden query-box launches usually hand keyboard ownership to the preview surface.
      // Do not let generic launcher focus recovery pull focus back to the offstage editor.
      return;
    }

    // request focus to action query box since it will lose focus when tap
    queryBoxFocusNode.requestFocus();

    // force to focus the editable text state to ensure keyboard input works
    // on macos sometimes the keyboard input does not work after requestFocus in certain scenarios
    // E.g. when in explorer layout mode, sometimes the keyboard input does not work after requestFocus
    // which cause the user cannot type in the query box
    final editableTextState = queryBoxTextFieldKey.currentState?.editableTextKey.currentState;
    editableTextState?.requestKeyboard();

    // SelectAll is explicit: launcher activation may select the old query for
    // overwrite, while ordinary refocus should preserve the editor's current
    // selection/cursor.
    // Bug fix: some show-time focus requests finish after the user has already
    // typed the first character. Evaluate the optional SelectAll guard at the
    // last moment so those late completions can recover keyboard focus without
    // replacing the user's in-progress query selection.
    final canSelectAll = shouldSelectAll?.call() ?? true;
    if (selectAll && canSelectAll) {
      queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
    }
  }

  bool _shouldSelectAllForVisibleLauncherFocus({required bool selectAll, required int focusToken, required String textBeforeFocusSequence}) {
    if (!selectAll) {
      return false;
    }
    if (focusToken != _visibleLauncherFocusToken) {
      return false;
    }

    // Bug fix: SelectAll belongs to the launcher activation snapshot. If text
    // changed after the window became visible, the user has already started the
    // next query, so a delayed focus retry should only recover keyboard focus
    // and must not select text that the user just typed.
    return queryBoxTextFieldController.text == textBeforeFocusSequence;
  }

  Future<void> _focusQueryBoxAfterLauncherShow({required String traceId, required bool selectAll}) async {
    final focusToken = ++_visibleLauncherFocusToken;
    final textBeforeFocusSequence = queryBoxTextFieldController.text;

    if (Platform.isWindows) {
      Logger.instance.debug(traceId, "windows launcher focus sequence start: token=$focusToken, textLength=${textBeforeFocusSequence.length}");
    }
    await focusLauncherKeyboardTarget(
      selectAll: selectAll,
      shouldSelectAll: () => _shouldSelectAllForVisibleLauncherFocus(selectAll: selectAll, focusToken: focusToken, textBeforeFocusSequence: textBeforeFocusSequence),
    );
    if (focusToken != _visibleLauncherFocusToken) {
      return;
    }

    // Bug fix: on Windows the native show/focus call can complete before the
    // Flutter editable text is ready to accept keyboard focus. Retry once after
    // the first visible frame, but gate SelectAll against the original text so
    // early user input such as "qianlifeng" cannot lose its first character.
    if (Platform.isWindows) {
      unawaited(
        Future.delayed(const Duration(milliseconds: 100), () async {
          if (focusToken != _visibleLauncherFocusToken) {
            return;
          }
          Logger.instance.debug(traceId, "windows launcher delayed native focus retry: token=$focusToken");
          await focusLauncherKeyboardTarget(
            selectAll: selectAll,
            ensureNativeWindowFocus: true,
            shouldSelectAll: () => _shouldSelectAllForVisibleLauncherFocus(selectAll: selectAll, focusToken: focusToken, textBeforeFocusSequence: textBeforeFocusSequence),
          );
        }),
      );
    }
  }

  void showActionPanel(String traceId) {
    isShowActionPanel.value = true;
    SchedulerBinding.instance.addPostFrameCallback((_) {
      actionListViewController.requestFocus();
    });
    resizeHeight(traceId: traceId, reason: "show action panel");
  }

  void openActionPanelForActiveResult(String traceId) {
    final activeResult = getActiveResult();
    final toolbarMsgActions = buildToolbarMsgActions();
    if ((activeResult == null || activeResult.isGroup) && toolbarMsgActions.isEmpty) {
      return;
    }

    final actions = buildUnifiedActions(traceId, activeResult);
    if (actions.isEmpty) {
      return;
    }

    if (isShowFormActionPanel.value) {
      hideFormActionPanel(traceId, reason: "open action panel");
    }

    refreshActionsForActiveResult(traceId, preserveSelection: false);
    showActionPanel(traceId);
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  WoxQueryResult? getActiveResult() {
    final controller = activeResultViewController;
    if (controller.activeIndex.value >= controller.items.length || controller.activeIndex.value < 0 || controller.items.isEmpty) {
      return null;
    }

    if (isShowingPendingResultPlaceholder) {
      return null;
    }

    final activeResult = controller.items[controller.activeIndex.value].value.data;
    if (activeResult.queryId != currentQuery.value.queryId) {
      return null;
    }

    return activeResult;
  }

  /// given a hotkey, find the action in the result
  WoxResultAction? getActionByHotkey(WoxQueryResult? result, HotKey hotkey) {
    if (result == null && buildToolbarMsgActions().isEmpty) {
      return null;
    }

    final actions = buildUnifiedActions(const UuidV4().generate(), result);
    var filteredActions = actions.where((action) {
      var actionHotkey = WoxHotkey.parseHotkeyFromString(action.hotkey);
      if (actionHotkey != null && WoxHotkey.equals(actionHotkey.normalHotkey, hotkey)) {
        return true;
      }

      return false;
    });

    if (filteredActions.isEmpty) {
      return null;
    }

    return filteredActions.first;
  }

  WoxResultAction? getActionByToolbarHotkey(WoxQueryResult? result, String hotkey) {
    if (result == null && buildToolbarMsgActions().isEmpty) {
      return null;
    }

    final normalizedHotkey = normalizeToolbarHotkey(hotkey);
    final actions = buildUnifiedActions(const UuidV4().generate(), result);
    return actions.firstWhereOrNull((action) => normalizeToolbarHotkey(action.hotkey) == normalizedHotkey);
  }

  void handleToolbarActionTap(String traceId, ToolbarActionInfo actionInfo) {
    if (actionInfo.action != null && activeResultViewController.items.isEmpty) {
      actionInfo.action!.call();
      return;
    }

    if (normalizeToolbarHotkey(actionInfo.hotkey) == normalizeToolbarHotkey(moreActionsHotkey)) {
      openActionPanelForActiveResult(traceId);
      return;
    }

    final activeResult = getActiveResult();
    final action = getActionByToolbarHotkey(activeResult, actionInfo.hotkey);
    if (action != null) {
      executeAction(traceId, activeResult, action);
      return;
    }

    if (actionInfo.action != null) {
      actionInfo.action!.call();
    }
  }

  Future<void> executeAction(String traceId, WoxQueryResult? result, WoxResultAction? action) async {
    Logger.instance.debug(traceId, "user execute result action: ${action?.name}");

    if (action == null) {
      Logger.instance.error(traceId, "active action is null");
      return;
    }

    var preventHideAfterAction = action.preventHideAfterAction;
    Logger.instance.debug(traceId, "execute action: ${action.name}, prevent hide after action: $preventHideAfterAction");

    if (action.type == WoxResultActionTypeEnum.WOX_RESULT_ACTION_TYPE_LOCAL.code) {
      final executed = action.runLocalAction(traceId);
      if (!executed) {
        return;
      }

      if (!preventHideAfterAction) {
        await hideApp(traceId);
      }

      actionListViewController.clearFilter(traceId);
      hideActionPanel(traceId);
      hideFormActionPanel(traceId, reason: "local action executed");
      return;
    }

    if (result == null) {
      Logger.instance.error(traceId, "active query result is null");
      return;
    }

    if (action.type == WoxResultActionTypeEnum.WOX_RESULT_ACTION_TYPE_FORM.code) {
      showFormActionPanel(traceId, action, result.id);
      return;
    } else {
      await WoxWebsocketMsgUtil.instance.sendMessage(
        WoxWebsocketMsg(
          requestId: const UuidV4().generate(),
          traceId: traceId,
          type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
          method: WoxMsgMethodEnum.WOX_MSG_METHOD_ACTION.code,
          data: {"resultId": result.id, "actionId": action.id, "queryId": result.queryId},
        ),
      );
    }

    // clear the search text after action is executed
    actionListViewController.clearFilter(traceId);

    if (!preventHideAfterAction) {
      hideApp(traceId);
    }

    hideActionPanel(traceId);
    hideFormActionPanel(traceId, reason: "non-form action executed");
  }

  Future<void> submitFormAction(String traceId, Map<String, String> values) async {
    final action = activeFormAction.value;
    final resultId = activeFormResultId.value;
    final queryId = currentQuery.value.queryId;
    if (action == null || resultId.isEmpty) {
      hideFormActionPanel(traceId, reason: "submit form without active action");
      return;
    }

    await WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_FORM_ACTION.code,
        data: {"resultId": resultId, "actionId": action.id, "queryId": queryId, "values": values},
      ),
    );

    hideFormActionPanel(traceId, reason: "submit form action");
  }

  Future<void> autoCompleteQuery(String traceId) async {
    var activeResult = getActiveResult();
    if (activeResult == null) {
      return;
    }

    onQueryChanged(
      traceId,
      PlainQuery(queryId: const UuidV4().generate(), queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: activeResult.title, querySelection: Selection.empty()),
      "auto complete query",
      moveCursorToEnd: true,
    );
  }

  Future<bool> acceptQueryCompletionHint(String traceId) async {
    final hint = queryCompletionHint.value;
    if (hint == null || !isQueryCompletionHintValid(hint)) {
      syncQueryBoxCompletionHint();
      return false;
    }

    unawaited(recordQueryCompletionHintAccepted(traceId, hint));

    final nextQueryRefinements =
        shouldPreserveQueryRefinementsForTextChange(currentQuery.value, hint.completionText)
            ? cloneQueryRefinementPayload(currentQuery.value.queryRefinements)
            : <String, String>{};
    final nextContextData =
        shouldPreserveQueryContextDataForTextChange(currentQuery.value, hint.completionText) ? cloneQueryContextData(currentQuery.value.contextData) : <String, String>{};
    await onQueryChanged(
      traceId,
      PlainQuery(
        queryId: const UuidV4().generate(),
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: hint.completionText,
        querySelection: Selection.empty(),
        queryRefinements: nextQueryRefinements,
        contextData: nextContextData,
      ),
      "accept query completion hint",
      moveCursorToEnd: true,
    );
    return true;
  }

  // Records accepted inline completions without blocking the query text update.
  Future<void> recordQueryCompletionHintAccepted(String traceId, QueryCompletionHint hint) async {
    try {
      await WoxWebsocketMsgUtil.instance.sendMessage(
        WoxWebsocketMsg(
          requestId: const UuidV4().generate(),
          traceId: traceId,
          type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
          method: WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY_COMPLETION_HINT_ACCEPTED.code,
          data: {"inputPrefix": hint.inputPrefix, "completionText": hint.completionText, "source": hint.source},
        ),
      );
    } catch (e) {
      Logger.instance.debug(traceId, "Failed to record query completion hint acceptance: $e");
    }
  }

  void onQueryBoxTextChanged(String value) {
    final traceId = const UuidV4().generate();
    canArrowUpHistory = false;
    resultListViewController.isMouseMoved = false;
    resultGridViewController.isMouseMoved = false;

    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      updateQueryBoxLineCount(traceId, value);
      // do local filter if query type is selection
      resultListViewController.filterItems(traceId, value);
      resultGridViewController.filterItems(traceId, value);
      // there maybe no results after filtering, we need to resize height to hide the action panel
      // or show the preview panel
      resizeHeight(traceId: traceId, reason: "selection query text changed");
    } else {
      final nextQueryRefinements =
          shouldPreserveQueryRefinementsForTextChange(currentQuery.value, value) ? cloneQueryRefinementPayload(currentQuery.value.queryRefinements) : <String, String>{};
      final nextContextData = shouldPreserveQueryContextDataForTextChange(currentQuery.value, value) ? cloneQueryContextData(currentQuery.value.contextData) : <String, String>{};
      final skipCompletionHint = reuseQueryCompletionHintForText(value);
      onQueryChanged(
        traceId,
        PlainQuery(
          queryId: const UuidV4().generate(),
          queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
          queryText: value,
          querySelection: Selection.empty(),
          queryRefinements: nextQueryRefinements,
          contextData: nextContextData,
        ),
        "user input changed",
        skipCompletionHint: skipCompletionHint,
        preserveCompletionHint: skipCompletionHint,
      );
    }
  }

  Future<void> queryMRU(String traceId) async {
    var startTime = DateTime.now().millisecondsSinceEpoch;
    var queryId = const UuidV4().generate();
    currentQuery.value = PlainQuery.emptyInput();
    currentQuery.value.queryId = queryId;
    backendQueryContext = QueryContext.empty();
    backendQueryContextQueryId = "";
    clearQueryCompletionHint();
    clearQueryRefinements(traceId);
    prepareQueryLayoutOnQueryChanged(traceId, currentQuery.value);

    try {
      final response = await WoxWebsocketMsgUtil.instance.sendMessage(
        WoxWebsocketMsg(
          requestId: const UuidV4().generate(),
          traceId: traceId,
          type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
          method: WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY_MRU.code,
          data: {"queryId": queryId},
        ),
      );

      if (response == null || response is! List) {
        Logger.instance.debug(traceId, "no MRU results");
        clearQueryResults(traceId);
        return;
      }

      final results = response.map((item) => WoxQueryResult.fromJson(item)).toList();
      if (results.isEmpty) {
        Logger.instance.debug(traceId, "no MRU results");
        clearQueryResults(traceId);
        return;
      }

      for (var result in results) {
        result.queryId = queryId;
      }
      await onReceivedQueryResults(traceId, queryId, results, isFinal: true);
      var endTime = DateTime.now().millisecondsSinceEpoch;
      Logger.instance.debug(traceId, "queryMRU via websocket took ${endTime - startTime} ms");
    } catch (e) {
      Logger.instance.error(traceId, "Failed to query MRU: $e");
      clearQueryResults(traceId);
    }
  }

  Future<void> onQueryChanged(
    String traceId,
    PlainQuery query,
    String changeReason, {
    bool moveCursorToEnd = false,
    bool skipCompletionHint = false,
    bool preserveCompletionHint = false,
  }) async {
    Logger.instance.debug(traceId, "query changed: ${query.queryText}, reason: $changeReason");
    final shouldSkipCompletionHint = skipCompletionHint || !isQueryCompletionHintEnabled();

    if (query.queryId == "") {
      query.queryId = const UuidV4().generate();
    }

    clearHoveredResult();

    //hide setting view if query changed
    if (isInSettingView.value) {
      isInSettingView.value = false;
      await WoxApi.instance.onSetting(traceId, false);
    }
    if (isInOnboardingView.value) {
      // Query-changing commands are launcher actions. Clear onboarding first so
      // selection/query hotkeys do not keep the guide mounted above results.
      isInOnboardingView.value = false;
      await WoxApi.instance.onOnboarding(traceId, false);
    }

    currentQuery.value = query;
    queryOnReceivedElapsedByResultKey.clear();
    backendQueryContext = QueryContext.empty();
    backendQueryContextQueryId = "";
    if (preserveCompletionHint) {
      syncQueryBoxCompletionHint();
    } else {
      clearQueryCompletionHint();
    }
    prepareQueryRefinementsOnQueryChanged(traceId, query);
    isCurrentQueryReturned = false;
    isShowActionPanel.value = false;
    clearQueryResultsTimer.cancel();
    resetPendingResultPlaceholder();
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      canArrowUpHistory = false;
    }

    if (queryBoxTextFieldController.text != query.queryText) {
      queryBoxTextFieldController.text = query.queryText;
    }
    if (moveCursorToEnd) {
      moveQueryBoxCursorToEnd();
    }
    updateQueryBoxLineCount(traceId, query.queryText);

    // Cancel previous loading timer and reset loading state
    loadingTimer?.cancel();
    // Only reset loading state if it is currently true to avoid unnecessary rebuilds
    if (isLoading.value) {
      isLoading.value = false;
    }

    prepareQueryLayoutOnQueryChanged(traceId, query);
    if (!query.isEmpty && currentQuery.value.queryId == query.queryId && !isCurrentQueryReturned) {
      // Query layout now arrives through QueryResponse instead of a pre-query
      // metadata HTTP request. Start the delayed loading timer for any backend
      // query and cancel it when results or a final empty response arrive.
      final hasResults = activeResultViewController.items.isNotEmpty && activeResultViewController.items.first.value.data.queryId == query.queryId;
      if (!hasResults) {
        loadingTimer = Timer(loadingDelay, () {
          // Double check before showing loading:
          // 1. Query is still the same
          // 2. We still don't have results (or results matching this query)
          final stillNoResults = activeResultViewController.items.isEmpty || activeResultViewController.items.first.value.data.queryId != query.queryId;
          if (currentQuery.value.queryId == query.queryId && stillNoResults && !isCurrentQueryReturned) {
            isLoading.value = true;
          }
        });
      }
    }

    if (query.isEmpty) {
      try {
        await WoxWebsocketMsgUtil.instance.sendMessage(
          WoxWebsocketMsg(
            requestId: const UuidV4().generate(),
            traceId: traceId,
            type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
            method: WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code,
            data: {
              "queryId": query.queryId,
              "queryType": query.queryType,
              "queryText": query.queryText,
              "querySelection": query.querySelection.toJson(),
              "queryRefinements": query.queryRefinements,
              "contextData": query.contextData,
              "skipCompletionHint": shouldSkipCompletionHint,
            },
          ),
        );
      } catch (e) {
        Logger.instance.error(traceId, "Failed to notify empty query: $e");
      }

      // Check if we should show MRU results when query is empty (based on start page setting)
      if (lastStartPage == WoxStartPageEnum.WOX_START_PAGE_MRU.code) {
        queryMRU(traceId);
      } else {
        cancelPendingResultTransitions();
        clearQueryResults(traceId);
      }
      return;
    }

    final currentQueryId = query.queryId;
    final isVisible = await windowManager.isVisible();
    // If app is hidden (e.g. tray query will trigger change query first then showapp), clear immediately so old results won't flash when shown.
    if (!isVisible) {
      cancelPendingResultTransitions();
      await clearQueryResults(traceId);
      Logger.instance.debug(traceId, "clear query results immediately because window is hidden");
    } else {
      refreshToolbarActionsForCurrentState(traceId);
      // Delay the stale-content clear slightly so queries that return almost
      // immediately can replace the old snapshot without showing an empty gap.
      // If the backend is still busy after this grace window, switch into the
      // placeholder state above so old content disappears but the window height
      // stays stable until the fresh results arrive.
      clearQueryResultsTimer = Timer(staleVisibleResultsDuration, () {
        if (isClosed) return;
        if (currentQuery.value.queryId != currentQueryId) return;
        if (isCurrentQueryReturned) return;
        showPendingResultPlaceholder(traceId);
      });
    }

    // Record query start time for performance metrics
    queryStartTimeMap[traceId] = DateTime.now().millisecondsSinceEpoch;

    WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code,
        data: {
          "queryId": query.queryId,
          "queryType": query.queryType,
          "queryText": query.queryText,
          "querySelection": query.querySelection.toJson(),
          "queryRefinements": query.queryRefinements,
          "contextData": query.contextData,
          "skipCompletionHint": shouldSkipCompletionHint,
        },
      ),
    );
  }

  void onRefreshQuery(String traceId, bool preserveSelectedIndex) {
    Logger.instance.debug(traceId, "refresh query, preserveSelectedIndex: $preserveSelectedIndex");

    // Save current active index if we need to preserve it
    if (preserveSelectedIndex) {
      final savedActiveIndex = activeResultViewController.activeIndex.value;
      pendingPreservedIndex = savedActiveIndex;
      Logger.instance.debug(traceId, "preserving selected index: $savedActiveIndex");
    }

    // Get current query and create a new query with the same content but new ID
    final currentQueryValue = currentQuery.value;
    final refreshedQuery = PlainQuery(
      queryId: const UuidV4().generate(),
      queryType: currentQueryValue.queryType,
      queryText: currentQueryValue.queryText,
      querySelection: currentQueryValue.querySelection,
      queryRefinements: cloneQueryRefinementPayload(currentQueryValue.queryRefinements),
      contextData: cloneQueryContextData(currentQueryValue.contextData),
    );

    // Re-execute the query
    onQueryChanged(traceId, refreshedQuery, "refresh query");
  }

  Future<void> handleWebSocketMessage(WoxWebsocketMsg msg) async {
    if (msg.method != WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code && msg.type == WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code) {
      Logger.instance.info(msg.traceId, "Received message: ${msg.method}");
    }

    if (msg.type == WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code) {
      return handleWebSocketRequestMessage(msg);
    } else if (msg.type == WoxMsgTypeEnum.WOX_MSG_TYPE_RESPONSE.code) {
      return handleWebSocketResponseMessage(msg);
    }
  }

  Future<void> handleWebSocketRequestMessage(WoxWebsocketMsg msg) async {
    if (msg.method == "ToggleApp") {
      toggleApp(msg.traceId, ShowAppParams.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "HideApp") {
      hideApp(msg.traceId);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ShowApp") {
      showApp(msg.traceId, ShowAppParams.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ChangeQuery") {
      final showSource = msg.data['ShowSource'] as String? ?? WoxShowSourceEnum.WOX_SHOW_SOURCE_DEFAULT.code;
      if (shouldRestoreQueryAfterHide(showSource)) {
        // Temporary query sources such as tray query, query hotkey, or selection query should not replace the main query session permanently.
        preserveQueryBeforeTemporaryQuery(msg.traceId, showSource);
      }
      await onQueryChanged(msg.traceId, PlainQuery.fromJson(msg.data), "receive change query from wox", moveCursorToEnd: true);
      focusQueryBox();
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "RefreshQuery") {
      final preserveSelectedIndex = msg.data['preserveSelectedIndex'] as bool? ?? false;
      onRefreshQuery(msg.traceId, preserveSelectedIndex);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "RefreshGlance") {
      final data = msg.data as Map<String, dynamic>? ?? {};
      final pluginId = data['PluginId'] as String? ?? "";
      final ids = (data['Ids'] as List<dynamic>? ?? []).map((item) => item.toString()).toList();
      await refreshGlance(msg.traceId, "manualRefresh", pluginId: pluginId, ids: ids);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ChangeTheme") {
      final theme = WoxTheme.fromJson(msg.data);
      WoxThemeUtil.instance.changeTheme(theme);
      resizeHeight(traceId: msg.traceId, reason: "theme changed"); // Theme height maybe changed, so we need to resize height
      // Theme change triggers widget rebuild which may lose focus, so we need to restore focus after rebuild
      SchedulerBinding.instance.addPostFrameCallback((_) {
        focusQueryBox();
      });
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "PickFiles") {
      final pickFilesParams = FileSelectorParams.fromJson(msg.data);
      final files = await FileSelector.pick(msg.traceId, pickFilesParams);
      responseWoxWebsocketRequest(msg, true, files);
    } else if (msg.method == "CaptureScreenshot") {
      final screenshotController = Get.find<WoxScreenshotController>();
      final result = await screenshotController.startCaptureSession(msg.traceId, CaptureScreenshotRequest.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, result.toJson());
    } else if (msg.method == "WriteClipboardImageFile") {
      final data = (msg.data as Map).map<String, dynamic>((key, value) => MapEntry(key.toString(), value));
      final filePath = data['filePath'] as String? ?? data['FilePath'] as String? ?? "";
      if (filePath.isEmpty) {
        responseWoxWebsocketRequest(msg, false, "filePath must be a non-empty string");
      } else {
        try {
          await ScreenshotPlatformBridge.instance.writeClipboardImageFile(filePath: filePath);
          responseWoxWebsocketRequest(msg, true, null);
        } catch (e) {
          Logger.instance.warn(msg.traceId, "WriteClipboardImageFile failed: $e");
          responseWoxWebsocketRequest(msg, false, e.toString());
        }
      }
    } else if (msg.method == "FocusSettingWindow") {
      await focusSettingWindow();
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "OpenSettingWindow") {
      openSetting(msg.traceId, SettingWindowContext.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "OpenOnboardingWindow") {
      openOnboarding(msg.traceId);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ShowToolbarMsg") {
      await showToolbarMsg(msg.traceId, ToolbarMsg.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ClearToolbarMsg") {
      final data = msg.data as Map<String, dynamic>? ?? {};
      final toolbarMsgId = data['toolbarMsgId'] as String? ?? "";
      await clearToolbarMsg(msg.traceId, toolbarMsgId);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "DiagnosticStatusChanged") {
      final data = msg.data as Map<String, dynamic>? ?? {};
      updateDiagnosticStatus(msg.traceId, data["enabled"] == true);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "AttentionUnreadCountChanged") {
      final data = msg.data as Map<String, dynamic>? ?? {};
      updateAttentionUnreadCount(msg.traceId, (data["unreadCount"] as num?)?.toInt() ?? 0);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "RecordHotkey") {
      final data = msg.data as Map<String, dynamic>? ?? {};
      final hotkey = data["Hotkey"]?.toString() ?? "";
      Logger.instance.info(msg.traceId, "Received RecordHotkey websocket request: hotkey=$hotkey");
      WoxHotkeyRecordingBus.instance.emit(hotkey);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "GetCurrentQuery") {
      responseWoxWebsocketRequest(msg, true, currentQuery.value.toJson());
    } else if (msg.method == "FocusToChatInput") {
      Get.find<WoxAIChatController>().focusToChatInput(msg.traceId);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "SendChatResponse") {
      handleChatResponse(msg.traceId, WoxAIChatData.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ReloadChatResources") {
      Get.find<WoxAIChatController>().reloadChatResources(msg.traceId, resourceName: msg.data as String);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ReloadSettingPlugins") {
      Get.find<WoxSettingController>().reloadPlugins(msg.traceId);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ReloadSetting") {
      await Get.find<WoxSettingController>().reloadSetting(msg.traceId);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ReloadSettingThemes") {
      await Get.find<WoxSettingController>().refreshThemeList();
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "RefreshAccountStatus") {
      unawaited(refreshAccountStatusAfterBillingDeeplink(msg.traceId));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_CLOUD_SYNC_PROGRESS_CHANGED.code) {
      final data = msg.data as Map<String, dynamic>? ?? {};
      if (Get.isRegistered<WoxSettingController>()) {
        Get.find<WoxSettingController>().applyCloudSyncProgress(data);
      }
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "UpdateResult") {
      final success = updateResult(msg.traceId, UpdatableResult.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, success);
    } else if (msg.method == "PushResults") {
      final data = msg.data as Map<String, dynamic>? ?? {};
      final queryId = data['QueryId'] as String? ?? "";
      final resultsData = data['Results'] as List<dynamic>? ?? [];
      final results = resultsData.map((item) => WoxQueryResult.fromJson(item)).toList();
      final success = await pushResults(msg.traceId, queryId, results);
      responseWoxWebsocketRequest(msg, true, success);
    }
  }

  Future<void> handleWebSocketResponseMessage(WoxWebsocketMsg msg) async {
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_TERMINAL_CHUNK.code) {
      final data = msg.data as Map<String, dynamic>? ?? {};
      handleTerminalChunk(data);
      return;
    }
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_TERMINAL_STATE.code) {
      final data = msg.data as Map<String, dynamic>? ?? {};
      handleTerminalState(data);
      return;
    }
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY_COMPLETION_HINT.code) {
      if (msg.sendTimestamp > 0) {
        final latency = DateTime.now().millisecondsSinceEpoch - msg.sendTimestamp;
        if (latency > 10) {
          Logger.instance.info(msg.traceId, "📨 Query completion hint WebSocket latency (Wox→UI): ${latency}ms");
        }
      }

      final data = msg.data as Map<String, dynamic>? ?? {};
      final queryId = data['QueryId'] as String? ?? "";
      final hintData = data['CompletionHint'];
      final hint = hintData is Map ? QueryCompletionHint.fromJson(Map<String, dynamic>.from(hintData)) : null;
      applyQueryCompletionHintForQueryId(msg.traceId, queryId, hint);
      return;
    }

    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
      final receiveTimestampMs = DateTime.now().millisecondsSinceEpoch;
      var websocketLatencyMs = -1;

      // Log WebSocket latency (Wox -> UI) only for Query method
      if (msg.sendTimestamp > 0) {
        websocketLatencyMs = receiveTimestampMs - msg.sendTimestamp;
        if (websocketLatencyMs > 10) {
          Logger.instance.info(msg.traceId, "📨 WebSocket latency (Wox→UI): ${websocketLatencyMs}ms");
        }
      }

      final applyTracker = WoxTimeTracker.start(msg.traceId, "ui_query_response_apply");
      final responseApplyStartUs = applyTracker.checkpointUs();

      // Parse QueryResponse object
      final queryResponse = msg.data as Map<String, dynamic>;
      final resultsData = queryResponse['Results'] as List<dynamic>;
      final queryId = queryResponse['QueryId'] as String? ?? "";
      final isFinal = queryResponse['IsFinal'] as bool? ?? false;
      final backendQueryStartTimestampMs = queryResponse['QueryStartTimestamp'] as int? ?? 0;
      applyTracker.setRawString("queryId", queryId);
      applyTracker.setBool("isFinal", isFinal);
      applyTracker.setInt("backendQueryStartTimestampMs", backendQueryStartTimestampMs);
      applyTracker.setInt("rawResultCount", resultsData.length);
      final actionIconRefsStartUs = applyTracker.checkpointUs();
      final actionIconRefs = _parseQueryActionIconRefs(queryResponse['ActionIconRefs']);
      if (actionIconRefs.isNotEmpty) {
        _resolveQueryActionIconRefs(resultsData, actionIconRefs);
      }
      applyTracker.setInt("actionIconRefCount", actionIconRefs.length);
      applyTracker.setElapsedUs("actionIconRefsRestoreUs", actionIconRefsStartUs);

      final receiveTracker = WoxTimeTracker.start(msg.traceId, "ui_query_response_receive");
      receiveTracker.setRawString("queryId", queryId);
      receiveTracker.setBool("isFinal", isFinal);
      receiveTracker.setInt("rawResultCount", resultsData.length);
      receiveTracker.setInt("actionIconRefCount", actionIconRefs.length);
      if (websocketLatencyMs >= 0) {
        receiveTracker.setInt("backendSendLatencyMs", websocketLatencyMs);
      }
      receiveTracker.log();
      _scheduleQueryEventLoopTurnTiming(
        traceId: msg.traceId,
        stage: "ui_query_receive_next_turn",
        queryId: queryId,
        resultCount: resultsData.length,
        isFinal: isFinal,
        scheduledTimestampMs: receiveTimestampMs,
      );

      final contextData = queryResponse['Context'];
      if (contextData is Map && contextData.isNotEmpty) {
        // Core owns the final query classification after shortcut expansion
        // and trigger-keyword parsing. Apply it before layout/results so Glance
        // and plugin identity do not depend on Flutter's local guess.
        final contextStartUs = applyTracker.checkpointUs();
        applyQueryContextForQueryId(msg.traceId, queryId, QueryContext.fromJson(Map<String, dynamic>.from(contextData)));
        applyTracker.setElapsedUs("contextApplyUs", contextStartUs);
      }
      final layoutData = queryResponse['Layout'];
      if (layoutData is Map && layoutData.isNotEmpty) {
        // QueryResponse layout replaces the old /query/metadata side request.
        // Apply it before results so list/grid switches happen under the same
        // query id and stale rows cannot be rendered with the new layout.
        final layoutStartUs = applyTracker.checkpointUs();
        applyQueryLayoutForQueryId(msg.traceId, queryId, QueryLayout.fromJson(Map<String, dynamic>.from(layoutData)));
        applyTracker.setElapsedUs("layoutApplyUs", layoutStartUs);
      }
      if (queryResponse.containsKey('Refinements')) {
        final refinementsStartUs = applyTracker.checkpointUs();
        final refinementsData = queryResponse['Refinements'];
        final refinements =
            refinementsData is List
                ? refinementsData.whereType<Map>().map((item) => WoxQueryRefinement.fromJson(Map<String, dynamic>.from(item))).toList()
                : <WoxQueryRefinement>[];
        applyQueryRefinementsForQueryId(msg.traceId, queryId, refinements);
        applyTracker.setInt("refinementCount", refinements.length);
        applyTracker.setElapsedUs("refinementsApplyUs", refinementsStartUs);
      }

      final resultParseStartUs = applyTracker.checkpointUs();
      var results = <WoxQueryResult>[];
      for (var item in resultsData) {
        results.add(WoxQueryResult.fromJson(item));
      }
      applyTracker.setInt("resultCount", results.length);
      applyTracker.setElapsedUs("resultParseUs", resultParseStartUs);

      Logger.instance.info(msg.traceId, "Received websocket message: ${msg.method}, results count: ${results.length}, isFinal: $isFinal");

      // Process results first
      final onReceivedStartUs = applyTracker.checkpointUs();
      final didApplyResults = await onReceivedQueryResults(msg.traceId, queryId, results, isFinal: isFinal, backendQueryStartTimestampMs: backendQueryStartTimestampMs);
      applyTracker.setElapsedUs("onReceivedUs", onReceivedStartUs);
      applyTracker.setBool("resultApplied", didApplyResults);
      if (!didApplyResults) {
        applyTracker.setBool("skippedAfterResultApply", true);
        applyTracker.setElapsedUs("totalUs", responseApplyStartUs);
        applyTracker.log();
        queryStartTimeMap.remove(msg.traceId);
        return;
      }

      // If this is the final final response, we must stop loading animation explicitly
      // This handles cases where results are empty but the query is finished
      // We explicitly check if this final response belongs to the current query
      if (isFinal && queryId == currentQuery.value.queryId) {
        final finalLoadingStartUs = applyTracker.checkpointUs();
        loadingTimer?.cancel();
        if (isLoading.value) {
          isLoading.value = false;
        }
        applyTracker.setElapsedUs("finalLoadingUs", finalLoadingStartUs);
      }
      applyTracker.setElapsedUs("totalUs", responseApplyStartUs);
      applyTracker.log();

      if (isFinal) {
        queryStartTimeMap.remove(msg.traceId);
      }
    }
  }

  void responseWoxWebsocketRequest(WoxWebsocketMsg request, bool success, dynamic data) {
    WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: request.requestId,
        traceId: request.traceId,
        sessionId: request.sessionId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_RESPONSE.code,
        method: request.method,
        data: data,
        success: success,
      ),
    );
  }

  Stream<Map<String, dynamic>> terminalChunkStream(String sessionId) {
    terminalChunkControllers.putIfAbsent(sessionId, () => StreamController<Map<String, dynamic>>.broadcast());
    return terminalChunkControllers[sessionId]!.stream;
  }

  Stream<Map<String, dynamic>> terminalStateStream(String sessionId) {
    terminalStateControllers.putIfAbsent(sessionId, () => StreamController<Map<String, dynamic>>.broadcast());
    return terminalStateControllers[sessionId]!.stream;
  }

  void handleTerminalChunk(Map<String, dynamic> data) {
    final sessionId = data['SessionId'] as String? ?? "";
    if (sessionId.isEmpty) {
      return;
    }
    if (terminalChunkControllers.containsKey(sessionId)) {
      terminalChunkControllers[sessionId]!.add(data);
    }
  }

  void handleTerminalState(Map<String, dynamic> data) {
    final sessionId = data['SessionId'] as String? ?? "";
    if (sessionId.isEmpty) {
      return;
    }
    if (terminalStateControllers.containsKey(sessionId)) {
      terminalStateControllers[sessionId]!.add(data);
    }
  }

  Future<void> subscribeTerminalSession(String traceId, String sessionId, {int cursor = 0}) async {
    await WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_TERMINAL_SUBSCRIBE.code,
        data: {"sessionId": sessionId, "cursor": cursor},
      ),
    );
  }

  Future<void> unsubscribeTerminalSession(String traceId, String sessionId) async {
    await WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_TERMINAL_UNSUBSCRIBE.code,
        data: {"sessionId": sessionId},
      ),
    );
  }

  Future<Map<String, dynamic>?> searchTerminalSession(String traceId, String sessionId, String pattern, {int cursor = 0, bool backward = false, bool caseSensitive = false}) async {
    final response = await WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_TERMINAL_SEARCH.code,
        data: {"sessionId": sessionId, "pattern": pattern, "cursor": cursor, "backward": backward, "caseSensitive": caseSensitive},
      ),
    );
    if (response is Map<String, dynamic>) {
      return response;
    }
    return null;
  }

  String getTerminalSessionId(WoxPreview preview) {
    if (preview.previewType != WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code || preview.previewData.isEmpty) {
      return "";
    }

    try {
      final parsed = jsonDecode(preview.previewData);
      if (parsed is Map && parsed['session_id'] is String) {
        return parsed['session_id'] as String;
      }
    } catch (_) {}

    return preview.previewData;
  }

  String getTerminalCommand(WoxPreview preview) {
    if (preview.previewType != WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code || preview.previewData.isEmpty) {
      return "";
    }

    try {
      final parsed = jsonDecode(preview.previewData);
      if (parsed is Map && parsed['command'] is String) {
        return parsed['command'] as String;
      }
    } catch (_) {}

    return "";
  }

  String getTerminalStatus(WoxPreview preview) {
    if (preview.previewType != WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code || preview.previewData.isEmpty) {
      return "";
    }

    try {
      final parsed = jsonDecode(preview.previewData);
      if (parsed is Map && parsed['status'] is String) {
        return parsed['status'] as String;
      }
    } catch (_) {}

    return "";
  }

  bool triggerTerminalFind() {
    if (!isShowPreviewPanel.value) {
      return false;
    }
    if (currentPreview.value.previewType != WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code) {
      return false;
    }
    final sessionId = getTerminalSessionId(currentPreview.value);
    if (sessionId.isEmpty) {
      return false;
    }

    terminalFindTrigger.value += 1;
    return true;
  }

  bool supportsPreviewFullscreen(WoxPreview preview) {
    return preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code ||
        preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code ||
        preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_QUERY_REQUIREMENT_SETTINGS.code ||
        preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_THEME_EDIT.code ||
        preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TRIGGER_KEYWORD_CONFLICT.code;
  }

  bool isCoreInteractiveSettingsPreview(WoxPreview preview) {
    return preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_QUERY_REQUIREMENT_SETTINGS.code ||
        preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_THEME_EDIT.code ||
        preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TRIGGER_KEYWORD_CONFLICT.code;
  }

  bool isCoreReferencePreview(WoxPreview preview) {
    return preview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_HOTKEY_OVERVIEW.code;
  }

  bool shouldShowPreviewPanelForPreview(WoxPreview preview) {
    if (preview.previewData.isEmpty) {
      return false;
    }

    // Core interactive settings previews are the exception to the normal grid rule:
    // grid items cannot carry enough text or controls to resolve blocked queries,
    // so these system previews must be visible even while successful results use grid.
    if (isCoreInteractiveSettingsPreview(preview)) {
      return true;
    }

    if (isCoreReferencePreview(preview)) {
      return true;
    }

    return !isInGridMode();
  }

  void syncPreviewModeForActivePreview(String traceId) {
    if (isShowPreviewPanel.value && isCoreInteractiveSettingsPreview(currentPreview.value)) {
      // These preview types own the full query result area. The previous generic
      // preview behavior kept grid/list results visible, which left too little
      // space for actionable settings forms.
      if (!isPreviewFullscreen.value) {
        final restoreRatio = resultPreviewRatio.value > 0 ? resultPreviewRatio.value : getPreferredResultPreviewRatio();
        lastResultPreviewRatioBeforePreviewFullscreen = restoreRatio;
        resultPreviewRatio.value = 0;
        isPreviewFullscreen.value = true;
        Logger.instance.debug(traceId, "core interactive settings preview enter fullscreen");
      }
      return;
    }

    syncPreviewFullscreenState();
  }

  double getPreferredResultPreviewRatio() {
    return preferredResultPreviewRatio > 0 ? preferredResultPreviewRatio : defaultResultPreviewRatio;
  }

  bool enterPreviewFullscreen(String traceId) {
    if (!isShowPreviewPanel.value || !supportsPreviewFullscreen(currentPreview.value)) {
      return false;
    }

    if (isPreviewFullscreen.value) {
      return true;
    }

    final restoreRatio = resultPreviewRatio.value > 0 ? resultPreviewRatio.value : getPreferredResultPreviewRatio();
    lastResultPreviewRatioBeforePreviewFullscreen = restoreRatio;
    resultPreviewRatio.value = 0;
    isPreviewFullscreen.value = true;
    Logger.instance.debug(traceId, "preview enter fullscreen");
    refreshActionsForActiveResult(traceId, preserveSelection: true);
    return true;
  }

  bool exitPreviewFullscreen(String traceId) {
    if (!isPreviewFullscreen.value) {
      return false;
    }

    final restoreRatio = lastResultPreviewRatioBeforePreviewFullscreen > 0 ? lastResultPreviewRatioBeforePreviewFullscreen : getPreferredResultPreviewRatio();
    resultPreviewRatio.value = restoreRatio;
    isPreviewFullscreen.value = false;
    Logger.instance.debug(traceId, "preview exit fullscreen, ratio restored: $restoreRatio");
    refreshActionsForActiveResult(traceId, preserveSelection: true);
    return true;
  }

  bool togglePreviewFullscreen(String traceId) {
    if (!isShowPreviewPanel.value || !supportsPreviewFullscreen(currentPreview.value)) {
      return false;
    }

    if (isPreviewFullscreen.value) {
      return exitPreviewFullscreen(traceId);
    }

    return enterPreviewFullscreen(traceId);
  }

  bool toggleTerminalPreviewFullscreen(String traceId) {
    return togglePreviewFullscreen(traceId);
  }

  void syncPreviewFullscreenState() {
    final isFullscreenPreviewVisible = isShowPreviewPanel.value && supportsPreviewFullscreen(currentPreview.value);
    if (isFullscreenPreviewVisible) {
      return;
    }

    if (isPreviewFullscreen.value) {
      final restoreRatio = lastResultPreviewRatioBeforePreviewFullscreen > 0 ? lastResultPreviewRatioBeforePreviewFullscreen : getPreferredResultPreviewRatio();
      resultPreviewRatio.value = restoreRatio;
    }
    isPreviewFullscreen.value = false;
  }

  Future<void> clearQueryResults(String traceId) async {
    Logger.instance.debug(traceId, "clear query results");
    cancelPendingResultTransitions();
    resultListViewController.clearItems();
    resultGridViewController.clearItems();
    actionListViewController.clearItems();
    isShowPreviewPanel.value = false;
    isShowActionPanel.value = false;
    syncPreviewFullscreenState();

    // Recompute toolbar actions from the current state instead of clearing the
    // right side directly. Persistent toolbar messages keep their own actions,
    // and clearing them here makes the toolbar and action panel disagree.
    refreshToolbarActionsForCurrentState(traceId);
    await resizeHeight(traceId: traceId, reason: "clear query results");
  }

  /// Calculate the window height based on the current result count.
  ///
  /// [overrideItemCount] allows callers to specify a virtual item count instead
  /// of reading the actual items in the active result view controller. This is
  /// used by the initial-show path to compute height with 0 results when a new
  /// query is about to be issued and old results should be ignored.
  ///
  /// [overrideGridHeight] lets callers provide a grid height that was
  /// estimated from incoming items before those items are committed.
  double calculateWindowHeight({int? overrideItemCount, double? overrideGridHeight}) {
    final maxResultCount = getMaxResultCount();
    final maxHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(maxResultCount);
    final itemCount = overrideItemCount ?? activeResultViewController.items.length;
    final hasItems = itemCount > 0;
    double resultHeight;

    if (isInGridMode()) {
      resultHeight = overrideGridHeight ?? resultGridViewController.calculateGridHeight();
    } else {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(itemCount);
    }

    if (resultHeight > maxHeight) {
      resultHeight = maxHeight;
    }
    if (isShowActionPanel.value || isShowPreviewPanel.value || isShowFormActionPanel.value) {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(maxResultCount);
    }

    if (hasItems) {
      resultHeight += WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingTop + WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingBottom;
    }
    // Only add toolbar height when toolbar is actually shown in UI.
    // Use local hasItems instead of the isShowToolbar getter so that
    // overrideItemCount is respected.
    final showToolbar = (hasItems || isShowDoctorCheckInfo || hasVisibleToolbarMsg || hasBugAwareToolbarIndicator) && !isToolbarHiddenForce.value;
    if (showToolbar) {
      resultHeight += WoxThemeUtil.instance.getToolbarHeight();
    }

    if (!isQueryBoxVisible.value && (itemCount == 0 || isLoading.value)) {
      resultHeight = math.max(resultHeight, WoxThemeUtil.instance.getResultListViewHeightByCount(1));
    }

    if (!isQueryBoxVisible.value && !isPreviewOnlyLayout) {
      resultHeight += WoxThemeUtil.instance.currentTheme.value.appPaddingBottom.toDouble();
    }

    final queryBoxHeight = isQueryBoxVisible.value ? getQueryBoxTotalHeight() : 0.0;
    final refinementHeight = getQueryRefinementBarHeight();
    var totalHeight = queryBoxHeight + refinementHeight + resultHeight;

    // On Windows with high DPI, add one pixel to avoid fractional cut-off.
    if (Platform.isWindows) {
      if (PlatformDispatcher.instance.views.first.devicePixelRatio > 1) {
        totalHeight = totalHeight + 1;
      }
    }

    // If toolbar is shown without results, remove bottom padding to blend with query box.
    final toolbarShowedWithoutResults = showToolbar && !hasItems;
    if (toolbarShowedWithoutResults && isQueryBoxVisible.value) {
      totalHeight -= WoxThemeUtil.instance.currentTheme.value.appPaddingBottom;
    }

    if (isShowingPendingResultPlaceholder && pendingResultPlaceholderHeight != null) {
      totalHeight = math.max(totalHeight, pendingResultPlaceholderHeight!);
    }

    return totalHeight;
  }

  /// Calculate the initial window height when showing the app.
  ///
  /// In continue mode (no incoming query injected), results from the previous
  /// session are preserved, so we use the real item count. Otherwise, a new
  /// query is about to be issued and old results should be ignored, so we
  /// compute the height as if there are 0 results.
  double calculateInitialShowWindowHeight(bool isIncomingQueryInjected) {
    if (!isIncomingQueryInjected && activeResultViewController.items.isNotEmpty) {
      return calculateWindowHeight();
    }

    if (!isIncomingQueryInjected && pendingRestoredQueryId == currentQuery.value.queryId && pendingRestoredQueryWindowHeight != null) {
      return math.max(calculateWindowHeight(overrideItemCount: 0), pendingRestoredQueryWindowHeight!);
    }

    return calculateWindowHeight(overrideItemCount: 0);
  }

  // select all text in query box
  void selectQueryBoxAllText(String traceId) {
    Logger.instance.info(traceId, "select query box all text");
    queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
  }

  /// reset and jump active result to top of the list
  void resetActiveResult() {
    final controller = activeResultViewController;
    if (controller.items.isNotEmpty) {
      if (controller.items.first.value.isGroup) {
        controller.updateActiveIndex(const UuidV4().generate(), 1);
      } else {
        controller.updateActiveIndex(const UuidV4().generate(), 0);
      }
    }
  }

  String formatWindowSize(Size size) {
    return "${size.width}x${size.height}";
  }

  Future<void> _waitForNextFlutterFrame({required String traceId, required String reason}) async {
    // Bug fix: management-window transitions need a concrete Flutter frame
    // boundary after native geometry changes. A scheduled frame plus timeout
    // keeps the staging path deterministic without risking a stuck open if the
    // hidden Windows window does not produce a frame promptly.
    final completer = Completer<void>();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!completer.isCompleted) {
        completer.complete();
      }
    });
    SchedulerBinding.instance.scheduleFrame();
    await completer.future.timeout(
      const Duration(milliseconds: 60),
      onTimeout: () {
        Logger.instance.warn(traceId, "wait for next Flutter frame timed out: $reason");
      },
    );
  }

  int logicalToPhysicalPixels(double logicalPixels) {
    return (logicalPixels * PlatformDispatcher.instance.views.first.devicePixelRatio).round();
  }

  bool isWindowSizeEffectivelyEqual(Size left, Size right) {
    return (logicalToPhysicalPixels(left.width) - logicalToPhysicalPixels(right.width)).abs() <= 1 &&
        (logicalToPhysicalPixels(left.height) - logicalToPhysicalPixels(right.height)).abs() <= 1;
  }

  /// Invalidates launcher-sized resize work before switching to a fixed management window.
  void cancelLauncherResizeRequests(String traceId, String reason) {
    resizeRequestToken++;
    ongoingResizeTargetSize = null;
    Logger.instance.debug(traceId, "resize cancelled: reason=$reason, management view transition");
  }

  Future<void> resizeHeight({required String traceId, String reason = "unspecified", bool forceDwmRecomposition = false, double? overrideTargetHeight}) async {
    final tracker = WoxTimeTracker.start(traceId, "ui_resize_height");
    final totalStartUs = tracker.checkpointUs();
    tracker.setString("reason", reason);
    tracker.setBool("forceDwmRecomposition", forceDwmRecomposition);
    tracker.setBool("hasOverrideTargetHeight", overrideTargetHeight != null);

    // Don't resize when a management view is active or being staged; settings
    // and onboarding use fixed window geometry instead of launcher content
    // height, and route mounting is delayed until that geometry is ready.
    if (isInSettingView.value || isInOnboardingView.value || isManagementWindowTransitionActive) {
      Logger.instance.debug(traceId, "resize skipped: reason=$reason, management view is active");
      tracker.setBool("skippedManagementView", true);
      tracker.setElapsedUs("totalUs", totalStartUs);
      tracker.log();
      return;
    }

    var totalHeight = overrideTargetHeight ?? calculateWindowHeight();

    // Force DWM to recompose Acrylic by adding a single pixel to bypass caching identical sizes
    if (forceDwmRecomposition && Platform.isWindows) {
      totalHeight += 1;
    }

    double targetWidth = forceWindowWidth != 0 ? forceWindowWidth : WoxSettingUtil.instance.currentSetting.appWidth.toDouble();
    final targetSize = Size(targetWidth, totalHeight.toDouble());

    // Linux Wayland resize can race with show/hide and result updates. In that
    // path, Dart-side target tracking can be stale while the native window is
    // still at the old height, so Linux always sends the target to the runner.
    if (!Platform.isLinux && !forceDwmRecomposition && ongoingResizeTargetSize != null && isWindowSizeEffectivelyEqual(ongoingResizeTargetSize!, targetSize)) {
      Logger.instance.debug(traceId, "resize skipped: reason=$reason, target=${formatWindowSize(targetSize)}, duplicateTargetInFlight=true");
      tracker.setDouble("targetWidth", targetSize.width);
      tracker.setDouble("targetHeight", targetSize.height);
      tracker.setBool("skippedDuplicateTarget", true);
      tracker.setBool("skippedBeforePlatformSize", true);
      tracker.setElapsedUs("totalUs", totalStartUs);
      tracker.log();
      return;
    }

    // Claim the resize request before the first native await so slower older
    // requests cannot become "latest" after a newer shrink/expand request.
    final currentResizeToken = ++resizeRequestToken;
    ongoingResizeTargetSize = targetSize;

    try {
      bool cancelResizeIfNeeded(String phase, {bool beforeRetry = false}) {
        final superseded = resizeRequestToken != currentResizeToken;
        final managementViewActive = isInSettingView.value || isInOnboardingView.value;
        if (!superseded && !managementViewActive) {
          return false;
        }

        Logger.instance.debug(
          traceId,
          "resize skipped: reason=$reason, target=${formatWindowSize(targetSize)}, phase=$phase, superseded=$superseded, managementViewActive=$managementViewActive",
        );
        tracker.setDouble("targetWidth", targetSize.width);
        tracker.setDouble("targetHeight", targetSize.height);
        tracker.setBool("skippedSuperseded", superseded);
        tracker.setBool("skippedManagementView", managementViewActive);
        if (beforeRetry) {
          tracker.setBool("skippedBeforePlatformRetry", true);
        } else {
          tracker.setBool("skippedBeforePlatformSet", true);
        }
        tracker.setElapsedUs("totalUs", totalStartUs);
        tracker.log();
        return true;
      }

      final getCurrentSizeStartUs = tracker.checkpointUs();
      final currentSize = await windowManager.getSize();
      tracker.setElapsedUs("getCurrentSizeUs", getCurrentSizeStartUs);
      if (cancelResizeIfNeeded("after get current size")) {
        return;
      }

      final isSameSize = isWindowSizeEffectivelyEqual(currentSize, targetSize);
      tracker.setDouble("beforeWidth", currentSize.width);
      tracker.setDouble("beforeHeight", currentSize.height);
      tracker.setDouble("targetWidth", targetSize.width);
      tracker.setDouble("targetHeight", targetSize.height);
      tracker.setBool("sameSize", isSameSize);
      Logger.instance.debug(
        traceId,
        "resize requested: reason=$reason, before=${formatWindowSize(currentSize)}, target=${formatWindowSize(targetSize)}, sameSize=$isSameSize, forceDwmRecomposition=$forceDwmRecomposition",
      );

      // On Linux Wayland, getSize can report the requested/default size before
      // the mapped allocation has actually reached that size. Let the native
      // runner apply the request; it filters stale resize sequences.
      if (!Platform.isLinux && isSameSize && !forceDwmRecomposition) {
        committedWindowHeight = targetSize.height;
        Logger.instance.debug(traceId, "resize skipped: reason=$reason, before=${formatWindowSize(currentSize)}, target=${formatWindowSize(targetSize)}, sameSize=true");
        tracker.setBool("skippedSameSize", true);
        tracker.setElapsedUs("totalUs", totalStartUs);
        tracker.log();
        return;
      }

      if (isQueryBoxAtBottom.value) {
        // When the query box is anchored to the bottom, grow the window upward.
        // Use getPosition + getSize to compute the current bottom edge, then adjust top to grow upward.
        final getPositionStartUs = tracker.checkpointUs();
        final pos = await windowManager.getPosition();
        tracker.setElapsedUs("getPositionUs", getPositionStartUs);
        if (cancelResizeIfNeeded("after get position")) {
          return;
        }

        double currentBottom = pos.dy + currentSize.height;

        if (currentBottom <= 0) {
          // Fallback if bounds are weird
        } else {
          double newTop = currentBottom - totalHeight;
          // Apply position and size together to avoid intermediate-frame flicker.
          final setBoundsStartUs = tracker.checkpointUs();
          await windowManager.setBounds(Offset(pos.dx, newTop), targetSize);
          tracker.setElapsedUs("setBoundsUs", setBoundsStartUs);
          final getResizedSizeStartUs = tracker.checkpointUs();
          var resizedSize = await windowManager.getSize();
          tracker.setElapsedUs("getResizedSizeUs", getResizedSizeStartUs);
          if (!isWindowSizeEffectivelyEqual(resizedSize, targetSize)) {
            // Native resize can report success before the top-level HWND reaches the requested size.
            // Reapplying once keeps result-driven launcher resizing deterministic without blocking the normal path.
            Logger.instance.warn(
              traceId,
              "resize readback mismatch: reason=$reason, target=${formatWindowSize(targetSize)}, after=${formatWindowSize(resizedSize)}, retry=setBounds",
            );
            final retrySetBoundsStartUs = tracker.checkpointUs();
            await Future.delayed(const Duration(milliseconds: 16));
            if (cancelResizeIfNeeded("before retry setBounds", beforeRetry: true)) {
              return;
            }
            await windowManager.setBounds(Offset(pos.dx, newTop), targetSize);
            tracker.setElapsedUs("retrySetBoundsUs", retrySetBoundsStartUs);
            final getRetriedSizeStartUs = tracker.checkpointUs();
            resizedSize = await windowManager.getSize();
            tracker.setElapsedUs("getRetriedSizeUs", getRetriedSizeStartUs);
          }
          Logger.instance.debug(
            traceId,
            "resize applied: reason=$reason, before=${formatWindowSize(currentSize)}, target=${formatWindowSize(targetSize)}, after=${formatWindowSize(resizedSize)}, mode=setBounds, growUpward=true",
          );

          committedWindowHeight = targetSize.height;
          windowFlickerDetector.recordResize(totalHeight.toInt());
          tracker.setRawString("mode", "setBounds");
          tracker.setBool("growUpward", true);
          tracker.setDouble("afterWidth", resizedSize.width);
          tracker.setDouble("afterHeight", resizedSize.height);
          tracker.setElapsedUs("totalUs", totalStartUs);
          tracker.log();
          return;
        }
      }

      final setSizeStartUs = tracker.checkpointUs();
      await windowManager.setSize(targetSize);
      tracker.setElapsedUs("setSizeUs", setSizeStartUs);
      final getResizedSizeStartUs = tracker.checkpointUs();
      var resizedSize = await windowManager.getSize();
      tracker.setElapsedUs("getResizedSizeUs", getResizedSizeStartUs);
      if (!isWindowSizeEffectivelyEqual(resizedSize, targetSize)) {
        // Native resize can report success before the top-level HWND reaches the requested size.
        // Reapplying once keeps result-driven launcher resizing deterministic without blocking the normal path.
        Logger.instance.warn(traceId, "resize readback mismatch: reason=$reason, target=${formatWindowSize(targetSize)}, after=${formatWindowSize(resizedSize)}, retry=setSize");
        final retrySetSizeStartUs = tracker.checkpointUs();
        await Future.delayed(const Duration(milliseconds: 16));
        if (cancelResizeIfNeeded("before retry setSize", beforeRetry: true)) {
          return;
        }
        await windowManager.setSize(targetSize);
        tracker.setElapsedUs("retrySetSizeUs", retrySetSizeStartUs);
        final getRetriedSizeStartUs = tracker.checkpointUs();
        resizedSize = await windowManager.getSize();
        tracker.setElapsedUs("getRetriedSizeUs", getRetriedSizeStartUs);
      }
      Logger.instance.debug(
        traceId,
        "resize applied: reason=$reason, before=${formatWindowSize(currentSize)}, target=${formatWindowSize(targetSize)}, after=${formatWindowSize(resizedSize)}, mode=setSize, growUpward=false",
      );
      committedWindowHeight = targetSize.height;
      windowFlickerDetector.recordResize(totalHeight.toInt());
      tracker.setRawString("mode", "setSize");
      tracker.setBool("growUpward", false);
      tracker.setDouble("afterWidth", resizedSize.width);
      tracker.setDouble("afterHeight", resizedSize.height);
      tracker.setElapsedUs("totalUs", totalStartUs);
      tracker.log();
    } finally {
      if (resizeRequestToken == currentResizeToken) {
        ongoingResizeTargetSize = null;
      }
    }
  }

  void updateQueryBoxTextWrapWidth(String traceId, double width) {
    if ((queryBoxTextWrapWidth - width).abs() < 1) {
      return;
    }

    queryBoxTextWrapWidth = width;
    updateQueryBoxLineCount(traceId, queryBoxTextFieldController.text);
  }

  int calculateQueryBoxLineCount(String text) {
    final normalizedText = text.replaceAll('\r\n', '\n');
    if (queryBoxTextWrapWidth <= 0) {
      return normalizedText.isEmpty ? 1 : normalizedText.split('\n').length.clamp(1, QUERY_BOX_MAX_LINES).toInt();
    }

    final metrics = WoxInterfaceSizeUtil.instance.current;
    final painter = TextPainter(
      text: TextSpan(text: normalizedText.isEmpty ? ' ' : normalizedText, style: TextStyle(fontSize: metrics.queryBoxFontSize)),
      textDirection: TextDirection.ltr,
      textScaler: TextScaler.noScaling,
    )..layout(minWidth: 0, maxWidth: queryBoxTextWrapWidth);

    // Query text can wrap visually even when it has no explicit newline. The previous explicit-newline
    // count kept the window one line tall, so wrapped content and the caret could move outside the
    // visible input area. Measuring the actual layout preserves pasted multi-line text and lets long
    // single-line queries expand up to the existing query box limit.
    final lineCount = painter.computeLineMetrics().length.clamp(1, QUERY_BOX_MAX_LINES).toInt();
    painter.dispose();
    return lineCount;
  }

  void updateQueryBoxLineCount(String traceId, String text) {
    final rawLineCount = calculateQueryBoxLineCount(text);
    final clampedLineCount = rawLineCount.clamp(1, QUERY_BOX_MAX_LINES);
    if (queryBoxLineCount.value == clampedLineCount) {
      return;
    }
    queryBoxLineCount.value = clampedLineCount;
    resizeHeight(traceId: traceId, reason: "query box line count changed");
  }

  double getQueryBoxInputHeight() {
    final extraLines = queryBoxLineCount.value - 1;
    final metrics = WoxInterfaceSizeUtil.instance.current;
    // Density changes the query-box content height, so multi-line expansion must
    // derive from the current metrics instead of the old normal-only constant.
    return metrics.queryBoxBaseHeight + (metrics.queryBoxLineHeight * extraLines);
  }

  double getQueryBoxTotalHeight() {
    final extraLines = queryBoxLineCount.value - 1;
    return WoxThemeUtil.instance.getQueryBoxHeight() + (WoxInterfaceSizeUtil.instance.current.queryBoxLineHeight * extraLines);
  }

  void clearHoveredResult() {
    resultListViewController.clearHoveredResult();
    resultGridViewController.clearHoveredResult();
  }

  void updateLazyLoadedResultIcon(String traceId, WoxListItem<WoxQueryResult> item, WoxImage icon) {
    if (icon.imageData.isEmpty) {
      return;
    }

    // Lazy result icons are rendered through a core-owned token first, then
    // replaced in the result model after Flutter receives the resized cache
    // image. Updating the model avoids repeated token requests when a list/grid
    // child is disposed and rebuilt during scrolling.
    updateResult(traceId, UpdatableResult(id: item.id, icon: icon));
  }

  bool updateResult(String traceId, UpdatableResult updatableResult) {
    // Try to find the result in the current items
    try {
      final result = activeResultViewController.items.firstWhere((element) => element.value.data.id == updatableResult.id);
      var needUpdate = false;
      var updatedResult = result.value;
      var updatedData = result.value.data;

      // Update only non-null fields
      if (updatableResult.title != null) {
        updatedResult = updatedResult.copyWith(title: updatableResult.title);
        updatedData.title = updatableResult.title!;
        needUpdate = true;
      }

      if (updatableResult.subTitle != null) {
        updatedResult = updatedResult.copyWith(subTitle: updatableResult.subTitle);
        updatedData.subTitle = updatableResult.subTitle!;
        needUpdate = true;
      }

      if (updatableResult.icon != null) {
        updatedResult = updatedResult.copyWith(icon: updatableResult.icon);
        updatedData.icon = updatableResult.icon!;
        needUpdate = true;
      }

      if (updatableResult.tails != null) {
        updatedResult = updatedResult.copyWith(tails: updatableResult.tails);
        updatedData.tails = updatableResult.tails!;
        needUpdate = true;
      }

      if (updatableResult.preview != null) {
        updatedData.preview = updatableResult.preview!;
        needUpdate = true;
      }

      if (updatableResult.actions != null) {
        updatedData.actions = updatableResult.actions!;
        needUpdate = true;
      }

      if (updatableResult.hasDragDataUpdate) {
        updatedData.dragData = updatableResult.dragData?.isFiles == true ? updatableResult.dragData : null;
        needUpdate = true;
      }

      if (needUpdate) {
        // Force create a new WoxListItem with updated data to trigger reactive update
        updatedResult = updatedResult.copyWith(data: updatedData);
        resultListViewController.updateItem(traceId, updatedResult);
        resultGridViewController.updateItem(traceId, updatedResult);

        // If this result is currently active, update the preview and actions
        if (activeResultViewController.isItemActive(updatedData.id)) {
          if (updatableResult.preview != null) {
            final oldShowPreview = isShowPreviewPanel.value;
            currentPreview.value = updatableResult.preview!;
            // Query requirement settings override the normal grid-preview
            // restriction so users can fix missing settings without leaving the
            // current query.
            isShowPreviewPanel.value = shouldShowPreviewPanelForPreview(currentPreview.value);
            syncPreviewModeForActivePreview(traceId);

            // If preview panel visibility changed, resize window height
            if (oldShowPreview != isShowPreviewPanel.value) {
              resizeHeight(traceId: traceId, reason: "active result preview visibility changed");
            }
          }

          if (updatableResult.actions != null) {
            refreshActionsForActiveResult(traceId, preserveSelection: true);
          } else if (updatableResult.preview != null) {
            refreshActionsForActiveResult(traceId, preserveSelection: true);
          }
        }
      }

      return true; // Successfully found and updated the result
    } catch (e) {
      // Result not found in current items (no longer visible)
      return false;
    }
  }

  Future<bool> pushResults(String traceId, String queryId, List<WoxQueryResult> results) async {
    if (queryId.isEmpty) {
      Logger.instance.error(traceId, "push results ignored: query id is empty");
      return false;
    }
    if (currentQuery.value.queryId != queryId) {
      Logger.instance.error(traceId, "query id is not matched, ignore the results");
      return false;
    }

    // Query matched, so we stop loading animation regardless of result count
    if (queryId == currentQuery.value.queryId) {
      loadingTimer?.cancel();
      isLoading.value = false;
    }

    if (results.isEmpty) {
      return true;
    }

    if (pendingRestoredQueryId == queryId) {
      pendingRestoredQueryId = null;
      pendingRestoredQueryWindowHeight = null;
    }

    for (var result in results) {
      if (result.queryId.isEmpty) {
        result.queryId = queryId;
      }
    }

    await onReceivedQueryResults(traceId, queryId, results, isFinal: false);
    return true;
  }

  /// Process doctor check results and update the doctor check info
  DoctorCheckInfo processDoctorCheckResults(List<DoctorCheckResult> results) {
    // Ignored checks are skipped in the toolbar but remain visible in the
    // results list so the doctor query can still show them with an Unignore
    // action.
    final activeResults = results.where((r) => !r.ignored).toList();

    // Check if all non-ignored tests passed
    bool allPassed = true;
    for (var result in activeResults) {
      if (!result.passed) {
        allPassed = false;
        break;
      }
    }
    doctorCheckPassed = allPassed;

    // Determine appropriate icon and message based on issue type
    WoxImage icon = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: QUERY_ICON_DOCTOR_WARNING);
    String message = "";

    for (var result in activeResults) {
      if (!result.passed) {
        message = result.description;
        if (result.isVersionIssue) {
          icon = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: UPDATE_ICON);
          break;
        } else if (result.isPermissionIssue) {
          icon = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: PERMISSION_ICON);
          break;
        }
      }
    }

    return DoctorCheckInfo(results: results, allPassed: allPassed, icon: icon, message: message);
  }

  void doctorCheck() async {
    final traceId = const UuidV4().generate();
    final wasToolbarVisible = isToolbarVisible;
    var results = await WoxApi.instance.doctorCheck(traceId);
    final checkInfo = processDoctorCheckResults(results);
    doctorCheckInfo.value = checkInfo;
    updateDoctorToolbarIfNeeded(traceId);
    final toolbarVisible = isToolbarVisible;
    if (wasToolbarVisible != toolbarVisible) {
      await resizeHeight(traceId: traceId, reason: "doctor check toolbar visibility changed");
    }
    Logger.instance.debug(traceId, "doctor check result: ${checkInfo.allPassed}, details: ${checkInfo.results.length} items");
  }

  @override
  void dispose() {
    glanceRefreshTimer?.cancel();
    hiddenCacheClearTimer?.cancel();
    cancelPendingResultTransitions();
    loadingTimer?.cancel();
    launcherFocusNode.dispose();
    queryBoxFocusNode.dispose();
    queryBoxTextFieldController.dispose();
    actionListViewController.dispose();
    resultListViewController.dispose();
    resultGridViewController.dispose();
    for (final controller in terminalChunkControllers.values) {
      controller.close();
    }
    for (final controller in terminalStateControllers.values) {
      controller.close();
    }
    terminalChunkControllers.clear();
    terminalStateControllers.clear();
    _manualFilePreviewLoadRequests.close();
    super.dispose();
  }

  Future<void> openSetting(String traceId, SettingWindowContext context) async {
    final settingController = Get.find<WoxSettingController>();
    var wasWindowVisible = false;
    try {
      wasWindowVisible = await windowManager.isVisible();
    } catch (_) {}

    // Save current position before switching (used if we return to launcher)
    try {
      positionBeforeOpenSetting = await windowManager.getPosition();
    } catch (_) {}

    // Bug fix: settings exit behavior follows the opener source instead of
    // current visibility. Launcher-origin opens can temporarily lose native
    // visibility, while tray-origin opens must still close directly back to
    // hidden state.
    isSettingOpenedFromHidden = context.source == SettingWindowContext.sourceTray;
    // Bug fix: settings search is a per-visit affordance. Clearing it at entry
    // protects against stale text if a previous settings route was hidden by a
    // platform path that did not go through the normal back button flow.
    settingController.clearSettingSearch();
    settingController.activeNavPath.value = 'general';
    cancelLauncherResizeRequests(traceId, "enter setting view");
    isManagementWindowTransitionActive = true;

    try {
      final stageWindowsSettingOpen = Platform.isWindows && wasWindowVisible;
      await WoxApi.instance.onSetting(traceId, true);
      await WoxApi.instance.onOnboarding(traceId, false);

      // Bug fix: keep the route switch responsive by drawing settings from the
      // cached theme/settings first. The old awaited refresh made Windows stay
      // hidden while HTTP reloads ran; refreshing in the background keeps data
      // fresh without delaying the first settings frame.
      unawaited(() async {
        try {
          await Future.wait([WoxThemeUtil.instance.loadTheme(traceId), settingController.reloadSetting(traceId)]);
        } catch (e) {
          Logger.instance.error(traceId, "Failed to refresh settings window data in background: $e");
        }
      }());

      if (context.path == "/about") {
        // The onboarding entry lives on About, so tests and future deep links need
        // the same openSetting path support that plugin/data pages already have.
        settingController.activeNavPath.value = 'about';
      }

      const settingWindowPreferredSize = Size(1200, 800);
      const settingWindowMaxWorkAreaFraction = 0.8;
      final settingWindowSize =
          Platform.isWindows
              ? await WindowsWindowManager.instance.constrainSizeToCursorDisplayWorkArea(settingWindowPreferredSize, maxWorkAreaFraction: settingWindowMaxWorkAreaFraction)
              : settingWindowPreferredSize;
      if (stageWindowsSettingOpen) {
        // Bug fix: Windows paints the native acrylic/Mica background as soon as
        // the root HWND grows. Hide only the final geometry staging window so the
        // user sees an immediate settings route switch, then the final 1200x800
        // settings frame, without watching the backdrop expand first.
        await windowManager.hide();
      }
      await windowManager.setSize(settingWindowSize);
      if (Platform.isLinux) {
        isInSettingView.value = true;
        isInOnboardingView.value = false;
        // On Linux we need to show first before positioning works reliably.
        await windowManager.show();
        await windowManager.center(settingWindowSize.width, settingWindowSize.height);
      } else {
        await windowManager.center(settingWindowSize.width, settingWindowSize.height);
        if (Platform.isWindows) {
          // Bug fix: management routes must not mount against the previous
          // launcher-sized constraints. Stage native geometry first, then wait
          // one frame after the route swap so smoke tests never observe the
          // intermediate constrained layout as a RenderFlex overflow.
          await _waitForNextFlutterFrame(traceId: traceId, reason: "settings window geometry");
        }
        isInSettingView.value = true;
        isInOnboardingView.value = false;
        if (Platform.isWindows) {
          await _waitForNextFlutterFrame(traceId: traceId, reason: "settings route switch");
        }
        await windowManager.show();
      }

      if (context.path == "/plugin/setting") {
        WidgetsBinding.instance.addPostFrameCallback((_) async {
          await Future.delayed(const Duration(milliseconds: 100));
          await settingController.switchToPluginList(traceId, false);
          settingController.focusInstalledPlugin(context.param);
          settingController.switchToPluginSettingTab();
        });
      }
      if (context.path == "/data") {
        WidgetsBinding.instance.addPostFrameCallback((_) async {
          await Future.delayed(const Duration(milliseconds: 100));
          await settingController.switchToBackupView(traceId);
        });
      }
      if (context.path == "/general" && context.param.trim().isNotEmpty) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          settingController.focusGeneralSection(context.param);
        });
      }
    } finally {
      isManagementWindowTransitionActive = false;
    }

    await windowManager.focus();
    await windowManager.setAlwaysOnTop(false);

    // Load heavier settings-page data after the window is visible. These lists
    // (plugins, fonts, backups, usage) are not required for the first frame and
    // should not lengthen the short Windows hide-and-resize staging path.
    settingController.preloadSettingViewData(traceId, forceRefresh: true);

    WidgetsBinding.instance.addPostFrameCallback((_) async {
      await Future.delayed(const Duration(milliseconds: 50));
      // Feature: the search box is the primary settings entry point, so new
      // settings sessions put keyboard focus there instead of the passive page
      // focus node.
      settingController.settingSearchFocusNode.requestFocus();
    });

    // On Windows, ensure focus is properly set after window is shown
    if (Platform.isWindows) {
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        await Future.delayed(const Duration(milliseconds: 100));
        await windowManager.focus();
        settingController.settingSearchFocusNode.requestFocus();
        Logger.instance.info(traceId, "[SETTING] Windows focus requested after delay");
      });
    }
  }

  // Focuses an existing settings window without changing its active page.
  Future<void> focusSettingWindow() async {
    if (!isInSettingView.value) {
      return;
    }

    final settingController = Get.find<WoxSettingController>();
    await windowManager.show();
    await windowManager.focus();
    await windowManager.setAlwaysOnTop(false);
    if (settingController.settingFocusNode.canRequestFocus) {
      settingController.settingFocusNode.requestFocus();
    }
  }

  // Refreshes billing-adjacent state after core handles a billing callback.
  Future<void> refreshAccountStatusAfterBillingDeeplink(String traceId) async {
    final settingController = Get.find<WoxSettingController>();
    try {
      await Future.wait([settingController.refreshAccountStatus(), settingController.refreshCloudSyncStatus()]);
    } catch (e) {
      Logger.instance.error(traceId, "Failed to refresh account status after billing deeplink: $e");
    }
  }

  Future<void> exitSetting(String traceId) async {
    final settingController = Get.find<WoxSettingController>();
    closeAllDialogsInSetting();
    // Bug fix: search text should not survive closing settings. Reset before
    // either exit branch so returning to the launcher and hidden-window exits
    // both reopen with a clean search field.
    settingController.clearSettingSearch();
    settingController.settingSearchFocusNode.unfocus();

    if (isSettingOpenedFromHidden) {
      // For hidden-opened settings, exit means hide the window directly
      await hideApp(traceId);
      return;
    }

    // Switch back to launcher
    isInSettingView.value = false;
    await WoxApi.instance.onSetting(traceId, false);
    await windowManager.setAlwaysOnTop(true);
    await resizeHeight(traceId: traceId, reason: "exit setting view");
    await windowManager.setPosition(positionBeforeOpenSetting);
    await windowManager.focus();
    // Bug fix: leaving settings marks the launcher visible before query-box
    // focus has finished. Await the first focus request so callers and smoke
    // tests observe the real postcondition, then keep the delayed retry for
    // platforms that report window focus before the launcher text field rebuilds.
    await _focusQueryBoxAfterLauncherShow(traceId: traceId, selectAll: true);
  }

  Future<void> openOnboarding(String traceId) async {
    final settingController = Get.find<WoxSettingController>();
    var wasWindowVisible = false;
    try {
      wasWindowVisible = await windowManager.isVisible();
    } catch (_) {}

    closeAllDialogsInSetting();
    // Bug fix: onboarding has its own page-level Escape handling, but the
    // shared window can still carry focus from the launcher query box or
    // settings view during the route swap. Drop those old focus owners before
    // mounting the guide so their Escape-to-hide handlers cannot fire first.
    queryBoxFocusNode.unfocus();
    settingController.settingFocusNode.unfocus();

    cancelLauncherResizeRequests(traceId, "enter onboarding view");
    isManagementWindowTransitionActive = true;

    try {
      await WoxApi.instance.onSetting(traceId, false);
      await WoxApi.instance.onOnboarding(traceId, true);

      await WoxThemeUtil.instance.loadTheme(traceId);
      await settingController.reloadSetting(traceId);

      // Feature refinement: onboarding still uses the management-window contract,
      // but it is narrower than settings so the shared Wox examples read closer
      // to the real launcher width instead of stretching across a settings page.
      if (Platform.isWindows && wasWindowVisible) {
        await windowManager.hide();
      }
      await windowManager.setSize(_onboardingWindowSize);
      if (!Platform.isLinux) {
        await windowManager.center(_onboardingWindowSize.width, _onboardingWindowSize.height);
        if (Platform.isWindows) {
          // Bug fix: About can reopen onboarding while the shared window still
          // has settings or launcher constraints. Mounting onboarding only after
          // native geometry reaches the guide size avoids one-frame overflows.
          await _waitForNextFlutterFrame(traceId: traceId, reason: "onboarding window geometry");
        }
      }

      isInSettingView.value = false;
      isInOnboardingView.value = true;
      if (Platform.isWindows) {
        await _waitForNextFlutterFrame(traceId: traceId, reason: "onboarding route switch");
      }
      if (Platform.isLinux) {
        await windowManager.show();
        await windowManager.center(_onboardingWindowSize.width, _onboardingWindowSize.height);
      } else {
        await windowManager.show();
      }
    } finally {
      isManagementWindowTransitionActive = false;
    }

    await windowManager.focus();
    await windowManager.setAlwaysOnTop(false);
  }

  Future<void> finishOnboarding(String traceId, {required bool markFinished}) async {
    final settingController = Get.find<WoxSettingController>();

    if (markFinished) {
      // Skip and finish deliberately share this durable state transition so the
      // guide is never auto-shown again after the user leaves it once.
      await settingController.updateConfig("OnboardingFinished", "true");
    }

    isInOnboardingView.value = false;
    await WoxApi.instance.onOnboarding(traceId, false);
    await windowManager.setAlwaysOnTop(true);
    await WoxApi.instance.show(traceId);
  }

  void closeAllDialogsInSetting() {
    final navigator = Get.key.currentState;
    if (navigator == null) {
      return;
    }

    navigator.popUntil((route) => route.isFirst);
  }

  Future<void> showToolbarMsg(String traceId, ToolbarMsg msg) async {
    if (msg.isPersistent) {
      final wasVisible = hasVisibleToolbarMsg;
      toolbarMsg.value = msg;
      refreshToolbarActionsForCurrentState(traceId);
      refreshVisibleActionPanel(traceId, preserveSelection: true);
      if (!wasVisible) {
        await resizeHeight(traceId: traceId, reason: "show toolbar msg");
      }
      return;
    }

    // Snooze/mute enforcement is handled by backend before pushing to UI.
    if (hasVisibleToolbarMsg) {
      return;
    }

    toolbar.value = ToolbarInfo(text: msg.text, icon: msg.icon, actions: toolbar.value.actions);
    if (msg.displaySeconds > 0) {
      Future.delayed(Duration(seconds: msg.displaySeconds), () {
        // only hide toolbar msg when the text is the same as the one we are showing
        if (toolbar.value.text == msg.text) {
          toolbar.value = toolbar.value.emptyLeftSide();
        }
      });
    }
  }

  void executeDefaultAction(String traceId) {
    Logger.instance.info(traceId, "execute default action");

    if (activeResultViewController.items.isEmpty) {
      final actions = buildUnifiedActions(traceId, null);
      final defaultAction = actions.firstWhereOrNull((action) => action.isDefault) ?? actions.firstWhereOrNull((action) => action.hotkey.toLowerCase() == "enter");
      if (defaultAction != null) {
        executeAction(traceId, null, defaultAction);
        return;
      }
    }

    // Get the active result
    if (activeResultViewController.items.isEmpty) {
      Logger.instance.debug(traceId, "no results to execute");
      return;
    }

    var activeResult = activeResultViewController.activeItem;
    if (activeResult.isGroup) {
      Logger.instance.debug(traceId, "cannot execute group item");
      return;
    }

    // Get the active action (from action panel if visible, otherwise default action)
    WoxResultAction? actionToExecute;
    if (isShowActionPanel.value && actionListViewController.items.isNotEmpty) {
      actionToExecute = actionListViewController.activeItem.data;
    } else {
      final actions = buildUnifiedActions(traceId, activeResult.data);
      actionToExecute = actions.firstWhereOrNull((action) => action.isDefault);
      if (actionToExecute == null && actions.isNotEmpty) {
        actionToExecute = actions.first;
      }
    }

    if (actionToExecute == null) {
      Logger.instance.error(traceId, "no action to execute");
      return;
    }

    executeAction(traceId, activeResult.data, actionToExecute);
  }

  void handleChatResponse(String traceId, WoxAIChatData data) {
    for (var result in activeResultViewController.items) {
      if (result.value.data.preview.previewType != WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code) {
        continue;
      }

      try {
        var previewData = jsonDecode(result.value.data.preview.previewData);
        if (previewData['Id'] != data.id) {
          continue;
        }
      } catch (_) {
        continue;
      }

      // update preview in result list view item
      // otherwise, the preview will lost when user switch to other result and back
      result.value.data.preview = WoxPreview(
        previewType: WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code,
        previewData: jsonEncode(data.toJson()),
        scrollPosition: WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code,
      );

      Get.find<WoxAIChatController>().handleChatResponse(traceId, data);
    }
  }

  void moveQueryBoxCursorToStart() {
    queryBoxTextFieldController.selection = TextSelection.fromPosition(const TextPosition(offset: 0));
    if (queryBoxScrollController.hasClients) {
      queryBoxScrollController.jumpTo(0);
    }
  }

  void moveQueryBoxCursorToEnd() {
    queryBoxTextFieldController.selection = TextSelection.collapsed(offset: queryBoxTextFieldController.text.length);
    if (queryBoxScrollController.hasClients) {
      queryBoxScrollController.jumpTo(queryBoxScrollController.position.maxScrollExtent);
    }
  }

  void handleQueryBoxArrowUp() {
    if (canArrowUpHistory) {
      if (currentQueryHistoryIndex < latestQueryHistories.length - 1) {
        currentQueryHistoryIndex = currentQueryHistoryIndex + 1;
        var changedQuery = latestQueryHistories[currentQueryHistoryIndex].query;
        if (changedQuery != null) {
          final traceId = const UuidV4().generate();
          onQueryChanged(traceId, changedQuery, "user arrow up history");
          selectQueryBoxAllText(traceId);
        }
      }
      return;
    }

    activeResultViewController.updateActiveIndexByDirection(const UuidV4().generate(), WoxDirectionEnum.WOX_DIRECTION_UP.code);
  }

  void handleQueryBoxArrowDown() {
    canArrowUpHistory = false;
    activeResultViewController.updateActiveIndexByDirection(const UuidV4().generate(), WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
  }

  void handleQueryBoxArrowLeft() {
    activeResultViewController.updateActiveIndexByDirection(const UuidV4().generate(), WoxDirectionEnum.WOX_DIRECTION_LEFT.code);
  }

  void handleQueryBoxArrowRight() {
    activeResultViewController.updateActiveIndexByDirection(const UuidV4().generate(), WoxDirectionEnum.WOX_DIRECTION_RIGHT.code);
  }

  bool isInGridMode() {
    return isGridLayout.value == true;
  }

  void onResultItemActivated(String traceId, WoxListItem<WoxQueryResult> item) {
    currentPreview.value = item.data.preview;
    isShowPreviewPanel.value = shouldShowPreviewPanelForPreview(currentPreview.value);
    syncPreviewModeForActivePreview(traceId);
    refreshActionsForActiveResult(traceId, preserveSelection: false);
  }

  Future<void> startResultDrag(String traceId, WoxListItem<WoxQueryResult> item) async {
    if (item.isGroup || item.data.dragData == null || !item.data.dragData!.isFiles) {
      return;
    }

    final status = await ResultDragPlatformBridge.instance.startFileDrag(traceId, item.data.dragData!.files);
    if (status == ResultDragStatus.success || status == ResultDragStatus.cancel) {
      // Keep launcher visible only when the user releases inside Wox itself,
      // which native drag reports as cancel_in_source.
      await hideApp(traceId);
    }
  }

  void onResultItemsEmpty(String traceId) {
    // Hide preview panel when there are no results after filtering
    // otherwise in selection mode, when no result filtered, preview may still be shown
    isShowPreviewPanel.value = false;
    currentPreview.value = WoxPreview.empty();
    syncPreviewFullscreenState();

    refreshToolbarActionsForCurrentState(traceId);
  }

  void updateToolbarWithActions(String traceId) {
    final activeResult = getActiveResult();
    final toolbarActions = buildToolbarActionsForCurrentState(activeResult);
    if (toolbarActions.isEmpty) {
      toolbar.value = toolbar.value.emptyRightSide();
      return;
    }

    toolbar.value = toolbar.value.copyWith(actions: toolbarActions);
  }

  void refreshToolbarActionsForCurrentState(String traceId) {
    // Bug fix: when fast typing keeps the previous result snapshot visible during
    // the stale-results grace window, clearing toolbar actions immediately makes
    // only the footer switch to the "no result" state and causes visible flicker.
    // Keep the current toolbar until the visible results are actually cleared or
    // replaced so the whole launcher transitions as one snapshot.
    if (hasVisibleStaleResultsDuringQueryTransition) {
      return;
    }

    final activeResult = getActiveResult();
    if (activeResult != null && !activeResult.isGroup) {
      updateToolbarWithActions(traceId);
      return;
    }

    final toolbarMsgActions = buildToolbarMsgActions();
    if (toolbarMsgActions.isNotEmpty) {
      updateToolbarWithActions(traceId);
      return;
    }

    toolbar.value = toolbar.value.emptyRightSide();
    updateDoctorToolbarIfNeeded(traceId);
  }

  void refreshVisibleActionPanel(String traceId, {required bool preserveSelection}) {
    if (!isShowActionPanel.value) {
      return;
    }

    refreshActionsForActiveResult(traceId, preserveSelection: preserveSelection);
  }

  Future<void> clearToolbarMsg(String traceId, String toolbarMsgId) async {
    if (toolbarMsgId.isEmpty || toolbarMsg.value.id != toolbarMsgId) {
      return;
    }

    final wasVisible = hasVisibleToolbarMsg;
    toolbarMsg.value = ToolbarMsg.empty();
    refreshToolbarActionsForCurrentState(traceId);
    refreshVisibleActionPanel(traceId, preserveSelection: true);
    if (wasVisible) {
      await resizeHeight(traceId: traceId, reason: "clear toolbar msg");
    }
  }

  Future<void> handleDropFiles(DropDoneDetails details) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, "Received drop files: ${details.files.map((e) => e.path).join(", ")}");

    await windowManager.focus();
    focusQueryBox();

    canArrowUpHistory = false;

    PlainQuery woxChangeQuery = PlainQuery(
      queryId: const UuidV4().generate(),
      queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code,
      queryText: "",
      querySelection: Selection(type: WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code, text: "", filePaths: details.files.map((e) => e.path).toList()),
    );

    onQueryChanged(traceId, woxChangeQuery, "user drop files");
  }

  void prepareQueryLayoutOnQueryChanged(String traceId, PlainQuery query) {
    if (isLikelyPluginInputQuery(query)) {
      // Plugin-query layout now arrives with QueryResponse. Keep the previous
      // plugin icon/layout visible until the same query id returns its layout,
      // matching the old metadata request behavior and avoiding an icon clear
      // followed by an immediate reset.
      return;
    }

    // Empty, global, and selection queries have deterministic local layout.
    // Reset them immediately so stale plugin grid/ratio state does not leak
    // into non-plugin surfaces while plugin-specific metadata waits for results.
    applyQueryLayoutForQuery(traceId, query, QueryLayout.empty());
  }

  bool applyQueryContextForQueryId(String traceId, String queryId, QueryContext queryContext) {
    if (currentQuery.value.queryId != queryId) {
      // QueryContext is delivered asynchronously with QueryResponse. Guarding
      // by query id prevents a late backend classification from changing Glance
      // visibility or plugin identity for a newer query.
      Logger.instance.debug(traceId, "ignore stale query context for queryId=$queryId");
      return false;
    }

    backendQueryContext = queryContext;
    backendQueryContextQueryId = queryId;
    applyQueryContextForQuery(traceId, currentQuery.value, queryContext);
    return true;
  }

  void applyQueryContextForQuery(String traceId, PlainQuery query, QueryContext queryContext) {
    if (queryContext.isGlobalQuery) {
      // Backend-confirmed global queries should use the global accessory area.
      // This corrects local trigger-keyword guesses for text that contains
      // spaces but does not actually target a plugin.
      applyQueryLayoutForQuery(traceId, query, QueryLayout.empty());
      return;
    }

    // Plugin and selection contexts reserve the query-box accessory for their
    // own identity, so clear cached Glance rows even before layout arrives.
    glanceItems.clear();
  }

  bool applyQueryLayoutForQueryId(String traceId, String queryId, QueryLayout queryLayout) {
    if (currentQuery.value.queryId != queryId) {
      // Layout is asynchronous just like result batches. Guarding by query id
      // prevents a late QueryResponse from switching icon, preview ratio, or
      // list/grid mode after the user has already typed another query.
      Logger.instance.debug(traceId, "ignore stale query layout for queryId=$queryId");
      return false;
    }

    applyQueryLayoutForQuery(traceId, currentQuery.value, queryLayout);
    return true;
  }

  void applyQueryLayoutForQuery(String traceId, PlainQuery query, QueryLayout queryLayout) {
    updateQueryIconOnQueryChanged(traceId, query, queryLayout);
    updateResultPreviewWidthRatioOnQueryChanged(traceId, query, queryLayout);
    updateGridLayoutParamsOnQueryChanged(traceId, query, queryLayout);
  }

  /// Change the query icon based on the query
  void updateQueryIconOnQueryChanged(String traceId, PlainQuery query, QueryLayout queryLayout) {
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      glanceItems.clear();
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code) {
        queryIcon.value = QueryIconInfo(icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_FILE));
      }
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code) {
        queryIcon.value = QueryIconInfo(icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_TEXT));
      }
      return;
    }

    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      if (isGlobalInputQuery(query)) {
        queryIcon.value = QueryIconInfo.empty();
        if (glanceItems.isEmpty || query.queryText.isEmpty) {
          // Global query classification now uses plugin trigger metadata, not
          // whitespace. Empty global input gets an explicit refresh because
          // clearing a search can otherwise leave a stale in-flight refresh as
          // the last Glance update and make the accessory disappear.
          unawaited(refreshGlance(traceId, "manualRefresh"));
        }
        return;
      }

      glanceItems.clear();
      queryIcon.value = QueryIconInfo(icon: queryLayout.icon);
      return;
    }

    queryIcon.value = QueryIconInfo.empty();
  }

  /// Update the result preview width ratio based on the query
  void updateResultPreviewWidthRatioOnQueryChanged(String traceId, PlainQuery query, QueryLayout queryLayout) {
    double nextRatio = defaultResultPreviewRatio;
    if (query.isEmpty) {
      preferredResultPreviewRatio = nextRatio;
      if (isPreviewFullscreen.value) {
        lastResultPreviewRatioBeforePreviewFullscreen = nextRatio;
      } else {
        resultPreviewRatio.value = nextRatio;
      }
      return;
    }
    if (isGlobalInputQuery(query)) {
      preferredResultPreviewRatio = nextRatio;
      if (isPreviewFullscreen.value) {
        lastResultPreviewRatioBeforePreviewFullscreen = nextRatio;
      } else {
        resultPreviewRatio.value = nextRatio;
      }
      return;
    }
    // Selection queries default to a 6:4 split (list 40%, preview 60%).
    // Only apply when the backend did not return an explicit ratio; an explicit
    // zero means the plugin wants a preview-only layout and must be respected.
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code && queryLayout.resultPreviewWidthRatio == null) {
      nextRatio = defaultResultPreviewRatio;
      preferredResultPreviewRatio = nextRatio;
      if (isPreviewFullscreen.value) {
        lastResultPreviewRatioBeforePreviewFullscreen = nextRatio;
      } else {
        resultPreviewRatio.value = nextRatio;
      }
      return;
    }

    nextRatio = queryLayout.resultPreviewWidthRatio ?? defaultResultPreviewRatio;
    Logger.instance.debug(traceId, "update result preview width ratio: $nextRatio");
    if (nextRatio < 0 || nextRatio > 1) {
      nextRatio = defaultResultPreviewRatio;
    }
    preferredResultPreviewRatio = nextRatio;
    if (isPreviewFullscreen.value) {
      lastResultPreviewRatioBeforePreviewFullscreen = nextRatio;
    } else {
      resultPreviewRatio.value = nextRatio;
    }
  }

  void updateGridLayoutParamsOnQueryChanged(String traceId, PlainQuery query, QueryLayout queryLayout) {
    final wasGridLayout = isGridLayout.value;
    if (query.isEmpty) {
      isGridLayout.value = false;
      gridLayoutParams.value = GridLayoutParams.empty();
      if (wasGridLayout) {
        resizeHeight(traceId: traceId, reason: "exit grid layout for empty query");
      }
      return;
    }
    if (isGlobalInputQuery(query)) {
      isGridLayout.value = false;
      gridLayoutParams.value = GridLayoutParams.empty();
      if (wasGridLayout) {
        resizeHeight(traceId: traceId, reason: "exit grid layout for global query");
      }
      return;
    }

    if (queryLayout.isGridLayout) {
      isGridLayout.value = true;
      gridLayoutParams.value = queryLayout.gridLayoutParams;
    } else {
      isGridLayout.value = false;
      gridLayoutParams.value = GridLayoutParams.empty();
    }
    resultGridViewController.updateGridParams(gridLayoutParams.value);

    Logger.instance.debug(traceId, "update grid layout params: columns=${queryLayout.gridLayoutParams.columns}");

    if (wasGridLayout != isGridLayout.value) {
      clearStaleResultsForLayoutTransition(traceId);
      if (!isGridLayout.value) {
        resizeHeight(traceId: traceId, reason: "switch from grid layout to list layout");
      } else if (resultGridViewController.rowHeight > 0) {
        resizeHeight(traceId: traceId, reason: "switch from list layout to grid layout");
      }
    }
  }

  // Quick select related methods

  /// Check if the quick select modifier key is still pressed.
  bool isQuickSelectModifierPressed() {
    return Platform.isMacOS ? HardwareKeyboard.instance.isMetaPressed : HardwareKeyboard.instance.isAltPressed;
  }

  /// Start the quick select timer when modifier key is pressed
  void startQuickSelectTimer(String traceId) {
    if (isQuickSelectMode.value || activeResultViewController.items.isEmpty) {
      return;
    }

    Logger.instance.debug(traceId, "Quick select: starting timer");
    isQuickSelectKeyPressed = true;

    quickSelectTimer?.cancel();
    quickSelectTimer = Timer(Duration(milliseconds: quickSelectDelay), () {
      if (isQuickSelectKeyPressed && isQuickSelectModifierPressed()) {
        Logger.instance.debug(traceId, "Quick select: activating mode");
        activateQuickSelectMode(traceId);
      }
    });
  }

  // Core sends repeated action icons as response-local references to keep large
  // query payloads smaller. Resolve them before normal result parsing so the
  // rest of the UI still deals with ordinary WoxImage objects.
  Map<String, WoxImage> _parseQueryActionIconRefs(dynamic rawRefs) {
    if (rawRefs is! Map) {
      return <String, WoxImage>{};
    }

    final refs = <String, WoxImage>{};
    for (final entry in rawRefs.entries) {
      final rawIcon = entry.value;
      if (rawIcon is Map) {
        refs[entry.key.toString()] = WoxImage.fromJson(Map<String, dynamic>.from(rawIcon));
      }
    }
    return refs;
  }

  void _resolveQueryActionIconRefs(List<dynamic> resultsData, Map<String, WoxImage> refs) {
    for (final rawResult in resultsData) {
      if (rawResult is! Map) {
        continue;
      }

      final actions = rawResult['Actions'];
      if (actions is! List) {
        continue;
      }

      for (final rawAction in actions) {
        if (rawAction is! Map) {
          continue;
        }

        final rawIcon = rawAction['Icon'];
        if (rawIcon is! Map || rawIcon['ImageType'] != _queryActionIconRefType) {
          continue;
        }

        final resolvedIcon = refs[rawIcon['ImageData']?.toString() ?? ""];
        if (resolvedIcon != null) {
          rawAction['Icon'] = resolvedIcon.toJson();
        }
      }
    }
  }

  void _scheduleQueryEventLoopTurnTiming({
    required String traceId,
    required String stage,
    required String queryId,
    required int resultCount,
    required bool isFinal,
    required int scheduledTimestampMs,
  }) {
    if (!Env.isDev) {
      return;
    }

    Timer.run(() {
      final tracker = WoxTimeTracker.start(traceId, stage);
      tracker.setRawString("queryId", queryId);
      tracker.setInt("resultCount", resultCount);
      tracker.setBool("isFinal", isFinal);
      tracker.setInt("delayMs", DateTime.now().millisecondsSinceEpoch - scheduledTimestampMs);
      tracker.log();
    });
  }

  /// Stop the quick select timer when modifier key is released
  void stopQuickSelectTimer(String traceId) {
    Logger.instance.debug(traceId, "Quick select: stopping timer");
    isQuickSelectKeyPressed = false;
    quickSelectTimer?.cancel();

    if (isQuickSelectMode.value) {
      deactivateQuickSelectMode(traceId);
    }
  }

  /// Activate quick select mode and add number labels to results
  void activateQuickSelectMode(String traceId) {
    Logger.instance.debug(traceId, "Quick select: activating mode");
    isQuickSelectMode.value = true;
    updateQuickSelectNumbers(traceId);
  }

  /// Deactivate quick select mode and remove number labels
  void deactivateQuickSelectMode(String traceId) {
    Logger.instance.debug(traceId, "Quick select: deactivating mode");
    isQuickSelectMode.value = false;
    updateQuickSelectNumbers(traceId);
  }

  /// Update quick select numbers for all result items
  void updateQuickSelectNumbers(String traceId) {
    var items = activeResultViewController.items;

    // Get visible range
    var visibleRange = _getVisibleItemRange();
    var visibleStartIndex = visibleRange['start'] ?? 0;
    var visibleEndIndex = visibleRange['end'] ?? items.length - 1;

    // Count non-group items in visible range for numbering
    var quickSelectNumber = 1;

    for (int i = 0; i < items.length; i++) {
      var item = items[i].value;

      bool isInVisibleRange = i >= visibleStartIndex && i <= visibleEndIndex;
      bool shouldShowQuickSelect = isQuickSelectMode.value && !item.isGroup && isInVisibleRange && quickSelectNumber <= 9;

      // Update quick select properties
      var updatedItem = item.copyWith(isShowQuickSelect: shouldShowQuickSelect, quickSelectNumber: shouldShowQuickSelect ? quickSelectNumber.toString() : '');
      final hasQuickSelectChanged = item.isShowQuickSelect != updatedItem.isShowQuickSelect || item.quickSelectNumber != updatedItem.quickSelectNumber;

      // Increment number only for non-group items in visible range that get a number
      if (shouldShowQuickSelect) {
        quickSelectNumber++;
      }

      // Directly update the reactive item to trigger UI refresh
      items[i].value = updatedItem;
      if (hasQuickSelectChanged) {
        activeResultViewController.refreshItemByIndex(i);
      }
    }
  }

  /// Get the range of visible items in the result list
  Map<String, int> _getVisibleItemRange() {
    final controller = activeResultViewController;
    if (!controller.scrollController.hasClients || controller.items.isEmpty) {
      return {'start': 0, 'end': controller.items.length - 1};
    }

    final itemHeight = WoxThemeUtil.instance.getResultItemHeight();
    final currentOffset = controller.scrollController.offset;
    final viewportHeight = controller.scrollController.position.viewportDimension;

    if (viewportHeight <= 0) {
      return {'start': 0, 'end': controller.items.length - 1};
    }

    final firstVisibleItemIndex = (currentOffset / itemHeight).floor();
    final visibleItemCount = (viewportHeight / itemHeight).ceil();
    final lastVisibleItemIndex = (firstVisibleItemIndex + visibleItemCount - 1).clamp(0, controller.items.length - 1);

    return {'start': firstVisibleItemIndex.clamp(0, controller.items.length - 1), 'end': lastVisibleItemIndex};
  }

  /// Handle number key press in quick select mode
  bool handleQuickSelectNumberKey(String traceId, int number) {
    if (!isQuickSelectMode.value || number < 1 || number > 9) {
      return false;
    }

    var items = activeResultViewController.items;

    // Get visible range
    var visibleRange = _getVisibleItemRange();
    var visibleStartIndex = visibleRange['start'] ?? 0;
    var visibleEndIndex = visibleRange['end'] ?? items.length - 1;

    // Find the item with the matching quick select number in visible range
    var quickSelectNumber = 1;
    for (int i = visibleStartIndex; i <= visibleEndIndex && i < items.length; i++) {
      var item = items[i].value;

      if (!item.isGroup) {
        if (quickSelectNumber == number) {
          Logger.instance.debug(traceId, "Quick select: selecting item $number at index $i");
          activeResultViewController.updateActiveIndex(traceId, i);
          executeDefaultAction(traceId);
          return true;
        }
        quickSelectNumber++;

        // Stop if we've exceeded the number we're looking for
        if (quickSelectNumber > 9) {
          break;
        }
      }
    }
    return false;
  }

  int calculatePreservedActionIndex(String? oldActionName) {
    var items = actionListViewController.items;

    // If action panel is not visible, use default action
    if (!isShowActionPanel.value) {
      var defaultIndex = items.indexWhere((element) => element.value.data.isDefault);
      return defaultIndex != -1 ? defaultIndex : 0;
    }

    // Try to find the same action by name
    if (oldActionName != null) {
      var sameActionIndex = items.indexWhere((element) => element.value.data.name == oldActionName);
      if (sameActionIndex != -1) {
        return sameActionIndex;
      }
    }

    // Fallback to default action
    var defaultIndex = items.indexWhere((element) => element.value.data.isDefault);
    return defaultIndex != -1 ? defaultIndex : 0;
  }

  String? getCurrentActionName() {
    var oldActionIndex = actionListViewController.activeIndex.value;
    if (actionListViewController.items.isNotEmpty && oldActionIndex < actionListViewController.items.length) {
      return actionListViewController.items[oldActionIndex].value.data.name;
    }
    return null;
  }
}
