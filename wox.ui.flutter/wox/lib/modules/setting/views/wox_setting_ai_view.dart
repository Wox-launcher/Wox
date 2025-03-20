import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

class WoxSettingAIView extends WoxSettingBaseView {
  const WoxSettingAIView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return form(children: [
        formField(
          label: controller.tr("ui_ai_model"),
          child: WoxSettingPluginTable(
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
                  "SelectOptions": [
                    {"Label": "OpenAI", "Value": "openai"},
                    {"Label": "Google", "Value": "google"},
                    {"Label": "Ollama", "Value": "ollama"},
                    {"Label": "Groq", "Value": "groq"},
                  ],
                  "TextMaxLines": 1,
                  "Validators": [
                    {"Type": "not_empty"}
                  ],
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
              if (rowValues["Name"] == "ollama") {
                if (rowValues["Host"] == null || rowValues["Host"] == "") {
                  return controller.tr("ui_ai_providers_host_required");
                }
              }
              return null;
            },
          ),
        ),
      ]);
    });
  }
}
