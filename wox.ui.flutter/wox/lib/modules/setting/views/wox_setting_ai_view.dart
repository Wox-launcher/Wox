import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_path_finder.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxSettingAIView extends WoxSettingBaseView {
  const WoxSettingAIView({super.key});

  @override
  Widget build(BuildContext context) {
    return FutureBuilder(
      future: WoxApi.instance.findAIProviders(const UuidV4().generate()),
      builder: (context, snapshot) {
        if (snapshot.hasData) {
          return form(
            title: controller.tr("ui_ai"),
            description: controller.tr("ui_ai_description"),
            children: [
              settingTarget(
                settingKey: "AIProviders",
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 24),
                  child: Obx(() {
                    return WoxSettingPluginTable(
                      inlineTitleActions: true,
                      tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                      value: json.encode(controller.woxSetting.value.aiProviders),
                      item: PluginSettingValueTable.fromJson({
                        "Key": "AIProviders",
                        "Title": "i18n:ui_ai_model",
                        "Columns": [
                          {
                            "Key": "Status",
                            "Label": "i18n:ui_ai_providers_status",
                            "HideInUpdate": true,
                            "Width": 40,
                            "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeAIModelStatus,
                          },
                          {
                            "Key": "Name",
                            "Label": "i18n:ui_ai_providers_name",
                            "Tooltip": "i18n:ui_ai_providers_name_tooltip",
                            "Width": 100,
                            "Type": "select",
                            "SelectOptions":
                                snapshot.data!
                                    .map(
                                      (e) => {
                                        "Label": e.name,
                                        "Value": e.name,
                                        "Icon": e.icon.toJson(),
                                        "Extra": {"DefaultHost": e.defaultHost},
                                      },
                                    )
                                    .toList(),
                            "TextMaxLines": 1,
                            "Validators": [
                              {"Type": "not_empty"},
                            ],
                            "OnChangedActions": [
                              {"TargetKey": "Host", "ValueFromSelectedOptionExtra": "DefaultHost", "OverwriteMode": "always", "ApplyOnInit": true},
                            ],
                          },
                          {"Key": "Alias", "Label": "i18n:ui_ai_providers_alias", "Tooltip": "i18n:ui_ai_providers_alias_tooltip", "Width": 120, "Type": "text", "TextMaxLines": 1},

                          {"Key": "Host", "Label": "i18n:ui_ai_providers_host", "Tooltip": "i18n:ui_ai_providers_host_tooltip", "Width": 160, "Type": "text"},
                          {"Key": "ApiKey", "Label": "i18n:ui_ai_providers_api_key", "Tooltip": "i18n:ui_ai_providers_api_key_tooltip", "Type": "text", "TextMaxLines": 1},
                        ],
                        "SortColumnKey": "Name",
                      }),
                      onUpdate: (key, value) async {
                        await controller.updateConfig("AIProviders", value);
                        return null;
                      },
                      onUpdateValidate: (rowValues) async {
                        if (rowValues["Name"] != "ollama") {
                          if (rowValues["ApiKey"] == null || rowValues["ApiKey"] == "") {
                            return const [PluginSettingTableValidationError(key: "ApiKey", errorMsg: "ui_ai_providers_api_key_required")];
                          }
                        }
                        return const [];
                      },
                    );
                  }),
                ),
              ),
              settingTarget(
                settingKey: "AIMCPServers",
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 24),
                  child: Obx(() {
                    return WoxSettingPluginTable(
                      inlineTitleActions: true,
                      tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                      value: json.encode(controller.woxSetting.value.aiMCPServers),
                      item: _buildMCPServersTable(),
                      onUpdate: (key, value) async {
                        await controller.updateConfig("AIMCPServers", value);
                        return null;
                      },
                      onUpdateValidate: (rowValues) async {
                        final type = rowValues["Type"]?.toString() ?? "";
                        if (rowValues["Name"] == null || rowValues["Name"] == "") {
                          return const [PluginSettingTableValidationError(key: "Name", errorMsg: "plugin_ai_chat_mcp_server_name_required")];
                        }
                        if (type == "stdio" && (rowValues["Command"] == null || rowValues["Command"] == "")) {
                          return const [PluginSettingTableValidationError(key: "Command", errorMsg: "plugin_ai_chat_mcp_server_command_required")];
                        }
                        if (type == "streamable-http" && (rowValues["Url"] == null || rowValues["Url"] == "")) {
                          return const [PluginSettingTableValidationError(key: "Url", errorMsg: "plugin_ai_chat_mcp_server_url_required")];
                        }
                        return const [];
                      },
                    );
                  }),
                ),
              ),
              settingTarget(
                settingKey: "AISkills",
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 24),
                  child: Obx(() {
                    return WoxSettingPluginTable(
                      inlineTitleActions: true,
                      showCloneAction: false,
                      showEditAction: false,
                      tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                      value: json.encode(controller.woxSetting.value.aiSkills),
                      item: _buildSkillsTable(),
                      customCellBuilder: (column, row) {
                        if (column.key == "Source") {
                          final source = row["Source"]?.toString() ?? "";
                          final label = source == "remote" ? controller.tr("ui_ai_skill_type_remote") : controller.tr("ui_ai_skill_type_local");
                          return Text(label, style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor), fontSize: 13));
                        }
                        return null;
                      },
                      rowTrailingActionsBuilder: (context, row) {
                        final path = row["Path"]?.toString().trim() ?? "";
                        if (path.isEmpty) return const <Widget>[];
                        return [
                          WoxTooltip(
                            message: controller.tr("plugin_file_open"),
                            child: WoxButton.text(
                              text: '',
                              icon: Icon(Icons.folder_open, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
                              padding: const EdgeInsets.symmetric(horizontal: 4),
                              onPressed: () => controller.openFolder(path),
                            ),
                          ),
                        ];
                      },
                      customCreateDialogBuilder: (context, saveRow, {initialRow}) async {
                        await _showAddSkillDialog(context, saveRow, initialRow: initialRow);
                      },
                      onUpdate: (key, value) async {
                        await controller.updateConfig(key, value);
                        return null;
                      },
                    );
                  }),
                ),
              ),
            ],
          );
        }
        return const SizedBox.shrink();
      },
    );
  }

  PluginSettingValueTable _buildMCPServersTable() {
    return PluginSettingValueTable.fromJson({
      "Key": "AIMCPServers",
      "Title": "i18n:ui_ai_mcp_servers",
      "Tooltip": "i18n:ui_ai_mcp_servers_tooltip",
      "Columns": [
        {
          "Key": "Name",
          "Label": "i18n:plugin_ai_chat_mcp_server_name",
          "Tooltip": "i18n:plugin_ai_chat_mcp_server_name_tooltip",
          "Width": 100,
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "Validators": [
            {"Type": "not_empty"},
          ],
        },
        {
          "Key": "Tools",
          "Label": "i18n:plugin_ai_chat_mcp_server_tools",
          "Tooltip": "i18n:plugin_ai_chat_mcp_server_tools_tooltip",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeAIMCPServerTools,
          "Width": 50,
          "HideInUpdate": true,
        },
        {"Key": "Disabled", "Label": "i18n:plugin_ai_chat_mcp_server_disabled", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox, "Width": 80},
        {
          "Key": "Type",
          "Label": "i18n:plugin_ai_chat_mcp_server_type",
          "Tooltip": "i18n:plugin_ai_chat_mcp_server_type_tooltip",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeSelect,
          "Width": 80,
          "SelectOptions": [
            {"Label": "STDIO", "Value": "stdio"},
            {"Label": "Streamable HTTP", "Value": "streamable-http"},
          ],
          "Validators": [
            {"Type": "not_empty"},
          ],
        },
        {
          "Key": "Command",
          "Label": "i18n:plugin_ai_chat_mcp_server_command",
          "Tooltip": "i18n:plugin_ai_chat_mcp_server_command_tooltip",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "Width": 100,
        },
        {
          "Key": "EnvironmentVariables",
          "Label": "i18n:plugin_ai_chat_mcp_server_environment_variables",
          "Tooltip": "i18n:plugin_ai_chat_mcp_server_environment_variables_tooltip",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeTextList,
          "Width": 160,
        },
        {
          "Key": "Url",
          "Label": "i18n:plugin_ai_chat_mcp_server_url",
          "Tooltip": "i18n:plugin_ai_chat_mcp_server_url_tooltip",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "TextMaxLines": 10,
          "Width": 120,
        },
      ],
      "SortColumnKey": "Name",
    });
  }

  PluginSettingValueTable _buildSkillsTable() {
    return PluginSettingValueTable.fromJson({
      "Key": "AISkills",
      "Title": "i18n:ui_ai_skills",
      "Tooltip": "i18n:ui_ai_skills_tooltip",
      "MaxHeight": 360,
      "Columns": [
        {"Key": "Name", "Label": "i18n:plugin_ai_chat_skill_name", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 200, "HideInUpdate": true},
        {"Key": "Source", "Label": "i18n:plugin_ai_chat_skill_type", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 100, "HideInUpdate": true},
        {
          "Key": "Description",
          "Label": "i18n:plugin_ai_chat_skill_description",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "Width": 400,
          "HideInUpdate": true,
        },
        {
          "Key": "SourceUrl",
          "Label": "i18n:plugin_ai_chat_skill_source_url",
          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
          "Width": 200,
          "HideInUpdate": true,
          "HideInTable": true,
        },
        {"Key": "SourceName", "Label": "", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 0, "HideInUpdate": true, "HideInTable": true},
        {"Key": "ManifestPath", "Label": "", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 0, "HideInUpdate": true, "HideInTable": true},
        {"Key": "Enabled", "Label": "", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox, "Width": 0, "HideInUpdate": true, "HideInTable": true},
        {"Key": "Error", "Label": "", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 0, "HideInUpdate": true, "HideInTable": true},
        {"Key": "Path", "Label": "i18n:ui_ai_skill_add_path", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeDirPath, "Width": 400, "HideInTable": true},
      ],
      "SortColumnKey": "Name",
    });
  }

  Future<void> _showAddSkillDialog(BuildContext context, Future<String?> Function(Map<String, dynamic> row) saveRow, {Map<String, dynamic>? initialRow}) async {
    int selectedTab = 0; // 0 = local, 1 = remote
    String path = initialRow?["Path"]?.toString() ?? "";
    String remoteUrl = "";
    String? error;
    bool isCloning = false;

    await showDialog(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (context, setState) {
            return WoxDialog(
              title: Text(controller.tr("ui_ai_skill_add"), style: TextStyle(color: getThemeTextColor(), fontSize: 16, fontWeight: FontWeight.w600)),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Tab selector
                  Row(
                    children: [
                      _buildAddSkillTab(
                        context,
                        0,
                        selectedTab,
                        controller.tr("ui_ai_skill_add_local"),
                        () => setState(() {
                          selectedTab = 0;
                          error = null;
                        }),
                      ),
                      const SizedBox(width: 8),
                      _buildAddSkillTab(
                        context,
                        1,
                        selectedTab,
                        controller.tr("ui_ai_skill_add_remote"),
                        () => setState(() {
                          selectedTab = 1;
                          error = null;
                        }),
                      ),
                    ],
                  ),
                  const SizedBox(height: 16),
                  if (selectedTab == 0) ...[
                    Text(controller.tr("ui_ai_skill_add_local_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
                    const SizedBox(height: 12),
                    WoxPathFinder(
                      value: path,
                      enabled: true,
                      showOpenButton: false,
                      showChangeButton: true,
                      confirmOnChange: false,
                      changeButtonTextKey: 'ui_runtime_browse',
                      onChanged: (p) {
                        setState(() {
                          path = p;
                          error = null;
                        });
                      },
                    ),
                  ] else ...[
                    Text(controller.tr("ui_ai_skill_add_remote_hint"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
                    const SizedBox(height: 12),
                    WoxTextField(
                      controller: TextEditingController(text: remoteUrl),
                      hintText: 'https://github.com/user/repo',
                      width: 480,
                      onChanged: (v) {
                        remoteUrl = v;
                        error = null;
                      },
                    ),
                  ],
                  if (error != null) ...[const SizedBox(height: 8), Text(error!, style: TextStyle(color: Colors.red, fontSize: 13))],
                  if (isCloning) ...[
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2)),
                        const SizedBox(width: 8),
                        Text(controller.tr("ui_ai_skill_cloning"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
                      ],
                    ),
                  ],
                ],
              ),
              actions: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    WoxButton.secondary(text: controller.tr("ui_cancel"), onPressed: () => Navigator.pop(context)),
                    const SizedBox(width: 16),
                    WoxButton.primary(
                      text: controller.tr("ui_add"),
                      onPressed:
                          isCloning
                              ? null
                              : () async {
                                if (selectedTab == 0) {
                                  // Local skill
                                  if (path.trim().isEmpty) {
                                    setState(() {
                                      error = controller.tr("ui_ai_skill_add_path_required");
                                    });
                                    return;
                                  }
                                  final err = await saveRow({"Path": path});
                                  if (err != null) {
                                    setState(() {
                                      error = err;
                                    });
                                    return;
                                  }
                                  if (context.mounted) Navigator.pop(context);
                                } else {
                                  // Remote skill
                                  if (remoteUrl.trim().isEmpty) {
                                    setState(() {
                                      error = controller.tr("ui_ai_skill_add_url_required");
                                    });
                                    return;
                                  }
                                  setState(() {
                                    isCloning = true;
                                    error = null;
                                  });
                                  try {
                                    final traceId = const UuidV4().generate();
                                    final skills = await WoxApi.instance.cloneAISkill(traceId, remoteUrl.trim());
                                    if (skills.isEmpty) {
                                      setState(() {
                                        isCloning = false;
                                        error = controller.tr("ui_ai_skill_clone_no_skills");
                                      });
                                      return;
                                    }
                                    // Save all discovered skills at once. Calling saveRow
                                    // in a loop would overwrite previous saves because each
                                    // call reads the stale snapshot from when the dialog
                                    // was opened.
                                    final existingRows = json.decode(json.encode(controller.woxSetting.value.aiSkills)) as List;
                                    final allRows = <Map<String, dynamic>>[
                                      for (final r in existingRows) Map<String, dynamic>.from(r as Map),
                                      for (final skill in skills) skill.toJson(),
                                    ];
                                    await controller.updateConfig("AISkills", json.encode(allRows));
                                    if (context.mounted) Navigator.pop(context);
                                  } catch (e) {
                                    setState(() {
                                      isCloning = false;
                                      error = e.toString();
                                    });
                                  }
                                }
                              },
                    ),
                  ],
                ),
              ],
            );
          },
        );
      },
    );
  }

  Widget _buildAddSkillTab(BuildContext context, int index, int selectedIndex, String label, VoidCallback onTap) {
    final isSelected = index == selectedIndex;
    final textColor = getThemeTextColor();
    final bgColor = getThemeSettingDividerColor();
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: isSelected ? textColor.withAlpha(20) : Colors.transparent,
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: isSelected ? textColor.withAlpha(60) : bgColor),
        ),
        child: Text(label, style: TextStyle(color: isSelected ? textColor : textColor.withAlpha(120), fontSize: 13, fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal)),
      ),
    );
  }
}
