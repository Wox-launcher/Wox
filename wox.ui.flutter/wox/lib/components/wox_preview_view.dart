import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_code_editor/flutter_code_editor.dart';
import 'package:flutter_svg/svg.dart';
import 'package:get/get.dart';
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
import 'package:wox/components/wox_ai_chat_view.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/color_util.dart';
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
    "conf": Mode(),
    "js": javascript,
    "ts": typescript,
    "yml": yaml,
    "sh": bash,
    "py": python,
  };

  Widget scrollableContent({required Widget child}) {
    return Scrollbar(
      controller: scrollController,
      child: SingleChildScrollView(
        controller: scrollController,
        child: child,
      ),
    );
  }

  Widget buildMarkdown(String markdownData) {
    return scrollableContent(
      child: WoxMarkdownView(
        data: markdownData,
        fontColor: safeFromCssColor(widget.woxTheme.previewFontColor),
      ),
    );
  }

  Widget buildText(String txtData) {
    return scrollableContent(
      child: SelectableText(
        txtData,
        style: TextStyle(color: safeFromCssColor(widget.woxTheme.previewFontColor)),
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
              controller: scrollController,
              child: SingleChildScrollView(
                controller: scrollController,
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
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: preview view data");

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
          // if file is bigger than 1MB, do not preview
          var file = File(widget.woxPreview.previewData);
          if (file.lengthSync() > 1 * 1024 * 1024) {
            contentWidget = buildText("File too big to preview, current size: ${(file.lengthSync() / 1024 / 1024).toInt()} MB");
            return contentWidget;
          }

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
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code) {
      var previewChatData = WoxAIChatData.fromJson(jsonDecode(widget.woxPreview.previewData));
      var chatController = Get.find<WoxAIChatController>();
      var launcherController = Get.find<WoxLauncherController>();
      chatController.aiChatData.value = previewChatData;

      // If hasPendingAutoFocusToChatInput is true, focus to chat input after the UI has been built
      if (launcherController.hasPendingAutoFocusToChatInput) {
        chatController.focusToChatInput(const UuidV4().generate());
        launcherController.hasPendingAutoFocusToChatInput = false;
      }

      contentWidget = const WoxAIChatView();
    }

    if (widget.woxPreview.scrollPosition == WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (scrollController.hasClients) {
          scrollController.jumpTo(scrollController.position.maxScrollExtent);
        }
      });
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
                  selectionColor: safeFromCssColor(widget.woxTheme.previewTextSelectionColor),
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
                          Divider(color: safeFromCssColor(widget.woxTheme.previewSplitLineColor)),
                          Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              ConstrainedBox(
                                constraints: const BoxConstraints(maxWidth: 80),
                                child: Text(e.key, overflow: TextOverflow.ellipsis, style: TextStyle(color: safeFromCssColor(widget.woxTheme.previewPropertyTitleColor))),
                              ),
                              ConstrainedBox(
                                constraints: const BoxConstraints(maxWidth: 260),
                                child: Text(e.value, overflow: TextOverflow.ellipsis, style: TextStyle(color: safeFromCssColor(widget.woxTheme.previewPropertyContentColor))),
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
