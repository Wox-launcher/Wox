import 'package:wox/entity/validator/wox_setting_validator.dart';

import '../wox_plugin_setting.dart';

class PluginSettingValueSelect {
  late String key;
  late String label;
  late String suffix;
  late String defaultValue;
  late String tooltip;
  late List<PluginSettingValueSelectOption> options;
  late PluginSettingValueStyle style;
  late List<PluginSettingValidatorItem> validators;

  PluginSettingValueSelect.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    suffix = json['Suffix'];
    defaultValue = json['DefaultValue'];
    tooltip = json['Tooltip'];
    if (json['Options'] != null) {
      options = (json['Options'] as List).map((e) => PluginSettingValueSelectOption.fromJson(e)).toList();
    } else {
      options = [];
    }

    if (json['Style'] != null) {
      style = PluginSettingValueStyle.fromJson(json['Style']);
    } else {
      style = PluginSettingValueStyle.fromJson(<String, dynamic>{});
    }

    if (json['Validators'] != null) {
      validators = (json['Validators'] as List).map((e) => PluginSettingValidatorItem.fromJson(e)).toList();
    } else {
      validators = [];
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
