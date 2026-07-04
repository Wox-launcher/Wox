import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/consts.dart';

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
              _buildWebSearchSection(),
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
                child: Obx(() {
                  return WoxSettingPluginTable(
                    inlineTitleActions: true,
                    readonly: true,
                    tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                    value: json.encode(controller.woxSetting.value.aiSkills),
                    item: _buildSkillsTable(),
                    onUpdate: (key, value) async {
                      return null;
                    },
                  );
                }),
              ),
            ],
          );
        }
        return const SizedBox.shrink();
      },
    );
  }

  Widget _buildWebSearchSection() {
    return Obx(() {
      final config = controller.woxSetting.value.aiWebSearch;
      final endpointController = TextEditingController(text: config.endpoint);
      final apiKeyController = TextEditingController(text: config.apiKey);
      final resultCountController = TextEditingController(text: config.searchResultCount.toString());
      final fetchMaxController = TextEditingController(text: config.fetchMaxCharacters.toString());

      return settingTarget(
        settingKey: "AIWebSearch",
        child: formSection(
          title: controller.tr("ui_ai_web_search"),
          children: [
            formField(
              label: controller.tr("ui_ai_web_search_enabled"),
              tips: controller.tr("ui_ai_web_search_enabled_tips"),
              child: WoxSwitch(value: config.enabled, onChanged: (value) => _updateAIWebSearch(config.copyWith(enabled: value))),
            ),
            formField(
              label: controller.tr("ui_ai_web_search_provider"),
              tips: controller.tr("ui_ai_web_search_provider_tips"),
              child: WoxDropdownButton<String>(
                width: 300,
                value: config.provider,
                items: const [
                  WoxDropdownItem(value: "exa", label: "Exa"),
                  WoxDropdownItem(value: "tavily", label: "Tavily"),
                  WoxDropdownItem(value: "brave", label: "Brave"),
                  WoxDropdownItem(value: "searxng", label: "SearXNG"),
                ],
                onChanged: (value) {
                  if (value == null) {
                    return;
                  }
                  _updateAIWebSearch(config.copyWith(provider: value, endpoint: _defaultEndpointForProvider(value)));
                },
              ),
            ),
            formField(
              label: controller.tr("ui_ai_web_search_endpoint"),
              tips: controller.tr("ui_ai_web_search_endpoint_tips"),
              child: WoxTextField(
                width: 520,
                enabled: config.enabled,
                controller: endpointController,
                onChanged: (value) => _updateAIWebSearch(config.copyWith(endpoint: value.trim())),
              ),
            ),
            formField(
              label: controller.tr("ui_ai_web_search_api_key"),
              tips: controller.tr("ui_ai_web_search_api_key_tips"),
              child: WoxTextField(
                width: 420,
                enabled: config.enabled,
                controller: apiKeyController,
                obscureText: true,
                onChanged: (value) => _updateAIWebSearch(config.copyWith(apiKey: value.trim())),
              ),
            ),
            formField(
              label: controller.tr("ui_ai_web_search_result_count"),
              tips: controller.tr("ui_ai_web_search_result_count_tips"),
              child: WoxTextField(
                width: 120,
                enabled: config.enabled,
                controller: resultCountController,
                onChanged: (value) {
                  final parsed = int.tryParse(value.trim());
                  if (parsed != null) {
                    _updateAIWebSearch(config.copyWith(searchResultCount: parsed));
                  }
                },
              ),
            ),
            formField(
              label: controller.tr("ui_ai_web_search_fetch_max"),
              tips: controller.tr("ui_ai_web_search_fetch_max_tips"),
              child: WoxTextField(
                width: 120,
                enabled: config.enabled,
                controller: fetchMaxController,
                onChanged: (value) {
                  final parsed = int.tryParse(value.trim());
                  if (parsed != null) {
                    _updateAIWebSearch(config.copyWith(fetchMaxCharacters: parsed));
                  }
                },
              ),
            ),
          ],
        ),
      );
    });
  }

  void _updateAIWebSearch(AIWebSearchConfig config) {
    controller.updateConfig("AIWebSearch", jsonEncode(config.toJson()));
  }

  String _defaultEndpointForProvider(String provider) {
    switch (provider) {
      case "exa":
        return "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa";
      case "tavily":
        return "https://api.tavily.com";
      case "brave":
        return "https://api.search.brave.com";
      default:
        return "";
    }
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
        {"Key": "SourceName", "Label": "i18n:plugin_ai_chat_skill_source", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 90},
        {"Key": "Name", "Label": "i18n:plugin_ai_chat_skill_name", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 150},
        {"Key": "Description", "Label": "i18n:plugin_ai_chat_skill_description", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 260},
        {"Key": "ManifestPath", "Label": "i18n:plugin_ai_chat_skill_manifest_path", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 300},
        {"Key": "Enabled", "Label": "i18n:plugin_ai_chat_skill_enabled", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox, "Width": 70},
        {"Key": "Error", "Label": "i18n:plugin_ai_chat_skill_error", "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText, "Width": 160},
      ],
      "SortColumnKey": "SourceName",
    });
  }
}
