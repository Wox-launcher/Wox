import 'package:flutter/material.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_image.dart';

typedef WoxFilePreviewTextBuilder = Widget Function(String text);
typedef WoxFilePreviewMarkdownBuilder = Widget Function(String markdown);
typedef WoxFilePreviewImageSurfaceBuilder = Widget Function(Widget image, {WoxImage? overlayImage});
typedef WoxFilePreviewTranslator = String Function(String key);
typedef WoxFilePreviewTranslationFormatter = String Function(String key, [Map<String, String> replacements]);

// WoxFilePreviewResult lets file renderers return both their widget and the
// scaffold scrolling contract that used to be encoded in wox_preview_view.dart.
class WoxFilePreviewResult {
  final Widget content;
  final bool contentHandlesScrolling;

  const WoxFilePreviewResult({required this.content, this.contentHandlesScrolling = false});
}

// WoxFilePreviewContext carries only the shared preview helpers that file
// renderers need, keeping renderer files independent from WoxPreviewView state.
class WoxFilePreviewContext {
  final String filePath;
  final String fileExtension;
  final ScrollController scrollController;
  final WoxFilePreviewTextBuilder buildText;
  final WoxFilePreviewMarkdownBuilder buildMarkdown;
  final WoxFilePreviewImageSurfaceBuilder buildImageSurface;
  final WoxFilePreviewTranslator translate;
  final WoxLauncherController launcherController;

  const WoxFilePreviewContext({
    required this.filePath,
    required this.fileExtension,
    required this.scrollController,
    required this.buildText,
    required this.buildMarkdown,
    required this.buildImageSurface,
    required this.translate,
    required this.launcherController,
  });

  // Keeps file-preview placeholders local to renderers while using the shared
  // launcher translation table.
  String tr(String key, [Map<String, String> replacements = const {}]) {
    var value = translate(key);
    for (final entry in replacements.entries) {
      value = value.replaceAll("{${entry.key}}", entry.value);
    }
    return value;
  }
}

abstract class WoxFilePreviewRenderer {
  bool supports(String fileExtension);

  WoxFilePreviewResult render(WoxFilePreviewContext context);
}
