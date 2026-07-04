import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:wox/components/file_preview/file_preview_policy.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_loading_indicator.dart';

class TextFilePreviewRenderer implements WoxFilePreviewRenderer {
  static const int textPreviewMaxBytes = 512 * 1024;
  static const int textPreviewMaxLines = 2000;

  @override
  bool supports(String fileExtension) {
    return fileExtension == "txt" || fileExtension == "text";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_text_not_found", {"path": context.filePath})));
    }

    return WoxFilePreviewPolicy.buildDeferredPreview(
      context: context,
      file: file,
      manualLoadThresholdBytes: WoxFilePreviewPolicy.textThresholdBytes,
      icon: Icons.description_rounded,
      accent: const Color(0xFF64748B),
      typeLabel: WoxFilePreviewPolicy.extensionTypeLabel(context),
      loadedPreviewHandlesScrolling: false,
      previewBuilder: (_) => _TextFilePreview(file: file, buildText: context.buildText, tr: context.tr),
    );
  }
}

class _TextFilePreview extends StatefulWidget {
  final File file;
  final WoxFilePreviewTextBuilder buildText;
  final WoxFilePreviewTranslationFormatter tr;

  const _TextFilePreview({required this.file, required this.buildText, required this.tr});

  @override
  State<_TextFilePreview> createState() => _TextFilePreviewState();
}

class _TextFilePreviewState extends State<_TextFilePreview> {
  late final Future<_TextPreviewData> _textFuture;

  @override
  void initState() {
    super.initState();
    _textFuture = _loadTextPreview(widget.file);
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder(
      future: _textFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }
        if (snapshot.hasError) {
          return Text(widget.tr("ui_file_preview_error", {"error": snapshot.error.toString()}));
        }
        if (snapshot.hasData) {
          final data = snapshot.data!;
          final text = data.isTruncated ? "${widget.tr("ui_file_preview_text_preview_limited", {"lines": data.lineCount.toString()})}\n\n${data.text}" : data.text;
          return widget.buildText(text);
        }
        return const SizedBox();
      },
    );
  }
}

class _TextPreviewData {
  final String text;
  final int lineCount;
  final bool isTruncated;

  const _TextPreviewData({required this.text, required this.lineCount, required this.isTruncated});
}

Future<_TextPreviewData> _loadTextPreview(File file) async {
  if (file.lengthSync() > TextFilePreviewRenderer.textPreviewMaxBytes) {
    return await _loadLimitedTextPreview(file, isTruncated: true);
  }

  final text = await file.readAsString();
  final limited = _limitTextLines(text, TextFilePreviewRenderer.textPreviewMaxLines);
  return _TextPreviewData(text: limited.text, lineCount: limited.lineCount, isTruncated: limited.isTruncated);
}

Future<_TextPreviewData> _loadLimitedTextPreview(File file, {required bool isTruncated}) async {
  final handle = await file.open();
  late final List<int> bytes;
  try {
    bytes = await handle.read(TextFilePreviewRenderer.textPreviewMaxBytes + 1);
  } finally {
    await handle.close();
  }

  final truncatedByBytes = bytes.length > TextFilePreviewRenderer.textPreviewMaxBytes;
  final previewBytes = truncatedByBytes ? bytes.sublist(0, TextFilePreviewRenderer.textPreviewMaxBytes) : bytes;
  final text = _decodeTextPreview(previewBytes);
  final limited = _limitTextLines(text, TextFilePreviewRenderer.textPreviewMaxLines);
  return _TextPreviewData(text: limited.text, lineCount: limited.lineCount, isTruncated: isTruncated || truncatedByBytes || limited.isTruncated);
}

String _decodeTextPreview(List<int> bytes) {
  if (bytes.length >= 3 && bytes[0] == 0xEF && bytes[1] == 0xBB && bytes[2] == 0xBF) {
    return utf8.decode(bytes.sublist(3), allowMalformed: true);
  }
  return utf8.decode(bytes, allowMalformed: true);
}

_LimitedText _limitTextLines(String text, int maxLines) {
  if (text.isEmpty || maxLines <= 0) {
    return const _LimitedText(text: "", lineCount: 0, isTruncated: false);
  }

  var lineCount = 1;
  for (var i = 0; i < text.length; i++) {
    if (text.codeUnitAt(i) != 0x0A) {
      continue;
    }
    if (lineCount >= maxLines) {
      return _LimitedText(text: text.substring(0, i), lineCount: lineCount, isTruncated: true);
    }
    lineCount++;
  }

  return _LimitedText(text: text, lineCount: lineCount, isTruncated: false);
}

class _LimitedText {
  final String text;
  final int lineCount;
  final bool isTruncated;

  const _LimitedText({required this.text, required this.lineCount, required this.isTruncated});
}
