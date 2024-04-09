class PluginSettingValueCheckBox {
  late String key;
  late String label;
  late String defaultValue;

  PluginSettingValueCheckBox.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    defaultValue = json['DefaultValue'];
  }
}
