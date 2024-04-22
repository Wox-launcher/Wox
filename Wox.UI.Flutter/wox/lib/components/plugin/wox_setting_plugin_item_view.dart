import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

abstract class WoxSettingPluginItem extends StatelessWidget {
  final PluginDetail plugin;
  final Function onUpdate;

  const WoxSettingPluginItem(this.plugin, this.onUpdate, {super.key});

  Future<void> updateConfig(String key, String value) async {
    onUpdate(key, value);
  }

  String getSetting(String key) {
    return plugin.setting.settings[key] ?? "";
  }

  Widget withFlexible(List<Widget> children) {
    return Wrap(
      crossAxisAlignment: WrapCrossAlignment.center,
      children: children,
    );
  }

  Widget layout({required List<Widget> children, required PluginSettingValueStyle style}) {
    if (style.hasAnyPadding()) {
      return Padding(
        padding: EdgeInsets.only(
          top: style.paddingTop,
          bottom: style.paddingBottom,
          left: style.paddingLeft,
          right: style.paddingRight,
        ),
        child: withFlexible(children),
      );
    }

    return withFlexible(children);
  }

  Widget label(String text, PluginSettingValueStyle style) {
    if (text != "") {
      if (style.labelWidth > 0) {
        return Padding(
          padding: const EdgeInsets.only(right: 4),
          child: SizedBox(
            width: style.labelWidth,
            child: Text(text, style: const TextStyle(overflow: TextOverflow.ellipsis), textAlign: TextAlign.right),
          ),
        );
      } else {
        return Padding(
          padding: const EdgeInsets.only(right: 4),
          child: Text(text),
        );
      }
    }

    return const SizedBox.shrink();
  }

  Widget suffix(String text) {
    if (text != "") {
      return Padding(
        padding: const EdgeInsets.only(left: 4),
        child: Text(text),
      );
    }

    return const SizedBox.shrink();
  }
}
