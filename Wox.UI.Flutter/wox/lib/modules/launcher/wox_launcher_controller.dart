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
import 'package:wox/interfaces/wox_launcher_interface.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/picker.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

class WoxLauncherController extends GetxController implements WoxLauncherInterface {
  final _query = PlainQuery.empty().obs;
  final queryIcon = WoxImage.empty().obs;
  final _activeResultIndex = 0.obs;
  final _activeActionIndex = 0.obs;
  final _resultItemGlobalKeys = <GlobalKey>[];
  final _resultActionItemGlobalKeys = <GlobalKey>[];
  final queryBoxFocusNode = FocusNode();
  final resultActionFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();
  final queryBoxScrollController = ScrollController(initialScrollOffset: 0.0);
  final resultActionTextFieldController = TextEditingController();
  final resultListViewScrollController = ScrollController(initialScrollOffset: 0.0);
  final resultActionListViewScrollController = ScrollController(initialScrollOffset: 0.0);
  final currentPreview = WoxPreview.empty().obs;
  final Rx<WoxTheme> woxTheme = WoxThemeUtil.instance.currentTheme.obs;
  final isShowActionPanel = false.obs;
  final isShowPreviewPanel = false.obs;
  final queryResults = <WoxQueryResult>[].obs;
  final _resultActions = <WoxResultAction>[].obs;
  final filterResultActions = <WoxResultAction>[].obs;
  var _clearQueryResultsTimer = Timer(const Duration(milliseconds: 200), () => {});
  var refreshCounter = 0;
  final latestQueryHistories = <QueryHistory>[];
  var selectedQueryHistoryIndex = 0;
  var lastQueryMode = WoxLastQueryModeEnum.WOX_LAST_QUERY_MODE_PRESERVE.code;
  var canArrowUpHistory = true;
  final isInSettingView = false.obs;
  var positionBeforeOpenSetting = const Offset(0, 0);

  @override
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

  @override
  Future<void> showApp(String traceId, ShowAppParams params) async {
    if (_query.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      canArrowUpHistory = true;
    }

    latestQueryHistories.assignAll(params.queryHistories);
    lastQueryMode = params.lastQueryMode;

    if (params.selectAll) {
      _selectQueryBoxAllText();
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

  @override
  Future<void> hideApp(String traceId) async {
    isShowActionPanel.value = false;
    await windowManager.hide();

    if (lastQueryMode == WoxLastQueryModeEnum.WOX_LAST_QUERY_MODE_PRESERVE.code) {
      //skip the first one, because it's the current query
      selectedQueryHistoryIndex = 0;
    } else {
      selectedQueryHistoryIndex = -1;
    }

    //clear query box text if query type is selection
    if (getCurrentQuery().queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      onQueryChanged(traceId, PlainQuery.emptyInput(), "clear input after hide app");
    }

    WoxApi.instance.onHide(_query.value);
  }

  PlainQuery getCurrentQuery() {
    return _query.value;
  }

  @override
  Future<void> toggleActionPanel(String traceId) async {
    if (queryResults.isEmpty) {
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
    resultActionTextFieldController.text = "";
    queryBoxFocusNode.requestFocus();
    resizeHeight();
  }

  void showActionPanel() {
    _activeActionIndex.value = 0;
    _resultActions.value = queryResults[_activeResultIndex.value].actions;
    filterResultActions.value = _resultActions;
    for (var _ in filterResultActions) {
      _resultActionItemGlobalKeys.add(GlobalKey());
    }
    isShowActionPanel.value = true;
    resultActionFocusNode.requestFocus();
    resizeHeight();
  }

  getActiveQueryResult() {
    return queryResults[_activeResultIndex.value];
  }

  @override
  Future<void> executeResultAction(String traceId) async {
    if (queryResults.isEmpty) {
      return;
    }

    Logger.instance.debug(traceId, "user execute result action");
    WoxQueryResult woxQueryResult = getActiveQueryResult();
    WoxResultAction woxResultAction = WoxResultAction.empty();
    if (isShowActionPanel.value) {
      if (filterResultActions.isNotEmpty) {
        woxResultAction = filterResultActions[_activeActionIndex.value];
      }
    } else {
      final defaultActionIndex = woxQueryResult.actions.indexWhere((element) => element.isDefault);
      if (defaultActionIndex != -1) {
        woxResultAction = woxQueryResult.actions[defaultActionIndex];
      }
    }
    if (woxResultAction.id.isNotEmpty) {
      final msg = WoxWebsocketMsg(
          requestId: const UuidV4().generate(),
          traceId: traceId,
          type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
          method: WoxMsgMethodEnum.WOX_MSG_METHOD_ACTION.code,
          data: {
            "resultId": woxQueryResult.id,
            "actionId": woxResultAction.id,
          });
      WoxWebsocketMsgUtil.instance.sendMessage(msg);
      hideActionPanel();
      if (!woxResultAction.preventHideAfterAction) {
        hideApp(traceId);
      }
    }
  }

  @override
  Future<void> autoCompleteQuery(String traceId) async {
    if (queryResults.isEmpty) {
      return;
    }

    final queryText = queryResults[_activeResultIndex.value].title;
    onQueryChanged(
      traceId,
      PlainQuery(
        queryId: const UuidV4().generate(),
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: queryText.value,
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
    if (_query.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      plainQuery.queryType = WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code;
      plainQuery.querySelection = _query.value.querySelection;
    }

    onQueryChanged(const UuidV4().generate(), plainQuery, "user input changed");
  }

  @override
  void onQueryChanged(String traceId, PlainQuery query, String changeReason, {bool moveCursorToEnd = false}) {
    Logger.instance.debug(traceId, "query changed: ${query.queryText}, reason: $changeReason");

    changeQueryIcon(traceId, query);

    //hide setting view if query changed
    if (isInSettingView.value) {
      isInSettingView.value = false;
    }

    if (query.queryId == "") {
      query.queryId = const UuidV4().generate();
    }

    _query.value = query;
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
    if (query.isEmpty) {
      _clearQueryResults();
      return;
    }

    // delay clear results, otherwise windows height will shrink immediately,
    // and then the query result is received which will expand the windows height. so it will causes window flicker
    _clearQueryResultsTimer.cancel();
    _clearQueryResultsTimer = Timer(
      const Duration(milliseconds: 100),
      () {
        _clearQueryResults();
      },
    );

    final msg = WoxWebsocketMsg(
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
    );
    WoxWebsocketMsgUtil.instance.sendMessage(msg);
  }

  @override
  void onQueryActionChanged(String traceId, String queryAction) {
    filterResultActions.value = _resultActions.where((element) => transferChineseToPinYin(element.name.toLowerCase()).contains(queryAction.toLowerCase())).toList().obs();
    filterResultActions.refresh();
  }

  @override
  void changeResultScrollPosition(String traceId, WoxEventDeviceType deviceType, WoxDirection direction) {
    final prevResultIndex = _activeResultIndex.value;
    _resetActiveResultIndex(direction);
    if (queryResults.length < MAX_LIST_VIEW_ITEM_COUNT) {
      queryResults.refresh();
      return;
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      if (_activeResultIndex.value < prevResultIndex) {
        resultListViewScrollController.jumpTo(0);
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code
            ? _isResultItemAtBottom(_activeResultIndex.value - 1)
            : !_isResultItemAtBottom(queryResults.length - 1);
        if (shouldJump) {
          resultListViewScrollController
              .jumpTo(resultListViewScrollController.offset.ceil() + WoxThemeUtil.instance.getResultItemHeight() * (_activeResultIndex.value - prevResultIndex).abs());
        }
      }
    }
    if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (_activeResultIndex.value > prevResultIndex) {
        resultListViewScrollController.jumpTo(WoxThemeUtil.instance.getResultListViewHeightByCount(queryResults.length - MAX_LIST_VIEW_ITEM_COUNT));
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code ? _isResultItemAtTop(_activeResultIndex.value + 1) : !_isResultItemAtTop(0);
        if (shouldJump) {
          resultListViewScrollController
              .jumpTo(resultListViewScrollController.offset.ceil() - WoxThemeUtil.instance.getResultItemHeight() * (_activeResultIndex.value - prevResultIndex).abs());
        }
      }
    }
    queryResults.refresh();
  }

  @override
  void changeResultActionScrollPosition(String traceId, WoxEventDeviceType deviceType, WoxDirection direction) {
    _resetActiveResultActionIndex(direction);
    filterResultActions.refresh();
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

      _onReceivedQueryResults(results);
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

  bool _isResultItemAtBottom(int index) {
    RenderBox? renderBox = _resultItemGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) return false;

    if (renderBox.localToGlobal(Offset.zero).dy.ceil() >=
        WoxThemeUtil.instance.getQueryBoxHeight() + WoxThemeUtil.instance.getResultListViewHeightByCount(MAX_LIST_VIEW_ITEM_COUNT - 1)) {
      return true;
    }
    return false;
  }

  bool _isResultItemAtTop(int index) {
    if (index < 0) {
      return false;
    }
    RenderBox? renderBox = _resultItemGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) return false;

    if (renderBox.localToGlobal(Offset.zero).dy.ceil() <= WoxThemeUtil.instance.getQueryBoxHeight()) {
      return true;
    }
    return false;
  }

  void _clearQueryResults() {
    queryResults.clear();
    isShowPreviewPanel.value = false;
    isShowActionPanel.value = false;
    _resultItemGlobalKeys.clear();
    resizeHeight();
  }

  void _onReceivedQueryResults(List<WoxQueryResult> results) {
    if (results.isEmpty || _query.value.queryId != results.first.queryId) {
      return;
    }

    //cancel clear results timer
    _clearQueryResultsTimer.cancel();

    //merge and sort results
    final currentQueryResults = queryResults.where((item) => item.queryId == _query.value.queryId).toList();
    final finalResults = List<WoxQueryResult>.from(currentQueryResults)..addAll(results);
    // wrap group into WoxQueryResult
    final groups = finalResults.map((e) => e.group).toSet().toList();
    // sort groups by group score desc
    groups.sort((a, b) => finalResults.where((element) => element.group == b).first.groupScore.compareTo(finalResults.where((element) => element.group == a).first.groupScore));

    var finalResultsSorted = <WoxQueryResult>[];
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

    queryResults.assignAll(finalResultsSorted);
    for (var _ in queryResults) {
      _resultItemGlobalKeys.add(GlobalKey());
    }

    //reset active result and preview
    if (currentQueryResults.isEmpty) {
      _resetActiveResult();
    }
    resizeHeight();
  }

  // select all text in query box
  void _selectQueryBoxAllText() {
    queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
  }

  void _resetActiveResult() {
    if (queryResults[0].isGroup) {
      _activeResultIndex.value = 1;
    } else {
      _activeResultIndex.value = 0;
    }
    if (resultListViewScrollController.hasClients) {
      resultListViewScrollController.jumpTo(0);
    }

    //reset preview
    if (queryResults.isNotEmpty) {
      currentPreview.value = queryResults[_activeResultIndex.value].preview;
    } else {
      currentPreview.value = WoxPreview.empty();
    }
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
  }

  void resizeHeight() {
    double resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(queryResults.length > 10 ? 10 : queryResults.length);
    if (isShowActionPanel.value || isShowPreviewPanel.value) {
      resultHeight = WoxThemeUtil.instance.getResultListViewHeightByCount(10);
    }
    if (queryResults.isNotEmpty) {
      resultHeight += woxTheme.value.resultContainerPaddingTop + woxTheme.value.resultContainerPaddingBottom;
      resultHeight += WoxThemeUtil.instance.getResultTipHeight();
    }
    final totalHeight = WoxThemeUtil.instance.getQueryBoxHeight() + resultHeight;
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
      if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.info(const UuidV4().generate(), "Resize window height to $totalHeightFinal");
      windowManager.setSize(Size(800, totalHeightFinal));
    } else {
      if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.info(const UuidV4().generate(), "Resize window height to $totalHeight");
      windowManager.setSize(Size(800, totalHeight.toDouble()));
    }
  }

  void _resetActiveResultIndex(WoxDirection woxDirection) {
    if (queryResults.isEmpty) {
      return;
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      // select next none group result
      _activeResultIndex.value++;
      if (_activeResultIndex.value == queryResults.length) {
        _activeResultIndex.value = 0;
      }
      while (queryResults[_activeResultIndex.value].isGroup) {
        _activeResultIndex.value++;
        if (_activeResultIndex.value == queryResults.length) {
          _activeResultIndex.value = 0;
          break;
        }
      }
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      // select previous none group result
      _activeResultIndex.value--;
      if (_activeResultIndex.value == -1) {
        _activeResultIndex.value = queryResults.length - 1;
      }
      while (queryResults[_activeResultIndex.value].isGroup) {
        _activeResultIndex.value--;
        if (_activeResultIndex.value == -1) {
          _activeResultIndex.value = queryResults.length - 1;
          break;
        }
      }
    }
    currentPreview.value = queryResults[_activeResultIndex.value].preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
  }

  void _resetActiveResultActionIndex(WoxDirection woxDirection) {
    if (filterResultActions.isEmpty) {
      return;
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      if (_activeActionIndex.value == filterResultActions.length - 1) {
        _activeActionIndex.value = 0;
      } else {
        _activeActionIndex.value++;
      }
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (_activeActionIndex.value == 0) {
        _activeActionIndex.value = filterResultActions.length - 1;
      } else {
        _activeActionIndex.value--;
      }
    }
  }

  WoxQueryResult getQueryResultByIndex(int index) {
    return queryResults[index];
  }

  WoxResultAction getQueryResultActionByIndex(int index) {
    return filterResultActions[index];
  }

  GlobalKey getResultItemGlobalKeyByIndex(int index) {
    return _resultItemGlobalKeys[index];
  }

  GlobalKey getResultActionItemGlobalKeyByIndex(int index) {
    return _resultActionItemGlobalKeys[index];
  }

  bool isQueryResultActiveByIndex(int index) {
    return _activeResultIndex.value == index;
  }

  bool isResultActionActiveByIndex(int index) {
    return _activeActionIndex.value == index;
  }

  startRefreshSchedule() {
    var isRequesting = <String, bool>{};
    Timer.periodic(const Duration(milliseconds: 100), (timer) async {
      var isVisible = await windowManager.isVisible();
      if (!isVisible) {
        return;
      }

      refreshCounter = refreshCounter + 100;
      for (var result in queryResults) {
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
            if (!queryResults.any((element) => element.id == result.id)) {
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
            final resultIndex = queryResults.indexWhere((element) => element.id == result.id);
            if (isQueryResultActiveByIndex(resultIndex)) {
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
    resultListViewScrollController.dispose();
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
      if (selectedQueryHistoryIndex < latestQueryHistories.length - 1) {
        selectedQueryHistoryIndex = selectedQueryHistoryIndex + 1;
        var changedQuery = latestQueryHistories[selectedQueryHistoryIndex].query;
        if (changedQuery != null) {
          onQueryChanged(const UuidV4().generate(), changedQuery, "user arrow up history");
          _selectQueryBoxAllText();
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

  Future<void> changeQueryIcon(String traceId, PlainQuery query) async {
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code) {
        queryIcon.value = WoxImage(
            imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code,
            imageData:
                '<svg t="1704957058350" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="4383" width="200" height="200"><path d="M127.921872 233.342828H852.118006c24.16765 0 43.960122 19.792472 43.960122 43.960122v522.104578c0 24.16765-19.792472 43.960122-43.960122 43.960122H172.090336c-24.16765 0-43.960122-19.792472-43.960122-43.960122L127.921872 233.342828z" fill="#FFB300" p-id="4384"></path><path d="M156.4647 180.63235h312.721058c15.625636 0 28.334486 13.125534 28.334486 29.376195V233.342828H127.921872v-23.334283c0-16.250661 12.917192-29.376195 28.542828-29.376195z" fill="#FFA000" p-id="4385"></path><path d="M361.889725 258.343845h348.347508v535.855138H312.512716V303.137335z" fill="#FFFFFF" p-id="4386"></path><path d="M170.631943 372.723499h282.719837l59.7941-47.918616H852.118006c23.542625 0 42.710071 19.792472 42.710071 43.960122v430.642523c0 24.16765-19.167447 43.960122-42.710071 43.960122H170.631943c-23.542625 0-42.710071-19.792472-42.710071-43.960122V416.683622c0-24.16765 19.375788-43.960122 42.710071-43.960123z" fill="#FFD54F" p-id="4387"></path><path d="M361.473042 303.76236l-48.960326-0.625025 48.960326-44.79349z" fill="#BDBDBD" p-id="4388"></path></svg>');
        return;
      }
      if (query.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code) {
        queryIcon.value = WoxImage(
            imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code,
            imageData:
                '<svg t="1704958243895" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="5762" width="200" height="200"><path d="M925.48105 1024H98.092461a98.51895 98.51895 0 0 1-97.879217-98.945439V98.732195A98.732195 98.732195 0 0 1 98.092461 0.426489h827.388589a98.732195 98.732195 0 0 1 98.305706 98.305706v826.322366a98.51895 98.51895 0 0 1-98.305706 98.945439z m-829.094544-959.600167a33.052895 33.052895 0 0 0-32.199917 32.626406v829.734277a32.83965 32.83965 0 0 0 32.199917 33.26614h831.653477a32.83965 32.83965 0 0 0 31.773428-33.26614V97.026239a33.266139 33.266139 0 0 0-32.626406-32.626406z" fill="#0077F0" p-id="5763"></path><path d="M281.69596 230.943773h460.60808v73.569347h-187.655144v488.969596h-85.297792V304.51312h-187.655144z" fill="#0077F0" opacity=".5" p-id="5764"></path></svg>');
        return;
      }
    }

    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      var img = await WoxApi.instance.getQueryIcon(query);
      queryIcon.value = img;
      return;
    }

    queryIcon.value = WoxImage.empty();
  }
}
