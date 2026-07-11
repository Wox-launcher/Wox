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
      item.style.paddingTop = 8;
      item.style.paddingBottom = 4;
    }

    return layout(
      label: "",
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(item.content, style: TextStyle(fontSize: 16, color: getThemeTextColor())),
          Padding(padding: EdgeInsets.only(top: 6), child: Container(height: 1, color: getThemeSettingDividerColor())),
        ],
      ),
      style: item.style,
      tooltip: item.tooltip,
      includeBottomSpacing: false,
    );
  }
}
