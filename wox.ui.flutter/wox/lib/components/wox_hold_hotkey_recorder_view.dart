import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';

/// Maps a left/right physical modifier key to the hold-hotkey string used by
/// the Go backend (e.g. PhysicalKeyboardKey.metaLeft → "left_cmd").
String? _physicalKeyToHoldString(PhysicalKeyboardKey key) {
  if (key == PhysicalKeyboardKey.controlLeft) return 'left_ctrl';
  if (key == PhysicalKeyboardKey.controlRight) return 'right_ctrl';
  if (key == PhysicalKeyboardKey.shiftLeft) return 'left_shift';
  if (key == PhysicalKeyboardKey.shiftRight) return 'right_shift';
  if (key == PhysicalKeyboardKey.altLeft) return 'left_alt';
  if (key == PhysicalKeyboardKey.altRight) return 'right_alt';
  if (key == PhysicalKeyboardKey.metaLeft) return Platform.isMacOS ? 'left_cmd' : 'left_win';
  if (key == PhysicalKeyboardKey.metaRight) return Platform.isMacOS ? 'right_cmd' : 'right_win';
  if (key == PhysicalKeyboardKey.capsLock) return 'caps_lock';
  return null;
}

/// WoxHoldHotkeyRecorder is a specialised hotkey recorder for hold-mode
/// triggers. Unlike the normal recorder which captures modifier+key
/// combinations, this widget only accepts a single modifier key (with
/// left/right distinction) or Caps Lock. The recorded value is a string like
/// "left_cmd" that the Go backend interprets as a hold-modifier hotkey.
class WoxHoldHotkeyRecorder extends StatefulWidget {
  final String value;
  final ValueChanged<String> onRecorded;

  const WoxHoldHotkeyRecorder({super.key, required this.value, required this.onRecorded});

  @override
  State<WoxHoldHotkeyRecorder> createState() => _WoxHoldHotkeyRecorderState();
}

class _WoxHoldHotkeyRecorderState extends State<WoxHoldHotkeyRecorder> {
  bool _isFocused = false;
  late FocusNode _focusNode;

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  void initState() {
    super.initState();
    _focusNode = FocusNode();
    _focusNode.addListener(() {
      setState(() {
        _isFocused = _focusNode.hasFocus;
      });
    });
  }

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  bool _handleKeyEvent(KeyEvent keyEvent) {
    // Backspace clears the recorded key.
    if (keyEvent.logicalKey == LogicalKeyboardKey.backspace && keyEvent is KeyDownEvent) {
      widget.onRecorded('');
      setState(() {});
      return true;
    }

    if (keyEvent is! KeyDownEvent) {
      return false;
    }

    // Only accept single modifier keys or Caps Lock.
    final physicalKey = keyEvent.physicalKey;
    final holdStr = _physicalKeyToHoldString(physicalKey);
    if (holdStr == null) {
      return false;
    }

    widget.onRecorded(holdStr);
    setState(() {});
    return true;
  }

  /// Converts a hold-hotkey string to a display label, using the same
  /// modifier naming convention as the rest of the app.
  String _holdStringToLabel(String s) {
    final lower = s.toLowerCase();
    // Caps Lock is not a modifier key in the left/right sense.
    if (lower == 'caps_lock') {
      return tr('ui_hotkey_modifier_capslock');
    }

    // Parse "left_cmd" / "right_ctrl" etc. into "Left Cmd" / "Right Ctrl".
    final parts = lower.split('_');
    if (parts.length != 2) return s;

    final side = parts[0]; // "left" or "right"
    final modKey = parts[1]; // "cmd", "ctrl", "shift", "alt", "win", "super"

    String sideLabel;
    if (side == 'left') {
      sideLabel = tr('ui_hotkey_side_left');
    } else if (side == 'right') {
      sideLabel = tr('ui_hotkey_side_right');
    } else {
      return s;
    }

    String modLabel;
    switch (modKey) {
      case 'cmd':
      case 'win':
      case 'super':
        modLabel = Platform.isMacOS ? tr('ui_hotkey_modifier_cmd') : tr('ui_hotkey_modifier_win');
        break;
      case 'ctrl':
        modLabel = tr('ui_hotkey_modifier_ctrl');
        break;
      case 'shift':
        modLabel = tr('ui_hotkey_modifier_shift');
        break;
      case 'alt':
        modLabel = Platform.isMacOS ? tr('ui_hotkey_modifier_option') : tr('ui_hotkey_modifier_alt');
        break;
      default:
        return s;
    }

    return '$sideLabel $modLabel';
  }

  Widget _buildRecorderBox() {
    final hasValue = widget.value.trim().isNotEmpty;
    return Container(
      decoration: BoxDecoration(
        border: Border.all(color: _isFocused ? getThemeActiveBackgroundColor() : getThemeSubTextColor().withValues(alpha: 0.55)),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
        child: hasValue
            ? Text(
                '${tr('ui_hotkey_hold_prefix')} ${_holdStringToLabel(widget.value)}',
                style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500),
              )
            : SizedBox(
                width: 80,
                height: 18,
                child: Text(
                  _isFocused ? tr('ui_hotkey_recording') : tr('ui_hotkey_click_to_set'),
                  style: TextStyle(color: Colors.grey[400], fontSize: 13),
                ),
              ),
      ),
    );
  }

  Widget _buildFocusedHint() {
    return Text(
      tr('ui_hotkey_hold_press_hint'),
      style: TextStyle(color: Colors.grey[500], fontSize: 13),
    );
  }

  @override
  Widget build(BuildContext context) {
    final recorderBox = _buildRecorderBox();

    Widget content;
    if (!_isFocused) {
      content = recorderBox;
    } else {
      // Show the hint to the right of the recorder box, matching the normal
      // hotkey recorder's right-tip layout.
      content = Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          recorderBox,
          Padding(padding: const EdgeInsets.only(left: 8.0), child: _buildFocusedHint()),
        ],
      );
    }

    return Focus(
      focusNode: _focusNode,
      onKeyEvent: (node, event) => _handleKeyEvent(event) ? KeyEventResult.handled : KeyEventResult.ignored,
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onTapDown: (_) {
          _focusNode.requestFocus();
        },
        child: content,
      ),
    );
  }
}