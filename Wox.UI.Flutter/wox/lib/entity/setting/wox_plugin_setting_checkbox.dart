import '../wox_plugin_setting.dart';

class PluginSettingValueCheckBox {
  late String key;
  late String label;
  late String defaultValue;
  late String tooltip;
  late PluginSettingValueStyle style;

  PluginSettingValueCheckBox.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    defaultValue = json['DefaultValue'];
    tooltip = json['Tooltip'];
    if (json['Style'] != null) {
      style = PluginSettingValueStyle.fromJson(json['Style']);
    } else {
      style = PluginSettingValueStyle.fromJson(<String, dynamic>{});
    }
  }
}
