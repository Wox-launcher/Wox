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
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/websocket.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxLauncherController extends GetxController {
  final query = WoxQuery.empty().obs;
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();
  final currentPreview = WoxPreview.empty().obs;
  final WoxTheme woxTheme = WoxThemeUtil.instance.currentTheme.obs();
  final int baseItemHeight = 50;
  final activeResultIndex = 0.obs;
  final isShowActionPanel = false.obs;
  final isShowPreviewPanel = false.obs;
  final queryResults = <WoxQueryResult>[].obs;
  late final WoxWebsocket ws;
  var clearQueryResultsTimer = Timer(const Duration(milliseconds: 200), () => {});

  @override
  void onInit() {
    super.onInit();
    _setupWebSocket();
  }

  void _setupWebSocket() {
    ws = WoxWebsocket(Uri.parse("ws://localhost:${Env.serverPort}/ws"), onMessageReceived: _handleWebSocketMessage);
    ws.connect();
  }

  Future<void> toggleApp(ShowAppParams params) async {
    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      hide();
    } else {
      show(params);
    }
  }

  Future<void> show(ShowAppParams params) async {
    if (params.selectAll) {
      selectAll();
    }
    if (params.position.type == PositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    }

    await windowManager.show();
    queryBoxFocusNode.requestFocus();
  }

  Future<void> hide() async {
    await windowManager.blur();
    await windowManager.hide();
  }

  void selectAll() {
    queryBoxTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryBoxTextFieldController.text.length);
  }

  Future<void> selectResult() async {}

  void onQueryChanged(WoxQuery query) {
    this.query.value = query;
    isShowActionPanel.value = false;
    if (query.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code) {
      queryBoxTextFieldController.text = query.queryText;
    } else {
      queryBoxTextFieldController.text = query.toString();
    }
    if (query.isEmpty) {
      queryResults.clear();
      resizeHeight();
      return;
    }
    // delay clear results, otherwise windows height will shrink immediately,
    // and then the query result is received which will expand the windows height. so it will causes window flicker
    clearQueryResultsTimer.cancel();
    clearQueryResultsTimer = Timer(
      const Duration(milliseconds: 100),
      () {
        queryResults.clear();
        resizeHeight();
      },
    );

    final msg = WoxWebsocketMsg(id: const UuidV4().generate(), type: WoxWebsocketMsgTypeEnum.WOX_WEBSOCKET_MSG_TYPE_REQUEST.code, method: "Query", data: {
      "queryId": query.queryId,
      "queryType": query.queryType,
      "queryText": query.queryText,
      "querySelection": query.querySelection.toJson(),
    });
    ws.sendMessage(msg);
  }

  void _handleWebSocketMessage(event) {
    var msg = WoxWebsocketMsg.fromJson(jsonDecode(event));
    Logger.instance.info("Received message: ${msg.toJson()}");
    if (msg.method == "ToggleApp") {
      toggleApp(ShowAppParams.fromJson(msg.data));
    } else if (msg.method == "HideApp") {
      hide();
    } else if (msg.method == "ShowApp") {
      show(ShowAppParams.fromJson(msg.data));
    } else if (msg.method == "ChangeQuery") {
      var changedQuery = WoxQuery.fromJson(msg.data);
      changedQuery.queryId = const UuidV4().generate();
      onQueryChanged(changedQuery);
    } else if (msg.method == "Query") {
      var results = <WoxQueryResult>[];
      for (var item in msg.data) {
        results.add(WoxQueryResult.fromJson(item));
      }
      onReceiveQueryResults(results);
    }
  }

  void onReceiveQueryResults(List<WoxQueryResult> results) {
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
      resetActiveResult();
    }

    resizeHeight();
  }

  void resetActiveResult() {
    activeResultIndex.value = 0;

    //reset preview
    if (queryResults.isNotEmpty) {
      currentPreview.value = queryResults[activeResultIndex.value].preview;
    } else {
      currentPreview.value = WoxPreview.empty();
    }
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
  }

  void resizeHeight() {
    print(queryResults.length);
    var resultHeight = getResultHeightByCount(queryResults.length);

    print(resultHeight);
    if (resultHeight > getMaxHeight() || isShowActionPanel.value || isShowPreviewPanel.value) {
      resultHeight = getMaxHeight();
    }
    final totalHeight = queryBoxContainerHeight() + resultHeight;
    print(totalHeight);
    windowManager.setSize(Size(800, totalHeight.toDouble()));
  }

  void arrowUp() {}

  void arrowDown() {}

  void toggleActionPanel() {}

  double queryBoxContainerHeight() {
    return WoxThemeUtil.instance.getWoxBoxContainerHeight();
  }

  double baseResultItemHeight() {
    return (baseItemHeight + woxTheme.resultItemPaddingTop + woxTheme.resultItemPaddingBottom).toDouble();
  }

  double getResultHeightByCount(int count) {
    if (count == 0) {
      return 0;
    }
    return baseResultItemHeight() * (count > 10 ? 10 : count) + woxTheme.resultContainerPaddingTop + woxTheme.resultContainerPaddingBottom;
  }

  double getMaxHeight() {
    return getResultHeightByCount(10);
  }

  WoxQueryResult getQueryResultByIndex(int index) {
    return queryResults[index];
  }
}
