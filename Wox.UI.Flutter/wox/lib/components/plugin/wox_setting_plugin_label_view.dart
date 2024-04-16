import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/components/wox_tooltip_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginLabel extends WoxSettingPluginItem {
  final PluginSettingValueLabel item;

  const WoxSettingPluginLabel(super.plugin, this.item, super.onUpdate, {super.key, required});

  @override
  Widget build(BuildContext context) {
    return layout(children: [
      Text(item.content),
      if (item.tooltip != "") WoxTooltipView(tooltip: item.tooltip),
    ], style: item.style);
  }
}
