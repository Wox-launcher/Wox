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
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

import 'wox_setting_plugin_item_view.dart';
import 'wox_setting_plugin_table_update_view.dart';

class WoxSettingPluginTable extends WoxSettingPluginItem {
  static const int tableMaxHeightMin = 120;
  static const double _headerRowHeight = 36;
  static const double _dataRowHeight = 36;
  static const double _tableHorizontalMargin = 5;
  static const ScrollPhysics _tableScrollPhysics = ClampingScrollPhysics();
  final PluginSettingValueTable item;
  static const String rowUniqueIdKey = "wox_table_row_id";
  final double tableWidth;
  final operationWidth = 80.0;
  final columnSpacing = 10.0;
  final columnTooltipWidth = 20.0;
  final bool readonly;
  final Future<String?> Function(Map<String, dynamic> rowValues)? onUpdateValidate;
  final ScrollController horizontalHeaderScrollController = ScrollController();
  final ScrollController horizontalBodyScrollController = ScrollController();
  final ScrollController verticalBodyScrollController = ScrollController();
  final ScrollController verticalPinnedBodyScrollController = ScrollController();

  WoxSettingPluginTable({
    super.key,
    required this.item,
    required super.value,
    required super.onUpdate,
    super.labelWidth = SETTING_LABEL_DEAULT_WIDTH,
    this.tableWidth = 740.0,
    this.readonly = false,
    this.onUpdateValidate,
  }) {
    _setupScrollSync();
  }

  void _setupScrollSync() {
    horizontalBodyScrollController.addListener(() {
      _syncScrollOffset(horizontalBodyScrollController, horizontalHeaderScrollController);
    });

    verticalBodyScrollController.addListener(() {
      _syncScrollOffset(verticalBodyScrollController, verticalPinnedBodyScrollController);
    });

    verticalPinnedBodyScrollController.addListener(() {
      _syncScrollOffset(verticalPinnedBodyScrollController, verticalBodyScrollController);
    });
  }

  void _syncScrollOffset(ScrollController source, ScrollController target) {
    if (!source.hasClients || !target.hasClients) {
      return;
    }

    final targetPosition = target.position;
    final clampedOffset = source.offset.clamp(targetPosition.minScrollExtent, targetPosition.maxScrollExtent);
    if ((target.offset - clampedOffset).abs() < 0.5) {
      return;
    }

    target.jumpTo(clampedOffset);
  }

  PluginSettingValueTableColumn buildOperationColumnDefinition() {
    return PluginSettingValueTableColumn.fromJson(<String, dynamic>{
      "Key": "Operation",
      "Label": tr("ui_operation"),
      "Tooltip": "",
      "Width": operationWidth.toInt(),
      "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeText,
      "TextMaxLines": 1,
    });
  }

  List<PluginSettingValueTableColumn> buildVisibleColumns() {
    return item.columns.where((column) => !column.hideInTable).toList();
  }

  double resolveColumnWidth({required PluginSettingValueTableColumn column, required bool isOperation}) {
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
    return width.toDouble();
  }

  double estimateDataTableWidth(List<PluginSettingValueTableColumn> columns, {required bool includeOperationColumn}) {
    if (columns.isEmpty && !includeOperationColumn) {
      return 0;
    }

    final allColumns = <({PluginSettingValueTableColumn column, bool isOperation})>[
      for (final column in columns) (column: column, isOperation: false),
      if (includeOperationColumn) (column: buildOperationColumnDefinition(), isOperation: true),
    ];

    var width = _tableHorizontalMargin * 2;
    for (var index = 0; index < allColumns.length; index++) {
      final current = allColumns[index];
      width += resolveColumnWidth(column: current.column, isOperation: current.isOperation);
      if (index != allColumns.length - 1) {
        width += columnSpacing;
      }
    }

    return width;
  }

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
    return SizedBox(width: resolveColumnWidth(column: column, isOperation: isOperation), child: Align(alignment: Alignment.centerLeft, child: child));
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
      PluginSettingValueSelectOption selectOption;
      if (column.selectOptions.isEmpty) {
        selectOption = PluginSettingValueSelectOption.fromJson(<String, dynamic>{});
      } else {
        selectOption = column.selectOptions.firstWhere((element) => element.value == value, orElse: () => column.selectOptions.first);
      }
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

  DataCell buildOperationCell(BuildContext context, Map<String, dynamic> row, List<dynamic> rows) {
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
                        if (rowsJson == "" || rowsJson == "null") {
                          rowsJson = "[]";
                        }
                        var decoded = json.decode(rowsJson);
                        var freshRows = decoded is List ? decoded : [];

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
                    final outlineColor = getThemeActiveBackgroundColor().withValues(alpha: isDarkTheme ? 0.22 : 0.15);

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
                                if (rowsJson == "" || rowsJson == "null") {
                                  rowsJson = "[]";
                                }
                                var decoded = json.decode(rowsJson);
                                var freshRows = decoded is List ? decoded : [];

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

  Widget buildOperationHeaderCell() {
    final operationColumn = buildOperationColumnDefinition();
    return columnWidth(
      column: operationColumn,
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
    );
  }

  DataColumn buildHeaderDataColumn(PluginSettingValueTableColumn column, {required bool isOperation}) {
    return DataColumn(label: isOperation ? buildOperationHeaderCell() : columnWidth(column: column, isHeader: true, isOperation: false, child: buildHeaderCell(column)));
  }

  DataColumn buildBodyDataColumn(PluginSettingValueTableColumn column, {required bool isOperation}) {
    return DataColumn(label: columnWidth(column: column, isHeader: false, isOperation: isOperation, child: const SizedBox.shrink()));
  }

  DataTable buildStyledTable({
    required List<DataColumn> columns,
    required List<DataRow> rows,
    required double headingRowHeight,
    bool showTopBorder = true,
    bool showBottomBorder = true,
    bool showLeftBorder = true,
    bool showRightBorder = true,
    Color? topBorderColor,
    Color? leftBorderColor,
    Color? rightBorderColor,
    Color? bottomBorderColor,
  }) {
    final borderColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewSplitLineColor);

    return DataTable(
      columnSpacing: columnSpacing,
      horizontalMargin: _tableHorizontalMargin,
      clipBehavior: Clip.hardEdge,
      dividerThickness: 0,
      headingRowHeight: headingRowHeight,
      dataRowMinHeight: _dataRowHeight,
      dataRowMaxHeight: _dataRowHeight,
      headingRowColor: WidgetStateProperty.resolveWith((states) => safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor)),
      border: TableBorder(
        top: showTopBorder ? BorderSide(color: topBorderColor ?? borderColor) : BorderSide.none,
        bottom: showBottomBorder ? BorderSide(color: bottomBorderColor ?? borderColor) : BorderSide.none,
        left: showLeftBorder ? BorderSide(color: leftBorderColor ?? borderColor) : BorderSide.none,
        right: showRightBorder ? BorderSide(color: rightBorderColor ?? borderColor) : BorderSide.none,
        horizontalInside: BorderSide(color: borderColor),
        verticalInside: BorderSide(color: borderColor),
      ),
      columns: columns,
      rows: rows,
    );
  }

  Widget buildEmptyTable() {
    final visibleColumns = buildVisibleColumns();
    final headerBorderColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor);
    if (visibleColumns.isEmpty) {
      return Center(child: Text(tr("ui_no_data"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)));
    }

    return ConstrainedBox(
      constraints: const BoxConstraints(maxHeight: 100),
      child: Scrollbar(
        thumbVisibility: true,
        controller: horizontalBodyScrollController,
        child: SingleChildScrollView(
          controller: horizontalBodyScrollController,
          physics: _tableScrollPhysics,
          scrollDirection: Axis.horizontal,
          child: SizedBox(
            width: tableWidth,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                buildStyledTable(
                  columns: [
                    for (var column in visibleColumns) buildHeaderDataColumn(column, isOperation: false),
                    if (!readonly) buildHeaderDataColumn(buildOperationColumnDefinition(), isOperation: true),
                  ],
                  headingRowHeight: _headerRowHeight,
                  rows: const [],
                  topBorderColor: headerBorderColor,
                  showBottomBorder: false,
                  leftBorderColor: headerBorderColor,
                  rightBorderColor: headerBorderColor,
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
    if (rowsJson == "" || rowsJson == "null") {
      return buildEmptyTable();
    }
    var decoded = json.decode(rowsJson);
    if (decoded == null || decoded is! List) {
      return buildEmptyTable();
    }
    var rows = decoded;
    if (rows.isEmpty) {
      return buildEmptyTable();
    }

    //give each row a unique key
    for (var row in rows) {
      (row as Map<String, dynamic>)[rowUniqueIdKey] = const UuidV4().generate();
    }

    //sort the rows if needed
    if (item.sortColumnKey.isNotEmpty) {
      rows.sort((a, b) {
        final rowA = a as Map<String, dynamic>;
        final rowB = b as Map<String, dynamic>;
        var aValue = rowA[item.sortColumnKey] ?? "";
        var bValue = rowB[item.sortColumnKey] ?? "";
        if (item.sortOrder == "asc") {
          return aValue.toString().compareTo(bValue.toString());
        } else {
          return bValue.toString().compareTo(aValue.toString());
        }
      });
    }

    final visibleColumns = buildVisibleColumns();
    if (visibleColumns.isEmpty && readonly) {
      return buildEmptyTable();
    }

    final pinnedColumn = !readonly ? buildOperationColumnDefinition() : visibleColumns.last;
    final pinnedIsOperation = !readonly;
    final leftColumns = pinnedIsOperation ? visibleColumns : visibleColumns.sublist(0, visibleColumns.length - 1);

    final leftDataRows = <DataRow>[];
    if (leftColumns.isNotEmpty) {
      for (final row in rows) {
        final rowMap = row as Map<String, dynamic>;
        leftDataRows.add(DataRow(cells: [for (final column in leftColumns) DataCell(buildRowCell(column, rowMap))]));
      }
    }

    final pinnedDataRows = <DataRow>[];
    for (final row in rows) {
      final rowMap = row as Map<String, dynamic>;
      pinnedDataRows.add(DataRow(cells: [if (pinnedIsOperation) DataCell(buildOperationCell(context, rowMap, rows).child) else DataCell(buildRowCell(pinnedColumn, rowMap))]));
    }

    var tableMaxHeight = item.maxHeight;
    if (tableMaxHeight < tableMaxHeightMin) {
      tableMaxHeight = tableMaxHeightMin;
    }

    final tableBodyMaxHeight = (tableMaxHeight - _headerRowHeight).clamp(_dataRowHeight, double.infinity).toDouble();
    final tableBodyContentHeight = rows.length * _dataRowHeight;
    final tableBodyHeight = tableBodyContentHeight > tableBodyMaxHeight ? tableBodyMaxHeight : tableBodyContentHeight;
    final pinnedSectionWidth = estimateDataTableWidth(pinnedIsOperation ? const [] : [pinnedColumn], includeOperationColumn: pinnedIsOperation);
    final leftViewportWidth = (tableWidth - pinnedSectionWidth).clamp(0.0, tableWidth);
    final leftContentWidth = estimateDataTableWidth(leftColumns, includeOperationColumn: false);
    final leftSectionMinWidth =
        leftColumns.isEmpty
            ? 0.0
            : leftContentWidth > leftViewportWidth
            ? leftContentWidth
            : leftViewportWidth;
    final headerBorderColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor);

    if (leftColumns.isEmpty) {
      return SizedBox(
        width: tableWidth,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              width: pinnedSectionWidth,
              child: buildStyledTable(
                columns: [buildHeaderDataColumn(pinnedColumn, isOperation: pinnedIsOperation)],
                rows: const [],
                headingRowHeight: _headerRowHeight,
                topBorderColor: headerBorderColor,
                showBottomBorder: false,
                leftBorderColor: headerBorderColor,
                rightBorderColor: headerBorderColor,
              ),
            ),
            SizedBox(
              width: pinnedSectionWidth,
              height: tableBodyHeight,
              child: Scrollbar(
                controller: verticalPinnedBodyScrollController,
                child: SingleChildScrollView(
                  controller: verticalPinnedBodyScrollController,
                  physics: _tableScrollPhysics,
                  child: buildStyledTable(
                    columns: [buildBodyDataColumn(pinnedColumn, isOperation: pinnedIsOperation)],
                    rows: pinnedDataRows,
                    headingRowHeight: 0,
                    showTopBorder: false,
                  ),
                ),
              ),
            ),
          ],
        ),
      );
    }

    return SizedBox(
      width: tableWidth,
      child: Column(
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: SingleChildScrollView(
                  controller: horizontalHeaderScrollController,
                  physics: const NeverScrollableScrollPhysics(),
                  scrollDirection: Axis.horizontal,
                  child: ConstrainedBox(
                    constraints: BoxConstraints(minWidth: leftSectionMinWidth),
                    child: buildStyledTable(
                      columns: [for (final column in leftColumns) buildHeaderDataColumn(column, isOperation: false)],
                      rows: const [],
                      headingRowHeight: _headerRowHeight,
                      showRightBorder: false,
                      topBorderColor: headerBorderColor,
                      showBottomBorder: false,
                      leftBorderColor: headerBorderColor,
                    ),
                  ),
                ),
              ),
              SizedBox(
                width: pinnedSectionWidth,
                child: buildStyledTable(
                  columns: [buildHeaderDataColumn(pinnedColumn, isOperation: pinnedIsOperation)],
                  rows: const [],
                  headingRowHeight: _headerRowHeight,
                  topBorderColor: headerBorderColor,
                  showBottomBorder: false,
                  rightBorderColor: headerBorderColor,
                ),
              ),
            ],
          ),
          SizedBox(
            height: tableBodyHeight,
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Scrollbar(
                    controller: horizontalBodyScrollController,
                    thumbVisibility: true,
                    child: SingleChildScrollView(
                      controller: horizontalBodyScrollController,
                      physics: _tableScrollPhysics,
                      scrollDirection: Axis.horizontal,
                      child: ConstrainedBox(
                        constraints: BoxConstraints(minWidth: leftSectionMinWidth),
                        child: SingleChildScrollView(
                          controller: verticalBodyScrollController,
                          physics: _tableScrollPhysics,
                          child: buildStyledTable(
                            columns: [for (final column in leftColumns) buildBodyDataColumn(column, isOperation: false)],
                            rows: leftDataRows,
                            headingRowHeight: 0,
                            showTopBorder: false,
                            showRightBorder: false,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
                SizedBox(
                  width: pinnedSectionWidth,
                  child: Scrollbar(
                    controller: verticalPinnedBodyScrollController,
                    thumbVisibility: true,
                    child: SingleChildScrollView(
                      controller: verticalPinnedBodyScrollController,
                      physics: _tableScrollPhysics,
                      child: buildStyledTable(
                        columns: [buildBodyDataColumn(pinnedColumn, isOperation: pinnedIsOperation)],
                        rows: pinnedDataRows,
                        headingRowHeight: 0,
                        showTopBorder: false,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: layout(
        label: item.title,
        style: item.style,
        tooltip: item.tooltip,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (!readonly)
              SizedBox(
                width: tableWidth,
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
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
                                if (rowsJson == "" || rowsJson == "null") {
                                  rowsJson = "[]";
                                }
                                var decoded = json.decode(rowsJson);
                                var rows = decoded is List ? decoded : [];
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
            if (!readonly) const SizedBox(height: 6),
            buildTable(context),
          ],
        ),
      ),
    );
  }
}
