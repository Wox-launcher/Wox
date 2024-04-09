import 'dart:ui';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

abstract class WoxSettingPluginItem extends StatelessWidget {
  final Map<String, String> settings;
  final Function onUpdate;

  const WoxSettingPluginItem(this.settings, this.onUpdate, {super.key});

  void updateConfig(String key, String value) {
    Get.find<WoxSettingController>().updateConfig(key, value);
  }

  String getSetting(String key) {
    return settings[key] ?? "";
  }
}
