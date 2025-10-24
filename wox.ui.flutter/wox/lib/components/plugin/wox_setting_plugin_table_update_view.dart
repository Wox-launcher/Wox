import 'dart:convert';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/services.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_ai_model_selector_view.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_tooltip_view.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/picker.dart';
import 'package:wox/utils/colors.dart';
import 'package:get/get.dart';

class WoxSettingPluginTableUpdate extends StatefulWidget {
  final PluginSettingValueTable item;
  final Map<String, dynamic> row;
  final Function onUpdate;
  final Future<String?> Function(Map<String, dynamic> rowValues)? onUpdateValidate;

  const WoxSettingPluginTableUpdate({
    super.key,
    required this.item,
    required this.row,
    required this.onUpdate,
    this.onUpdateValidate,
  });

  @override
  State<WoxSettingPluginTableUpdate> createState() => _WoxSettingPluginTableUpdateState();
}

class _WoxSettingPluginTableUpdateState extends State<WoxSettingPluginTableUpdate> {
  Map<String, dynamic> values = {};
  bool isUpdate = false;
  bool disableBrowse = false;
  Map<String, String> fieldValidationErrors = {};
  Map<String, TextEditingController> textboxEditingController = {};
  List<PluginSettingValueTableColumn> columns = [];
  String? customValidationError;

  // Store tool list to avoid repeated requests
  List<AIMCPTool> allMCPTools = [];
  bool isLoadingTools = true;

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

    widget.row.forEach((key, value) {
      values[key] = value;
    });

    if (values.isEmpty) {
      for (var column in columns) {
        if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeTextList) {
          values[column.key] = [];
        } else if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox) {
          values[column.key] = false;
        } else {
          values[column.key] = "";
        }
      }
    } else {
      isUpdate = true;
    }

    for (var column in columns) {
      // init text box controller
      if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeText) {
        textboxEditingController[column.key] = TextEditingController(text: getValue(column.key));
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
        }
      }
    }
  }

  @override
  void dispose() {
    for (var controller in textboxEditingController.values) {
      controller.dispose();
    }
    super.dispose();
  }

  dynamic getValue(String key) {
    return values[key] ?? "";
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

  void updateValue(String key, dynamic value) {
    values[key] = value;
  }

  double getMaxColumnWidth() {
    double max = 0;
    for (var column in columns) {
      if (column.width > max) {
        max = column.width.toDouble();
      }
    }

    return max > 0 ? max + 22 /* for tooltip width*/ : 100;
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  Widget _buildWoxImageEditor(PluginSettingValueTableColumn column) {
    WoxImage? currentImage;
    dynamic imgJson = getValue(column.key);

    if (imgJson is String && imgJson == "") {
      currentImage = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖");
    } else if (imgJson is WoxImage) {
      currentImage = imgJson;
    } else {
      //image sholuld be WoxImage map
      try {
        currentImage = WoxImage.fromJson(imgJson);
      } catch (e) {
        currentImage = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖");
      }
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Container(
          width: 80,
          height: 80,
          decoration: BoxDecoration(
            border: Border.all(color: getThemeSubTextColor().withAlpha(76)),
            borderRadius: BorderRadius.circular(8),
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: Center(
              // Center the preview content (especially emoji) in the 80x80 box
              child: WoxImageView(
                woxImage: currentImage,
                width: 80,
                height: 80,
              ),
            ),
          ),
        ),
        const SizedBox(width: 16),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            ElevatedButton(
              style: ButtonStyle(
                padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 12, vertical: 8)),
                backgroundColor: WidgetStateProperty.all(getThemeActiveBackgroundColor().withAlpha(20)),
                foregroundColor: WidgetStateProperty.all(getThemeTextColor()),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.emoji_emotions_outlined, size: 14, color: getThemeTextColor()),
                  const SizedBox(width: 6),
                  Text(tr('ui_image_editor_emoji'), style: TextStyle(color: getThemeTextColor())),
                ],
              ),
              onPressed: () async {
                final emojiResult = await _showEmojiPicker(context);
                if (emojiResult != null && emojiResult.isNotEmpty) {
                  final newImage = WoxImage(
                    imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code,
                    imageData: emojiResult,
                  );
                  updateValue(column.key, newImage);
                  setState(() {});
                }
              },
            ),
            ElevatedButton(
              style: ButtonStyle(
                padding: WidgetStateProperty.all(const EdgeInsets.symmetric(horizontal: 12, vertical: 8)),
                backgroundColor: WidgetStateProperty.all(getThemeActiveBackgroundColor().withAlpha(20)),
                foregroundColor: WidgetStateProperty.all(getThemeTextColor()),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.file_upload_outlined, size: 14, color: getThemeTextColor()),
                  const SizedBox(width: 6),
                  Text(tr('ui_image_editor_upload_image'), style: TextStyle(color: getThemeTextColor())),
                ],
              ),
              onPressed: () async {
                final result = await FilePicker.platform.pickFiles(
                  type: FileType.image,
                  allowMultiple: false,
                );

                if (result != null && result.files.isNotEmpty && result.files.first.path != null) {
                  final filePath = result.files.first.path!;
                  final file = File(filePath);
                  if (await file.exists()) {
                    final bytes = await file.readAsBytes();
                    final base64Image = base64Encode(bytes);

                    final newImage = WoxImage(
                      imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code,
                      imageData: "data:image/png;base64,$base64Image",
                    );

                    updateValue(column.key, newImage);
                    setState(() {});
                  }
                }
              },
            ),
          ],
        ),
      ],
    );
  }

  Future<String?> _showEmojiPicker(BuildContext context) async {
    final commonEmojis = ["🤖", "👨", "👩", "🧠", "💡", "🔍", "📊", "📈", "📝", "🛠", "⚙️", "🧩", "🎮", "🎯", "🏆", "🎨", "🎭", "🎬", "📱", "💻"];

    String? selectedEmoji;

    await showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          title: Text(tr('ui_select_emoji')),
          content: SizedBox(
            width: 300,
            height: 200,
            child: GridView.builder(
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 5,
                childAspectRatio: 1,
              ),
              itemCount: commonEmojis.length,
              itemBuilder: (context, index) {
                return InkWell(
                  onTap: () {
                    selectedEmoji = commonEmojis[index];
                    Navigator.pop(context);
                  },
                  child: Center(
                    child: Text(
                      commonEmojis[index],
                      style: const TextStyle(fontSize: 24),
                    ),
                  ),
                );
              },
            ),
          ),
          actions: [
            TextButton(
              child: Text(tr('ui_cancel')),
              onPressed: () {
                Navigator.pop(context);
              },
            ),
          ],
        );
      },
    );

    return selectedEmoji;
  }

  // Load all tools list
  Future<void> _loadAllTools() async {
    try {
      final tools = await WoxApi.instance.findAIMCPServerToolsAll();
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

  Future<void> _saveData(BuildContext context) async {
    // validate field validators first
    for (var column in columns) {
      if (column.validators.isNotEmpty) {
        for (var element in column.validators) {
          var errMsg = element.validator.validate(getValue(column.key));
          if (errMsg != "") {
            fieldValidationErrors[column.key] = errMsg;
          } else {
            fieldValidationErrors.remove(column.key);
          }
        }
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
      String? validationError = await widget.onUpdateValidate!(values);
      if (validationError != null) {
        if (mounted) {
          setState(() {
            customValidationError = validationError;
          });
        }
        return;
      } else {
        if (mounted) {
          setState(() {
            customValidationError = null;
          });
        }
      }
    }

    widget.onUpdate(widget.item.key, values);
    if (mounted && context.mounted) {
      Navigator.pop(context);
    }
  }

  Widget buildColumn(PluginSettingValueTableColumn column) {
    switch (column.type) {
      case PluginSettingValueType.pluginSettingValueTableColumnTypeText:
        return Expanded(
          child: Focus(
            onFocusChange: (hasFocus) {
              if (!hasFocus) {
                for (var element in column.validators) {
                  var errMsg = element.validator.validate(textboxEditingController[column.key]!.text);
                  if (errMsg != "") {
                    fieldValidationErrors[column.key] = errMsg;
                    setState(() {});
                    return;
                  }
                }

                fieldValidationErrors.remove(column.key);
                setState(() {});
              }
            },
            child: TextField(
              controller: textboxEditingController[column.key],
              style: TextStyle(color: getThemeTextColor(), fontSize: 13),
              decoration: InputDecoration(
                enabledBorder: UnderlineInputBorder(
                  borderSide: BorderSide(color: getThemeTextColor().withOpacity(0.3)),
                ),
                focusedBorder: UnderlineInputBorder(
                  borderSide: BorderSide(color: getThemeActiveBackgroundColor(), width: 2),
                ),
              ),
              onChanged: (value) {
                updateValue(column.key, value);

                for (var element in column.validators) {
                  var errMsg = element.validator.validate(value);
                  if (errMsg != "") {
                    fieldValidationErrors[column.key] = errMsg;
                    setState(() {});
                    return;
                  }
                }

                fieldValidationErrors.remove(column.key);
                setState(() {});
              },
              maxLines: column.textMaxLines,
            ),
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox:
        return Checkbox(
          value: getValueBool(column.key),
          onChanged: (value) {
            updateValue(column.key, value);
            setState(() {});
          },
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeHotkey:
        return WoxHotkeyRecorder(
          hotkey: WoxHotkey.parseHotkeyFromString(getValue(column.key)),
          onHotKeyRecorded: (hotkey) {
            updateValue(column.key, hotkey);
            setState(() {});
          },
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeDirPath:
        return Expanded(
          child: TextField(
            controller: TextEditingController(text: getValue(column.key)),
            onChanged: (value) {
              updateValue(column.key, value);
            },
            decoration: InputDecoration(
              suffixIcon: ElevatedButton(
                onPressed: disableBrowse
                    ? null
                    : () async {
                        disableBrowse = true;
                        final selectedDirectory = await FileSelector.pick(
                          const UuidV4().generate(),
                          FileSelectorParams(isDirectory: true),
                        );
                        if (selectedDirectory.isNotEmpty) {
                          updateValue(column.key, selectedDirectory[0]);
                          setState(() {});
                        }
                        disableBrowse = false;
                      },
                child: Text('Browse', style: TextStyle(color: getThemeTextColor())),
              ),
            ),
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeSelect:
        return Expanded(
          child: DropdownButton<String>(
            value: getValue(column.key),
            isExpanded: true,
            onChanged: (value) {
              updateValue(column.key, value);
              setState(() {});
            },
            items: column.selectOptions.map((e) {
              return DropdownMenuItem(
                value: e.value,
                child: Text(e.label),
              );
            }).toList(),
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeSelectAIModel:
        return Expanded(
          child: SizedBox(
            width: 400, // Limit width to prevent overflow
            child: WoxAIModelSelectorView(
              initialValue: getValue(column.key),
              onModelSelected: (modelJson) {
                updateValue(column.key, modelJson);
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
                return const Center(child: CircularProgressIndicator(strokeWidth: 2));
              }

              final selectedTools = getValue(column.key) is List ? getValue(column.key) : [];

              return Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text("${selectedTools.length} tools selected", style: TextStyle(color: getThemeTextColor())),
                  const SizedBox(height: 8),
                  Container(
                    height: 200,
                    width: 400, // Limit width to prevent overflow
                    decoration: BoxDecoration(
                      border: Border.all(color: getThemeSubTextColor().withAlpha(76)), // 0.3 * 255 �?76
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: ListView.builder(
                      itemCount: allMCPTools.length,
                      itemBuilder: (context, index) {
                        final tool = allMCPTools[index];
                        final isSelected = selectedTools.contains(tool.name);

                        return Padding(
                          padding: const EdgeInsets.symmetric(vertical: 2.0, horizontal: 4.0),
                          child: CheckboxListTile(
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
                              });
                            },
                            title: Text(
                              tool.name, // Only display tool name
                              style: TextStyle(color: getThemeTextColor()),
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
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
          child: _buildWoxImageEditor(column),
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
                        child: Focus(
                          onFocusChange: (hasFocus) {
                            if (!hasFocus) {
                              //validate
                              for (var element in column.validators) {
                                var errMsg = element.validator.validate(columnValues);
                                if (errMsg != "") {
                                  fieldValidationErrors[column.key] = errMsg;
                                  setState(() {});
                                  return;
                                }
                              }

                              fieldValidationErrors.remove(column.key);
                              setState(() {});
                            }
                          },
                          child: TextField(
                            controller: textboxEditingController[column.key + i.toString()],
                            onChanged: (value) {
                              columnValues[i] = value;

                              for (var element in column.validators) {
                                var errMsg = element.validator.validate(columnValues);
                                if (errMsg != "") {
                                  fieldValidationErrors[column.key] = errMsg;
                                  setState(() {});
                                  return;
                                }
                              }

                              fieldValidationErrors.remove(column.key);
                              setState(() {});
                            },
                            maxLines: 1,
                            style: TextStyle(
                              overflow: TextOverflow.ellipsis,
                              color: getThemeTextColor(),
                            ),
                          ),
                        ),
                      ),
                      IconButton(
                        icon: Icon(Icons.delete, color: getThemeActiveBackgroundColor()),
                        onPressed: () {
                          columnValues.removeAt(i);
                          //remove controller
                          textboxEditingController.remove(column.key + i.toString());
                          values[column.key] = columnValues;

                          //validate
                          for (var element in column.validators) {
                            var errMsg = element.validator.validate(columnValues);
                            if (errMsg != "") {
                              fieldValidationErrors[column.key] = errMsg;
                              setState(() {});
                              return;
                            }
                          }

                          fieldValidationErrors.remove(column.key);
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
      return MaterialApp(
        debugShowCheckedModeBanner: false,
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: getThemeActiveBackgroundColor(),
            brightness: getThemeBackgroundColor().computeLuminance() < 0.5 ? Brightness.dark : Brightness.light,
          ),
          scaffoldBackgroundColor: Colors.transparent,
          cardColor: getThemeCardBackgroundColor(),
          shadowColor: getThemeTextColor().withAlpha(50),
        ),
        home: Focus(
          autofocus: true,
          onKeyEvent: (node, event) {
            if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
              Navigator.pop(context);
              return KeyEventResult.handled;
            }
            return KeyEventResult.ignored;
          },
          child: AlertDialog(
            contentPadding: const EdgeInsets.all(24),
            content: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  for (var column in columns)
                    if (!column.hideInUpdate)
                      Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: Row(
                          children: [
                            SizedBox(
                              width: getMaxColumnWidth(),
                              child: Row(
                                children: [
                                  Text(
                                    tr(column.label),
                                    style: TextStyle(color: getThemeTextColor()),
                                  ),
                                  if (column.tooltip != "")
                                    WoxTooltipView(
                                      tooltip: tr(column.tooltip),
                                      color: getThemeTextColor(),
                                    ),
                                ],
                              ),
                            ),
                            const SizedBox(width: 10),
                            buildColumn(column),
                          ],
                        ),
                      ),
                  if (customValidationError != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 10),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(
                              customValidationError!,
                              style: const TextStyle(color: Colors.red),
                            ),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
            ),
            actions: [
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  ElevatedButton(
                    style: ButtonStyle(
                      foregroundColor: WidgetStateProperty.all(getThemeTextColor()),
                    ),
                    child: const Text('Cancel'),
                    onPressed: () => Navigator.pop(context),
                  ),
                  const SizedBox(width: 16),
                  ElevatedButton(
                    style: ButtonStyle(
                      backgroundColor: WidgetStateProperty.all(getThemeActiveBackgroundColor()),
                      foregroundColor: WidgetStateProperty.all(getThemeActionItemActiveColor()),
                    ),
                    onPressed: () {
                      _saveData(context);
                    },
                    child: const Text('Save'),
                  ),
                ],
              ),
            ],
          ),
        ),
      );
    });
  }
}
