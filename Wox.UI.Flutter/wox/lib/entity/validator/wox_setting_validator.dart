import 'wox_setting_validator_is_number.dart';
import 'wox_setting_validator_not_empty.dart';

interface class PluginSettingValidator {
  validate(dynamic value) => String; //if validate success return empty string, else return error message
}

class PluginSettingValidatorItem {
  late String type;
  late PluginSettingValidator validator;

  PluginSettingValidatorItem.fromJson(Map<String, dynamic> json) {
    type = json['Type'];
    if (type == "not_empty") {
      validator = PluginSettingValidatorNotEmpty.fromJson(<String, dynamic>{});
    } else if (type == "is_number") {
      validator = PluginSettingValidatorIsNumber.fromJson(json['Value']);
    }
  }
}
