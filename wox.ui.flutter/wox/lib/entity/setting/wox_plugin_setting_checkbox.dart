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
    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();
  }
}
