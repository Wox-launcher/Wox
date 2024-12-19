import 'package:wox/entity/validator/wox_setting_validator.dart';

class PluginSettingValidatorNotEmpty implements PluginSettingValidator {
  @override
  String validate(dynamic value) {
    if (value is String) {
      if (value.trim().isEmpty) {
        return "Value can not be empty";
      }
    }
    if (value is List) {
      if (value.isEmpty) {
        return "Value can not be empty";
      }

      if (value is List<String>) {
        //check every string
        for (var item in value) {
          if (item.trim().isEmpty) {
            return "Value can not be empty";
          }
        }
      }
    }

    return "";
  }

  PluginSettingValidatorNotEmpty.fromJson(Map<String, dynamic> json);
}
