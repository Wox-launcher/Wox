import 'package:flutter/material.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/components/wox_tooltip_icon_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/utils/colors.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginCheckbox extends WoxSettingPluginItem {
  final PluginSettingValueCheckBox item;

  const WoxSettingPluginCheckbox({super.key, required this.item, required super.value, required super.onUpdate});

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        label(item.label, item.style),
        if (item.tooltip != "") WoxTooltipIconView(tooltip: item.tooltip, paddingLeft: 0, color: getThemeTextColor()),
        WoxSwitch(
          value: getSetting(item.key) == "true",
          onChanged: (value) {
            if (value == true) {
              updateConfig(item.key, "true");
            } else {
              updateConfig(item.key, "false");
            }
          },
        ),
      ],
      style: item.style,
    );
  }
}
