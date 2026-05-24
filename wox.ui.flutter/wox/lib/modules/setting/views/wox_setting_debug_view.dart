import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

class WoxSettingDebugView extends WoxSettingBaseView {
  const WoxSettingDebugView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return form(
        title: controller.tr("ui_debug"),
        description: controller.tr("ui_debug_description"),
        children: [
          formField(
            settingKey: "ShowScoreTail",
            label: controller.tr("ui_debug_show_score_tail"),
            tips: controller.tr("ui_debug_show_score_tail_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showScoreTail,
              onChanged: (bool value) {
                // New debug setting: score tails are useful when tuning ranking,
                // but keeping the switch here avoids editing backend call sites
                // whenever a developer needs to inspect scores.
                controller.updateConfig("ShowScoreTail", value.toString());
              },
            ),
          ),
          formField(
            settingKey: "ShowPerformanceTail",
            label: controller.tr("ui_debug_show_performance_tail"),
            tips: controller.tr("ui_debug_show_performance_tail_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTail,
              onChanged: (bool value) {
                // New debug setting: query timing tails were previously forced
                // in dev; the persisted toggle keeps performance inspection
                // available while letting developers turn off noisy tags.
                controller.updateConfig("ShowPerformanceTail", value.toString());
              },
            ),
          ),
        ],
      );
    });
  }
}
