import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

abstract class WoxSettingPluginItem extends StatelessWidget {
  final PluginDetail plugin;
  final Function onUpdate;

  const WoxSettingPluginItem(this.plugin, this.onUpdate, {super.key});

  void updateConfig(String key, String value) {
    Get.find<WoxSettingController>().updatePluginSetting(plugin.id, key, value);
    onUpdate(key, value);
  }

  String getSetting(String key) {
    return plugin.setting.settings[key] ?? "";
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

  Widget withFlexible(List<Widget> children) {
    return Wrap(
      crossAxisAlignment: WrapCrossAlignment.center,
      children: children,
    );
  }
}
