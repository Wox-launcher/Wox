import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
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
            ],
          );
        }
        return const SizedBox.shrink();
      },
    );
  }
}
