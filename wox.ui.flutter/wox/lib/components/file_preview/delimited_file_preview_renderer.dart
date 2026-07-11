import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_policy.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class DelimitedFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "csv" || fileExtension == "tsv";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_delimited_not_found", {"path": context.filePath})));
    }

    final delimiter = context.fileExtension == "tsv" ? "\t" : ",";
    final typeLabel = context.fileExtension == "tsv" ? context.tr("ui_file_preview_type_tsv") : context.tr("ui_file_preview_type_csv");

    return WoxFilePreviewPolicy.buildDeferredPreview(
      context: context,
      file: file,
      manualLoadThresholdBytes: WoxFilePreviewPolicy.tableThresholdBytes,
      icon: Icons.table_chart_rounded,
      accent: const Color(0xFF22C55E),
      typeLabel: typeLabel,
      loadedPreviewHandlesScrolling: false,
      previewBuilder: (_) => _buildDelimitedPreview(file: file, delimiter: delimiter, typeLabel: typeLabel, tr: context.tr),
    );
  }
}

Widget _buildDelimitedPreview({required File file, required String delimiter, required String typeLabel, required WoxFilePreviewTranslationFormatter tr}) {
  final data = _loadDelimitedPreview(file, delimiter);

  return WoxFileInfoPreview(
    icon: Icons.table_chart_rounded,
    fileIconPath: file.path,
    accent: const Color(0xFF22C55E),
    title: path.basename(file.path),
    subtitle: tr("ui_file_preview_delimited_summary", {"rows": data.rowCount.toString(), "columns": data.columnCount.toString()}),
    properties: [
      ...buildWoxFilePreviewCommonProperties(file, typeLabel: typeLabel, tr: tr),
      WoxFilePreviewProperty(label: tr("ui_file_preview_delimited_rows"), value: data.rowCount.toString()),
      WoxFilePreviewProperty(label: tr("ui_file_preview_delimited_columns"), value: data.columnCount.toString()),
    ],
    sections: [
      WoxFilePreviewSection(
        title: data.hasMoreRows ? tr("ui_file_preview_delimited_preview_first", {"count": data.rows.length.toString()}) : tr("ui_file_preview_delimited_preview"),
        child: _DelimitedTablePreview(data: data),
      ),
    ],
  );
}

class _DelimitedTablePreview extends StatelessWidget {
  final _DelimitedPreviewData data;

  const _DelimitedTablePreview({required this.data});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    final borderColor = getThemeDividerColor().withValues(alpha: 0.32);

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: ConstrainedBox(
        constraints: BoxConstraints(minWidth: metrics.scaledSpacing(420)),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (var rowIndex = 0; rowIndex < data.rows.length; rowIndex++)
              Container(
                decoration: BoxDecoration(border: Border(bottom: BorderSide(color: borderColor))),
                child: Row(
                  children: [
                    for (var columnIndex = 0; columnIndex < data.columnCount; columnIndex++)
                      Container(
                        width: metrics.scaledSpacing(150),
                        padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(10), vertical: metrics.scaledSpacing(8)),
                        decoration: BoxDecoration(border: Border(right: BorderSide(color: borderColor))),
                        child: Text(
                          columnIndex < data.rows[rowIndex].length ? data.rows[rowIndex][columnIndex] : "",
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            color: rowIndex == 0 ? textColor : subTextColor,
                            fontSize: metrics.smallLabelFontSize,
                            fontWeight: rowIndex == 0 ? FontWeight.w800 : FontWeight.w500,
                            height: 1.25,
                          ),
                        ),
                      ),
                  ],
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _DelimitedPreviewData {
  final List<List<String>> rows;
  final int rowCount;
  final int columnCount;

  const _DelimitedPreviewData({required this.rows, required this.rowCount, required this.columnCount});

  bool get hasMoreRows => rowCount > rows.length;
}

_DelimitedPreviewData _loadDelimitedPreview(File file, String delimiter) {
  final text = _readDelimitedText(file);
  final rows = _parseDelimitedRows(text, delimiter);
  final previewRows = rows.take(40).toList();
  final columnCount = rows.fold<int>(0, (maxColumns, row) => row.length > maxColumns ? row.length : maxColumns);
  return _DelimitedPreviewData(rows: previewRows, rowCount: rows.length, columnCount: columnCount);
}

List<List<String>> _parseDelimitedRows(String text, String delimiter) {
  final rows = <List<String>>[];
  final row = <String>[];
  final field = StringBuffer();
  var inQuotes = false;

  void finishField() {
    row.add(field.toString());
    field.clear();
  }

  void finishRow() {
    finishField();
    rows.add(List<String>.from(row));
    row.clear();
  }

  for (var i = 0; i < text.length; i++) {
    final char = text[i];
    if (char == '"') {
      if (inQuotes && i + 1 < text.length && text[i + 1] == '"') {
        field.write('"');
        i++;
      } else {
        inQuotes = !inQuotes;
      }
      continue;
    }
    if (!inQuotes && char == delimiter) {
      finishField();
      continue;
    }
    if (!inQuotes && (char == "\n" || char == "\r")) {
      if (char == "\r" && i + 1 < text.length && text[i + 1] == "\n") {
        i++;
      }
      finishRow();
      continue;
    }
    field.write(char);
  }

  if (field.isNotEmpty || row.isNotEmpty) {
    finishRow();
  }
  return rows;
}

String _readDelimitedText(File file) {
  final handle = file.openSync();
  late final List<int> bytes;
  try {
    bytes = handle.readSync(512 * 1024);
  } finally {
    handle.closeSync();
  }
  if (bytes.length >= 3 && bytes[0] == 0xEF && bytes[1] == 0xBB && bytes[2] == 0xBF) {
    return utf8.decode(bytes.sublist(3), allowMalformed: true);
  }
  return utf8.decode(bytes, allowMalformed: true);
}
