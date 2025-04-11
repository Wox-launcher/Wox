import 'dart:async';
import 'dart:io';
import 'dart:convert';

import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:get/get.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:lpinyin/lpinyin.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controllers/wox_list_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/enums/wox_actions_type_enum.dart';
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
import 'package:fuzzywuzzy/fuzzywuzzy.dart';
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

  /// The timer to clear query results.
  /// On every query changed, it will reset the timer and will clear the query results after N ms.
  /// If there is no this delay mechanism, the window will flicker for fast typing.
  Timer clearQueryResultsTimer = Timer(const Duration(), () => {});
  final clearQueryResultDelay = 100;

  // action related variables
  /// The list of result actions for the active query result.
  final actions = <WoxResultAction>[].obs;
  final originalActions = <WoxResultAction>[]; // the original actions, used to filter and restore selection actions
  final activeActionIndex = 0.obs;
  final isShowActionPanel = false.obs;
  final actionTextFieldController = TextEditingController();
  final actionScrollerController = ScrollController(initialScrollOffset: 0.0);
  Function(String) actionEscFunction = (String traceId) => {}; // the function to handle the esc key in the action panel
  final actionsTitle = "Actions".obs;
  var actionsType = WoxActionsTypeEnum.WOX_ACTIONS_TYPE_RESULT.code;
  final actionListViewController = Get.put(WoxListController<WoxResultAction>(), tag: 'action');

  // ai chat related variables
  final aiChatFocusNode = FocusNode();
  final List<AIModel> aiModels = [];
  final ScrollController aiChatScrollController = ScrollController();

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
        onItemExecuted: (item) {
          onEnter(const UuidV4().generate());
        },
        onItemActive: (item) {
          onResultItemActivated(const UuidV4().generate(), item);
        },
      ),
      tag: 'result',
    );

    actionEscFunction = (String traceId) {
      hideActionPanel(traceId);
    };

    reloadAIModels();
  }

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
    final existingQueryResults = resultListViewController.items.where((item) => item.data.queryId == currentQuery.value.queryId).toList();
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
      resetActionsByActiveResult(traceId, "receive query results: ${currentQuery.value.queryText}");
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

    WoxApi.instance.onShow();
  }

  void reloadAIModels() {
    WoxApi.instance.findAIModels().then((models) {
      aiModels.assignAll(models);
      Logger.instance.debug(const UuidV4().generate(), "reload ai models: ${aiModels.length}");
    });
  }

  Future<void> hideApp(String traceId) async {
    //clear query box text if query type is selection or last query mode is empty
    if (currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code || lastQueryMode == WoxLastQueryModeEnum.WOX_LAST_QUERY_MODE_EMPTY.code) {
      currentQuery.value = PlainQuery.emptyInput();
      queryBoxTextFieldController.clear();
      actionTextFieldController.clear();
      await clearQueryResults();
    }

    // switch to the launcher view if in setting view
    if (isInSettingView.value) {
      isInSettingView.value = false;
    }

    hideActionPanel(traceId);

    // must invoke after clearQueryResults, because clearQueryResults will trigger resizeHeight, which will cause the focus can't return to the other window in windows
    // Also,  on windows, the hide method will cause onKeyEvent state inconsistent (the keyup event will be suppressed until the next show, so next time you press the esc key,
    // the previous keyup event will be triggered, which in our case will not trigger the hide app action. you may need to press the esc key twice),
    // so we need to delay the hide method on windows. if we someday find a better solution, we can remove this delay
    if (Platform.isWindows) {
      Future.delayed(const Duration(milliseconds: 50), () {
        windowManager.hide();
      });
    } else {
      await windowManager.hide();
    }

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
    actionTextFieldController.text = "";
    focusQueryBox(selectAll: true);
    resetActionsByActiveResult(traceId, "hide action panel");
    resizeHeight();
  }

  void focusQueryBox({bool selectAll = false}) {
    // Store current cursor position before changing focus
    final currentPosition = queryBoxTextFieldController.selection.baseOffset;

    // request focus to action query box since it will lose focus when tap
    queryBoxFocusNode.requestFocus();

    // by default requestFocus will select all text, if selectAll is false, then restore to the previously stored cursor position
    if (!selectAll) {
      SchedulerBinding.instance.addPostFrameCallback((_) {
        // Restore to the previously stored cursor position
        final currentText = queryBoxTextFieldController.text;
        queryBoxTextFieldController.value = TextEditingValue(
          text: currentText,
          selection: TextSelection.collapsed(offset: currentPosition),
        );
      });
    }
  }

  void showActionPanel(String traceId) {
    isShowActionPanel.value = true;
    SchedulerBinding.instance.addPostFrameCallback((_) {
      actionListViewController.requestFocus();
    });
    resizeHeight();
  }

  Future<void> showActionPanelForModelSelection(String traceId, WoxPreviewChatData aiChatData) async {
    originalActions.assignAll(aiModels.map((model) => WoxResultAction(
          id: const UuidV4().generate(),
          name: "${model.name} (${model.provider})",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
          isDefault: false,
          preventHideAfterAction: true,
          hotkey: "",
          isSystemAction: false,
          onExecute: (traceId) {
            aiChatData.model = WoxPreviewChatModel(name: model.name, provider: model.provider);

            for (var result in resultListViewController.items) {
              if (result.data.contextData == aiChatData.id) {
                result.data.preview = WoxPreview(
                  previewType: WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code,
                  previewData: jsonEncode(aiChatData.toJson()),
                  previewProperties: {},
                  scrollPosition: WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code,
                );
                currentPreview.value = result.data.preview;
              }
            }

            hideActionPanel(traceId);
            focusToChatInput(traceId);
          },
        )));
    actionsType = WoxActionsTypeEnum.WOX_ACTIONS_TYPE_AI_MODEL.code;
    actionsTitle.value = "Models";
    actions.assignAll(originalActions);
    actionEscFunction = (String traceId) {
      hideActionPanel(traceId);
      focusToChatInput(traceId);

      // reset the actionEscFunction to the default
      actionEscFunction = (String traceId) {
        hideActionPanel(traceId);
      };
    };
    showActionPanel(traceId);
  }

  void scrollToBottomOfAiChat() {
    if (aiChatScrollController.hasClients) {
      aiChatScrollController.jumpTo(aiChatScrollController.position.maxScrollExtent);
    }
  }

  WoxQueryResult? getActiveResult() {
    if (resultListViewController.activeIndex.value >= resultListViewController.items.length ||
        resultListViewController.activeIndex.value < 0 ||
        resultListViewController.items.isEmpty) {
      return null;
    }

    return resultListViewController.items[resultListViewController.activeIndex.value].data;
  }

  WoxResultAction? getActiveAction() {
    if (actions.isEmpty || resultListViewController.activeIndex.value >= actions.length || resultListViewController.activeIndex.value < 0) {
      return null;
    }

    return actions[resultListViewController.activeIndex.value];
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

  Future<void> onEnter(String traceId) async {
    toolbar.value.action?.call();
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

    if (action.onExecute != null) {
      action.onExecute!(traceId);
    } else {
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
    }

    // clear the search text after action is executed
    actionTextFieldController.text = "";

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
      filterSelectionResults(const UuidV4().generate(), value);
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

  void filterSelectionResults(String traceId, String filterText) {
    resultListViewController.filterItems(traceId, filterText);
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
      clearQueryResults();
      return;
    }

    // delay clear results, otherwise windows height will shrink immediately,
    // and then the query result is received which will expand the windows height. so it will causes window flicker
    clearQueryResultsTimer.cancel();
    clearQueryResultsTimer = Timer(
      Duration(milliseconds: clearQueryResultDelay),
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

  /// check if the query text is fuzzy match with the filter text based on the setting
  bool isFuzzyMatch(String traceId, String queryText, String filterText) {
    if (WoxSettingUtil.instance.currentSetting.usePinYin) {
      queryText = transferChineseToPinYin(queryText).toLowerCase();
    } else {
      queryText = queryText.toLowerCase();
    }

    var score = weightedRatio(queryText, filterText.toLowerCase());
    Logger.instance.debug(traceId, "calculate fuzzy match score, queryText: $queryText, filterText: $filterText, score: $score");
    return score > 50;
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
      handleChatResponse(msg.traceId, WoxPreviewChatData.fromJson(msg.data));
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

  Future<void> clearQueryResults() async {
    resultListViewController.clearItems();
    actions.clear();
    toolbar.value = ToolbarInfo.empty();
    isShowPreviewPanel.value = false;
    isShowActionPanel.value = false;

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
      if (resultListViewController.items.first.data.isGroup) {
        resultListViewController.updateActiveIndexByDirection(const UuidV4().generate(), WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
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
    final totalHeight = WoxThemeUtil.instance.getQueryBoxHeight() + resultHeight;

    if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.debug(const UuidV4().generate(), "Resize: window height to $totalHeight");
    await windowManager.setSize(Size(WoxSettingUtil.instance.currentSetting.appWidth.toDouble(), totalHeight.toDouble()));
  }

  void clearHoveredResult() {
    resultListViewController.clearHoveredResult();
  }

  /// update active actions based on active result and reset active action index to 0
  void resetActionsByActiveResult(String traceId, String reason, {bool remainIndex = false}) {
    var activeQueryResult = getActiveResult();
    if (activeQueryResult == null || activeQueryResult.actions.isEmpty) {
      Logger.instance.info(traceId, "update active actions, reason: $reason, current active result: null");
      activeActionIndex.value = -1;
      actions.clear();
      return;
    }

    actionsType = WoxActionsTypeEnum.WOX_ACTIONS_TYPE_RESULT.code;
    actionsTitle.value = "Actions";
    originalActions.assignAll(activeQueryResult.actions);

    Logger.instance
        .info(traceId, "update active actions, reason: $reason, current active result: ${activeQueryResult.title}, active action: ${activeQueryResult.actions.first.name}");

    String? previousActionName;
    if (remainIndex && actions.isNotEmpty && activeActionIndex.value >= 0 && activeActionIndex.value < actions.length) {
      previousActionName = actions[activeActionIndex.value].name;
    }

    final filterText = actionTextFieldController.text;
    List<WoxResultAction> newActions;
    if (filterText.isNotEmpty) {
      newActions = originalActions.where((element) {
        return isFuzzyMatch(traceId, element.name, filterText);
      }).toList();
      activeActionIndex.value = newActions.isEmpty ? -1 : 0;
      remainIndex = false;
    } else {
      newActions = List.from(originalActions);
    }

    // Only update actions if they have actually changed
    if (!WoxResultAction.listEquals(actions, newActions)) {
      actions.assignAll(newActions);
    }

    // remain the same action index
    if (remainIndex && previousActionName != null) {
      final newIndex = actions.indexWhere((action) => action.name == previousActionName);
      if (newIndex != -1) {
        activeActionIndex.value = newIndex;
      } else {
        activeActionIndex.value = 0;
      }
    } else {
      activeActionIndex.value = 0;
    }

    updateToolbarByActiveAction(traceId);
  }

  updateResult(String traceId, UpdateableResult updateableResult) {
    final result = resultListViewController.items.firstWhere((element) => element.data.id == updateableResult.id);
    var needUpdate = false;
    var updatedResult = result.copyWith();

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
        if (result.data.refreshInterval > 0 && refreshCounter % result.data.refreshInterval == 0) {
          if (isRequesting.containsKey(result.data.id)) {
            continue;
          } else {
            isRequesting[result.data.id] = true;
          }

          final traceId = const UuidV4().generate();
          final msg = WoxWebsocketMsg(
            requestId: const UuidV4().generate(),
            traceId: traceId,
            type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
            method: WoxMsgMethodEnum.WOX_MSG_METHOD_REFRESH.code,
            data: {
              "queryId": result.data.queryId,
              "refreshableResult": WoxRefreshableResult(
                resultId: result.data.id,
                title: result.data.title,
                subTitle: result.data.subTitle,
                icon: result.data.icon,
                preview: result.data.preview,
                tails: result.data.tails,
                contextData: result.data.contextData,
                refreshInterval: result.data.refreshInterval,
                actions: result.data.actions,
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
            if (!resultListViewController.items.any((element) => element.data.id == result.data.id)) {
              isRequesting.remove(result.data.id);
              Logger.instance
                  .info(traceId, "result <${result.data.title}> (resultId: ${result.data.id}) is removed (maybe caused by new query) during refresh, skip update result");
              return;
            }

            final refreshResult = WoxRefreshableResult.fromJson(resp);
            final resultIndex = resultListViewController.items.indexWhere((element) => element.data.id == result.data.id);
            resultListViewController.items[resultIndex].data.title = refreshResult.title;
            resultListViewController.items[resultIndex].data.subTitle = refreshResult.subTitle;
            resultListViewController.items[resultIndex].data.icon = refreshResult.icon;
            resultListViewController.items[resultIndex].data.preview = refreshResult.preview;
            resultListViewController.items[resultIndex].data.tails.assignAll(refreshResult.tails);
            resultListViewController.items[resultIndex].data.actions.assignAll(refreshResult.actions);

            // only update preview and toolbar when current result is active
            if (resultListViewController.isItemActive(result.data.id)) {
              currentPreview.value = result.data.preview;
              final oldShowPreview = isShowPreviewPanel.value;
              isShowPreviewPanel.value = currentPreview.value.previewData != "";
              if (oldShowPreview != isShowPreviewPanel.value) {
                Logger.instance.debug(traceId, "preview panel visibility changed, resize height");
                resizeHeight();
              }

              // only reset actions when action type is result
              if (actionsType == WoxActionsTypeEnum.WOX_ACTIONS_TYPE_RESULT.code) {
                resetActionsByActiveResult(traceId, "refresh active result", remainIndex: true);
              }
            }

            result.data.contextData = refreshResult.contextData;
            result.data.refreshInterval = refreshResult.refreshInterval;

            // update result list view item
            resultListViewController.updateItem(
                traceId,
                WoxListItem(
                  id: result.data.id,
                  icon: result.data.icon,
                  title: result.data.title,
                  tails: result.data.tails,
                  subTitle: result.data.subTitle,
                  isGroup: result.data.isGroup,
                  data: result.data,
                ));

            isRequesting.remove(result.data.id);
          });
        }
      }
    });
  }

  startDoctorCheckSchedule() {
    Timer.periodic(const Duration(minutes: 1), (timer) async {
      doctorCheckPassed = await WoxApi.instance.doctorCheck();
      Logger.instance.debug(const UuidV4().generate(), "doctor check result: $doctorCheckPassed");
    });
  }

  @override
  void dispose() {
    queryBoxFocusNode.dispose();
    queryBoxTextFieldController.dispose();
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

  void focusToChatInput(String traceId) {
    Logger.instance.info(traceId, "focus to chat input");
    actionsType = WoxActionsTypeEnum.WOX_ACTIONS_TYPE_AI_MODEL.code;
    SchedulerBinding.instance.addPostFrameCallback((_) {
      aiChatFocusNode.requestFocus();
    });
  }

  Future<void> sendChatRequest(String traceId, WoxPreviewChatData data) async {
    WoxApi.instance.sendChatRequest(data);
  }

  void handleChatResponse(String traceId, WoxPreviewChatData data) {
    for (var result in resultListViewController.items) {
      if (result.data.contextData == data.id) {
        result.data.preview = WoxPreview(
          previewType: WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code,
          previewData: jsonEncode(data.toJson()),
          previewProperties: {},
          scrollPosition: WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code,
        );
        currentPreview.value = result.data.preview;
        SchedulerBinding.instance.addPostFrameCallback((_) {
          scrollToBottomOfAiChat();
        });
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

  void onResultItemActivated(String traceId, WoxListItem item) {
    currentPreview.value = item.data.preview;
    isShowPreviewPanel.value = currentPreview.value.previewData != "";

    resetActionsByActiveResult(traceId, "update active result index");
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
    focusQueryBox(selectAll: false);

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
    //if doctor check is not passed and query is empty, show doctor icon
    if (query.isEmpty && !doctorCheckPassed) {
      queryIcon.value = QueryIconInfo(
        icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: QUERY_ICON_DOCTOR_WARNING),
        action: () {
          onQueryChanged(traceId, PlainQuery.text("doctor "), "user click query icon");
        },
      );
      return;
    }

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

    // if query is empty, we need to immediately update the toolbar
    if (query.isEmpty) {
      // if query is empty and doctor check is not passed, show doctor warning
      if (!doctorCheckPassed) {
        Logger.instance.debug(traceId, "update toolbar to doctor warning, query is empty and doctor check not passed");
        toolbar.value = ToolbarInfo(
          text: "Doctor check not passed",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: QUERY_ICON_DOCTOR_WARNING),
          hotkey: "enter",
          actionName: "Check",
          action: () {
            onQueryChanged(traceId, PlainQuery.text("doctor "), "user click query icon");
          },
        );
      } else {
        Logger.instance.debug(traceId, "update toolbar to empty because of query changed and is empty");
        toolbar.value = ToolbarInfo.empty();
      }
      return;
    }

    // if query is not empty, update the toolbar after 100ms to avoid flickering
    cleanToolbarTimer = Timer(Duration(milliseconds: cleanToolbarDelay), () {
      Logger.instance.debug(traceId, "update toolbar to empty because of query changed");
      toolbar.value = ToolbarInfo.empty();
    });
  }

  /// Update the toolbar based on the active action
  void updateToolbarByActiveAction(String traceId) {
    var activeAction = getActiveAction();
    if (activeAction != null) {
      Logger.instance.debug(traceId, "update toolbar to active action: ${activeAction.name}");

      // cancel the timer if it is running
      cleanToolbarTimer.cancel();

      // only update action and hotkey if it's different from the current one
      if (toolbar.value.actionName != activeAction.name || toolbar.value.hotkey != activeAction.hotkey) {
        toolbar.value = ToolbarInfo(
          hotkey: "enter",
          actionName: activeAction.name,
          action: () {
            executeAction(traceId, getActiveResult(), activeAction);
          },
        );
      }
    } else {
      Logger.instance.debug(traceId, "update toolbar to empty, no active action");
      toolbar.value = ToolbarInfo.empty();
    }
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
