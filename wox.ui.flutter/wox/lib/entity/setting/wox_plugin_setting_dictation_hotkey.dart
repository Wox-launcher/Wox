import '../wox_plugin_setting.dart';

class PluginSettingValueDictationHotkey {
  late String key;
  late String label;
  late String tooltip;
  late String defaultValue;
  late PluginSettingValueStyle style;

  PluginSettingValueDictationHotkey.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    tooltip = json['Tooltip'];
    defaultValue = json['DefaultValue'];
    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();
  }
}
