import 'package:flutter/material.dart';
import 'package:markdown_widget/markdown_widget.dart';
import 'package:wox/utils/colors.dart';

class WoxMarkdownView extends StatelessWidget {
  final String data;
  final Color fontColor;

  const WoxMarkdownView({super.key, required this.fontColor, required this.data});

  @override
  Widget build(BuildContext context) {
    final fontTextStyle = TextStyle(
      fontSize: 14,
      color: fontColor,
    );
    final bool isDarkFont = fontColor.computeLuminance() < 0.5;
    final contrastBackgroundColor = fontColor;
    final contrastFontStyle = isDarkFont ? fontTextStyle.copyWith(color: fontColor.lighter(90)) : fontTextStyle.copyWith(color: fontColor.darker(90));

    return MarkdownBlock(
      data: data,
      config: MarkdownConfig(configs: [
        PConfig(
          textStyle: fontTextStyle,
        ),
        H1Config(
          style: fontTextStyle,
        ),
        H2Config(
          style: fontTextStyle,
        ),
        H3Config(
          style: fontTextStyle,
        ),
        H4Config(
          style: fontTextStyle,
        ),
        H5Config(
          style: fontTextStyle,
        ),
        H6Config(
          style: fontTextStyle,
        ),
        BlockquoteConfig(
          textColor: fontColor,
        ),
        TableConfig(
          headerStyle: fontTextStyle,
          bodyStyle: fontTextStyle,
        ),
        CodeConfig(
          style: contrastFontStyle.copyWith(
            backgroundColor: contrastBackgroundColor,
          ),
        ),
        PreConfig(
          textStyle: contrastFontStyle,
          styleNotMatched: contrastFontStyle,
          decoration: BoxDecoration(
            color: contrastBackgroundColor,
            borderRadius: BorderRadius.circular(4),
          ),
        ),
        HrConfig(
          color: fontColor,
          height: 1.5,
        ),
        ListConfig(marker: (isOrdered, depth, index) => getDefaultMarker(isOrdered, depth, fontColor, index, 8, MarkdownConfig())),
        LinkConfig(
          style: TextStyle(
            decoration: TextDecoration.underline,
            decorationColor: fontColor,
            color: fontColor,
          ),
        )
      ]),
      selectable: true,
    );
  }
}
