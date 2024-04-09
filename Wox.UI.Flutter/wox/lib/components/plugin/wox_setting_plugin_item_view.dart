import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/entity/wox_plugin.dart';
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
}
