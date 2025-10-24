import 'package:flutter/material.dart';
import 'package:wox/components/wox_tooltip_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/utils/colors.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginLabel extends WoxSettingPluginItem {
  final PluginSettingValueLabel item;

  const WoxSettingPluginLabel({super.key, required this.item, required super.value, required super.onUpdate});

  @override
  Widget build(BuildContext context) {
    return layout(children: [
      Text(item.content, style: TextStyle(color: getThemeTextColor(), fontSize: 13)),
      if (item.tooltip != "")
        WoxTooltipView(
          tooltip: item.tooltip,
          color: getThemeTextColor(),
        ),
    ], style: item.style);
  }
}
