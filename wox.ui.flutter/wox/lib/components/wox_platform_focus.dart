import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Platform-specific Focus widget.
/// Currently it delegates to Flutter's default Focus behavior on every platform.
class WoxPlatformFocus extends StatefulWidget {
  final Widget child;
  final FocusNode? focusNode;
  final bool autofocus;
  final KeyEventResult Function(FocusNode, KeyEvent)? onKeyEvent;
  final void Function(bool)? onFocusChange;

  const WoxPlatformFocus({super.key, required this.child, this.focusNode, this.autofocus = false, this.onKeyEvent, this.onFocusChange});

  @override
  State<WoxPlatformFocus> createState() => _WoxPlatformFocusState();
}

class _WoxPlatformFocusState extends State<WoxPlatformFocus> {
  late FocusNode _focusNode;
  bool _isOwnFocusNode = false;

  @override
  void initState() {
    super.initState();

    if (widget.focusNode == null) {
      _focusNode = FocusNode();
      _isOwnFocusNode = true;
    } else {
      _focusNode = widget.focusNode!;
    }

    _focusNode.addListener(_onFocusChange);
  }

  void _onFocusChange() {
    widget.onFocusChange?.call(_focusNode.hasFocus);
  }

  @override
  void dispose() {
    _focusNode.removeListener(_onFocusChange);

    if (_isOwnFocusNode) {
      _focusNode.dispose();
    }

    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Focus(focusNode: _focusNode, autofocus: widget.autofocus, onKeyEvent: widget.onKeyEvent, child: widget.child);
  }
}
