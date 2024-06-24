import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/picker.dart';

class WoxSettingPluginTableUpdate extends StatefulWidget {
  final PluginSettingValueTable item;
  final Map<String, dynamic> row;
  final Function onUpdate;

  const WoxSettingPluginTableUpdate({super.key, required this.item, required this.row, required this.onUpdate});

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

  @override
  void initState() {
    super.initState();

    for (var element in widget.item.columns) {
      if (!element.hideInUpdate) {
        columns.add(element);
      }
    }

    widget.row.forEach((key, value) {
      values[key] = value;
    });

    if (values.isEmpty) {
      for (var column in columns) {
        values[column.key] = "";
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

    return max > 0 ? max : 100;
  }

  Future<List<PluginSettingValueSelectOption>> getSelectionAIModelOptions() async {
    final models = await WoxApi.instance.findAIModels();
    return models.map((e) {
      return PluginSettingValueSelectOption(value: jsonEncode(e), label: "${e.provider} - ${e.name}");
    }).toList();
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
            child: TextBox(
              controller: textboxEditingController[column.key],
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
          checked: getValueBool(column.key),
          onChanged: (value) {
            updateValue(column.key, value);
            setState(() {});
          },
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeHotkey:
        return WoxHotkeyRecorder(
          hotkey: WoxHotkey.parseHotkey(getValue(column.key)),
          onHotKeyRecorded: (hotkey) {
            updateValue(column.key, hotkey);
            setState(() {});
          },
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeDirPath:
        return Expanded(
          child: TextBox(
            controller: TextEditingController(text: getValue(column.key)),
            onChanged: (value) {
              updateValue(column.key, value);
            },
            suffixMode: OverlayVisibilityMode.always,
            suffix: Button(
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
              child: const Text('Browse'),
            ),
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeSelect:
        return Expanded(
          child: ComboBox<String>(
            value: getValue(column.key),
            onChanged: (value) {
              updateValue(column.key, value);
              setState(() {});
            },
            items: column.selectOptions.map((e) {
              return ComboBoxItem(
                value: e.value,
                child: Text(e.label),
              );
            }).toList(),
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeSelectAIModel:
        return Expanded(
          child: FutureBuilder(
            future: getSelectionAIModelOptions(),
            builder: (context, snapshot) {
              if (snapshot.connectionState == ConnectionState.done) {
                return ComboBox<String>(
                  value: getValue(column.key),
                  onChanged: (value) {
                    updateValue(column.key, value);
                    setState(() {});
                  },
                  items: snapshot.data?.map((e) {
                    return ComboBoxItem(
                      value: e.value,
                      child: Text(e.label),
                    );
                  }).toList(),
                );
              } else {
                return const SizedBox();
              }
            },
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeWoxImage:
        return Text("wox image...");
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
                          child: TextBox(
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
                            style: const TextStyle(
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                        ),
                      ),
                      IconButton(
                        icon: const Icon(FluentIcons.delete),
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
                          icon: const Icon(FluentIcons.add),
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
                  icon: const Icon(FluentIcons.add),
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
    return FluentApp(
      debugShowCheckedModeBanner: false,
      home: ContentDialog(
        constraints: const BoxConstraints(maxWidth: 800, maxHeight: 600),
        content: SingleChildScrollView(
          child: Column(children: [
            for (var column in columns)
              if (!column.hideInUpdate)
                Padding(
                  padding: const EdgeInsets.only(bottom: 20.0),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          SizedBox(
                            width: getMaxColumnWidth(),
                            child: Text(
                              column.label,
                              style: const TextStyle(overflow: TextOverflow.ellipsis),
                              textAlign: TextAlign.right,
                            ),
                          ),
                          const SizedBox(width: 16),
                          buildColumn(column),
                        ],
                      ),
                      if (column.tooltip != "")
                        Padding(
                          padding: EdgeInsets.only(left: getMaxColumnWidth() + 16, top: 4),
                          child: Text(
                            column.tooltip,
                            style: TextStyle(color: Colors.grey[90], fontSize: 12),
                          ),
                        ),
                      if (fieldValidationErrors.containsKey(column.key))
                        Padding(
                          padding: EdgeInsets.only(left: getMaxColumnWidth() + 16, top: 4),
                          child: Text(
                            fieldValidationErrors[column.key]!,
                            style: TextStyle(color: Colors.red, fontSize: 12),
                          ),
                        ),
                    ],
                  ),
                ),
          ]),
        ),
        actions: [
          Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              Button(
                child: const Text('Cancel'),
                onPressed: () => Navigator.pop(context),
              ),
              const SizedBox(width: 16),
              FilledButton(
                child: const Text('Confirm'),
                onPressed: () {
                  // validate
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

                  widget.onUpdate(widget.item.key, values);

                  Navigator.pop(context);
                },
              ),
            ],
          )
        ],
      ),
    );
  }
}
