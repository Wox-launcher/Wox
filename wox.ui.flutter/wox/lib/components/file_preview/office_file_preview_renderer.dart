import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/file_preview/windows_preview_handler_view.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

enum _OfficePreviewKind {
  word({"doc", "docx"}, Icons.description_rounded, Color(0xFF2563EB), "ui_file_preview_type_word_document"),
  excel({"xls", "xlsx"}, Icons.grid_on_rounded, Color(0xFF16A34A), "ui_file_preview_type_excel_workbook"),
  powerpoint({"ppt", "pptx"}, Icons.slideshow_rounded, Color(0xFFEA580C), "ui_file_preview_type_powerpoint_presentation");

  final Set<String> extensions;
  final IconData icon;
  final Color accent;
  final String typeKey;

  const _OfficePreviewKind(this.extensions, this.icon, this.accent, this.typeKey);

  static _OfficePreviewKind? fromExtension(String extension) {
    for (final kind in values) {
      if (kind.extensions.contains(extension)) {
        return kind;
      }
    }
    return null;
  }
}

class OfficeFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return _OfficePreviewKind.fromExtension(fileExtension) != null;
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_office_not_found", {"path": context.filePath})));
    }

    final kind = _OfficePreviewKind.fromExtension(context.fileExtension);
    if (kind == null) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_unsupported_type", {"extension": context.fileExtension})));
    }

    return WoxFilePreviewResult(
      content: WoxWindowsPreviewHandlerView(filePath: file.path, fallbackBuilder: (error) => _buildFallbackPreview(context, file, kind, error)),
      contentHandlesScrolling: true,
    );
  }

  /// Builds the non-native fallback shown when Windows has no registered Office
  /// preview handler, or when the native handler fails to initialize.
  Widget _buildFallbackPreview(WoxFilePreviewContext previewContext, File file, _OfficePreviewKind kind, String? error) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final detail = Platform.isWindows ? previewContext.tr("ui_file_preview_office_preview_handler_unavailable") : previewContext.tr("ui_file_preview_office_preview_windows_only");
    final errorText = error?.trim().isNotEmpty == true ? previewContext.tr("ui_file_preview_office_preview_handler_error", {"error": error!}) : "";

    final fallbackContent = WoxFileInfoPreview(
      icon: kind.icon,
      accent: kind.accent,
      title: path.basename(file.path),
      subtitle: previewContext.tr("ui_file_preview_office_preview_unavailable"),
      properties: buildWoxFilePreviewCommonProperties(file, typeLabel: previewContext.tr(kind.typeKey), tr: previewContext.tr),
      sections: [
        WoxFilePreviewSection(
          title: previewContext.tr("ui_file_preview_office_preview_unavailable_title"),
          child: Padding(
            padding: EdgeInsets.all(metrics.scaledSpacing(12)),
            child: Text(errorText.isEmpty ? detail : "$detail\n$errorText", style: TextStyle(color: getThemeTextColor(), fontSize: metrics.resultSubtitleFontSize, height: 1.4)),
          ),
        ),
      ],
    );

    return LayoutBuilder(
      builder:
          (context, constraints) => Scrollbar(
            thumbVisibility: true,
            controller: previewContext.scrollController,
            child: SingleChildScrollView(
              controller: previewContext.scrollController,
              child: ConstrainedBox(
                constraints: BoxConstraints(minWidth: constraints.maxWidth, maxWidth: constraints.maxWidth, minHeight: constraints.maxHeight),
                child: fallbackContent,
              ),
            ),
          ),
    );
  }
}
