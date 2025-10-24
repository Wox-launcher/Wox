import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get_state_manager/src/rx_flutter/rx_obx_widget.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

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
                        "Key": "ApiKey",
                        "Label": "i18n:ui_ai_providers_api_key",
                        "Tooltip": "i18n:ui_ai_providers_api_key_tooltip",
                        "Type": "text",
                        "TextMaxLines": 1,
                        "Width": 250,
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
          ]);
        }
        return const SizedBox.shrink();
      },
    );
  }
}
