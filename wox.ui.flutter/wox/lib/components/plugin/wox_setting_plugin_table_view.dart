import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_tooltip_icon_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

import 'wox_setting_plugin_item_view.dart';
import 'wox_setting_plugin_table_update_view.dart';

class WoxSettingPluginTable extends WoxSettingPluginItem {
  static const int tableMaxHeightMin = 120;
  final PluginSettingValueTable item;
  static const String rowUniqueIdKey = "wox_table_row_id";
  final double tableWidth;
  final operationWidth = 80.0;
  final columnSpacing = 10.0;
  final columnTooltipWidth = 20.0;
  final bool readonly;
  final Future<String?> Function(Map<String, dynamic> rowValues)? onUpdateValidate;
  final ScrollController horizontalScrollController = ScrollController();
  final ScrollController verticalScrollController = ScrollController();

  WoxSettingPluginTable({super.key, required this.item, required super.value, required super.onUpdate, this.tableWidth = 740.0, this.readonly = false, this.onUpdateValidate});

  double calculateColumnWidthForZeroWidth(PluginSettingValueTableColumn column) {
    // if there are multiple columns which have width set to 0, we will set the max width to 100 for each column
    // if there is only one column which has width set to 0, we will set the max width to tableWidth - (other columns width)
    // if all columns have width set to 0, we will set the max width to 100 for each column
    var zeroWidthColumnCount = 0;
    var totalWidth = 0.0;
    var totalColumnTooltipWidth = 0.0;
    for (var element in item.columns) {
      if (element.hideInTable) {
        continue;
      }

      totalWidth += element.width + columnSpacing;
      if (element.width == 0) {
        zeroWidthColumnCount++;
      }
      if (element.tooltip.isNotEmpty) {
        totalColumnTooltipWidth += columnTooltipWidth;
      }
    }
    if (zeroWidthColumnCount == 1) {
      return tableWidth - totalWidth - (operationWidth + columnSpacing) - totalColumnTooltipWidth;
    }

    return 100.0;
  }

  Widget columnWidth({required PluginSettingValueTableColumn column, required bool isHeader, required bool isOperation, required Widget child}) {
    var width = column.width;
    if (isOperation) {
      width = operationWidth.toInt();
    }
    if (width == 0) {
      width = calculateColumnWidthForZeroWidth(column).toInt();
    }
    if (column.tooltip.isNotEmpty) {
      width += columnTooltipWidth.toInt();
    }

    return SizedBox(width: width.toDouble(), child: Align(alignment: Alignment.centerLeft, child: child));
  }

  Widget buildHeaderCell(PluginSettingValueTableColumn column) {
    final String translatedLabel = tr(column.label);
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Flexible(
          child: Tooltip(
            message: translatedLabel,
            child: Text(
              translatedLabel,
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
              style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor), fontSize: 13),
            ),
          ),
        ),
        if (column.tooltip != "")
          WoxTooltipIconView(tooltip: tr(column.tooltip), paddingRight: 0, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor)),
      ],
    );
  }

  Widget buildRowCell(PluginSettingValueTableColumn column, Map<String, dynamic> row) {
    var value = row[column.key] ?? "";

    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeText) {
      return columnWidth(
        column: column,
        isHeader: false,
        isOperation: false,
        child: Text(value, style: TextStyle(overflow: TextOverflow.ellipsis, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeDirPath) {
      return columnWidth(
        column: column,
        isHeader: false,
        isOperation: false,
        child: Text(value, style: TextStyle(overflow: TextOverflow.ellipsis, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeHotkey) {
      return columnWidth(
        column: column,
        isHeader: false,
        isOperation: false,
        child: Text(value, style: TextStyle(overflow: TextOverflow.ellipsis, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox) {
      var isChecked = false;
      if (value is bool) {
        isChecked = value;
      } else if (value is String) {
        isChecked = value == "true";
      }
      return Row(children: [isChecked ? Icon(Icons.check_box, color: getThemeTextColor()) : Icon(Icons.check_box_outline_blank, color: getThemeTextColor())]);
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeTextList) {
      if (value is String && value == "") {
        value = <String>[];
      }

      return Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          for (var txt in value)
            columnWidth(
              column: column,
              isHeader: false,
              isOperation: false,
              child: Text(
                "${(value as List<dynamic>).length == 1 ? "" : "-"} $txt",
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor)),
              ),
            ),
        ],
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeWoxImage) {
      // there are two types of wox image
      // 1. WoxImage map
      // 2. WoxImage json string
      // if the value is a map, we will convert it to json string

      if (value is Map<String, dynamic>) {
        value = json.encode(value);
      }
      if (value == "") {
        return const SizedBox.shrink();
      }

      final woxImage = WoxImage.fromJson(jsonDecode(value));
      return Row(children: [WoxImageView(woxImage: woxImage, width: 24, height: 24)]);
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeSelect) {
      var selectOption = column.selectOptions.firstWhere((element) => element.value == value, orElse: () => PluginSettingValueSelectOption.fromJson(<String, dynamic>{}));
      return columnWidth(
        column: column,
        isHeader: false,
        isOperation: false,
        child: Text(selectOption.label, style: TextStyle(overflow: TextOverflow.ellipsis, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeSelectAIModel) {
      var model = AIModel.fromJson(json.decode(value));
      final providerDisplayName = model.providerAlias.isEmpty ? model.provider : "${model.provider} (${model.providerAlias})";
      return columnWidth(
        column: column,
        isHeader: false,
        isOperation: false,
        child: Text(
          "$providerDisplayName - ${model.name}",
          style: TextStyle(overflow: TextOverflow.ellipsis, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor)),
        ),
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAIModelStatus) {
      var providerName = row["Name"] ?? "";
      var modelName = row["ApiKey"] ?? "";
      var host = row["Host"] ?? "";

      return FutureBuilder<String>(
        future: WoxApi.instance.pingAIModel(const UuidV4().generate(), providerName, modelName, host),
        builder: (context, snapshot) {
          return columnWidth(
            column: column,
            isHeader: false,
            isOperation: false,
            child:
                snapshot.connectionState == ConnectionState.waiting
                    ? const Icon(Icons.circle, color: Colors.grey)
                    : snapshot.error != null
                    ? Tooltip(message: snapshot.error?.toString() ?? "", child: const Icon(Icons.circle, color: Colors.red))
                    : const Icon(Icons.circle, color: Colors.green),
          );
        },
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAIMCPServerTools) {
      var disabled = row["disabled"] ?? false;
      if (disabled) {
        return const Icon(Icons.circle, color: Colors.grey);
      }

      return FutureBuilder<List<AIMCPTool>>(
        future: WoxApi.instance.findAIMCPServerTools(const UuidV4().generate(), row),
        builder: (context, snapshot) {
          return columnWidth(
            column: column,
            isHeader: false,
            isOperation: false,
            child:
                snapshot.connectionState == ConnectionState.waiting
                    ? const Icon(Icons.circle, color: Colors.grey)
                    : snapshot.error != null
                    ? Tooltip(message: snapshot.error?.toString() ?? "", child: const Icon(Icons.circle, color: Colors.red))
                    : Tooltip(
                      message: snapshot.data?.map((e) => e.name).join("\n") ?? "",
                      child: Text("${snapshot.data?.length ?? 0} tools", style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
                    ),
          );
        },
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectMCPServerTools) {
      final toolNames = value as List<dynamic>;
      return Tooltip(
        message: toolNames.join("\n"),
        child: columnWidth(
          column: column,
          isHeader: false,
          isOperation: false,
          child: Text("${toolNames.length} tools", style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
        ),
      );
    }

    return Text("Unknown column type: ${column.type}");
  }

  /// Find the index of a row in [freshRows] that matches [originalRow] by comparing
  /// all fields except the unique row ID key.
  int _findRowIndex(List<dynamic> freshRows, Map<String, dynamic> originalRow) {
    for (var i = 0; i < freshRows.length; i++) {
      var candidate = freshRows[i] as Map<String, dynamic>;
      bool match = true;
      for (var key in originalRow.keys) {
        if (key == rowUniqueIdKey) continue;
        if ('${candidate[key]}' != '${originalRow[key]}') {
          match = false;
          break;
        }
      }
      if (match) return i;
    }
    return -1;
  }

  DataCell buildOperationCell(context, row, rows) {
    // Deep-copy snapshot to avoid sharing nested List/Map references with update dialog state
    final originalRow = json.decode(json.encode(row)) as Map<String, dynamic>;
    originalRow.remove(rowUniqueIdKey);

    return DataCell(
      SizedBox(
        width: operationWidth,
        child: Row(
          mainAxisAlignment: MainAxisAlignment.start, // Align buttons to the start
          mainAxisSize: MainAxisSize.min, // Use minimum space needed
          children: [
            WoxButton.text(
              text: '',
              icon: Icon(Icons.edit, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
              padding: const EdgeInsets.symmetric(horizontal: 4),
              onPressed: () {
                showDialog(
                  context: context,
                  builder: (context) {
                    return WoxSettingPluginTableUpdate(
                      item: item,
                      row: row,
                      onUpdateValidate: onUpdateValidate,
                      onUpdate: (key, value) async {
                        // Re-read the latest rows from the current setting value
                        // to avoid using stale closure-captured data
                        var rowsJson = getSetting(key);
                        if (rowsJson == "") {
                          rowsJson = "[]";
                        }
                        var freshRows = json.decode(rowsJson) as List<dynamic>;

                        // Find the target row in fresh data by matching original field values
                        var idx = _findRowIndex(freshRows, originalRow);
                        if (idx >= 0) {
                          // Remove the unique key from the updated value before saving
                          var updatedRow = Map<String, dynamic>.from(value);
                          updatedRow.remove(rowUniqueIdKey);
                          freshRows[idx] = updatedRow;
                        }

                        updateConfig(key, json.encode(freshRows));
                      },
                    );
                  },
                );
              },
            ),
            WoxButton.text(
              text: '',
              icon: Icon(Icons.delete, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
              padding: const EdgeInsets.symmetric(horizontal: 4),
              onPressed: () {
                //confirm delete
                showDialog(
                  context: context,
                  builder: (context) {
                    final themeBackground = getThemeBackgroundColor();
                    final isDarkTheme = themeBackground.computeLuminance() < 0.5;
                    final baseSurface = themeBackground.withAlpha(255);
                    final cardColor = (isDarkTheme ? baseSurface.lighter(12) : baseSurface.darker(6)).withAlpha(255);
                    final outlineColor = getThemeActiveBackgroundColor().withOpacity(isDarkTheme ? 0.22 : 0.15);

                    return AlertDialog(
                      backgroundColor: cardColor,
                      surfaceTintColor: Colors.transparent,
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20), side: BorderSide(color: outlineColor)),
                      content: Text(tr("ui_delete_row_confirm"), style: TextStyle(color: getThemeTextColor())),
                      actions: [
                        Row(
                          mainAxisAlignment: MainAxisAlignment.end,
                          children: [
                            WoxButton.secondary(text: tr("ui_cancel"), onPressed: () => Navigator.pop(context)),
                            const SizedBox(width: 16),
                            WoxButton.primary(
                              text: tr("ui_delete"),
                              onPressed: () {
                                Navigator.pop(context);

                                // Re-read the latest rows from the current setting value
                                var rowsJson = getSetting(item.key);
                                if (rowsJson == "") {
                                  rowsJson = "[]";
                                }
                                var freshRows = json.decode(rowsJson) as List<dynamic>;

                                // Find and remove the target row by matching original field values
                                var idx = _findRowIndex(freshRows, originalRow);
                                if (idx >= 0) {
                                  freshRows.removeAt(idx);
                                }

                                updateConfig(item.key, json.encode(freshRows));
                              },
                            ),
                          ],
                        ),
                      ],
                    );
                  },
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget buildEmptyTable() {
    return ConstrainedBox(
      constraints: const BoxConstraints(maxHeight: 100),
      child: Scrollbar(
        thumbVisibility: true,
        controller: horizontalScrollController,
        child: SingleChildScrollView(
          controller: horizontalScrollController,
          scrollDirection: Axis.horizontal,
          child: SizedBox(
            width: tableWidth,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                DataTable(
                  columnSpacing: columnSpacing,
                  horizontalMargin: 5,
                  clipBehavior: Clip.hardEdge,
                  headingRowHeight: 36,
                  dataRowMinHeight: 36,
                  dataRowMaxHeight: 36,
                  headingRowColor: WidgetStateProperty.resolveWith((states) => safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor)),
                  border: TableBorder.all(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewSplitLineColor)),
                  columns: [
                    for (var column in item.columns)
                      DataColumn(
                        label: columnWidth(
                          column: column,
                          isHeader: false,
                          isOperation: false,
                          child: Tooltip(
                            message: tr(column.label),
                            child: Text(
                              tr(column.label),
                              overflow: TextOverflow.ellipsis,
                              maxLines: 1,
                              style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor)),
                            ),
                          ),
                        ),
                      ),
                    DataColumn(
                      label: columnWidth(
                        column: PluginSettingValueTableColumn.fromJson(<String, dynamic>{
                          "Key": "Operation",
                          "Label": tr("ui_operation"),
                          "Tooltip": "",
                          "Width": operationWidth.toInt(),
                          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
                          "TextMaxLines": 1,
                        }),
                        isHeader: false,
                        isOperation: true,
                        child: Tooltip(
                          message: tr("ui_operation"),
                          child: Text(
                            tr("ui_operation"),
                            overflow: TextOverflow.ellipsis,
                            maxLines: 1,
                            style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor), fontSize: 13),
                          ),
                        ),
                      ),
                    ),
                  ],
                  rows: const [],
                ),
                Center(child: Padding(padding: const EdgeInsets.only(top: 10), child: Text(tr("ui_no_data"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)))),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget buildTable(BuildContext context) {
    var rowsJson = getSetting(item.key);
    if (rowsJson == "") {
      return buildEmptyTable();
    }
    var rows = json.decode(rowsJson);
    if (rows == null || rows.isEmpty) {
      return buildEmptyTable();
    }

    //give each row a unique key
    for (var row in rows) {
      row[rowUniqueIdKey] = const UuidV4().generate();
    }

    //sort the rows if needed
    if (item.sortColumnKey.isNotEmpty) {
      rows.sort((a, b) {
        var aValue = a[item.sortColumnKey] ?? "";
        var bValue = b[item.sortColumnKey] ?? "";
        if (item.sortOrder == "asc") {
          return aValue.toString().compareTo(bValue.toString());
        } else {
          return bValue.toString().compareTo(aValue.toString());
        }
      });
    }

    var dataRows = <DataRow>[];
    for (var row in rows) {
      dataRows.add(
        DataRow(
          cells: [
            for (var column in item.columns)
              if (!column.hideInTable) DataCell(buildRowCell(column, row)),
            if (!readonly) buildOperationCell(context, row, rows),
          ],
        ),
      );
    }

    var tableMaxHeight = item.maxHeight;
    if (tableMaxHeight < tableMaxHeightMin) {
      tableMaxHeight = tableMaxHeightMin;
    }

    return Scrollbar(
      controller: horizontalScrollController,
      child: SingleChildScrollView(
        controller: horizontalScrollController,
        scrollDirection: Axis.horizontal,
        child: ConstrainedBox(
          // Keep a bounded viewport height for vertical scrolling, but allow width to grow
          // beyond tableWidth when columns sum exceeds it so that horizontal scroll works.
          constraints: BoxConstraints(maxHeight: tableMaxHeight.toDouble(), minWidth: tableWidth),
          child: Scrollbar(
            controller: verticalScrollController,
            child: SingleChildScrollView(
              controller: verticalScrollController,
              child: DataTable(
                columnSpacing: columnSpacing,
                horizontalMargin: 5,
                headingRowHeight: 36,
                dataRowMinHeight: 36,
                dataRowMaxHeight: 36,
                headingRowColor: WidgetStateProperty.resolveWith((states) => safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor)),
                border: TableBorder.all(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewSplitLineColor)),
                columns: [
                  for (var column in item.columns)
                    if (!column.hideInTable) DataColumn(label: columnWidth(column: column, isHeader: true, isOperation: false, child: buildHeaderCell(column))),
                  if (!readonly)
                    DataColumn(
                      label: columnWidth(
                        column: PluginSettingValueTableColumn.fromJson(<String, dynamic>{
                          "Key": "Operation",
                          "Label": tr("ui_operation"),
                          "Tooltip": "",
                          "Width": operationWidth.toInt(),
                          "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
                          "TextMaxLines": 1,
                        }),
                        isHeader: true,
                        isOperation: true,
                        child: Tooltip(
                          message: tr("ui_operation"),
                          child: Text(
                            tr("ui_operation"),
                            overflow: TextOverflow.ellipsis,
                            maxLines: 1,
                            style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor), fontSize: 13),
                          ),
                        ),
                      ),
                    ),
                ],
                rows: dataRows,
              ),
            ),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: tableWidth,
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Row(
                  children: [
                    Text(item.title, style: TextStyle(color: getThemeTextColor(), fontSize: 13)),
                    if (item.tooltip != "") WoxTooltipIconView(tooltip: item.tooltip, color: getThemeTextColor()),
                  ],
                ),
                if (!readonly)
                  WoxButton.text(
                    text: tr("ui_add"),
                    icon: Icon(Icons.add, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
                    padding: EdgeInsets.zero,
                    onPressed: () {
                      showDialog(
                        context: context,
                        builder: (context) {
                          return WoxSettingPluginTableUpdate(
                            item: item,
                            row: const {},
                            onUpdate: (key, row) {
                              var rowsJson = getSetting(key);
                              if (rowsJson == "") {
                                rowsJson = "[]";
                              }
                              var rows = json.decode(rowsJson);
                              rows.add(row);
                              //remove the unique key
                              rows.forEach((element) {
                                element.remove(rowUniqueIdKey);
                              });

                              updateConfig(key, json.encode(rows));
                            },
                          );
                        },
                      );
                    },
                  ),
              ],
            ),
          ),
          const SizedBox(height: 6),
          buildTable(context),
        ],
      ),
    );
  }
}
