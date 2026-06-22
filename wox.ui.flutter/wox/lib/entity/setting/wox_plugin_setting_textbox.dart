import 'package:wox/entity/validator/wox_setting_validator.dart';

import '../wox_plugin_setting.dart';

class PluginSettingValueTextBox {
  late String key;
  late String label;
  late String suffix;
  late String defaultValue;
  late String tooltip;
  late int maxLines;
  late List<PluginSettingValidatorItem> validators;

  late PluginSettingValueStyle style;

  PluginSettingValueTextBox.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    suffix = json['Suffix'];
    defaultValue = json['DefaultValue'];
    tooltip = json['Tooltip'];
    maxLines = json['MaxLines'] ?? 1;

    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();

    if (json['Validators'] != null) {
      validators = (json['Validators'] as List).map((e) => PluginSettingValidatorItem.fromJson(e)).toList();
    } else {
      validators = [];
    }
  }
}
