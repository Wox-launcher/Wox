import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_web_socket_msg_type_enum.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

class WoxLauncherController extends GetxController {
  final query = WoxChangeQuery.empty().obs;
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();
  final scrollController = ScrollController(initialScrollOffset: 0.0);
  final currentPreview = WoxPreview.empty().obs;
  final WoxTheme woxTheme = WoxThemeUtil.instance.currentTheme.obs();
  final int baseItemHeight = 50;
  final activeResultIndex = 0.obs;
  final activeActionIndex = 0.obs;
  final isShowActionPanel = false.obs;
  final isShowPreviewPanel = false.obs;
  final queryResults = <WoxQueryResult>[].obs;
  var clearQueryResultsTimer = Timer(const Duration(milliseconds: 200), () => {});
  String currentScrollDirection = "down";

  Future<void> toggleApp(ShowAppParams params) async {
    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      hideApp();
    } else {
      showApp(params);
    }
  }

  Future<void> showApp(ShowAppParams params) async {
    if (params.selectAll) {
      _selectQueryBoxAllText();
    }
    if (params.position.type == PositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    }
    await windowManager.show();
    queryBoxFocusNode.requestFocus();
  }

  Future<void> hideApp() async {
    await windowManager.blur();
    await windowManager.hide();
  }

  // execute action provided by result item
  Future<void> handleResultItemAction() async {
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

  void onQueryChanged(WoxChangeQuery query) {
    this.query.value = query;
    isShowActionPanel.value = false;
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      queryBoxTextFieldController.text = query.queryText;
    } else {
      queryBoxTextFieldController.text = query.toString();
    }
    if (query.isEmpty) {
      queryResults.clear();
      _resizeHeight();
      return;
    }
    // delay clear results, otherwise windows height will shrink immediately,
    // and then the query result is received which will expand the windows height. so it will causes window flicker
    clearQueryResultsTimer.cancel();
    clearQueryResultsTimer = Timer(
      const Duration(milliseconds: 100),
      () {
        queryResults.clear();
        _resizeHeight();
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

  void arrowUp() {
    currentScrollDirection = "up";
    _resetActiveResultIndex();
    _resetResultPreview();
    _arrowMoveScrollbar();
  }

  void arrowDown() {
    currentScrollDirection = "down";
    _resetActiveResultIndex();
    _resetResultPreview();
    _arrowMoveScrollbar();
  }

  void mouseWheelScrollUp() {
    currentScrollDirection = "up";
    _resetActiveResultIndex();
    _resetResultPreview();
    _mouseWheelMoveScrollbar();
  }

  void mouseWheelScrollDown() {
    currentScrollDirection = "down";
    _resetActiveResultIndex();
    _resetResultPreview();
    _mouseWheelMoveScrollbar();
  }

  void _onReceivedQueryResults(List<WoxQueryResult> results) {
    if (results.isEmpty || query.value.queryId != results.first.queryId) {
      return;
    }

    //cancel clear results timer
    clearQueryResultsTimer.cancel();

    //merge and sort results
    final currentQueryResults = queryResults.where((item) => item.queryId == query.value.queryId).toList();
    final finalResults = List<WoxQueryResult>.from(currentQueryResults)..addAll(results);
    finalResults.sort((a, b) => b.score.compareTo(a.score));
    queryResults.assignAll(finalResults);

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

  // check if item exists in bottom of list view
  bool _isBottomItemExists() {
    if (currentScrollDirection == "down") {
      if (activeResultIndex.value > queryResults.length - MAX_LIST_VIEW_ITEM_COUNT) {
        return true;
      }
    }
    if (currentScrollDirection == "up") {
      if (activeResultIndex.value > queryResults.length - 1 - MAX_LIST_VIEW_ITEM_COUNT) {
        return true;
      }
    }
    return false;
  }

  // check if item exists in top of list view
  bool _isOffsetOverFlowBoundary(double offset) {
    if (offset < 0) {
      return true;
    }
    if (offset > scrollController.position.maxScrollExtent) {
      return true;
    }
    return false;
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
    double resultHeight = getResultHeightByCount(queryResults.length > 10 ? 10 : queryResults.length);
    if (isShowActionPanel.value || isShowPreviewPanel.value) {
      resultHeight = getMaxHeight();
    }
    final totalHeight = queryBoxContainerHeight() + resultHeight + (queryResults.isNotEmpty ? woxTheme.resultContainerPaddingTop + woxTheme.resultContainerPaddingBottom : 0);
    windowManager.setSize(Size(800, totalHeight.toDouble()));
  }

  void _arrowMoveScrollbar() {
    if (queryResults.length <= MAX_LIST_VIEW_ITEM_COUNT) {
      queryResults.refresh();
      return;
    }
    if (currentScrollDirection == "down") {
      if (activeResultIndex.value == 0) {
        scrollController.jumpTo(0);
      } else if (activeResultIndex.value >= MAX_LIST_VIEW_ITEM_COUNT && !_isOffsetOverFlowBoundary(scrollController.offset.ceil() + getResultHeightByCount(1))) {
        scrollController.jumpTo(scrollController.offset.ceil() + getResultHeightByCount(1));
      }
    }

    if (currentScrollDirection == "up") {
      if (activeResultIndex.value == queryResults.length - 1) {
        scrollController.jumpTo(getResultHeightByCount(queryResults.length - MAX_LIST_VIEW_ITEM_COUNT));
      } else if (activeResultIndex.value <= queryResults.length - 1 - MAX_LIST_VIEW_ITEM_COUNT &&
          !_isOffsetOverFlowBoundary(scrollController.offset.ceil() - getResultHeightByCount(1))) {
        scrollController.jumpTo(scrollController.offset.ceil() - getResultHeightByCount(1));
      }
    }
    queryResults.refresh();
  }

  void _mouseWheelMoveScrollbar() {
    if (queryResults.length <= MAX_LIST_VIEW_ITEM_COUNT) {
      queryResults.refresh();
      return;
    }
    if (currentScrollDirection == "down") {
      if (activeResultIndex.value == 0) {
        scrollController.jumpTo(0);
      } else if (!_isBottomItemExists()) {
        scrollController.jumpTo(scrollController.offset.ceil() + getResultHeightByCount(1));
      }
    }
    if (currentScrollDirection == "up") {
      if (activeResultIndex.value == queryResults.length - 1) {
        scrollController.jumpTo(getResultHeightByCount(queryResults.length - MAX_LIST_VIEW_ITEM_COUNT));
      } else if (!_isBottomItemExists()) {
        scrollController.jumpTo(scrollController.offset.ceil() - getResultHeightByCount(1));
      }
    }
    queryResults.refresh();
  }

  void _resetActiveResultIndex() {
    if (queryResults.isEmpty) {
      return;
    }
    if (currentScrollDirection == "down") {
      if (activeResultIndex.value == queryResults.length - 1) {
        activeResultIndex.value = 0;
      } else {
        activeResultIndex.value++;
      }
    } else {
      if (activeResultIndex.value == 0) {
        activeResultIndex.value = queryResults.length - 1;
      } else {
        activeResultIndex.value--;
      }
    }
  }

  void _resetResultPreview() {
    currentPreview.value = queryResults[activeResultIndex.value].preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
  }

  void toggleActionPanel() {}

  // query box container height
  double queryBoxContainerHeight() {
    return WoxThemeUtil.instance.getWoxBoxContainerHeight();
  }

  // single result item height
  double baseResultItemHeight() {
    return (baseItemHeight + woxTheme.resultItemPaddingTop + woxTheme.resultItemPaddingBottom).toDouble();
  }

  double getResultHeightByCount(int count) {
    if (count == 0) {
      return 0;
    }
    return baseResultItemHeight() * count;
  }

  double getMaxHeight() {
    return getResultHeightByCount(MAX_LIST_VIEW_ITEM_COUNT) + woxTheme.resultContainerPaddingTop + woxTheme.resultContainerPaddingBottom;
  }

  WoxQueryResult getQueryResultByIndex(int index) {
    return queryResults[index];
  }

  @override
  void dispose() {
    queryBoxFocusNode.dispose();
    queryBoxTextFieldController.dispose();
    scrollController.dispose();
    super.dispose();
  }
}
