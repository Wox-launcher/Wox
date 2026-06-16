import 'dart:convert';

import 'package:wox/entity/wox_plugin_setting.dart';

class QueryRequirementSettingsPreviewRequirement {
  late String settingKey;
  late String message;

  QueryRequirementSettingsPreviewRequirement.fromJson(Map<String, dynamic> json) {
    settingKey = json['SettingKey']?.toString() ?? "";
    message = json['Message']?.toString() ?? "";
  }
}

class QueryRequirementSettingsPreviewData {
  late String pluginId;
  late String pluginName;
  late String title;
  late String message;
  late List<QueryRequirementSettingsPreviewRequirement> requirements;
  late List<PluginSettingDefinitionItem> settingDefinitions;
  late Map<String, String> values;

  QueryRequirementSettingsPreviewData.fromJson(Map<String, dynamic> json) {
    pluginId = json['PluginId']?.toString() ?? "";
    pluginName = json['PluginName']?.toString() ?? "";
    title = json['Title']?.toString() ?? "";
    message = json['Message']?.toString() ?? "";
    requirements = (json['Requirements'] as List<dynamic>? ?? []).map((item) => QueryRequirementSettingsPreviewRequirement.fromJson(Map<String, dynamic>.from(item))).toList();
    settingDefinitions = (json['SettingDefinitions'] as List<dynamic>? ?? []).map((item) => PluginSettingDefinitionItem.fromJson(Map<String, dynamic>.from(item))).toList();
    values = (json['Values'] as Map<String, dynamic>? ?? {}).map((key, value) => MapEntry(key, value?.toString() ?? ""));
  }

  factory QueryRequirementSettingsPreviewData.fromPreviewData(String previewData) {
    return QueryRequirementSettingsPreviewData.fromJson(Map<String, dynamic>.from(jsonDecode(previewData)));
  }
}
