import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/wox_plugin_setting_textbox.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginTextBox extends WoxSettingPluginItem {
  final PluginSettingValueTextBox item;

  const WoxSettingPluginTextBox(this.item, super.settings, super.onUpdate, {super.key, required});

  @override
  Widget build(BuildContext context) {
    return TextBox(
      controller: TextEditingController(text: getSetting(item.key)),
      onChanged: (value) {
        updateConfig(item.key, value);
      },
    );
  }
}
