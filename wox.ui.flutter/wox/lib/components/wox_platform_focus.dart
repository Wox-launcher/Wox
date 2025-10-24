import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/windows_window_manager.dart';
import 'package:uuid/v4.dart';

/// Platform-specific Focus widget
/// On Windows, uses custom keyboard event handling from message loop
/// On other platforms, uses default Flutter Focus widget
class WoxPlatformFocus extends StatefulWidget {
  final Widget child;
  final FocusNode? focusNode;
  final bool autofocus;
  final KeyEventResult Function(FocusNode, KeyEvent)? onKeyEvent;
  final void Function(bool)? onFocusChange;

  const WoxPlatformFocus({
    super.key,
    required this.child,
    this.focusNode,
    this.autofocus = false,
    this.onKeyEvent,
    this.onFocusChange,
  });

  @override
  State<WoxPlatformFocus> createState() => _WoxPlatformFocusState();
}

class _WoxPlatformFocusState extends State<WoxPlatformFocus> {
  late FocusNode _focusNode;
  bool _isOwnFocusNode = false;

  // Windows-specific: track key states
  final Set<int> _pressedKeys = {};

  @override
  void initState() {
    super.initState();

    if (widget.focusNode == null) {
      _focusNode = FocusNode();
      _isOwnFocusNode = true;
    } else {
      _focusNode = widget.focusNode!;
    }

    // On Windows, register keyboard event listener
    if (Platform.isWindows) {
      WindowsWindowManager.instance.addKeyboardEventListener(_handleWindowsKeyboardEvent);
    }

    // Listen to focus changes to clear pressed keys when focus is lost
    _focusNode.addListener(_onFocusChange);
  }

  void _onFocusChange() {
    if (!_focusNode.hasFocus) {
      // Clear pressed keys when focus is lost
      _pressedKeys.clear();
    }

    // Call user's onFocusChange callback if provided
    widget.onFocusChange?.call(_focusNode.hasFocus);
  }

  @override
  void dispose() {
    // Remove focus listener
    _focusNode.removeListener(_onFocusChange);

    // On Windows, unregister keyboard event listener
    if (Platform.isWindows) {
      WindowsWindowManager.instance.removeKeyboardEventListener(_handleWindowsKeyboardEvent);
    }

    if (_isOwnFocusNode) {
      _focusNode.dispose();
    }
    super.dispose();
  }

  /// Handle keyboard events from Windows message loop
  void _handleWindowsKeyboardEvent(String eventType, int keyCode, int scanCode, WindowsModifierKeyStates modifierStates) {
    if (!_focusNode.hasFocus) {
      return; // Only handle events when focused
    }

    final traceId = const UuidV4().generate();

    // Convert Windows virtual key code to LogicalKeyboardKey
    final logicalKey = _getLogicalKeyFromVirtualKey(keyCode);
    if (logicalKey == null) {
      Logger.instance.debug(traceId, "[KEYLOG][PLATFORM-FOCUS] Unknown key code: $keyCode");
      return;
    }

    // Create KeyEvent based on event type
    // Get physical key from logical key (more reliable than scan code)
    final physicalKey = _getPhysicalKeyFromLogicalKey(logicalKey);

    // Skip if we couldn't map to a valid physical key
    if (physicalKey.usbHidUsage == 0x00000000) {
      Logger.instance.debug(traceId, "[KEYLOG][PLATFORM-FOCUS] Skipping unmapped key: ${logicalKey.keyLabel}");
      return;
    }

    KeyEvent? keyEvent;
    if (eventType == 'keydown') {
      // Check if this is a repeat
      if (_pressedKeys.contains(keyCode)) {
        keyEvent = KeyRepeatEvent(
          physicalKey: physicalKey,
          logicalKey: logicalKey,
          timeStamp: Duration.zero,
        );
      } else {
        _pressedKeys.add(keyCode);
        keyEvent = KeyDownEvent(
          physicalKey: physicalKey,
          logicalKey: logicalKey,
          timeStamp: Duration.zero,
        );
      }
    } else if (eventType == 'keyup') {
      _pressedKeys.remove(keyCode);
      keyEvent = KeyUpEvent(
        physicalKey: physicalKey,
        logicalKey: logicalKey,
        timeStamp: Duration.zero,
      );
    }

    if (keyEvent != null && widget.onKeyEvent != null) {
      String eventTypeStr = keyEvent is KeyDownEvent
          ? "DOWN"
          : keyEvent is KeyUpEvent
              ? "UP"
              : keyEvent is KeyRepeatEvent
                  ? "REPEAT"
                  : "UNKNOWN";
      Logger.instance.debug(traceId, "[KEYLOG][PLATFORM-FOCUS] KeyEvent: ${logicalKey.keyLabel} ($eventTypeStr) from Windows msgloop");

      // Call user's onKeyEvent callback
      // Note: We don't call HardwareKeyboard.instance.handleKeyEvent because
      // we're using WindowsWindowManager.currentModifierStates for modifier key tracking
      widget.onKeyEvent!(_focusNode, keyEvent);
    }
  }

  /// Get PhysicalKeyboardKey from LogicalKeyboardKey
  PhysicalKeyboardKey _getPhysicalKeyFromLogicalKey(LogicalKeyboardKey logicalKey) {
    // Map common logical keys to physical keys
    // For letters A-Z
    if (logicalKey.keyId >= LogicalKeyboardKey.keyA.keyId && logicalKey.keyId <= LogicalKeyboardKey.keyZ.keyId) {
      final offset = logicalKey.keyId - LogicalKeyboardKey.keyA.keyId;
      return PhysicalKeyboardKey(0x00070004 + offset); // USB HID usage codes for A-Z
    }

    // For digits 0-9
    if (logicalKey.keyId >= LogicalKeyboardKey.digit0.keyId && logicalKey.keyId <= LogicalKeyboardKey.digit9.keyId) {
      if (logicalKey == LogicalKeyboardKey.digit0) {
        return PhysicalKeyboardKey.digit0;
      }
      final offset = logicalKey.keyId - LogicalKeyboardKey.digit1.keyId;
      return PhysicalKeyboardKey(0x0007001e + offset); // USB HID usage codes for 1-9
    }

    // Common keys
    switch (logicalKey) {
      case LogicalKeyboardKey.enter:
        return PhysicalKeyboardKey.enter;
      case LogicalKeyboardKey.escape:
        return PhysicalKeyboardKey.escape;
      case LogicalKeyboardKey.backspace:
        return PhysicalKeyboardKey.backspace;
      case LogicalKeyboardKey.tab:
        return PhysicalKeyboardKey.tab;
      case LogicalKeyboardKey.space:
        return PhysicalKeyboardKey.space;
      case LogicalKeyboardKey.delete:
        return PhysicalKeyboardKey.delete;
      case LogicalKeyboardKey.arrowLeft:
        return PhysicalKeyboardKey.arrowLeft;
      case LogicalKeyboardKey.arrowRight:
        return PhysicalKeyboardKey.arrowRight;
      case LogicalKeyboardKey.arrowUp:
        return PhysicalKeyboardKey.arrowUp;
      case LogicalKeyboardKey.arrowDown:
        return PhysicalKeyboardKey.arrowDown;

      // Modifier keys
      case LogicalKeyboardKey.shift:
      case LogicalKeyboardKey.shiftLeft:
        return PhysicalKeyboardKey.shiftLeft;
      case LogicalKeyboardKey.shiftRight:
        return PhysicalKeyboardKey.shiftRight;
      case LogicalKeyboardKey.control:
      case LogicalKeyboardKey.controlLeft:
        return PhysicalKeyboardKey.controlLeft;
      case LogicalKeyboardKey.controlRight:
        return PhysicalKeyboardKey.controlRight;
      case LogicalKeyboardKey.alt:
      case LogicalKeyboardKey.altLeft:
        return PhysicalKeyboardKey.altLeft;
      case LogicalKeyboardKey.altRight:
        return PhysicalKeyboardKey.altRight;
      case LogicalKeyboardKey.meta:
      case LogicalKeyboardKey.metaLeft:
        return PhysicalKeyboardKey.metaLeft;
      case LogicalKeyboardKey.metaRight:
        return PhysicalKeyboardKey.metaRight;

      default:
        // Return a generic physical key if we can't map it
        return const PhysicalKeyboardKey(0x00000000);
    }
  }

  /// Convert Windows virtual key code to LogicalKeyboardKey
  LogicalKeyboardKey? _getLogicalKeyFromVirtualKey(int vk) {
    // Common keys mapping
    switch (vk) {
      case 0x0D:
        return LogicalKeyboardKey.enter;
      case 0x1B:
        return LogicalKeyboardKey.escape;
      case 0x08:
        return LogicalKeyboardKey.backspace;
      case 0x09:
        return LogicalKeyboardKey.tab;
      case 0x20:
        return LogicalKeyboardKey.space;
      case 0x25:
        return LogicalKeyboardKey.arrowLeft;
      case 0x26:
        return LogicalKeyboardKey.arrowUp;
      case 0x27:
        return LogicalKeyboardKey.arrowRight;
      case 0x28:
        return LogicalKeyboardKey.arrowDown;
      case 0x2E:
        return LogicalKeyboardKey.delete;
      case 0x24:
        return LogicalKeyboardKey.home;
      case 0x23:
        return LogicalKeyboardKey.end;
      case 0x21:
        return LogicalKeyboardKey.pageUp;
      case 0x22:
        return LogicalKeyboardKey.pageDown;

      // Number keys (0-9)
      case 0x30:
        return LogicalKeyboardKey.digit0;
      case 0x31:
        return LogicalKeyboardKey.digit1;
      case 0x32:
        return LogicalKeyboardKey.digit2;
      case 0x33:
        return LogicalKeyboardKey.digit3;
      case 0x34:
        return LogicalKeyboardKey.digit4;
      case 0x35:
        return LogicalKeyboardKey.digit5;
      case 0x36:
        return LogicalKeyboardKey.digit6;
      case 0x37:
        return LogicalKeyboardKey.digit7;
      case 0x38:
        return LogicalKeyboardKey.digit8;
      case 0x39:
        return LogicalKeyboardKey.digit9;

      // Letter keys (A-Z)
      case 0x41:
        return LogicalKeyboardKey.keyA;
      case 0x42:
        return LogicalKeyboardKey.keyB;
      case 0x43:
        return LogicalKeyboardKey.keyC;
      case 0x44:
        return LogicalKeyboardKey.keyD;
      case 0x45:
        return LogicalKeyboardKey.keyE;
      case 0x46:
        return LogicalKeyboardKey.keyF;
      case 0x47:
        return LogicalKeyboardKey.keyG;
      case 0x48:
        return LogicalKeyboardKey.keyH;
      case 0x49:
        return LogicalKeyboardKey.keyI;
      case 0x4A:
        return LogicalKeyboardKey.keyJ;
      case 0x4B:
        return LogicalKeyboardKey.keyK;
      case 0x4C:
        return LogicalKeyboardKey.keyL;
      case 0x4D:
        return LogicalKeyboardKey.keyM;
      case 0x4E:
        return LogicalKeyboardKey.keyN;
      case 0x4F:
        return LogicalKeyboardKey.keyO;
      case 0x50:
        return LogicalKeyboardKey.keyP;
      case 0x51:
        return LogicalKeyboardKey.keyQ;
      case 0x52:
        return LogicalKeyboardKey.keyR;
      case 0x53:
        return LogicalKeyboardKey.keyS;
      case 0x54:
        return LogicalKeyboardKey.keyT;
      case 0x55:
        return LogicalKeyboardKey.keyU;
      case 0x56:
        return LogicalKeyboardKey.keyV;
      case 0x57:
        return LogicalKeyboardKey.keyW;
      case 0x58:
        return LogicalKeyboardKey.keyX;
      case 0x59:
        return LogicalKeyboardKey.keyY;
      case 0x5A:
        return LogicalKeyboardKey.keyZ;

      // Function keys (F1-F12)
      case 0x70:
        return LogicalKeyboardKey.f1;
      case 0x71:
        return LogicalKeyboardKey.f2;
      case 0x72:
        return LogicalKeyboardKey.f3;
      case 0x73:
        return LogicalKeyboardKey.f4;
      case 0x74:
        return LogicalKeyboardKey.f5;
      case 0x75:
        return LogicalKeyboardKey.f6;
      case 0x76:
        return LogicalKeyboardKey.f7;
      case 0x77:
        return LogicalKeyboardKey.f8;
      case 0x78:
        return LogicalKeyboardKey.f9;
      case 0x79:
        return LogicalKeyboardKey.f10;
      case 0x7A:
        return LogicalKeyboardKey.f11;
      case 0x7B:
        return LogicalKeyboardKey.f12;

      // Modifier keys
      case 0x10:
        return LogicalKeyboardKey.shift;
      case 0x11:
        return LogicalKeyboardKey.control;
      case 0x12:
        return LogicalKeyboardKey.alt;

      default:
        return null;
    }
  }

  @override
  Widget build(BuildContext context) {
    // On Windows, we handle keyboard events ourselves via Windows message loop
    // Don't pass onKeyEvent to Focus to avoid duplicate event handling
    if (Platform.isWindows) {
      return Focus(
        focusNode: _focusNode,
        autofocus: widget.autofocus,
        // Don't pass onKeyEvent - we handle it in _handleWindowsKeyboardEvent
        child: widget.child,
      );
    }

    // On other platforms, use default Focus widget
    return Focus(
      focusNode: _focusNode,
      autofocus: widget.autofocus,
      onKeyEvent: widget.onKeyEvent,
      child: widget.child,
    );
  }
}
