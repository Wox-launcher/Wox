import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_code_editor/flutter_code_editor.dart';
import 'package:flutter_inappwebview/flutter_inappwebview.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:flutter_svg/svg.dart';
import 'package:from_css_color/from_css_color.dart';
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
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/log.dart';
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
        physics: const ClampingScrollPhysics(),
        styleSheet: MarkdownStyleSheet.fromTheme(styleTheme).copyWith(
          horizontalRuleDecoration: BoxDecoration(
            border: Border(
              top: BorderSide(
                color: fromCssColor(widget.woxTheme.previewFontColor).withAlpha((0.6 * 255).round()),
                width: 1,
              ),
              bottom: const BorderSide(
                color: Colors.transparent,
                width: 10,
              ),
            ),
          ),
          blockquoteDecoration: BoxDecoration(
            color: fromCssColor(widget.woxTheme.previewFontColor).withAlpha((0.1 * 255).round()),
            border: Border(
              left: BorderSide(color: fromCssColor(widget.woxTheme.previewFontColor).withAlpha((0.2 * 255).round()), width: 2),
            ),
          ),
        ));
  }

  Widget buildText(String txtData) {
    return SingleChildScrollView(
      controller: scrollController,
      child: Scrollbar(
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

  Widget buildHtml(String htmlContent) {
    return Container(
      color: Colors.transparent,
      child: InAppWebView(
        initialData: InAppWebViewInitialData(
          data: htmlContent,
          mimeType: 'text/html',
          encoding: 'utf8',
        ),
        initialSettings: InAppWebViewSettings(
          supportZoom: false,
          transparentBackground: true,
          disableHorizontalScroll: false,
          disableVerticalScroll: false,
          javaScriptEnabled: true,
          isInspectable: true,
        ),
        gestureRecognizers: const {},
        onScrollChanged: (controller, x, y) {
          if (widget.woxPreview.scrollPosition == WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code) {
            controller.scrollTo(x: x, y: y);
          }
        },
        onPageCommitVisible: (controller, url) {
          // inject css color variables
          final fontColor = widget.woxTheme.previewFontColor;
          final backgroundColor = widget.woxTheme.appBackgroundColor;
          final splitLineColor = widget.woxTheme.previewSplitLineColor;
          final propertyTitleColor = widget.woxTheme.previewPropertyTitleColor;
          final propertyContentColor = widget.woxTheme.previewPropertyContentColor;
          final selectionColor = widget.woxTheme.previewTextSelectionColor;

          controller.evaluateJavascript(source: """
            var themeStyle = document.createElement('style');
            themeStyle.innerHTML = ":root { " +
              "--preview-font-color: $fontColor; " +
              "--preview-background-color: $backgroundColor; " +
              "--preview-split-line-color: $splitLineColor; " +
              "--preview-property-title-color: $propertyTitleColor; " +
              "--preview-property-content-color: $propertyContentColor; " +
              "--preview-selection-color: $selectionColor; " +
            "}";
            document.head.appendChild(themeStyle);
          """);

          var defaultFontFamily = DefaultTextStyle.of(context).style.fontFamily; // default maybe Roboto
          var fallbackFontFamily = "PingFang SC,Segoe UI,Microsoft YaHei,sans-serif";
          if (defaultFontFamily != null) {
            defaultFontFamily += ",$fallbackFontFamily";
          } else {
            defaultFontFamily = fallbackFontFamily;
          }

          // inject font css to match Flutter fonts
          controller.evaluateJavascript(source: """
            var fontStyle = document.createElement('style');
            fontStyle.innerHTML = `
              @import url('https://fonts.googleapis.com/css2?family=Roboto:ital,wght@0,100..900;1,100..900&display=swap');

              body, html {
                font-family: $defaultFontFamily;
                font-size: 14px;
                line-height: 1.5;
                -webkit-font-smoothing: antialiased;
                -moz-osx-font-smoothing: grayscale;
                margin: 0;
                padding: 0;
                background-color: transparent;
                color: var(--preview-font-color);
              }
            `;
            document.head.appendChild(fontStyle);
          """);
        },
      ),
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
        chatController.focusToChatInput(const UuidV4().toString());
        launcherController.hasPendingAutoFocusToChatInput = false;
      }

      contentWidget = const WoxAIChatView();
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_HTML.code) {
      contentWidget = buildHtml(widget.woxPreview.previewData);
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
