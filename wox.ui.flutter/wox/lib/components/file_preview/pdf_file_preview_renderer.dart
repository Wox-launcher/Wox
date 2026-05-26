import 'dart:io';

import 'package:flutter/material.dart';
import 'package:syncfusion_flutter_pdfviewer/pdfviewer.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';

class PdfFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "pdf";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final viewer = context.filePath.startsWith("http") ? SfPdfViewer.network(context.filePath) : SfPdfViewer.file(File(context.filePath));
    // The PDF viewer can capture focus from the query box, so keep it excluded
    // while still letting the viewer own its internal scrolling.
    return WoxFilePreviewResult(content: ExcludeFocus(child: viewer), contentHandlesScrolling: true);
  }
}
