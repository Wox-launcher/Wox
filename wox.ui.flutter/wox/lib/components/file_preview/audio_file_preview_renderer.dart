import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:wox/components/file_preview/file_preview_media_source.dart';
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

  // Use Wox core's loopback media endpoint so large audio files stream through
  // browser range requests instead of becoming a data URL in Flutter memory.
  String _buildPausedAudioPreviewHtml(File file) {
    final source = buildFilePreviewMediaSource(file);
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
}
