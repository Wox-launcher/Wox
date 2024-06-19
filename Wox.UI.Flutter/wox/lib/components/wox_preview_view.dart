import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_code_editor/flutter_code_editor.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_svg/svg.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:highlight/highlight.dart';
import 'package:highlight/languages/all.dart';
import 'package:highlight/languages/bash.dart';
import 'package:highlight/languages/javascript.dart';
import 'package:highlight/languages/python.dart';
import 'package:highlight/languages/typescript.dart';
import 'package:highlight/languages/yaml.dart';
import 'package:syncfusion_flutter_pdfviewer/pdfviewer.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_http_util.dart';
import 'package:flutter_highlight/themes/monokai.dart';

class WoxPreviewView extends StatefulWidget {
  final WoxPreview woxPreview;
  final WoxTheme woxTheme;

  const WoxPreviewView({super.key, required this.woxPreview, required this.woxTheme});

  @override
  State<WoxPreviewView> createState() => _WoxPreviewViewState();
}

class _WoxPreviewViewState extends State<WoxPreviewView> {
  final scrollController = ScrollController();
  final allCodeLanguages = {
    ...allLanguages,
    "txt": Mode(),
    "js": javascript,
    "ts": typescript,
    "yml": yaml,
    "sh": bash,
    "py": python,
  };

  Widget buildMarkdown(String markdownData) {
    var styleTheme = Theme.of(context).copyWith(
      textTheme: Theme.of(context).textTheme.apply(
            bodyColor: fromCssColor(widget.woxTheme.previewFontColor),
            displayColor: fromCssColor(widget.woxTheme.previewFontColor),
          ),
      cardColor: Colors.transparent,
    );

    return Markdown(
        controller: scrollController,
        data: markdownData,
        padding: EdgeInsets.zero,
        selectable: true,
        styleSheet: MarkdownStyleSheet.fromTheme(styleTheme).copyWith(
          horizontalRuleDecoration: BoxDecoration(
            border: Border(
              top: BorderSide(
                color: fromCssColor(widget.woxTheme.previewFontColor).withOpacity(0.6),
                width: 1,
              ),
              bottom: const BorderSide(
                color: Colors.transparent,
                width: 10,
              ),
            ),
          ),
        ));
  }

  Widget buildText(String txtData) {
    return Scrollbar(
      child: SingleChildScrollView(
        controller: scrollController,
        child: SelectableText(
          txtData,
          style: TextStyle(color: fromCssColor(widget.woxTheme.previewFontColor)),
        ),
      ),
    );
  }

  Widget buildPdf(String pdfPath) {
    if (pdfPath.startsWith("http")) {
      return SfPdfViewer.network(widget.woxPreview.previewData);
    } else {
      return SfPdfViewer.file(File(pdfPath));
    }
  }

  Widget buildCode(String codePath, String fileExtension) {
    return FutureBuilder(
      future: File(codePath).readAsString(),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const CircularProgressIndicator();
        }
        if (snapshot.hasError) {
          return Text("Error: ${snapshot.error}");
        }

        if (snapshot.hasData) {
          return CodeTheme(
            data: CodeThemeData(styles: monokaiTheme),
            child: Scrollbar(
              child: SingleChildScrollView(
                child: CodeField(
                  textStyle: const TextStyle(fontSize: 13),
                  readOnly: true,
                  gutterStyle: GutterStyle.none,
                  controller: CodeController(
                    text: snapshot.data,
                    readOnly: true,
                    language: allCodeLanguages[fileExtension]!,
                  ),
                ),
              ),
            ),
          );
        }

        return const SizedBox();
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: preview view data");

    if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code) {
      return FutureBuilder<WoxPreview>(
        future: WoxHttpUtil.instance.getData<WoxPreview>(widget.woxPreview.previewData),
        builder: (context, snapshot) {
          if (snapshot.hasData) {
            return WoxPreviewView(
              woxPreview: snapshot.data!,
              woxTheme: widget.woxTheme,
            );
          } else if (snapshot.hasError) {
            return Text("${snapshot.error}");
          }

          return const CircularProgressIndicator();
        },
      );
    }

    if (widget.woxPreview.scrollPosition == WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (scrollController.hasClients) {
          scrollController.jumpTo(scrollController.position.maxScrollExtent);
        }
      });
    }

    Widget contentWidget = const SizedBox();
    if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_MARKDOWN.code) {
      contentWidget = buildMarkdown(widget.woxPreview.previewData);
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TEXT.code) {
      contentWidget = buildText(widget.woxPreview.previewData);
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_FILE.code) {
      if (widget.woxPreview.previewData.isEmpty) {
        contentWidget = SelectableText("Invalid file data: ${widget.woxPreview.previewData}", style: const TextStyle(color: Colors.red));
      } else {
        // render by file extension
        var fileExtension = widget.woxPreview.previewData.split(".").last.toLowerCase();
        if (fileExtension == "pdf") {
          contentWidget = buildPdf(widget.woxPreview.previewData);
        } else if (fileExtension == "md") {
          if (File(widget.woxPreview.previewData).existsSync()) {
            contentWidget = buildMarkdown(File(widget.woxPreview.previewData).readAsStringSync());
          } else {
            contentWidget = buildText("Markdown file not found: ${widget.woxPreview.previewData}");
          }
        } else if (fileExtension == "png" ||
            fileExtension == "gif" ||
            fileExtension == "bmp" ||
            fileExtension == "webp" ||
            fileExtension == "jpeg" ||
            fileExtension == "jpg" ||
            fileExtension == "svg") {
          if (File(widget.woxPreview.previewData).existsSync()) {
            if (fileExtension == "svg") {
              contentWidget = Center(
                child: SvgPicture.file(File(widget.woxPreview.previewData)),
              );
            } else {
              contentWidget = Center(
                child: Image.file(File(widget.woxPreview.previewData)),
              );
            }
          } else {
            contentWidget = buildText("Image file not found: ${widget.woxPreview.previewData}");
          }
        } else if (allCodeLanguages.containsKey(fileExtension)) {
          if (File(widget.woxPreview.previewData).existsSync()) {
            contentWidget = buildCode(widget.woxPreview.previewData, fileExtension);
          } else {
            contentWidget = buildText("Code file not found: ${widget.woxPreview.previewData}");
          }
        } else {
          // unsupported file type
          contentWidget = buildText("Unsupported file type preview: $fileExtension");
        }
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_IMAGE.code) {
      final parsedWoxImage = WoxImage.parse(widget.woxPreview.previewData);
      if (parsedWoxImage == null) {
        contentWidget = SelectableText("Invalid image data: ${widget.woxPreview.previewData}", style: const TextStyle(color: Colors.red));
      } else {
        contentWidget = Center(
          child: WoxImageView(woxImage: parsedWoxImage),
        );
      }
    }

    return Container(
      padding: const EdgeInsets.all(10.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Theme(
              data: ThemeData(
                textSelectionTheme: TextSelectionThemeData(
                  selectionColor: fromCssColor(widget.woxTheme.previewTextSelectionColor),
                ),
              ),
              child: contentWidget,
            ),
          ),
          //show previewProperties
          if (widget.woxPreview.previewProperties.isNotEmpty)
            Container(
              padding: const EdgeInsets.only(top: 10.0),
              child: Column(
                children: [
                  ...widget.woxPreview.previewProperties.entries.map((e) => Column(
                        children: [
                          Divider(color: fromCssColor(widget.woxTheme.previewSplitLineColor)),
                          Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              ConstrainedBox(
                                constraints: const BoxConstraints(maxWidth: 80),
                                child: Text(e.key, overflow: TextOverflow.ellipsis, style: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor))),
                              ),
                              ConstrainedBox(
                                constraints: const BoxConstraints(maxWidth: 260),
                                child: Text(e.value, overflow: TextOverflow.ellipsis, style: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyContentColor))),
                              ),
                            ],
                          ),
                        ],
                      ))
                ],
              ),
            ),
        ],
      ),
    );
  }
}
