import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/wox_plugin_setting_head.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginHead extends WoxSettingPluginItem {
  final PluginSettingValueHead item;

  const WoxSettingPluginHead(super.plugin, this.item, super.onUpdate, {super.key, required});

  @override
  Widget build(BuildContext context) {
    // Add padding to the head if it doesn't have any
    if (item.style.paddingTop == 0 && item.style.paddingBottom == 0) {
      item.style.paddingTop = 4;
      item.style.paddingBottom = 4;
    }

    return layout(
      children: [
        Row(
          children: [
            Text(
              item.content,
              style: const TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.bold,
              ),
            ),
          ],
        ),
      ],
      style: item.style,
    );
  }
}
