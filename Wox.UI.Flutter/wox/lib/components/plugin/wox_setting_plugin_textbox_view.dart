import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/wox_plugin_setting_textbox.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginTextBox extends WoxSettingPluginItem {
  final PluginSettingValueTextBox item;
  final controller = TextEditingController();

  WoxSettingPluginTextBox(super.plugin, this.item, super.onUpdate, {super.key, required}) {
    controller.text = getSetting(item.key);
  }

  @override
  Widget build(BuildContext context) {
    return Flexible(
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          if (item.label != "") Text(item.label),
          Padding(
            padding: const EdgeInsets.only(left: 3, right: 3),
            child: SizedBox(
              width: item.width > 0 ? item.width.toDouble() : 100,
              child: Focus(
                onFocusChange: (hasFocus) {
                  if (!hasFocus) {
                    updateConfig(item.key, controller.text);
                  }
                },
                child: TextBox(
                  controller: controller,
                ),
              ),
            ),
          ),
          if (item.suffix != "") Text(item.suffix),
        ],
      ),
    );
  }
}
