import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';

class ExecutableFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "exe";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_executable_not_found", {"path": context.filePath})));
    }

    final typeLabel = context.tr("ui_file_preview_type_windows_executable");

    return WoxFilePreviewResult(
      content: WoxFileInfoPreview(
        icon: Icons.apps_rounded,
        fileIconPath: file.path,
        accent: const Color(0xFF38BDF8),
        title: path.basenameWithoutExtension(file.path),
        subtitle: typeLabel,
        properties: buildWoxFilePreviewCommonProperties(file, typeLabel: typeLabel, tr: context.tr),
      ),
    );
  }
}
