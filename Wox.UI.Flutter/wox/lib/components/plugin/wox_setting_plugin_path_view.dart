import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/components/wox_path_finder_view.dart';
import 'package:wox/components/wox_tooltip_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_path.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginPath extends WoxSettingPluginItem {
  final PluginSettingValuePath item;
  final controller = TextEditingController();

  WoxSettingPluginPath({super.key, required this.item, required super.value, required super.onUpdate}) {
    controller.text = getSetting(item.key);
  }

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        label(item.label, item.style),
        if (item.tooltip != "") WoxTooltipView(tooltip: item.tooltip, paddingLeft: 0),
        SizedBox(
          width: item.style.width > 0 ? item.style.width.toDouble() : 100,
          child: Focus(
            onFocusChange: (hasFocus) {
              if (!hasFocus) {
                for (var element in item.validators) {
                  var errMsg = element.validator.validate(controller.text);
                  item.tooltip = errMsg;
                  if (errMsg != "") {
                    return;
                  }
                }

                updateConfig(item.key, controller.text);
              }
            },
            child: WoxPathFinder(
              path: controller.text,
              showOpenButton: false,
              showChangeButton: true,
              onChanged: (value) {
                controller.text = value;

                for (var element in item.validators) {
                  var errMsg = element.validator.validate(value);
                  item.tooltip = errMsg;
                  break;
                }
              },
            ),
          ),
        ),
        suffix(item.suffix),
      ],
      style: item.style,
    );
  }
}
