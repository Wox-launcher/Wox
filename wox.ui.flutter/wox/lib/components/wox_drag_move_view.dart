import 'package:flutter/widgets.dart';
import 'package:wox/utils/windows/window_manager.dart';

class WoxDragMoveArea extends StatelessWidget {
  const WoxDragMoveArea({
    super.key,
    required this.child,
    this.onDragEnd,
  });

  final Widget child;

  /// Callback that is called when the dragging is completed
  final VoidCallback? onDragEnd;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.translucent,
      onPanStart: (details) {
        windowManager.startDragging();
      },
      onPanEnd: (details) {
        if (onDragEnd != null) {
          onDragEnd!();
        }
      },
      child: child,
    );
  }
}
