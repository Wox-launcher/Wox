import '../wox_plugin_setting.dart';

class PluginSettingValueNewLine {
  late PluginSettingValueStyle style;

  PluginSettingValueNewLine.fromJson(Map<String, dynamic> json) {
    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();
  }
}
