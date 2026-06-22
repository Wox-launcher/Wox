import '../wox_plugin_setting.dart';

class PluginSettingValueLabel {
  late String content;
  late String tooltip;
  late bool reserveLabelSpace;
  late PluginSettingValueStyle style;

  PluginSettingValueLabel.fromJson(Map<String, dynamic> json) {
    content = json['Content'];
    tooltip = json['Tooltip'];
    reserveLabelSpace = json['ReserveLabelSpace'] ?? false;
    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();
  }
}
