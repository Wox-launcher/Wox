import 'dart:convert';
import 'dart:io';

import 'package:wox/components/file_preview/file_preview_media_source.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_webview_preview.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';

class PdfFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "pdf";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final source = context.filePath.startsWith("http") ? context.filePath : buildFilePreviewMediaSource(File(context.filePath));
    final previewData = WoxPreviewWebviewData(url: source, cacheDisabled: true);
    return WoxFilePreviewResult(content: WoxWebViewPreview(previewData: jsonEncode(previewData.toJson()), showToolbar: false), contentHandlesScrolling: true);
  }
}
