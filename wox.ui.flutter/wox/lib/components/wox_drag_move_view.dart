import 'dart:async';
import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_drag_move_state.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/window_manager.dart';

class WoxDragMoveArea extends StatelessWidget {
  const WoxDragMoveArea({super.key, required this.child, this.onDragEnd, this.debugSource = "unknown"});

  final Widget child;

  /// Callback that is called when the dragging is completed
  final VoidCallback? onDragEnd;
  final String debugSource;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.translucent,
      onPanStart: (details) {
        final traceId = const UuidV4().generate();
        if (Platform.isLinux) {
          WoxDragMoveState.begin(traceId, debugSource);
          Logger.instance.info(
            traceId,
            "linux-window-drag dart stage=pan-start source=$debugSource local=${details.localPosition.dx},${details.localPosition.dy} global=${details.globalPosition.dx},${details.globalPosition.dy}",
          );
        }
        unawaited(windowManager.startDragging(traceId: traceId, source: debugSource));
      },
      onPanEnd: (_) {
        _finishDrag("pan-end");
        if (onDragEnd != null) {
          onDragEnd!();
        }
      },
      onPanCancel: () {
        _finishDrag("pan-cancel");
      },
      child: child,
    );
  }

  /// Logs and clears the Linux drag marker when Flutter sees the pan finish.
  void _finishDrag(String reason) {
    if (!Platform.isLinux) {
      return;
    }

    final traceId = WoxDragMoveState.activeTraceId ?? const UuidV4().generate();
    Logger.instance.info(traceId, "linux-window-drag dart stage=$reason source=$debugSource");
    WoxDragMoveState.end();
  }
}
