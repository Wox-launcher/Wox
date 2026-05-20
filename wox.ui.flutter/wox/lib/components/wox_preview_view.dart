import 'dart:async';
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
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_ai_chat_view.dart';
import 'package:wox/components/wox_ai_stream_preview_view.dart';
import 'package:wox/components/wox_list_preview_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/components/wox_plugin_detail_view.dart';
import 'package:wox/components/wox_preview_scaffold.dart';
import 'package:wox/components/wox_query_requirement_settings_preview_view.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/components/wox_update_view.dart';
import 'package:wox/components/wox_trigger_keyword_conflict_preview_view.dart';
import 'package:wox/components/wox_webview_preview.dart';
import 'package:wox/components/wox_terminal_preview_view.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_preview_ai_stream.dart';
import 'package:wox/entity/wox_preview_list.dart';
import 'package:wox/entity/wox_query_requirement_settings_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_trigger_keyword_conflict_preview.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
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
  final launcherController = Get.find<WoxLauncherController>();
  final allCodeLanguages = {...allLanguages, "txt": Mode(), "conf": Mode(), "js": javascript, "ts": typescript, "yml": yaml, "sh": bash, "py": python};
  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  Widget scrollableContent({required Widget child}) {
    return child;
  }

  Widget buildMarkdown(String markdownData) {
    final textColor = safeFromCssColor(widget.woxTheme.previewFontColor);

    // Markdown no longer draws its own frame because WoxPreviewScaffold owns the
    // shared scroll surface. Keeping only content padding here lets markdown,
    // text, and image previews share one outer background and scrollbar model.
    return scrollableContent(
      child: Padding(
        padding: EdgeInsets.all(_metrics.previewMarkdownPadding),
        child: WoxMarkdownView(data: markdownData, fontColor: textColor, fontSize: _metrics.resultSubtitleFontSize, enableImageOverlay: true),
      ),
    );
  }

  Widget buildText(String txtData) {
    final textColor = safeFromCssColor(widget.woxTheme.previewFontColor);
    final quoteColor = textColor.withValues(alpha: 0.16);
    final bodyColor = textColor.withValues(alpha: 0.86);
    final quoteTextStyle = TextStyle(color: bodyColor, fontSize: _metrics.previewTextQuoteFontSize, height: 1.45, fontWeight: FontWeight.w400, letterSpacing: 0);
    final plainTextStyle = TextStyle(color: bodyColor, fontSize: _metrics.previewTextFontSize, height: 1.55, fontWeight: FontWeight.w400, letterSpacing: 0);

    // Text previews keep their reader typography and optional quote treatment,
    // but the frame moved to WoxPreviewScaffold so the scrollbar sits inside the
    // same outer surface used by markdown and image previews. The quote
    // treatment is still chosen from measured layout space because the preview
    // height changes when metadata pills are present.
    return scrollableContent(
      child: LayoutBuilder(
        builder: (context, constraints) {
          final viewportHeight = constraints.hasBoundedHeight ? constraints.maxHeight : constraints.minHeight;
          final viewportWidth = constraints.hasBoundedWidth ? constraints.maxWidth : constraints.minWidth;
          final quoteHorizontalPadding = _metrics.previewTextQuoteHPadding;
          final quoteTop = _metrics.previewTextQuoteTopPadding;
          final quoteBottom = _metrics.previewTextQuoteBottomPadding;
          final quoteSize = _metrics.previewTextQuoteGlyphSize;
          final quoteTextTopPadding = _metrics.previewTextQuoteTextTopPadding;
          final quoteTextBottomPadding = _metrics.previewTextQuoteTextBottomPadding;
          final quoteTextMaxWidth = viewportWidth - quoteHorizontalPadding * 2;
          // The quote glyphs are decorative background marks, so the fit check
          // should use the text padding area instead of subtracting the full
          // glyph height. Subtracting the full quote boxes was too conservative
          // and hid quotes even when the text visually fit between them.
          final quoteSafeHeight = viewportHeight - quoteTextTopPadding - quoteTextBottomPadding;
          var shouldShowQuote = false;

          if (viewportWidth.isFinite && viewportHeight.isFinite && quoteTextMaxWidth > 0 && quoteSafeHeight > 0) {
            final textPainter = TextPainter(text: TextSpan(text: txtData, style: quoteTextStyle), textAlign: TextAlign.center, textDirection: Directionality.of(context))
              ..layout(maxWidth: quoteTextMaxWidth);
            shouldShowQuote = textPainter.height <= quoteSafeHeight;
          }

          return SizedBox(
            height: shouldShowQuote ? viewportHeight : null,
            child: Stack(
              children: [
                if (shouldShowQuote)
                  Positioned(
                    left: _metrics.previewTextQuoteGlyphOffset,
                    top: quoteTop,
                    child: Text("“", style: TextStyle(color: quoteColor, fontSize: quoteSize, height: 1, fontWeight: FontWeight.w700)),
                  ),
                if (shouldShowQuote)
                  Positioned(
                    right: _metrics.previewTextQuoteGlyphOffset,
                    bottom: quoteBottom,
                    child: Text("”", style: TextStyle(color: quoteColor, fontSize: quoteSize, height: 1, fontWeight: FontWeight.w700)),
                  ),
                Padding(
                  padding: EdgeInsets.fromLTRB(
                    shouldShowQuote ? quoteHorizontalPadding : _metrics.previewTextPadding,
                    shouldShowQuote ? quoteTextTopPadding : _metrics.previewTextPadding,
                    shouldShowQuote ? quoteHorizontalPadding : _metrics.previewTextPadding,
                    shouldShowQuote ? quoteTextBottomPadding : _metrics.previewTextPadding,
                  ),
                  child: Align(
                    alignment: shouldShowQuote ? Alignment.center : Alignment.topLeft,
                    child: WoxSelectableText(txtData, textAlign: shouldShowQuote ? TextAlign.center : TextAlign.left, style: shouldShowQuote ? quoteTextStyle : plainTextStyle),
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  Widget buildPdf(String pdfPath) {
    final Widget viewer = pdfPath.startsWith("http") ? SfPdfViewer.network(widget.woxPreview.previewData) : SfPdfViewer.file(File(pdfPath));
    // pdf viewer will capture focus from query box, so we need to exclude it
    return ExcludeFocus(child: viewer);
  }

  Widget buildCode(String codePath, String fileExtension) {
    return FutureBuilder(
      future: File(codePath).readAsString(),
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }
        if (snapshot.hasError) {
          return Text("Error: ${snapshot.error}");
        }

        if (snapshot.hasData) {
          return CodeTheme(
            data: CodeThemeData(styles: monokaiTheme),
            // Code preview keeps its own editor scroller, but the scrollbar now
            // lives inside the scaffold-provided frame instead of floating on
            // the launcher panel.
            child: Scrollbar(
              thumbVisibility: true,
              controller: scrollController,
              child: SingleChildScrollView(
                controller: scrollController,
                child: CodeField(
                  // Preview typography is part of the launcher surface, so it
                  // follows interface density while settings controls keep
                  // their existing fixed sizing.
                  textStyle: TextStyle(fontSize: _metrics.resultSubtitleFontSize),
                  readOnly: true,
                  gutterStyle: GutterStyle.none,
                  controller: CodeController(text: snapshot.data, readOnly: true, language: allCodeLanguages[fileExtension]!),
                ),
              ),
            ),
          );
        }

        return const SizedBox();
      },
    );
  }

  bool canOpenPreviewImageOverlay(WoxImage image) {
    return image.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code ||
        image.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code ||
        image.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code;
  }

  Future<void> openPreviewImageOverlay(WoxImage image) async {
    final traceId = const UuidV4().generate();
    final start = DateTime.now();
    try {
      // Diagnostic logging: preview-image clicks usually use local files, so this marks the UI
      // boundary before core measures header/decode/native overlay costs.
      Logger.instance.info(traceId, "preview image overlay click start: type=${image.imageType}, dataLength=${image.imageData.length}");
      await WoxApi.instance.showPreviewImageOverlay(traceId, image);
      Logger.instance.info(traceId, "preview image overlay click finished, cost ${DateTime.now().difference(start).inMilliseconds} ms");
    } catch (e) {
      Logger.instance.error(traceId, "Failed to open preview image overlay: $e");
    }
  }

  Widget buildImageSurface(Widget image, {WoxImage? overlayImage}) {
    // The scaffold now supplies the shared image/text/markdown substrate. This
    // renderer only centers the asset and keeps the overlay affordance so images
    // do not create a nested frame inside the unified preview surface.
    return LayoutBuilder(
      builder: (context, constraints) {
        final content = SizedBox(
          width: constraints.maxWidth,
          height: constraints.maxHeight,
          child: Padding(padding: EdgeInsets.all(_metrics.scaledSpacing(12)), child: Center(child: image)),
        );
        if (overlayImage == null || !canOpenPreviewImageOverlay(overlayImage)) {
          return content;
        }

        // The inline image already communicates the visual target, but it previously behaved like
        // static decoration. The cursor plus click handler make the native overlay affordance clear
        // while keeping all enlarged-image rendering in core's overlay layer.
        return MouseRegion(
          cursor: SystemMouseCursors.click,
          child: GestureDetector(behavior: HitTestBehavior.opaque, onTap: () => unawaited(openPreviewImageOverlay(overlayImage)), child: content),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) {
      Logger.instance.debug(const UuidV4().generate(), "repaint: preview view data");
    }

    Widget contentWidget = const SizedBox();
    bool isPdfViewer = false;
    bool contentHandlesScrolling = false;
    if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_MARKDOWN.code) {
      contentWidget = buildMarkdown(widget.woxPreview.previewData);
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TEXT.code) {
      contentWidget = buildText(widget.woxPreview.previewData);
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_FILE.code) {
      if (widget.woxPreview.previewData.isEmpty) {
        contentWidget = WoxSelectableText("Invalid file data: ${widget.woxPreview.previewData}", style: const TextStyle(color: Colors.red));
      } else {
        // render by file extension
        var fileExtension = widget.woxPreview.previewData.split(".").last.toLowerCase();
        if (fileExtension == "pdf") {
          isPdfViewer = true;
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
              contentHandlesScrolling = true;
              contentWidget = buildImageSurface(
                SvgPicture.file(File(widget.woxPreview.previewData), fit: BoxFit.contain),
                overlayImage: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code, imageData: widget.woxPreview.previewData),
              );
            } else {
              contentHandlesScrolling = true;
              contentWidget = buildImageSurface(
                Image.file(File(widget.woxPreview.previewData), fit: BoxFit.contain),
                overlayImage: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code, imageData: widget.woxPreview.previewData),
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
          } else if (File(widget.woxPreview.previewData).existsSync()) {
            contentHandlesScrolling = true;
            contentWidget = buildCode(widget.woxPreview.previewData, fileExtension);
          } else {
            contentWidget = buildText("Code file not found: ${widget.woxPreview.previewData}");
          }
        } else {
          // unsupported file type
          contentWidget = buildText("Unsupported file type preview: $fileExtension");
        }
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_LIST.code) {
      try {
        // Plugins send generic list previews as JSON rows so long-running
        // actions can show progress without abusing file-list or markdown data.
        // The catch branch keeps malformed payloads debuggable.
        contentWidget = WoxListPreviewView(data: WoxPreviewListData.fromPreviewData(widget.woxPreview.previewData), woxTheme: widget.woxTheme);
      } catch (e) {
        contentWidget = buildText("Invalid list preview data: $e");
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_IMAGE.code) {
      final parsedWoxImage = WoxImage.parse(widget.woxPreview.previewData);
      if (parsedWoxImage == null) {
        contentWidget = WoxSelectableText("Invalid image data: ${widget.woxPreview.previewData}", style: const TextStyle(color: Colors.red));
      } else {
        contentHandlesScrolling = true;
        final overlayWoxImage = widget.woxPreview.previewOverlayData.isNotEmpty ? WoxImage.parse(widget.woxPreview.previewOverlayData) ?? parsedWoxImage : parsedWoxImage;
        contentWidget = buildImageSurface(WoxImageView(woxImage: parsedWoxImage), overlayImage: overlayWoxImage);
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_PLUGIN_DETAIL.code) {
      contentHandlesScrolling = true;
      contentWidget = WoxPluginDetailView(pluginDetailJson: widget.woxPreview.previewData);
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_CHAT.code) {
      var previewChatData = WoxAIChatData.fromJson(jsonDecode(widget.woxPreview.previewData));
      var chatController = Get.find<WoxAIChatController>();
      chatController.aiChatData.value = previewChatData;

      // Handle scroll position for chat view
      if (widget.woxPreview.scrollPosition == WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          chatController.scrollToBottomOfAiChat();
        });
      }

      // Chat view has its own layout structure with Expanded widgets, return it directly
      return Container(padding: const EdgeInsets.only(top: 10.0, bottom: 10.0), child: const WoxAIChatView());
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_UPDATE.code) {
      try {
        final previewData = UpdatePreviewData.fromJson(jsonDecode(widget.woxPreview.previewData));
        return WoxUpdateView(data: previewData);
      } catch (e) {
        contentWidget = buildText("Invalid update preview data: $e");
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_AI_STREAM.code) {
      try {
        // AI streams are rendered as a text-preview variant so reasoning can be
        // visually muted while metadata remains in the shared external pill row.
        contentWidget = WoxAIStreamPreviewView(data: WoxPreviewAIStream.fromPreviewData(widget.woxPreview.previewData), woxTheme: widget.woxTheme);
      } catch (e) {
        contentWidget = buildText("Invalid AI stream preview data: $e");
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_QUERY_REQUIREMENT_SETTINGS.code) {
      try {
        // Core generates this preview when query prerequisites block plugin
        // execution. Rendering it as a native settings form keeps users inside
        // the query flow instead of forcing a separate settings-window detour.
        final previewData = QueryRequirementSettingsPreviewData.fromPreviewData(widget.woxPreview.previewData);
        return WoxQueryRequirementSettingsPreviewView(data: previewData);
      } catch (e) {
        contentWidget = buildText("Invalid query requirement settings preview data: $e");
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TRIGGER_KEYWORD_CONFLICT.code) {
      try {
        // Core generates this preview when duplicate trigger keywords block query
        // dispatch. Rendering a dedicated editor lets users fix either plugin
        // while staying inside the ambiguous query that exposed the conflict.
        final previewData = TriggerKeywordConflictPreviewData.fromPreviewData(widget.woxPreview.previewData);
        return WoxTriggerKeywordConflictPreviewView(data: previewData);
      } catch (e) {
        contentWidget = buildText("Invalid trigger keyword conflict preview data: $e");
      }
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TERMINAL.code) {
      // Terminal previews have their own status bar, search state, and scrolling.
      // Keep them out of the generic shell so the new default styling does not
      // disturb the interactive terminal surface.
      return Container(
        padding: launcherController.isFullscreenPreviewOnly() ? EdgeInsets.zero : const EdgeInsets.only(top: 10.0, bottom: 10.0, left: 10.0),
        child: WoxTerminalPreviewView(woxPreview: widget.woxPreview, woxTheme: widget.woxTheme),
      );
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_WEBVIEW.code) {
      // WebView owns platform view sizing and navigation, so only preserve the
      // existing preview padding instead of wrapping it in the generic scroller.
      return Container(
        padding: launcherController.isFullscreenPreviewOnly() ? EdgeInsets.zero : const EdgeInsets.only(top: 10.0, bottom: 10.0, left: 10.0),
        child: WoxWebViewPreview(previewData: widget.woxPreview.previewData),
      );
    }

    if (widget.woxPreview.scrollPosition == WoxPreviewScrollPositionEnum.WOX_PREVIEW_SCROLL_POSITION_BOTTOM.code) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (scrollController.hasClients) {
          scrollController.jumpTo(scrollController.position.maxScrollExtent);
        }
      });
    }

    return WoxPreviewScaffold(
      woxTheme: widget.woxTheme,
      scrollController: scrollController,
      properties: launcherController.supportsPreviewFullscreen(widget.woxPreview) && launcherController.isPreviewFullscreen.value ? {} : widget.woxPreview.previewProperties,
      contentHandlesScrolling: isPdfViewer || contentHandlesScrolling,
      child: contentWidget,
    );
  }
}
