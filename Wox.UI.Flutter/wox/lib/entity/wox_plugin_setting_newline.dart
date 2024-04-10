import 'wox_plugin_setting.dart';

class PluginSettingValueNewLine {
  late PluginSettingValueStyle style;

  PluginSettingValueNewLine.fromJson(Map<String, dynamic> json) {
    if (json['Style'] != null) {
      style = PluginSettingValueStyle.fromJson(json['Style']);
    } else {
      style = PluginSettingValueStyle.fromJson(<String, dynamic>{});
    }
  }
}
