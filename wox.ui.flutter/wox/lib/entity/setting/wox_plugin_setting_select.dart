import 'package:wox/entity/validator/wox_setting_validator.dart';
import 'package:wox/entity/wox_image.dart';

import '../wox_plugin_setting.dart';

class PluginSettingValueSelect {
  late String key;
  late String label;
  late String suffix;
  late String defaultValue;
  late String tooltip;
  late bool isMulti;
  late List<PluginSettingValueSelectOption> options;
  late PluginSettingValueStyle style;
  late List<PluginSettingValidatorItem> validators;

  PluginSettingValueSelect.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    suffix = json['Suffix'];
    defaultValue = json['DefaultValue'];
    tooltip = json['Tooltip'];
    isMulti = json['IsMulti'] ?? false;
    if (json['Options'] != null) {
      options = (json['Options'] as List).map((e) => PluginSettingValueSelectOption.fromJson(e)).toList();
    } else {
      options = [];
    }

    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();

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
  late WoxImage icon;
  late bool isSelectAll;
  late Map<String, dynamic> extra;

  PluginSettingValueSelectOption({required this.label, required this.value, required this.icon, this.isSelectAll = false, Map<String, dynamic>? extra}) : extra = extra ?? {};

  PluginSettingValueSelectOption.fromJson(Map<String, dynamic> json) {
    label = json['Label'] ?? "";
    value = json['Value'] ?? "";
    icon = json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : WoxImage.empty();
    isSelectAll = json['IsSelectAll'] ?? false;
    extra = json['Extra'] is Map ? Map<String, dynamic>.from(json['Extra']) : <String, dynamic>{};
  }
}
