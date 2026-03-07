import 'package:wox/entity/validator/wox_setting_validator.dart';

class PluginSettingValidatorNotEmpty implements PluginSettingValidator {
  @override
  String validate(dynamic value) {
    if (value is String) {
      if (value.trim().isEmpty) {
        return "i18n:ui_validator_value_can_not_be_empty";
      }
    }
    if (value is List) {
      if (value.isEmpty) {
        return "i18n:ui_validator_value_can_not_be_empty";
      }

      if (value is List<String>) {
        //check every string
        for (var item in value) {
          if (item.trim().isEmpty) {
            return "i18n:ui_validator_value_can_not_be_empty";
          }
        }
      }
    }

    return "";
  }

  PluginSettingValidatorNotEmpty.fromJson(Map<String, dynamic> json);
}
