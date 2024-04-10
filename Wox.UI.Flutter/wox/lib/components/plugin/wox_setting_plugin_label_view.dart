import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/wox_plugin_setting_label.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginLabel extends WoxSettingPluginItem {
  final PluginSettingValueLabel item;

  const WoxSettingPluginLabel(super.plugin, this.item, super.onUpdate, {super.key, required});

  @override
  Widget build(BuildContext context) {
    return layout(children: [Text(item.content)], style: item.style);
  }
}
