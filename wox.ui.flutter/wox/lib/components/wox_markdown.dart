import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:gpt_markdown/custom_widgets/custom_error_image.dart';
import 'package:gpt_markdown/custom_widgets/markdown_config.dart';
import 'package:gpt_markdown/custom_widgets/unordered_ordered_list.dart';
import 'package:gpt_markdown/gpt_markdown.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

class WoxMarkdownView extends StatelessWidget {
  final String data;
  final Color fontColor;
  final double fontSize;
  final Color? linkColor;
  final Color? linkHoverColor;
  final bool selectable;
  final bool enableImageOverlay;

  const WoxMarkdownView({
    super.key,
    required this.fontColor,
    required this.data,
    this.fontSize = 14,
    this.linkColor,
    this.linkHoverColor,
    this.selectable = true,
    this.enableImageOverlay = false,
  });

  @override
  Widget build(BuildContext context) {
    final baseTextStyle = TextStyle(fontSize: fontSize);
    final fontTextStyle = baseTextStyle.copyWith(color: fontColor);
    final bool isDarkFont = fontColor.computeLuminance() < 0.5;
    final codeBackgroundColor = isDarkFont ? Colors.black.withValues(alpha: 0.06) : Colors.white.withValues(alpha: 0.08);
    // Markdown code blocks inherit the caller's density-aware font size instead
    // of pinning preview code to the old normal-size bucket.
    final codeFontSize = (fontSize - 1).clamp(10.0, double.infinity).toDouble();
    final codeLabelFontSize = (fontSize - 2).clamp(9.0, double.infinity).toDouble();
    final codeTextStyle = fontTextStyle.copyWith(fontSize: codeFontSize, color: fontColor);
    final dividerColor = getThemeDividerColor();
    // Glass themes use low-contrast active/accent colors, so markdown links no
    // longer use caller-provided highlight colors. Keeping links text-colored
    // and underlined makes them readable on every preview surface.
    final effectiveLinkColor = fontColor;

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
      linkColor: effectiveLinkColor,
      linkHoverColor: effectiveLinkColor,
    );

    final normalizedData = normalizeMarkdownImages(data);

    final markdownBody = GptMarkdownTheme(
      gptThemeData: themeData,
      child: GptMarkdown(
        normalizedData,
        style: baseTextStyle,
        textDirection: Directionality.of(context),
        linkBuilder: (context, text, url, style) {
          TextSpan? span;
          if (text is TextSpan) {
            span = TextSpan(
              text: text.text,
              children: text.children,
              style: (text.style ?? style).copyWith(decoration: TextDecoration.underline, decorationColor: (text.style?.color ?? style.color)),
            );
          }
          final linkText = Text.rich(span ?? text, style: style);
          final linkWidget = MouseRegion(cursor: SystemMouseCursors.click, child: linkText);
          if (!selectable) {
            return linkWidget;
          }
          return SelectionContainer.disabled(child: linkWidget);
        },
        onLinkTap: (String url, String title) {
          final uri = Uri.tryParse(url);
          if (uri != null) {
            launchUrl(uri);
          }
        },
        // Bug fix: gpt_markdown 1.1.7 made imageBuilder a four-argument callback.
        // Passing the new dimensions into Wox's shared image builder keeps the upgraded
        // dependency compatible without splitting remote/local image behavior.
        imageBuilder: (context, url, width, height) => buildImage(context, url, width: width, height: height),
        tableBuilder: (context, rows, textStyle, config) => buildMarkdownTable(context, rows, textStyle, config, fontColor, dividerColor),
        inlineComponents: [ATagMd(), WoxImageMd(), TableMd(), StrikeMd(), BoldMd(), ItalicMd(), UnderLineMd(), LatexMath(), LatexMathMultiLine(), HighlightedText(), SourceTag()],
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
                    child: Text(trimmedName, style: codeTextStyle.copyWith(fontSize: codeLabelFontSize, fontWeight: FontWeight.w600)),
                  ),
                if (trimmedName.isNotEmpty) Divider(height: 1, color: dividerColor.withValues(alpha: 0.4)),
                SingleChildScrollView(scrollDirection: Axis.horizontal, padding: const EdgeInsets.all(8), child: Text(code, style: codeTextStyle)),
              ],
            ),
          );
        },
      ),
    );

    return DefaultTextStyle.merge(style: fontTextStyle, child: selectable ? WoxSelectionArea(child: markdownBody) : markdownBody);
  }

  Widget buildMarkdownTable(BuildContext context, List<CustomTableRow> rows, TextStyle textStyle, GptMarkdownConfig config, Color textColor, Color dividerColor) {
    final controller = ScrollController();
    final isLightText = textColor.computeLuminance() >= 0.5;
    final borderColor = dividerColor.withValues(alpha: isLightText ? 0.58 : 0.42);
    final headerBackgroundColor = isLightText ? Colors.white.withValues(alpha: 0.08) : Colors.black.withValues(alpha: 0.055);

    // Bug fix: gpt_markdown's built-in table renderer reads Flutter's ambient
    // Material theme instead of Wox's preview colors. In dark launcher previews
    // that produced a very bright header with almost invisible text, so Wox owns
    // the table colors here while still letting the package parse markdown cells.
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Scrollbar(
        controller: controller,
        child: SingleChildScrollView(
          controller: controller,
          scrollDirection: Axis.horizontal,
          child: Table(
            textDirection: config.textDirection,
            defaultColumnWidth: CustomTableColumnWidth(),
            defaultVerticalAlignment: TableCellVerticalAlignment.middle,
            border: TableBorder.all(width: 1, color: borderColor),
            children:
                rows
                    .map(
                      (row) => TableRow(
                        decoration: row.isHeader ? BoxDecoration(color: headerBackgroundColor) : null,
                        children: row.fields.map((field) => buildMarkdownTableCell(context, row, field, textStyle, config, textColor)).toList(),
                      ),
                    )
                    .toList(),
          ),
        ),
      ),
    );
  }

  Widget buildMarkdownTableCell(BuildContext context, CustomTableRow row, CustomTableField field, TextStyle textStyle, GptMarkdownConfig config, Color textColor) {
    final cellStyle = textStyle.copyWith(color: textColor, fontWeight: row.isHeader ? FontWeight.w700 : textStyle.fontWeight);
    final cellConfig = config.copyWith(style: cellStyle);
    Widget content = Padding(padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5), child: MdWidget(context, field.data.trim(), false, config: cellConfig));

    switch (field.alignment) {
      case TextAlign.center:
        content = Center(child: content);
        break;
      case TextAlign.right:
        content = Align(alignment: Alignment.centerRight, child: content);
        break;
      case TextAlign.left:
      default:
        content = Align(alignment: Alignment.centerLeft, child: content);
        break;
    }

    return content;
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
      return alt.isEmpty ? '![]($path)' : '![$alt]($path)';
    });

    return text;
  }

  Widget buildImage(BuildContext context, String url, {double? width, double? height}) {
    final trimmed = url.trim();
    if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
      return buildImageOverlayTrigger(
        applyMarkdownImageSize(
          Image.network(trimmed, fit: BoxFit.fill, errorBuilder: (context, error, stackTrace) => Text(error.toString(), style: const TextStyle(color: Colors.red))),
          width,
          height,
        ),
        WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code, imageData: trimmed),
      );
    }

    final resolvedPath = resolveLocalImagePath(trimmed);
    if (resolvedPath.isEmpty) {
      return Text(url);
    }
    final file = File(resolvedPath);
    return buildImageOverlayTrigger(
      applyMarkdownImageSize(Image.file(file, fit: BoxFit.fill, errorBuilder: (context, error, stackTrace) => Text(error.toString())), width, height),
      WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code, imageData: resolvedPath),
    );
  }

  Widget applyMarkdownImageSize(Widget image, double? width, double? height) {
    if (width == null && height == null) {
      return image;
    }

    // Bug fix: the package now owns parsing width/height metadata and passes it to the
    // callback. Applying it at Wox's image boundary keeps package-rendered images and
    // WoxImageMd-rendered images consistent without adding a second layout wrapper.
    return SizedBox(width: width, height: height, child: image);
  }

  Widget buildImageOverlayTrigger(Widget image, WoxImage overlayImage) {
    if (!enableImageOverlay) {
      return image;
    }

    // Markdown preview images used to be static inline content even when the same preview surface
    // could open a native overlay. This opt-in wrapper keeps settings and release-note markdown
    // unchanged while giving preview markdown the same enlarged-image affordance as image previews.
    return SelectionContainer.disabled(
      child: MouseRegion(
        cursor: SystemMouseCursors.click,
        child: GestureDetector(behavior: HitTestBehavior.opaque, onTap: () => unawaited(openMarkdownImageOverlay(overlayImage)), child: image),
      ),
    );
  }

  Future<void> openMarkdownImageOverlay(WoxImage image) async {
    final traceId = const UuidV4().generate();
    final start = DateTime.now();
    try {
      // Diagnostic logging: markdown images can be remote URLs, so keep the click boundary visible
      // while core logs the download/decode/native overlay stages for the same trace id.
      Logger.instance.info(traceId, "markdown image overlay click start: type=${image.imageType}, dataLength=${image.imageData.length}");
      await WoxApi.instance.showPreviewImageOverlay(traceId, image);
      Logger.instance.info(traceId, "markdown image overlay click finished, cost ${DateTime.now().difference(start).inMilliseconds} ms");
    } catch (e) {
      Logger.instance.error(traceId, "Failed to open markdown image overlay: $e");
    }
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

    double? width;
    double? height;
    if (altText.isNotEmpty) {
      // Bug fix: WoxImageMd owns this custom image syntax, so the upstream parser cannot
      // inject the new width/height arguments for us. Keep the existing alt-size syntax
      // but forward those dimensions through the current four-argument ImageBuilder API.
      final size = RegExp(r'^([0-9]+)?x?([0-9]+)?').firstMatch(altText.trim());
      width = double.tryParse(size?.group(1)?.trim() ?? '');
      height = double.tryParse(size?.group(2)?.trim() ?? '');
    }

    final imageBuilder = config.imageBuilder;
    final Widget image =
        imageBuilder?.call(context, url, width, height) ??
        Image(
          image: NetworkImage(url),
          loadingBuilder: (BuildContext context, Widget child, ImageChunkEvent? loadingProgress) {
            if (loadingProgress == null) {
              return child;
            }
            return const Center(child: WoxLoadingIndicator(size: 24));
          },
          fit: BoxFit.fill,
          errorBuilder: (context, error, stackTrace) {
            return const CustomImageError();
          },
        );

    final fallbackSizedImage = imageBuilder == null && (width != null || height != null) ? SizedBox(width: width, height: height, child: image) : image;
    return WidgetSpan(alignment: PlaceholderAlignment.bottom, child: fallbackSizedImage);
  }
}
