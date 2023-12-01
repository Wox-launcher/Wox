import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:window_manager/window_manager.dart';

class WoxLauncherController extends GetxController with GetSingleTickerProviderStateMixin {
  final queryBoxFocusNode = FocusNode();
  final queryBoxTextFieldController = TextEditingController();

  Future<void> hide() async {
    await windowManager.blur();
    await windowManager.hide();
  }

  Future<void> selectResult() async {}

  void arrowUp() {}

  void arrowDown() {}

  void toggleActionPanel() {}
}
