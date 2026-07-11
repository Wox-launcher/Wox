import 'dart:io';

import 'package:flutter/material.dart';
import 'package:wox/components/file_preview/file_preview_policy.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_loading_indicator.dart';

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

    return WoxFilePreviewPolicy.buildDeferredPreview(
      context: context,
      file: file,
      manualLoadThresholdBytes: WoxFilePreviewPolicy.textThresholdBytes,
      icon: Icons.article_rounded,
      accent: const Color(0xFF64748B),
      typeLabel: WoxFilePreviewPolicy.extensionTypeLabel(context),
      loadedPreviewHandlesScrolling: false,
      previewBuilder: (_) => _MarkdownFilePreview(file: file, buildMarkdown: context.buildMarkdown, tr: context.tr),
    );
  }
}

class _MarkdownFilePreview extends StatefulWidget {
  final File file;
  final WoxFilePreviewMarkdownBuilder buildMarkdown;
  final WoxFilePreviewTranslationFormatter tr;

  const _MarkdownFilePreview({required this.file, required this.buildMarkdown, required this.tr});

  @override
  State<_MarkdownFilePreview> createState() => _MarkdownFilePreviewState();
}

class _MarkdownFilePreviewState extends State<_MarkdownFilePreview> {
  late final Future<String> _markdownFuture;

  @override
  void initState() {
    super.initState();
    _markdownFuture = widget.file.readAsString();
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder(
      future: _markdownFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }
        if (snapshot.hasError) {
          return Text(widget.tr("ui_file_preview_error", {"error": snapshot.error.toString()}));
        }
        if (snapshot.hasData) {
          return widget.buildMarkdown(snapshot.data!);
        }
        return const SizedBox();
      },
    );
  }
}
