import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/picker.dart';
import 'package:flutter/material.dart' as material;

class WoxSettingDataView extends WoxSettingBaseView {
  const WoxSettingDataView({super.key});

  @override
  Widget build(BuildContext context) {
    return form(children: [
      formField(
        label: controller.tr("ui_data_config_location"),
        child: Obx(() {
          return TextBox(
            controller: TextEditingController(text: controller.userDataLocation.value),
            readOnly: true,
            suffix: Row(
              children: [
                Button(
                  style: ButtonStyle(
                    backgroundColor: ButtonState.all(Colors.blue),
                    foregroundColor: ButtonState.all(Colors.white),
                  ),
                  child: Text(controller.tr("ui_data_config_location_change")),
                  onPressed: () async {
                    // Store the context before async operation
                    final currentContext = context;
                    final result = await FileSelector.pick(
                      const UuidV4().generate(),
                      FileSelectorParams(isDirectory: true),
                    );
                    if (result.isNotEmpty) {
                      if (currentContext.mounted) {
                        showDialog(
                          context: currentContext,
                          builder: (context) {
                            return ContentDialog(
                              content: Text(controller.tr("ui_data_config_location_change_confirm").replaceAll("{0}", result[0])),
                              actions: [
                                Button(
                                  child: Text(controller.tr("ui_data_config_location_change_cancel")),
                                  onPressed: () {
                                    Navigator.pop(context);
                                  },
                                ),
                                FilledButton(
                                  child: Text(controller.tr("ui_data_config_location_change_confirm_button")),
                                  onPressed: () {
                                    Navigator.pop(context);
                                    controller.updateUserDataLocation(result[0]);
                                  },
                                ),
                              ],
                            );
                          },
                        );
                      }
                    }
                  },
                ),
                const SizedBox(width: 10),
                Button(
                  child: Text(controller.tr("plugin_file_open")),
                  onPressed: () {
                    controller.openFolder(controller.userDataLocation.value);
                  },
                ),
              ],
            ),
          );
        }),
        tips: controller.tr("ui_data_config_location_tips"),
      ),
      formField(
        label: controller.tr("ui_data_backup_auto_title"),
        child: Obx(() {
          return ToggleSwitch(
            checked: controller.woxSetting.value.enableAutoBackup,
            onChanged: (value) {
              controller.updateConfig("EnableAutoBackup", value.toString());
            },
          );
        }),
        tips: controller.tr("ui_data_backup_auto_tips"),
      ),
      formField(
        label: controller.tr("ui_data_backup_list_title"),
        child: Column(
          children: [
            Row(
              children: [
                Button(
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
                  headingRowHeight: 40,
                  headingRowColor: material.MaterialStateProperty.resolveWith((states) => material.Colors.grey[200]),
                  border: TableBorder.all(color: material.Colors.grey[300]!),
                  columns: [
                    material.DataColumn(
                      label: Expanded(
                        child: Text(
                          controller.tr("ui_data_backup_date"),
                          style: const TextStyle(
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ),
                    ),
                    material.DataColumn(
                      label: Expanded(
                        child: Text(
                          controller.tr("ui_data_backup_type"),
                          style: const TextStyle(
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                      ),
                    ),
                    material.DataColumn(
                      label: Text(
                        controller.tr("ui_operation"),
                        style: const TextStyle(
                          overflow: TextOverflow.ellipsis,
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
                            style: const TextStyle(
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ),
                        material.DataCell(
                          Text(
                            backup.type == "auto" ? controller.tr("ui_data_backup_type_auto") : controller.tr("ui_data_backup_type_manual"),
                            style: const TextStyle(
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ),
                        material.DataCell(
                          Row(
                            children: [
                              HyperlinkButton(
                                child: Text(controller.tr("ui_data_backup_restore")),
                                onPressed: () {
                                  showDialog(
                                    context: context,
                                    builder: (context) {
                                      return ContentDialog(
                                        title: Text(controller.tr("ui_data_backup_restore_confirm_title")),
                                        content: Text(controller.tr("ui_data_backup_restore_confirm_message")),
                                        actions: [
                                          Button(
                                            child: Text(controller.tr("ui_data_backup_restore_cancel")),
                                            onPressed: () {
                                              Navigator.pop(context);
                                            },
                                          ),
                                          FilledButton(
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
                              HyperlinkButton(
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
