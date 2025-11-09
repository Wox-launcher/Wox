import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_setting_controller.dart';

class WoxHotkeyRecorder extends StatefulWidget {
  final ValueChanged<String> onHotKeyRecorded;
  final HotkeyX? hotkey;

  const WoxHotkeyRecorder({super.key, required this.onHotKeyRecorded, required this.hotkey});

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
      if (_pressedModifiers.isEmpty) {
        return null;
      }

      // Convert pressed modifiers to HotKeyModifier list
      List<HotKeyModifier> modifiers = [];
      for (var key in _pressedModifiers) {
        final modifier = WoxHotkey.convertToModifier(key);
        if (modifier != null && !modifiers.contains(modifier)) {
          modifiers.add(modifier);
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
    WoxApi.instance.isHotkeyAvailable(hotkeyStr).then((isAvailable) {
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

  Widget buildSingleKeyView(String keyLabel) {
    return Container(
      padding: const EdgeInsets.only(left: 5, right: 5, top: 3, bottom: 3),
      decoration: BoxDecoration(
        color: Theme.of(context).canvasColor,
        border: Border.all(
          color: Theme.of(context).dividerColor,
          width: 1,
        ),
        borderRadius: BorderRadius.circular(3),
        boxShadow: <BoxShadow>[
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.3),
            offset: const Offset(0.0, 1.0),
          ),
        ],
      ),
      child: Text(
        keyLabel,
        style: TextStyle(
          color: Theme.of(context).textTheme.bodyMedium?.color,
          fontSize: 12,
        ),
      ),
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
        child: Row(
          children: [
            Container(
              decoration: BoxDecoration(
                border: Border.all(color: _isFocused ? getThemeActiveBackgroundColor() : getThemeSubTextColor()),
                borderRadius: BorderRadius.circular(4),
              ),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
                child: _hotKey == null || (!_hotKey!.isDoubleHotkey && !_hotKey!.isNormalHotkey && !_hotKey!.isSingleHotkey)
                    ? SizedBox(
                        width: 80,
                        height: 18,
                        child: Text(
                          _isFocused ? tr("ui_hotkey_recording") : tr("ui_hotkey_click_to_set"),
                          style: TextStyle(color: Colors.grey[400], fontSize: 13),
                        ),
                      )
                    : _hotKey!.isDoubleHotkey
                        ? Wrap(
                            spacing: 8,
                            children: [
                              buildSingleKeyView(WoxHotkey.getModifierStr(_hotKey!.doubleHotkey!)),
                              buildSingleKeyView(WoxHotkey.getModifierStr(_hotKey!.doubleHotkey!)),
                            ],
                          )
                        : HotKeyVirtualView(hotKey: _hotKey!.normalHotkey!),
              ),
            ),
            if (_isFocused)
              Padding(
                padding: const EdgeInsets.only(left: 8.0),
                child: Text(
                  tr("ui_hotkey_press_hint"),
                  style: TextStyle(color: Colors.grey[500], fontSize: 13),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
