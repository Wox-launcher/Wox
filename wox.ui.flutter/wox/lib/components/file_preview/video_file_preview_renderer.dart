import 'dart:convert';
import 'dart:io';

import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_webview_preview.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';

class VideoFilePreviewRenderer implements WoxFilePreviewRenderer {
  static const videoExtensions = {"mp4", "m4v", "mov", "webm"};

  @override
  bool supports(String fileExtension) {
    return videoExtensions.contains(fileExtension);
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_video_not_found", {"path": context.filePath})));
    }

    // Reuse Wox's existing WebView platform layer instead of introducing a
    // native media dependency that downloads binary archives during CMake.
    final previewData = WoxPreviewWebviewData(url: file.uri.toString(), cacheDisabled: true);
    return WoxFilePreviewResult(content: WoxWebViewPreview(previewData: jsonEncode(previewData.toJson()), showToolbar: false), contentHandlesScrolling: true);
  }
}
