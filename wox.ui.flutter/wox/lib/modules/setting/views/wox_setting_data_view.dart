import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/picker.dart';
import 'package:wox/utils/wox_setting_focus_util.dart';

class WoxSettingDataView extends WoxSettingBaseView {
  const WoxSettingDataView({super.key});

  static const String _backupTableKey = "backup_list";
  static const String _backupTableDateKey = "date";
  static const String _backupTableTypeKey = "type";
  static const String _backupTableOperationKey = "operation";
  static const String _backupTableIdKey = "id";
  static const String _backupTablePathKey = "path";

  Widget _buildAutoBackupTips() {
    return Wrap(
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Text(controller.tr("ui_data_backup_auto_tips_prefix"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
        WoxButton.text(
          text: controller.tr("ui_data_backup_folder_link"),
          onPressed: () async {
            try {
              final backupPath = await WoxApi.instance.getBackupFolder(const UuidV4().generate());
              await controller.openFolder(backupPath);
            } catch (e) {
              // Handle error silently or show a notification
            }
          },
        ),
        Text(controller.tr("ui_data_backup_auto_tips_suffix"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
      ],
    );
  }

  PluginSettingValueTable _buildBackupTableDefinition() {
    // Backup List used to be the only top-level settings table backed by Material
    // DataTable directly. Defining it as a read-only Wox table keeps border,
    // header, empty-state, and scrolling behavior aligned with other settings tables.
    return PluginSettingValueTable.fromJson(<String, dynamic>{
      "Key": _backupTableKey,
      "DefaultValue": "[]",
      "Title": "",
      "Tooltip": "",
      "MaxHeight": 300,
      "Columns": [
        {
          "Key": _backupTableDateKey,
          "Label": "ui_data_backup_date",
          "Tooltip": "",
          "Width": 350,
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "TextMaxLines": 1,
        },
        {
          "Key": _backupTableTypeKey,
          "Label": "ui_data_backup_type",
          "Tooltip": "",
          "Width": 220,
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "TextMaxLines": 1,
        },
        {
          "Key": _backupTableOperationKey,
          "Label": "ui_operation",
          "Tooltip": "",
          "Width": 300,
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "TextMaxLines": 1,
        },
      ],
    });
  }

  String _formatBackupDate(DateTime date) {
    return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')} ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}:${date.second.toString().padLeft(2, '0')}';
  }

  String _encodeBackupTableRows() {
    return jsonEncode(
      controller.backups.map((backup) {
        final date = DateTime.fromMillisecondsSinceEpoch(backup.timestamp);
        return <String, dynamic>{
          _backupTableDateKey: _formatBackupDate(date),
          _backupTableTypeKey: backup.type == "auto" ? controller.tr("ui_data_backup_type_auto") : controller.tr("ui_data_backup_type_manual"),
          _backupTableOperationKey: "",
          _backupTableIdKey: backup.id,
          _backupTablePathKey: backup.path,
        };
      }).toList(),
    );
  }

  Widget _buildBackupOperationCell(BuildContext context, Map<String, dynamic> row) {
    final backupId = row[_backupTableIdKey]?.toString() ?? "";
    final backupPath = row[_backupTablePathKey]?.toString() ?? "";

    return Row(
      children: [
        WoxButton.text(
          text: controller.tr("ui_data_backup_restore"),
          onPressed: () async {
            await showDialog(
              context: context,
              barrierColor: getThemePopupBarrierColor(),
              builder: (context) {
                return WoxDialog(
                  title: Text(controller.tr("ui_data_backup_restore_confirm_title")),
                  content: Text(controller.tr("ui_data_backup_restore_confirm_message")),
                  actions: [
                    WoxButton.secondary(
                      text: controller.tr("ui_data_backup_restore_cancel"),
                      onPressed: () {
                        Navigator.pop(context);
                      },
                    ),
                    WoxButton.primary(
                      text: controller.tr("ui_data_backup_restore_confirm"),
                      onPressed: () {
                        Navigator.pop(context);
                        if (backupId.isNotEmpty) {
                          controller.restoreBackup(backupId);
                        }
                      },
                    ),
                  ],
                );
              },
            );
            WoxSettingFocusUtil.restoreIfInSettingView();
          },
        ),
        WoxButton.text(
          text: controller.tr("plugin_file_open"),
          onPressed: () {
            if (backupPath.isNotEmpty) {
              controller.openFolder(backupPath);
            }
          },
        ),
      ],
    );
  }

  Widget _buildBackupListTable(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 28),
      child: SizedBox(
        width: GENERAL_SETTING_TABLE_WIDTH,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(controller.tr("ui_data_backup_list_title"), style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500)),
                const Spacer(),
                // Keep backup creation visually aligned with table Add buttons: compact, outlined, and anchored to the table header edge.
                Obx(() {
                  final isBackingUp = controller.isBackingUp.value;
                  // Manual backups can run long enough for users to click again. Disabling the
                  // button uses the existing gray disabled style, while the spinner shows that
                  // the click was accepted without adding another dialog or background state.
                  return WoxButton.secondary(
                    text: controller.tr("ui_data_backup_now"),
                    icon: isBackingUp ? WoxLoadingIndicator(size: 14, color: getThemeTextColor().withValues(alpha: 0.5)) : Icon(Icons.add, color: getThemeSubTextColor()),
                    height: 30,
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                    onPressed:
                        isBackingUp
                            ? null
                            : () {
                              controller.backupNow();
                            },
                  );
                }),
              ],
            ),
            const SizedBox(height: 10),
            Obx(() {
              // Feed generated backup rows through the shared Wox table so this read-only
              // list uses the same grid painting as plugin tables, query hotkeys, and AI tables.
              return WoxSettingPluginTable(
                item: _buildBackupTableDefinition(),
                value: _encodeBackupTableRows(),
                onUpdate: (key, value) async => null,
                tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                readonly: true,
                customCellBuilder: (column, row) {
                  if (column.key == _backupTableOperationKey) {
                    return _buildBackupOperationCell(context, row);
                  }

                  return null;
                },
              );
            }),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return form(
      width: GENERAL_SETTING_WIDE_FORM_WIDTH,
      title: controller.tr("ui_data"),
      description: controller.tr("ui_data_description"),
      children: [
        formSection(
          title: controller.tr("ui_data_section_storage"),
          children: [
            formField(
              settingKey: "UserDataLocation",
              label: controller.tr("ui_data_config_location"),
              labelWidth: GENERAL_SETTING_WIDE_LABEL_WIDTH,
              child: Obx(
                () => Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // The full path is noisy in this dense settings page. Keep the two actions
                    // users need and leave the explanatory copy to describe what location changes do.
                    WoxButton.secondary(
                      text: controller.tr("plugin_file_open"),
                      onPressed: () {
                        controller.openFolder(controller.userDataLocation.value);
                      },
                    ),
                    const SizedBox(width: 10),
                    WoxButton.primary(
                      text: controller.tr("ui_data_config_location_change"),
                      onPressed: () async {
                        final selectedDirectory = await FileSelector.pick(const UuidV4().generate(), FileSelectorParams(isDirectory: true));
                        if (selectedDirectory.isEmpty || !context.mounted) {
                          return;
                        }

                        final picked = selectedDirectory[0];
                        // The compact Data layout no longer embeds WoxPathFinder, so it keeps the
                        // same confirmation flow here before moving Wox's storage location.
                        await showDialog(
                          context: context,
                          barrierColor: getThemePopupBarrierColor(),
                          builder:
                              (dialogContext) => WoxDialog(
                                content: Text(controller.tr("ui_data_config_location_change_confirm").replaceAll("{0}", picked)),
                                actions: [
                                  WoxButton.secondary(text: controller.tr("ui_data_config_location_change_cancel"), onPressed: () => Navigator.pop(dialogContext)),
                                  WoxButton.primary(
                                    text: controller.tr("ui_data_config_location_change_confirm_button"),
                                    onPressed: () {
                                      Navigator.pop(dialogContext);
                                      controller.updateUserDataLocation(picked);
                                    },
                                  ),
                                ],
                              ),
                        );
                        WoxSettingFocusUtil.restoreIfInSettingView();
                      },
                    ),
                  ],
                ),
              ),
              tips: controller.tr("ui_data_config_location_tips"),
            ),
          ],
        ),
        formSection(
          title: controller.tr("ui_data_section_backup"),
          children: [
            formField(
              settingKey: "EnableAutoBackup",
              label: controller.tr("ui_data_backup_auto_title"),
              labelWidth: GENERAL_SETTING_WIDE_LABEL_WIDTH,
              child: Obx(() {
                return WoxSwitch(
                  value: controller.woxSetting.value.enableAutoBackup,
                  onChanged: (value) {
                    controller.updateConfig("EnableAutoBackup", value.toString());
                  },
                );
              }),
              tipsWidget: _buildAutoBackupTips(),
            ),
            _buildBackupListTable(context),
          ],
        ),
        formSection(
          title: controller.tr("ui_data_section_logs"),
          children: [
            formField(
              label: controller.tr("ui_data_log_level_title"),
              labelWidth: GENERAL_SETTING_WIDE_LABEL_WIDTH,
              child: Obx(() {
                final logLevel = controller.woxSetting.value.logLevel.toUpperCase();
                final selectedLogLevel = logLevel == "DEBUG" ? "DEBUG" : "INFO";
                final isUpdatingLogLevel = controller.isUpdatingLogLevel.value;
                return WoxDropdownButton<String>(
                  value: selectedLogLevel,
                  items: [
                    WoxDropdownItem(value: "INFO", label: controller.tr("ui_data_log_level_info")),
                    WoxDropdownItem(value: "DEBUG", label: controller.tr("ui_data_log_level_debug")),
                  ],
                  onChanged:
                      isUpdatingLogLevel
                          ? null
                          : (value) {
                            if (value != null) {
                              controller.updateLogLevel(value);
                            }
                          },
                  isExpanded: true,
                );
              }),
              tips: controller.tr("ui_data_log_level_tips"),
            ),
            formField(
              label: controller.tr("ui_data_log_clear_title"),
              labelWidth: GENERAL_SETTING_WIDE_LABEL_WIDTH,
              child: Obx(() {
                final isClearing = controller.isClearingLogs.value;
                return Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    WoxButton.secondary(
                      text: controller.tr("ui_data_log_clear_button"),
                      icon: isClearing ? WoxLoadingIndicator(size: 14, color: getThemeTextColor()) : null,
                      onPressed:
                          isClearing
                              ? null
                              : () async {
                                await showDialog(
                                  context: context,
                                  barrierColor: getThemePopupBarrierColor(),
                                  builder: (dialogContext) {
                                    return WoxDialog(
                                      title: Text(controller.tr("ui_data_log_clear_confirm_title")),
                                      content: Text(controller.tr("ui_data_log_clear_confirm_message")),
                                      actions: [
                                        WoxButton.secondary(
                                          text: controller.tr("ui_data_log_clear_cancel"),
                                          onPressed: () {
                                            Navigator.pop(dialogContext);
                                          },
                                        ),
                                        WoxButton.primary(
                                          text: controller.tr("ui_data_log_clear_confirm"),
                                          onPressed: () {
                                            Navigator.pop(dialogContext);
                                            controller.clearLogs();
                                          },
                                        ),
                                      ],
                                    );
                                  },
                                );
                                WoxSettingFocusUtil.restoreIfInSettingView();
                              },
                    ),
                    const SizedBox(width: 10),
                    WoxButton.secondary(
                      text: controller.tr("ui_data_log_open_button"),
                      onPressed:
                          isClearing
                              ? null
                              : () {
                                controller.openLogFile();
                              },
                    ),
                  ],
                );
              }),
              tips: controller.tr("ui_data_log_clear_tips"),
            ),
          ],
        ),
      ],
    );
  }
}
