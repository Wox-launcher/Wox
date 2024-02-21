import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:from_css_color/from_css_color.dart';
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

class WoxPreviewView extends StatefulWidget {
  final WoxPreview woxPreview;
  final WoxTheme woxTheme;

  const WoxPreviewView({super.key, required this.woxPreview, required this.woxTheme});

  @override
  State<WoxPreviewView> createState() => _WoxPreviewViewState();
}

class _WoxPreviewViewState extends State<WoxPreviewView> {
  final scrollController = ScrollController();

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
      var styleTheme = Theme.of(context).copyWith(
        textTheme: Theme.of(context).textTheme.apply(
              bodyColor: fromCssColor(widget.woxTheme.previewFontColor),
              displayColor: fromCssColor(widget.woxTheme.previewFontColor),
            ),
        cardColor: Colors.transparent,
      );
      contentWidget = Markdown(
          controller: scrollController,
          data: widget.woxPreview.previewData,
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
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TEXT.code) {
      contentWidget = SingleChildScrollView(
        controller: scrollController,
        child: SelectableText(
          widget.woxPreview.previewData,
          style: TextStyle(color: fromCssColor(widget.woxTheme.previewFontColor)),
        ),
      );
    } else if (widget.woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_PDF.code) {
      if (widget.woxPreview.previewData.isEmpty) {
        contentWidget = SelectableText("Invalid pdf data: ${widget.woxPreview.previewData}", style: const TextStyle(color: Colors.red));
      } else {
        if (widget.woxPreview.previewData.startsWith("http")) {
          contentWidget = SfPdfViewer.network(
            widget.woxPreview.previewData,
          );
        } else {
          contentWidget = SfPdfViewer.file(File(widget.woxPreview.previewData));
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
                              Text(e.key, style: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor))),
                              Text(e.value, style: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyContentColor))),
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
