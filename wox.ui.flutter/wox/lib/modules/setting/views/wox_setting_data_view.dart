import 'package:flutter/material.dart' hide DataTable;
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/picker.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:flutter/material.dart' as material;
import 'package:wox/api/wox_api.dart';
import 'package:wox/utils/color_util.dart';

class WoxSettingDataView extends WoxSettingBaseView {
  const WoxSettingDataView({super.key});

  Widget _buildAutoBackupTips() {
    return Wrap(
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Text(
          controller.tr("ui_data_backup_auto_tips_prefix"),
          style: TextStyle(color: getThemeSubTextColor(), fontSize: 13),
        ),
        TextButton(
          onPressed: () async {
            try {
              final backupPath = await WoxApi.instance.getBackupFolder();
              await controller.openFolder(backupPath);
            } catch (e) {
              // Handle error silently or show a notification
            }
          },
          child: Text(
            controller.tr("ui_data_backup_folder_link"),
            style: TextStyle(
              color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor),
              fontSize: 13,
              decoration: TextDecoration.underline,
            ),
          ),
        ),
        Text(
          controller.tr("ui_data_backup_auto_tips_suffix"),
          style: TextStyle(color: getThemeSubTextColor(), fontSize: 13),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return form(children: [
      formField(
        label: controller.tr("ui_data_config_location"),
        child: Row(
          children: [
            Expanded(
              child: Obx(() {
                return TextField(
                  controller: TextEditingController(text: controller.userDataLocation.value),
                  readOnly: true,
                );
              }),
            ),
            const SizedBox(width: 10),
            ElevatedButton(
              child: Text(controller.tr("ui_data_config_location_change")),
              onPressed: () async {
                final currentContext = context;
                final result = await FileSelector.pick(
                  const UuidV4().generate(),
                  FileSelectorParams(isDirectory: true),
                );
                if (result.isNotEmpty && currentContext.mounted) {
                  showDialog(
                    context: currentContext,
                    builder: (context) => AlertDialog(
                      content: Text(controller.tr("ui_data_config_location_change_confirm").replaceAll("{0}", result[0])),
                      actions: [
                        TextButton(
                          child: Text(controller.tr("ui_data_config_location_change_cancel")),
                          onPressed: () => Navigator.pop(context),
                        ),
                        ElevatedButton(
                          child: Text(controller.tr("ui_data_config_location_change_confirm_button")),
                          onPressed: () {
                            Navigator.pop(context);
                            controller.updateUserDataLocation(result[0]);
                          },
                        ),
                      ],
                    ),
                  );
                }
              },
            ),
            const SizedBox(width: 10),
            ElevatedButton(
              child: Text(controller.tr("plugin_file_open")),
              onPressed: () => controller.openFolder(controller.userDataLocation.value),
            ),
          ],
        ),
        tips: controller.tr("ui_data_config_location_tips"),
      ),
      formField(
        label: controller.tr("ui_data_backup_auto_title"),
        child: Obx(() {
          return WoxSwitch(
            value: controller.woxSetting.value.enableAutoBackup,
            onChanged: (value) {
              controller.updateConfig("EnableAutoBackup", value.toString());
            },
          );
        }),
        tips: null,
        customTips: _buildAutoBackupTips(),
      ),
      formField(
        label: controller.tr("ui_data_backup_list_title"),
        child: Column(
          children: [
            Row(
              children: [
                ElevatedButton(
                  child: Text(controller.tr("ui_data_backup_now")),
                  onPressed: () {
                    controller.backupNow();
                  },
                ),
              ],
            ),
            const SizedBox(height: 10),
            SizedBox(
              width: 760,
              child: Obx(() {
                if (controller.backups.isEmpty) {
                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 20),
                    child: Center(
                      child: Text(controller.tr("ui_data_backup_empty")),
                    ),
                  );
                }

                return material.DataTable(
                  columnSpacing: 10,
                  horizontalMargin: 5,
                  headingRowHeight: 36,
                  dataRowMinHeight: 36,
                  dataRowMaxHeight: 36,
                  headingRowColor: material.WidgetStateProperty.resolveWith((states) => safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor)),
                  border: TableBorder.all(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewSplitLineColor)),
                  columns: [
                    material.DataColumn(
                      label: Expanded(
                        child: Text(
                          controller.tr("ui_data_backup_date"),
                          style: TextStyle(
                            overflow: TextOverflow.ellipsis,
                            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor),
                            fontSize: 13,
                          ),
                        ),
                      ),
                    ),
                    material.DataColumn(
                      label: Expanded(
                        child: Text(
                          controller.tr("ui_data_backup_type"),
                          style: TextStyle(
                            overflow: TextOverflow.ellipsis,
                            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor),
                            fontSize: 13,
                          ),
                        ),
                      ),
                    ),
                    material.DataColumn(
                      label: Text(
                        controller.tr("ui_operation"),
                        style: TextStyle(
                          overflow: TextOverflow.ellipsis,
                          color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor),
                          fontSize: 13,
                        ),
                      ),
                    ),
                  ],
                  rows: controller.backups.map((backup) {
                    final date = DateTime.fromMillisecondsSinceEpoch(backup.timestamp);
                    final dateStr =
                        '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')} ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}:${date.second.toString().padLeft(2, '0')}';

                    return material.DataRow(
                      cells: [
                        material.DataCell(
                          Text(
                            dateStr,
                            style: TextStyle(
                              overflow: TextOverflow.ellipsis,
                              color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor),
                            ),
                          ),
                        ),
                        material.DataCell(
                          Text(
                            backup.type == "auto" ? controller.tr("ui_data_backup_type_auto") : controller.tr("ui_data_backup_type_manual"),
                            style: TextStyle(
                              overflow: TextOverflow.ellipsis,
                              color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor),
                            ),
                          ),
                        ),
                        material.DataCell(
                          Row(
                            children: [
                              TextButton(
                                style: TextButton.styleFrom(
                                  foregroundColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor),
                                ),
                                child: Text(controller.tr("ui_data_backup_restore")),
                                onPressed: () {
                                  showDialog(
                                    context: context,
                                    builder: (context) {
                                      return AlertDialog(
                                        title: Text(controller.tr("ui_data_backup_restore_confirm_title")),
                                        content: Text(controller.tr("ui_data_backup_restore_confirm_message")),
                                        actions: [
                                          TextButton(
                                            child: Text(controller.tr("ui_data_backup_restore_cancel")),
                                            onPressed: () {
                                              Navigator.pop(context);
                                            },
                                          ),
                                          ElevatedButton(
                                            child: Text(controller.tr("ui_data_backup_restore_confirm")),
                                            onPressed: () {
                                              Navigator.pop(context);
                                              controller.restoreBackup(backup.id);
                                            },
                                          ),
                                        ],
                                      );
                                    },
                                  );
                                },
                              ),
                              TextButton(
                                style: TextButton.styleFrom(
                                  foregroundColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor),
                                ),
                                child: Text(controller.tr("plugin_file_open")),
                                onPressed: () {
                                  controller.openFolder(backup.path);
                                },
                              ),
                            ],
                          ),
                        ),
                      ],
                    );
                  }).toList(),
                );
              }),
            ),
          ],
        ),
      ),
    ]);
  }
}
