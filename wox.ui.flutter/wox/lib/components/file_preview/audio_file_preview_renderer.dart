import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_webview_preview.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';

class AudioFilePreviewRenderer implements WoxFilePreviewRenderer {
  static const audioExtensions = {"mp3", "wav", "m4a", "aac", "flac", "ogg", "opus"};

  @override
  bool supports(String fileExtension) {
    return audioExtensions.contains(fileExtension);
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_audio_not_found", {"path": context.filePath})));
    }

    final typeLabel = context.tr("ui_file_preview_type_audio");
    final previewData = WoxPreviewWebviewData(url: "", html: _buildPausedAudioPreviewHtml(file), cacheDisabled: true);

    return WoxFilePreviewResult(
      content: SizedBox(height: 86, child: WoxWebViewPreview(previewData: jsonEncode(previewData.toJson()), showToolbar: false)),
      previewTags: [WoxPreviewTag(label: typeLabel, tooltip: context.tr("ui_file_preview_property_type"))],
    );
  }

  // Data URLs keep audio preview behavior consistent across native WebView
  // implementations and avoid autoplay from the WebView default media page.
  String _buildPausedAudioPreviewHtml(File file) {
    final mimeType = _resolveAudioMimeType(path.extension(file.path).replaceFirst(".", "").toLowerCase());
    final source = "data:$mimeType;base64,${base64Encode(file.readAsBytesSync())}";
    return '''
<!doctype html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
html, body {
  margin: 0;
  width: 100%;
  height: 100%;
  background: transparent;
  color-scheme: light dark;
}
body {
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
audio {
  width: calc(100% - 28px);
  max-width: 760px;
}
</style>
</head>
<body>
<audio controls preload="metadata" src="$source"></audio>
</body>
</html>
''';
  }

  String _resolveAudioMimeType(String extension) {
    return switch (extension) {
      "mp3" => "audio/mpeg",
      "wav" => "audio/wav",
      "m4a" => "audio/mp4",
      "aac" => "audio/aac",
      "flac" => "audio/flac",
      "ogg" => "audio/ogg",
      "opus" => "audio/ogg",
      _ => "audio/mpeg",
    };
  }
}
