import 'wox_plugin_setting.dart';

class PluginSettingValueTextBox {
  late String key;
  late String label;
  late String suffix;
  late String defaultValue;
  late String tooltip;

  late PluginSettingValueStyle style;

  PluginSettingValueTextBox.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    suffix = json['Suffix'];
    defaultValue = json['DefaultValue'];
    tooltip = json['Tooltip'];

    if (json['Style'] != null) {
      style = PluginSettingValueStyle.fromJson(json['Style']);
    } else {
      style = PluginSettingValueStyle.fromJson(<String, dynamic>{});
    }
  }
}
