import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/wox_plugin_setting_checkbox.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginCheckbox extends WoxSettingPluginItem {
  final PluginSettingValueCheckBox checkBox;

  const WoxSettingPluginCheckbox(this.checkBox, super.settings, super.onUpdate, {super.key, required});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Text(checkBox.label),
        const SizedBox(width: 10),
        ToggleSwitch(
          checked: getSetting(checkBox.key) == "true",
          onChanged: (value) {
            if (value == true) {
              updateConfig(checkBox.key, "true");
            } else {
              updateConfig(checkBox.key, "false");
            }
            onUpdate(key, value);
          },
        ),
      ],
    );
  }
}
