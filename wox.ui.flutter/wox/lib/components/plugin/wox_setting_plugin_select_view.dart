import 'package:flutter/material.dart';
import 'package:wox/components/wox_tooltip_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/utils/colors.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginSelect extends WoxSettingPluginItem {
  final PluginSettingValueSelect item;

  const WoxSettingPluginSelect({super.key, required this.item, required super.value, required super.onUpdate});

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        label(item.label, item.style),
        if (item.tooltip != "") WoxTooltipView(tooltip: item.tooltip, paddingLeft: 0, color: getThemeTextColor()),
        DropdownButton<String>(
          value: getSetting(item.key),
          isExpanded: true,
          dropdownColor: getThemeCardBackgroundColor(),
          style: TextStyle(color: getThemeTextColor(), fontSize: 13),
          items: item.options.map((e) {
            return DropdownMenuItem(
              value: e.value,
              child: Text(e.label),
            );
          }).toList(),
          onChanged: (v) {
            updateConfig(item.key, v ?? "");
          },
        ),
        suffix(item.suffix),
      ],
      style: item.style,
    );
  }
}
