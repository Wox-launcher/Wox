import 'package:wox/entity/validator/wox_setting_validator.dart';

class PluginSettingValidatorIsNumber implements PluginSettingValidator {
  late bool isInteger;
  late bool isFloat;

  @override
  String validate(dynamic value) {
    if (value is! String) {
      return "invalid value";
    }

    if (isInteger) {
      if (int.tryParse(value) == null) {
        return "Value must be an integer";
      }
    } else if (isFloat) {
      if (double.tryParse(value) == null) {
        return "Value must be a number";
      }
    }
    return "";
  }

  PluginSettingValidatorIsNumber.fromJson(Map<String, dynamic> json) {
    isInteger = json['IsInteger'];
    isFloat = json['IsFloat'];
  }
}
