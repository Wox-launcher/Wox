import 'package:wox/entity/setting/wox_plugin_setting_table.dart';

import 'setting/wox_plugin_setting_checkbox.dart';
import 'setting/wox_plugin_setting_head.dart';
import 'setting/wox_plugin_setting_label.dart';
import 'setting/wox_plugin_setting_newline.dart';
import 'setting/wox_plugin_setting_select.dart';
import 'setting/wox_plugin_setting_select_ai_model.dart';
import 'setting/wox_plugin_setting_textbox.dart';

class PluginSettingDefinitionItem {
  late String type;
  late dynamic value;
  late List<String> disabledInPlatforms;
  late bool isPlatformSpecific;

  PluginSettingDefinitionItem.fromJson(Map<String, dynamic> json) {
    if (json['DisabledInPlatforms'] == null) {
      disabledInPlatforms = <String>[];
    } else {
      disabledInPlatforms = (json['DisabledInPlatforms'] as List).map((e) => e.toString()).toList();
    }
    isPlatformSpecific = json['IsPlatformSpecific'];
    type = json['Type'];

    if (type == "checkbox") {
      value = PluginSettingValueCheckBox.fromJson(json['Value']);
    } else if (type == "head") {
      value = PluginSettingValueHead.fromJson(json['Value']);
    } else if (type == "label") {
      value = PluginSettingValueLabel.fromJson(json['Value']);
    } else if (type == "newline") {
      value = PluginSettingValueNewLine.fromJson(<String, dynamic>{});
    } else if (type == "select") {
      value = PluginSettingValueSelect.fromJson(json['Value']);
    } else if (type == "selectAIModel") {
      value = PluginSettingValueSelectAIModel.fromJson(json['Value']);
    } else if (type == "table") {
      value = PluginSettingValueTable.fromJson(json['Value']);
    } else if (type == "textbox") {
      value = PluginSettingValueTextBox.fromJson(json['Value']);
    } else {
      throw Exception("Unknown setting type: $type");
    }
  }
}

class PluginSettingValueStyle {
  late double paddingLeft;
  late double paddingTop;
  late double paddingRight;
  late double paddingBottom;
  late double width;

  PluginSettingValueStyle.defaults() {
    paddingLeft = 0;
    paddingTop = 0;
    paddingRight = 0;
    paddingBottom = 0;
    width = 0;
  }

  PluginSettingValueStyle.fromJson(Map<String, dynamic> json) {
    // Plugin-provided pixel styling is deprecated and intentionally ignored so
    // old plugins stay loadable while Wox keeps setting pages visually consistent.
    paddingLeft = 0;
    paddingTop = 0;
    paddingRight = 0;
    paddingBottom = 0;
    width = 0;
  }
}
