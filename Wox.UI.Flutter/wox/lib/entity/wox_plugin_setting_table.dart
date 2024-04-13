import 'package:uuid/v4.dart';

import 'wox_plugin_setting.dart';
import 'wox_plugin_setting_select.dart';

class PluginSettingValueTable {
  late String key;
  late String defaultValue;
  late String title;
  late String tooltip;
  late List<PluginSettingValueTableColumn> columns;
  late PluginSettingValueStyle style;

  PluginSettingValueTable.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    defaultValue = json['DefaultValue'];
    title = json['Title'] ?? "";
    tooltip = json['Tooltip'] ?? "";
    if (json['Columns'] != null) {
      columns = (json['Columns'] as List).map((e) => PluginSettingValueTableColumn.fromJson(e)).toList();
    } else {
      columns = [];
    }

    if (json['Style'] != null) {
      style = PluginSettingValueStyle.fromJson(json['Style']);
    } else {
      style = PluginSettingValueStyle.fromJson(<String, dynamic>{});
    }
  }
}

class PluginSettingValueType {
  static const pluginSettingValueTableColumnTypeText = "text";
  static const pluginSettingValueTableColumnTypeTextList = "textList";
  static const pluginSettingValueTableColumnTypeCheckbox = "checkbox";
  static const pluginSettingValueTableColumnTypeDirPath = "dirPath";
  static const pluginSettingValueTableColumnTypeSelect = "select";
  static const pluginSettingValueTableColumnTypeWoxImage = "woxImage";
}

class PluginSettingValueTableColumn {
  late String key;
  late String label;
  late String tooltip;
  late int width;
  late String type; //see PluginSettingValueType
  late List<PluginSettingValueSelectOption> selectOptions; // Only used when Type is PluginSettingValueTableColumnTypeSelect
  late int textMaxLines; // Only used when Type is PluginSettingValueTableColumnTypeText

  PluginSettingValueTableColumn.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    tooltip = json['Tooltip'];
    width = json['Width'];
    type = json['Type'];
    if (json['SelectOptions'] != null) {
      selectOptions = (json['SelectOptions'] as List).map((e) => PluginSettingValueSelectOption.fromJson(e)).toList();
    } else {
      selectOptions = [];
    }
    textMaxLines = json['TextMaxLines'] ?? 1;
    if (textMaxLines < 1) {
      textMaxLines = 1;
    }
  }
}
