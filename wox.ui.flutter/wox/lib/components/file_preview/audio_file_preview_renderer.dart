import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_webview_preview.dart';
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
    final previewData = WoxPreviewWebviewData(url: file.uri.toString(), cacheDisabled: true);

    return WoxFilePreviewResult(
      content: WoxFileInfoPreview(
        icon: Icons.audio_file_rounded,
        fileIconPath: file.path,
        accent: const Color(0xFF14B8A6),
        title: path.basename(file.path),
        subtitle: typeLabel,
        properties: buildWoxFilePreviewCommonProperties(file, typeLabel: typeLabel, tr: context.tr),
        sections: [
          WoxFilePreviewSection(
            title: context.tr("ui_file_preview_audio_player"),
            child: SizedBox(height: 86, child: WoxWebViewPreview(previewData: jsonEncode(previewData.toJson()), showToolbar: false)),
          ),
        ],
      ),
    );
  }
}
