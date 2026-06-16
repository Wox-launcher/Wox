import 'package:flutter/material.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/utils/colors.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginHead extends WoxSettingPluginItem {
  final PluginSettingValueHead item;

  const WoxSettingPluginHead({super.key, required this.item, required super.value, required super.onUpdate, required super.labelWidth});

  @override
  Widget build(BuildContext context) {
    // Add padding to the head if it doesn't have any
    if (item.style.paddingTop == 0 && item.style.paddingBottom == 0) {
      item.style.paddingTop = 4;
      item.style.paddingBottom = 4;
    }

    return layout(
      label: "",
      child: Row(children: [Text(item.content, style: TextStyle(fontSize: 16, color: getThemeTextColor()))]),
      style: item.style,
      tooltip: item.tooltip,
      includeBottomSpacing: false,
    );
  }
}
