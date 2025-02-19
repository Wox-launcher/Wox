import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingAIView extends GetView<WoxSettingController> {
  const WoxSettingAIView({super.key});

  Widget form({required double width, required List<Widget> children}) {
    return Column(
      children: [
        ...children.map((e) => SizedBox(
              width: width,
              child: e,
            )),
      ],
    );
  }

  Widget formField({required String label, required Widget child, String? tips, double labelWidth = 100}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 18),
      child: Column(
        children: [
          Row(
            children: [
              Padding(
                padding: const EdgeInsets.only(right: 20),
                child: SizedBox(width: labelWidth, child: Text(label, textAlign: TextAlign.right)),
              ),
              child,
            ],
          ),
          if (tips != null)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Row(
                children: [
                  Padding(
                    padding: const EdgeInsets.only(right: 20),
                    child: SizedBox(width: labelWidth, child: const Text("")),
                  ),
                  Flexible(
                    child: Text(
                      tips,
                      style: TextStyle(color: Colors.grey[90], fontSize: 13),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      child: Padding(
          padding: const EdgeInsets.all(20),
          child: form(width: 1000, children: [
            formField(
              label: "AI Providers",
              child: Obx(() {
                return WoxSettingPluginTable(
                  value: json.encode(controller.woxSetting.value.aiProviders),
                  tableWidth: 750,
                  item: PluginSettingValueTable.fromJson({
                    "Key": "AIProviders",
                    "Columns": [
                      {
                        "Key": "Name",
                        "Label": "Provider Name",
                        "Tooltip": "The name of the AI provider.",
                        "Width": 120,
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
                        "Label": "API Key",
                        "Tooltip": "The API key of the AI provider.",
                        "Type": "text",
                        "TextMaxLines": 1,
                      },
                      {
                        "Key": "Host",
                        "Label": "Host",
                        "Tooltip": "The host of the AI provider.",
                        "Width": 200,
                        "Type": "text",
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
                        return "API key is required.";
                      }
                    }
                    if (rowValues["Name"] == "ollama") {
                      if (rowValues["Host"] == null || rowValues["Host"] == "") {
                        return "Host is required.";
                      }
                    }
                    return null;
                  },
                );
              }),
            ),
          ])),
    );
  }
}
