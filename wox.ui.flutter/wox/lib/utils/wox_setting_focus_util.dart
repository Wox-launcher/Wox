import 'package:flutter/widgets.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';

class WoxSettingFocusUtil {
  static void restoreIfInSettingView() {
    if (!Get.isRegistered<WoxLauncherController>() || !Get.isRegistered<WoxSettingController>()) {
      return;
    }

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!Get.isRegistered<WoxLauncherController>() || !Get.isRegistered<WoxSettingController>()) {
        return;
      }

      final launcherController = Get.find<WoxLauncherController>();
      if (!launcherController.isInSettingView.value) {
        return;
      }

      final settingController = Get.find<WoxSettingController>();
      if (!settingController.settingFocusNode.canRequestFocus) {
        return;
      }

      settingController.settingFocusNode.requestFocus();
    });
  }
}
