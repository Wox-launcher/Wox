import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_code_editor/flutter_code_editor.dart';
import 'package:flutter_highlight/themes/github.dart';
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
import 'package:wox/components/file_preview/file_preview_policy.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class CodeFilePreviewRenderer implements WoxFilePreviewRenderer {
  // Syntax-highlighted CodeField construction is much heavier than plain text.
  // Keep the highlighter reserved for small files and let larger code files use
  // the limited plain-text preview instead of forcing a manual load step.
  static const int codeHighlightedMaxBytes = 30 * 1024;
  static const int codeHighlightedMaxLines = 4000;
  static const int codePlainPreviewMaxBytes = 512 * 1024;
  static const int codePlainPreviewMaxLines = 2000;

  // Keep this list explicit so code preview does not load highlight's full
  // language catalog during launcher startup.
  static final allCodeLanguages = <String, Mode>{
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
    return WoxFilePreviewPolicy.buildDeferredPreview(
      context: context,
      file: file,
      manualLoadThresholdBytes: fileSize,
      icon: Icons.code_rounded,
      accent: const Color(0xFF8B5CF6),
      typeLabel: WoxFilePreviewPolicy.extensionTypeLabel(context),
      previewBuilder: (_) => _CodeFilePreview(file: file, fileExtension: context.fileExtension, scrollController: context.scrollController, tr: context.tr),
    );
  }
}

class _CodeFilePreview extends StatefulWidget {
  final File file;
  final String fileExtension;
  final ScrollController scrollController;
  final WoxFilePreviewTranslationFormatter tr;

  const _CodeFilePreview({required this.file, required this.fileExtension, required this.scrollController, required this.tr});

  @override
  State<_CodeFilePreview> createState() => _CodeFilePreviewState();
}

class _CodeFilePreviewState extends State<_CodeFilePreview> {
  late final Future<_CodePreviewData> _previewFuture;

  @override
  void initState() {
    super.initState();
    _previewFuture = _loadCodePreview(widget.file);
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder(
      future: _previewFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }
        if (snapshot.hasError) {
          return Text(widget.tr("ui_file_preview_error", {"error": snapshot.error.toString()}));
        }

        if (snapshot.hasData) {
          final data = snapshot.data!;
          if (data.useSyntaxHighlighting) {
            return _buildHighlightedPreview(data);
          }
          return _buildPlainPreview(data);
        }

        return const SizedBox();
      },
    );
  }

  Widget _buildHighlightedPreview(_CodePreviewData data) {
    final codeTheme = _codePreviewTheme();
    final codeBackgroundColor = _codePreviewBackgroundColor();
    return CodeTheme(
      data: CodeThemeData(styles: codeTheme),
      // Code preview keeps its own editor scroller while the scaffold owns the
      // shared outer frame and metadata area. Large files never reach this path
      // because CodeController highlights and lays out the whole document on the
      // UI isolate.
      child: LayoutBuilder(
        builder:
            (context, constraints) => Container(
              color: codeBackgroundColor,
              child: Scrollbar(
                thumbVisibility: true,
                controller: widget.scrollController,
                child: SingleChildScrollView(
                  controller: widget.scrollController,
                  child: ConstrainedBox(
                    constraints: BoxConstraints(minWidth: constraints.maxWidth, maxWidth: constraints.maxWidth, minHeight: constraints.maxHeight),
                    child: CodeField(
                      background: codeBackgroundColor,
                      textStyle: TextStyle(fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize),
                      readOnly: true,
                      gutterStyle: GutterStyle.none,
                      controller: CodeController(text: data.text, readOnly: true, language: CodeFilePreviewRenderer.allCodeLanguages[widget.fileExtension]!),
                    ),
                  ),
                ),
              ),
            ),
      ),
    );
  }

  Widget _buildPlainPreview(_CodePreviewData data) {
    final codeTheme = _codePreviewTheme();
    final codeBackgroundColor = _codePreviewBackgroundColor();
    final codeTextColor = _codePreviewTextColor(codeTheme);
    final metrics = WoxInterfaceSizeUtil.instance.current;

    return LayoutBuilder(
      builder:
          (context, constraints) => Container(
            color: codeBackgroundColor,
            child: Scrollbar(
              thumbVisibility: true,
              controller: widget.scrollController,
              child: SingleChildScrollView(
                controller: widget.scrollController,
                child: ConstrainedBox(
                  constraints: BoxConstraints(minWidth: constraints.maxWidth, maxWidth: constraints.maxWidth, minHeight: constraints.maxHeight),
                  child: Padding(
                    padding: EdgeInsets.all(metrics.scaledSpacing(12)),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        if (data.isTruncated)
                          Padding(
                            padding: EdgeInsets.only(bottom: metrics.scaledSpacing(12)),
                            child: Text(
                              widget.tr("ui_file_preview_code_preview_limited", {"lines": data.lineCount.toString()}),
                              style: TextStyle(color: codeTextColor.withValues(alpha: 0.72), fontSize: metrics.smallLabelFontSize, height: 1.35),
                            ),
                          ),
                        WoxSelectableText(
                          data.text,
                          style: TextStyle(
                            color: codeTextColor,
                            fontSize: metrics.resultSubtitleFontSize,
                            height: 1.35,
                            fontFamily: "monospace",
                            fontFamilyFallback: const ["Menlo", "Consolas", "Courier New"],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
    );
  }
}

Map<String, TextStyle> _codePreviewTheme() {
  final theme = Map<String, TextStyle>.from(isThemeDark() ? monokaiTheme : githubTheme);
  final rootStyle = theme["root"] ?? const TextStyle();
  theme["root"] = rootStyle.copyWith(backgroundColor: _codePreviewBackgroundColor());
  return theme;
}

Color _codePreviewBackgroundColor() {
  return getThemeBackgroundColor();
}

Color _codePreviewTextColor(Map<String, TextStyle> theme) {
  return theme["root"]?.color ?? (isThemeDark() ? Colors.grey.shade100 : Colors.grey.shade900);
}

class _CodePreviewData {
  final String text;
  final int lineCount;
  final bool useSyntaxHighlighting;
  final bool isTruncated;

  const _CodePreviewData({required this.text, required this.lineCount, required this.useSyntaxHighlighting, required this.isTruncated});
}

Future<_CodePreviewData> _loadCodePreview(File file) async {
  final fileSize = file.lengthSync();
  if (fileSize > CodeFilePreviewRenderer.codeHighlightedMaxBytes) {
    return await _loadPlainCodePreview(file, isTruncated: true);
  }

  final text = await file.readAsString();
  final lineCount = _countLines(text);
  if (lineCount > CodeFilePreviewRenderer.codeHighlightedMaxLines) {
    final limited = _limitLines(text, CodeFilePreviewRenderer.codePlainPreviewMaxLines);
    return _CodePreviewData(text: limited.text, lineCount: limited.lineCount, useSyntaxHighlighting: false, isTruncated: limited.isTruncated);
  }

  return _CodePreviewData(text: text, lineCount: lineCount, useSyntaxHighlighting: true, isTruncated: false);
}

Future<_CodePreviewData> _loadPlainCodePreview(File file, {required bool isTruncated}) async {
  final handle = await file.open();
  late final List<int> bytes;
  try {
    bytes = await handle.read(CodeFilePreviewRenderer.codePlainPreviewMaxBytes + 1);
  } finally {
    await handle.close();
  }

  final truncatedByBytes = bytes.length > CodeFilePreviewRenderer.codePlainPreviewMaxBytes;
  final previewBytes = truncatedByBytes ? bytes.sublist(0, CodeFilePreviewRenderer.codePlainPreviewMaxBytes) : bytes;
  final text = _decodeTextPreview(previewBytes);
  final limited = _limitLines(text, CodeFilePreviewRenderer.codePlainPreviewMaxLines);
  return _CodePreviewData(text: limited.text, lineCount: limited.lineCount, useSyntaxHighlighting: false, isTruncated: isTruncated || truncatedByBytes || limited.isTruncated);
}

String _decodeTextPreview(List<int> bytes) {
  if (bytes.length >= 3 && bytes[0] == 0xEF && bytes[1] == 0xBB && bytes[2] == 0xBF) {
    return utf8.decode(bytes.sublist(3), allowMalformed: true);
  }
  return utf8.decode(bytes, allowMalformed: true);
}

int _countLines(String text) {
  if (text.isEmpty) {
    return 0;
  }

  var count = 1;
  for (var i = 0; i < text.length; i++) {
    if (text.codeUnitAt(i) == 0x0A) {
      count++;
    }
  }
  return count;
}

_LimitedCodeText _limitLines(String text, int maxLines) {
  if (text.isEmpty || maxLines <= 0) {
    return const _LimitedCodeText(text: "", lineCount: 0, isTruncated: false);
  }

  var lineCount = 1;
  for (var i = 0; i < text.length; i++) {
    if (text.codeUnitAt(i) != 0x0A) {
      continue;
    }
    if (lineCount >= maxLines) {
      return _LimitedCodeText(text: text.substring(0, i), lineCount: lineCount, isTruncated: true);
    }
    lineCount++;
  }

  return _LimitedCodeText(text: text, lineCount: lineCount, isTruncated: false);
}

class _LimitedCodeText {
  final String text;
  final int lineCount;
  final bool isTruncated;

  const _LimitedCodeText({required this.text, required this.lineCount, required this.isTruncated});
}
