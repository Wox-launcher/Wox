import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_checkbox_tile.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingCloudSyncView extends WoxSettingBaseView {
  const WoxSettingCloudSyncView({super.key});

  Widget buildCloudSyncInfoRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 140,
            child: Text(
              label,
              style: TextStyle(color: getThemeSubTextColor(), fontSize: 12),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: TextStyle(color: getThemeTextColor(), fontSize: 12),
            ),
          ),
        ],
      ),
    );
  }

  String formatCloudSyncTime(int timestamp) {
    if (timestamp <= 0) {
      return controller.tr("ui_cloud_sync_never");
    }
    final date = DateTime.fromMillisecondsSinceEpoch(timestamp);
    return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')} '
        '${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}:${date.second.toString().padLeft(2, '0')}';
  }

  String normalizeCloudSyncError(String error) {
    if (error.contains('cloud sync is not configured')) {
      return controller.tr("ui_cloud_sync_not_configured");
    }
    return error;
  }

  Future<void> showRecoveryCodeDialog(BuildContext context, String code) async {
    await showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          title: Text(controller.tr("ui_cloud_sync_recovery_code_title")),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              SelectableText(code, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600)),
              const SizedBox(height: 10),
              Text(
                controller.tr("ui_cloud_sync_recovery_code_tips"),
                style: TextStyle(color: getThemeSubTextColor(), fontSize: 12),
              ),
            ],
          ),
          actions: [
            WoxButton.secondary(
              text: controller.tr("ui_cloud_sync_recovery_code_copy"),
              onPressed: () {
                Clipboard.setData(ClipboardData(text: code));
              },
            ),
            WoxButton.primary(
              text: controller.tr("ui_cloud_sync_recovery_code_close"),
              onPressed: () {
                Navigator.pop(context);
              },
            ),
          ],
        );
      },
    );
  }

  Future<Map<String, String>?> showCloudSyncInitKeyDialog(BuildContext context) async {
    final recoveryController = TextEditingController();
    final deviceController = TextEditingController();
    try {
      return await showDialog<Map<String, String>>(
        context: context,
        builder: (context) {
          return AlertDialog(
            title: Text(controller.tr("ui_cloud_sync_key_init_title")),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(controller.tr("ui_cloud_sync_recovery_code_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 6),
                WoxTextField(
                  controller: recoveryController,
                  hintText: controller.tr("ui_cloud_sync_recovery_code_hint"),
                  width: 360,
                ),
                const SizedBox(height: 12),
                Text(controller.tr("ui_cloud_sync_device_name"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 6),
                WoxTextField(
                  controller: deviceController,
                  hintText: controller.tr("ui_cloud_sync_device_name_hint"),
                  width: 360,
                ),
              ],
            ),
            actions: [
              WoxButton.secondary(
                text: controller.tr("ui_cloud_sync_cancel"),
                onPressed: () {
                  Navigator.pop(context);
                },
              ),
              WoxButton.primary(
                text: controller.tr("ui_cloud_sync_confirm"),
                onPressed: () {
                  Navigator.pop(context, {
                    "recoveryCode": recoveryController.text.trim(),
                    "deviceName": deviceController.text.trim(),
                  });
                },
              ),
            ],
          );
        },
      );
    } finally {
      recoveryController.dispose();
      deviceController.dispose();
    }
  }

  Future<String?> showCloudSyncFetchKeyDialog(BuildContext context) async {
    final recoveryController = TextEditingController();
    try {
      return await showDialog<String>(
        context: context,
        builder: (context) {
          return AlertDialog(
            title: Text(controller.tr("ui_cloud_sync_key_fetch_title")),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(controller.tr("ui_cloud_sync_recovery_code_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 6),
                WoxTextField(
                  controller: recoveryController,
                  hintText: controller.tr("ui_cloud_sync_recovery_code_hint"),
                  width: 360,
                ),
              ],
            ),
            actions: [
              WoxButton.secondary(
                text: controller.tr("ui_cloud_sync_cancel"),
                onPressed: () {
                  Navigator.pop(context);
                },
              ),
              WoxButton.primary(
                text: controller.tr("ui_cloud_sync_confirm"),
                onPressed: () {
                  Navigator.pop(context, recoveryController.text.trim());
                },
              ),
            ],
          );
        },
      );
    } finally {
      recoveryController.dispose();
    }
  }

  Future<bool?> showCloudSyncResetDialog(BuildContext context) async {
    return await showDialog<bool>(
      context: context,
      builder: (context) {
        return AlertDialog(
          title: Text(controller.tr("ui_cloud_sync_reset_title")),
          content: Text(
            controller.tr("ui_cloud_sync_reset_warning"),
            style: TextStyle(color: getThemeSubTextColor(), fontSize: 12),
          ),
          actions: [
            WoxButton.secondary(
              text: controller.tr("ui_cloud_sync_cancel"),
              onPressed: () {
                Navigator.pop(context, false);
              },
            ),
            WoxButton.primary(
              text: controller.tr("ui_cloud_sync_reset_confirm"),
              onPressed: () {
                Navigator.pop(context, true);
              },
            ),
          ],
        );
      },
    );
  }

  Widget buildCloudSyncSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Obx(() {
          final status = controller.cloudSyncStatus.value;
          final state = status.state;
          final isLoading = controller.isCloudSyncStatusLoading.value;
          final statusError = controller.cloudSyncStatusError.value;
          final actionError = controller.cloudSyncActionError.value;
          final isBusy = controller.isCloudSyncActionLoading.value;
          final isEnabled = status.enabled && statusError.isEmpty;
          final hasKey = status.keyStatus.available;

          final statusValue = status.enabled ? controller.tr("ui_cloud_sync_enabled") : controller.tr("ui_cloud_sync_disabled");
          final keyStatusValue = hasKey ? controller.tr("ui_cloud_sync_key_available") : controller.tr("ui_cloud_sync_key_missing");

          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (isLoading) ...[
                Text(controller.tr("ui_cloud_sync_loading"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                const SizedBox(height: 8),
              ],
              if (statusError.isNotEmpty) ...[
                Text(normalizeCloudSyncError(statusError), style: const TextStyle(color: Colors.red, fontSize: 12)),
                const SizedBox(height: 8),
              ],
              buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_status_label"), statusValue),
              buildCloudSyncInfoRow(
                controller.tr("ui_cloud_sync_device_id"),
                status.deviceId.isNotEmpty ? status.deviceId : '-',
              ),
              buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_key_status"), keyStatusValue),
              if (hasKey) buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_key_version"), status.keyStatus.version.toString()),
              if (state != null) ...[
                buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_last_push"), formatCloudSyncTime(state.lastPushTs)),
                buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_last_pull"), formatCloudSyncTime(state.lastPullTs)),
                buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_backoff_until"), formatCloudSyncTime(state.backoffUntil)),
                buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_retry_count"), state.retryCount.toString()),
                buildCloudSyncInfoRow(
                  controller.tr("ui_cloud_sync_bootstrapped"),
                  state.bootstrapped ? controller.tr("ui_cloud_sync_enabled") : controller.tr("ui_cloud_sync_disabled"),
                ),
                if (state.lastError.isNotEmpty) buildCloudSyncInfoRow(controller.tr("ui_cloud_sync_last_error"), state.lastError),
              ],
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  WoxButton.secondary(
                    text: controller.tr("ui_cloud_sync_refresh"),
                    onPressed: isBusy ? null : () => controller.refreshCloudSyncStatus(),
                  ),
                  WoxButton.primary(
                    text: controller.tr("ui_cloud_sync_push"),
                    onPressed: isEnabled && hasKey && !isBusy ? () => controller.cloudSyncPush() : null,
                  ),
                  WoxButton.secondary(
                    text: controller.tr("ui_cloud_sync_pull"),
                    onPressed: isEnabled && hasKey && !isBusy ? () => controller.cloudSyncPull() : null,
                  ),
                ],
              ),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  WoxButton.secondary(
                    text: controller.tr("ui_cloud_sync_recovery_code"),
                    onPressed: isEnabled && !isBusy
                        ? () async {
                            final code = await controller.cloudSyncGenerateRecoveryCode();
                            if (code != null && code.isNotEmpty) {
                              await showRecoveryCodeDialog(context, code);
                            }
                          }
                        : null,
                  ),
                  if (!hasKey)
                    WoxButton.primary(
                      text: controller.tr("ui_cloud_sync_init_key"),
                      onPressed: isEnabled && !isBusy
                          ? () async {
                              final input = await showCloudSyncInitKeyDialog(context);
                              if (input == null) {
                                return;
                              }
                              final recoveryCode = input["recoveryCode"] ?? '';
                              if (recoveryCode.isEmpty) {
                                return;
                              }
                              await controller.cloudSyncInitKey(recoveryCode, input["deviceName"] ?? '');
                            }
                          : null,
                    ),
                  if (!hasKey)
                    WoxButton.secondary(
                      text: controller.tr("ui_cloud_sync_fetch_key"),
                      onPressed: isEnabled && !isBusy
                          ? () async {
                              final recoveryCode = await showCloudSyncFetchKeyDialog(context);
                              if (recoveryCode == null || recoveryCode.isEmpty) {
                                return;
                              }
                              await controller.cloudSyncFetchKey(recoveryCode);
                            }
                          : null,
                    ),
                  WoxButton.secondary(
                    text: controller.tr("ui_cloud_sync_reset"),
                    onPressed: isEnabled && !isBusy
                        ? () async {
                            final token = await controller.cloudSyncPrepareReset();
                            if (token == null || token.isEmpty) {
                              return;
                            }
                            final confirmed = await showCloudSyncResetDialog(context);
                            if (confirmed == true) {
                              await controller.cloudSyncReset(token);
                            }
                          }
                        : null,
                  ),
                ],
              ),
              if (actionError.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text(actionError, style: const TextStyle(color: Colors.red, fontSize: 12)),
              ],
            ],
          );
        }),
      ],
    );
  }

  Widget buildCloudSyncPluginExclusions() {
    return Obx(() {
      if (controller.isCloudSyncPluginListLoading.value) {
        return Text(controller.tr("ui_cloud_sync_plugin_exclusions_loading"));
      }
      if (controller.installedPlugins.isEmpty) {
        return Text(controller.tr("ui_cloud_sync_plugin_exclusions_empty"));
      }

      final disabled = controller.woxSetting.value.cloudSyncDisabledPlugins.toSet();
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              WoxButton.secondary(
                text: controller.tr("ui_cloud_sync_plugin_exclusions_refresh"),
                onPressed: () async {
                  controller.isCloudSyncPluginListLoading.value = true;
                  try {
                    await controller.loadInstalledPlugins(const UuidV4().generate());
                  } finally {
                    controller.isCloudSyncPluginListLoading.value = false;
                  }
                },
              ),
            ],
          ),
          const SizedBox(height: 8),
          ...controller.installedPlugins.map((plugin) {
            final pluginId = plugin.id;
            final title = plugin.name.isNotEmpty ? plugin.name : pluginId;
            final isDisabled = disabled.contains(pluginId);
            return WoxCheckboxTile(
              value: isDisabled,
              onChanged: (checked) async {
                final updated = List<String>.from(controller.woxSetting.value.cloudSyncDisabledPlugins);
                if (checked) {
                  if (!updated.contains(pluginId)) {
                    updated.add(pluginId);
                  }
                } else {
                  updated.remove(pluginId);
                }
                await controller.updateCloudSyncDisabledPlugins(updated);
              },
              title: title,
            );
          }).toList(),
        ],
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    return form(children: [
      formField(
        label: controller.tr("ui_cloud_sync"),
        child: buildCloudSyncSection(context),
      ),
      formField(
        label: controller.tr("ui_cloud_sync_plugin_exclusions"),
        child: buildCloudSyncPluginExclusions(),
        tips: controller.tr("ui_cloud_sync_plugin_exclusions_tips"),
      ),
    ]);
  }
}
