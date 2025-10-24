import 'package:flutter/material.dart';
import 'package:wox/components/wox_ai_model_selector_view.dart';
import 'package:wox/components/wox_tooltip_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/utils/colors.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginSelectAIModel extends WoxSettingPluginItem {
  final PluginSettingValueSelectAIModel item;

  const WoxSettingPluginSelectAIModel({super.key, required this.item, required super.value, required super.onUpdate});

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        label(item.label, item.style),
        if (item.tooltip != "") WoxTooltipView(tooltip: item.tooltip, paddingLeft: 0, color: getThemeTextColor()),
        Padding(
          padding: const EdgeInsets.only(top: 6),
          child: Expanded(
            child: WoxAIModelSelectorView(
              initialValue: value,
              onModelSelected: (modelJson) {
                updateConfig(item.key, modelJson);
              },
            ),
          ),
        ),
        suffix(item.suffix),
      ],
      style: item.style,
    );
  }
}
