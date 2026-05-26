import 'dart:io';

import 'package:wox/components/file_preview/file_preview_renderer.dart';

class MarkdownFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "md";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_markdown_not_found", {"path": context.filePath})));
    }

    return WoxFilePreviewResult(content: context.buildMarkdown(file.readAsStringSync()));
  }
}
