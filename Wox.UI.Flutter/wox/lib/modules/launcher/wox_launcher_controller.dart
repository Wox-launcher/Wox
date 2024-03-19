import 'dart:async';
import 'dart:io';
import 'dart:ui';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:lpinyin/lpinyin.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_last_query_mode_enum.dart';
import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/enums/wox_msg_type_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';
import 'package:wox/interfaces/wox_launcher_interface.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

class WoxLauncherController extends GetxController implements WoxLauncherInterface {
  final _query = WoxChangeQuery.empty().obs;
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
  final activeResultIndex = 0.obs;
  final activeActionIndex = 0.obs;
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

  @override
  Future<void> toggleApp(String traceId, ShowAppParams params) async {
    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      hideApp(traceId);
    } else {
      showApp(traceId, params);
    }
  }

  @override
  Future<void> showApp(String traceId, ShowAppParams params) async {
    canArrowUpHistory = true;
    latestQueryHistories.assignAll(params.queryHistories);
    lastQueryMode = params.lastQueryMode;

    if (params.selectAll) {
      _selectQueryBoxAllText();
    }
    if (params.position.type == WoxPositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    }
    await windowManager.show();
    await windowManager.focus();
    queryBoxFocusNode.requestFocus();

    WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_VISIBILITY_CHANGED.code,
        data: {"isVisible": "true", "query": _query.value.toJson()},
      ),
    );
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
      onQueryChanged(traceId, WoxChangeQuery.emptyInput(), "clear input after hide app");
    }

    WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_VISIBILITY_CHANGED.code,
        data: {"isVisible": "false", "query": _query.value.toJson()},
      ),
    );
  }

  WoxChangeQuery getCurrentQuery() {
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

  @override
  Future<void> executeResultAction(String traceId) async {
    if (queryResults.isEmpty) {
      return;
    }

    Logger.instance.debug(traceId, "user execute result action");
    WoxQueryResult woxQueryResult = queryResults[_activeResultIndex.value];
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
      WoxChangeQuery(
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

    WoxChangeQuery woxChangeQuery = WoxChangeQuery(
      queryId: const UuidV4().generate(),
      queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
      queryText: value,
      querySelection: Selection.empty(),
    );

    // do filter if query type is selection
    if (_query.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      woxChangeQuery.queryType = WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code;
      woxChangeQuery.querySelection = _query.value.querySelection;
    }

    onQueryChanged(const UuidV4().generate(), woxChangeQuery, "user input changed");
  }

  @override
  void onQueryChanged(String traceId, WoxChangeQuery query, String changeReason, {bool moveCursorToEnd = false}) {
    Logger.instance.debug(traceId, "query changed: ${query.queryText}, reason: $changeReason");

    if (query.queryId == "") {
      query.queryId = const UuidV4().generate();
    }

    _query.value = query;
    isShowActionPanel.value = false;

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
  Future<List<String>> pickFiles(String traceId, PickFilesParams params) async {
    if (params.isDirectory) {
      String? selectedDirectory = await FilePicker.platform.getDirectoryPath();
      if (selectedDirectory != null) {
        return [selectedDirectory];
      }
    }

    return [];
  }

  @override
  void onQueryActionChanged(String traceId, String queryAction) {
    filterResultActions.value = _resultActions.where((element) => transferChineseToPinYin(element.name.toLowerCase()).contains(queryAction.toLowerCase())).toList().obs();
    filterResultActions.refresh();
  }

  @override
  void changeResultScrollPosition(String traceId, WoxEventDeviceType deviceType, WoxDirection direction) {
    _resetActiveResultIndex(direction);
    if (queryResults.length < MAX_LIST_VIEW_ITEM_COUNT) {
      queryResults.refresh();
      return;
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      if (_activeResultIndex.value == 0) {
        resultListViewScrollController.jumpTo(0);
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code
            ? _isResultItemAtBottom(_activeResultIndex.value - 1)
            : !_isResultItemAtBottom(queryResults.length - 1);
        if (shouldJump) {
          resultListViewScrollController.jumpTo(resultListViewScrollController.offset.ceil() + WoxThemeUtil.instance.getResultListViewHeightByCount(1));
        }
      }
    }
    if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (_activeResultIndex.value == queryResults.length - 1) {
        resultListViewScrollController.jumpTo(WoxThemeUtil.instance.getResultListViewHeightByCount(queryResults.length - MAX_LIST_VIEW_ITEM_COUNT));
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code ? _isResultItemAtTop(_activeResultIndex.value + 1) : !_isResultItemAtTop(0);
        if (shouldJump) {
          resultListViewScrollController.jumpTo(resultListViewScrollController.offset.ceil() - WoxThemeUtil.instance.getResultListViewHeightByCount(1));
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
    if (msg.method != WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
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
      onQueryChanged(msg.traceId, WoxChangeQuery.fromJson(msg.data), "receive change query from wox", moveCursorToEnd: true);
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "ChangeTheme") {
      final theme = WoxTheme.fromJson(msg.data);
      WoxThemeUtil.instance.changeTheme(theme);
      woxTheme.value = theme;
      responseWoxWebsocketRequest(msg, true, null);
    } else if (msg.method == "PickFiles") {
      final pickFilesParams = PickFilesParams.fromJson(msg.data);
      final files = await pickFiles(msg.traceId, pickFilesParams);
      responseWoxWebsocketRequest(msg, true, files);
    } else if (msg.method == "OpenSettingWindow") {
      isInSettingView.value = true;
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
    if (renderBox?.localToGlobal(Offset.zero).dy.ceil() ==
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
    if (renderBox?.localToGlobal(Offset.zero).dy.ceil() == WoxThemeUtil.instance.getQueryBoxHeight()) {
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
    finalResults.sort((a, b) => b.score.compareTo(a.score));
    queryResults.assignAll(finalResults);
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
    _activeResultIndex.value = 0;
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
    final totalHeight = WoxThemeUtil.instance.getQueryBoxHeight() +
        resultHeight +
        (queryResults.isNotEmpty ? woxTheme.value.resultContainerPaddingTop + woxTheme.value.resultContainerPaddingBottom : 0);
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
      if (_activeResultIndex.value == queryResults.length - 1) {
        _activeResultIndex.value = 0;
      } else {
        _activeResultIndex.value++;
      }
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (_activeResultIndex.value == 0) {
        _activeResultIndex.value = queryResults.length - 1;
      } else {
        _activeResultIndex.value--;
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
            currentPreview.value = refreshResult.preview;
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

  void moveQueryBoxCursorToStart() {
    queryBoxTextFieldController.selection = TextSelection.fromPosition(const TextPosition(offset: 0));
    queryBoxScrollController.jumpTo(0);
  }

  void moveQueryBoxCursorToEnd() {
    queryBoxTextFieldController.selection = TextSelection.collapsed(offset: queryBoxTextFieldController.text.length);
    queryBoxScrollController.jumpTo(queryBoxScrollController.position.maxScrollExtent);
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

    WoxChangeQuery woxChangeQuery = WoxChangeQuery(
      queryId: const UuidV4().generate(),
      queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code,
      queryText: "",
      querySelection: Selection(type: WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code, text: "", filePaths: details.files.map((e) => e.path).toList()),
    );

    onQueryChanged(const UuidV4().generate(), woxChangeQuery, "user drop files");
  }
}
