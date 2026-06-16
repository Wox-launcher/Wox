import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingDebugView extends WoxSettingBaseView {
  const WoxSettingDebugView({super.key});

  Future<String?> showCloudSyncServerUrlDialog(BuildContext context) async {
    final urlController = TextEditingController(text: controller.woxSetting.value.cloudSyncServerUrl);
    try {
      return await showDialog<String>(
        context: context,
        barrierColor: getThemePopupBarrierColor(),
        builder: (context) {
          return WoxDialog(
            title: Text(controller.tr("ui_cloud_sync_server_url")),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                WoxTextField(controller: urlController, hintText: "http://127.0.0.1:8787", width: 420),
                const SizedBox(height: 8),
                Text(controller.tr("ui_cloud_sync_server_url_tips"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
              ],
            ),
            actions: [
              WoxButton.secondary(text: controller.tr("ui_cloud_sync_cancel"), onPressed: () => Navigator.pop(context)),
              WoxButton.primary(text: controller.tr("ui_cloud_sync_confirm"), onPressed: () => Navigator.pop(context, urlController.text.trim())),
            ],
          );
        },
      );
    } finally {
      urlController.dispose();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final configuredUrl = controller.woxSetting.value.cloudSyncServerUrl.trim();
      final displayUrl = configuredUrl.isEmpty ? "https://sync.woxlauncher.com" : configuredUrl;
      return form(
        title: controller.tr("ui_debug"),
        description: controller.tr("ui_debug_description"),
        children: [
          formField(
            settingKey: "CloudSyncServerUrl",
            label: controller.tr("ui_cloud_sync_server_url"),
            tips: controller.tr("ui_cloud_sync_server_url_tips"),
            child: Wrap(
              spacing: 8,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(displayUrl, style: TextStyle(color: getThemeTextColor(), fontSize: 13)),
                WoxButton.secondary(
                  text: controller.tr("ui_cloud_sync_server_url_update"),
                  onPressed: () async {
                    final url = await showCloudSyncServerUrlDialog(context);
                    if (url == null) {
                      return;
                    }
                    await controller.updateCloudSyncServerUrl(url);
                  },
                ),
              ],
            ),
          ),
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
