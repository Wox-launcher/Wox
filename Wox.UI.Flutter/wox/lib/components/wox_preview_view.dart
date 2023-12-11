import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxPreviewView extends StatelessWidget {
  final WoxPreview woxPreview;
  final WoxTheme woxTheme;

  const WoxPreviewView({super.key, required this.woxPreview, required this.woxTheme});

  @override
  Widget build(BuildContext context) {
    if (woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code) {
      return FutureBuilder<WoxPreview>(
        future: WoxHttpUtil.instance.getData<WoxPreview>(woxPreview.previewData),
        builder: (context, snapshot) {
          if (snapshot.hasData) {
            return WoxPreviewView(
              woxPreview: snapshot.data!,
              woxTheme: woxTheme,
            );
          } else if (snapshot.hasError) {
            return Text("${snapshot.error}");
          }

          return const CircularProgressIndicator();
        },
      );
    }

    Widget contentWidget = const SizedBox();
    if (woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_MARKDOWN.code) {
      var styleTheme = Theme.of(context).copyWith(
        textTheme: Theme.of(context).textTheme.apply(
              bodyColor: fromCssColor(woxTheme.previewFontColor),
              displayColor: fromCssColor(woxTheme.previewFontColor),
            ),
        cardColor: Colors.transparent,
      );
      contentWidget = Markdown(
        data: woxPreview.previewData,
        padding: EdgeInsets.zero,
        selectable: true,
        styleSheet: MarkdownStyleSheet.fromTheme(styleTheme),
      );
    } else if (woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TEXT.code) {
      contentWidget = SelectableText(woxPreview.previewData, style: TextStyle(color: fromCssColor(woxTheme.previewFontColor)));
    } else if (woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_IMAGE.code) {
      final parsedWoxImage = WoxImage.parse(woxPreview.previewData);
      if (parsedWoxImage == null) {
        contentWidget = SelectableText("Invalid image data: ${woxPreview.previewData}", style: const TextStyle(color: Colors.red));
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
          Expanded(child: contentWidget),
          //show previewProperties
          if (woxPreview.previewProperties.isNotEmpty)
            Container(
              padding: const EdgeInsets.only(top: 10.0),
              child: Column(
                children: [
                  ...woxPreview.previewProperties.entries.map((e) => Column(
                        children: [
                          Divider(color: fromCssColor(woxTheme.previewSplitLineColor)),
                          Row(
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              Text(e.key, style: TextStyle(color: fromCssColor(woxTheme.previewPropertyTitleColor))),
                              Text(e.value, style: TextStyle(color: fromCssColor(woxTheme.previewPropertyContentColor))),
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
