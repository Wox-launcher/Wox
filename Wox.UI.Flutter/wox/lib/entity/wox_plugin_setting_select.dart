class PluginSettingValueSelect {
  late String key;
  late String label;
  late String suffix;
  late String defaultValue;
  late List<PluginSettingValueSelectOption> options;

  PluginSettingValueSelect.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    suffix = json['Suffix'];
    defaultValue = json['DefaultValue'];
    if (json['Options'] != null) {
      options = (json['Options'] as List).map((e) => PluginSettingValueSelectOption.fromJson(e)).toList();
    } else {
      options = [];
    }
  }
}

class PluginSettingValueSelectOption {
  late String label;
  late String value;

  PluginSettingValueSelectOption.fromJson(Map<String, dynamic> json) {
    label = json['Label'];
    value = json['Value'];
  }
}
