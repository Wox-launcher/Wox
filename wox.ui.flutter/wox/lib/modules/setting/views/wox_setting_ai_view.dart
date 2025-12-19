import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxSettingAIView extends WoxSettingBaseView {
  const WoxSettingAIView({super.key});

  @override
  Widget build(BuildContext context) {
    return FutureBuilder(
      future: WoxApi.instance.findAIProviders(),
      builder: (context, snapshot) {
        if (snapshot.hasData) {
          return form(children: [
            formField(
              label: controller.tr("ui_ai_model"),
              child: Obx(() {
                return WoxSettingPluginTable(
                  value: json.encode(controller.woxSetting.value.aiProviders),
                  item: PluginSettingValueTable.fromJson({
                    "Key": "AIProviders",
                    "Columns": [
                      {
                        "Key": "Name",
                        "Label": "i18n:ui_ai_providers_name",
                        "Tooltip": "i18n:ui_ai_providers_name_tooltip",
                        "Width": 100,
                        "Type": "select",
                        "SelectOptions": snapshot.data!.map((e) => {"Label": e.name, "Value": e.name}).toList(),
                        "TextMaxLines": 1,
                        "Validators": [
                          {"Type": "not_empty"}
                        ],
                      },
                      {
                        "Key": "Alias",
                        "Label": "i18n:ui_ai_providers_alias",
                        "Tooltip": "i18n:ui_ai_providers_alias_tooltip",
                        "Width": 120,
                        "Type": "text",
                        "TextMaxLines": 1,
                      },
                      {
                        "Key": "ApiKey",
                        "Label": "i18n:ui_ai_providers_api_key",
                        "Tooltip": "i18n:ui_ai_providers_api_key_tooltip",
                        "Type": "text",
                        "TextMaxLines": 1,
                      },
                      {
                        "Key": "Host",
                        "Label": "i18n:ui_ai_providers_host",
                        "Tooltip": "i18n:ui_ai_providers_host_tooltip",
                        "Width": 160,
                        "Type": "text",
                      },
                      {
                        "Key": "Status",
                        "Label": "i18n:ui_ai_providers_status",
                        "HideInUpdate": true,
                        "Width": 60,
                        "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeAIModelStatus,
                      }
                    ],
                    "SortColumnKey": "Name"
                  }),
                  onUpdate: (key, value) {
                    controller.updateConfig("AIProviders", value);
                  },
                  onUpdateValidate: (rowValues) async {
                    if (rowValues["Name"] != "ollama") {
                      if (rowValues["ApiKey"] == null || rowValues["ApiKey"] == "") {
                        return controller.tr("ui_ai_providers_api_key_required");
                      }
                    }
                    return null;
                  },
                );
              }),
            ),
            formField(
              label: controller.tr("ui_ai_mcp_server_enable"),
              tips: controller.tr("ui_ai_mcp_server_enable_tips"),
              child: Obx(() {
                return WoxSwitch(
                  value: controller.woxSetting.value.enableMCPServer,
                  onChanged: (bool value) {
                    controller.updateConfig("EnableMCPServer", value.toString());
                  },
                );
              }),
            ),
            formField(
              label: controller.tr("ui_ai_mcp_server_port"),
              tips: controller.tr("ui_ai_mcp_server_port_tips"),
              child: Obx(() {
                final textColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor);
                final borderColor = textColor.withValues(alpha: 0.3);
                final focusBorderColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxCursorColor);
                return SizedBox(
                  width: 100,
                  child: TextField(
                    controller: TextEditingController(text: controller.woxSetting.value.mcpServerPort.toString()),
                    keyboardType: TextInputType.number,
                    inputFormatters: [FilteringTextInputFormatter.digitsOnly],
                    style: TextStyle(color: textColor, fontSize: 14),
                    decoration: InputDecoration(
                      isDense: true,
                      contentPadding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(4),
                        borderSide: BorderSide(color: borderColor),
                      ),
                      enabledBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(4),
                        borderSide: BorderSide(color: borderColor),
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(4),
                        borderSide: BorderSide(color: focusBorderColor),
                      ),
                    ),
                    onSubmitted: (value) {
                      final port = int.tryParse(value);
                      if (port != null && port > 0 && port < 65536) {
                        controller.updateConfig("MCPServerPort", value);
                      }
                    },
                  ),
                );
              }),
            ),
          ]);
        }
        return const SizedBox.shrink();
      },
    );
  }
}
