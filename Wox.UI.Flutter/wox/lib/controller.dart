import 'dart:async';
import 'dart:convert';

import 'package:flutter/widgets.dart';
import 'package:get/get.dart';
import 'package:logger/logger.dart';
import 'package:uuid/v4.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:window_manager/window_manager.dart';

import 'entity.dart';

class WoxController extends GetxController {
  final query = ChangedQuery.empty().obs;
  final queryTextFieldController = TextEditingController();
  final queryResults = <QueryResult>[].obs;
  final currentPreview = WoxPreview.empty().obs;
  final activeResultIndex = 0.obs;
  final isShowActionList = false.obs;
  var onQueryChangeTimer = Timer(const Duration(milliseconds: 200), () => {});
  var hasLastQueryResult = false;

  static const double maxHeight = 500;

  late final WebSocketChannel channel;

  void connect() {
    channel = WebSocketChannel.connect(Uri.parse("ws://localhost:34987/ws"));
    channel.stream.listen((event) {
      var msg = WebsocketMsg.fromJson(jsonDecode(event));
      if (msg.method == "ToggleApp") {
        toggleApp(ShowAppParams.fromJson(msg.data));
      } else if (msg.method == "HideApp") {
        hide();
      } else if (msg.method == "ShowApp") {
        hide();
      } else if (msg.method == "ChangeQuery") {
        onQueryChanged(ChangedQuery.fromJson(msg.data));
      } else if (msg.method == "Query") {
        var results = <QueryResult>[];
        for (var item in msg.data) {
          results.add(QueryResult.fromJson(item));
        }
        onReceiveQueryResults(results);
      }
    });
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

  Future<void> hide() async {
    await windowManager.blur();
    await windowManager.hide();
  }

  void arrowDown() {
    if (activeResultIndex.value == queryResults.length - 1) {
      activeResultIndex.value = 0;
    } else {
      activeResultIndex.value++;
    }

    currentPreview.value = queryResults[activeResultIndex.value].preview;
    queryResults.refresh();
  }

  void arrowUp() {
    if (activeResultIndex.value == 0) {
      activeResultIndex.value = queryResults.length - 1;
    } else {
      activeResultIndex.value--;
    }

    currentPreview.value = queryResults[activeResultIndex.value].preview;
    queryResults.refresh();
  }

  Future<void> selectResult() async {
    final result = queryResults[activeResultIndex.value];
    final action = result.actions.first;
    final msg = WebsocketMsg(id: const UuidV4().generate(), method: "Action", data: {
      "resultId": result.id,
      "actionId": action.id,
    });
    channel.sink.add(jsonEncode(msg));

    if (!action.preventHideAfterAction) {
      await hide();
    }
  }

  void toggleActionList() {
    isShowActionList.value = !isShowActionList.value;
    resizeHeight();
  }

  void resetActiveResultIndex() {
    activeResultIndex.value = 0;
    if (queryResults.isNotEmpty) {
      currentPreview.value = queryResults[activeResultIndex.value].preview;
    } else {
      currentPreview.value = WoxPreview.empty();
    }
  }

  void selectAll() {
    queryTextFieldController.selection = TextSelection(baseOffset: 0, extentOffset: queryTextFieldController.text.length);
  }

  Future<void> show(ShowAppParams params) async {
    if (params.selectAll) {
      selectAll();
    }
    if (params.position.type == positionTypeMouseScreen) {
      await windowManager.setPosition(Offset(params.position.x.toDouble(), params.position.y.toDouble()));
    }

    await windowManager.show();
    await windowManager.focus();
  }

  void onQueryChanged(ChangedQuery query) {
    resetActiveResultIndex();
    this.query.value = query;
    isShowActionList.value = false;
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
    hasLastQueryResult = false;
    onQueryChangeTimer.cancel();

    onQueryChangeTimer = Timer(
      const Duration(milliseconds: 200),
      () {
        if (!hasLastQueryResult) {
          Logger().i("clear results");
          queryResults.clear();
          resizeHeight();
        }
      },
    );

    final msg = WebsocketMsg(id: const UuidV4().generate(), method: "Query", data: {
      "queryId": query.queryId,
      "queryType": query.queryType,
      "queryText": query.queryText,
      "querySelection": query.querySelection.toJson(),
    });
    channel.sink.add(jsonEncode(msg));
  }

  void onReceiveQueryResults(List<QueryResult> results) {
    if (results.isEmpty) {
      return;
    }
    //not current query result
    if (query.value.queryId != results.first.queryId) {
      return;
    }

    hasLastQueryResult = true;

    final finalResults = <QueryResult>[];
    queryResults.where((i) => i.queryId == query.value.queryId).forEach((element) {
      finalResults.remove(element);
    });
    finalResults.addAll(results);

    //sort by score desc
    finalResults.sort((a, b) => b.score.compareTo(a.score));

    queryResults.assignAll(finalResults);

    resizeHeight();
  }

  void resizeHeight() {
    //based on current query result count
    const queryBoxHeight = 48;
    const resultItemHeight = 40;
    var resultHeight = queryResults.length * resultItemHeight;
    if (resultHeight > maxHeight || isShowActionList.value) {
      resultHeight = maxHeight.toInt();
    }
    final totalHeight = queryBoxHeight + resultHeight;
    windowManager.setSize(Size(800, totalHeight.toDouble()));
  }
}
