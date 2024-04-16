import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/setting/wox_plugin_setting_newline.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginNewLine extends WoxSettingPluginItem {
  final PluginSettingValueNewLine item;

  const WoxSettingPluginNewLine(super.plugin, this.item, super.onUpdate, {super.key, required});

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        const Padding(
          padding: EdgeInsets.all(4),
          child: Row(
            children: [
              SizedBox(
                width: 1,
              ),
            ],
          ),
        ),
      ],
      style: item.style,
    );
  }
}
