import 'wox_setting_validator_is_number.dart';
import 'wox_setting_validator_not_empty.dart';

abstract interface class PluginSettingValidator {
  String validate(dynamic value); //if validate success return empty string, else return error message
}

class PluginSettingValidators {
  static String validateAll(dynamic value, List<PluginSettingValidatorItem> validators) {
    for (final validator in validators) {
      final errorMessage = validator.validator.validate(value);
      if (errorMessage.trim().isNotEmpty) {
        return errorMessage;
      }
    }

    return "";
  }
}

class PluginSettingValidatorItem {
  late String type;
  late PluginSettingValidator validator;

  PluginSettingValidatorItem.fromJson(Map<String, dynamic> json) {
    type = json['Type'];
    if (type == "not_empty") {
      validator = PluginSettingValidatorNotEmpty.fromJson(<String, dynamic>{});
    } else if (type == "is_number") {
      validator = PluginSettingValidatorIsNumber.fromJson(json["Value"]);
    }
  }
}
