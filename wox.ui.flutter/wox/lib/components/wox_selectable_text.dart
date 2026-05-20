import 'dart:ui' show BoxHeightStyle, BoxWidthStyle;

import 'package:flutter/cupertino.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxSelectableText extends StatelessWidget {
  final String? data;
  final TextSpan? textSpan;
  final FocusNode? focusNode;
  final TextStyle? style;
  final StrutStyle? strutStyle;
  final TextAlign? textAlign;
  final TextDirection? textDirection;
  final TextScaler? textScaler;
  final bool showCursor;
  final bool autofocus;
  final int? minLines;
  final int? maxLines;
  final double cursorWidth;
  final double? cursorHeight;
  final Radius? cursorRadius;
  final Color? cursorColor;
  final Color? selectionColor;
  final BoxHeightStyle selectionHeightStyle;
  final BoxWidthStyle selectionWidthStyle;
  final DragStartBehavior dragStartBehavior;
  final bool enableInteractiveSelection;
  final TextSelectionControls? selectionControls;
  final GestureTapCallback? onTap;
  final ScrollPhysics? scrollPhysics;
  final ScrollBehavior? scrollBehavior;
  final String? semanticsLabel;
  final TextHeightBehavior? textHeightBehavior;
  final TextWidthBasis? textWidthBasis;
  final SelectionChangedCallback? onSelectionChanged;
  final TextMagnifierConfiguration? magnifierConfiguration;

  const WoxSelectableText(
    String this.data, {
    super.key,
    this.focusNode,
    this.style,
    this.strutStyle,
    this.textAlign,
    this.textDirection,
    this.textScaler,
    this.showCursor = false,
    this.autofocus = false,
    this.minLines,
    this.maxLines,
    this.cursorWidth = 2.0,
    this.cursorHeight,
    this.cursorRadius,
    this.cursorColor,
    this.selectionColor,
    this.selectionHeightStyle = BoxHeightStyle.tight,
    this.selectionWidthStyle = BoxWidthStyle.tight,
    this.dragStartBehavior = DragStartBehavior.start,
    this.enableInteractiveSelection = true,
    this.selectionControls,
    this.onTap,
    this.scrollPhysics,
    this.scrollBehavior,
    this.semanticsLabel,
    this.textHeightBehavior,
    this.textWidthBasis,
    this.onSelectionChanged,
    this.magnifierConfiguration,
  }) : textSpan = null;

  const WoxSelectableText.rich(
    TextSpan this.textSpan, {
    super.key,
    this.focusNode,
    this.style,
    this.strutStyle,
    this.textAlign,
    this.textDirection,
    this.textScaler,
    this.showCursor = false,
    this.autofocus = false,
    this.minLines,
    this.maxLines,
    this.cursorWidth = 2.0,
    this.cursorHeight,
    this.cursorRadius,
    this.cursorColor,
    this.selectionColor,
    this.selectionHeightStyle = BoxHeightStyle.tight,
    this.selectionWidthStyle = BoxWidthStyle.tight,
    this.dragStartBehavior = DragStartBehavior.start,
    this.enableInteractiveSelection = true,
    this.selectionControls,
    this.onTap,
    this.scrollPhysics,
    this.scrollBehavior,
    this.semanticsLabel,
    this.textHeightBehavior,
    this.textWidthBasis,
    this.onSelectionChanged,
    this.magnifierConfiguration,
  }) : data = null;

  @override
  Widget build(BuildContext context) {
    final child = data != null ? _buildPlainText() : _buildRichText();
    return WoxSelectionTheme(child: child);
  }

  Widget _buildPlainText() {
    return SelectableText(
      data!,
      focusNode: focusNode,
      style: style,
      strutStyle: strutStyle,
      textAlign: textAlign,
      textDirection: textDirection,
      textScaler: textScaler,
      showCursor: showCursor,
      autofocus: autofocus,
      minLines: minLines,
      maxLines: maxLines,
      cursorWidth: cursorWidth,
      cursorHeight: cursorHeight,
      cursorRadius: cursorRadius,
      cursorColor: cursorColor,
      selectionColor: selectionColor,
      selectionHeightStyle: selectionHeightStyle,
      selectionWidthStyle: selectionWidthStyle,
      dragStartBehavior: dragStartBehavior,
      enableInteractiveSelection: enableInteractiveSelection,
      selectionControls: selectionControls,
      onTap: onTap,
      scrollPhysics: scrollPhysics,
      scrollBehavior: scrollBehavior,
      semanticsLabel: semanticsLabel,
      textHeightBehavior: textHeightBehavior,
      textWidthBasis: textWidthBasis,
      onSelectionChanged: onSelectionChanged,
      contextMenuBuilder: WoxSelectionTheme.editableTextContextMenuBuilder,
      magnifierConfiguration: magnifierConfiguration,
    );
  }

  Widget _buildRichText() {
    return SelectableText.rich(
      textSpan!,
      focusNode: focusNode,
      style: style,
      strutStyle: strutStyle,
      textAlign: textAlign,
      textDirection: textDirection,
      textScaler: textScaler,
      showCursor: showCursor,
      autofocus: autofocus,
      minLines: minLines,
      maxLines: maxLines,
      cursorWidth: cursorWidth,
      cursorHeight: cursorHeight,
      cursorRadius: cursorRadius,
      cursorColor: cursorColor,
      selectionColor: selectionColor,
      selectionHeightStyle: selectionHeightStyle,
      selectionWidthStyle: selectionWidthStyle,
      dragStartBehavior: dragStartBehavior,
      enableInteractiveSelection: enableInteractiveSelection,
      selectionControls: selectionControls,
      onTap: onTap,
      scrollPhysics: scrollPhysics,
      scrollBehavior: scrollBehavior,
      semanticsLabel: semanticsLabel,
      textHeightBehavior: textHeightBehavior,
      textWidthBasis: textWidthBasis,
      onSelectionChanged: onSelectionChanged,
      contextMenuBuilder: WoxSelectionTheme.editableTextContextMenuBuilder,
      magnifierConfiguration: magnifierConfiguration,
    );
  }
}

class WoxSelectionArea extends StatelessWidget {
  final Widget child;
  final FocusNode? focusNode;
  final TextSelectionControls? selectionControls;
  final TextMagnifierConfiguration? magnifierConfiguration;

  const WoxSelectionArea({super.key, required this.child, this.focusNode, this.selectionControls, this.magnifierConfiguration});

  @override
  Widget build(BuildContext context) {
    return WoxSelectionTheme(
      child: SelectionArea(
        focusNode: focusNode,
        selectionControls: selectionControls,
        contextMenuBuilder: WoxSelectionTheme.selectableRegionContextMenuBuilder,
        magnifierConfiguration: magnifierConfiguration,
        child: child,
      ),
    );
  }
}

class WoxSelectionTheme extends StatelessWidget {
  final Widget child;

  const WoxSelectionTheme({super.key, required this.child});

  static Widget editableTextContextMenuBuilder(BuildContext context, EditableTextState editableTextState) {
    return _buildThemedContextMenu(context, AdaptiveTextSelectionToolbar.editableText(editableTextState: editableTextState));
  }

  static Widget selectableRegionContextMenuBuilder(BuildContext context, SelectableRegionState selectableRegionState) {
    return _buildThemedContextMenu(context, AdaptiveTextSelectionToolbar.selectableRegion(selectableRegionState: selectableRegionState));
  }

  static Widget _buildThemedContextMenu(BuildContext context, Widget child) {
    final selectionColor = selectionColorOf(context);
    final selectionForegroundColor = _foregroundFor(selectionColor);
    final previewFontColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewFontColor, defaultColor: Theme.of(context).colorScheme.onSurface);
    final baseTheme = Theme.of(context);
    final materialTheme = baseTheme.copyWith(
      colorScheme: baseTheme.colorScheme.copyWith(primary: selectionColor, onPrimary: selectionForegroundColor),
      textButtonTheme: TextButtonThemeData(
        style: ButtonStyle(
          foregroundColor: WidgetStateProperty.resolveWith<Color?>((states) => states.contains(WidgetState.hovered) ? selectionForegroundColor : previewFontColor),
          overlayColor: WidgetStateProperty.resolveWith<Color?>((states) => states.contains(WidgetState.hovered) ? selectionColor : Colors.transparent),
        ),
      ),
    );

    // Bug fix: Flutter's selectable text context menu does not read
    // TextSelectionTheme for its highlighted menu row. macOS reads
    // CupertinoTheme.primaryColor, while other desktop toolbars lean on
    // Material theme colors, so this wrapper supplies both without changing the
    // platform toolbar's default labels, actions, and placement behavior.
    return Theme(
      data: materialTheme,
      child: CupertinoTheme(data: CupertinoTheme.of(context).copyWith(primaryColor: selectionColor, primaryContrastingColor: selectionForegroundColor), child: child),
    );
  }

  static Color selectionColorOf(BuildContext context) {
    return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewTextSelectionColor, defaultColor: Theme.of(context).colorScheme.primary);
  }

  static Color _foregroundFor(Color backgroundColor) {
    return backgroundColor.computeLuminance() > 0.5 ? Colors.black : Colors.white;
  }

  @override
  Widget build(BuildContext context) {
    final selectionColor = selectionColorOf(context);

    // Feature: all Wox selectable display text uses the same theme-owned
    // selection color. Previous call sites used raw SelectableText/SelectionArea
    // directly, which let Flutter defaults leak into preview and settings
    // surfaces and made the right-click menu highlight inconsistent.
    return TextSelectionTheme(data: TextSelectionThemeData(selectionColor: selectionColor), child: child);
  }
}
