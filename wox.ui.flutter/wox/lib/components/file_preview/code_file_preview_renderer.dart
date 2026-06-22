import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_code_editor/flutter_code_editor.dart';
import 'package:flutter_highlight/themes/monokai.dart';
import 'package:highlight/highlight.dart';
import 'package:highlight/languages/bash.dart';
import 'package:highlight/languages/cmake.dart';
import 'package:highlight/languages/cpp.dart';
import 'package:highlight/languages/cs.dart';
import 'package:highlight/languages/css.dart';
import 'package:highlight/languages/dart.dart';
import 'package:highlight/languages/diff.dart';
import 'package:highlight/languages/dockerfile.dart';
import 'package:highlight/languages/dos.dart';
import 'package:highlight/languages/go.dart';
import 'package:highlight/languages/gradle.dart';
import 'package:highlight/languages/ini.dart';
import 'package:highlight/languages/java.dart';
import 'package:highlight/languages/javascript.dart';
import 'package:highlight/languages/json.dart';
import 'package:highlight/languages/kotlin.dart';
import 'package:highlight/languages/less.dart';
import 'package:highlight/languages/makefile.dart';
import 'package:highlight/languages/markdown.dart';
import 'package:highlight/languages/nginx.dart';
import 'package:highlight/languages/objectivec.dart';
import 'package:highlight/languages/php.dart';
import 'package:highlight/languages/plaintext.dart';
import 'package:highlight/languages/powershell.dart';
import 'package:highlight/languages/properties.dart';
import 'package:highlight/languages/protobuf.dart';
import 'package:highlight/languages/python.dart';
import 'package:highlight/languages/ruby.dart';
import 'package:highlight/languages/rust.dart';
import 'package:highlight/languages/scala.dart';
import 'package:highlight/languages/scss.dart';
import 'package:highlight/languages/shell.dart';
import 'package:highlight/languages/sql.dart';
import 'package:highlight/languages/swift.dart';
import 'package:highlight/languages/typescript.dart';
import 'package:highlight/languages/xml.dart';
import 'package:highlight/languages/yaml.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class CodeFilePreviewRenderer implements WoxFilePreviewRenderer {
  // Keep this list explicit so code preview does not load highlight's full
  // language catalog during launcher startup.
  static final allCodeLanguages = <String, Mode>{
    "txt": plaintext,
    "text": plaintext,
    "log": plaintext,
    "conf": plaintext,
    "config": plaintext,
    "env": plaintext,
    "lock": plaintext,
    "toml": Mode(),
    "ini": ini,
    "properties": properties,
    "props": properties,
    "json": json,
    "yaml": yaml,
    "yml": yaml,
    "xml": xml,
    "html": xml,
    "htm": xml,
    "md": markdown,
    "markdown": markdown,
    "css": css,
    "scss": scss,
    "less": less,
    "js": javascript,
    "jsx": javascript,
    "mjs": javascript,
    "cjs": javascript,
    "ts": typescript,
    "tsx": typescript,
    "py": python,
    "pyw": python,
    "java": java,
    "kt": kotlin,
    "kts": kotlin,
    "dart": dart,
    "go": go,
    "rs": rust,
    "rb": ruby,
    "php": php,
    "scala": scala,
    "swift": swift,
    "c": cpp,
    "cc": cpp,
    "cpp": cpp,
    "cxx": cpp,
    "h": cpp,
    "hpp": cpp,
    "cs": cs,
    "m": objectivec,
    "mm": objectivec,
    "sh": bash,
    "bash": bash,
    "zsh": shell,
    "fish": shell,
    "ps1": powershell,
    "bat": dos,
    "cmd": dos,
    "sql": sql,
    "proto": protobuf,
    "diff": diff,
    "patch": diff,
    "dockerfile": dockerfile,
    "makefile": makefile,
    "mk": makefile,
    "cmake": cmake,
    "gradle": gradle,
    "nginx": nginx,
  };

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
