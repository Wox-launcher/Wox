import 'package:wox/entity/validator/wox_setting_validator.dart';

class PluginSettingValidatorUnique implements PluginSettingValidator {
  @override
  String validate(dynamic value, {PluginSettingValidationContext? context}) {
    if (context == null || context.tableColumnKey.isEmpty) {
      return "";
    }

    final normalizedValue = _normalize(value);
    if (normalizedValue.isEmpty) {
      return "";
    }

    var matchCount = 0;
    for (final row in context.tableRows) {
      if (_normalize(row[context.tableColumnKey]) == normalizedValue) {
        matchCount++;
      }
    }

    if (_normalize(context.originalTableRow[context.tableColumnKey]) == normalizedValue && matchCount > 0) {
      matchCount--;
    }

    return matchCount > 0 ? "i18n:ui_validator_value_must_be_unique" : "";
  }

  String _normalize(dynamic value) {
    return value?.toString().trim().toLowerCase() ?? "";
  }

  PluginSettingValidatorUnique.fromJson(Map<String, dynamic> json);
}
