import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_setting_controller.dart';

enum WoxHotkeyRecorderTipPosition { left, right }

class WoxHotkeyRecorder extends StatefulWidget {
  final ValueChanged<String> onHotKeyRecorded;
  final HotkeyX? hotkey;
  final WoxHotkeyRecorderTipPosition tipPosition;

  const WoxHotkeyRecorder({super.key, required this.onHotKeyRecorded, required this.hotkey, this.tipPosition = WoxHotkeyRecorderTipPosition.left});

  @override
  State<WoxHotkeyRecorder> createState() => _WoxHotkeyRecorderState();
}

/// Internal class to track hotkey state and handle keyboard events.
///
/// Why we need this instead of using WoxHotkey.parseNormalHotkeyFromEvent:
///
/// Problem:
/// When the OS intercepts certain key combinations (e.g., cmd+space on macOS for input method switching),
/// the event sequence becomes corrupted:
///   1. User presses: Cmd DOWN
///   2. User presses: Space DOWN
///   3. OS intercepts cmd+space and triggers input method switching
///   4. Flutter receives: Cmd DOWN → Cmd UP (synthesized) → Space DOWN → Cmd DOWN (synthesized) → Space UP → Cmd UP
///
/// The synthesized events cause two issues:
///   1. HardwareKeyboard.instance state becomes unreliable (it thinks Cmd is released when it's actually still pressed)
///   2. WoxHotkey.parseNormalHotkeyFromEvent relies on HardwareKeyboard.instance, so it returns null when Space DOWN arrives
///
/// Solution:
/// This tracker manually maintains modifier key states by:
///   1. Ignoring all synthesized events (they're Flutter's "guesses", not real user input)
///   2. Tracking modifier keys ourselves in _pressedModifiers Set
///   3. When a non-modifier key is pressed, we check our own _pressedModifiers instead of HardwareKeyboard.instance
///
/// This approach:
///   - Works correctly even when OS intercepts key combinations
///   - Handles both normal hotkeys (cmd+space) and double-click hotkeys (cmd+cmd)
///   - Is cross-platform compatible (synthesized events occur on macOS, Linux, and potentially Windows)
class _HotkeyTracker {
  final Set<PhysicalKeyboardKey> _pressedModifiers = {};
  final Map<LogicalKeyboardKey, int> _lastKeyUpTimestamp = {};
  static const int _doubleClickThreshold = 500; // milliseconds

  void reset() {
    _pressedModifiers.clear();
    _lastKeyUpTimestamp.clear();
  }

  /// Process a keyboard event and return the detected hotkey string, or null if no hotkey detected
  String? processKeyEvent(KeyEvent keyEvent) {
    // Ignore synthesized events from Flutter
    // Flutter synthesizes events when the OS intercepts certain key combinations
    // (e.g., cmd+space on macOS, which is reserved for input method switching)
    if (keyEvent.synthesized) {
      return null;
    }

    // Track modifier key states manually (more reliable than HardwareKeyboard.instance
    // which gets corrupted by synthesized events)
    if (WoxHotkey.isModifierKey(keyEvent.physicalKey)) {
      if (keyEvent is KeyDownEvent) {
        _pressedModifiers.add(keyEvent.physicalKey);
      } else if (keyEvent is KeyUpEvent) {
        _pressedModifiers.remove(keyEvent.physicalKey);

        // Check for double-click modifier keys
        final now = DateTime.now().millisecondsSinceEpoch;
        final lastPress = _lastKeyUpTimestamp[keyEvent.logicalKey] ?? 0;

        if (now - lastPress <= _doubleClickThreshold) {
          // Double click detected
          final modifierStr = WoxHotkey.getModifierStr(WoxHotkey.convertToModifier(keyEvent.physicalKey)!);
          return "$modifierStr+$modifierStr";
        }

        _lastKeyUpTimestamp[keyEvent.logicalKey] = now;
      }
      return null;
    }

    // Handle normal hotkeys (modifier + key)
    if (keyEvent is! KeyUpEvent && WoxHotkey.isAllowedKey(keyEvent.physicalKey)) {
      if (!Platform.isWindows && _pressedModifiers.isEmpty) {
        return null;
      }

      // On Windows, rely on native modifier states from message loop.
      // Win key events may not always be delivered as normal Flutter key events.
      final modifiers = <HotKeyModifier>[];
      if (Platform.isWindows) {
        modifiers.addAll(WoxHotkey.getPressedModifiers());
      } else {
        // On other platforms, use tracker-maintained modifier states.
        for (var key in _pressedModifiers) {
          final modifier = WoxHotkey.convertToModifier(key);
          if (modifier != null && !modifiers.contains(modifier)) {
            modifiers.add(modifier);
          }
        }
      }

      if (modifiers.isEmpty) {
        return null;
      }

      final hotkey = HotKey(key: keyEvent.physicalKey, modifiers: modifiers, scope: HotKeyScope.system);
      return WoxHotkey.normalHotkeyToStr(hotkey);
    }

    return null;
  }
}

class _WoxHotkeyRecorderState extends State<WoxHotkeyRecorder> {
  HotkeyX? _hotKey;
  bool _isFocused = false;
  late FocusNode _focusNode;
  final _tracker = _HotkeyTracker();

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  void initState() {
    super.initState();

    _focusNode = FocusNode();
    _hotKey = widget.hotkey;
    HardwareKeyboard.instance.addHandler(_handleKeyEvent);
  }

  @override
  void dispose() {
    super.dispose();

    HardwareKeyboard.instance.removeHandler(_handleKeyEvent);
  }

  bool _handleKeyEvent(KeyEvent keyEvent) {
    if (_isFocused == false) return false;

    Logger.instance.debug(const UuidV4().generate(), "Hotkey: $keyEvent");

    // backspace to clear hotkey
    if (keyEvent.logicalKey == LogicalKeyboardKey.backspace) {
      _hotKey = null;
      widget.onHotKeyRecorded("");
      setState(() {});
      return true;
    }

    // Process the key event
    final hotkeyStr = _tracker.processKeyEvent(keyEvent);
    if (hotkeyStr == null) {
      return false;
    }

    // Check if hotkey is available and update state
    Logger.instance.debug(const UuidV4().generate(), "Hotkey str: $hotkeyStr");
    WoxApi.instance.isHotkeyAvailable(const UuidV4().generate(), hotkeyStr).then((isAvailable) {
      Logger.instance.debug(const UuidV4().generate(), "Hotkey available: $isAvailable");
      if (!isAvailable) {
        return false;
      }

      _hotKey = WoxHotkey.parseHotkeyFromString(hotkeyStr);
      widget.onHotKeyRecorded(hotkeyStr);
      setState(() {});
      return true;
    });

    return true;
  }

  Widget _buildRecorderBox() {
    return Container(
      // Match the quieter setting control treatment; focus still uses the accent color while idle borders no longer dominate the row.
      decoration: BoxDecoration(
        border: Border.all(color: _isFocused ? getThemeActiveBackgroundColor() : getThemeSubTextColor().withValues(alpha: 0.55)),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
        child:
            _hotKey == null || (!_hotKey!.isDoubleHotkey && !_hotKey!.isNormalHotkey && !_hotKey!.isSingleHotkey)
                ? SizedBox(
                  width: 80,
                  height: 18,
                  child: Text(_isFocused ? tr("ui_hotkey_recording") : tr("ui_hotkey_click_to_set"), style: TextStyle(color: Colors.grey[400], fontSize: 13)),
                )
                : WoxHotkeyView(
                  // Feature fix: the recorder preview used hotkey_manager's
                  // raw key labels, which still rendered Apple-style modifier
                  // glyphs on non-macOS. Reusing WoxHotkeyView keeps settings
                  // and toolbar shortcut labels platform-consistent.
                  hotkey: _hotKey!,
                  backgroundColor: Theme.of(context).canvasColor,
                  borderColor: Theme.of(context).dividerColor,
                  textColor: Theme.of(context).textTheme.bodyMedium?.color ?? getThemeTextColor(),
                ),
      ),
    );
  }

  Widget _buildFocusedHint({bool singleLine = false}) {
    return Text(
      tr("ui_hotkey_press_hint"),
      maxLines: singleLine ? 1 : null,
      softWrap: !singleLine,
      overflow: singleLine ? TextOverflow.visible : TextOverflow.clip,
      style: TextStyle(color: Colors.grey[500], fontSize: 13),
    );
  }

  Widget _buildRecorderContent() {
    final recorderBox = _buildRecorderBox();
    if (!_isFocused) {
      return recorderBox;
    }

    if (widget.tipPosition == WoxHotkeyRecorderTipPosition.right) {
      // Dense table-edit rows have their labels below the control area, so the recording hint stays to the right to avoid covering the row content.
      return Row(mainAxisSize: MainAxisSize.min, children: [recorderBox, Padding(padding: const EdgeInsets.only(left: 8.0), child: _buildFocusedHint())]);
    }

    // General settings align the recorder itself to the right edge. The left hint is painted outside the recorder's layout box
    // so focusing the control does not push the keycaps away from their idle position.
    return Stack(
      clipBehavior: Clip.none,
      children: [
        recorderBox,
        Positioned.fill(
          child: OverflowBox(
            maxWidth: double.infinity,
            alignment: Alignment.centerLeft,
            child: FractionalTranslation(
              translation: const Offset(-1, 0),
              child: Padding(padding: const EdgeInsets.only(right: 8.0), child: Center(heightFactor: 1, child: _buildFocusedHint(singleLine: true))),
            ),
          ),
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Focus(
      focusNode: _focusNode,
      onFocusChange: (value) {
        _isFocused = value;
        if (_isFocused) {
          _tracker.reset();
        }

        setState(() {});
      },
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onTapDown: (_) {
          _focusNode.requestFocus();
        },
        child: _buildRecorderContent(),
      ),
    );
  }
}
