import 'package:flutter/material.dart';
import 'package:markdown_widget/markdown_widget.dart';

class WoxMarkdownView extends StatelessWidget {
  final String data;
  final Color fontColor;

  const WoxMarkdownView({super.key, required this.fontColor, required this.data});

  @override
  Widget build(BuildContext context) {
    final fontTextStyle = TextStyle(fontSize: 14, color: fontColor);
    final bool isDarkFont = fontColor.computeLuminance() < 0.5;
    final codeBackgroundColor = isDarkFont ? Colors.black.withValues(alpha: 0.06) : Colors.white.withValues(alpha: 0.08);
    final codeTextStyle = fontTextStyle.copyWith(fontSize: 13, color: fontColor);

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
          style: codeTextStyle.copyWith(backgroundColor: codeBackgroundColor),
        ),
        PreConfig(
          textStyle: codeTextStyle,
          styleNotMatched: codeTextStyle,
          decoration: BoxDecoration(color: codeBackgroundColor, borderRadius: BorderRadius.circular(4)),
        ),
        HrConfig(
          color: fontColor,
          height: 1.5,
        ),
        ListConfig(
          marker: (isOrdered, depth, index) => getDefaultMarker(
            isOrdered,
            depth,
            fontColor,
            index,
            8,
            MarkdownConfig(),
          ),
        ),
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
