import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/enums/wox_msg_type_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
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

  @override
  Future<void> toggleApp(ShowAppParams params) async {
    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      hideApp();
    } else {
      showApp(params);
    }
  }

  @override
  Future<void> showApp(ShowAppParams params) async {
    if (params.selectAll) {
      _selectQueryBoxAllText();
    }
    if (params.position.type == WoxPositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    }
    await windowManager.show();
    await windowManager.focus();
    queryBoxFocusNode.requestFocus();
  }

  @override
  Future<void> hideApp() async {
    isShowActionPanel.value = false;
    await windowManager.hide();
  }

  @override
  Future<void> toggleActionPanel() async {
    if (queryResults.isEmpty) {
      return;
    }
    if (isShowActionPanel.value) {
      isShowActionPanel.value = false;
      queryBoxFocusNode.requestFocus();
    } else {
      _activeActionIndex.value = 0;
      _resultActions.value = queryResults[_activeResultIndex.value].actions;
      filterResultActions.value = _resultActions;
      for (var _ in filterResultActions) {
        _resultActionItemGlobalKeys.add(GlobalKey());
      }
      isShowActionPanel.value = true;
      resultActionFocusNode.requestFocus();
    }
    _resizeHeight();
  }

  @override
  Future<void> executeResultAction() async {
    WoxQueryResult woxQueryResult = queryResults[_activeResultIndex.value];
    WoxResultAction woxResultAction = WoxResultAction.empty();
    if (isShowActionPanel.value) {
      woxResultAction = filterResultActions[_activeActionIndex.value];
    } else {
      final defaultActionIndex = woxQueryResult.actions.indexWhere((element) => element.isDefault);
      if (defaultActionIndex != -1) {
        woxResultAction = woxQueryResult.actions[defaultActionIndex];
      }
    }
    if (woxResultAction.id.isNotEmpty) {
      final msg = WoxWebsocketMsg(id: const UuidV4().generate(), type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code, method: WoxMsgMethodEnum.WOX_MSG_METHOD_ACTION.code, data: {
        "resultId": woxQueryResult.id,
        "actionId": woxResultAction.id,
      });
      WoxWebsocketMsgUtil.instance.sendMessage(msg);
      if (!woxResultAction.preventHideAfterAction) {
        hideApp();
      }
    }
  }

  @override
  void onQueryChanged(WoxChangeQuery query) {
    _query.value = query;
    isShowActionPanel.value = false;
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      queryBoxTextFieldController.text = query.queryText;
    } else {
      queryBoxTextFieldController.text = query.toString();
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

    final msg = WoxWebsocketMsg(id: const UuidV4().generate(), type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code, method: WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code, data: {
      "queryId": query.queryId,
      "queryType": query.queryType,
      "queryText": query.queryText,
      "querySelection": query.querySelection.toJson(),
    });
    WoxWebsocketMsgUtil.instance.sendMessage(msg);
  }

  @override
  void onQueryActionChanged(String queryAction) {
    filterResultActions.value = _resultActions.where((element) => element.name.toLowerCase().contains(queryAction.toLowerCase())).toList().obs();
    filterResultActions.refresh();
  }

  @override
  void changeResultScrollPosition(WoxEventDeviceType deviceType, WoxDirection direction) {
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
  void changeResultActionScrollPosition(WoxEventDeviceType deviceType, WoxDirection direction) {
    _resetActiveResultActionIndex(direction);
    filterResultActions.refresh();
  }

  void handleWebSocketMessage(event) {
    var msg = WoxWebsocketMsg.fromJson(jsonDecode(event));
    Logger.instance.info("Received message: ${msg.toJson()}");
    if (msg.method == "ToggleApp") {
      toggleApp(ShowAppParams.fromJson(msg.data));
    } else if (msg.method == "HideApp") {
      hideApp();
    } else if (msg.method == "ShowApp") {
      showApp(ShowAppParams.fromJson(msg.data));
    } else if (msg.method == "ChangeQuery") {
      var changedQuery = WoxChangeQuery.fromJson(msg.data);
      changedQuery.queryId = const UuidV4().generate();
      onQueryChanged(changedQuery);
    } else if (msg.method == "Query") {
      var results = <WoxQueryResult>[];
      for (var item in msg.data) {
        results.add(WoxQueryResult.fromJson(item));
      }
      _onReceivedQueryResults(results);
    } else if (msg.method == "ChangeTheme") {
      final theme = WoxTheme.fromJson(msg.data);
      WoxThemeUtil.instance.changeTheme(theme);
      woxTheme.value = theme;
    }
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
    _resizeHeight();
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
    _resizeHeight();
  }

  // select all text in query box
  void _selectQueryBoxAllText() {
    queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
  }

  void _resetActiveResult() {
    _activeResultIndex.value = 0;

    //reset preview
    if (queryResults.isNotEmpty) {
      currentPreview.value = queryResults[_activeResultIndex.value].preview;
    } else {
      currentPreview.value = WoxPreview.empty();
    }
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
  }

  void _resizeHeight() {
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

      final totalHeightFinal = totalHeight.toDouble() + (10 / window.devicePixelRatio).ceil();
      Logger.instance.info("Resize window height to $totalHeightFinal");
      windowManager.setSize(Size(800, totalHeightFinal));
    } else {
      Logger.instance.info("Resize window height to $totalHeight");
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

  @override
  void dispose() {
    queryBoxFocusNode.dispose();
    queryBoxTextFieldController.dispose();
    resultListViewScrollController.dispose();
    super.dispose();
  }
}
