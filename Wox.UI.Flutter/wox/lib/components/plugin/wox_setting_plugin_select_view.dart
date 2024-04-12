import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/wox_plugin_setting_select.dart';
import 'package:wox/entity/wox_plugin_setting_textbox.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginSelect extends WoxSettingPluginItem {
  final PluginSettingValueSelect item;

  const WoxSettingPluginSelect(super.plugin, this.item, super.onUpdate, {super.key, required});

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        label(item.label, item.style),
        ComboBox<String>(
          value: getSetting(item.key),
          items: item.options.map((e) {
            return ComboBoxItem(
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
