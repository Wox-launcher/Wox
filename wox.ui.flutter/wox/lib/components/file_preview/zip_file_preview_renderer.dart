import 'dart:io';
import 'dart:typed_data';

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
    return {"zip", "tar", "tgz", "gz"}.contains(fileExtension);
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_archive_not_found", {"path": context.filePath})));
    }

    return WoxFilePreviewResult(content: _ZipFilePreview(file: file, tr: context.tr, kind: _ArchivePreviewKind.fromPath(file.path)));
  }
}

class _ZipFilePreview extends StatefulWidget {
  final File file;
  final WoxFilePreviewTranslationFormatter tr;
  final _ArchivePreviewKind kind;

  const _ZipFilePreview({required this.file, required this.tr, required this.kind});

  @override
  State<_ZipFilePreview> createState() => _ZipFilePreviewState();
}

class _ZipFilePreviewState extends State<_ZipFilePreview> {
  late final Future<_ZipPreviewData> _previewFuture;

  @override
  void initState() {
    super.initState();
    _previewFuture = _loadArchivePreview(widget.file, widget.kind);
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
            subtitle: widget.tr("ui_file_preview_archive_read_failed"),
            properties: buildWoxFilePreviewCommonProperties(widget.file, typeLabel: widget.tr(widget.kind.typeKey), tr: widget.tr),
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
            ...buildWoxFilePreviewCommonProperties(widget.file, typeLabel: widget.tr(widget.kind.typeKey), tr: widget.tr),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_entries"), value: data.entryCount.toString()),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_files"), value: data.fileCount.toString()),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_folders"), value: data.directoryCount.toString()),
            WoxFilePreviewProperty(label: widget.tr("ui_file_preview_zip_uncompressed"), value: formatWoxFilePreviewSize(data.totalUncompressedSize)),
            if (data.inferredFileName.isNotEmpty) WoxFilePreviewProperty(label: widget.tr("ui_file_preview_archive_inferred_file"), value: data.inferredFileName),
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
  final String inferredFileName;

  const _ZipPreviewData({
    required this.entryCount,
    required this.fileCount,
    required this.directoryCount,
    required this.totalUncompressedSize,
    required this.entries,
    this.inferredFileName = "",
  });

  bool get hasMoreEntries => entryCount > entries.length;
}

class _ZipPreviewEntry {
  final String name;
  final int size;
  final bool isDirectory;
  final DateTime? modified;

  const _ZipPreviewEntry({required this.name, required this.size, required this.isDirectory, required this.modified});
}

enum _ArchivePreviewKind {
  zip("ui_file_preview_type_zip_archive"),
  tar("ui_file_preview_type_tar_archive"),
  tgz("ui_file_preview_type_tgz_archive"),
  gzip("ui_file_preview_type_gzip_archive");

  final String typeKey;

  const _ArchivePreviewKind(this.typeKey);

  static _ArchivePreviewKind fromPath(String filePath) {
    final lower = filePath.toLowerCase();
    if (lower.endsWith(".tar.gz") || lower.endsWith(".tgz")) {
      return _ArchivePreviewKind.tgz;
    }
    if (lower.endsWith(".tar")) {
      return _ArchivePreviewKind.tar;
    }
    if (lower.endsWith(".gz")) {
      return _ArchivePreviewKind.gzip;
    }
    return _ArchivePreviewKind.zip;
  }
}

// Reads archive directory metadata only where the format supports it. Plain
// gzip has no directory, so it is shown as a single inferred decompressed file.
Future<_ZipPreviewData> _loadArchivePreview(File file, _ArchivePreviewKind kind) async {
  if (kind == _ArchivePreviewKind.gzip) {
    return _loadGzipPreview(file);
  }

  final input = InputFileStream(file.path);
  try {
    if (kind == _ArchivePreviewKind.zip) {
      return _archiveToPreviewData(ZipDecoder().decodeStream(input, verify: false));
    }
    if (kind == _ArchivePreviewKind.tar) {
      return _archiveToPreviewData(TarDecoder().decodeStream(input, storeData: false));
    }
  } finally {
    input.closeSync();
  }

  final bytes = file.readAsBytesSync();
  final decompressed = GZipDecoder().decodeBytes(bytes, verify: false);
  return _archiveToPreviewData(TarDecoder().decodeBytes(decompressed, storeData: false));
}

_ZipPreviewData _archiveToPreviewData(Archive archive) {
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
}

_ZipPreviewData _loadGzipPreview(File file) {
  final bytes = file.readAsBytesSync();
  final inferredFileName = _stripGzipExtension(path.basename(file.path));
  final uncompressedSize = bytes.length >= 4 ? ByteData.sublistView(bytes).getUint32(bytes.length - 4, Endian.little) : 0;
  return _ZipPreviewData(
    entryCount: 1,
    fileCount: 1,
    directoryCount: 0,
    totalUncompressedSize: uncompressedSize,
    inferredFileName: inferredFileName,
    entries: [_ZipPreviewEntry(name: inferredFileName, size: uncompressedSize, isDirectory: false, modified: null)],
  );
}

String _stripGzipExtension(String name) {
  final lower = name.toLowerCase();
  if (lower.endsWith(".gz")) {
    return name.substring(0, name.length - 3);
  }
  return name;
}
