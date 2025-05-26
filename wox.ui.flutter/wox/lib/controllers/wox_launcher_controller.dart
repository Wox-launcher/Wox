import 'dart:async';
import 'dart:io';
import 'dart:convert';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:get/get.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controllers/wox_list_controller.dart';
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
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_last_query_mode_enum.dart';
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

class WoxLauncherController extends GetxController {
  //query related variables
  final currentQuery = PlainQuery.empty().obs;
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();
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
  final clearQueryResultDelay = 100;

  // ai chat related variables
  bool hasPendingAutoFocusToChatInput = false;

  /// This flag is used to control whether the user can arrow up to show history when the app is first shown.
  var canArrowUpHistory = true;
  final latestQueryHistories = <QueryHistory>[]; // the latest query histories
  var currentQueryHistoryIndex = 0; //  query history index, used to navigate query history

  var refreshCounter = 0;
  var lastQueryMode = WoxLastQueryModeEnum.WOX_LAST_QUERY_MODE_PRESERVE.code;
  final isInSettingView = false.obs;
  var positionBeforeOpenSetting = const Offset(0, 0);

  /// The icon at end of query box.
  final queryIcon = QueryIconInfo.empty().obs;

  /// The result of the doctor check.
  var doctorCheckPassed = true;
  final doctorCheckInfo = DoctorCheckInfo.empty().obs;

  // toolbar related variables
  final toolbar = ToolbarInfo.empty().obs;
  final toolbarCopyText = 'Copy'.obs;
  // The timer to clean the toolbar when query changed
  // on every query changed, it will reset the timer and will clear the toolbar after N ms
  // If there is no this delay mechanism, the toolbar will flicker for fast typing
  Timer cleanToolbarTimer = Timer(const Duration(), () => {});
  final cleanToolbarDelay = 1000;

  @override
  void onInit() {
    super.onInit();

    resultListViewController = Get.put(
      WoxListController<WoxQueryResult>(
        onItemExecuted: (traceId, item) {
          executeToolbarAction(traceId);
        },
        onItemActive: onResultItemActivated,
      ),
      tag: 'result',
    );

    actionListViewController = Get.put(
      WoxListController<WoxResultAction>(
        onItemExecuted: (traceId, item) {
          executeToolbarAction(traceId);
        },
        onItemActive: onActionItemActivated,
        onFilterBoxEscPressed: hideActionPanel,
        onFilterBoxLostFocus: hideActionPanel,
      ),
      tag: 'action',
    );

    // Add focus listener to query box
    queryBoxFocusNode.addListener(() {
      if (queryBoxFocusNode.hasFocus) {
        // Call API when query box gains focus
        WoxApi.instance.onQueryBoxFocus();
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

    //ignore the results if the query id is not matched
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
    final groups = finalResults.map((e) => e.group).toSet().toList();
    groups.sort((a, b) => finalResults.where((element) => element.group == b).first.groupScore.compareTo(finalResults.where((element) => element.group == a).first.groupScore));
    for (var group in groups) {
      final groupResults = finalResults.where((element) => element.group == group).toList();
      final groupResultsSorted = groupResults..sort((a, b) => b.score.compareTo(a.score));
      if (group != "") {
        finalResultsSorted.add(WoxQueryResult.empty()
          ..title = group
          ..isGroup = true
          ..score = groupResultsSorted.first.groupScore);
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

    resultListViewController.updateItems(traceId, finalResultsSorted.map((e) => WoxListItem.fromQueryResult(e)).toList());

    // if current query already has results and active result is not the first one, then do not reset active result and action
    // this will prevent the active result from being reset to the first one when the query results are received
    if (existingQueryResults.isEmpty || resultListViewController.activeIndex.value == 0) {
      resetActiveResult();
    }

    resizeHeight();
  }

  Future<void> toggleApp(String traceId, ShowAppParams params) async {
    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      if (isInSettingView.value) {
        isInSettingView.value = false;
        showApp(traceId, params);
      } else {
        hideApp(traceId);
      }
    } else {
      showApp(traceId, params);
    }
  }

  Future<void> showApp(String traceId, ShowAppParams params) async {
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      canArrowUpHistory = true;
      if (lastQueryMode == WoxLastQueryModeEnum.WOX_LAST_QUERY_MODE_PRESERVE.code) {
        //skip the first one, because it's the current query
        currentQueryHistoryIndex = 0;
      } else {
        currentQueryHistoryIndex = -1;
      }
    }

    // update some properties to latest for later use
    latestQueryHistories.assignAll(params.queryHistories);
    lastQueryMode = params.lastQueryMode;

    // Handle different position types
    // on linux, we need to show first and then set position or center it
    if (Platform.isLinux) {
      await windowManager.show();
    }
    if (params.position.type == WoxPositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code || params.position.type == WoxPositionTypeEnum.POSITION_TYPE_ACTIVE_SCREEN.code) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    } else if (params.position.type == WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code) {
      // For last location, we don't need to set position as it will remain where it was last positioned
      // but if it's the first time to show, we need to set the position to the center of the screen
      var position = await windowManager.getPosition();
      if (position.dx == 0 && position.dy == 0) {
        await windowManager.center(WoxSettingUtil.instance.currentSetting.appWidth.toDouble(), 600);
      }
    }

    await windowManager.show();
    await windowManager.focus();
    focusQueryBox(selectAll: params.selectAll);

    if (params.autoFocusToChatInput) {
      hasPendingAutoFocusToChatInput = true;
    }

    WoxApi.instance.onShow();
  }

  Future<void> hideApp(String traceId) async {
    //clear query box text if query type is selection or last query mode is empty
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code || lastQueryMode == WoxLastQueryModeEnum.WOX_LAST_QUERY_MODE_EMPTY.code) {
      currentQuery.value = PlainQuery.emptyInput();
      queryBoxTextFieldController.clear();
      hideActionPanel(traceId);
      await clearQueryResults(traceId);
    }

    // switch to the launcher view if in setting view
    if (isInSettingView.value) {
      isInSettingView.value = false;
    }

    hideActionPanel(traceId);
    await windowManager.hide();

    await WoxApi.instance.onHide(currentQuery.value);
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

    await WoxWebsocketMsgUtil.instance.sendMessage(WoxWebsocketMsg(
      requestId: const UuidV4().generate(),
      traceId: traceId,
      type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
      method: WoxMsgMethodEnum.WOX_MSG_METHOD_ACTION.code,
      data: {
        "resultId": result.id,
        "actionId": action.id,
      },
    ));

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
      PlainQuery(
        queryId: const UuidV4().generate(),
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: activeResult.title,
        querySelection: Selection.empty(),
      ),
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
    } else {
      onQueryChanged(
        const UuidV4().generate(),
        PlainQuery(
          queryId: const UuidV4().generate(),
          queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
          queryText: value,
          querySelection: Selection.empty(),
        ),
        "user input changed",
      );
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
    updateToolbarOnQueryChanged(traceId, query);
    if (query.isEmpty) {
      clearQueryResults(traceId);
      return;
    }

    // delay clear results, otherwise windows height will shrink immediately,
    // and then the query result is received which will expand the windows height. so it will causes window flicker
    clearQueryResultsTimer.cancel();
    clearQueryResultsTimer = Timer(
      Duration(milliseconds: clearQueryResultDelay),
      () {
        clearQueryResults(traceId);
      },
    );
    WoxWebsocketMsgUtil.instance.sendMessage(WoxWebsocketMsg(
      requestId: const UuidV4().generate(),
      traceId: traceId,
      type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
      method: WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code,
      data: {
        "queryId": query.queryId,
        "queryType": query.queryType,
        "queryText": query.queryText,
        "querySelection": query.querySelection.toJson(),
      },
    ));
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
    } else if (msg.method == "ChangeTheme") {
      final theme = WoxTheme.fromJson(msg.data);
      WoxThemeUtil.instance.changeTheme(theme);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "PickFiles") {
      final pickFilesParams = FileSelectorParams.fromJson(msg.data);
      final files = await FileSelector.pick(msg.traceId, pickFilesParams);
      responseWoxWebsocketRequest(msg, true, files);
    } else if (msg.method == "OpenSettingWindow") {
      openSettingWindow(msg.traceId, SettingWindowContext.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ShowToolbarMsg") {
      showToolbarMsg(msg.traceId, ToolbarMsg.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "GetCurrentQuery") {
      responseWoxWebsocketRequest(msg, true, currentQuery.value.toJson());
    } else if (msg.method == "IsVisible") {
      var isVisible = await windowManager.isVisible();
      responseWoxWebsocketRequest(msg, true, isVisible);
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
      updateResult(msg.traceId, UpdateableResult.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    }
  }

  Future<void> handleWebSocketResponseMessage(WoxWebsocketMsg msg) async {
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
      var results = <WoxQueryResult>[];
      for (var item in msg.data) {
        results.add(WoxQueryResult.fromJson(item));
      }
      Logger.instance.info(msg.traceId, "Received websocket message: ${msg.method}, results count: ${results.length}");

      onReceivedQueryResults(msg.traceId, results);
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
        hotkey: "enter",
        actionName: tr("plugin_doctor_check"),
        action: () {
          onQueryChanged(traceId, PlainQuery.text("doctor "), "user click doctor icon");
        },
      );
    } else {
      Logger.instance.debug(traceId, "update toolbar to empty because of query changed and is empty");
      toolbar.value = ToolbarInfo.empty();
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
    double resultHeight =
        WoxThemeUtil.instance.getResultListViewHeightByCount(resultListViewController.items.length > maxResultCount ? maxResultCount : resultListViewController.items.length);
    if (isShowActionPanel.value || isShowPreviewPanel.value) {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(maxResultCount);
    }
    if (resultListViewController.items.isNotEmpty) {
      resultHeight += WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingTop + WoxThemeUtil.instance.currentTheme.value.resultContainerPaddingBottom;
    }
    if (toolbar.value.isNotEmpty()) {
      resultHeight += WoxThemeUtil.instance.getToolbarHeight();
    }
    var totalHeight = WoxThemeUtil.instance.getQueryBoxHeight() + resultHeight;

    // if toolbar is showed without results, remove the bottom padding, This way, the toolbar can blend seamlessly with the query box.
    if (isToolbarShowedWithoutResults) {
      totalHeight -= WoxThemeUtil.instance.currentTheme.value.appPaddingBottom;
    }

    if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.debug(const UuidV4().generate(), "Resize: window height to $totalHeight");
    await windowManager.setSize(Size(WoxSettingUtil.instance.currentSetting.appWidth.toDouble(), totalHeight.toDouble()));
  }

  void clearHoveredResult() {
    resultListViewController.clearHoveredResult();
  }

  updateResult(String traceId, UpdateableResult updateableResult) {
    final result = resultListViewController.items.firstWhere((element) => element.value.data.id == updateableResult.id);
    var needUpdate = false;
    var updatedResult = result.value.copyWith();

    if (updateableResult.title != null) {
      updatedResult = updatedResult.copyWith(title: updateableResult.title);
      needUpdate = true;
    }

    if (needUpdate) {
      resultListViewController.updateItem(
        traceId,
        updatedResult,
      );
    }
  }

  startRefreshSchedule() {
    var isRequesting = <String, bool>{};
    Timer.periodic(const Duration(milliseconds: 100), (timer) async {
      var isVisible = await windowManager.isVisible();
      if (!isVisible) {
        return;
      }

      refreshCounter = refreshCounter + 100;
      for (var result in resultListViewController.items) {
        if (result.value.data.refreshInterval > 0 && refreshCounter % result.value.data.refreshInterval == 0) {
          if (isRequesting.containsKey(result.value.data.id)) {
            continue;
          } else {
            isRequesting[result.value.data.id] = true;
          }

          final traceId = const UuidV4().generate();
          final msg = WoxWebsocketMsg(
            requestId: const UuidV4().generate(),
            traceId: traceId,
            type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
            method: WoxMsgMethodEnum.WOX_MSG_METHOD_REFRESH.code,
            data: {
              "queryId": result.value.data.queryId,
              "refreshableResult": WoxRefreshableResult(
                resultId: result.value.data.id,
                title: result.value.data.title,
                subTitle: result.value.data.subTitle,
                icon: result.value.data.icon,
                preview: result.value.data.preview,
                tails: result.value.data.tails,
                contextData: result.value.data.contextData,
                refreshInterval: result.value.data.refreshInterval,
                actions: result.value.data.actions,
              ).toJson(),
            },
          );
          final startTime = DateTime.now().millisecondsSinceEpoch;
          WoxWebsocketMsgUtil.instance.sendMessage(msg).then((resp) {
            final endTime = DateTime.now().millisecondsSinceEpoch;
            if (endTime - startTime > 100) {
              Logger.instance.warn(traceId, "refresh result <${result.value.data.title}> (resultId: ${result.value.data.id}) too slow, cost ${endTime - startTime} ms");
            }

            // check result id, because the result may be removed during the refresh
            if (!resultListViewController.items.any((element) => element.value.data.id == result.value.data.id)) {
              isRequesting.remove(result.value.data.id);
              Logger.instance.info(
                  traceId, "result <${result.value.data.title}> (resultId: ${result.value.data.id}) is removed (maybe caused by new query) during refresh, skip update result");
              return;
            }

            final refreshResult = WoxRefreshableResult.fromJson(resp);
            result.value.data.title = refreshResult.title;
            result.value.data.subTitle = refreshResult.subTitle;
            result.value.data.icon = refreshResult.icon;
            result.value.data.preview = refreshResult.preview;
            result.value.data.tails.assignAll(refreshResult.tails);
            result.value.data.actions.assignAll(refreshResult.actions);
            result.value.data.contextData = refreshResult.contextData;
            result.value.data.refreshInterval = refreshResult.refreshInterval;

            // only update preview and toolbar when current result is active
            if (resultListViewController.isItemActive(result.value.data.id)) {
              currentPreview.value = result.value.data.preview;
              final oldShowPreview = isShowPreviewPanel.value;
              isShowPreviewPanel.value = currentPreview.value.previewData != "";
              if (oldShowPreview != isShowPreviewPanel.value) {
                Logger.instance.debug(traceId, "preview panel visibility changed, resize height");
                resizeHeight();
              }

              var actions = result.value.data.actions.map((e) => WoxListItem.fromResultAction(e)).toList();
              var oldActionIndex = actionListViewController.activeIndex.value;
              var oldActionCount = actionListViewController.items.length;
              actionListViewController.updateItems(traceId, actions);
              // update active index to default action
              // if action panel is visible, prefer to keep the active index
              if (!isShowActionPanel.value || oldActionIndex >= actions.length || oldActionCount != actions.length) {
                var defaultActionIndex = actions.indexWhere((element) => element.data.isDefault);
                if (defaultActionIndex != -1) {
                  actionListViewController.updateActiveIndex(traceId, defaultActionIndex);
                } else {
                  actionListViewController.updateActiveIndex(traceId, 0);
                }
              } else {
                // keep the active index, we need to this to trigger the onItemActive callback, so the toolbar info can be updated
                actionListViewController.updateActiveIndex(traceId, actionListViewController.activeIndex.value);
              }
            }

            // update result list view item
            resultListViewController.updateItem(
                traceId,
                WoxListItem(
                  id: result.value.data.id,
                  icon: result.value.data.icon,
                  title: result.value.data.title,
                  tails: result.value.data.tails,
                  subTitle: result.value.data.subTitle,
                  isGroup: result.value.data.isGroup,
                  data: result.value.data,
                ));

            isRequesting.remove(result.value.data.id);
          });
        }
      }
    });
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

    return DoctorCheckInfo(
      results: results,
      allPassed: allPassed,
      icon: icon,
      message: message,
    );
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

  Future<void> openSettingWindow(String traceId, SettingWindowContext context) async {
    isInSettingView.value = true;
    if (context.path == "/plugin/setting") {
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        await Future.delayed(const Duration(milliseconds: 100));
        var settingController = Get.find<WoxSettingController>();
        settingController.switchToPluginList(false);
        settingController.filterPluginKeywordController.text = context.param;
        settingController.filterPlugins();
        settingController.setFirstFilteredPluginDetailActive();
        settingController.switchToPluginSettingTab();
      });
    }

    // if user open setting window from silent query, the windows may not visible yet
    var isVisible = await windowManager.isVisible();
    if (!isVisible) {
      await showApp(
          traceId,
          ShowAppParams(
            queryHistories: latestQueryHistories,
            lastQueryMode: lastQueryMode,
            selectAll: true,
            position: Position(
              type: WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code,
              x: 0,
              y: 0,
            ),
          ));
    }
  }

  void showToolbarMsg(String traceId, ToolbarMsg msg) {
    // cancel the timer if it is running
    cleanToolbarTimer.cancel();

    toolbar.value = ToolbarInfo(
      text: msg.text,
      icon: msg.icon,
      action: toolbar.value.action,
      actionName: toolbar.value.actionName,
      hotkey: toolbar.value.hotkey,
    );
    if (msg.displaySeconds > 0) {
      Future.delayed(Duration(seconds: msg.displaySeconds), () {
        // only hide toolbar msg when the text is the same as the one we are showing
        if (toolbar.value.text == msg.text) {
          toolbar.value = ToolbarInfo(
            text: "",
            icon: WoxImage.empty(),
            action: toolbar.value.action,
            actionName: toolbar.value.actionName,
            hotkey: toolbar.value.hotkey,
          );
        }
      });
    }
  }

  void executeToolbarAction(String traceId) {
    Logger.instance.info(traceId, "execute toolbar action");
    toolbar.value.action?.call();
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
  }

  void onActionItemActivated(String traceId, WoxListItem<WoxResultAction> item) {
    Logger.instance.debug(traceId, "on result action item activated: ${item.data.name}");

    // update toolbar to active action
    var action = item.data;
    Logger.instance.debug(traceId, "update toolbar to active result: ${item.data.name}, default action: ${action.name}");

    // cancel the timer if it is running
    cleanToolbarTimer.cancel();

    // only update action and hotkey if it's different from the current one
    if (toolbar.value.actionName != action.name || toolbar.value.hotkey != action.hotkey) {
      toolbar.value = ToolbarInfo(
        hotkey: "enter",
        actionName: action.name,
        action: () {
          var result = resultListViewController.items.firstWhereOrNull((element) => element.value.data.id == action.resultId);
          if (result != null) {
            executeAction(traceId, result.value.data, action);
          } else {
            Logger.instance.error(traceId, "associated result not found, cannot execute action: ${action.name}");
          }
        },
      );
    }
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

  void updateToolbarOnQueryChanged(String traceId, PlainQuery query) {
    cleanToolbarTimer.cancel();

    if (isShowDoctorCheckInfo) {
      return;
    }

    // if query is not empty, update the toolbar after 100ms to avoid flickering
    cleanToolbarTimer = Timer(Duration(milliseconds: cleanToolbarDelay), () {
      Logger.instance.debug(traceId, "update toolbar to empty because of query changed");
      toolbar.value = ToolbarInfo.empty();
    });
  }

  /// Update the toolbar to chat view
  void updateToolbarByChat(String traceId) {
    Logger.instance.debug(traceId, "update toolbar to chat");
    toolbar.value = ToolbarInfo(
      hotkey: "cmd+j",
      actionName: "Select models",
      action: () {},
    );
  }
}
