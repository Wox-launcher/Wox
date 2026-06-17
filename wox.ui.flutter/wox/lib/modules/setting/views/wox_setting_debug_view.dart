import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

const _cloudSyncProductionUrl = "https://sync.woxlauncher.com";
const _cloudSyncLocalUrl = "http://127.0.0.1:8787";

class WoxSettingDebugView extends WoxSettingBaseView {
  const WoxSettingDebugView({super.key});

  @override
  Widget build(BuildContext context) {
    return form(
      title: controller.tr("ui_debug"),
      description: controller.tr("ui_debug_description"),
      children: [
        formField(
          settingKey: "CloudSyncServerUrl",
          label: controller.tr("ui_cloud_sync_server_url"),
          tips: controller.tr("ui_cloud_sync_server_url_tips"),
          child: Obx(() {
            final configuredUrl = controller.woxSetting.value.cloudSyncServerUrl.trim();
            final selectedUrl = configuredUrl == _cloudSyncLocalUrl ? _cloudSyncLocalUrl : _cloudSyncProductionUrl;
            return WoxDropdownButton<String>(
              width: 360,
              value: selectedUrl,
              items: [
                WoxDropdownItem(value: _cloudSyncProductionUrl, label: controller.tr("ui_cloud_sync_server_url_production"), subtitle: _cloudSyncProductionUrl),
                WoxDropdownItem(value: _cloudSyncLocalUrl, label: controller.tr("ui_cloud_sync_server_url_local"), subtitle: _cloudSyncLocalUrl),
              ],
              onChanged: (url) async {
                if (url == null) {
                  return;
                }
                await controller.updateCloudSyncServerUrl(url);
              },
            );
          }),
        ),
        formField(
          settingKey: "ShowScoreTail",
          label: controller.tr("ui_debug_show_score_tail"),
          tips: controller.tr("ui_debug_show_score_tail_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.showScoreTail,
              onChanged: (bool value) {
                // New debug setting: score tails are useful when tuning ranking,
                // but keeping the switch here avoids editing backend call sites
                // whenever a developer needs to inspect scores.
                controller.updateConfig("ShowScoreTail", value.toString());
              },
            );
          }),
        ),
        formField(
          settingKey: "ShowPerformanceTail",
          label: controller.tr("ui_debug_show_performance_tail"),
          tips: controller.tr("ui_debug_show_performance_tail_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTail,
              onChanged: (bool value) {
                // New debug setting: query timing tails were previously forced
                // in dev; the persisted toggle keeps performance inspection
                // available while letting developers turn off noisy tags.
                controller.updateConfig("ShowPerformanceTail", value.toString());
              },
            );
          }),
        ),
        formField(
          settingKey: "ShowPerformanceTailBatch",
          label: controller.tr("ui_debug_show_performance_tail_batch"),
          tips: controller.tr("ui_debug_show_performance_tail_batch_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailBatch,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailBatch", value.toString());
                      }
                      : null,
            );
          }),
        ),
        formField(
          settingKey: "ShowPerformanceTailPluginQuery",
          label: controller.tr("ui_debug_show_performance_tail_plugin_query"),
          tips: controller.tr("ui_debug_show_performance_tail_plugin_query_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailPluginQuery,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailPluginQuery", value.toString());
                      }
                      : null,
            );
          }),
        ),
        formField(
          settingKey: "ShowPerformanceTailBackendPrepared",
          label: controller.tr("ui_debug_show_performance_tail_backend_prepared"),
          tips: controller.tr("ui_debug_show_performance_tail_backend_prepared_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailBackendPrepared,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailBackendPrepared", value.toString());
                      }
                      : null,
            );
          }),
        ),
        formField(
          settingKey: "ShowPerformanceTailUiReceived",
          label: controller.tr("ui_debug_show_performance_tail_ui_received"),
          tips: controller.tr("ui_debug_show_performance_tail_ui_received_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTailUiReceived,
              onChanged:
                  controller.woxSetting.value.showPerformanceTail
                      ? (bool value) {
                        controller.updateConfig("ShowPerformanceTailUiReceived", value.toString());
                      }
                      : null,
            );
          }),
        ),
      ],
    );
  }
}
