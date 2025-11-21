import 'dart:async';
import 'dart:io';
import 'dart:convert';
import 'dart:ui';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controllers/wox_list_controller.dart';
import 'package:wox/controllers/query_box_text_editing_controller.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/models/doctor_check_result.dart';
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

import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/utils/window_flicker_detector.dart';
import 'package:wox/utils/color_util.dart';

class WoxLauncherController extends GetxController {
  //query related variables
  final currentQuery = PlainQuery.empty().obs;
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = QueryBoxTextEditingController(
    selectedTextStyle: TextStyle(
      color: safeFromCssColor(
        WoxThemeUtil.instance.currentTheme.value.queryBoxTextSelectionColor,
      ),
    ),
  );
  final queryBoxScrollController = ScrollController(initialScrollOffset: 0.0);

  //preview related variables
  final currentPreview = WoxPreview.empty().obs;
  final isShowPreviewPanel = false.obs;

  /// The ratio of result panel width to total width, value range: 0.0-1.0
  /// e.g., 0.3 means result panel takes 30% width, preview panel takes 70%
  final resultPreviewRatio = 0.5.obs;

  // result related variables
  late final WoxListController<WoxQueryResult> resultListViewController;

  // action related variables
  late final WoxListController<WoxResultAction> actionListViewController;
  final isShowActionPanel = false.obs;

  /// The timer to clear query results.
  /// On every query changed, it will reset the timer and will clear the query results after N ms.
  /// If there is no this delay mechanism, the window will flicker for fast typing.
  Timer clearQueryResultsTimer = Timer(const Duration(), () => {});
  int clearQueryResultDelay = 100; // adaptive between 100-200ms based on flicker detection
  final windowFlickerDetector = WindowFlickerDetector(minDelay: 100, maxDelay: 200);

  // ai chat related variables
  bool hasPendingAutoFocusToChatInput = false;

  /// This flag is used to control whether the user can arrow up to show history when the app is first shown.
  var canArrowUpHistory = true;
  final latestQueryHistories = <QueryHistory>[]; // the latest query histories
  var currentQueryHistoryIndex = 0; //  query history index, used to navigate query history

  /// Pending preserved index for query refresh
  int? _pendingPreservedIndex;

  var lastLaunchMode = WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code;
  var lastStartPage = WoxStartPageEnum.WOX_START_PAGE_MRU.code;
  final isInSettingView = false.obs;
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

  // toolbar related variables
  final toolbar = ToolbarInfo.empty().obs;
  // store i18n key instead of literal text
  final toolbarCopyText = 'toolbar_copy'.obs;

  // quick select related variables
  final isQuickSelectMode = false.obs;
  Timer? quickSelectTimer;
  final quickSelectDelay = 300; // delay to show number labels
  bool isQuickSelectKeyPressed = false;

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
      ),
      tag: 'result',
    );

    actionListViewController = Get.put(
      WoxListController<WoxResultAction>(
        onItemExecuted: (traceId, item) {
          executeDefaultAction(traceId);
        },
        onItemActive: onActionItemActivated,
        onFilterBoxEscPressed: hideActionPanel,
      ),
      tag: 'action',
    );

    // Add focus listener to query box
    queryBoxFocusNode.addListener(() {
      if (queryBoxFocusNode.hasFocus) {
        var traceId = const UuidV4().generate();
        hideActionPanel(traceId);

        // Call API when query box gains focus
        WoxApi.instance.onQueryBoxFocus();
      }
    });

    // Add scroll listener to update quick select numbers when scrolling
    resultListViewController.scrollController.addListener(() {
      if (isQuickSelectMode.value) {
        updateQuickSelectNumbers(const UuidV4().generate());
      }
    });

    // Initialize doctor check info
    doctorCheckInfo.value = DoctorCheckInfo.empty();
  }

  bool get isShowDoctorCheckInfo => currentQuery.value.isEmpty && !doctorCheckInfo.value.allPassed;

  bool get isShowToolbar => resultListViewController.items.isNotEmpty || isShowDoctorCheckInfo;

  bool get isToolbarShowedWithoutResults => isShowToolbar && resultListViewController.items.isEmpty;

  /// Triggered when received query results from the server.
  void onReceivedQueryResults(String traceId, List<WoxQueryResult> receivedResults) {
    if (receivedResults.isEmpty) {
      return;
    }

    if (currentQuery.value.queryId != receivedResults.first.queryId) {
      Logger.instance.error(traceId, "query id is not matched, ignore the results");
      return;
    }

    //cancel clear results timer
    clearQueryResultsTimer.cancel();

    //merge results
    final existingQueryResults = resultListViewController.items.where((item) => item.value.data.queryId == currentQuery.value.queryId).map((e) => e.value.data).toList();
    final finalResults = List<WoxQueryResult>.from(existingQueryResults)..addAll(receivedResults);

    //group results
    var finalResultsSorted = <WoxQueryResult>[];
    // Build group to score mapping to avoid repeated list traversal
    final groupScoreMap = <String, int>{};
    for (var result in finalResults) {
      if (!groupScoreMap.containsKey(result.group)) {
        groupScoreMap[result.group] = result.groupScore;
      }
    }

    final groups = finalResults.map((e) => e.group).toSet().toList();
    groups.sort((a, b) => groupScoreMap[b]!.compareTo(groupScoreMap[a]!));

    for (var group in groups) {
      final groupResults = finalResults.where((element) => element.group == group).toList();
      final groupResultsSorted = groupResults..sort((a, b) => b.score.compareTo(a.score));
      if (group != "") {
        finalResultsSorted.add(
          WoxQueryResult.empty()
            ..title = group
            ..isGroup = true
            ..score = groupResultsSorted.first.groupScore,
        );
      }
      finalResultsSorted.addAll(groupResultsSorted);
    }

    // move default action to the first for every result
    for (var element in finalResultsSorted) {
      final defaultActionIndex = element.actions.indexWhere((element) => element.isDefault);
      if (defaultActionIndex != -1) {
        final defaultAction = element.actions[defaultActionIndex];
        element.actions.removeAt(defaultActionIndex);
        element.actions.insert(0, defaultAction);
      }
    }

    // Use silent mode to avoid triggering onItemActive callback during updateItems (which may cause a little performance issue)
    // Following resetActiveResult will trigger the callback
    resultListViewController.updateItems(traceId, finalResultsSorted.map((e) => WoxListItem.fromQueryResult(e)).toList(), silent: true);

    // Handle index preservation or reset
    if (_pendingPreservedIndex != null) {
      // Restore the preserved index
      final targetIndex = _pendingPreservedIndex!;
      _pendingPreservedIndex = null; // Clear the pending index

      // Ensure the index is within bounds
      if (targetIndex < resultListViewController.items.length) {
        // Skip group items - find the next non-group item
        var actualIndex = targetIndex;
        while (actualIndex < resultListViewController.items.length && resultListViewController.items[actualIndex].value.data.isGroup) {
          actualIndex++;
        }

        // If we found a valid non-group item, use it; otherwise reset to first
        if (actualIndex < resultListViewController.items.length) {
          resultListViewController.updateActiveIndex(traceId, actualIndex);
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
      if (existingQueryResults.isEmpty || resultListViewController.activeIndex.value == 0) {
        resetActiveResult();
      }
    }

    resizeHeight();
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
        await queryMRU(traceId);
      } else {
        // Blank page - clear results
        await clearQueryResults(traceId);
      }
    }

    // Handle different position types
    // on linux, we need to show first and then set position or center it
    if (Platform.isLinux) {
      await windowManager.show();
    }
    // Use the position calculated by backend
    await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));

    await windowManager.show();
    if (!isInSettingView.value) {
      await windowManager.setAlwaysOnTop(true);
    }
    await windowManager.focus();
    focusQueryBox(selectAll: params.selectAll);

    if (params.autoFocusToChatInput) {
      hasPendingAutoFocusToChatInput = true;
    }

    WoxApi.instance.onShow();
  }

  Future<void> hideApp(String traceId) async {
    //clear query box text if query type is selection or launch mode is fresh
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code || lastLaunchMode == WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code) {
      currentQuery.value = PlainQuery.emptyInput();
      queryBoxTextFieldController.clear();
      hideActionPanel(traceId);
      await clearQueryResults(traceId);
    }

    hideActionPanel(traceId);

    // Clean up quick select state
    if (isQuickSelectMode.value) {
      deactivateQuickSelectMode(traceId);
    }
    quickSelectTimer?.cancel();
    isQuickSelectKeyPressed = false;
    isSettingOpenedFromHidden = false;
    isInSettingView.value = false;

    await windowManager.hide();
    await WoxApi.instance.onHide();
  }

  void saveWindowPositionIfNeeded() {
    final setting = WoxSettingUtil.instance.currentSetting;
    if (setting.showPosition == WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code) {
      // Run in async task with delay to ensure window position is fully updated
      Future.delayed(const Duration(milliseconds: 500), () async {
        try {
          final position = await windowManager.getPosition();
          await WoxApi.instance.saveWindowPosition(position.dx.toInt(), position.dy.toInt());
        } catch (e) {
          Logger.instance.error(const UuidV4().generate(), "Failed to save window position: $e");
        }
      });
    }
  }

  Future<void> toggleActionPanel(String traceId) async {
    if (resultListViewController.items.isEmpty) {
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

  void focusQueryBox({bool selectAll = false}) {
    // request focus to action query box since it will lose focus when tap
    queryBoxFocusNode.requestFocus();

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
    if (resultListViewController.activeIndex.value >= resultListViewController.items.length ||
        resultListViewController.activeIndex.value < 0 ||
        resultListViewController.items.isEmpty) {
      return null;
    }

    return resultListViewController.items[resultListViewController.activeIndex.value].value.data;
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

    await WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_ACTION.code,
        data: {"resultId": result.id, "actionId": action.id},
      ),
    );

    // clear the search text after action is executed
    actionListViewController.clearFilter(traceId);

    if (!preventHideAfterAction) {
      hideApp(traceId);
    }
    if (isShowActionPanel.value) {
      hideActionPanel(traceId);
    }
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

    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      // do local filter if query type is selection
      resultListViewController.filterItems(const UuidV4().generate(), value);
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
    var queryId = const UuidV4().generate();
    currentQuery.value = PlainQuery.emptyInput();
    currentQuery.value.queryId = queryId;

    updateQueryIconOnQueryChanged(traceId, currentQuery.value);

    try {
      final results = await WoxApi.instance.queryMRU(traceId);
      if (results.isEmpty) {
        Logger.instance.debug(traceId, "no MRU results");
        clearQueryResults(traceId);
        return;
      }

      for (var result in results) {
        result.queryId = queryId;
      }
      onReceivedQueryResults(traceId, results);
    } catch (e) {
      Logger.instance.error(traceId, "Failed to query MRU: $e");
      clearQueryResults(traceId);
    }
  }

  void onQueryChanged(String traceId, PlainQuery query, String changeReason, {bool moveCursorToEnd = false}) {
    Logger.instance.debug(traceId, "query changed: ${query.queryText}, reason: $changeReason");

    if (query.queryId == "") {
      query.queryId = const UuidV4().generate();
    }

    clearHoveredResult();

    //hide setting view if query changed
    if (isInSettingView.value) {
      isInSettingView.value = false;
    }

    currentQuery.value = query;
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
    updateQueryIconOnQueryChanged(traceId, query);
    updateResultPreviewWidthRatioOnQueryChanged(traceId, query);
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
      clearQueryResults(traceId);
    });

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
      final savedActiveIndex = resultListViewController.activeIndex.value;
      _pendingPreservedIndex = savedActiveIndex;
      Logger.instance.debug(traceId, "preserving selected index: $savedActiveIndex");
    }

    // Set longer delay for clearing results to avoid flicker
    // since refresh query usually returns similar results
    clearQueryResultDelay = 250;

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
      onQueryChanged(msg.traceId, PlainQuery.fromJson(msg.data), "receive change query from wox", moveCursorToEnd: true);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "RefreshQuery") {
      final preserveSelectedIndex = msg.data['preserveSelectedIndex'] as bool? ?? false;
      onRefreshQuery(msg.traceId, preserveSelectedIndex);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ChangeTheme") {
      final theme = WoxTheme.fromJson(msg.data);
      WoxThemeUtil.instance.changeTheme(theme);
      resizeHeight(); // Theme height maybe changed, so we need to resize height
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
      focusToChatInput(msg.traceId);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "SendChatResponse") {
      handleChatResponse(msg.traceId, WoxAIChatData.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ReloadChatResources") {
      Get.find<WoxAIChatController>().reloadChatResources(msg.traceId, resourceName: msg.data as String);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "UpdateResult") {
      final success = updateResult(msg.traceId, UpdatableResult.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, success);
    }
  }

  Future<void> handleWebSocketResponseMessage(WoxWebsocketMsg msg) async {
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
      final isFinal = queryResponse['IsFinal'] as bool? ?? false;

      var results = <WoxQueryResult>[];
      for (var item in resultsData) {
        results.add(WoxQueryResult.fromJson(item));
      }

      Logger.instance.info(msg.traceId, "Received websocket message: ${msg.method}, results count: ${results.length}, isFinal: $isFinal");

      // Process results first
      onReceivedQueryResults(msg.traceId, results);

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
            Logger.instance.info(msg.traceId, "🎨 COMPLETE PAINT: ${completePaintTime}ms (total ${resultListViewController.items.length} results rendered)");
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
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_RESPONSE.code,
        method: request.method,
        data: data,
        success: success,
      ),
    );
  }

  Future<void> clearQueryResults(String traceId) async {
    resultListViewController.clearItems();
    actionListViewController.clearItems();
    isShowPreviewPanel.value = false;
    isShowActionPanel.value = false;

    if (isShowDoctorCheckInfo) {
      Logger.instance.debug(traceId, "update toolbar to doctor warning, query is empty and doctor check not passed");
      toolbar.value = ToolbarInfo(
        text: doctorCheckInfo.value.message,
        icon: doctorCheckInfo.value.icon,
        actions: [
          ToolbarActionInfo(
            name: tr("plugin_doctor_check"),
            hotkey: "enter",
            action: () {
              onQueryChanged(traceId, PlainQuery.text("doctor "), "user click doctor icon");
            },
          ),
        ],
      );
    } else {
      Logger.instance.debug(traceId, "update toolbar to empty because of query changed and is empty");
      toolbar.value = toolbar.value.emptyRightSide();
    }

    await resizeHeight();
  }

  // select all text in query box
  void selectQueryBoxAllText(String traceId) {
    Logger.instance.info(traceId, "select query box all text");
    queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
  }

  /// reset and jump active result to top of the list
  void resetActiveResult() {
    if (resultListViewController.items.isNotEmpty) {
      if (resultListViewController.items.first.value.isGroup) {
        resultListViewController.updateActiveIndex(const UuidV4().generate(), 1);
      } else {
        resultListViewController.updateActiveIndex(const UuidV4().generate(), 0);
      }
    }
  }

  Future<void> resizeHeight() async {
    final maxResultCount = WoxSettingUtil.instance.currentSetting.maxResultCount;
    final actualResultCount = resultListViewController.items.length > maxResultCount ? maxResultCount : resultListViewController.items.length;
    double resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(actualResultCount);

    if (isShowActionPanel.value || isShowPreviewPanel.value) {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(maxResultCount);
    }
    if (resultListViewController.items.isNotEmpty) {
      resultHeight += WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingTop + WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingBottom;
    }
    // Only add toolbar height when toolbar is actually shown in UI
    if (isShowToolbar) {
      resultHeight += WoxThemeUtil.instance.getToolbarHeight();
    }
    var totalHeight = WoxThemeUtil.instance.getQueryBoxHeight() + resultHeight;

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

    await windowManager.setSize(Size(WoxSettingUtil.instance.currentSetting.appWidth.toDouble(), totalHeight.toDouble()));
    windowFlickerDetector.recordResize(totalHeight.toInt());
  }

  void clearHoveredResult() {
    resultListViewController.clearHoveredResult();
  }

  bool updateResult(String traceId, UpdatableResult UpdatableResult) {
    // Try to find the result in the current items
    try {
      final result = resultListViewController.items.firstWhere((element) => element.value.data.id == UpdatableResult.id);
      var needUpdate = false;
      var updatedResult = result.value;
      var updatedData = result.value.data;

      // Update only non-null fields
      if (UpdatableResult.title != null) {
        updatedResult = updatedResult.copyWith(title: UpdatableResult.title);
        updatedData.title = UpdatableResult.title!;
        needUpdate = true;
      }

      if (UpdatableResult.subTitle != null) {
        updatedResult = updatedResult.copyWith(subTitle: UpdatableResult.subTitle);
        updatedData.subTitle = UpdatableResult.subTitle!;
        needUpdate = true;
      }

      if (UpdatableResult.tails != null) {
        updatedResult = updatedResult.copyWith(tails: UpdatableResult.tails);
        updatedData.tails = UpdatableResult.tails!;
        needUpdate = true;
      }

      if (UpdatableResult.preview != null) {
        updatedData.preview = UpdatableResult.preview!;
        needUpdate = true;
      }

      if (UpdatableResult.actions != null) {
        updatedData.actions = UpdatableResult.actions!;
        needUpdate = true;
      }

      if (needUpdate) {
        // Force create a new WoxListItem with updated data to trigger reactive update
        updatedResult = updatedResult.copyWith(data: updatedData);
        resultListViewController.updateItem(traceId, updatedResult);

        // If this result is currently active, update the preview and actions
        if (resultListViewController.isItemActive(updatedData.id)) {
          if (UpdatableResult.preview != null) {
            final oldShowPreview = isShowPreviewPanel.value;
            currentPreview.value = UpdatableResult.preview!;
            isShowPreviewPanel.value = currentPreview.value.previewData != "";

            // If preview panel visibility changed, resize window height
            if (oldShowPreview != isShowPreviewPanel.value) {
              resizeHeight();
            }
          }

          if (UpdatableResult.actions != null) {
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

      return true; // Successfully found and updated the result
    } catch (e) {
      // Result not found in current items (no longer visible)
      return false;
    }
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
    var results = await WoxApi.instance.doctorCheck();
    final checkInfo = processDoctorCheckResults(results);
    doctorCheckInfo.value = checkInfo;
    Logger.instance.debug(const UuidV4().generate(), "doctor check result: ${checkInfo.allPassed}, details: ${checkInfo.results.length} items");
  }

  @override
  void dispose() {
    queryBoxFocusNode.dispose();
    queryBoxTextFieldController.dispose();
    actionListViewController.dispose();
    resultListViewController.dispose();
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

    // Preload theme/settings for settings view
    await WoxThemeUtil.instance.loadTheme();
    await WoxSettingUtil.instance.loadSetting();
    settingController.activeNavPath.value = 'general';

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
    if (isSettingOpenedFromHidden) {
      // For hidden-opened settings, exit means hide the window directly
      await hideApp(traceId);
      return;
    }

    // Switch back to launcher
    isInSettingView.value = false;
    await windowManager.setAlwaysOnTop(true);
    await resizeHeight();
    await windowManager.setPosition(positionBeforeOpenSetting);
    await windowManager.focus();
    focusQueryBox(selectAll: true);
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

    // First check if toolbar has action callbacks (e.g., doctor check)
    if (toolbar.value.actions != null && toolbar.value.actions!.isNotEmpty) {
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
    if (resultListViewController.items.isEmpty) {
      Logger.instance.debug(traceId, "no results to execute");
      return;
    }

    var activeResult = resultListViewController.activeItem;
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

  void focusToChatInput(String traceId) {
    Logger.instance.info(traceId, "focus to chat input");
    Get.find<WoxAIChatController>().focusToChatInput(traceId);
  }

  void handleChatResponse(String traceId, WoxAIChatData data) {
    for (var result in resultListViewController.items) {
      if (result.value.data.contextData == data.id) {
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

    resultListViewController.updateActiveIndexByDirection(const UuidV4().generate(), WoxDirectionEnum.WOX_DIRECTION_UP.code);
  }

  void handleQueryBoxArrowDown() {
    canArrowUpHistory = false;
    resultListViewController.updateActiveIndexByDirection(const UuidV4().generate(), WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
  }

  void onResultItemActivated(String traceId, WoxListItem<WoxQueryResult> item) {
    currentPreview.value = item.data.preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";

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
    var toolbarActions = actionsWithHotkeys.map((action) {
      return ToolbarActionInfo(
        name: action.name,
        hotkey: action.hotkey,
      );
    }).toList();

    // Add "More Actions" hotkey at the end if there are actions
    if (shouldShowMoreActions) {
      final moreActionsHotkey = Platform.isMacOS ? "cmd+j" : "alt+j";
      toolbarActions.add(
        ToolbarActionInfo(
          name: tr("toolbar_more_actions"),
          hotkey: moreActionsHotkey,
        ),
      );
    }

    // Update toolbar with all actions
    toolbar.value = toolbar.value.copyWith(
      actions: toolbarActions,
    );
  }

  void onActionItemActivated(String traceId, WoxListItem<WoxResultAction> item) {
    Logger.instance.debug(traceId, "on action item activated: ${item.data.name}");
    // No need to update toolbar here, executeDefaultAction will handle action execution
  }

  Future<void> handleDropFiles(DropDoneDetails details) async {
    Logger.instance.info(const UuidV4().generate(), "Received drop files: $details");

    await windowManager.focus();
    focusQueryBox();

    canArrowUpHistory = false;

    PlainQuery woxChangeQuery = PlainQuery(
      queryId: const UuidV4().generate(),
      queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code,
      queryText: "",
      querySelection: Selection(type: WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code, text: "", filePaths: details.files.map((e) => e.path).toList()),
    );

    onQueryChanged(const UuidV4().generate(), woxChangeQuery, "user drop files");
  }

  /// Change the query icon based on the query
  Future<void> updateQueryIconOnQueryChanged(String traceId, PlainQuery query) async {
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code) {
        queryIcon.value = QueryIconInfo(
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_FILE),
        );
      }
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code) {
        queryIcon.value = QueryIconInfo(
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_TEXT),
        );
      }
      return;
    }

    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      // if there is no space in the query, then this must be a global query
      if (!query.queryText.contains(" ")) {
        queryIcon.value = QueryIconInfo.empty();
        return;
      }

      var img = await WoxApi.instance.getQueryIcon(query);
      queryIcon.value = QueryIconInfo(icon: img);
      return;
    }

    queryIcon.value = QueryIconInfo.empty();
  }

  /// Update the result preview width ratio based on the query
  Future<void> updateResultPreviewWidthRatioOnQueryChanged(String traceId, PlainQuery query) async {
    if (query.isEmpty) {
      resultPreviewRatio.value = 0.5;
      return;
    }
    // if there is no space in the query, then this must be a global query
    if (!query.queryText.contains(" ")) {
      resultPreviewRatio.value = 0.5;
      return;
    }

    var resultPreviewWidthRatio = await WoxApi.instance.getResultPreviewWidthRatio(query);
    Logger.instance.debug(traceId, "update result preview width ratio: $resultPreviewWidthRatio");
    resultPreviewRatio.value = resultPreviewWidthRatio;
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
    if (isQuickSelectMode.value || resultListViewController.items.isEmpty) {
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
    var items = resultListViewController.items;

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
    if (!resultListViewController.scrollController.hasClients || resultListViewController.items.isEmpty) {
      return {'start': 0, 'end': resultListViewController.items.length - 1};
    }

    final itemHeight = WoxThemeUtil.instance.getResultItemHeight();
    final currentOffset = resultListViewController.scrollController.offset;
    final viewportHeight = resultListViewController.scrollController.position.viewportDimension;

    if (viewportHeight <= 0) {
      return {'start': 0, 'end': resultListViewController.items.length - 1};
    }

    final firstVisibleItemIndex = (currentOffset / itemHeight).floor();
    final visibleItemCount = (viewportHeight / itemHeight).ceil();
    final lastVisibleItemIndex = (firstVisibleItemIndex + visibleItemCount - 1).clamp(0, resultListViewController.items.length - 1);

    return {'start': firstVisibleItemIndex.clamp(0, resultListViewController.items.length - 1), 'end': lastVisibleItemIndex};
  }

  /// Handle number key press in quick select mode
  bool handleQuickSelectNumberKey(String traceId, int number) {
    if (!isQuickSelectMode.value || number < 1 || number > 9) {
      return false;
    }

    var items = resultListViewController.items;

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
          resultListViewController.updateActiveIndex(traceId, i);
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
