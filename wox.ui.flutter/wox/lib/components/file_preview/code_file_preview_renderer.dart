import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_code_editor/flutter_code_editor.dart';
import 'package:flutter_highlight/themes/monokai.dart';
import 'package:highlight/highlight.dart';
import 'package:highlight/languages/all.dart';
import 'package:highlight/languages/bash.dart';
import 'package:highlight/languages/javascript.dart';
import 'package:highlight/languages/python.dart';
import 'package:highlight/languages/typescript.dart';
import 'package:highlight/languages/yaml.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class CodeFilePreviewRenderer implements WoxFilePreviewRenderer {
  static final allCodeLanguages = {...allLanguages, "txt": Mode(), "conf": Mode(), "toml": Mode(), "js": javascript, "ts": typescript, "yml": yaml, "sh": bash, "py": python};

  @override
  bool supports(String fileExtension) {
    return allCodeLanguages.containsKey(fileExtension);
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_code_not_found", {"path": context.filePath})));
    }

    final fileSize = file.lengthSync();
    if (fileSize > 1 * 1024 * 1024) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_too_large", {"size": (fileSize / 1024 / 1024).toInt().toString()})));
    }

    return WoxFilePreviewResult(
      content: _CodeFilePreview(file: file, fileExtension: context.fileExtension, scrollController: context.scrollController, tr: context.tr),
      contentHandlesScrolling: true,
    );
  }
}

class _CodeFilePreview extends StatelessWidget {
  final File file;
  final String fileExtension;
  final ScrollController scrollController;
  final WoxFilePreviewTranslationFormatter tr;

  const _CodeFilePreview({required this.file, required this.fileExtension, required this.scrollController, required this.tr});

  @override
  Widget build(BuildContext context) {
    return FutureBuilder(
      future: file.readAsString(),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }
        if (snapshot.hasError) {
          return Text(tr("ui_file_preview_error", {"error": snapshot.error.toString()}));
        }

        if (snapshot.hasData) {
          return CodeTheme(
            data: CodeThemeData(styles: monokaiTheme),
            // Code preview keeps its own editor scroller while the scaffold
            // owns the shared outer frame and metadata area.
            child: Scrollbar(
              thumbVisibility: true,
              controller: scrollController,
              child: SingleChildScrollView(
                controller: scrollController,
                child: CodeField(
                  textStyle: TextStyle(fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize),
                  readOnly: true,
                  gutterStyle: GutterStyle.none,
                  controller: CodeController(text: snapshot.data, readOnly: true, language: CodeFilePreviewRenderer.allCodeLanguages[fileExtension]!),
                ),
              ),
            ),
          );
        }

        return const SizedBox();
      },
    );
  }
}
