import 'dart:io';

import 'package:archive/archive.dart';
import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class ZipFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "zip";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_zip_not_found", {"path": context.filePath})));
    }

    return WoxFilePreviewResult(content: _ZipFilePreview(file: file, tr: context.tr));
  }
}

class _ZipFilePreview extends StatefulWidget {
  final File file;
  final WoxFilePreviewTranslationFormatter tr;

  const _ZipFilePreview({required this.file, required this.tr});

  @override
  State<_ZipFilePreview> createState() => _ZipFilePreviewState();
}

class _ZipFilePreviewState extends State<_ZipFilePreview> {
  late final Future<_ZipPreviewData> _previewFuture;

  @override
  void initState() {
    super.initState();
    _previewFuture = _loadZipPreview(widget.file);
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<_ZipPreviewData>(
      future: _previewFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }
        if (snapshot.hasError) {
          return WoxFileInfoPreview(
            icon: Icons.folder_zip_rounded,
            accent: const Color(0xFFF59E0B),
            title: path.basename(widget.file.path),
            subtitle: widget.tr("ui_file_preview_zip_read_failed"),
            properties: buildWoxFilePreviewCommonProperties(widget.file, typeLabel: widget.tr("ui_file_preview_type_zip_archive"), tr: widget.tr),
            sections: [
              WoxFilePreviewSection(
                title: widget.tr("ui_file_preview_error_title"),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Text(snapshot.error.toString(), style: TextStyle(color: getThemeTextColor(), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize)),
                ),
              ),
            ],
          );
        }

        final data = snapshot.data!;
        return WoxFileInfoPreview(
          icon: Icons.folder_zip_rounded,
          accent: const Color(0xFFF59E0B),
          title: path.basename(widget.file.path),
          subtitle: widget.tr("ui_file_preview_zip_summary", {"files": data.fileCount.toString(), "folders": data.directoryCount.toString()}),
          properties: [
            ...buildWoxFilePreviewCommonProperties(widget.file, typeLabel: widget.tr("ui_file_preview_type_zip_archive"), tr: widget.tr),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_entries"), value: data.entryCount.toString()),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_files"), value: data.fileCount.toString()),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_folders"), value: data.directoryCount.toString()),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_uncompressed"), value: formatWoxFilePreviewSize(data.totalUncompressedSize)),
          ],
          sections: [
            WoxFilePreviewSection(
              title: data.hasMoreEntries ? widget.tr("ui_file_preview_zip_contents_first", {"count": data.entries.length.toString()}) : widget.tr("ui_file_preview_zip_contents"),
              child: Column(
                children: [
                  for (final entry in data.entries) _ZipEntryRow(entry: entry),
                  if (data.hasMoreEntries)
                    Padding(
                      padding: const EdgeInsets.all(12),
                      child: Text(
                        widget.tr("ui_file_preview_zip_more_entries_not_shown", {"count": (data.entryCount - data.entries.length).toString()}),
                        style: TextStyle(color: getThemeSubTextColor(), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize),
                      ),
                    ),
                ],
              ),
            ),
          ],
        );
      },
    );
  }
}

class _ZipEntryRow extends StatelessWidget {
  final _ZipPreviewEntry entry;

  const _ZipEntryRow({required this.entry});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();

    return Container(
      padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(12), vertical: metrics.scaledSpacing(9)),
      decoration: BoxDecoration(border: Border(bottom: BorderSide(color: getThemeDividerColor().withValues(alpha: 0.32)))),
      child: Row(
        children: [
          Icon(
            entry.isDirectory ? Icons.folder_outlined : Icons.insert_drive_file_outlined,
            color: entry.isDirectory ? const Color(0xFFF59E0B) : subTextColor,
            size: metrics.scaledSpacing(18),
          ),
          SizedBox(width: metrics.scaledSpacing(10)),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  entry.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: textColor, fontSize: metrics.resultSubtitleFontSize, fontWeight: FontWeight.w700),
                ),
                if (entry.modified != null)
                  Padding(
                    padding: EdgeInsets.only(top: metrics.scaledSpacing(2)),
                    child: Text(
                      formatWoxFilePreviewDate(entry.modified!),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: subTextColor, fontSize: metrics.smallLabelFontSize),
                    ),
                  ),
              ],
            ),
          ),
          if (!entry.isDirectory)
            Padding(
              padding: EdgeInsets.only(left: metrics.scaledSpacing(10)),
              child: Text(
                formatWoxFilePreviewSize(entry.size),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: subTextColor, fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700),
              ),
            ),
        ],
      ),
    );
  }
}

class _ZipPreviewData {
  final int entryCount;
  final int fileCount;
  final int directoryCount;
  final int totalUncompressedSize;
  final List<_ZipPreviewEntry> entries;

  const _ZipPreviewData({required this.entryCount, required this.fileCount, required this.directoryCount, required this.totalUncompressedSize, required this.entries});

  bool get hasMoreEntries => entryCount > entries.length;
}

class _ZipPreviewEntry {
  final String name;
  final int size;
  final bool isDirectory;
  final DateTime? modified;

  const _ZipPreviewEntry({required this.name, required this.size, required this.isDirectory, required this.modified});
}

// Reads ZIP directory metadata only. Entry content is not decompressed, keeping
// preview cheap even for large archives.
Future<_ZipPreviewData> _loadZipPreview(File file) async {
  final input = InputFileStream(file.path);
  try {
    final archive = ZipDecoder().decodeStream(input, verify: false);
    var fileCount = 0;
    var directoryCount = 0;
    var totalUncompressedSize = 0;
    final entries = <_ZipPreviewEntry>[];

    for (final entry in archive.files) {
      if (entry.isDirectory) {
        directoryCount++;
      } else {
        fileCount++;
        totalUncompressedSize += entry.size;
      }
      if (entries.length < 120) {
        entries.add(_ZipPreviewEntry(name: entry.name, size: entry.size, isDirectory: entry.isDirectory, modified: entry.lastModDateTime));
      }
    }

    return _ZipPreviewData(entryCount: archive.files.length, fileCount: fileCount, directoryCount: directoryCount, totalUncompressedSize: totalUncompressedSize, entries: entries);
  } finally {
    input.closeSync();
  }
}
