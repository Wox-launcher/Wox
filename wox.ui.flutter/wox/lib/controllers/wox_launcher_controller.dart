import 'dart:async';
import 'dart:io';
import 'dart:convert';
import 'dart:ui';

import 'package:extended_text_field/extended_text_field.dart';
import 'package:flutter/foundation.dart';
import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controllers/query_box_text_editing_controller.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_base_list_controller.dart';
import 'package:wox/controllers/wox_grid_controller.dart';
import 'package:wox/controllers/wox_list_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/enums/wox_layout_mode_enum.dart';
import 'package:wox/models/doctor_check_result.dart';
import 'package:wox/utils/wox_theme_util.dart';
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
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/picker.dart';
import 'package:wox/utils/wox_setting_util.dart';

import 'package:wox/utils/wox_websocket_msg_util.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/utils/window_flicker_detector.dart';
import 'package:wox/utils/color_util.dart';

class WoxLauncherController extends GetxController {
  //query related variables
  final currentQuery = PlainQuery.empty().obs;
  // is current query returned results or finished without results
  bool isCurrentQueryReturned = false;
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = QueryBoxTextEditingController(
    selectedTextStyle: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxTextSelectionColor)),
  );
  final queryBoxScrollController = ScrollController(initialScrollOffset: 0.0);
  final queryBoxLineCount = 1.obs;
  final queryBoxTextFieldKey = GlobalKey<ExtendedTextFieldState>();

  //preview related variables
  final currentPreview = WoxPreview.empty().obs;
  final isShowPreviewPanel = false.obs;
  final terminalFindTrigger = 0.obs;
  final isTerminalPreviewFullscreen = false.obs;
  final Map<String, StreamController<Map<String, dynamic>>> terminalChunkControllers = {};
  final Map<String, StreamController<Map<String, dynamic>>> terminalStateControllers = {};
  double lastResultPreviewRatioBeforeTerminalFullscreen = 0.5;

  /// The ratio of result panel width to total width, value range: 0.0-1.0
  /// e.g., 0.3 means result panel takes 30% width, preview panel takes 70%
  final resultPreviewRatio = 0.5.obs;

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

  /// The timer to clear query results.
  /// On every query changed, it will reset the timer and will clear the query results after N ms.
  /// If there is no this delay mechanism, the window will flicker for fast typing.
  Timer clearQueryResultsTimer = Timer(const Duration(), () => {});
  int clearQueryResultDelay = 100; // adaptive based on flicker detection
  final windowFlickerDetector = WindowFlickerDetector();

  /// Timer for debouncing resize height on Windows when height increases
  Timer? resizeHeightDebounceTimer;
  int resizeHeightDebounceDelay = 100; // ms

  /// This flag is used to control whether the user can arrow up to show history when the app is first shown.
  var canArrowUpHistory = true;
  final latestQueryHistories = <QueryHistory>[]; // the latest query histories
  var currentQueryHistoryIndex = 0; //  query history index, used to navigate query history

  /// Pending preserved index for query refresh
  int? pendingPreservedIndex;

  var lastLaunchMode = WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code;
  var lastStartPage = WoxStartPageEnum.WOX_START_PAGE_MRU.code;
  final isInSettingView = false.obs;

  // UI Control Flags
  final isQueryBoxAtBottom = false.obs;
  final isToolbarHiddenForce = false.obs;
  double forceWindowWidth = 0;
  var forceHideOnBlur = false;

  var positionBeforeOpenSetting = const Offset(0, 0);
  // Whether settings was opened when window was hidden (e.g., from tray)
  bool isSettingOpenedFromHidden = false;

  // Performance metrics: Map<traceId, startTime>
  final Map<String, int> queryStartTimeMap = {};

  /// The icon at end of query box.
  final queryIcon = QueryIconInfo.empty().obs;

  /// The result of the doctor check.
  var doctorCheckPassed = true;
  final doctorCheckInfo = DoctorCheckInfo.empty().obs;
  String lastAppliedDoctorToolbarMessage = '';

  // toolbar related variables
  final toolbar = ToolbarInfo.empty().obs;
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

  @override
  void onInit() {
    super.onInit();

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
      if (queryBoxFocusNode.hasFocus) {
        var traceId = const UuidV4().generate();
        hideActionPanel(traceId);
        hideFormActionPanel(traceId);

        // Call API when query box gains focus
        WoxApi.instance.onQueryBoxFocus(traceId);
      }
    });

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

  bool get isShowDoctorCheckInfo => currentQuery.value.isEmpty && !doctorCheckInfo.value.allPassed;

  bool get shouldShowUpdateActionInToolbar {
    if (currentQuery.value.isEmpty == false || doctorCheckInfo.value.allPassed) {
      return false;
    }

    return doctorCheckInfo.value.results.any((result) => result.isVersionIssue);
  }

  bool get isShowToolbar => activeResultViewController.items.isNotEmpty || isShowDoctorCheckInfo;

  bool get isToolbarShowedWithoutResults => isShowToolbar && activeResultViewController.items.isEmpty;

  /// Triggered when received query results from the server.
  void onReceivedQueryResults(String traceId, String queryId, List<WoxQueryResult> receivedResults) {
    // Cancel loading timer and hide loading animation when results are received
    if (queryId == currentQuery.value.queryId) {
      isCurrentQueryReturned = true;
      loadingTimer?.cancel();
      isLoading.value = false;
    } else {
      Logger.instance.error(traceId, "query id is not matched, ignore the results");
      return;
    }

    if (receivedResults.isEmpty) {
      return;
    }

    //cancel clear results timer
    clearQueryResultsTimer.cancel();

    // 1. Use silent mode to avoid triggering onItemActive callback during updateItems (which may cause a little performance issue)
    //    Following resetActiveResult in updateActiveResultIndex will trigger the callback
    // 2. We need update items in both list and grid controllers, because metdata query (grid and list layout change relay on this) may after results arrival,
    //    at this point, we don't know which layout this query will use, so we update both
    final listItems = receivedResults.map((e) => WoxListItem.fromQueryResult(e)).toList();
    resultListViewController.updateItems(traceId, listItems, silent: true);
    resultGridViewController.updateItems(traceId, listItems, silent: true);

    updateActiveResultIndex(traceId);
    updateDoctorToolbarIfNeeded(traceId);
    resizeHeight();
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

    // When there are results (e.g. MRU), keep the right-side hotkeys and only show the warning on the left.
    // When there are no results, show a direct action to open the doctor page.
    if (activeResultViewController.items.isEmpty) {
      final updateAction = buildUpdateToolbarAction();
      final actions = <ToolbarActionInfo>[];
      if (updateAction != null) {
        actions.add(updateAction);
      }
      actions.add(
        ToolbarActionInfo(
          name: tr("plugin_doctor_check"),
          hotkey: "enter",
          action: () {
            onQueryChanged(traceId, PlainQuery.text("doctor "), "user click doctor icon");
          },
        ),
      );

      toolbar.value = ToolbarInfo(text: doctorCheckInfo.value.message, icon: doctorCheckInfo.value.icon, actions: actions);
    } else {
      final updateAction = buildUpdateToolbarAction();
      if (updateAction == null) {
        toolbar.value = toolbar.value.copyWith(text: doctorCheckInfo.value.message, icon: doctorCheckInfo.value.icon);
      } else {
        final mergedActions = List<ToolbarActionInfo>.from(toolbar.value.actions ?? []);
        final updateHotkey = updateAction.hotkey.toLowerCase();
        final hasUpdateAction = mergedActions.any((action) => action.hotkey.toLowerCase() == updateHotkey || action.name == updateAction.name);
        if (!hasUpdateAction) {
          mergedActions.insert(0, updateAction);
        }
        toolbar.value = toolbar.value.copyWith(text: doctorCheckInfo.value.message, icon: doctorCheckInfo.value.icon, actions: mergedActions);
      }
    }

    lastAppliedDoctorToolbarMessage = doctorCheckInfo.value.message;
  }

  void openUpdateFromToolbar(String traceId) {
    onQueryChanged(traceId, PlainQuery.text("update "), "toolbar go to update");
  }

  ToolbarActionInfo? buildUpdateToolbarAction() {
    if (!shouldShowUpdateActionInToolbar) {
      return null;
    }

    return ToolbarActionInfo(name: tr("plugin_doctor_go_to_update"), hotkey: "ctrl+u");
  }

  Future<void> toggleApp(String traceId, ShowAppParams params) async {
    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      if (isInSettingView.value) {
        exitSetting(traceId);
      } else {
        hideApp(traceId);
      }
    } else {
      showApp(traceId, params);
    }
  }

  Future<void> showApp(String traceId, ShowAppParams params) async {
    // update some properties to latest for later use
    latestQueryHistories.assignAll(params.queryHistories);
    lastLaunchMode = params.launchMode;
    lastStartPage = params.startPage;

    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      canArrowUpHistory = true;
      if (lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code) {
        //skip the first one, because it's the current query
        currentQueryHistoryIndex = 0;
      } else {
        currentQueryHistoryIndex = -1;
      }
    }

    // Handle launch mode: fresh or continue
    final isInputWithText = currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code && currentQuery.value.queryText.isNotEmpty;
    final isSelectionQuery = currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code;

    if (lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code) {
      // Fresh mode: clear query if not opened via query hotkey or selection
      if (!isInputWithText && !isSelectionQuery) {
        currentQuery.value = PlainQuery.emptyInput();
        queryBoxTextFieldController.clear();
      }
    }
    // Continue mode: keep last query (do nothing)

    // Handle start page: show content when query is empty (works in both modes)
    if (!isInputWithText && !isSelectionQuery) {
      if (lastStartPage == WoxStartPageEnum.WOX_START_PAGE_MRU.code) {
        queryMRU(traceId);
      } else {
        // Blank page - clear results
        await clearQueryResults(traceId);
      }
    }

    // Handle explorer layout mode
    if (params.layoutMode == WoxLayoutModeEnum.WOX_LAYOUT_MODE_EXPLORER.code) {
      isQueryBoxAtBottom.value = true;
      isToolbarHiddenForce.value = true;
      forceWindowWidth = WoxSettingUtil.instance.currentSetting.appWidth.toDouble() / 2;
      forceHideOnBlur = true;
    }

    if (params.layoutMode == WoxLayoutModeEnum.WOX_LAYOUT_MODE_TRAY_QUERY.code) {
      isQueryBoxAtBottom.value = Platform.isWindows;
      isToolbarHiddenForce.value = true;
      final configuredTrayWidth = params.windowWidth;
      forceWindowWidth = configuredTrayWidth > 0 ? configuredTrayWidth.toDouble() : WoxSettingUtil.instance.currentSetting.appWidth.toDouble() / 2;
      forceHideOnBlur = true;
    }

    // Reset to default layout if no layout mode specified
    if (params.layoutMode == null || params.layoutMode == WoxLayoutModeEnum.WOX_LAYOUT_MODE_DEFAULT.code) {
      setDefaultLayoutMode(traceId);
    }

    // Handle different position types
    // on linux, we need to show first and then set position or center it
    if (Platform.isLinux) {
      await windowManager.show();
    }
    final targetPosition = Offset(params.position.x.toDouble(), params.position.y.toDouble());
    // Apply position+size together before showing to avoid opening with stale width.
    final initialHeight = getQueryBoxTotalHeight();
    final targetWidth = forceWindowWidth != 0 ? forceWindowWidth : WoxSettingUtil.instance.currentSetting.appWidth.toDouble();
    await windowManager.setBounds(targetPosition, Size(targetWidth, initialHeight));

    // Set always-on-top BEFORE show() so the TOPMOST flag is already in place
    // when the window becomes visible, avoiding transient blur on Windows.
    if (!isInSettingView.value) {
      await windowManager.setAlwaysOnTop(true);
    }
    await windowManager.show();
    await windowManager.focus();
    focusQueryBox(selectAll: params.selectAll);

    if (params.isQueryFocus) {
      Logger.instance.debug(traceId, "need to auto focus to chat input on show app (query focus)");
      if (isShowPreviewPanel.value && currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code) {
        final chatController = Get.find<WoxAIChatController>();
        chatController.focusToChatInput(traceId);
        chatController.collapseLeftPanel();
      }
    }

    WoxApi.instance.onShow(traceId);
  }

  void setDefaultLayoutMode(String traceId) {
    isQueryBoxAtBottom.value = false;
    isToolbarHiddenForce.value = false;
    forceWindowWidth = 0;
    forceHideOnBlur = false;
  }

  Future<void> hideApp(String traceId) async {
    //clear query box text if query type is selection or launch mode is fresh
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code || lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code) {
      currentQuery.value = PlainQuery.emptyInput();
      queryBoxTextFieldController.clear();
      hideActionPanel(traceId);
      hideFormActionPanel(traceId);
      await clearQueryResults(traceId);
    }

    hideActionPanel(traceId);
    hideFormActionPanel(traceId);

    // Clean up quick select state
    if (isQuickSelectMode.value) {
      deactivateQuickSelectMode(traceId);
    }
    quickSelectTimer?.cancel();
    isQuickSelectKeyPressed = false;
    isSettingOpenedFromHidden = false;
    isInSettingView.value = false;
    await WoxApi.instance.onSetting(traceId, false);
    setDefaultLayoutMode(traceId);

    await windowManager.hide();
    await WoxApi.instance.onHide(traceId);
  }

  void saveWindowPositionIfNeeded() {
    final setting = WoxSettingUtil.instance.currentSetting;
    if (setting.showPosition == WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code) {
      // Run in async task with delay to ensure window position is fully updated
      Future.delayed(const Duration(milliseconds: 500), () async {
        final traceId = const UuidV4().generate();
        try {
          final position = await windowManager.getPosition();
          await WoxApi.instance.saveWindowPosition(traceId, position.dx.toInt(), position.dy.toInt());
        } catch (e) {
          Logger.instance.error(traceId, "Failed to save window position: $e");
        }
      });
    }
  }

  Future<void> toggleActionPanel(String traceId) async {
    if (activeResultViewController.items.isEmpty) {
      return;
    }

    if (isShowActionPanel.value) {
      hideActionPanel(traceId);
    } else {
      showActionPanel(traceId);
    }
  }

  bool isActionHotkey(HotKey hotkey) {
    if (Platform.isMacOS) {
      return WoxHotkey.equals(hotkey, WoxHotkey.parseHotkeyFromString("cmd+J")!.normalHotkey);
    } else {
      return WoxHotkey.equals(hotkey, WoxHotkey.parseHotkeyFromString("alt+J")!.normalHotkey);
    }
  }

  void hideActionPanel(String traceId) {
    isShowActionPanel.value = false;
    actionListViewController.clearFilter(traceId);
    focusQueryBox();
    resizeHeight();
  }

  void showFormActionPanel(WoxResultAction action, String resultId) {
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
    resizeHeight();
  }

  void hideFormActionPanel(String traceId) {
    activeFormAction.value = null;
    activeFormResultId.value = "";
    formActionValues.clear();
    isShowFormActionPanel.value = false;
    focusQueryBox();
    resizeHeight();
  }

  Future<void> focusQueryBox({bool selectAll = false}) async {
    // only focus when window is visible
    // otherwise it will gain focus but not visible, causing some issues on windows
    // e.g. active window snapshot is wrong
    final isVisible = await windowManager.isVisible();
    if (!isVisible) {
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

    // by default requestFocus will select all text, if selectAll is false, then restore to the previously stored cursor position
    if (selectAll) {
      queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
    }
  }

  void showActionPanel(String traceId) {
    isShowActionPanel.value = true;
    SchedulerBinding.instance.addPostFrameCallback((_) {
      actionListViewController.requestFocus();
    });
    resizeHeight();
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  WoxQueryResult? getActiveResult() {
    final controller = activeResultViewController;
    if (controller.activeIndex.value >= controller.items.length || controller.activeIndex.value < 0 || controller.items.isEmpty) {
      return null;
    }

    return controller.items[controller.activeIndex.value].value.data;
  }

  /// given a hotkey, find the action in the result
  WoxResultAction? getActionByHotkey(WoxQueryResult? result, HotKey hotkey) {
    if (result == null) {
      return null;
    }

    var filteredActions = result.actions.where((action) {
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

  Future<void> executeAction(String traceId, WoxQueryResult? result, WoxResultAction? action) async {
    Logger.instance.debug(traceId, "user execute result action: ${action?.name}");

    if (result == null) {
      Logger.instance.error(traceId, "active query result is null");
      return;
    }
    if (action == null) {
      Logger.instance.error(traceId, "active action is null");
      return;
    }

    var preventHideAfterAction = action.preventHideAfterAction;
    Logger.instance.debug(traceId, "execute action: ${action.name}, prevent hide after action: $preventHideAfterAction");

    if (action.type == "form") {
      showFormActionPanel(action, result.id);
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
    hideFormActionPanel(traceId);
  }

  Future<void> submitFormAction(String traceId, Map<String, String> values) async {
    final action = activeFormAction.value;
    final resultId = activeFormResultId.value;
    final queryId = currentQuery.value.queryId;
    if (action == null || resultId.isEmpty) {
      hideFormActionPanel(traceId);
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

    hideFormActionPanel(traceId);
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

  void onQueryBoxTextChanged(String value) {
    canArrowUpHistory = false;
    resultListViewController.isMouseMoved = false;
    resultGridViewController.isMouseMoved = false;

    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      updateQueryBoxLineCount(value);
      // do local filter if query type is selection
      final traceId = const UuidV4().generate();
      resultListViewController.filterItems(traceId, value);
      resultGridViewController.filterItems(traceId, value);
      // there maybe no results after filtering, we need to resize height to hide the action panel
      // or show the preview panel
      resizeHeight();
    } else {
      onQueryChanged(
        const UuidV4().generate(),
        PlainQuery(queryId: const UuidV4().generate(), queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: value, querySelection: Selection.empty()),
        "user input changed",
      );
    }
  }

  Future<void> queryMRU(String traceId) async {
    var startTime = DateTime.now().millisecondsSinceEpoch;
    var queryId = const UuidV4().generate();
    currentQuery.value = PlainQuery.emptyInput();
    currentQuery.value.queryId = queryId;
    updatePluginMetadataOnQueryChanged(traceId, currentQuery.value);

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
      onReceivedQueryResults(traceId, queryId, results);
      var endTime = DateTime.now().millisecondsSinceEpoch;
      Logger.instance.debug(traceId, "queryMRU via websocket took ${endTime - startTime} ms");
    } catch (e) {
      Logger.instance.error(traceId, "Failed to query MRU: $e");
      clearQueryResults(traceId);
    }
  }

  Future<void> onQueryChanged(String traceId, PlainQuery query, String changeReason, {bool moveCursorToEnd = false}) async {
    Logger.instance.debug(traceId, "query changed: ${query.queryText}, reason: $changeReason");

    if (query.queryId == "") {
      query.queryId = const UuidV4().generate();
    }

    clearHoveredResult();

    //hide setting view if query changed
    if (isInSettingView.value) {
      isInSettingView.value = false;
      await WoxApi.instance.onSetting(traceId, false);
    }

    currentQuery.value = query;
    isCurrentQueryReturned = false;
    isShowActionPanel.value = false;
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      canArrowUpHistory = false;
    }

    if (queryBoxTextFieldController.text != query.queryText) {
      queryBoxTextFieldController.text = query.queryText;
    }
    if (moveCursorToEnd) {
      moveQueryBoxCursorToEnd();
    }
    updateQueryBoxLineCount(query.queryText);
    updateQueryBoxLineCount(query.queryText);

    // Cancel previous loading timer and reset loading state
    loadingTimer?.cancel();
    // Only reset loading state if it is currently true to avoid unnecessary rebuilds
    if (isLoading.value) {
      isLoading.value = false;
    }

    updatePluginMetadataOnQueryChanged(traceId, query).then((isPluginQuery) {
      if (!isPluginQuery) return;
      if (currentQuery.value.queryId != query.queryId) return;
      // If query has returned (isFinal received or results arrived), don't start loading animation
      if (isCurrentQueryReturned) return;

      // Logic to prevent starting the timer if results have already arrived (Race Condition Fix)
      // Check if we currently have results for this query
      bool hasResults = activeResultViewController.items.isNotEmpty && activeResultViewController.items.first.value.data.queryId == query.queryId;

      if (!hasResults) {
        loadingTimer = Timer(loadingDelay, () {
          // Double check before showing loading:
          // 1. Query is still the same
          // 2. We still don't have results (or results matching this query)
          bool stillNoResults = activeResultViewController.items.isEmpty || activeResultViewController.items.first.value.data.queryId != query.queryId;
          if (currentQuery.value.queryId == query.queryId && stillNoResults && !isCurrentQueryReturned) {
            isLoading.value = true;
          }
        });
      }
    });

    if (query.isEmpty) {
      // Check if we should show MRU results when query is empty (based on start page setting)
      if (lastStartPage == WoxStartPageEnum.WOX_START_PAGE_MRU.code) {
        queryMRU(traceId);
      } else {
        clearQueryResults(traceId);
      }
      return;
    }

    // delay clear results, otherwise windows height will shrink immediately,
    // and then the query result is received which will expand the windows height. so it will causes window flicker
    clearQueryResultsTimer.cancel();

    final currentQueryId = query.queryId;
    final isVisible = await windowManager.isVisible();
    // If app is hidden (e.g. tray query will trigger change query first then showapp), clear immediately so old results won't flash when shown.
    if (!isVisible) {
      await clearQueryResults(traceId);
      Logger.instance.debug(traceId, "clear query results immediately because window is hidden");
    } else {
      // delay clear results, otherwise windows height will shrink immediately,
      // and then the query result is received which will expand the windows height. so it will causes window flicker
      // Adaptive: adjust clearQueryResultDelay based on recent resize flicker
      // Note: clearQueryResultDelay may have been set by onRefreshQuery for longer delay
      if (changeReason != "refresh query") {
        final adjust = windowFlickerDetector.adjustClearDelay(clearQueryResultDelay);
        clearQueryResultDelay = adjust.newDelay;
        Logger.instance.debug(
          const UuidV4().generate(),
          "Adaptive clear delay: $clearQueryResultDelay ms (flicker=${adjust.status.flicker}, reason=${adjust.status.reason}, events=${adjust.status.events})",
        );
      }

      clearQueryResultsTimer = Timer(Duration(milliseconds: clearQueryResultDelay), () {
        if (currentQuery.value.queryId != currentQueryId) return;

        final hasResultsNow = activeResultViewController.items.isNotEmpty && activeResultViewController.items.first.value.data.queryId == currentQueryId;
        if (hasResultsNow) return;

        clearQueryResults(traceId);
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
        data: {"queryId": query.queryId, "queryType": query.queryType, "queryText": query.queryText, "querySelection": query.querySelection.toJson()},
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

    // Set longer delay for clearing results to avoid flicker
    // since refresh query usually returns similar results
    clearQueryResultDelay = 400;

    // Get current query and create a new query with the same content but new ID
    final currentQueryValue = currentQuery.value;
    final refreshedQuery = PlainQuery(
      queryId: const UuidV4().generate(),
      queryType: currentQueryValue.queryType,
      queryText: currentQueryValue.queryText,
      querySelection: currentQueryValue.querySelection,
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
      await onQueryChanged(msg.traceId, PlainQuery.fromJson(msg.data), "receive change query from wox", moveCursorToEnd: true);
      focusQueryBox();
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "RefreshQuery") {
      final preserveSelectedIndex = msg.data['preserveSelectedIndex'] as bool? ?? false;
      onRefreshQuery(msg.traceId, preserveSelectedIndex);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ChangeTheme") {
      final theme = WoxTheme.fromJson(msg.data);
      WoxThemeUtil.instance.changeTheme(theme);
      resizeHeight(); // Theme height maybe changed, so we need to resize height
      // Theme change triggers widget rebuild which may lose focus, so we need to restore focus after rebuild
      SchedulerBinding.instance.addPostFrameCallback((_) {
        focusQueryBox();
      });
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "PickFiles") {
      final pickFilesParams = FileSelectorParams.fromJson(msg.data);
      final files = await FileSelector.pick(msg.traceId, pickFilesParams);
      responseWoxWebsocketRequest(msg, true, files);
    } else if (msg.method == "OpenSettingWindow") {
      openSetting(msg.traceId, SettingWindowContext.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ShowToolbarMsg") {
      showToolbarMsg(msg.traceId, ToolbarMsg.fromJson(msg.data));
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
    } else if (msg.method == "UpdateResult") {
      final success = updateResult(msg.traceId, UpdatableResult.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, success);
    } else if (msg.method == "PushResults") {
      final data = msg.data as Map<String, dynamic>? ?? {};
      final queryId = data['QueryId'] as String? ?? "";
      final resultsData = data['Results'] as List<dynamic>? ?? [];
      final results = resultsData.map((item) => WoxQueryResult.fromJson(item)).toList();
      final success = pushResults(msg.traceId, queryId, results);
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

    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
      // Log WebSocket latency (Wox -> UI) only for Query method
      if (msg.sendTimestamp > 0) {
        final receiveTimestamp = DateTime.now().millisecondsSinceEpoch;
        final latency = receiveTimestamp - msg.sendTimestamp;
        if (latency > 10) {
          Logger.instance.info(msg.traceId, "📨 WebSocket latency (Wox→UI): ${latency}ms");
        }
      }

      // Parse QueryResponse object
      final queryResponse = msg.data as Map<String, dynamic>;
      final resultsData = queryResponse['Results'] as List<dynamic>;
      final queryId = queryResponse['QueryId'] as String? ?? "";
      final isFinal = queryResponse['IsFinal'] as bool? ?? false;

      var results = <WoxQueryResult>[];
      for (var item in resultsData) {
        results.add(WoxQueryResult.fromJson(item));
      }

      Logger.instance.info(msg.traceId, "Received websocket message: ${msg.method}, results count: ${results.length}, isFinal: $isFinal");

      // Process results first
      onReceivedQueryResults(msg.traceId, queryId, results);

      // If this is the final final response, we must stop loading animation explicitly
      // This handles cases where results are empty but the query is finished
      // We explicitly check if this final response belongs to the current query
      if (isFinal && queryId == currentQuery.value.queryId) {
        loadingTimer?.cancel();
        if (isLoading.value) {
          isLoading.value = false;
        }
      }

      // Record First Paint after results are rendered (use post-frame callback)
      final queryStartTime = queryStartTimeMap[msg.traceId];
      if (results.isNotEmpty && queryStartTime != null) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          // Check if this traceId still exists (not removed by Complete Paint)
          if (queryStartTimeMap.containsKey(msg.traceId)) {
            final firstPaintTime = DateTime.now().millisecondsSinceEpoch - queryStartTime;
            Logger.instance.info(msg.traceId, "⚡ FIRST PAINT: ${firstPaintTime}ms (${results.length} results rendered)");
            // Remove after recording First Paint to avoid recording it again
            queryStartTimeMap.remove(msg.traceId);
          }
        });
      }

      // Record Complete Paint when backend signals final batch
      if (isFinal && queryStartTime != null) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          // Check if this traceId still exists (might be removed by First Paint)
          final startTime = queryStartTimeMap[msg.traceId];
          if (startTime != null) {
            final completePaintTime = DateTime.now().millisecondsSinceEpoch - startTime;
            Logger.instance.info(msg.traceId, "🎨 COMPLETE PAINT: ${completePaintTime}ms (total ${activeResultViewController.items.length} results rendered)");
            // Clean up to avoid memory leak
            queryStartTimeMap.remove(msg.traceId);
          }
        });
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

  bool toggleTerminalPreviewFullscreen(String traceId) {
    if (!isShowPreviewPanel.value || currentPreview.value.previewType != WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code) {
      return false;
    }

    if (isTerminalPreviewFullscreen.value) {
      final restoreRatio = lastResultPreviewRatioBeforeTerminalFullscreen <= 0 ? 0.5 : lastResultPreviewRatioBeforeTerminalFullscreen;
      resultPreviewRatio.value = restoreRatio;
      isTerminalPreviewFullscreen.value = false;
      Logger.instance.debug(traceId, "terminal preview exit fullscreen, ratio restored: $restoreRatio");
      return true;
    }

    if (resultPreviewRatio.value > 0) {
      lastResultPreviewRatioBeforeTerminalFullscreen = resultPreviewRatio.value;
    }
    resultPreviewRatio.value = 0;
    isTerminalPreviewFullscreen.value = true;
    Logger.instance.debug(traceId, "terminal preview enter fullscreen");
    return true;
  }

  void syncTerminalPreviewFullscreenState() {
    final isTerminalPreviewVisible = isShowPreviewPanel.value && currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code;
    if (isTerminalPreviewVisible) {
      return;
    }

    if (isTerminalPreviewFullscreen.value) {
      final restoreRatio = lastResultPreviewRatioBeforeTerminalFullscreen <= 0 ? 0.5 : lastResultPreviewRatioBeforeTerminalFullscreen;
      resultPreviewRatio.value = restoreRatio;
    }
    isTerminalPreviewFullscreen.value = false;
  }

  Future<void> clearQueryResults(String traceId) async {
    Logger.instance.debug(traceId, "clear query results");
    resultListViewController.clearItems();
    resultGridViewController.clearItems();
    actionListViewController.clearItems();
    isShowPreviewPanel.value = false;
    isShowActionPanel.value = false;
    syncTerminalPreviewFullscreenState();

    if (isShowDoctorCheckInfo) {
      Logger.instance.debug(traceId, "update toolbar to doctor warning, query is empty and doctor check not passed");
    } else {
      Logger.instance.debug(traceId, "update toolbar to empty because of query changed and is empty");
      toolbar.value = toolbar.value.emptyRightSide();
    }

    updateDoctorToolbarIfNeeded(traceId);
    await resizeHeight();
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

  Future<void> resizeHeight() async {
    // Don't resize when in setting view, setting view has its own fixed size (1200x800)
    if (isInSettingView.value) {
      return;
    }
    final maxResultCount = WoxSettingUtil.instance.currentSetting.maxResultCount;
    final maxHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(maxResultCount);
    final itemCount = activeResultViewController.items.length;
    double resultHeight;

    if (isInGridMode()) {
      resultHeight = resultGridViewController.calculateGridHeight();
    } else {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(itemCount);
    }

    if (resultHeight > maxHeight) {
      resultHeight = maxHeight;
    }
    if (isShowActionPanel.value || isShowPreviewPanel.value || isShowFormActionPanel.value) {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(maxResultCount);
    }

    if (itemCount > 0) {
      resultHeight += WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingTop + WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingBottom;
    }
    // Only add toolbar height when toolbar is actually shown in UI
    if (isShowToolbar && !isToolbarHiddenForce.value) {
      resultHeight += WoxThemeUtil.instance.getToolbarHeight();
    }
    var totalHeight = getQueryBoxTotalHeight() + resultHeight;

    // on windows, if the resolution is scaled in high DPI, the height need to add addtional 1 pixel to avoid cut off
    // otherwise, "Bottom overflowed by 0.x pixels" will happen
    if (Platform.isWindows) {
      if (PlatformDispatcher.instance.views.first.devicePixelRatio > 1) {
        totalHeight = totalHeight + 1;
      }
    }

    // if toolbar is showed without results, remove the bottom padding, This way, the toolbar can blend seamlessly with the query box.
    if (isToolbarShowedWithoutResults) {
      totalHeight -= WoxThemeUtil.instance.currentTheme.value.appPaddingBottom;
    }

    double targetWidth = forceWindowWidth != 0 ? forceWindowWidth : WoxSettingUtil.instance.currentSetting.appWidth.toDouble();
    if (isQueryBoxAtBottom.value) {
      // In explorer/tray-query mode, we anchor to the bottom.
      // Use getPosition + getSize to compute the current bottom edge, then adjust top to grow upward.
      final pos = await windowManager.getPosition();
      final currentSize = await windowManager.getSize();
      double currentBottom = pos.dy + currentSize.height;

      if (currentBottom <= 0) {
        // Fallback if bounds are weird
      } else {
        double newTop = currentBottom - totalHeight;

        // Apply position and size together to avoid intermediate-frame flicker.
        await windowManager.setBounds(Offset(pos.dx, newTop), Size(targetWidth, totalHeight));

        windowFlickerDetector.recordResize(totalHeight.toInt());
        return;
      }
    }

    // Windows-specific debounce: if height is increasing, debounce and only apply the last resize.
    // If height is decreasing or same, resize immediately.
    if (Platform.isWindows && !isQueryBoxAtBottom.value) {
      final currentSize = await windowManager.getSize();
      if (totalHeight > currentSize.height) {
        resizeHeightDebounceTimer?.cancel();
        resizeHeightDebounceTimer = Timer(Duration(milliseconds: resizeHeightDebounceDelay), () async {
          await windowManager.setSize(Size(targetWidth, totalHeight.toDouble()));
          windowFlickerDetector.recordResize(totalHeight.toInt());
        });
        return;
      }
    }

    resizeHeightDebounceTimer?.cancel();
    await windowManager.setSize(Size(targetWidth, totalHeight.toDouble()));
    windowFlickerDetector.recordResize(totalHeight.toInt());
  }

  void updateQueryBoxLineCount(String text) {
    final normalizedText = text.replaceAll('\r\n', '\n');
    final rawLineCount = normalizedText.isEmpty ? 1 : normalizedText.split('\n').length;
    final clampedLineCount = rawLineCount.clamp(1, QUERY_BOX_MAX_LINES);
    if (queryBoxLineCount.value == clampedLineCount) {
      return;
    }
    queryBoxLineCount.value = clampedLineCount;
    resizeHeight();
  }

  double getQueryBoxInputHeight() {
    final extraLines = queryBoxLineCount.value - 1;
    return QUERY_BOX_BASE_HEIGHT + (QUERY_BOX_LINE_HEIGHT * extraLines);
  }

  double getQueryBoxTotalHeight() {
    final extraLines = queryBoxLineCount.value - 1;
    return WoxThemeUtil.instance.getQueryBoxHeight() + (QUERY_BOX_LINE_HEIGHT * extraLines);
  }

  void clearHoveredResult() {
    resultListViewController.clearHoveredResult();
    resultGridViewController.clearHoveredResult();
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
            // Grid layout doesn't support preview panel
            isShowPreviewPanel.value = !isInGridMode() && currentPreview.value.previewData != "";
            syncTerminalPreviewFullscreenState();

            // If preview panel visibility changed, resize window height
            if (oldShowPreview != isShowPreviewPanel.value) {
              resizeHeight();
            }
          }

          if (updatableResult.actions != null) {
            // Optimization: Check if actions actually changed to avoid unnecessary repaint
            var currentActions = actionListViewController.items.map((e) => e.value.data).toList();
            if (!listEquals(currentActions, updatableResult.actions!)) {
              // Save user's current selection before updateItems (which calls filterItems and resets index)
              var oldActionName = getCurrentActionName();

              var actions = updatedData.actions.map((e) => WoxListItem.fromResultAction(e)).toList();
              actionListViewController.updateItems(traceId, actions);

              // Restore user's selected action after refresh
              var newActiveIndex = calculatePreservedActionIndex(oldActionName);
              if (actionListViewController.activeIndex.value != newActiveIndex) {
                actionListViewController.updateActiveIndex(traceId, newActiveIndex);
              }

              // Update toolbar with all actions with hotkeys
              updateToolbarWithActions(traceId, updatedData.actions);
            }
          }
        }
      }

      return true; // Successfully found and updated the result
    } catch (e) {
      // Result not found in current items (no longer visible)
      return false;
    }
  }

  bool pushResults(String traceId, String queryId, List<WoxQueryResult> results) {
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

    for (var result in results) {
      if (result.queryId.isEmpty) {
        result.queryId = queryId;
      }
    }

    onReceivedQueryResults(traceId, queryId, results);
    return true;
  }

  /// Process doctor check results and update the doctor check info
  DoctorCheckInfo processDoctorCheckResults(List<DoctorCheckResult> results) {
    // Check if all tests passed
    bool allPassed = true;
    for (var result in results) {
      if (!result.passed) {
        allPassed = false;
        break;
      }
    }
    doctorCheckPassed = allPassed;

    // Determine appropriate icon and message based on issue type
    WoxImage icon = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: QUERY_ICON_DOCTOR_WARNING);
    String message = "";

    for (var result in results) {
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
    var results = await WoxApi.instance.doctorCheck(traceId);
    final checkInfo = processDoctorCheckResults(results);
    doctorCheckInfo.value = checkInfo;
    updateDoctorToolbarIfNeeded(traceId);
    Logger.instance.debug(traceId, "doctor check result: ${checkInfo.allPassed}, details: ${checkInfo.results.length} items");
  }

  @override
  void dispose() {
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
    super.dispose();
  }

  Future<void> openSetting(String traceId, SettingWindowContext context) async {
    final settingController = Get.find<WoxSettingController>();

    // Save current position before switching (used if we return to launcher)
    try {
      positionBeforeOpenSetting = await windowManager.getPosition();
    } catch (_) {}

    // Mark whether settings opened while window is hidden (e.g., from tray)
    var isVisible = await windowManager.isVisible();
    isSettingOpenedFromHidden = !isVisible;
    isInSettingView.value = true;
    await WoxApi.instance.onSetting(traceId, true);

    // Preload theme/settings for settings view
    await WoxThemeUtil.instance.loadTheme(traceId);
    await settingController.reloadSetting(traceId);
    settingController.activeNavPath.value = 'general';

    // Keep settings in sync with runtime and plugin state when opening.
    unawaited(settingController.refreshRuntimeStatuses());
    unawaited(settingController.reloadPlugins(traceId));

    if (context.path == "/plugin/setting") {
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        await Future.delayed(const Duration(milliseconds: 100));
        await settingController.switchToPluginList(traceId, false);
        settingController.filterPluginKeywordController.text = context.param;
        settingController.filterPlugins();
        settingController.setFirstFilteredPluginDetailActive();
        settingController.switchToPluginSettingTab();
      });
    }
    if (context.path == "/data") {
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        await Future.delayed(const Duration(milliseconds: 100));
        await settingController.switchToDataView(traceId);
      });
    }

    await windowManager.setAlwaysOnTop(false);
    await windowManager.setSize(const Size(1200, 800));
    if (Platform.isLinux) {
      // On Linux we need to show first before positioning works reliably
      await windowManager.show();
      await windowManager.center(1200, 800);
    } else {
      await windowManager.center(1200, 800);
      await windowManager.show();
    }
    await windowManager.focus();

    WidgetsBinding.instance.addPostFrameCallback((_) async {
      await Future.delayed(const Duration(milliseconds: 50));
      settingController.settingFocusNode.requestFocus();
    });

    // On Windows, ensure focus is properly set after window is shown
    if (Platform.isWindows) {
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        await Future.delayed(const Duration(milliseconds: 100));
        await windowManager.focus();
        settingController.settingFocusNode.requestFocus();
        Logger.instance.info(traceId, "[SETTING] Windows focus requested after delay");
      });
    }
  }

  Future<void> exitSetting(String traceId) async {
    closeAllDialogsInSetting();

    if (isSettingOpenedFromHidden) {
      // For hidden-opened settings, exit means hide the window directly
      await hideApp(traceId);
      return;
    }

    // Switch back to launcher
    isInSettingView.value = false;
    await WoxApi.instance.onSetting(traceId, false);
    await windowManager.setAlwaysOnTop(true);
    await resizeHeight();
    await windowManager.setPosition(positionBeforeOpenSetting);
    await windowManager.focus();
    focusQueryBox(selectAll: true);
  }

  void closeAllDialogsInSetting() {
    final navigator = Get.key.currentState;
    if (navigator == null) {
      return;
    }

    navigator.popUntil((route) => route.isFirst);
  }

  void showToolbarMsg(String traceId, ToolbarMsg msg) {
    // Snooze/mute enforcement is handled by backend before pushing to UI.

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

    // Only use toolbar action callbacks when there are no results.
    // Otherwise Enter should execute the active result.
    if (activeResultViewController.items.isEmpty && toolbar.value.actions != null && toolbar.value.actions!.isNotEmpty) {
      // Find the default action (with Enter hotkey) or use the last one
      var defaultToolbarAction = toolbar.value.actions!.firstWhereOrNull((action) => action.hotkey.toLowerCase() == "enter");
      defaultToolbarAction ??= toolbar.value.actions!.last;

      if (defaultToolbarAction.action != null) {
        Logger.instance.debug(traceId, "executing toolbar action callback: ${defaultToolbarAction.name}");
        defaultToolbarAction.action!.call();
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
      // Find default action
      actionToExecute = activeResult.data.actions.firstWhereOrNull((action) => action.isDefault);
      if (actionToExecute == null && activeResult.data.actions.isNotEmpty) {
        // If no default action, use the first action
        actionToExecute = activeResult.data.actions.first;
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
        previewProperties: {},
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
    // Grid layout doesn't support preview panel
    isShowPreviewPanel.value = !isInGridMode() && currentPreview.value.previewData != "";
    syncTerminalPreviewFullscreenState();

    // update actions list
    var actions = item.data.actions.map((e) => WoxListItem.fromResultAction(e)).toList();
    actionListViewController.updateItems(traceId, actions);

    // update active index to default action
    var defaultActionIndex = actions.indexWhere((element) => element.data.isDefault);
    if (defaultActionIndex != -1) {
      actionListViewController.updateActiveIndex(traceId, defaultActionIndex);
    }

    // update toolbar to show all actions with hotkeys
    updateToolbarWithActions(traceId, item.data.actions);
  }

  void onResultItemsEmpty(String traceId) {
    // Hide preview panel when there are no results after filtering
    // otherwise in selection mode, when no result filtered, preview may still be shown
    isShowPreviewPanel.value = false;
    currentPreview.value = WoxPreview.empty();
    syncTerminalPreviewFullscreenState();

    // Clear toolbar actions so height isn't reserved by resizeHeight
    toolbar.value = toolbar.value.emptyRightSide();
  }

  void updateToolbarWithActions(String traceId, List<WoxResultAction> actions) {
    // Filter actions that have hotkeys
    var actionsWithHotkeys = actions.where((action) => action.hotkey.isNotEmpty).toList();

    // Check if we should show "More Actions" hotkey
    // Only show when there are >= 1 actions (regardless of whether they have hotkeys)
    final shouldShowMoreActions = actions.isNotEmpty;

    if (actionsWithHotkeys.isEmpty && !shouldShowMoreActions) {
      // No actions with hotkeys and no actions at all, clear toolbar right side
      toolbar.value = toolbar.value.emptyRightSide();
      return;
    }

    // Sort actions: non-default actions first, then default action (Enter) at the end
    actionsWithHotkeys.sort((a, b) {
      if (a.isDefault && !b.isDefault) return 1; // a is default, move to end
      if (!a.isDefault && b.isDefault) return -1; // b is default, move to end
      return 0; // keep original order
    });

    // Build toolbar action info list
    var toolbarActions =
        actionsWithHotkeys.map((action) {
          return ToolbarActionInfo(name: tr(action.name), hotkey: action.hotkey);
        }).toList();

    final updateAction = buildUpdateToolbarAction();
    if (updateAction != null) {
      final updateHotkey = updateAction.hotkey.toLowerCase();
      final hasUpdateAction = toolbarActions.any((action) => action.hotkey.toLowerCase() == updateHotkey || action.name == updateAction.name);
      if (!hasUpdateAction) {
        toolbarActions.insert(0, updateAction);
      }
    }

    // Add "More Actions" hotkey at the end if there are actions
    if (shouldShowMoreActions) {
      final moreActionsHotkey = Platform.isMacOS ? "cmd+j" : "alt+j";
      toolbarActions.add(ToolbarActionInfo(name: tr("toolbar_more_actions"), hotkey: moreActionsHotkey));
    }

    // Update toolbar with all actions
    toolbar.value = toolbar.value.copyWith(actions: toolbarActions);
  }

  Future<void> handleDropFiles(DropDoneDetails details) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, "Received drop files: $details");

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

  /// Update the plugin metadata based on the query
  /// E.g. plugin icon, plugin features, etc.
  Future<bool> updatePluginMetadataOnQueryChanged(String traceId, PlainQuery query) async {
    var queryMetadata = QueryMetadata(icon: WoxImage.empty(), resultPreviewWidthRatio: 0.5, isGridLayout: false, gridLayoutParams: GridLayoutParams.empty());
    var isPluginQuery = false;

    if (!query.isEmpty && query.queryText.contains(" ")) {
      // if there is space in the query, then this  may be a plugin query, fetch metadata
      try {
        queryMetadata = await WoxApi.instance.getQueryMetadata(traceId, query);
        if (queryMetadata.icon.imageData.isNotEmpty) {
          isPluginQuery = true;
        }
        Logger.instance.debug(
          traceId,
          "fetched query metadata: isPluginQuery=$isPluginQuery, resultPreviewWidthRatio=${queryMetadata.resultPreviewWidthRatio}, isGridLayout=${queryMetadata.isGridLayout}",
        );
      } catch (e) {
        Logger.instance.error(traceId, "query metadata failed: $e");
      }
    }

    updateQueryIconOnQueryChanged(traceId, query, queryMetadata);
    updateResultPreviewWidthRatioOnQueryChanged(traceId, query, queryMetadata);
    updateGridLayoutParamsOnQueryChanged(traceId, query, queryMetadata);
    return isPluginQuery;
  }

  /// Change the query icon based on the query
  Future<void> updateQueryIconOnQueryChanged(String traceId, PlainQuery query, QueryMetadata queryMetadata) async {
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code) {
        queryIcon.value = QueryIconInfo(icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_FILE));
      }
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code) {
        queryIcon.value = QueryIconInfo(icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_TEXT));
      }
      return;
    }

    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      // if there is no space in the query, then this must be a global query
      if (!query.queryText.contains(" ")) {
        queryIcon.value = QueryIconInfo.empty();
        return;
      }

      queryIcon.value = QueryIconInfo(icon: queryMetadata.icon);
      return;
    }

    queryIcon.value = QueryIconInfo.empty();
  }

  /// Update the result preview width ratio based on the query
  Future<void> updateResultPreviewWidthRatioOnQueryChanged(String traceId, PlainQuery query, QueryMetadata queryMetadata) async {
    if (query.isEmpty) {
      resultPreviewRatio.value = 0.5;
      return;
    }
    // if there is no space in the query, then this must be a global query
    if (!query.queryText.contains(" ")) {
      resultPreviewRatio.value = 0.5;
      return;
    }

    Logger.instance.debug(traceId, "update result preview width ratio: ${queryMetadata.resultPreviewWidthRatio}");
    resultPreviewRatio.value = queryMetadata.resultPreviewWidthRatio;
  }

  Future<void> updateGridLayoutParamsOnQueryChanged(String traceId, PlainQuery query, QueryMetadata queryMetadata) async {
    final wasGridLayout = isGridLayout.value;
    if (query.isEmpty) {
      isGridLayout.value = false;
      gridLayoutParams.value = GridLayoutParams.empty();
      if (wasGridLayout) {
        resizeHeight();
      }
      return;
    }
    // if there is no space in the query, then this must be a global query
    if (!query.queryText.contains(" ")) {
      isGridLayout.value = false;
      gridLayoutParams.value = GridLayoutParams.empty();
      if (wasGridLayout) {
        resizeHeight();
      }
      return;
    }

    if (queryMetadata.isGridLayout) {
      isGridLayout.value = true;
      gridLayoutParams.value = queryMetadata.gridLayoutParams;
    } else {
      isGridLayout.value = false;
      gridLayoutParams.value = GridLayoutParams.empty();
    }
    resultGridViewController.updateGridParams(gridLayoutParams.value);

    Logger.instance.debug(traceId, "update grid layout params: columns=${queryMetadata.gridLayoutParams.columns}");

    if (wasGridLayout != isGridLayout.value) {
      if (!isGridLayout.value) {
        resizeHeight();
      } else if (resultGridViewController.rowHeight > 0) {
        resizeHeight();
      }
    }
  }

  // Quick select related methods

  /// Check if the quick select modifier key is pressed (Cmd on macOS, Alt on Windows/Linux)
  bool isQuickSelectModifierPressed() {
    if (Platform.isMacOS) {
      return HardwareKeyboard.instance.isMetaPressed;
    } else {
      return HardwareKeyboard.instance.isAltPressed;
    }
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

      // Increment number only for non-group items in visible range that get a number
      if (shouldShowQuickSelect) {
        quickSelectNumber++;
      }

      // Directly update the reactive item to trigger UI refresh
      items[i].value = updatedItem;
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
