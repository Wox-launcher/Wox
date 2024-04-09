class PluginSettingValueLabel {
  late String content;

  PluginSettingValueLabel.fromJson(Map<String, dynamic> json) {
    content = json['Content'];
  }
}
