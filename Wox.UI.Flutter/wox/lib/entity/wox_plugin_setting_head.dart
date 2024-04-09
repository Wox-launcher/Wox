class PluginSettingValueHead {
  late String content;

  PluginSettingValueHead.fromJson(Map<String, dynamic> json) {
    content = json['Content'];
  }
}
