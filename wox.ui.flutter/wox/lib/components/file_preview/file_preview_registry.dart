import 'package:wox/components/file_preview/code_file_preview_renderer.dart';
import 'package:wox/components/file_preview/executable_file_preview_renderer.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/file_preview/image_file_preview_renderer.dart';
import 'package:wox/components/file_preview/markdown_file_preview_renderer.dart';
import 'package:wox/components/file_preview/office_file_preview_renderer.dart';
import 'package:wox/components/file_preview/pdf_file_preview_renderer.dart';
import 'package:wox/components/file_preview/shortcut_file_preview_renderer.dart';
import 'package:wox/components/file_preview/video_file_preview_renderer.dart';
import 'package:wox/components/file_preview/zip_file_preview_renderer.dart';

class WoxFilePreviewRegistry {
  final List<WoxFilePreviewRenderer> renderers;

  const WoxFilePreviewRegistry({required this.renderers});

  // Renderer order is intentional: specific visual formats should win before
  // the broad code-language matcher checks highlight's language table.
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    for (final renderer in renderers) {
      if (renderer.supports(context.fileExtension)) {
        return renderer.render(context);
      }
    }

    return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_unsupported_type", {"extension": context.fileExtension})));
  }
}

final defaultWoxFilePreviewRegistry = WoxFilePreviewRegistry(
  renderers: [
    PdfFilePreviewRenderer(),
    MarkdownFilePreviewRenderer(),
    ImageFilePreviewRenderer(),
    VideoFilePreviewRenderer(),
    OfficeFilePreviewRenderer(),
    ExecutableFilePreviewRenderer(),
    ShortcutFilePreviewRenderer(),
    ZipFilePreviewRenderer(),
    CodeFilePreviewRenderer(),
  ],
);

String resolveWoxFilePreviewExtension(String filePath) {
  return filePath.split(".").last.toLowerCase();
}
