import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entity/wox_query_result.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxLauncherController extends GetxController with GetSingleTickerProviderStateMixin {
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();
  final WoxTheme woxTheme = WoxThemeUtil.instance.currentTheme.obs();
  final int baseItemHeight = 50;
  final activeResultIndex = 0.obs;
  final isShowActionPanel = false.obs;
  final isShowPreviewPanel = false.obs;
  final queryResults = <WoxQueryResult>[].obs;

  Future<void> hide() async {
    await windowManager.blur();
    await windowManager.hide();
  }

  Future<void> selectResult() async {}

  WoxQueryResult getQueryResultByIndex(int index) {
    return queryResults[index];
  }

  void arrowUp() {}

  void arrowDown() {}

  void toggleActionPanel() {}

  double baseResultItemHeight() {
    return (baseItemHeight * woxTheme.resultItemPaddingTop + woxTheme.resultItemPaddingBottom).toDouble();
  }

  double getMaxHeight() {
    return baseResultItemHeight() * 10 + woxTheme.resultContainerPaddingTop + woxTheme.resultContainerPaddingBottom;
  }
}
