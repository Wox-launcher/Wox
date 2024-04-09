class PluginSettingValueTextBox {
  late String key;
  late String label;
  late String suffix;
  late String defaultValue;
  late int width;

  PluginSettingValueTextBox.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    suffix = json['Suffix'];
    defaultValue = json['DefaultValue'];
    width = json['Width'];
  }
}
