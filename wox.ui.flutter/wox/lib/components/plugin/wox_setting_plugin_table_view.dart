import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/components/wox_tooltip_icon_view.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_setting_focus_util.dart';

import 'wox_setting_plugin_item_view.dart';
import 'wox_setting_plugin_table_update_view.dart';

typedef WoxSettingPluginTableCreateDialogBuilder =
    Future<void> Function(BuildContext context, Future<String?> Function(Map<String, dynamic> row) saveRow, {Map<String, dynamic> initialRow});
typedef WoxSettingPluginTableEditDialogBuilder = Future<void> Function(BuildContext context, Map<String, dynamic> row, Future<String?> Function(Map<String, dynamic> row) saveRow);

class WoxSettingPluginTable extends WoxSettingPluginItem {
  static const int tableMaxHeightMin = 120;
  static const double _headerRowHeight = 36;
  static const double _dataRowHeight = 36;
  static const double _tableHorizontalMargin = 5;
  static const ScrollPhysics _tableScrollPhysics = ClampingScrollPhysics();
  final PluginSettingValueTable item;
  static const String rowUniqueIdKey = "wox_table_row_id";
  final double tableWidth;
  final operationWidth = 120.0;
  final columnSpacing = 10.0;
  final columnTooltipWidth = 20.0;
  final bool readonly;
  final bool inlineTitleActions;
  final List<Widget> titleActions;
  final List<Widget> trailingActions;
  final int minimumRowCount;
  final String minimumRowDeleteMessage;
  final WoxSettingPluginTableCreateDialogBuilder? customCreateDialogBuilder;
  final WoxSettingPluginTableEditDialogBuilder? customEditDialogBuilder;
  final Widget? Function(PluginSettingValueTableColumn column, Map<String, dynamic> row)? customCellBuilder;
  final Future<List<PluginSettingTableValidationError>> Function(Map<String, dynamic> rowValues)? onUpdateValidate;
  final int? autoOpenEditRowIndex;
  final ScrollController horizontalHeaderScrollController = ScrollController();
  final ScrollController horizontalBodyScrollController = ScrollController();
  final ScrollController verticalBodyScrollController = ScrollController();
  final ScrollController verticalPinnedBodyScrollController = ScrollController();
  final Map<String, Future<String>> _aiModelStatusFutures = <String, Future<String>>{};
  final Map<String, Object> _aiModelStatusErrors = <String, Object>{};
  final Set<String> _aiModelStatusSuccessKeys = <String>{};

  WoxSettingPluginTable({
    super.key,
    required this.item,
    required super.value,
    required super.onUpdate,
    super.labelWidth = PLUGIN_SETTING_LABEL_WIDTH,
    this.tableWidth = PLUGIN_SETTING_TABLE_WIDTH,
    this.readonly = false,
    this.inlineTitleActions = false,
    this.titleActions = const [],
    this.trailingActions = const [],
    this.minimumRowCount = 0,
    this.minimumRowDeleteMessage = "",
    this.customCreateDialogBuilder,
    this.customEditDialogBuilder,
    this.customCellBuilder,
    this.onUpdateValidate,
    this.autoOpenEditRowIndex,
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

  List<dynamic> decodeRowsJson(String rowsJson) {
    final normalized = rowsJson.trim();
    if (normalized.isEmpty || normalized == "null") {
      return [];
    }

    try {
      final decoded = json.decode(normalized);
      if (decoded is List) {
        return decoded;
      }
    } catch (_) {
      // Ignore invalid persisted table data and render an empty table instead of crashing.
    }

    return [];
  }

  AIModel? decodeAIModel(dynamic value) {
    if (value is! String || value.trim().isEmpty) {
      return null;
    }

    try {
      return AIModel.fromJson(json.decode(value));
    } catch (_) {
      return null;
    }
  }

  IgnoredHotkeyApp? decodeIgnoredHotkeyApp(dynamic value) {
    if (value is IgnoredHotkeyApp) {
      return value.identity.trim().isEmpty ? null : value;
    }
    if (value is Map<String, dynamic>) {
      final app = IgnoredHotkeyApp.fromJson(value);
      return app.identity.trim().isEmpty ? null : app;
    }
    if (value is Map) {
      final app = IgnoredHotkeyApp.fromJson(Map<String, dynamic>.from(value));
      return app.identity.trim().isEmpty ? null : app;
    }
    if (value is String && value.trim().isNotEmpty) {
      try {
        final app = IgnoredHotkeyApp.fromJson(Map<String, dynamic>.from(jsonDecode(value.trim())));
        return app.identity.trim().isEmpty ? null : app;
      } catch (_) {
        return null;
      }
    }

    return null;
  }

  Future<String> getAIModelStatusFuture({required String cacheKey, required String providerName, required String apiKey, required String host}) {
    // Keep ping futures stable across hover-triggered table rebuilds so finished status dots do not flash back to the waiting state.
    return _aiModelStatusFutures.putIfAbsent(cacheKey, () async {
      try {
        final result = await WoxApi.instance.pingAIModel(const UuidV4().generate(), providerName, apiKey, host);
        _aiModelStatusErrors.remove(cacheKey);
        _aiModelStatusSuccessKeys.add(cacheKey);
        return result;
      } catch (error) {
        _aiModelStatusSuccessKeys.remove(cacheKey);
        _aiModelStatusErrors[cacheKey] = error;
        rethrow;
      }
    });
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
    const defaultFlexibleColumnWidth = 100.0;
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
      final availableWidth = tableWidth - totalWidth - (operationWidth + columnSpacing) - totalColumnTooltipWidth;
      if (availableWidth > 0) {
        return availableWidth;
      }

      // When fixed-width columns already exceed the nominal table width,
      // fall back to a sane default instead of producing a negative column width.
      return defaultFlexibleColumnWidth;
    }

    return defaultFlexibleColumnWidth;
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
          child: Text(
            translatedLabel,
            overflow: TextOverflow.ellipsis,
            maxLines: 1,
            style: TextStyle(color: getThemeTextColor().withValues(alpha: 0.88), fontSize: 13, fontWeight: FontWeight.w600),
          ),
        ),
        if (column.tooltip != "") WoxTooltipIconView(tooltip: tr(column.tooltip), paddingRight: 0, color: getThemeTextColor().withValues(alpha: 0.72)),
      ],
    );
  }

  Widget buildRowCell(PluginSettingValueTableColumn column, Map<String, dynamic> row) {
    var value = row[column.key] ?? "";
    final customCell = customCellBuilder?.call(column, row);
    if (customCell != null) {
      // Some built-in setting tables need domain-specific display without changing
      // persisted values. Keeping the hook here lets callers polish cells such as
      // global trigger keywords while the generic table still owns sizing.
      return columnWidth(column: column, isHeader: false, isOperation: false, child: customCell);
    }

    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeText ||
        column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeQueryHotkeyQuery ||
        column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAICommandPrompt) {
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
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeApp) {
      final app = decodeIgnoredHotkeyApp(value);
      if (app == null) {
        return columnWidth(column: column, isHeader: false, isOperation: false, child: const SizedBox.shrink());
      }

      return columnWidth(
        column: column,
        isHeader: false,
        isOperation: false,
        child: Row(
          children: [
            if (app.icon.imageData.isNotEmpty) ...[WoxImageView(woxImage: app.icon, width: 18, height: 18), const SizedBox(width: 8)],
            Expanded(
              child: Text(
                app.name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor)),
              ),
            ),
          ],
        ),
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

      try {
        final woxImage = WoxImage.fromJson(jsonDecode(value));
        return Row(children: [WoxImageView(woxImage: woxImage, width: 24, height: 24)]);
      } catch (_) {
        return const SizedBox.shrink();
      }
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
        child: Row(
          children: [
            if (selectOption.icon.imageData.isNotEmpty) ...[WoxImageView(woxImage: selectOption.icon, width: 18, height: 18), const SizedBox(width: 8)],
            Expanded(
              child: Text(
                selectOption.label,
                style: TextStyle(overflow: TextOverflow.ellipsis, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor)),
              ),
            ),
          ],
        ),
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeSelectAIModel) {
      final model = decodeAIModel(value);
      if (model == null) {
        return columnWidth(
          column: column,
          isHeader: false,
          isOperation: false,
          child: Text("", style: TextStyle(overflow: TextOverflow.ellipsis, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
        );
      }

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
      var apiKey = row["ApiKey"] ?? "";
      var host = row["Host"] ?? "";
      final statusCacheKey = json.encode([providerName, apiKey, host]);

      if (_aiModelStatusErrors.containsKey(statusCacheKey)) {
        return columnWidth(
          column: column,
          isHeader: false,
          isOperation: false,
          child: WoxTooltip(message: _aiModelStatusErrors[statusCacheKey]?.toString() ?? "", child: const Icon(Icons.circle, color: Colors.red)),
        );
      }

      if (_aiModelStatusSuccessKeys.contains(statusCacheKey)) {
        return columnWidth(column: column, isHeader: false, isOperation: false, child: const Icon(Icons.circle, color: Colors.green));
      }

      return FutureBuilder<String>(
        future: getAIModelStatusFuture(cacheKey: statusCacheKey, providerName: providerName, apiKey: apiKey, host: host),
        builder: (context, snapshot) {
          return columnWidth(
            column: column,
            isHeader: false,
            isOperation: false,
            child:
                snapshot.connectionState == ConnectionState.waiting
                    ? const Icon(Icons.circle, color: Colors.grey)
                    : snapshot.error != null
                    ? WoxTooltip(message: snapshot.error?.toString() ?? "", child: const Icon(Icons.circle, color: Colors.red))
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
                    ? WoxTooltip(message: snapshot.error?.toString() ?? "", child: const Icon(Icons.circle, color: Colors.red))
                    : WoxTooltip(
                      message: snapshot.data?.map((e) => e.name).join("\n") ?? "",
                      child: Text("${snapshot.data?.length ?? 0} tools", style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor))),
                    ),
          );
        },
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeAISelectMCPServerTools) {
      final toolNames = value as List<dynamic>;
      return WoxTooltip(
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

  // Persists a new row using the latest table snapshot so custom create dialogs
  // can reuse the same save path as the shared table editor.
  Future<String?> _saveNewRow(Map<String, dynamic> row) async {
    final rows = decodeRowsJson(getSetting(item.key));
    rows.add(Map<String, dynamic>.from(row));

    for (final element in rows) {
      if (element is Map<String, dynamic>) {
        element.remove(rowUniqueIdKey);
      } else if (element is Map) {
        element.remove(rowUniqueIdKey);
      }
    }

    return updateConfig(item.key, json.encode(rows));
  }

  // Saves an edited row by re-matching the original snapshot instead of trusting
  // stale row indices from the visible table.
  Future<String?> _saveEditedRow(Map<String, dynamic> originalRow, Map<String, dynamic> updatedValues) async {
    final freshRows = decodeRowsJson(getSetting(item.key));
    final idx = _findRowIndex(freshRows, originalRow);
    if (idx < 0) {
      return "Failed to save row: the original row was not found";
    }

    final updatedRow = Map<String, dynamic>.from(updatedValues);
    updatedRow.remove(rowUniqueIdKey);
    freshRows[idx] = updatedRow;

    return updateConfig(item.key, json.encode(freshRows));
  }

  Future<void> _showEditRowDialog(BuildContext context, Map<String, dynamic> row) async {
    final originalRow = json.decode(json.encode(row)) as Map<String, dynamic>;
    originalRow.remove(rowUniqueIdKey);

    if (customEditDialogBuilder != null) {
      await customEditDialogBuilder!(context, Map<String, dynamic>.from(originalRow), (updatedRow) => _saveEditedRow(originalRow, updatedRow));
      WoxSettingFocusUtil.restoreIfInSettingView();
      return;
    }

    await showDialog(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) {
        return WoxSettingPluginTableUpdate(
          item: item,
          row: Map<String, dynamic>.from(originalRow),
          onUpdateValidate: onUpdateValidate,
          onUpdate: (key, value) async => _saveEditedRow(originalRow, value),
        );
      },
    );
    WoxSettingFocusUtil.restoreIfInSettingView();
  }

  // Opens the row editor with copied values while saving through the create path.
  Future<void> _showCloneRowDialog(BuildContext context, Map<String, dynamic> row) async {
    final clonedRow = json.decode(json.encode(row)) as Map<String, dynamic>;
    clonedRow.remove(rowUniqueIdKey);

    if (customCreateDialogBuilder != null) {
      await customCreateDialogBuilder!(context, _saveNewRow, initialRow: Map<String, dynamic>.from(clonedRow));
      WoxSettingFocusUtil.restoreIfInSettingView();
      return;
    }

    if (customEditDialogBuilder != null) {
      await customEditDialogBuilder!(context, Map<String, dynamic>.from(clonedRow), (updatedRow) => _saveNewRow(updatedRow));
      WoxSettingFocusUtil.restoreIfInSettingView();
      return;
    }

    await showDialog(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) {
        return WoxSettingPluginTableUpdate(
          item: item,
          row: Map<String, dynamic>.from(clonedRow),
          onUpdateValidate: onUpdateValidate,
          onUpdate: (key, value) async => _saveNewRow(value),
        );
      },
    );
    WoxSettingFocusUtil.restoreIfInSettingView();
  }

  void _scheduleAutoOpenEditDialog(BuildContext context, List<dynamic> rows) {
    if (readonly || autoOpenEditRowIndex == null) {
      return;
    }

    WidgetsBinding.instance.addPostFrameCallback((_) async {
      final settingController = Get.find<WoxSettingController>();
      if (autoOpenEditRowIndex! < 0 || autoOpenEditRowIndex! >= rows.length) {
        if (settingController.pendingTrayQueryEditRowIndex.value == autoOpenEditRowIndex) {
          settingController.consumePendingTrayQueryEditRowIndex();
        }
        return;
      }

      final requestedRowIndex = settingController.consumePendingTrayQueryEditRowIndex();
      if (requestedRowIndex != autoOpenEditRowIndex || !context.mounted) {
        return;
      }

      final row = rows[requestedRowIndex!] as Map<String, dynamic>;
      await _showEditRowDialog(context, row);
    });
  }

  DataCell buildOperationCell(BuildContext context, Map<String, dynamic> row, List<dynamic> rows) {
    final originalRow = json.decode(json.encode(row)) as Map<String, dynamic>;
    originalRow.remove(rowUniqueIdKey);
    final isDeleteDisabled = rows.length <= minimumRowCount;
    final deleteDisabledMessage = minimumRowDeleteMessage.trim().isEmpty ? tr("ui_plugin_table_minimum_row_delete_message") : tr(minimumRowDeleteMessage);

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
              onPressed: () async {
                await _showEditRowDialog(context, row);
              },
            ),
            WoxTooltip(
              message: tr("ui_clone_row"),
              child: WoxButton.text(
                text: '',
                icon: Icon(Icons.content_copy, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
                padding: const EdgeInsets.symmetric(horizontal: 4),
                onPressed: () async {
                  await _showCloneRowDialog(context, row);
                },
              ),
            ),
            WoxTooltip(
              message: isDeleteDisabled ? deleteDisabledMessage : "",
              child: WoxButton.text(
                text: '',
                icon: Icon(Icons.delete, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor).withValues(alpha: isDeleteDisabled ? 0.45 : 1)),
                padding: const EdgeInsets.symmetric(horizontal: 4),
                onPressed:
                    isDeleteDisabled
                        ? null
                        : () async {
                          //confirm delete
                          await showDialog(
                            context: context,
                            barrierColor: getThemePopupBarrierColor(),
                            builder: (context) {
                              final cardColor = getThemePopupSurfaceColor();
                              final outlineColor = getThemePopupOutlineColor();

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

                                          // Re-read the latest rows before deleting so minimum-row
                                          // constraints remain correct when the table changed while
                                          // the confirmation dialog was open.
                                          final freshRows = decodeRowsJson(getSetting(item.key));
                                          if (freshRows.length <= minimumRowCount) {
                                            return;
                                          }

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
                          WoxSettingFocusUtil.restoreIfInSettingView();
                        },
              ),
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
      child: WoxTooltip(
        message: tr("ui_operation"),
        child: Text(
          tr("ui_operation"),
          overflow: TextOverflow.ellipsis,
          maxLines: 1,
          style: TextStyle(color: getThemeTextColor().withValues(alpha: 0.88), fontSize: 13, fontWeight: FontWeight.w600),
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
    // Settings table grid lines should match the surrounding settings dividers.
    // The previous local alpha made table borders look like a separate visual system.
    final borderColor = getThemeSettingDividerColor();

    return DataTable(
      columnSpacing: columnSpacing,
      horizontalMargin: _tableHorizontalMargin,
      clipBehavior: Clip.hardEdge,
      dividerThickness: 0,
      headingRowHeight: headingRowHeight,
      dataRowMinHeight: _dataRowHeight,
      dataRowMaxHeight: _dataRowHeight,
      headingRowColor: WidgetStateProperty.resolveWith((states) => getThemeTextColor().withValues(alpha: 0.055)),
      dataRowColor: WidgetStateProperty.resolveWith((states) => getThemeTextColor().withValues(alpha: 0.018)),
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

  Widget buildEmptyTable({required bool showScrollbars}) {
    final visibleColumns = buildVisibleColumns();
    final headerBorderColor = getThemeSettingDividerColor();
    if (visibleColumns.isEmpty) {
      return Center(child: Text(tr("ui_no_data"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)));
    }

    return ConstrainedBox(
      // The empty table includes both a header and an empty-state body; the previous 100px cap was smaller than their combined height and caused bottom overflow.
      constraints: const BoxConstraints(maxHeight: _headerRowHeight + 82),
      child: Scrollbar(
        thumbVisibility: showScrollbars,
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
                Container(
                  height: 82,
                  decoration: BoxDecoration(
                    border: Border(left: BorderSide(color: headerBorderColor), right: BorderSide(color: headerBorderColor), bottom: BorderSide(color: headerBorderColor)),
                  ),
                  child: Center(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.inbox_outlined, color: getThemeSubTextColor().withValues(alpha: 0.72), size: 24),
                        const SizedBox(height: 4),
                        Text(tr("ui_no_data"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget buildTable(BuildContext context, {required bool showScrollbars}) {
    final rows = decodeRowsJson(getSetting(item.key));
    if (rows.isEmpty) {
      return buildEmptyTable(showScrollbars: showScrollbars);
    }

    //give each row a unique key
    for (var row in rows) {
      (row as Map<String, dynamic>)[rowUniqueIdKey] = const UuidV4().generate();
    }

    _scheduleAutoOpenEditDialog(context, rows);

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
      return buildEmptyTable(showScrollbars: showScrollbars);
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
    final headerBorderColor = getThemeSettingDividerColor();

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
                thumbVisibility: showScrollbars,
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
                    thumbVisibility: showScrollbars,
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
                    thumbVisibility: showScrollbars,
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

  Widget buildAddButton(BuildContext context) {
    // Table creation should be a compact outlined action that can sit beside table
    // titles in top-level settings instead of forcing a separate full-width row.
    return WoxButton.secondary(
      text: tr("ui_add"),
      icon: Icon(Icons.add, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
      height: 30,
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      onPressed: () async {
        if (customCreateDialogBuilder != null) {
          await customCreateDialogBuilder!(context, _saveNewRow);
          WoxSettingFocusUtil.restoreIfInSettingView();
          return;
        }

        await showDialog(
          context: context,
          barrierColor: getThemePopupBarrierColor(),
          builder: (context) {
            return WoxSettingPluginTableUpdate(item: item, row: const {}, onUpdateValidate: onUpdateValidate, onUpdate: (key, row) async => _saveNewRow(row));
          },
        );
        WoxSettingFocusUtil.restoreIfInSettingView();
      },
    );
  }

  Widget buildInlineTitleHeader(BuildContext context) {
    final hasTitle = item.title.trim().isNotEmpty;
    final hasTooltip = item.tooltip.trim().isNotEmpty;
    final hasAction = !readonly || titleActions.isNotEmpty || trailingActions.isNotEmpty;

    if (!hasTitle && !hasTooltip && !hasAction) {
      return const SizedBox.shrink();
    }

    return SizedBox(
      width: tableWidth,
      child: Row(
        // Long table tips can wrap to multiple lines. Bottom-align the action so its
        // distance to the table stays fixed regardless of the description height.
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (hasTitle || titleActions.isNotEmpty)
                  Row(
                    mainAxisSize: MainAxisSize.max,
                    children: [
                      if (hasTitle)
                        Flexible(
                          fit: FlexFit.loose,
                          child: Text(
                            tr(item.title),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w600),
                          ),
                        ),
                      if (titleActions.isNotEmpty) ...[
                        const SizedBox(width: 6),
                        // Feature refinement: demo triggers sit directly beside the table title so the preview affordance is attached to the feature name instead of competing with the Add button.
                        ...titleActions,
                      ],
                    ],
                  ),
                if (hasTooltip) Padding(padding: const EdgeInsets.only(top: 4), child: tooltipText(item.tooltip)),
              ],
            ),
          ),
          if (trailingActions.isNotEmpty || !readonly) ...[
            const SizedBox(width: 16),
            Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                for (final action in trailingActions) ...[action, const SizedBox(width: 8)],
                if (!readonly) buildAddButton(context),
              ],
            ),
          ],
        ],
      ),
    );
  }

  Widget buildHoverAwareTable(BuildContext context) {
    var isHovered = false;

    return StatefulBuilder(
      builder: (context, setState) {
        // Scrollbars are useful when the pointer is working inside a table, but keeping
        // them visible all the time adds visual noise to long settings pages.
        return MouseRegion(
          onEnter: (_) => setState(() => isHovered = true),
          onExit: (_) => setState(() => isHovered = false),
          child: buildTable(context, showScrollbars: isHovered),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    if (inlineTitleActions) {
      // Built-in settings pages use full-width table blocks; keeping title, help text,
      // and Add in the table component makes the header match the table edge exactly.
      return applyStylePadding(
        style: item.style,
        child: Padding(
          padding: const EdgeInsets.only(top: 6, bottom: 10),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [buildInlineTitleHeader(context), const SizedBox(height: 8), buildHoverAwareTable(context)]),
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.only(top: 6),
      child: layout(
        label: item.title,
        style: item.style,
        tooltip: item.tooltip,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (trailingActions.isNotEmpty || !readonly)
              SizedBox(
                width: tableWidth,
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.end,
                  children: [
                    for (final action in trailingActions) ...[action, const SizedBox(width: 8)],
                    if (!readonly) buildAddButton(context),
                  ],
                ),
              ),
            if (trailingActions.isNotEmpty || !readonly) const SizedBox(height: 6),
            buildHoverAwareTable(context),
          ],
        ),
      ),
    );
  }
}
