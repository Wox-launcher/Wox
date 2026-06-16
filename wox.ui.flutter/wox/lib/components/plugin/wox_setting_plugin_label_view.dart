import 'package:flutter/material.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/utils/colors.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginLabel extends WoxSettingPluginItem {
  final PluginSettingValueLabel item;

  const WoxSettingPluginLabel({super.key, required this.item, required super.value, required super.onUpdate, required super.labelWidth});

  PluginSettingValueStyle buildEffectiveStyle() {
    final style = PluginSettingValueStyle.fromJson(<String, dynamic>{});
    style.paddingLeft = item.style.paddingLeft;
    style.paddingTop = item.style.paddingTop;
    style.paddingRight = item.style.paddingRight;
    style.paddingBottom = item.style.paddingBottom;
    style.width = item.style.width;

    if (item.reserveLabelSpace) {
      style.paddingLeft += labelWidth + WoxSettingPluginItem.defaultLabelGap;
    }

    return style;
  }

  @override
  Widget build(BuildContext context) {
    return layout(
      label: "",
      child: Text(item.content, style: TextStyle(color: getThemeTextColor(), fontSize: 13)),
      style: buildEffectiveStyle(),
      tooltip: item.tooltip,
      includeBottomSpacing: false,
    );
  }
}
