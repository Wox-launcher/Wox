import '../wox_plugin_setting.dart';

class PluginSettingValueHead {
  late String content;
  late String tooltip;
  late PluginSettingValueStyle style;

  PluginSettingValueHead.fromJson(Map<String, dynamic> json) {
    content = json['Content'];
    tooltip = json['Tooltip'];
    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();
  }
}
