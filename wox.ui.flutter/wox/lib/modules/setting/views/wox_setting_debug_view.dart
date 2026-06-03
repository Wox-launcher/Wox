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
          formField(
            settingKey: "ShowPerformanceTailBatch",
            label: controller.tr("ui_debug_show_performance_tail_batch"),
            tips: controller.tr("ui_debug_show_performance_tail_batch_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailBatch,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailBatch", value.toString());
                      }
                      : null,
            ),
          ),
          formField(
            settingKey: "ShowPerformanceTailPluginQuery",
            label: controller.tr("ui_debug_show_performance_tail_plugin_query"),
            tips: controller.tr("ui_debug_show_performance_tail_plugin_query_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailPluginQuery,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailPluginQuery", value.toString());
                      }
                      : null,
            ),
          ),
          formField(
            settingKey: "ShowPerformanceTailBackendPrepared",
            label: controller.tr("ui_debug_show_performance_tail_backend_prepared"),
            tips: controller.tr("ui_debug_show_performance_tail_backend_prepared_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailBackendPrepared,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailBackendPrepared", value.toString());
                      }
                      : null,
            ),
          ),
          formField(
            settingKey: "ShowPerformanceTailUiReceived",
            label: controller.tr("ui_debug_show_performance_tail_ui_received"),
            tips: controller.tr("ui_debug_show_performance_tail_ui_received_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailUiReceived,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailUiReceived", value.toString());
                      }
                      : null,
            ),
          ),
        ],
      );
    });
  }
}
