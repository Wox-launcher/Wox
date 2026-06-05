import 'dart:io';

import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/audio_file_preview_renderer.dart';
import 'package:wox/components/file_preview/calendar_contact_file_preview_renderer.dart';
import 'package:wox/components/file_preview/code_file_preview_renderer.dart';
import 'package:wox/components/file_preview/delimited_file_preview_renderer.dart';
import 'package:wox/components/file_preview/executable_file_preview_renderer.dart';
import 'package:wox/components/file_preview/folder_file_preview_renderer.dart';
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/file_preview/font_file_preview_renderer.dart';
import 'package:wox/components/file_preview/image_file_preview_renderer.dart';
import 'package:wox/components/file_preview/markdown_file_preview_renderer.dart';
import 'package:wox/components/file_preview/office_file_preview_renderer.dart';
import 'package:wox/components/file_preview/pdf_file_preview_renderer.dart';
import 'package:wox/components/file_preview/rdp_file_preview_renderer.dart';
import 'package:wox/components/file_preview/shortcut_file_preview_renderer.dart';
import 'package:wox/components/file_preview/video_file_preview_renderer.dart';
import 'package:wox/components/file_preview/zip_file_preview_renderer.dart';
import 'package:wox/entity/wox_preview.dart';

class WoxFilePreviewRegistry {
  final List<WoxFilePreviewRenderer> renderers;

  const WoxFilePreviewRegistry({required this.renderers});

  // Renderer order is intentional: specific visual formats should win before
  // the broad code-language matcher checks highlight's language table.
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    if (Directory(context.filePath).existsSync()) {
      return FolderFilePreviewRenderer().render(context);
    }

    for (final renderer in renderers) {
      if (renderer.supports(context.fileExtension)) {
        return _withCommonFileTags(context, renderer.render(context));
      }
    }

    return _withCommonFileTags(context, WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_unsupported_type", {"extension": context.fileExtension}))));
  }

  WoxFilePreviewResult _withCommonFileTags(WoxFilePreviewContext context, WoxFilePreviewResult result) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return result;
    }

    final tags = [...result.previewTags];
    final hasTypeTag = tags.any((tag) => tag.tooltip == context.tr("ui_file_preview_property_type"));
    if (!hasTypeTag) {
      final fallbackType = context.fileExtension.isEmpty ? context.tr("ui_file_preview_type_file") : context.fileExtension.toUpperCase();
      tags.add(WoxPreviewTag(label: fallbackType, tooltip: context.tr("ui_file_preview_property_type")));
    }

    final stat = file.statSync();
    tags.addAll([
      WoxPreviewTag(label: formatWoxFilePreviewSize(stat.size), tooltip: context.tr("ui_file_preview_property_size")),
      WoxPreviewTag(label: formatWoxFilePreviewDate(stat.modified), tooltip: context.tr("ui_file_preview_property_modified")),
      WoxPreviewTag(label: path.dirname(context.filePath), tooltip: context.tr("ui_file_preview_property_location")),
    ]);
    return result.copyWith(previewTags: tags);
  }
}

final defaultWoxFilePreviewRegistry = WoxFilePreviewRegistry(
  renderers: [
    PdfFilePreviewRenderer(),
    MarkdownFilePreviewRenderer(),
    ImageFilePreviewRenderer(),
    VideoFilePreviewRenderer(),
    AudioFilePreviewRenderer(),
    OfficeFilePreviewRenderer(),
    ExecutableFilePreviewRenderer(),
    ShortcutFilePreviewRenderer(),
    RdpFilePreviewRenderer(),
    ZipFilePreviewRenderer(),
    FontFilePreviewRenderer(),
    CalendarContactFilePreviewRenderer(),
    DelimitedFilePreviewRenderer(),
    CodeFilePreviewRenderer(),
  ],
);

String resolveWoxFilePreviewExtension(String filePath) {
  return filePath.split(".").last.toLowerCase();
}
