import 'package:flutter/material.dart';
import 'package:wox/components/wox_drag_move_view.dart';

/// A component that adds border drag functionality around the interface
/// Allows window dragging from top, bottom, left, and right borders
class WoxBorderDragMoveArea extends StatelessWidget {
  const WoxBorderDragMoveArea({
    super.key,
    required this.child,
    this.onDragEnd,
    this.borderWidth = 5.0,
  });

  final Widget child;
  final VoidCallback? onDragEnd;
  final double borderWidth;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        // Center content
        Positioned.fill(child: child),

        // Top drag area
        Positioned(
          top: 0,
          left: 0,
          right: 0,
          height: borderWidth,
          child: WoxDragMoveArea(
            onDragEnd: onDragEnd,
            child: Container(
              color: Colors.transparent,
            ),
          ),
        ),

        // Bottom drag area
        Positioned(
          bottom: 0,
          left: 0,
          right: 0,
          height: borderWidth,
          child: WoxDragMoveArea(
            onDragEnd: onDragEnd,
            child: Container(
              color: Colors.transparent,
            ),
          ),
        ),

        // Left drag area
        Positioned(
          top: borderWidth,
          bottom: borderWidth,
          left: 0,
          width: borderWidth,
          child: WoxDragMoveArea(
            onDragEnd: onDragEnd,
            child: Container(
              color: Colors.transparent,
            ),
          ),
        ),

        // Right drag area
        Positioned(
          top: borderWidth,
          bottom: borderWidth,
          right: 0,
          width: borderWidth,
          child: WoxDragMoveArea(
            onDragEnd: onDragEnd,
            child: Container(
              color: Colors.transparent,
            ),
          ),
        ),
      ],
    );
  }
}
