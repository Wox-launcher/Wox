import 'dart:async';
import 'dart:convert';

import 'package:flutter/widgets.dart';
import 'package:get/get.dart';
import 'package:logger/logger.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/websocket.dart';

import 'entity.dart';

class WoxController extends GetxController {
  final query = ChangedQuery.empty().obs;
  final queryTextFieldController = TextEditingController();
  final queryFocusNode = FocusNode();
  final queryResults = <QueryResult>[].obs;
  final actionTextFieldController = TextEditingController();
  final actionFocusNode = FocusNode();
  final actionResults = <WoxResultAction>[].obs;
  final currentPreview = WoxPreview.empty().obs;
  final activeResultIndex = 0.obs;
  final activeActionIndex = 0.obs;
  final isShowActionPanel = false.obs;
  final isShowPreviewPanel = false.obs;
  var clearQueryResultsTimer = Timer(const Duration(milliseconds: 200), () => {});
  late final WoxWebsocket ws;
  static const double maxHeight = 500;

  @override
  void onInit() {
    super.onInit();
    _setupWebSocket();
  }

  void _setupWebSocket() {
    ws = WoxWebsocket(Uri.parse("ws://localhost:${Env.serverPort}/ws"), onMessageReceived: _handleWebSocketMessage);
    ws.connect();
  }

  void _handleWebSocketMessage(event) {
    var msg = WebsocketMsg.fromJson(jsonDecode(event));
    if (msg.method == "ToggleApp") {
      toggleApp(ShowAppParams.fromJson(msg.data));
    } else if (msg.method == "HideApp") {
      hide();
    } else if (msg.method == "ShowApp") {
      show(ShowAppParams.fromJson(msg.data));
    } else if (msg.method == "ChangeQuery") {
      var changedQuery = ChangedQuery.fromJson(msg.data);
      changedQuery.queryId = const UuidV4().generate();
      onQueryChanged(changedQuery);
    } else if (msg.method == "Query") {
      var results = <QueryResult>[];
      for (var item in msg.data) {
        results.add(QueryResult.fromJson(item));
      }
      onReceiveQueryResults(results);
    }
  }

  Future<void> toggleApp(ShowAppParams params) async {
    Logger().i("Toggle app");
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
    if (params.position.type == positionTypeMouseScreen) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    }

    await windowManager.show();
    queryFocusNode.requestFocus();
  }

  Future<void> hide() async {
    await windowManager.hide();
  }

  void arrowDown() {
    if (activeResultIndex.value == queryResults.length - 1) {
      activeResultIndex.value = 0;
    } else {
      activeResultIndex.value++;
    }

    currentPreview.value = queryResults[activeResultIndex.value].preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
    queryResults.refresh();
  }

  void arrowDownAction() {
    if (activeActionIndex.value == actionResults.length - 1) {
      activeActionIndex.value = 0;
    } else {
      activeActionIndex.value++;
    }

    actionResults.refresh();
  }

  void arrowUp() {
    if (activeResultIndex.value == 0) {
      activeResultIndex.value = queryResults.length - 1;
    } else {
      activeResultIndex.value--;
    }

    currentPreview.value = queryResults[activeResultIndex.value].preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";
    queryResults.refresh();
  }

  void arrowUpAction() {
    if (activeActionIndex.value == 0) {
      activeActionIndex.value = actionResults.length - 1;
    } else {
      activeActionIndex.value--;
    }

    actionResults.refresh();
  }

  Future<void> selectResult() async {
    final defaultActionIndex = queryResults[activeResultIndex.value].actions.indexWhere((element) => element.isDefault);
    if (defaultActionIndex != -1) {
      activeActionIndex.value = defaultActionIndex;
      selectAction();
    }
  }

  void selectAction() {
    final result = queryResults[activeResultIndex.value];
    final action = result.actions[activeActionIndex.value];
    final msg = WebsocketMsg(id: const UuidV4().generate(), method: "Action", data: {
      "resultId": result.id,
      "actionId": action.id,
    });
    ws.sendMessage(msg);

    if (!action.preventHideAfterAction) {
      hide();
    }
  }

  void toggleActionPanel() {
    if (queryResults.isEmpty) {
      return;
    }

    if (isShowActionPanel.value) {
      isShowActionPanel.value = false;
      queryFocusNode.requestFocus();
    } else {
      isShowActionPanel.value = true;
      actionFocusNode.requestFocus();
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

  void selectAll() {
    queryTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryTextFieldController.text.length);
  }

  void onQueryChanged(ChangedQuery query) {
    this.query.value = query;
    isShowActionPanel.value = false;
    if (query.queryType == queryTypeInput) {
      queryTextFieldController.text = query.queryText;
    } else {
      queryTextFieldController.text = query.toString();
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
        Logger().i("clear results");
        queryResults.clear();
        resizeHeight();
      },
    );

    final msg = WebsocketMsg(id: const UuidV4().generate(), method: "Query", data: {
      "queryId": query.queryId,
      "queryType": query.queryType,
      "queryText": query.queryText,
      "querySelection": query.querySelection.toJson(),
    });
    ws.sendMessage(msg);
  }

  void onReceiveQueryResults(List<QueryResult> results) {
    if (results.isEmpty || query.value.queryId != results.first.queryId) {
      return;
    }

    //cancel clear results timer
    clearQueryResultsTimer.cancel();

    //merge and sort results
    final currentQueryResults = queryResults.where((item) => item.queryId == query.value.queryId).toList();
    final finalResults = List<QueryResult>.from(currentQueryResults)..addAll(results);
    finalResults.sort((a, b) => b.score.compareTo(a.score));
    queryResults.assignAll(finalResults);

    //reset active result and preview
    if (currentQueryResults.isEmpty) {
      resetActiveResult();
    }

    resizeHeight();
  }

  void onActionQueryChanged(String actionQuery) {
    if (actionQuery.isEmpty) {
      actionResults.value = queryResults[activeResultIndex.value].actions;
      return;
    }

    final results = <WoxResultAction>[];
    for (var action in queryResults[activeResultIndex.value].actions) {
      if (action.name.toLowerCase().contains(actionQuery.toLowerCase())) {
        results.add(action);
      }
    }
    actionResults.value = results;
    actionResults.refresh();
  }

  void resizeHeight() {
    const queryBoxHeight = 48;
    const resultItemHeight = 40;
    var resultHeight = queryResults.length * resultItemHeight;
    if (resultHeight > maxHeight || isShowActionPanel.value || isShowPreviewPanel.value) {
      resultHeight = maxHeight.toInt();
    }
    final totalHeight = queryBoxHeight + resultHeight;
    windowManager.setSize(Size(800, totalHeight.toDouble()));
  }
}
