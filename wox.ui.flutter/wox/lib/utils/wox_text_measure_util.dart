import 'package:flutter/material.dart';

class WoxTextMeasureUtil {
  WoxTextMeasureUtil._();

  static TextStyle _resolveStyle(BuildContext context, TextStyle style) {
    // Keep behavior aligned with Text widget inheritance.
    return DefaultTextStyle.of(context).style.merge(style);
  }

  static TextPainter _createPainter({required BuildContext context, required String text, required TextStyle style, int? maxLines}) {
    return TextPainter(
      text: TextSpan(text: text, style: _resolveStyle(context, style)),
      textDirection: Directionality.of(context),
      textScaler: MediaQuery.textScalerOf(context),
      locale: Localizations.maybeLocaleOf(context),
      maxLines: maxLines,
    );
  }

  static double measureTextWidth({required BuildContext context, required String text, required TextStyle style, int? maxLines = 1}) {
    final painter = _createPainter(context: context, text: text, style: style, maxLines: maxLines)..layout(minWidth: 0, maxWidth: double.infinity);
    return painter.width;
  }

  static Size measureTextSize({
    required BuildContext context,
    required String text,
    required TextStyle style,
    int? maxLines,
    double minWidth = 0,
    double maxWidth = double.infinity,
  }) {
    final painter = _createPainter(context: context, text: text, style: style, maxLines: maxLines)..layout(minWidth: minWidth, maxWidth: maxWidth);
    return Size(painter.width, painter.height);
  }

  static bool isTextOverflow({required BuildContext context, required String text, required TextStyle style, required double maxWidth, int? maxLines = 1}) {
    if (maxWidth <= 0) {
      return false;
    }

    final painter = _createPainter(context: context, text: text, style: style, maxLines: maxLines)..layout(minWidth: 0, maxWidth: double.infinity);
    return painter.width > maxWidth;
  }
}
