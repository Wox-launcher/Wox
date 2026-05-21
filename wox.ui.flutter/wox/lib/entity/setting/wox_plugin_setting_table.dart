import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/validator/wox_setting_validator.dart';
import 'package:wox/entity/wox_plugin_setting.dart';

class PluginSettingValueTable {
  static const int defaultMaxHeight = 300;
  late String key;
  late String defaultValue;
  late String title;
  late String tooltip;
  late List<PluginSettingValueTableColumn> columns;
  late String sortColumnKey; // The key of the column that should be used for sorting
  late String sortOrder; // asc or desc
  late int maxHeight; // Max table height in px, <= 0 means use default
  late int updateDialogWidth; // Optional add/update dialog content width, <= 0 means use the default adaptive width
  late PluginSettingValueStyle style;

  PluginSettingValueTable.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    defaultValue = json['DefaultValue'] ?? "";
    title = json['Title'] ?? "";
    tooltip = json['Tooltip'] ?? "";
    if (json['Columns'] != null) {
      columns = (json['Columns'] as List).map((e) => PluginSettingValueTableColumn.fromJson(e)).toList();
    } else {
      columns = [];
    }

    sortColumnKey = json['SortColumnKey'] ?? "";
    sortOrder = json['SortOrder'] ?? "asc";
    final rawMaxHeight = json['MaxHeight'];
    if (rawMaxHeight is num) {
      maxHeight = rawMaxHeight.toInt();
    } else {
      maxHeight = defaultMaxHeight;
    }
    if (maxHeight <= 0) {
      maxHeight = defaultMaxHeight;
    }

    // Dense table editors sometimes need more horizontal room than the shared
    // default can provide. Keep the field opt-in so existing tables continue to
    // use the adaptive width calculation unless their config asks for a wider
    // add/update dialog explicitly.
    final rawUpdateDialogWidth = json['UpdateDialogWidth'];
    if (rawUpdateDialogWidth is num) {
      updateDialogWidth = rawUpdateDialogWidth.toInt();
    } else {
      updateDialogWidth = 0;
    }

    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();
  }
}

class PluginSettingValueType {
  static const pluginSettingValueTableColumnTypeText = "text";
  static const pluginSettingValueTableColumnTypeTextList = "textList";
  static const pluginSettingValueTableColumnTypeCheckbox = "checkbox";
  static const pluginSettingValueTableColumnTypeDirPath = "dirPath";
  static const pluginSettingValueTableColumnTypeSelect = "select";
  static const pluginSettingValueTableColumnTypeSelectAIModel = "selectAIModel";
  static const pluginSettingValueTableColumnTypeAIModelStatus = "aiModelStatus";
  static const pluginSettingValueTableColumnTypeAIMCPServerTools = "aiMCPServerTools";
  static const pluginSettingValueTableColumnTypeAISelectMCPServerTools = "aiSelectMCPServerTools";
  static const pluginSettingValueTableColumnTypeWoxImage = "woxImage";
  static const pluginSettingValueTableColumnTypeHotkey = "hotkey";
  static const pluginSettingValueTableColumnTypeApp = "app";
}

class PluginSettingValueTableColumn {
  late String key;
  late String label;
  late String tooltip;
  late int width;
  late String type; //see PluginSettingValueType
  late List<PluginSettingValueSelectOption> selectOptions; // Only used when Type is PluginSettingValueTableColumnTypeSelect
  late int textMaxLines; // Only used when Type is PluginSettingValueTableColumnTypeText
  late bool hideInTable; // Hide this column in the table, but still show it in the setting dialog
  late bool hideInUpdate; // Hide this column in the update dialog
  late bool enableQueryVariablePicker; // Enable the dynamic query placeholder picker for text columns
  late List<PluginSettingValidatorItem> validators;
  late List<PluginSettingValueTableColumnChangeAction> onChangedActions;

  PluginSettingValueTableColumn.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'] ?? "";
    tooltip = json['Tooltip'] ?? "";
    width = json['Width'] ?? 0;
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
    hideInTable = json['HideInTable'] ?? false;
    hideInUpdate = json['HideInUpdate'] ?? false;
    // Query placeholder support is opt-in so the generic table editor keeps the
    // same behavior for normal text columns while query-template fields can
    // expose fast `{wox:...}` insertion.
    enableQueryVariablePicker = json['EnableQueryVariablePicker'] ?? false;

    if (json['Validators'] != null) {
      validators = (json['Validators'] as List).map((e) => PluginSettingValidatorItem.fromJson(e)).toList();
    } else {
      validators = [];
    }

    if (json['OnChangedActions'] != null) {
      onChangedActions = (json['OnChangedActions'] as List).map((e) => PluginSettingValueTableColumnChangeAction.fromJson(e)).toList();
    } else {
      onChangedActions = [];
    }
  }
}

class PluginSettingValueTableColumnChangeAction {
  // The field key that should receive the derived value.
  late String targetKey;

  // Read the value from the selected option's Extra map using this key.
  late String valueFromSelectedOptionExtra;

  // Controls when the target field can be overwritten: always or empty.
  late String overwriteMode;

  // Apply this action when a new row is initialized.
  late bool applyOnInit;

  PluginSettingValueTableColumnChangeAction.fromJson(Map<String, dynamic> json) {
    targetKey = json['TargetKey'] ?? "";
    valueFromSelectedOptionExtra = json['ValueFromSelectedOptionExtra'] ?? "";
    overwriteMode = json['OverwriteMode'] ?? "always";
    applyOnInit = json['ApplyOnInit'] ?? false;
  }
}

class PluginSettingTableValidationError {
  final String key;
  final String errorMsg;

  const PluginSettingTableValidationError({required this.key, required this.errorMsg});
}
