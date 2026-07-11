import 'dart:async';
import 'dart:io';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/windows/window_manager.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_text_measure_util.dart';
import 'package:wox/utils/wox_theme_util.dart';

enum WoxTooltipSide { left, top, right, bottom }

class WoxTooltip extends StatefulWidget {
  final String message;
  final Widget child;
  final Duration waitDuration;
  final WoxTooltipSide? preferSide;
  final WoxMultipleWindowHandle? windowHandle;

  const WoxTooltip({super.key, required this.message, required this.child, this.waitDuration = Duration.zero, this.preferSide, this.windowHandle});

  @override
  State<WoxTooltip> createState() => WoxTooltipState();
}

class WoxTooltipState extends State<WoxTooltip> {
  final LayerLink flutterOverlayLink = LayerLink();
  OverlayEntry? flutterOverlayEntry;
  Timer? showTimer;
  Timer? hideTimer;
  bool isHoveringTarget = false;
  bool isHoveringFlutterTooltip = false;
  late final String nativeTooltipName;
  bool nativeTooltipVisible = false;
  Alignment flutterOverlayTargetAnchor = Alignment.bottomLeft;
  Alignment flutterOverlayFollowerAnchor = Alignment.topLeft;
  double flutterOverlayWidth = 0;
  double flutterOverlayHeight = 0;
  double flutterOverlayOffsetX = 0;
  double flutterOverlayOffsetY = 6;
  double flutterOverlayMaxWidth = 360;
  double flutterOverlayGap = 6;
  double flutterOverlayMargin = 8;
  double flutterOverlayPreferredMaxWidth = 560;

  @override
  Widget build(BuildContext context) {
    if (widget.message.isEmpty) {
      return widget.child;
    }

    // Linux compositors, especially Wayland sessions, do not let Wox reliably
    // place an independent tooltip window at an absolute screen coordinate.
    // Keep Linux tooltips inside Flutter's layer tree so the position follows
    // the hovered widget instead of depending on native overlay placement.
    if (Platform.isLinux) {
      return CompositedTransformTarget(link: flutterOverlayLink, child: MouseRegion(onEnter: handleTargetEnter, onExit: handleTargetExit, child: widget.child));
    }

    return MouseRegion(onEnter: handleTargetEnter, onExit: handleTargetExit, child: widget.child);
  }

  @override
  void initState() {
    super.initState();
    nativeTooltipName = 'wox_tooltip_${const UuidV4().generate()}';
  }

  @override
  void didUpdateWidget(covariant WoxTooltip oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (Platform.isLinux) {
      if ((oldWidget.message != widget.message || oldWidget.preferSide != widget.preferSide) && flutterOverlayEntry != null) {
        if (widget.message.isEmpty || (!isHoveringTarget && !isHoveringFlutterTooltip)) {
          removeFlutterOverlay();
        } else {
          showFlutterOverlay();
        }
      }
      return;
    }

    if ((oldWidget.message != widget.message || oldWidget.preferSide != widget.preferSide) && nativeTooltipVisible) {
      if (widget.message.isEmpty || !isHoveringTarget) {
        unawaited(removeOverlay());
      } else {
        unawaited(showOverlay());
      }
    }
  }

  @override
  void dispose() {
    showTimer?.cancel();
    hideTimer?.cancel();
    removeFlutterOverlay();
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
    if (Platform.isLinux) {
      scheduleFlutterHide();
    }
  }

  void handleFlutterTooltipEnter(PointerEnterEvent event) {
    isHoveringFlutterTooltip = true;
    showFlutterOverlay();
  }

  void handleFlutterTooltipExit(PointerExitEvent event) {
    isHoveringFlutterTooltip = false;
    scheduleFlutterHide();
  }

  void scheduleFlutterHide() {
    hideTimer?.cancel();
    hideTimer = Timer(const Duration(milliseconds: 120), maybeHideFlutterOverlay);
  }

  void scheduleShow() {
    hideTimer?.cancel();
    showTimer?.cancel();
    if (widget.waitDuration == Duration.zero) {
      if (Platform.isLinux) {
        showFlutterOverlay();
      } else {
        unawaited(showOverlay());
      }
      return;
    }

    showTimer = Timer(widget.waitDuration, () {
      if (mounted && isHoveringTarget) {
        if (Platform.isLinux) {
          showFlutterOverlay();
        } else {
          unawaited(showOverlay());
        }
      }
    });
  }

  void maybeHideFlutterOverlay() {
    if (!isHoveringTarget && !isHoveringFlutterTooltip) {
      removeFlutterOverlay();
    }
  }

  // This is the Linux fallback for the native tooltip overlay. The native path
  // needs a separate top-level window, but Linux window managers can ignore or
  // reinterpret the requested position, so the tooltip can drift away from its
  // target. A Flutter overlay stays within the launcher window and uses local
  // composited coordinates, which are under Flutter's control.
  void showFlutterOverlay() {
    updateFlutterOverlayPlacement();
    if (flutterOverlayEntry != null) {
      flutterOverlayEntry?.markNeedsBuild();
      return;
    }

    final overlay = Overlay.of(context, rootOverlay: true);
    flutterOverlayEntry = OverlayEntry(
      builder: (context) {
        final textStyle = resolveFlutterTooltipTextStyle(context);
        final decoration = resolveFlutterTooltipDecoration();
        final padding = resolveFlutterTooltipPadding();

        return Positioned(
          left: 0,
          top: 0,
          child: CompositedTransformFollower(
            link: flutterOverlayLink,
            showWhenUnlinked: false,
            targetAnchor: flutterOverlayTargetAnchor,
            followerAnchor: flutterOverlayFollowerAnchor,
            offset: Offset(flutterOverlayOffsetX, flutterOverlayOffsetY),
            child: Material(
              color: Colors.transparent,
              child: MouseRegion(
                onEnter: handleFlutterTooltipEnter,
                onExit: handleFlutterTooltipExit,
                child: DecoratedBox(
                  decoration: decoration,
                  child: ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: flutterOverlayMaxWidth),
                    child: Padding(padding: padding, child: WoxSelectableText(widget.message, style: textStyle)),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
    overlay.insert(flutterOverlayEntry!);
  }

  // Recomputes the in-window tooltip placement and clamps it to the current Flutter viewport.
  void updateFlutterOverlayPlacement() {
    final renderObject = context.findRenderObject();
    if (renderObject is! RenderBox || !renderObject.hasSize) {
      return;
    }

    final targetSize = renderObject.size;
    final targetPosition = renderObject.localToGlobal(Offset.zero);
    final targetRect = targetPosition & targetSize;
    final mediaSize = MediaQuery.of(context).size;
    final textStyle = resolveFlutterTooltipTextStyle(context);
    final padding = resolveFlutterTooltipPadding();

    flutterOverlayMaxWidth = (mediaSize.width - flutterOverlayMargin * 2).clamp(0, flutterOverlayPreferredMaxWidth).toDouble();
    final maxTextWidth = (flutterOverlayMaxWidth - padding.horizontal).clamp(0, flutterOverlayMaxWidth).toDouble();
    final textSize = WoxTextMeasureUtil.measureTextSize(context: context, text: widget.message, style: textStyle, maxWidth: maxTextWidth);

    flutterOverlayWidth = (textSize.width + padding.horizontal).clamp(0, flutterOverlayMaxWidth);
    flutterOverlayHeight = textSize.height + padding.vertical;

    if (widget.preferSide == WoxTooltipSide.left) {
      updateFlutterHorizontalPlacement(targetRect, mediaSize, showOnLeft: true);
      return;
    }

    if (widget.preferSide == WoxTooltipSide.right) {
      updateFlutterHorizontalPlacement(targetRect, mediaSize, showOnLeft: false);
      return;
    }

    if (widget.preferSide == WoxTooltipSide.top) {
      updateFlutterVerticalPlacement(targetRect, mediaSize, showAbove: true);
      return;
    }

    if (widget.preferSide == WoxTooltipSide.bottom) {
      updateFlutterVerticalPlacement(targetRect, mediaSize, showAbove: false);
      return;
    }

    final spaceBelow = mediaSize.height - targetRect.bottom;
    final spaceAbove = targetRect.top;
    final showAbove = spaceBelow < flutterOverlayHeight + flutterOverlayGap && spaceAbove > spaceBelow;
    updateFlutterVerticalPlacement(targetRect, mediaSize, showAbove: showAbove);
  }

  // Top/bottom placement keeps auto mode's layout rules while also letting callers pin a side.
  void updateFlutterVerticalPlacement(Rect targetRect, Size mediaSize, {required bool showAbove}) {
    flutterOverlayTargetAnchor = showAbove ? Alignment.topLeft : Alignment.bottomLeft;
    flutterOverlayFollowerAnchor = showAbove ? Alignment.bottomLeft : Alignment.topLeft;

    final baseTop = showAbove ? targetRect.top - flutterOverlayHeight : targetRect.bottom;
    final preferredTop = baseTop + (showAbove ? -flutterOverlayGap : flutterOverlayGap);
    final minTop = flutterOverlayMargin;
    final maxTop = mediaSize.height - flutterOverlayMargin - flutterOverlayHeight;
    final clampedTop = maxTop < minTop ? minTop : preferredTop.clamp(minTop, maxTop).toDouble();
    flutterOverlayOffsetY = clampedTop - baseTop;

    final baseLeft = targetRect.left;
    final maxLeft = mediaSize.width - flutterOverlayMargin - flutterOverlayWidth;
    final clampedLeft = maxLeft < flutterOverlayMargin ? flutterOverlayMargin : baseLeft.clamp(flutterOverlayMargin, maxLeft).toDouble();
    flutterOverlayOffsetX = clampedLeft - baseLeft;
  }

  // Left/right placement is used by dense controls near the launcher edge.
  void updateFlutterHorizontalPlacement(Rect targetRect, Size mediaSize, {required bool showOnLeft}) {
    final targetCenterY = targetRect.top + targetRect.height / 2;
    final baseTop = targetCenterY - flutterOverlayHeight / 2;
    final minTop = flutterOverlayMargin;
    final maxTop = mediaSize.height - flutterOverlayMargin - flutterOverlayHeight;
    final clampedTop = maxTop < minTop ? minTop : baseTop.clamp(minTop, maxTop).toDouble();

    flutterOverlayTargetAnchor = showOnLeft ? Alignment.centerLeft : Alignment.centerRight;
    flutterOverlayFollowerAnchor = showOnLeft ? Alignment.centerRight : Alignment.centerLeft;
    flutterOverlayOffsetY = clampedTop - baseTop;

    final baseLeft = showOnLeft ? targetRect.left - flutterOverlayWidth : targetRect.right;
    final preferredLeft = baseLeft + (showOnLeft ? -flutterOverlayGap : flutterOverlayGap);
    final maxLeft = mediaSize.width - flutterOverlayMargin - flutterOverlayWidth;
    final clampedLeft = maxLeft < flutterOverlayMargin ? flutterOverlayMargin : preferredLeft.clamp(flutterOverlayMargin, maxLeft).toDouble();
    flutterOverlayOffsetX = clampedLeft - baseLeft;
  }

  void removeFlutterOverlay() {
    flutterOverlayEntry?.remove();
    flutterOverlayEntry = null;
  }

  EdgeInsets resolveFlutterTooltipPadding() {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(11), vertical: metrics.scaledSpacing(8));
  }

  TextStyle resolveFlutterTooltipTextStyle(BuildContext context) {
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    final fallbackTextColor = safeFromCssColor(woxTheme.resultItemTitleColor, defaultColor: Colors.white);
    final textColor = safeFromCssColor(woxTheme.previewFontColor, defaultColor: fallbackTextColor);
    final metrics = WoxInterfaceSizeUtil.instance.current;

    return (Theme.of(context).textTheme.bodySmall ?? const TextStyle()).copyWith(
      color: textColor.withValues(alpha: 0.96),
      fontSize: metrics.resultSubtitleFontSize,
      fontWeight: FontWeight.w600,
      height: 1.28,
      letterSpacing: 0,
    );
  }

  BoxDecoration resolveFlutterTooltipDecoration() {
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    final baseBackground = safeFromCssColor(woxTheme.appBackgroundColor, defaultColor: const Color(0xFF20242D));
    final panelBackground = safeFromCssColor(woxTheme.actionContainerBackgroundColor, defaultColor: getThemeCardBackgroundColor());
    final accentColor = safeFromCssColor(woxTheme.queryBoxCursorColor, defaultColor: getThemeActiveBackgroundColor());
    final dividerColor = safeFromCssColor(woxTheme.previewSplitLineColor, defaultColor: safeFromCssColor(woxTheme.resultItemSubTitleColor, defaultColor: Colors.white24));
    final isDarkTheme = baseBackground.computeLuminance() < 0.5;
    final mixedSurface = Color.lerp(baseBackground, panelBackground, 0.78) ?? panelBackground;
    final liftedSurface = isDarkTheme ? mixedSurface.lighter(5) : mixedSurface.darker(2);

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

  Future<void> showOverlay() async {
    if (!mounted || widget.message.isEmpty || !isHoveringTarget) {
      return;
    }

    final renderObject = context.findRenderObject();
    if (renderObject is! RenderBox || !renderObject.hasSize) {
      return;
    }

    final targetSize = renderObject.size;
    final targetPosition = renderObject.localToGlobal(Offset.zero);
    final targetRect = targetPosition & targetSize;

    try {
      final secondaryWindow = widget.windowHandle ?? WoxMultipleWindowScope.maybeHandleOf(context);
      final isWindowVisible = secondaryWindow == null ? await windowManager.isVisible() : WoxMultipleWindow.isOpen(secondaryWindow.id);
      if (!mounted || !isHoveringTarget || !isWindowVisible) {
        if (!isWindowVisible) {
          isHoveringTarget = false;
        }
        await removeOverlay();
        return;
      }

      final windowPosition = secondaryWindow == null ? await windowManager.getPosition() : await secondaryWindow.getPosition();
      if (!mounted || !isHoveringTarget) {
        await removeOverlay();
        return;
      }

      final traceId = const UuidV4().generate();
      await WoxApi.instance.showTooltipOverlay(
        traceId,
        nativeTooltipName,
        widget.message,
        widget.preferSide?.name ?? "",
        windowPosition.dx + targetRect.left,
        windowPosition.dy + targetRect.top,
        targetRect.width,
        targetRect.height,
      );
      nativeTooltipVisible = true;
    } catch (_) {
      nativeTooltipVisible = false;
    }
  }

  Future<void> removeOverlay() async {
    if (nativeTooltipVisible) {
      nativeTooltipVisible = false;
      try {
        await WoxApi.instance.hideTooltipOverlay(const UuidV4().generate(), nativeTooltipName);
      } catch (_) {}
    }
  }
}
