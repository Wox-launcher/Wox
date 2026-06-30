import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/deferred_file_preview.dart';
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';

// Centralizes heavy file preview thresholds so renderers do not each invent
// their own large-file behavior.
class WoxFilePreviewPolicy {
  static const int textThresholdBytes = 1 * 1024 * 1024;
  static const int tableThresholdBytes = 2 * 1024 * 1024;
  static const int archiveThresholdBytes = 10 * 1024 * 1024;
  static const int pdfThresholdBytes = 10 * 1024 * 1024;
  static const int officeThresholdBytes = 5 * 1024 * 1024;
  static const Duration autoLoadDelay = Duration(milliseconds: 180);

  static String cacheKey(File file) {
    final stat = file.statSync();
    return "file-preview:${file.path}:${stat.size}:${stat.modified.millisecondsSinceEpoch}";
  }

  static String extensionTypeLabel(WoxFilePreviewContext context) {
    final extension = context.fileExtension.trim();
    if (extension.isEmpty) {
      return context.tr("ui_file_preview_type_file");
    }
    return extension.toUpperCase();
  }

  static WoxFilePreviewResult buildDeferredPreview({
    required WoxFilePreviewContext context,
    required File file,
    required int manualLoadThresholdBytes,
    required IconData icon,
    required Color accent,
    required String typeLabel,
    required WidgetBuilder previewBuilder,
    String? title,
    String? subtitle,
    String? fileIconPath,
    String? previewKey,
    bool loadedPreviewHandlesScrolling = true,
    List<WoxFilePreviewProperty> extraProperties = const [],
  }) {
    final fileSize = file.lengthSync();
    return WoxFilePreviewResult(
      content: WoxDeferredFilePreview(
        previewKey: previewKey ?? cacheKey(file),
        icon: icon,
        fileIconPath: fileIconPath ?? file.path,
        accent: accent,
        title: title ?? path.basename(file.path),
        subtitle: subtitle ?? typeLabel,
        properties: [...buildWoxFilePreviewCommonProperties(file, typeLabel: typeLabel, tr: context.tr), ...extraProperties],
        messageTitle: context.tr("ui_file_preview_large_file_title"),
        message: context.tr("ui_file_preview_large_file_message", {"size": formatWoxFilePreviewSize(fileSize)}),
        actionLabel: context.tr("ui_file_preview_load_full_preview"),
        scrollController: context.scrollController,
        autoLoadDelay: fileSize > manualLoadThresholdBytes ? null : autoLoadDelay,
        loadedPreviewHandlesScrolling: loadedPreviewHandlesScrolling,
        previewBuilder: previewBuilder,
      ),
      contentHandlesScrolling: true,
    );
  }
}
