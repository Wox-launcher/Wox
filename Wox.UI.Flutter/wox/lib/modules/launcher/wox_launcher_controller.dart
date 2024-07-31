import 'dart:async';
import 'dart:io';
import 'dart:ui';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:lpinyin/lpinyin.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_last_query_mode_enum.dart';
import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/enums/wox_msg_type_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/picker.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

class WoxLauncherController extends GetxController {
  //query related variables
  final currentQuery = PlainQuery.empty().obs;
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();
  final queryBoxScrollController = ScrollController(initialScrollOffset: 0.0);
  final queryIcon = WoxImage.empty().obs;

  //preview related variables
  final currentPreview = WoxPreview.empty().obs;
  final isShowPreviewPanel = false.obs;

  // result related variables
  /// The list of query results.
  final results = <WoxQueryResult>[].obs;
  final activeResultIndex = 0.obs;
  final resultGlobalKeys = <GlobalKey>[]; // the global keys for each result item, used to calculate the position of the result item
  final resultScrollerController = ScrollController(initialScrollOffset: 0.0);

  /// The timer to clear query results.
  /// On every query changed, it will reset the timer and will clear the query results after N ms.
  /// If there is no this delay mechanism, the window will flicker for fast typing.
  var clearQueryResultsTimer = Timer(const Duration(), () => {});

  // action related variables
  /// The list of result actions for the active query result.
  final actions = <WoxResultAction>[].obs;
  final activeActionIndex = 0.obs;
  final isShowActionPanel = false.obs;
  final actionTextFieldController = TextEditingController();
  final actionFocusNode = FocusNode();
  final actionScrollerController = ScrollController(initialScrollOffset: 0.0);

  /// This flag is used to control whether the user can arrow up to show history when the app is first shown.
  var canArrowUpHistory = true;
  final latestQueryHistories = <QueryHistory>[]; // the latest query histories
  var currentQueryHistoryIndex = 0; //  query history index, used to navigate query history

  final Rx<WoxTheme> woxTheme = WoxThemeUtil.instance.currentTheme.obs;
  var refreshCounter = 0;
  var lastQueryMode = WoxLastQueryModeEnum.WOX_LAST_QUERY_MODE_PRESERVE.code;
  final isInSettingView = false.obs;
  var positionBeforeOpenSetting = const Offset(0, 0);

  /// Triggered when received query results from the server.
  void onReceivedQueryResults(List<WoxQueryResult> receivedResults) {
    if (receivedResults.isEmpty) {
      return;
    }

    //ignore the results if the query id is not matched
    if (currentQuery.value.queryId != receivedResults.first.queryId) {
      return;
    }

    //cancel clear results timer
    clearQueryResultsTimer.cancel();

    //merge results
    final currentQueryResults = results.where((item) => item.queryId == currentQuery.value.queryId).toList();
    final finalResults = List<WoxQueryResult>.from(currentQueryResults)..addAll(receivedResults);

    //group results
    var finalResultsSorted = <WoxQueryResult>[];
    final groups = finalResults.map((e) => e.group).toSet().toList();
    groups.sort((a, b) => finalResults.where((element) => element.group == b).first.groupScore.compareTo(finalResults.where((element) => element.group == a).first.groupScore));
    for (var group in groups) {
      final groupResults = finalResults.where((element) => element.group == group).toList();
      final groupResultsSorted = groupResults..sort((a, b) => b.score.compareTo(a.score));
      if (group != "") {
        finalResultsSorted.add(WoxQueryResult.empty()
          ..title.value = group
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

    results.assignAll(finalResultsSorted);
    for (var _ in results) {
      resultGlobalKeys.add(GlobalKey());
    }

    //reset active result and actions if current query has no results (this is the first time to receive results for the current query)
    //if current query has results, then keep the active result and actions
    if (currentQueryResults.isEmpty) {
      resetActiveResult();
      updateAndResetActiveAction();
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

    if (params.selectAll) {
      selectQueryBoxAllText();
    }
    if (params.position.type == WoxPositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    }
    await windowManager.show();
    if (Platform.isWindows) {
      // on windows, it is somehow necessary to invoke show twice to make the window show
      // otherwise, the window will not show up if it is the first time to invoke showApp
      await windowManager.show();
    }
    await windowManager.focus();
    queryBoxFocusNode.requestFocus();

    WoxApi.instance.onShow();
  }

  Future<void> hideApp(String traceId) async {
    isShowActionPanel.value = false;
    await windowManager.hide();

    //clear query box text if query type is selection
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      onQueryChanged(traceId, PlainQuery.emptyInput(), "clear input after hide app");
    }

    WoxApi.instance.onHide(currentQuery.value);
  }

  Future<void> toggleActionPanel(String traceId) async {
    if (results.isEmpty) {
      return;
    }

    if (isShowActionPanel.value) {
      hideActionPanel();
    } else {
      showActionPanel();
    }
  }

  void hideActionPanel() {
    isShowActionPanel.value = false;
    actionTextFieldController.text = "";
    queryBoxFocusNode.requestFocus();
    resizeHeight();
  }

  void showActionPanel() {
    isShowActionPanel.value = true;
    actionFocusNode.requestFocus();
    resizeHeight();
  }

  WoxQueryResult? getActiveResult() {
    if (activeResultIndex.value >= results.length || activeResultIndex.value < 0 || results.isEmpty) {
      return null;
    }

    return results[activeResultIndex.value];
  }

  WoxResultAction? getActiveAction() {
    if (actions.isEmpty || activeActionIndex.value >= actions.length || activeActionIndex.value < 0) {
      return null;
    }

    return actions[activeActionIndex.value];
  }

  Future<void> executeAction(String traceId) async {
    Logger.instance.debug(traceId, "user execute result action");

    WoxQueryResult? woxQueryResult = getActiveResult();
    if (woxQueryResult == null) {
      Logger.instance.error(traceId, "active query result is null");
      return;
    }

    WoxResultAction? activeAction = getActiveAction();
    if (activeAction == null) {
      Logger.instance.error(traceId, "active action is null");
      return;
    }

    await WoxWebsocketMsgUtil.instance.sendMessage(WoxWebsocketMsg(
      requestId: const UuidV4().generate(),
      traceId: traceId,
      type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
      method: WoxMsgMethodEnum.WOX_MSG_METHOD_ACTION.code,
      data: {
        "resultId": woxQueryResult.id,
        "actionId": activeAction.id,
      },
    ));

    if (!activeAction.preventHideAfterAction) {
      hideApp(traceId);
    }
    hideActionPanel();
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
        queryText: activeResult.title.value,
        querySelection: Selection.empty(),
      ),
      "auto complete query",
      moveCursorToEnd: true,
    );
  }

  void onQueryBoxTextChanged(String value) {
    canArrowUpHistory = false;

    PlainQuery plainQuery = PlainQuery(
      queryId: const UuidV4().generate(),
      queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
      queryText: value,
      querySelection: Selection.empty(),
    );

    // do filter if query type is selection
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      plainQuery.queryType = WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code;
      plainQuery.querySelection = currentQuery.value.querySelection;
    }

    onQueryChanged(const UuidV4().generate(), plainQuery, "user input changed");
  }

  void onQueryChanged(String traceId, PlainQuery query, String changeReason, {bool moveCursorToEnd = false}) {
    Logger.instance.debug(traceId, "query changed: ${query.queryText}, reason: $changeReason");

    if (query.queryId == "") {
      query.queryId = const UuidV4().generate();
    }

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
    changeQueryIcon(traceId, query);
    if (query.isEmpty) {
      clearQueryResults();
      return;
    }

    // delay clear results, otherwise windows height will shrink immediately,
    // and then the query result is received which will expand the windows height. so it will causes window flicker
    clearQueryResultsTimer.cancel();
    clearQueryResultsTimer = Timer(
      const Duration(milliseconds: 100),
      () {
        clearQueryResults();
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

  void onActionQueryBoxTextChanged(String traceId, String queryAction) {
    // restore all actions if query is empty
    var activeResult = getActiveResult();
    if (activeResult == null) {
      return;
    }

    if (queryAction.isEmpty) {
      actions.assignAll(activeResult.actions);
      return;
    }

    var filteredActions = activeResult.actions.where((element) {
      if (WoxSettingUtil.instance.currentSetting.usePinYin) {
        return transferChineseToPinYin(element.name.toLowerCase()).contains(queryAction.toLowerCase());
      }

      return element.name.toLowerCase().contains(queryAction.toLowerCase());
    }).toList();

    //if filtered actions is not changed, then return
    if (filteredActions.length == actions.length && filteredActions.every((element) => actions.contains(element))) {
      return;
    }

    actions.assignAll(filteredActions);
  }

  void changeResultScrollPosition(String traceId, WoxEventDeviceType deviceType, WoxDirection direction) {
    final prevResultIndex = activeResultIndex.value;
    resetActiveResultIndex(direction);
    if (results.length < MAX_LIST_VIEW_ITEM_COUNT) {
      results.refresh();
      return;
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      if (activeResultIndex.value < prevResultIndex) {
        resultScrollerController.jumpTo(0);
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code
            ? isResultItemAtBottom(activeResultIndex.value - 1)
            : !isResultItemAtBottom(results.length - 1);
        if (shouldJump) {
          resultScrollerController.jumpTo(resultScrollerController.offset.ceil() + WoxThemeUtil.instance.getResultItemHeight() * (activeResultIndex.value - prevResultIndex).abs());
        }
      }
    }
    if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (activeResultIndex.value > prevResultIndex) {
        resultScrollerController.jumpTo(WoxThemeUtil.instance.getResultListViewHeightByCount(results.length - MAX_LIST_VIEW_ITEM_COUNT));
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code ? isResultItemAtTop(activeResultIndex.value + 1) : !isResultItemAtTop(0);
        if (shouldJump) {
          resultScrollerController.jumpTo(resultScrollerController.offset.ceil() - WoxThemeUtil.instance.getResultItemHeight() * (activeResultIndex.value - prevResultIndex).abs());
        }
      }
    }
    results.refresh();
  }

  void changeActionScrollPosition(String traceId, WoxEventDeviceType deviceType, WoxDirection direction) {
    updateActiveAction(direction);
    actions.refresh();
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
      woxTheme.value = theme;
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "PickFiles") {
      final pickFilesParams = FileSelectorParams.fromJson(msg.data);
      final files = await FileSelector.pick(msg.traceId, pickFilesParams);
      responseWoxWebsocketRequest(msg, true, files);
    } else if (msg.method == "OpenSettingWindow") {
      openSettingWindow(msg.traceId, SettingWindowContext.fromJson(msg.data));
      responseWoxWebsocketRequest(msg, true, null);
    }
  }

  Future<void> handleWebSocketResponseMessage(WoxWebsocketMsg msg) async {
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
      var results = <WoxQueryResult>[];
      for (var item in msg.data) {
        results.add(WoxQueryResult.fromJson(item));
      }
      Logger.instance.info(msg.traceId, "Received message: ${msg.method}, results count: ${results.length}");

      onReceivedQueryResults(results);
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

  bool isResultItemAtBottom(int index) {
    RenderBox? renderBox = resultGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) return false;

    if (renderBox.localToGlobal(Offset.zero).dy.ceil() >=
        WoxThemeUtil.instance.getQueryBoxHeight() + WoxThemeUtil.instance.getResultListViewHeightByCount(MAX_LIST_VIEW_ITEM_COUNT - 1)) {
      return true;
    }
    return false;
  }

  bool isResultItemAtTop(int index) {
    if (index < 0) {
      return false;
    }
    RenderBox? renderBox = resultGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) return false;

    if (renderBox.localToGlobal(Offset.zero).dy.ceil() <= WoxThemeUtil.instance.getQueryBoxHeight()) {
      return true;
    }
    return false;
  }

  void clearQueryResults() {
    results.clear();
    isShowPreviewPanel.value = false;
    isShowActionPanel.value = false;
    resultGlobalKeys.clear();
    resizeHeight();
  }

  // select all text in query box
  void selectQueryBoxAllText() {
    queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
  }

  /// reset and jump active result to top of the list
  void resetActiveResult() {
    if (results[0].isGroup) {
      activeResultIndex.value = 1;
    } else {
      activeResultIndex.value = 0;
    }
    if (resultScrollerController.hasClients) {
      resultScrollerController.jumpTo(0);
    }

    //reset preview
    if (results.isNotEmpty) {
      currentPreview.value = results[activeResultIndex.value].preview;
    } else {
      currentPreview.value = WoxPreview.empty();
    }
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
  }

  Future<void> resizeHeight() async {
    double resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(results.length > 10 ? 10 : results.length);
    if (isShowActionPanel.value || isShowPreviewPanel.value) {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(10);
    }
    if (results.isNotEmpty) {
      resultHeight += woxTheme.value.resultContainerPaddingTop + woxTheme.value.resultContainerPaddingBottom;
      resultHeight += WoxThemeUtil.instance.getResultTipHeight();
    }
    final totalHeight = WoxThemeUtil.instance.getQueryBoxHeight() + resultHeight;

    //if the window height is same as the total height, then no need to resize
    var currentSize = await windowManager.getSize();
    if (currentSize.height == totalHeight) {
      if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.info(const UuidV4().generate(), "Resize: want to resize but the height is same as current height");
      return;
    }

    if (Platform.isWindows) {
      // on windows, if I set screen ratio to 2.0, then the window height should add more 4.5 pixel, otherwise it will show render error
      // still don't know why. here is the test result: ratio -> additional window height
      // 1.0 -> 9
      // 1.25-> 7.8
      // 1.5-> 6.3
      // 1.75-> 5.3
      // 2.0-> 4.5
      // 2.25-> 4.3
      // 2.5-> 3.8
      // 3.0-> 3

      final totalHeightFinal = totalHeight.toDouble() + (10 / PlatformDispatcher.instance.views.first.devicePixelRatio).ceil();
      if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.info(const UuidV4().generate(), "Resize: window height to $totalHeightFinal");
      windowManager.setSize(Size(800, totalHeightFinal));
    } else {
      if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.info(const UuidV4().generate(), "Resize: window height to $totalHeight");
      windowManager.setSize(Size(800, totalHeight.toDouble()));
    }
  }

  void resetActiveResultIndex(WoxDirection woxDirection) {
    if (results.isEmpty) {
      return;
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      // select next none group result
      activeResultIndex.value++;
      if (activeResultIndex.value == results.length) {
        activeResultIndex.value = 0;
      }
      while (results[activeResultIndex.value].isGroup) {
        activeResultIndex.value++;
        if (activeResultIndex.value == results.length) {
          activeResultIndex.value = 0;
          break;
        }
      }
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      // select previous none group result
      activeResultIndex.value--;
      if (activeResultIndex.value == -1) {
        activeResultIndex.value = results.length - 1;
      }
      while (results[activeResultIndex.value].isGroup) {
        activeResultIndex.value--;
        if (activeResultIndex.value == -1) {
          activeResultIndex.value = results.length - 1;
          break;
        }
      }
    }
    currentPreview.value = results[activeResultIndex.value].preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
    updateAndResetActiveAction();
  }

  void updateActiveAction(WoxDirection woxDirection) {
    if (actions.isEmpty) {
      return;
    }

    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      if (activeActionIndex.value == actions.length - 1) {
        activeActionIndex.value = 0;
      } else {
        activeActionIndex.value++;
      }
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (activeActionIndex.value == 0) {
        activeActionIndex.value = actions.length - 1;
      } else {
        activeActionIndex.value--;
      }
    }
  }

  WoxQueryResult getQueryResultByIndex(int index) {
    return results[index];
  }

  WoxResultAction geActionByIndex(int index) {
    return actions[index];
  }

  GlobalKey getResultItemGlobalKeyByIndex(int index) {
    return resultGlobalKeys[index];
  }

  bool isResultActiveByIndex(int index) {
    return activeResultIndex.value == index;
  }

  bool isActionActiveByIndex(int index) {
    return activeActionIndex.value == index;
  }

  /// update active actions based on active result and reset active action index to 0
  void updateAndResetActiveAction() {
    Logger.instance.info(const UuidV4().generate(), "update active actions, current active result index: ${activeResultIndex.value}");

    var activeQueryResult = getActiveResult();
    if (activeQueryResult == null) {
      return;
    }

    activeActionIndex.value = 0;
    actions.assignAll(activeQueryResult.actions);
  }

  startRefreshSchedule() {
    var isRequesting = <String, bool>{};
    Timer.periodic(const Duration(milliseconds: 100), (timer) async {
      var isVisible = await windowManager.isVisible();
      if (!isVisible) {
        return;
      }

      refreshCounter = refreshCounter + 100;
      for (var result in results) {
        if (result.refreshInterval > 0 && refreshCounter % result.refreshInterval == 0) {
          if (isRequesting.containsKey(result.id)) {
            continue;
          } else {
            isRequesting[result.id] = true;
          }

          final traceId = const UuidV4().generate();
          final msg = WoxWebsocketMsg(
            requestId: const UuidV4().generate(),
            traceId: traceId,
            type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
            method: WoxMsgMethodEnum.WOX_MSG_METHOD_REFRESH.code,
            data: {
              "queryId": result.queryId,
              "refreshableResult": WoxRefreshableResult(
                resultId: result.id,
                title: result.title.value,
                subTitle: result.subTitle.value,
                icon: result.icon.value,
                preview: result.preview,
                tails: result.tails,
                contextData: result.contextData,
                refreshInterval: result.refreshInterval,
              ).toJson(),
            },
          );
          final startTime = DateTime.now().millisecondsSinceEpoch;
          WoxWebsocketMsgUtil.instance.sendMessage(msg).then((resp) {
            final endTime = DateTime.now().millisecondsSinceEpoch;
            if (endTime - startTime > 100) {
              Logger.instance.warn(traceId, "refresh result <${result.title}> (resultId: ${result.id}) too slow, cost ${endTime - startTime} ms");
            }

            // check result id, because the result may be removed during the refresh
            if (!results.any((element) => element.id == result.id)) {
              isRequesting.remove(result.id);
              Logger.instance.info(traceId, "result <${result.title}> (resultId: ${result.id}) is removed (maybe caused by new query) during refresh, skip update result");
              return;
            }

            final refreshResult = WoxRefreshableResult.fromJson(resp);
            result.title.value = refreshResult.title;
            result.subTitle.value = refreshResult.subTitle;
            result.icon.value = refreshResult.icon;
            result.preview = refreshResult.preview;
            result.tails.assignAll(refreshResult.tails);

            // only update preview data when current result is active
            final resultIndex = results.indexWhere((element) => element.id == result.id);
            if (isResultActiveByIndex(resultIndex)) {
              currentPreview.value = result.preview;
            }

            result.contextData = refreshResult.contextData;
            result.refreshInterval = refreshResult.refreshInterval;
            isRequesting.remove(result.id);
          });
        }
      }
    });
  }

  @override
  void dispose() {
    queryBoxFocusNode.dispose();
    queryBoxTextFieldController.dispose();
    resultScrollerController.dispose();
    super.dispose();
  }

  Future<void> openSettingWindow(String traceId, SettingWindowContext context) async {
    isInSettingView.value = true;
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (context.path == "/plugin/setting") {
        var settingController = Get.find<WoxSettingController>();
        await settingController.switchToPluginList(false);
        settingController.filterPluginKeywordController.text = context.param;
        settingController.filterPlugins();
        settingController.setFirstFilteredPluginDetailActive();

        WidgetsBinding.instance.addPostFrameCallback((_) async {
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
    });
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
          onQueryChanged(const UuidV4().generate(), changedQuery, "user arrow up history");
          selectQueryBoxAllText();
        }
      }
      return;
    }

    changeResultScrollPosition(const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_UP.code);
  }

  void handleQueryBoxArrowDown() {
    canArrowUpHistory = false;
    changeResultScrollPosition(const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
  }

  String transferChineseToPinYin(String str) {
    RegExp regExp = RegExp(r'[\u4e00-\u9fa5]');
    if (regExp.hasMatch(str)) {
      return PinyinHelper.getPinyin(str, separator: "", format: PinyinFormat.WITHOUT_TONE);
    }
    return str;
  }

  Future<void> handleDropFiles(DropDoneDetails details) async {
    Logger.instance.info(const UuidV4().generate(), "Received drop files: $details");

    await windowManager.focus();
    queryBoxFocusNode.requestFocus();

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
  Future<void> changeQueryIcon(String traceId, PlainQuery query) async {
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code) {
        queryIcon.value = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_FILE);
      }
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code) {
        queryIcon.value = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_TEXT);
      }
      return;
    }

    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      var img = await WoxApi.instance.getQueryIcon(query);
      queryIcon.value = img;
      return;
    }

    queryIcon.value = WoxImage.empty();
  }
}
