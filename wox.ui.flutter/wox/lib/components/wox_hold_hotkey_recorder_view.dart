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

/// Parses a hold-hotkey string back to a display label.
String _holdStringToLabel(String s) {
  final lower = s.toLowerCase();
  const map = {
    'left_ctrl': 'Left Ctrl',
    'right_ctrl': 'Right Ctrl',
    'left_shift': 'Left Shift',
    'right_shift': 'Right Shift',
    'left_alt': 'Left Alt',
    'right_alt': 'Right Alt',
    'left_cmd': 'Left Cmd',
    'right_cmd': 'Right Cmd',
    'left_win': 'Left Win',
    'right_win': 'Right Win',
    'left_super': 'Left Super',
    'right_super': 'Right Super',
    'caps_lock': 'Caps Lock',
  };
  return map[lower] ?? s;
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

  @override
  Widget build(BuildContext context) {
    final hasValue = widget.value.trim().isNotEmpty;
    final accentColor = getThemeActiveBackgroundColor();

    return Focus(
      focusNode: _focusNode,
      onKeyEvent: (node, event) => _handleKeyEvent(event) ? KeyEventResult.handled : KeyEventResult.ignored,
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onTapDown: (_) {
          _focusNode.requestFocus();
        },
        child: Container(
          decoration: BoxDecoration(
            border: Border.all(color: _isFocused ? accentColor : getThemeSubTextColor().withValues(alpha: 0.55)),
            borderRadius: BorderRadius.circular(4),
          ),
          padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
          child: hasValue
              ? _buildKeyDisplay(accentColor)
              : SizedBox(
                  width: 120,
                  height: 18,
                  child: Text(
                    _isFocused ? tr('ui_hotkey_recording') : tr('ui_hotkey_click_to_set'),
                    style: TextStyle(color: Colors.grey[400], fontSize: 13),
                  ),
                ),
        ),
      ),
    );
  }

  Widget _buildKeyDisplay(Color accentColor) {
    final label = _holdStringToLabel(widget.value);

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          widget.value.toLowerCase().contains('caps') ? Icons.keyboard_capslock : Icons.touch_app,
          size: 16,
          color: accentColor,
        ),
        const SizedBox(width: 6),
        Text(
          label,
          style: TextStyle(
            color: getThemeTextColor(),
            fontSize: 13,
            fontWeight: FontWeight.w500,
          ),
        ),
        const SizedBox(width: 8),
        if (_isFocused)
          Text(
            tr('plugin_dictation_hotkey_hold_hint'),
            style: TextStyle(color: Colors.grey[500], fontSize: 11),
          ),
      ],
    );
  }
}