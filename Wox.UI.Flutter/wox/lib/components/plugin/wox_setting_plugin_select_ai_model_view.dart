import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_tooltip_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginSelectAIModel extends WoxSettingPluginItem {
  final PluginSettingValueSelectAIModel item;

  const WoxSettingPluginSelectAIModel({super.key, required this.item, required super.value, required super.onUpdate});

  Future<List<PluginSettingValueSelectOption>> getSelectionAIModelOptions() async {
    final models = await WoxApi.instance.findAIModels();
    return models.map((e) {
      return PluginSettingValueSelectOption(value: jsonEncode(e), label: "${e.provider} - ${e.name}");
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        label(item.label, item.style),
        if (item.tooltip != "") WoxTooltipView(tooltip: item.tooltip, paddingLeft: 0),
        FutureBuilder(
          future: getSelectionAIModelOptions(),
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.done) {
              return ComboBox<String>(
                value: value,
                onChanged: (value) {
                  updateConfig(item.key, value ?? "");
                },
                items: snapshot.data?.map((e) {
                  return ComboBoxItem(
                    value: e.value,
                    child: Text(e.label),
                  );
                }).toList(),
              );
            } else {
              return const SizedBox();
            }
          },
        ),
        suffix(item.suffix),
      ],
      style: item.style,
    );
  }
}
