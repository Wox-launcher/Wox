class PluginSettingValueDictationHotkey {
  late String key;
  late String label;
  late String tooltip;
  late String defaultValue;

  PluginSettingValueDictationHotkey.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    tooltip = json['Tooltip'];
    defaultValue = json['DefaultValue'];
  }
}
