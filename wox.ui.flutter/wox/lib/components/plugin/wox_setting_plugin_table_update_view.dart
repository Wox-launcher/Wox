import 'dart:convert';
import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_ai_command_default_action_dropdown.dart';
import 'package:wox/components/wox_ai_model_selector_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_app_selector.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_image_selector.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_query_variable_textfield.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/components/wox_checkbox_tile.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/components/wox_path_finder.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/validator/wox_setting_validator.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_text_measure_util.dart';
import 'package:get/get.dart';

class WoxSettingPluginTableUpdate extends StatefulWidget {
  final PluginSettingValueTable item;
  final Map<String, dynamic> row;
  final List<Map<String, dynamic>> existingRows;
  final Map<String, dynamic> originalRow;
  final Future<String?> Function(String key, Map<String, dynamic> row) onUpdate;
  final Future<List<PluginSettingTableValidationError>> Function(Map<String, dynamic> rowValues)? onUpdateValidate;

  const WoxSettingPluginTableUpdate({
    super.key,
    required this.item,
    required this.row,
    required this.onUpdate,
    this.existingRows = const [],
    this.originalRow = const {},
    this.onUpdateValidate,
  });

  @override
  State<WoxSettingPluginTableUpdate> createState() => _WoxSettingPluginTableUpdateState();
}

class _WoxSettingPluginTableUpdateState extends State<WoxSettingPluginTableUpdate> {
  Map<String, dynamic> values = {};
  bool isUpdate = false;
  bool isSaving = false;
  String saveErrorMessage = "";
  Map<String, String> fieldValidationErrors = {};
  Map<String, TextEditingController> textboxEditingController = {};
  final Map<String, FocusNode> _focusNodes = {};
  List<PluginSettingValueTableColumn> columns = [];
  final Set<String> _customValidationErrorKeys = {};
  bool _isEscapeClosePending = false;

  // Store tool list to avoid repeated requests
  List<AIMCPTool> allMCPTools = [];
  bool isLoadingTools = true;
  List<AISkill> allAISkills = [];
  bool isLoadingSkills = true;

  bool _isTextEditingColumn(PluginSettingValueTableColumn column) {
    return column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeText ||
        column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeQueryHotkeyQuery ||
        column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAICommandPrompt;
  }

  // Detects the AI command default action select without coupling the generic table editor to the AI plugin id.
  bool _isAICommandDefaultActionColumn(PluginSettingValueTableColumn column) {
    if (column.key != "defaultAction") {
      return false;
    }

    final values = column.selectOptions.map((option) => option.value).toSet();
    return values.contains(AICommandDefaultActionValue.run) && values.contains(AICommandDefaultActionValue.runAndShow) && values.contains(AICommandDefaultActionValue.runAndPaste);
  }

  @override
  void initState() {
    super.initState();

    for (var element in widget.item.columns) {
      if (!element.hideInUpdate) {
        columns.add(element);
      }
    }

    // Check if there are any tool list type columns, if so, preload the tool list
    if (columns.any((column) => column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectMCPServerTools)) {
      _loadAllTools();
    }
    if (columns.any((column) => column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectSkills)) {
      _loadAllSkills();
    }

    widget.row.forEach((key, value) {
      values[key] = value;
    });

    if (values.isEmpty) {
      for (var column in columns) {
        if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeTextList) {
          values[column.key] = [];
        } else if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectMCPServerTools ||
            column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectSkills) {
          values[column.key] = [];
        } else if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox) {
          values[column.key] = false;
        } else if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeSelect) {
          if (column.selectOptions.isNotEmpty) {
            values[column.key] = column.selectOptions.first.value;
          } else {
            values[column.key] = "";
          }
        } else if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeWoxImage) {
          values[column.key] = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖");
        } else if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeApp) {
          values[column.key] = IgnoredHotkeyApp.empty().toJson();
        } else {
          values[column.key] = "";
        }
      }
    } else {
      isUpdate = true;
    }

    for (var column in columns) {
      // init text box controller and focus node
      if (_isTextEditingColumn(column)) {
        textboxEditingController[column.key] = TextEditingController(text: getValue(column.key));
        _focusNodes[column.key] = FocusNode();
        _focusNodes[column.key]!.addListener(() {
          if (!_focusNodes[column.key]!.hasFocus) {
            setFieldValidationError(column.key, validateValue(textboxEditingController[column.key]!.text, column));
            setState(() {});
          }
        });
      }
      // init text box controller for text list
      if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeTextList) {
        var columnValues = getValue(column.key);
        if (columnValues is String && columnValues == "") {
          columnValues = [];
        }
        if (columnValues is List) {
          columnValues = columnValues.map((e) => e.toString()).toList();
        }
        updateValue(column.key, columnValues);

        for (var i = 0; i < columnValues.length; i++) {
          textboxEditingController[column.key + i.toString()] = TextEditingController(text: columnValues[i]);
          _focusNodes[column.key + i.toString()] = FocusNode();
          final focusKey = column.key + i.toString();
          _focusNodes[focusKey]!.addListener(() {
            if (!_focusNodes[focusKey]!.hasFocus) {
              setFieldValidationError(column.key, validateValue(columnValues, column));
              setState(() {});
            }
          });
        }
      }
    }

    if (!isUpdate) {
      for (final column in columns) {
        applyColumnInitActions(column);
      }
    }
  }

  @override
  void dispose() {
    for (var controller in textboxEditingController.values) {
      controller.dispose();
    }
    for (var node in _focusNodes.values) {
      node.dispose();
    }
    super.dispose();
  }

  dynamic getValue(String key) {
    return values[key] ?? "";
  }

  PluginSettingValueTableColumn? getColumn(String key) {
    for (final column in columns) {
      if (column.key == key) {
        return column;
      }
    }

    return null;
  }

  bool getValueBool(String key) {
    if (values[key] == null) {
      return false;
    }
    if (values[key] is bool) {
      return values[key];
    }
    if (values[key] is String) {
      return values[key] == "true";
    }

    return false;
  }

  IgnoredHotkeyApp getIgnoredHotkeyAppValue(String key) {
    final rawValue = values[key];
    if (rawValue is IgnoredHotkeyApp) {
      return rawValue;
    }
    if (rawValue is Map<String, dynamic>) {
      return IgnoredHotkeyApp.fromJson(rawValue);
    }
    if (rawValue is Map) {
      return IgnoredHotkeyApp.fromJson(Map<String, dynamic>.from(rawValue));
    }
    if (rawValue is String && rawValue.trim().isNotEmpty) {
      try {
        final jsonValue = rawValue.trim();
        return IgnoredHotkeyApp.fromJson(Map<String, dynamic>.from(jsonDecode(jsonValue)));
      } catch (_) {
        return IgnoredHotkeyApp.empty();
      }
    }

    return IgnoredHotkeyApp.empty();
  }

  void updateValue(String key, dynamic value) {
    values[key] = value;
  }

  void updateTextValue(String key, String value) {
    updateValue(key, value);
    final controller = textboxEditingController[key];
    if (controller != null && controller.text != value) {
      controller.text = value;
    }
  }

  String getSelectOptionExtraValue(String columnKey, String optionValue, String extraKey) {
    final column = getColumn(columnKey);
    if (column == null) {
      return "";
    }

    for (final option in column.selectOptions) {
      if (option.value == optionValue) {
        final value = option.extra[extraKey];
        return value is String ? value : "";
      }
    }

    return "";
  }

  bool shouldOverwriteByMode({required String overwriteMode, required String currentValue, required bool force}) {
    if (force) {
      return true;
    }

    switch (overwriteMode) {
      case "always":
        return true;
      case "empty":
        return currentValue.isEmpty;
      default:
        return false;
    }
  }

  void applySelectColumnChangeActions(PluginSettingValueTableColumn column, {bool force = false, bool initOnly = false}) {
    if (column.type != PluginSettingValueType.pluginSettingValueTableColumnTypeSelect) {
      return;
    }

    final selectedValue = getValue(column.key).toString();
    if (selectedValue.isEmpty) {
      return;
    }

    for (final action in column.onChangedActions) {
      if (initOnly && !action.applyOnInit) {
        continue;
      }

      final mappedValue = getSelectOptionExtraValue(column.key, selectedValue, action.valueFromSelectedOptionExtra);
      if (mappedValue.isEmpty) {
        continue;
      }

      final currentTargetValue = getValue(action.targetKey).toString();
      final shouldOverwrite = shouldOverwriteByMode(overwriteMode: action.overwriteMode, currentValue: currentTargetValue, force: force);
      if (!shouldOverwrite) {
        continue;
      }

      updateTextValue(action.targetKey, mappedValue);
      final targetColumn = getColumn(action.targetKey);
      if (targetColumn != null) {
        setFieldValidationError(action.targetKey, validateValue(mappedValue, targetColumn));
      }
    }
  }

  void applyColumnInitActions(PluginSettingValueTableColumn column) {
    applySelectColumnChangeActions(column, force: true, initOnly: true);
  }

  String validateValue(dynamic value, PluginSettingValueTableColumn column) {
    return PluginSettingValidators.validateAll(
      value,
      column.validators,
      context: PluginSettingValidationContext(tableRows: widget.existingRows, originalTableRow: widget.originalRow, tableColumnKey: column.key),
    );
  }

  void setFieldValidationError(String key, String errorMessage) {
    if (errorMessage.isEmpty) {
      fieldValidationErrors.remove(key);
      return;
    }

    fieldValidationErrors[key] = errorMessage;
  }

  void clearCustomValidationErrors() {
    for (final key in _customValidationErrorKeys) {
      fieldValidationErrors.remove(key);
    }
    _customValidationErrorKeys.clear();
  }

  double getMaxColumnWidth() {
    double max = 0;
    for (var column in columns) {
      if (column.width > max) {
        max = column.width.toDouble();
      }
    }

    return max > 0 ? max : 100;
  }

  double measureMaxLabelWidth(BuildContext context) {
    double max = 60;
    const labelStyle = TextStyle(fontSize: 14, fontWeight: FontWeight.w600);

    for (final column in columns) {
      final translatedLabel = tr(column.label).trim();
      if (translatedLabel.isEmpty) {
        continue;
      }

      final measuredWidth = WoxTextMeasureUtil.measureTextWidth(context: context, text: translatedLabel, style: labelStyle) + 8;
      if (measuredWidth > max) {
        max = measuredWidth;
      }
    }

    return max.clamp(60, 180).toDouble();
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  WoxImage getWoxImageValue(String key) {
    final imgJson = getValue(key);
    if (imgJson is WoxImage) {
      return imgJson;
    }
    if (imgJson is String && imgJson == "") {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖");
    }

    try {
      return WoxImage.fromJson(imgJson);
    } catch (e) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖");
    }
  }

  // Load all tools list
  Future<void> _loadAllTools() async {
    try {
      final tools = await WoxApi.instance.findAIMCPServerToolsAll(const UuidV4().generate());
      if (mounted) {
        setState(() {
          allMCPTools = tools;
          isLoadingTools = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          isLoadingTools = false;
        });
      }
    }
  }

  Future<void> _loadAllSkills() async {
    try {
      final skills = await WoxApi.instance.findAISkills(const UuidV4().generate());
      if (mounted) {
        setState(() {
          allAISkills = skills;
          isLoadingSkills = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          isLoadingSkills = false;
        });
      }
    }
  }

  Future<void> _saveData(BuildContext context) async {
    if (isSaving) {
      return;
    }

    setState(() {
      saveErrorMessage = "";
    });

    // validate field validators first
    for (var column in columns) {
      if (column.validators.isNotEmpty) {
        setFieldValidationError(column.key, validateValue(getValue(column.key), column));
      }
    }
    if (fieldValidationErrors.isNotEmpty) {
      setState(() {});
      return;
    }

    // remove empty text list
    for (var column in columns) {
      if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeTextList) {
        var columnValues = getValue(column.key);
        if (columnValues is List) {
          columnValues.removeWhere((element) => element == "");
        }
      }
    }

    // validate with onUpdateValidate if provided
    if (widget.onUpdateValidate != null) {
      clearCustomValidationErrors();
      final validationErrors = await widget.onUpdateValidate!(values);
      if (validationErrors.isNotEmpty) {
        for (final validationError in validationErrors) {
          setFieldValidationError(validationError.key, validationError.errorMsg);
          _customValidationErrorKeys.add(validationError.key);
        }
        if (mounted) {
          setState(() {});
        }
        return;
      }
    }

    // Await the real table save result so a failed plugin setting update cannot look
    // successful. The previous fire-and-forget flow closed the dialog immediately,
    // which made row-match or backend errors appear as a silent no-op.
    setState(() {
      isSaving = true;
    });
    String? saveError;
    try {
      saveError = await widget.onUpdate(widget.item.key, Map<String, dynamic>.from(values));
    } catch (e) {
      saveError = e.toString().replaceFirst('Exception: ', '');
    } finally {
      if (mounted) {
        setState(() {
          isSaving = false;
        });
      }
    }

    if (saveError != null && saveError.trim().isNotEmpty) {
      if (mounted) {
        setState(() {
          saveErrorMessage = saveError!;
        });
      }
      return;
    }

    if (mounted && context.mounted) {
      Navigator.pop(context);
    }
  }

  KeyEventResult _handleDialogKeyEvent(BuildContext context, KeyEvent event) {
    if (event.logicalKey != LogicalKeyboardKey.escape) {
      return KeyEventResult.ignored;
    }

    if (event is KeyDownEvent || event is KeyRepeatEvent) {
      // Bug fix: closing this settings dialog on KeyDown let the matching KeyUp
      // fall back to the settings page, whose own Escape handler exits settings.
      // Defer the pop until KeyUp so one physical Escape press closes only this dialog.
      _isEscapeClosePending = true;
      return KeyEventResult.handled;
    }

    if (event is KeyUpEvent) {
      if (_isEscapeClosePending) {
        _isEscapeClosePending = false;
        Navigator.pop(context);
      }
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  // Builds text-like columns that need a placeholder-aware editor.
  Widget _buildQueryVariableColumn({required PluginSettingValueTableColumn column, required WoxQueryVariableSource source}) {
    return Expanded(
      child: WoxQueryVariableTextField(
        key: ValueKey(column.key),
        controller: textboxEditingController[column.key],
        focusNode: _focusNodes[column.key],
        maxLines: column.textMaxLines,
        source: source,
        onChanged: (value) {
          updateValue(column.key, value);
          setFieldValidationError(column.key, validateValue(value, column));
          setState(() {});
        },
      ),
    );
  }

  Widget buildColumn(PluginSettingValueTableColumn column) {
    switch (column.type) {
      case PluginSettingValueType.pluginSettingValueTableColumnTypeText:
        return Expanded(
          child: WoxTextField(
            controller: textboxEditingController[column.key],
            focusNode: _focusNodes[column.key],
            maxLines: column.textMaxLines,
            onChanged: (value) {
              updateValue(column.key, value);
              setFieldValidationError(column.key, validateValue(value, column));
              setState(() {});
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeQueryHotkeyQuery:
        return _buildQueryVariableColumn(column: column, source: WoxQueryVariableSource.queryHotkey);
      case PluginSettingValueType.pluginSettingValueTableColumnTypeAICommandPrompt:
        return _buildQueryVariableColumn(column: column, source: WoxQueryVariableSource.aiCommand);
      case PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox:
        return WoxCheckbox(
          value: getValueBool(column.key),
          onChanged: (value) {
            updateValue(column.key, value);
            setFieldValidationError(column.key, validateValue(value, column));
            setState(() {});
          },
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeHotkey:
        return WoxHotkeyRecorder(
          hotkey: WoxHotkey.parseHotkeyFromString(getValue(column.key)),
          // Table edit rows keep the hint on the right so it stays inside the hotkey cell instead of competing with row labels and descriptions.
          tipPosition: WoxHotkeyRecorderTipPosition.right,
          onHotKeyRecorded: (hotkey) {
            updateValue(column.key, hotkey);
            setFieldValidationError(column.key, validateValue(hotkey, column));
            setState(() {});
          },
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeDirPath:
        return Expanded(
          child: WoxPathFinder(
            value: getValue(column.key),
            enabled: true,
            showOpenButton: false,
            showChangeButton: true,
            confirmOnChange: false,
            changeButtonTextKey: 'ui_runtime_browse',
            onChanged: (path) {
              updateValue(column.key, path);
              setFieldValidationError(column.key, validateValue(path, column));
              setState(() {});
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeApp:
        return Expanded(
          child: WoxAppSelector(
            value: getIgnoredHotkeyAppValue(column.key),
            onChanged: (app) {
              final appJson = app.toJson();
              updateValue(column.key, appJson);
              setFieldValidationError(column.key, validateValue(appJson, column));
              setState(() {});
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeSelect:
        if (_isAICommandDefaultActionColumn(column)) {
          return Expanded(
            child: WoxAICommandDefaultActionDropdown(
              value: getValue(column.key).toString(),
              fontSize: 13,
              underline: Container(height: 1, color: getThemeDividerColor().withValues(alpha: 0.6)),
              onChanged: (value) {
                updateValue(column.key, value);
                applySelectColumnChangeActions(column);
                setFieldValidationError(column.key, validateValue(value, column));
                setState(() {});
              },
            ),
          );
        }
        return Expanded(
          child: Builder(
            builder: (context) {
              final currentValue = getValue(column.key);
              // Ensure the current value exists in selectOptions, otherwise use first option or null
              final valueExists = column.selectOptions.any((e) => e.value == currentValue);
              final effectiveValue = valueExists ? currentValue : (column.selectOptions.isNotEmpty ? column.selectOptions.first.value : null);

              return WoxDropdownButton<String>(
                value: effectiveValue,
                isExpanded: true,
                fontSize: 13,
                underline: Container(height: 1, color: getThemeDividerColor().withValues(alpha: 0.6)),
                onChanged: (value) {
                  updateValue(column.key, value);
                  applySelectColumnChangeActions(column);
                  setFieldValidationError(column.key, validateValue(value ?? "", column));
                  setState(() {});
                },
                items:
                    column.selectOptions.map((e) {
                      return WoxDropdownItem(
                        value: e.value,
                        label: e.label,
                        leading: e.icon.imageData.isNotEmpty ? WoxImageView(woxImage: e.icon, width: 16, height: 16) : null,
                        isSelectAll: e.isSelectAll,
                      );
                    }).toList(),
              );
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeSelectAIModel:
        return Expanded(
          child: SizedBox(
            width: 400, // Limit width to prevent overflow
            child: WoxAIModelSelectorView(
              initialValue: getValue(column.key),
              onInitialModelResolved: (modelJson) {
                if (getValue(column.key) == modelJson) {
                  return;
                }

                updateValue(column.key, modelJson);
                setFieldValidationError(column.key, validateValue(modelJson, column));
                if (mounted) {
                  setState(() {});
                }
              },
              onModelSelected: (modelJson) {
                updateValue(column.key, modelJson);
                setFieldValidationError(column.key, validateValue(modelJson, column));
                setState(() {});
              },
            ),
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectMCPServerTools:
        return Expanded(
          child: Builder(
            builder: (context) {
              if (isLoadingTools) {
                return const Center(child: WoxLoadingIndicator(size: 16));
              }

              final selectedTools = getValue(column.key) is List ? getValue(column.key) : [];

              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text("${selectedTools.length} tools selected", style: TextStyle(color: getThemeTextColor())),
                  const SizedBox(height: 8),
                  Container(
                    height: 200,
                    width: double.infinity, // Fill available width for consistency
                    decoration: BoxDecoration(
                      border: Border.all(color: getThemeSubTextColor()), // unify with TextField border color
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: ListView.builder(
                      itemCount: allMCPTools.length,
                      itemBuilder: (context, index) {
                        final tool = allMCPTools[index];
                        final isSelected = selectedTools.contains(tool.name);

                        return WoxCheckboxTile(
                          value: isSelected,
                          onChanged: (value) {
                            setState(() {
                              if (value == true) {
                                if (!selectedTools.contains(tool.name)) {
                                  selectedTools.add(tool.name);
                                }
                              } else {
                                selectedTools.remove(tool.name);
                              }
                              updateValue(column.key, selectedTools);
                              setFieldValidationError(column.key, validateValue(selectedTools, column));
                            });
                          },
                          title: tool.name,
                        );
                      },
                    ),
                  ),
                ],
              );
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectSkills:
        return Expanded(
          child: Builder(
            builder: (context) {
              if (isLoadingSkills) {
                return const Center(child: WoxLoadingIndicator(size: 16));
              }

              final selectedSkills = getValue(column.key) is List ? getValue(column.key) : [];
              final enabledSkills = allAISkills.where((skill) => skill.enabled).toList();

              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text("${selectedSkills.length} skills selected", style: TextStyle(color: getThemeTextColor())),
                  const SizedBox(height: 8),
                  Container(
                    height: 200,
                    width: double.infinity,
                    decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor()), borderRadius: BorderRadius.circular(4)),
                    child: ListView.builder(
                      itemCount: enabledSkills.length,
                      itemBuilder: (context, index) {
                        final skill = enabledSkills[index];
                        final isSelected = selectedSkills.contains(skill.id);
                        final sourceName = skill.sourceName.isEmpty ? skill.source : skill.sourceName;
                        final title = sourceName.isEmpty ? skill.name : "${skill.name} - $sourceName";

                        return WoxCheckboxTile(
                          value: isSelected,
                          onChanged: (value) {
                            setState(() {
                              if (value) {
                                if (!selectedSkills.contains(skill.id)) {
                                  selectedSkills.add(skill.id);
                                }
                              } else {
                                selectedSkills.remove(skill.id);
                              }
                              updateValue(column.key, selectedSkills);
                              setFieldValidationError(column.key, validateValue(selectedSkills, column));
                            });
                          },
                          title: title,
                        );
                      },
                    ),
                  ),
                ],
              );
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeWoxImage:
        return Expanded(
          child: WoxImageSelector(
            value: getWoxImageValue(column.key),
            onChanged: (newImage) {
              updateValue(column.key, newImage);
              setFieldValidationError(column.key, validateValue(newImage, column));
              setState(() {});
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeTextList:
        var columnValues = getValue(column.key);
        return Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              for (var i = 0; i < columnValues.length; i++)
                Padding(
                  padding: const EdgeInsets.only(bottom: 8.0),
                  child: Row(
                    children: [
                      Expanded(
                        child: WoxTextField(
                          controller: textboxEditingController[column.key + i.toString()],
                          focusNode: _focusNodes[column.key + i.toString()],
                          maxLines: 1,
                          style: TextStyle(overflow: TextOverflow.ellipsis, color: getThemeTextColor()),
                          onChanged: (value) {
                            columnValues[i] = value;
                            setFieldValidationError(column.key, validateValue(columnValues, column));
                            setState(() {});
                          },
                        ),
                      ),
                      IconButton(
                        icon: Icon(Icons.delete, color: getThemeActiveBackgroundColor()),
                        onPressed: () {
                          columnValues.removeAt(i);
                          //remove controller
                          textboxEditingController.remove(column.key + i.toString());
                          values[column.key] = columnValues;
                          setFieldValidationError(column.key, validateValue(columnValues, column));
                          setState(() {});
                        },
                      ),
                      // last row show add button
                      if (i == columnValues.length - 1)
                        IconButton(
                          icon: Icon(Icons.add, color: getThemeActiveBackgroundColor()),
                          onPressed: () {
                            columnValues.add("");
                            textboxEditingController[column.key + (columnValues.length - 1).toString()] = TextEditingController();
                            values[column.key] = columnValues;
                            setFieldValidationError(column.key, validateValue(columnValues, column));
                            setState(() {});
                          },
                        ),
                      if (i != columnValues.length - 1) const SizedBox(width: 26),
                    ],
                  ),
                ),
              if (columnValues.isEmpty)
                IconButton(
                  icon: Icon(Icons.add, color: getThemeActiveBackgroundColor()),
                  onPressed: () {
                    columnValues.add("");
                    values[column.key] = columnValues;
                    setFieldValidationError(column.key, validateValue(columnValues, column));
                    setState(() {});
                  },
                ),
            ],
          ),
        );
      default:
        return const SizedBox();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final Color accentColor = getThemeActiveBackgroundColor();
      final Color textColor = getThemeTextColor();
      final double maxLabelWidth = measureMaxLabelWidth(context);
      // Table add/update dialogs used to have only the shared adaptive width,
      // which is too tight for dense rows such as Query Hotkeys. A table-level
      // override keeps the default behavior unchanged while allowing specific
      // tables to reserve enough room for long query fields and descriptions.
      final double dialogContentWidth =
          widget.item.updateDialogWidth > 0 ? widget.item.updateDialogWidth.toDouble() : math.max(600, maxLabelWidth + math.max(320, getMaxColumnWidth()));

      return Focus(
        autofocus: true,
        onKeyEvent: (node, event) => _handleDialogKeyEvent(context, event),
        child: WoxDialog(
          content: SizedBox(
            width: dialogContentWidth,
            child: SingleChildScrollView(
              // Desktop scrollbars overlay this scroll view, so reserve a small
              // right gutter for long tooltips and wide inputs instead of letting
              // the thumb cover readable text.
              child: Padding(
                padding: const EdgeInsets.only(right: 20),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const SizedBox(height: 4),
                    for (var column in columns)
                      if (!column.hideInUpdate)
                        Padding(
                          padding: const EdgeInsets.only(bottom: 10),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Row(
                                crossAxisAlignment: CrossAxisAlignment.center,
                                children: [
                                  SizedBox(
                                    width: maxLabelWidth,
                                    child: Text(tr(column.label), style: TextStyle(color: textColor.withValues(alpha: 0.92), fontSize: 14, fontWeight: FontWeight.w600)),
                                  ),
                                  const SizedBox(width: 10),
                                  // Table row editors use the same compact split layout as plugin
                                  // settings because their dialogs are much narrower than full
                                  // settings pages and need the input column to stay scannable.
                                  buildColumn(column),
                                ],
                              ),
                              if (column.tooltip != "")
                                Padding(
                                  padding: EdgeInsets.only(top: 4, left: maxLabelWidth + 10),
                                  child: ExcludeFocus(
                                    child: WoxMarkdownView(
                                      data: tr(column.tooltip),
                                      fontColor: textColor.withValues(alpha: 0.6),
                                      fontSize: 12,
                                      linkColor: accentColor,
                                      linkHoverColor: accentColor.withValues(alpha: 0.8),
                                      selectable: true,
                                    ),
                                  ),
                                ),
                              if ((fieldValidationErrors[column.key] ?? "").isNotEmpty)
                                Padding(
                                  padding: EdgeInsets.only(top: column.tooltip != "" ? 4 : 2, left: maxLabelWidth + 10),
                                  child: Text(tr(fieldValidationErrors[column.key]!), style: const TextStyle(color: Colors.red, fontSize: 12)),
                                ),
                            ],
                          ),
                        ),
                    if (saveErrorMessage.isNotEmpty)
                      Padding(
                        padding: EdgeInsets.only(top: 2, left: maxLabelWidth + 10),
                        child: Text(tr(saveErrorMessage), style: const TextStyle(color: Colors.red, fontSize: 12)),
                      ),
                  ],
                ),
              ),
            ),
          ),
          actions: [
            WoxButton.secondary(text: tr("ui_cancel"), padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12), onPressed: () => Navigator.pop(context)),
            const SizedBox(width: 12),
            WoxButton.primary(
              text: isSaving ? "${tr("ui_save")}..." : tr("ui_save"),
              padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 12),
              onPressed:
                  isSaving
                      ? null
                      : () {
                        _saveData(context);
                      },
            ),
          ],
        ),
      );
    });
  }
}
