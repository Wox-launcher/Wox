import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

typedef WoxFolderPreviewOpenPath = void Function(String path);

class FolderFilePreviewRenderer implements WoxFilePreviewRenderer {
  final WoxFolderPreviewOpenPath? openPath;

  const FolderFilePreviewRenderer({this.openPath});

  @override
  bool supports(String fileExtension) {
    // Folder previews are selected by path before extension-based renderers run.
    return false;
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final directory = Directory(context.filePath);
    if (!directory.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_folder_not_found", {"path": context.filePath})));
    }

    try {
      final data = _loadFolderPreview(directory);
      return WoxFilePreviewResult(
        content: _FolderFilePreview(data: data, tr: context.tr, openPath: openPath ?? _openPath),
        previewTags: _buildFolderPreviewTags(directory, data, context.tr),
      );
    } catch (e) {
      return WoxFilePreviewResult(
        content: context.buildText(context.tr("ui_file_preview_error", {"error": e.toString()})),
        previewTags: _buildFolderPreviewTags(directory, null, context.tr),
      );
    }
  }

  void _openPath(String path) {
    WoxApi.instance.open(const UuidV4().generate(), path);
  }
}

class _FolderFilePreview extends StatelessWidget {
  final _FolderPreviewData data;
  final WoxFilePreviewTranslationFormatter tr;
  final WoxFolderPreviewOpenPath openPath;

  const _FolderFilePreview({required this.data, required this.tr, required this.openPath});

  @override
  Widget build(BuildContext context) {
    if (data.entries.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(12),
        child: Text(tr("ui_file_preview_folder_empty"), style: TextStyle(color: getThemeSubTextColor(), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize)),
      );
    }

    return Column(
      children: [
        for (final entry in data.entries) _FolderEntryRow(entry: entry, openPath: openPath),
        if (data.hasMoreEntries)
          Padding(
            padding: const EdgeInsets.all(12),
            child: Text(
              tr("ui_file_preview_folder_more_entries_not_shown", {"count": (data.entryCount - data.entries.length).toString()}),
              style: TextStyle(color: getThemeSubTextColor(), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize),
            ),
          ),
      ],
    );
  }
}

class _FolderEntryRow extends StatelessWidget {
  final _FolderPreviewEntry entry;
  final WoxFolderPreviewOpenPath openPath;

  const _FolderEntryRow({required this.entry, required this.openPath});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () => openPath(entry.path),
        child: Container(
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
                    Padding(
                      padding: EdgeInsets.only(top: metrics.scaledSpacing(2)),
                      child: Text(
                        formatWoxFilePreviewDate(entry.modified),
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
        ),
      ),
    );
  }
}

class _FolderPreviewData {
  final int entryCount;
  final int fileCount;
  final int directoryCount;
  final List<_FolderPreviewEntry> entries;

  const _FolderPreviewData({required this.entryCount, required this.fileCount, required this.directoryCount, required this.entries});

  bool get hasMoreEntries => entryCount > entries.length;
}

class _FolderPreviewEntry {
  final String path;
  final String name;
  final int size;
  final bool isDirectory;
  final DateTime modified;

  const _FolderPreviewEntry({required this.path, required this.name, required this.size, required this.isDirectory, required this.modified});
}

_FolderPreviewData _loadFolderPreview(Directory directory) {
  final entries = <_FolderPreviewEntry>[];
  var fileCount = 0;
  var directoryCount = 0;

  for (final entity in directory.listSync(followLinks: false)) {
    final entryName = path.basename(entity.path);
    if (entryName.startsWith(".")) {
      continue;
    }

    final stat = entity.statSync();
    final isDirectory = stat.type == FileSystemEntityType.directory;
    if (isDirectory) {
      directoryCount++;
    } else {
      fileCount++;
    }

    entries.add(_FolderPreviewEntry(path: entity.path, name: entryName, size: stat.size, isDirectory: isDirectory, modified: stat.modified));
  }

  entries.sort((a, b) {
    if (a.isDirectory != b.isDirectory) {
      return a.isDirectory ? -1 : 1;
    }
    return a.name.toLowerCase().compareTo(b.name.toLowerCase());
  });

  return _FolderPreviewData(entryCount: entries.length, fileCount: fileCount, directoryCount: directoryCount, entries: entries.take(120).toList());
}

List<WoxPreviewTag> _buildFolderPreviewTags(Directory directory, _FolderPreviewData? data, WoxFilePreviewTranslationFormatter tr) {
  final stat = directory.statSync();
  return [
    WoxPreviewTag(label: tr("ui_file_preview_type_folder"), tooltip: tr("ui_file_preview_property_type")),
    if (data != null) WoxPreviewTag(label: tr("ui_file_preview_folder_files_count", {"count": data.fileCount.toString()}), tooltip: tr("ui_file_preview_folder_files")),
    if (data != null) WoxPreviewTag(label: tr("ui_file_preview_folder_folders_count", {"count": data.directoryCount.toString()}), tooltip: tr("ui_file_preview_folder_folders")),
    WoxPreviewTag(label: formatWoxFilePreviewDate(stat.modified), tooltip: tr("ui_file_preview_property_modified")),
    WoxPreviewTag(label: path.dirname(directory.path), tooltip: tr("ui_file_preview_property_location")),
  ];
}
