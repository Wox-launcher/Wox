import 'wox_setting_validator_is_number.dart';
import 'wox_setting_validator_not_empty.dart';
import 'wox_setting_validator_unique.dart';

abstract interface class PluginSettingValidator {
  String validate(dynamic value, {PluginSettingValidationContext? context}); //if validate success return empty string, else return error message
}

class PluginSettingValidationContext {
  final List<Map<String, dynamic>> tableRows;
  final Map<String, dynamic> originalTableRow;
  final String tableColumnKey;

  const PluginSettingValidationContext({this.tableRows = const [], this.originalTableRow = const {}, this.tableColumnKey = ""});
}

class PluginSettingValidators {
  static String validateAll(dynamic value, List<PluginSettingValidatorItem> validators, {PluginSettingValidationContext? context}) {
    for (final validator in validators) {
      final errorMessage = validator.validator.validate(value, context: context);
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
    } else if (type == "unique") {
      validator = PluginSettingValidatorUnique.fromJson(json["Value"] ?? <String, dynamic>{});
    }
  }
}
