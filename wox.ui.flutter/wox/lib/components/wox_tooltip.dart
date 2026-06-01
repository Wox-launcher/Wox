import 'dart:async';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
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
  Timer? showTimer;
  bool isHoveringTarget = false;
  late final String nativeTooltipName;
  bool nativeTooltipVisible = false;

  @override
  Widget build(BuildContext context) {
    if (widget.message.isEmpty) {
      return widget.child;
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
  }

  void scheduleShow() {
    showTimer?.cancel();
    if (widget.waitDuration == Duration.zero) {
      unawaited(showOverlay());
      return;
    }

    showTimer = Timer(widget.waitDuration, () {
      if (mounted && isHoveringTarget) {
        unawaited(showOverlay());
      }
    });
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
      final isWindowVisible = await windowManager.isVisible();
      if (!mounted || !isHoveringTarget || !isWindowVisible) {
        if (!isWindowVisible) {
          isHoveringTarget = false;
        }
        await removeOverlay();
        return;
      }

      final windowPosition = await windowManager.getPosition();
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
