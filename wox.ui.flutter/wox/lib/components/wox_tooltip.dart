import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:flutter/services.dart';

class WoxTooltip extends StatefulWidget {
  final String message;
  final Widget child;

  const WoxTooltip({
    super.key,
    required this.message,
    required this.child,
  });

  @override
  State<WoxTooltip> createState() => WoxTooltipState();
}

class WoxTooltipState extends State<WoxTooltip> {
  final LayerLink layerLink = LayerLink();
  OverlayEntry? overlayEntry;
  Timer? hideTimer;
  bool isHoveringTarget = false;
  bool isHoveringTooltip = false;
  bool showAbove = false;
  double tooltipWidth = 0;
  double tooltipHeight = 0;
  double tooltipOffsetX = 0;
  double tooltipOffsetY = 6;
  double tooltipMaxWidth = 360;
  double tooltipGap = 6;
  double tooltipMargin = 8;

  @override
  Widget build(BuildContext context) {
    if (widget.message.isEmpty) {
      return widget.child;
    }

    return CompositedTransformTarget(
      link: layerLink,
      child: MouseRegion(
        onEnter: handleTargetEnter,
        onExit: handleTargetExit,
        child: widget.child,
      ),
    );
  }

  @override
  void dispose() {
    hideTimer?.cancel();
    removeOverlay();
    super.dispose();
  }

  void handleTargetEnter(PointerEnterEvent event) {
    isHoveringTarget = true;
    showOverlay();
  }

  void handleTargetExit(PointerExitEvent event) {
    isHoveringTarget = false;
    scheduleHide();
  }

  void handleTooltipEnter(PointerEnterEvent event) {
    isHoveringTooltip = true;
    showOverlay();
  }

  void handleTooltipExit(PointerExitEvent event) {
    isHoveringTooltip = false;
    scheduleHide();
  }

  void scheduleHide() {
    hideTimer?.cancel();
    hideTimer = Timer(const Duration(milliseconds: 120), maybeHide);
  }

  void maybeHide() {
    if (!isHoveringTarget && !isHoveringTooltip) {
      removeOverlay();
    }
  }

  void showOverlay() {
    updatePlacement();
    if (overlayEntry != null) {
      overlayEntry?.markNeedsBuild();
      return;
    }

    final overlay = Overlay.of(context, rootOverlay: true);
    overlayEntry = OverlayEntry(
      builder: (context) {
        final theme = Theme.of(context);
        final tooltipTheme = theme.tooltipTheme;
        final textStyle = tooltipTheme.textStyle ?? theme.textTheme.bodySmall?.copyWith(color: Colors.white) ?? const TextStyle(color: Colors.white, fontSize: 12);
        final decoration = tooltipTheme.decoration ??
            BoxDecoration(
              color: Colors.grey.shade700,
              borderRadius: BorderRadius.circular(4),
            );
        final padding = tooltipTheme.padding ?? const EdgeInsets.symmetric(horizontal: 12, vertical: 8);
        final selectionColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewTextSelectionColor);

        return Positioned(
          left: 0,
          top: 0,
          child: CompositedTransformFollower(
            link: layerLink,
            showWhenUnlinked: false,
            targetAnchor: showAbove ? Alignment.topLeft : Alignment.bottomLeft,
            followerAnchor: showAbove ? Alignment.bottomLeft : Alignment.topLeft,
            offset: Offset(tooltipOffsetX, tooltipOffsetY),
            child: Material(
              color: Colors.transparent,
              child: MouseRegion(
                onEnter: handleTooltipEnter,
                onExit: handleTooltipExit,
                child: DecoratedBox(
                  decoration: decoration,
                  child: ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: tooltipMaxWidth),
                    child: Padding(
                      padding: padding,
                      child: TextSelectionTheme(
                        data: TextSelectionThemeData(selectionColor: selectionColor),
                        child: SelectableText(widget.message, style: textStyle),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
    overlay.insert(overlayEntry!);
  }

  void updatePlacement() {
    final renderObject = context.findRenderObject();
    if (renderObject is! RenderBox || !renderObject.hasSize) {
      return;
    }

    final targetSize = renderObject.size;
    final targetPosition = renderObject.localToGlobal(Offset.zero);
    final targetRect = targetPosition & targetSize;
    final mediaSize = MediaQuery.of(context).size;
    final theme = Theme.of(context);
    final tooltipTheme = theme.tooltipTheme;
    final textStyle = tooltipTheme.textStyle ?? theme.textTheme.bodySmall?.copyWith(color: Colors.white) ?? const TextStyle(color: Colors.white, fontSize: 12);
    final padding = tooltipTheme.padding ?? const EdgeInsets.symmetric(horizontal: 12, vertical: 8);

    tooltipMaxWidth = (mediaSize.width - tooltipMargin * 2).clamp(0, 360).toDouble();
    final maxTextWidth = (tooltipMaxWidth - padding.horizontal).clamp(0, tooltipMaxWidth).toDouble();
    final textPainter = TextPainter(
      text: TextSpan(text: widget.message, style: textStyle),
      textDirection: Directionality.of(context),
    );
    textPainter.layout(maxWidth: maxTextWidth);

    tooltipWidth = (textPainter.width + padding.horizontal).clamp(0, tooltipMaxWidth);
    tooltipHeight = textPainter.height + padding.vertical;

    final spaceBelow = mediaSize.height - targetRect.bottom;
    final spaceAbove = targetRect.top;
    showAbove = spaceBelow < tooltipHeight + tooltipGap && spaceAbove > spaceBelow;
    tooltipOffsetY = showAbove ? -tooltipGap : tooltipGap;

    final rightEdge = targetRect.left + tooltipWidth;
    tooltipOffsetX = 0;
    if (rightEdge > mediaSize.width - tooltipMargin) {
      tooltipOffsetX = (mediaSize.width - tooltipMargin) - rightEdge;
    } else if (targetRect.left < tooltipMargin) {
      tooltipOffsetX = tooltipMargin - targetRect.left;
    }
  }

  void removeOverlay() {
    overlayEntry?.remove();
    overlayEntry = null;
  }
}
