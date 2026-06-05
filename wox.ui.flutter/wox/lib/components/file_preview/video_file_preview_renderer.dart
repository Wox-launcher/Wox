import 'dart:convert';
import 'dart:io';

import 'package:wox/components/file_preview/file_preview_media_source.dart';
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

    final previewData = WoxPreviewWebviewData(url: "", html: _buildVideoPreviewHtml(file), cacheDisabled: true);
    return WoxFilePreviewResult(content: WoxWebViewPreview(previewData: jsonEncode(previewData.toJson()), showToolbar: false), contentHandlesScrolling: true);
  }

  // Serve the video through core's loopback endpoint so the browser can use
  // range requests while Wox keeps full control over the preview document.
  String _buildVideoPreviewHtml(File file) {
    final source = buildFilePreviewMediaSource(file);
    return '''
<!doctype html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
html,
body {
  margin: 0;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: transparent;
  color-scheme: light dark;
}
body {
  display: flex;
  align-items: center;
  justify-content: center;
}
video {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
  background: transparent;
}
</style>
</head>
<body>
<video controls preload="metadata" playsinline src="$source"></video>
<script>
(() => {
  const video = document.querySelector('video');
  if (!video) {
    return;
  }

  const showPreviewFrame = () => {
    if (video.currentTime !== 0 || !Number.isFinite(video.duration) || video.duration <= 0) {
      return;
    }

    try {
      video.currentTime = Math.min(0.1, Math.max(video.duration - 0.01, 0));
      video.pause();
    } catch (_) {}
  };

  video.addEventListener('loadedmetadata', showPreviewFrame, { once: true });
  video.addEventListener('seeked', () => video.pause(), { once: true });
})();
</script>
</body>
</html>
''';
  }
}
