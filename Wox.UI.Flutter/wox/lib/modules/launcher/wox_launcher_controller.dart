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
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_web_socket_msg_type_enum.dart';
import 'package:wox/interfaces/wox_launcher_interface.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

class WoxLauncherController extends GetxController implements WoxLauncherInterface {
  final query = WoxChangeQuery.empty().obs;
  final queryBoxFocusNode = FocusNode();
  final resultActionFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();
  final resultActionTextFieldController = TextEditingController();
  final scrollController = ScrollController(initialScrollOffset: 0.0);
  final currentPreview = WoxPreview.empty().obs;
  final WoxTheme woxTheme = WoxThemeUtil.instance.currentTheme.obs();
  final activeResultIndex = 0.obs;
  final activeActionIndex = 0.obs;
  final isShowActionPanel = false.obs;
  final isShowPreviewPanel = false.obs;
  final queryResults = <WoxQueryResult>[].obs;
  final resultActions = <WoxResultAction>[].obs;
  final resultItemGlobalKeys = <GlobalKey>[];
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
      isShowActionPanel.value = true;
      resultActionFocusNode.requestFocus();
    }
    _resizeHeight();
  }

  @override
  Future<void> executeResultAction() async {
    final defaultActionIndex = queryResults[activeResultIndex.value].actions.indexWhere((element) => element.isDefault);
    if (defaultActionIndex != -1) {
      activeActionIndex.value = defaultActionIndex;
      final result = queryResults[activeResultIndex.value];
      final action = result.actions[activeActionIndex.value];
      final msg = WoxWebsocketMsg(id: const UuidV4().generate(), method: "Action", type: WoxWebsocketMsgTypeEnum.WOX_WEBSOCKET_MSG_TYPE_REQUEST.code, data: {
        "resultId": result.id,
        "actionId": action.id,
      });
      WoxWebsocketMsgUtil.instance.sendMessage(msg);
      if (!action.preventHideAfterAction) {
        hideApp();
      }
    }
  }

  @override
  void onQueryChanged(WoxChangeQuery query) {
    this.query.value = query;
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

    final msg = WoxWebsocketMsg(id: const UuidV4().generate(), type: WoxWebsocketMsgTypeEnum.WOX_WEBSOCKET_MSG_TYPE_REQUEST.code, method: "Query", data: {
      "queryId": query.queryId,
      "queryType": query.queryType,
      "queryText": query.queryText,
      "querySelection": query.querySelection.toJson(),
    });
    WoxWebsocketMsgUtil.instance.sendMessage(msg);
  }

  @override
  void changeScrollPosition(WoxEventDeviceType deviceType, WoxDirection direction) {
    _resetActiveResultIndex(direction);
    if (queryResults.length < MAX_LIST_VIEW_ITEM_COUNT) {
      queryResults.refresh();
      return;
    }
    if (deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code) {
      if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
        if (activeResultIndex.value == 0) {
          scrollController.jumpTo(0);
        } else {
          if (_isResultItemAtBottom(activeResultIndex.value - 1)) {
            scrollController.jumpTo(scrollController.offset.ceil() + WoxThemeUtil.instance.getResultListViewHeightByCount(1));
          }
        }
      }
      if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
        if (activeResultIndex.value == queryResults.length - 1) {
          scrollController.jumpTo(WoxThemeUtil.instance.getResultListViewHeightByCount(queryResults.length - MAX_LIST_VIEW_ITEM_COUNT));
        } else {
          if (_isResultItemAtTop(activeResultIndex.value + 1)) {
            scrollController.jumpTo(scrollController.offset.ceil() - WoxThemeUtil.instance.getResultListViewHeightByCount(1));
          }
        }
      }
    }
    if (deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_MOUSE.code) {
      if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
        if (activeResultIndex.value == 0) {
          scrollController.jumpTo(0);
        } else {
          if (!_isResultItemAtBottom(queryResults.length - 1)) {
            scrollController.jumpTo(scrollController.offset.ceil() + WoxThemeUtil.instance.getResultListViewHeightByCount(1));
          }
        }
      }
      if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
        if (activeResultIndex.value == queryResults.length - 1) {
          scrollController.jumpTo(WoxThemeUtil.instance.getResultListViewHeightByCount(queryResults.length - MAX_LIST_VIEW_ITEM_COUNT));
        } else {
          if (!_isResultItemAtTop(0)) {
            scrollController.jumpTo(scrollController.offset.ceil() - WoxThemeUtil.instance.getResultListViewHeightByCount(1));
          }
        }
      }
    }
    queryResults.refresh();
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
    }
  }

  bool _isResultItemAtBottom(int index) {
    RenderBox? renderBox = resultItemGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
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
    RenderBox? renderBox = resultItemGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
    if (renderBox?.localToGlobal(Offset.zero).dy.ceil() == WoxThemeUtil.instance.getQueryBoxHeight()) {
      return true;
    }
    return false;
  }

  void _clearQueryResults() {
    queryResults.clear();
    resultItemGlobalKeys.clear();
    _resizeHeight();
  }

  void _onReceivedQueryResults(List<WoxQueryResult> results) {
    if (results.isEmpty || query.value.queryId != results.first.queryId) {
      return;
    }

    //cancel clear results timer
    _clearQueryResultsTimer.cancel();

    //merge and sort results
    final currentQueryResults = queryResults.where((item) => item.queryId == query.value.queryId).toList();
    final finalResults = List<WoxQueryResult>.from(currentQueryResults)..addAll(results);
    finalResults.sort((a, b) => b.score.compareTo(a.score));
    queryResults.assignAll(finalResults);
    for (var _ in queryResults) {
      resultItemGlobalKeys.add(GlobalKey());
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
    activeResultIndex.value = 0;

    //reset preview
    if (queryResults.isNotEmpty) {
      currentPreview.value = queryResults[activeResultIndex.value].preview;
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
    final totalHeight =
        WoxThemeUtil.instance.getQueryBoxHeight() + resultHeight + (queryResults.isNotEmpty ? woxTheme.resultContainerPaddingTop + woxTheme.resultContainerPaddingBottom : 0);
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
      if (activeResultIndex.value == queryResults.length - 1) {
        activeResultIndex.value = 0;
      } else {
        activeResultIndex.value++;
      }
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (activeResultIndex.value == 0) {
        activeResultIndex.value = queryResults.length - 1;
      } else {
        activeResultIndex.value--;
      }
    }
    currentPreview.value = queryResults[activeResultIndex.value].preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
  }

  WoxQueryResult getQueryResultByIndex(int index) {
    return queryResults[index];
  }

  GlobalKey getResultItemGlobalKeyByIndex(int index) {
    return resultItemGlobalKeys[index];
  }

  @override
  void dispose() {
    queryBoxFocusNode.dispose();
    queryBoxTextFieldController.dispose();
    scrollController.dispose();
    super.dispose();
  }
}
