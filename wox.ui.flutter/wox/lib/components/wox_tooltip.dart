import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_text_measure_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/windows/window_manager.dart';

enum WoxTooltipSide { left, top, right, bottom }

class WoxTooltip extends StatefulWidget {
  final String message;
  final Widget child;
  final Duration waitDuration;
  final WoxTooltipSide? preferSide;

  const WoxTooltip({super.key, required this.message, required this.child, this.waitDuration = Duration.zero, this.preferSide});

  @override
  State<WoxTooltip> createState() => WoxTooltipState();
}

class WoxTooltipState extends State<WoxTooltip> {
  final LayerLink layerLink = LayerLink();
  OverlayEntry? overlayEntry;
  Timer? showTimer;
  Timer? hideTimer;
  bool isHoveringTarget = false;
  bool isHoveringTooltip = false;
  Alignment targetAnchor = Alignment.bottomLeft;
  Alignment followerAnchor = Alignment.topLeft;
  double tooltipWidth = 0;
  double tooltipHeight = 0;
  double tooltipLeft = 0;
  double tooltipTop = 0;
  double tooltipOffsetX = 0;
  double tooltipOffsetY = 6;
  double tooltipMaxWidth = 360;
  double tooltipGap = 6;
  double tooltipMargin = 8;
  double tooltipPreferredMaxWidth = 560;
  double targetLeft = 0;
  double targetTop = 0;
  double targetWidth = 0;
  double targetHeight = 0;
  late final String nativeTooltipName;
  bool nativeTooltipVisible = false;
  bool nativeTooltipSupported = true;

  @override
  Widget build(BuildContext context) {
    if (widget.message.isEmpty) {
      return widget.child;
    }

    return CompositedTransformTarget(link: layerLink, child: MouseRegion(onEnter: handleTargetEnter, onExit: handleTargetExit, child: widget.child));
  }

  @override
  void initState() {
    super.initState();
    nativeTooltipName = 'wox_tooltip_${const UuidV4().generate()}';
  }

  @override
  void didUpdateWidget(covariant WoxTooltip oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.message != widget.message && nativeTooltipVisible) {
      unawaited(showOverlay());
    }
  }

  @override
  void dispose() {
    showTimer?.cancel();
    hideTimer?.cancel();
    unawaited(removeOverlay());
    super.dispose();
  }

  void handleTargetEnter(PointerEnterEvent event) {
    isHoveringTarget = true;
    scheduleShow();
  }

  void handleTargetExit(PointerExitEvent event) {
    isHoveringTarget = false;
    showTimer?.cancel();
    if (nativeTooltipSupported) {
      return;
    }
    scheduleHide();
  }

  void handleTooltipEnter(PointerEnterEvent event) {
    if (nativeTooltipSupported) {
      return;
    }
    isHoveringTooltip = true;
    unawaited(showOverlay());
  }

  void handleTooltipExit(PointerExitEvent event) {
    if (nativeTooltipSupported) {
      return;
    }
    isHoveringTooltip = false;
    scheduleHide();
  }

  void scheduleHide() {
    hideTimer?.cancel();
    hideTimer = Timer(const Duration(milliseconds: 120), maybeHide);
  }

  void scheduleShow() {
    hideTimer?.cancel();
    showTimer?.cancel();
    if (widget.waitDuration == Duration.zero) {
      unawaited(showOverlay());
      return;
    }

    // Material Tooltip used delayed hover in a few dense controls. WoxTooltip
    // owns the delay now so migrated call sites keep their interaction timing
    // while sharing one selectable, boundary-aware overlay implementation.
    showTimer = Timer(widget.waitDuration, () {
      if (mounted && isHoveringTarget) {
        unawaited(showOverlay());
      }
    });
  }

  void maybeHide() {
    if (!isHoveringTarget && !isHoveringTooltip) {
      unawaited(removeOverlay());
    }
  }

  Future<void> showOverlay() async {
    updatePlacement();
    if (nativeTooltipSupported && await showNativeOverlay()) {
      return;
    }

    showFallbackOverlay();
  }

  Future<bool> showNativeOverlay() async {
    if (!mounted || widget.message.isEmpty) {
      return false;
    }

    try {
      final windowPosition = await windowManager.getPosition();
      await WoxApi.instance.showTooltipOverlay(
        const UuidV4().generate(),
        nativeTooltipName,
        widget.message,
        windowPosition.dx + tooltipLeft,
        windowPosition.dy + tooltipTop,
        windowPosition.dx + targetLeft,
        windowPosition.dy + targetTop,
        targetWidth,
        targetHeight,
      );
      nativeTooltipVisible = true;
      removeFallbackOverlay();
      return true;
    } catch (_) {
      nativeTooltipSupported = false;
      nativeTooltipVisible = false;
      return false;
    }
  }

  void showFallbackOverlay() {
    if (overlayEntry != null) {
      overlayEntry?.markNeedsBuild();
      return;
    }

    final overlay = Overlay.of(context, rootOverlay: true);
    overlayEntry = OverlayEntry(
      builder: (context) {
        final textStyle = resolveTooltipTextStyle(context);
        final decoration = resolveTooltipDecoration();
        final padding = resolveTooltipPadding();

        return Positioned(
          left: 0,
          top: 0,
          child: CompositedTransformFollower(
            link: layerLink,
            showWhenUnlinked: false,
            targetAnchor: targetAnchor,
            followerAnchor: followerAnchor,
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
                    child: Padding(padding: padding, child: WoxSelectableText(widget.message, style: textStyle)),
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
    targetLeft = targetRect.left;
    targetTop = targetRect.top;
    targetWidth = targetRect.width;
    targetHeight = targetRect.height;
    final mediaSize = MediaQuery.of(context).size;
    final textStyle = resolveTooltipTextStyle(context);
    final padding = resolveTooltipPadding();

    tooltipMaxWidth = (mediaSize.width - tooltipMargin * 2).clamp(0, tooltipPreferredMaxWidth).toDouble();
    final maxTextWidth = (tooltipMaxWidth - padding.horizontal).clamp(0, tooltipMaxWidth).toDouble();
    final textSize = WoxTextMeasureUtil.measureTextSize(context: context, text: widget.message, style: textStyle, maxWidth: maxTextWidth);

    tooltipWidth = (textSize.width + padding.horizontal).clamp(0, tooltipMaxWidth);
    tooltipHeight = textSize.height + padding.vertical;

    if (widget.preferSide == WoxTooltipSide.left) {
      updateHorizontalPlacement(targetRect, mediaSize, showOnLeft: true);
      return;
    }

    if (widget.preferSide == WoxTooltipSide.right) {
      updateHorizontalPlacement(targetRect, mediaSize, showOnLeft: false);
      return;
    }

    if (widget.preferSide == WoxTooltipSide.top) {
      updateVerticalPlacement(targetRect, mediaSize, showAbove: true);
      return;
    }

    if (widget.preferSide == WoxTooltipSide.bottom) {
      updateVerticalPlacement(targetRect, mediaSize, showAbove: false);
      return;
    }

    final spaceBelow = mediaSize.height - targetRect.bottom;
    final spaceAbove = targetRect.top;
    final showAbove = spaceBelow < tooltipHeight + tooltipGap && spaceAbove > spaceBelow;
    updateVerticalPlacement(targetRect, mediaSize, showAbove: showAbove);
  }

  // Top/bottom placement keeps auto mode's layout rules while also letting
  // callers pin a specific side when nearby chrome would clip the overlay.
  void updateVerticalPlacement(Rect targetRect, Size mediaSize, {required bool showAbove}) {
    targetAnchor = showAbove ? Alignment.topLeft : Alignment.bottomLeft;
    followerAnchor = showAbove ? Alignment.bottomLeft : Alignment.topLeft;

    final baseTop = showAbove ? targetRect.top - tooltipHeight : targetRect.bottom;
    final preferredTop = baseTop + (showAbove ? -tooltipGap : tooltipGap);
    final minTop = tooltipMargin;
    final maxTop = mediaSize.height - tooltipMargin - tooltipHeight;
    final clampedTop = maxTop < minTop ? minTop : preferredTop.clamp(minTop, maxTop).toDouble();
    tooltipOffsetY = clampedTop - baseTop;
    tooltipTop = clampedTop;

    final baseLeft = targetRect.left;
    final maxLeft = mediaSize.width - tooltipMargin - tooltipWidth;
    final clampedLeft = maxLeft < tooltipMargin ? tooltipMargin : baseLeft.clamp(tooltipMargin, maxLeft).toDouble();
    tooltipOffsetX = clampedLeft - baseLeft;
    tooltipLeft = clampedLeft;
  }

  // Left/right placement is used by query-box accessory pills that sit against
  // the top-right launcher edge and should not open downward into clipped UI.
  void updateHorizontalPlacement(Rect targetRect, Size mediaSize, {required bool showOnLeft}) {
    final targetCenterY = targetRect.top + targetRect.height / 2;
    final baseTop = targetCenterY - tooltipHeight / 2;
    final minTop = tooltipMargin;
    final maxTop = mediaSize.height - tooltipMargin - tooltipHeight;
    final clampedTop = maxTop < minTop ? minTop : baseTop.clamp(minTop, maxTop).toDouble();

    targetAnchor = showOnLeft ? Alignment.centerLeft : Alignment.centerRight;
    followerAnchor = showOnLeft ? Alignment.centerRight : Alignment.centerLeft;
    tooltipOffsetY = clampedTop - baseTop;
    tooltipTop = clampedTop;

    final baseLeft = showOnLeft ? targetRect.left - tooltipWidth : targetRect.right;
    final preferredLeft = baseLeft + (showOnLeft ? -tooltipGap : tooltipGap);
    final maxLeft = mediaSize.width - tooltipMargin - tooltipWidth;
    final clampedLeft = maxLeft < tooltipMargin ? tooltipMargin : preferredLeft.clamp(tooltipMargin, maxLeft).toDouble();
    tooltipOffsetX = clampedLeft - baseLeft;
    tooltipLeft = clampedLeft;
  }

  Future<void> removeOverlay() async {
    if (nativeTooltipVisible) {
      nativeTooltipVisible = false;
      try {
        await WoxApi.instance.hideTooltipOverlay(const UuidV4().generate(), nativeTooltipName);
      } catch (_) {
        nativeTooltipSupported = false;
      }
    }

    removeFallbackOverlay();
  }

  void removeFallbackOverlay() {
    overlayEntry?.remove();
    overlayEntry = null;
  }

  EdgeInsets resolveTooltipPadding() {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(11), vertical: metrics.scaledSpacing(8));
  }

  TextStyle resolveTooltipTextStyle(BuildContext context) {
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    final fallbackTextColor = safeFromCssColor(woxTheme.resultItemTitleColor, defaultColor: Colors.white);
    final textColor = safeFromCssColor(woxTheme.previewFontColor, defaultColor: fallbackTextColor);
    final metrics = WoxInterfaceSizeUtil.instance.current;

    // Tooltip text should follow Wox density and font family, not Material's
    // default tooltip size. This keeps launcher, settings, and screenshot
    // hover text aligned after all call sites moved to WoxTooltip.
    return (Theme.of(context).textTheme.bodySmall ?? const TextStyle()).copyWith(
      color: textColor.withValues(alpha: 0.96),
      fontSize: metrics.resultSubtitleFontSize,
      fontWeight: FontWeight.w600,
      height: 1.28,
      letterSpacing: 0,
    );
  }

  BoxDecoration resolveTooltipDecoration() {
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    final baseBackground = safeFromCssColor(woxTheme.appBackgroundColor, defaultColor: const Color(0xFF20242D));
    final panelBackground = safeFromCssColor(woxTheme.actionContainerBackgroundColor, defaultColor: getThemeCardBackgroundColor());
    final accentColor = safeFromCssColor(woxTheme.queryBoxCursorColor, defaultColor: getThemeActiveBackgroundColor());
    final dividerColor = safeFromCssColor(woxTheme.previewSplitLineColor, defaultColor: safeFromCssColor(woxTheme.resultItemSubTitleColor, defaultColor: Colors.white24));
    final isDarkTheme = baseBackground.computeLuminance() < 0.5;
    final mixedSurface = Color.lerp(baseBackground, panelBackground, 0.78) ?? panelBackground;
    final liftedSurface = isDarkTheme ? mixedSurface.lighter(5) : mixedSurface.darker(2);

    // The old Material fallback produced a flat neutral gray that ignored Wox
    // themes. Blend the theme surface with a small amount of the active accent
    // so tooltips feel connected to the current launcher without becoming a
    // selected-result chip.
    final surfaceColor = Color.alphaBlend(accentColor.withValues(alpha: isDarkTheme ? 0.08 : 0.04), liftedSurface);
    final borderColor = Color.lerp(dividerColor, accentColor, isDarkTheme ? 0.24 : 0.18)!.withValues(alpha: isDarkTheme ? 0.62 : 0.42);

    return BoxDecoration(
      color: surfaceColor,
      borderRadius: BorderRadius.circular(8),
      border: Border.all(color: borderColor),
      boxShadow: [
        BoxShadow(color: Colors.black.withValues(alpha: isDarkTheme ? 0.30 : 0.14), blurRadius: 18, offset: const Offset(0, 8)),
        BoxShadow(color: accentColor.withValues(alpha: isDarkTheme ? 0.06 : 0.04), blurRadius: 1),
      ],
    );
  }
}
