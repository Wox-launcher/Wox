import 'dart:io';

import 'package:flutter/material.dart';
import 'package:gpt_markdown/custom_widgets/custom_error_image.dart';
import 'package:gpt_markdown/custom_widgets/markdown_config.dart';
import 'package:gpt_markdown/custom_widgets/unordered_ordered_list.dart';
import 'package:gpt_markdown/gpt_markdown.dart';
import 'package:wox/utils/colors.dart';

class WoxMarkdownView extends StatelessWidget {
  final String data;
  final Color fontColor;

  const WoxMarkdownView({super.key, required this.fontColor, required this.data});

  @override
  Widget build(BuildContext context) {
    const baseTextStyle = TextStyle(fontSize: 14);
    final fontTextStyle = baseTextStyle.copyWith(color: fontColor);
    final bool isDarkFont = fontColor.computeLuminance() < 0.5;
    final codeBackgroundColor = isDarkFont ? Colors.black.withValues(alpha: 0.06) : Colors.white.withValues(alpha: 0.08);
    final codeTextStyle = fontTextStyle.copyWith(fontSize: 13, color: fontColor);
    final dividerColor = getThemeDividerColor();
    final themeData = GptMarkdownThemeData(
      brightness: isDarkFont ? Brightness.light : Brightness.dark,
      highlightColor: codeBackgroundColor,
      h1: baseTextStyle,
      h2: baseTextStyle,
      h3: baseTextStyle,
      h4: baseTextStyle,
      h5: baseTextStyle,
      h6: baseTextStyle,
      hrLineThickness: 1.5,
      hrLineColor: dividerColor,
      linkColor: fontColor,
      linkHoverColor: fontColor.withValues(alpha: 0.85),
    );

    final normalizedData = normalizeMarkdownImages(data);

    return DefaultTextStyle.merge(
      style: fontTextStyle,
      child: SelectionArea(
        child: GptMarkdownTheme(
          gptThemeData: themeData,
          child: GptMarkdown(
            normalizedData,
            style: baseTextStyle,
            textDirection: Directionality.of(context),
            imageBuilder: (context, url) => buildImage(context, url),
            inlineComponents: [
              ATagMd(),
              WoxImageMd(),
              TableMd(),
              StrikeMd(),
              BoldMd(),
              ItalicMd(),
              UnderLineMd(),
              LatexMath(),
              LatexMathMultiLine(),
              HighlightedText(),
              SourceTag(),
            ],
            unOrderedListBuilder: (context, child, config) {
              final itemText = child is MdWidget ? child.exp.trimLeft() : '';
              if (RegExp(r'^\[(?:x|X| )\]\s+').hasMatch(itemText)) {
                return UnorderedListView(padding: 0, spacing: 0, bulletSize: 0, textDirection: config.textDirection, child: child);
              }
              final bulletColor = config.style?.color ?? DefaultTextStyle.of(context).style.color;
              final fontSize = config.style?.fontSize ?? DefaultTextStyle.of(context).style.fontSize ?? 14;
              return UnorderedListView(bulletColor: bulletColor, padding: 7, spacing: 10, bulletSize: 0.3 * fontSize, textDirection: config.textDirection, child: child);
            },
            codeBuilder: (context, name, code, closed) {
              final trimmedName = name.trim();
              final borderRadius = BorderRadius.circular(closed ? 4 : 0);
              return Container(
                margin: const EdgeInsets.symmetric(vertical: 6),
                decoration: BoxDecoration(color: codeBackgroundColor, borderRadius: borderRadius),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (trimmedName.isNotEmpty)
                      Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
                        child: Text(trimmedName, style: codeTextStyle.copyWith(fontSize: 12, fontWeight: FontWeight.w600)),
                      ),
                    if (trimmedName.isNotEmpty) Divider(height: 1, color: dividerColor.withValues(alpha: 0.4)),
                    SingleChildScrollView(scrollDirection: Axis.horizontal, padding: const EdgeInsets.all(8), child: Text(code, style: codeTextStyle)),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  String normalizeMarkdownImages(String input) {
    var text = input.replaceAllMapped(RegExp(r'!\[\[([^\]]+)\]\]'), (match) {
      final content = match.group(1)?.trim() ?? '';
      if (content.isEmpty) {
        return match.group(0) ?? '';
      }
      final parts = content.split('|');
      final path = parts.first.trim();
      final alt = parts.length > 1 ? parts.sublist(1).join('|').trim() : '';
      return alt.isEmpty ? '![]($path)' : '![${alt}]($path)';
    });

    return text;
  }

  Widget buildImage(BuildContext context, String url) {
    final trimmed = url.trim();
    if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
      return Image.network(trimmed, fit: BoxFit.fill, errorBuilder: (context, error, stackTrace) => const SizedBox());
    }

    final resolvedPath = resolveLocalImagePath(trimmed);
    if (resolvedPath.isEmpty) {
      return Text(url);
    }
    final file = File(resolvedPath);
    return Image.file(file, fit: BoxFit.fill, errorBuilder: (context, error, stackTrace) => const SizedBox());
  }

  String resolveLocalImagePath(String url) {
    if (url.startsWith('file://')) {
      return Uri.parse(url).toFilePath();
    }
    if (url.isEmpty) {
      return '';
    }
    return url.startsWith('/') ? url : '';
  }
}

class WoxImageMd extends InlineMd {
  @override
  RegExp get exp => RegExp(r'\!\[[^\[\]]*\]\([^\n]*?\)');

  @override
  InlineSpan span(BuildContext context, String text, GptMarkdownConfig config) {
    final basicMatch = RegExp(r'\!\[([^\[\]]*)\]\(').firstMatch(text.trim());
    if (basicMatch == null) {
      return const TextSpan();
    }

    final altText = basicMatch.group(1) ?? '';
    final urlStart = basicMatch.end;

    int parenCount = 0;
    int urlEnd = urlStart;

    for (int i = urlStart; i < text.length; i++) {
      final char = text[i];

      if (char == '(') {
        parenCount++;
      } else if (char == ')') {
        if (parenCount == 0) {
          urlEnd = i;
          break;
        } else {
          parenCount--;
        }
      }
    }

    if (urlEnd == urlStart) {
      return const TextSpan();
    }

    final url = text.substring(urlStart, urlEnd).trim();

    double? height;
    double? width;
    if (altText.isNotEmpty) {
      var size = RegExp(r'^([0-9]+)?x?([0-9]+)?').firstMatch(altText.trim());
      width = double.tryParse(size?[1]?.toString().trim() ?? 'a');
      height = double.tryParse(size?[2]?.toString().trim() ?? 'a');
    }

    final Widget image =
        config.imageBuilder?.call(context, url) ??
        Image(
          image: NetworkImage(url),
          loadingBuilder: (BuildContext context, Widget child, ImageChunkEvent? loadingProgress) {
            if (loadingProgress == null) {
              return child;
            }
            return CustomImageLoading(progress: loadingProgress.expectedTotalBytes != null ? loadingProgress.cumulativeBytesLoaded / loadingProgress.expectedTotalBytes! : 1);
          },
          fit: BoxFit.fill,
          errorBuilder: (context, error, stackTrace) {
            return const CustomImageError();
          },
        );

    final sizedImage = (width != null || height != null) ? SizedBox(width: width, height: height, child: image) : image;
    return WidgetSpan(alignment: PlaceholderAlignment.bottom, child: sizedImage);
  }
}
