import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:wox/components/file_preview/file_preview_media_source.dart';
import 'package:wox/components/file_preview/file_preview_policy.dart';
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
    if (context.filePath.startsWith("http")) {
      return WoxFilePreviewResult(content: _buildPdfWebView(context, context.filePath, cacheDisabled: true), contentHandlesScrolling: true);
    }

    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_pdf_not_found", {"path": context.filePath})));
    }

    final typeLabel = context.tr("ui_file_preview_type_pdf_document");
    final cacheKey = WoxFilePreviewPolicy.cacheKey(file);

    return WoxFilePreviewPolicy.buildDeferredPreview(
      context: context,
      file: file,
      manualLoadThresholdBytes: WoxFilePreviewPolicy.pdfThresholdBytes,
      icon: Icons.picture_as_pdf_rounded,
      accent: const Color(0xFFDC2626),
      typeLabel: typeLabel,
      previewKey: cacheKey,
      previewBuilder: (_) => _buildPdfWebView(context, buildFilePreviewMediaSource(file), cacheKey: cacheKey),
    );
  }

  Widget _buildPdfWebView(WoxFilePreviewContext context, String source, {String cacheKey = "", bool cacheDisabled = false}) {
    final previewData = WoxPreviewWebviewData(url: source, cacheKey: cacheKey, cacheDisabled: cacheDisabled);
    return WoxWebViewPreview(previewData: jsonEncode(previewData.toJson()), launcherController: context.launcherController, showToolbar: false);
  }
}
