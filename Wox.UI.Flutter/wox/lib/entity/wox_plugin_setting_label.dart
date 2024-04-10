import 'wox_plugin_setting.dart';

class PluginSettingValueLabel {
  late String content;
  late PluginSettingValueStyle style;

  PluginSettingValueLabel.fromJson(Map<String, dynamic> json) {
    content = json['Content'];
    if (json['Style'] != null) {
      style = PluginSettingValueStyle.fromJson(json['Style']);
    } else {
      style = PluginSettingValueStyle.fromJson(<String, dynamic>{});
    }
  }
}
