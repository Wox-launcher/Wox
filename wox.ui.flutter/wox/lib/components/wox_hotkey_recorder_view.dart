import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_hotkey_recording_bus.dart';
import 'package:wox/utils/log.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_setting_controller.dart';

enum WoxHotkeyRecorderTipPosition { left, right }

class WoxHotkeyRecorder extends StatefulWidget {
  final ValueChanged<String> onHotKeyRecorded;
  final ValueChanged<String>? onUnavailableHotKeyRecorded;
  final HotkeyX? hotkey;
  final WoxHotkeyRecorderTipPosition tipPosition;
  final bool recordUnavailableHotkey;

  const WoxHotkeyRecorder({
    super.key,
    required this.onHotKeyRecorded,
    required this.hotkey,
    this.onUnavailableHotKeyRecorded,
    this.tipPosition = WoxHotkeyRecorderTipPosition.left,
    this.recordUnavailableHotkey = false,
  });

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
  String _availabilityMessage = "";
  late FocusNode _focusNode;
  StreamSubscription<String>? _globalHotkeySubscription;
  final _tracker = _HotkeyTracker();

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  void initState() {
    super.initState();

    _focusNode = FocusNode();
    _hotKey = widget.hotkey;
    _globalHotkeySubscription = WoxHotkeyRecordingBus.instance.stream.listen((hotkey) {
      if (_isFocused) {
        Logger.instance.info(const UuidV4().generate(), "Hotkey recorder received backend RecordHotkey event: hotkey=$hotkey");
        _recordHotkey(hotkey);
      } else {
        Logger.instance.debug(const UuidV4().generate(), "Hotkey recorder ignored backend RecordHotkey event because it is not focused: hotkey=$hotkey");
      }
    });
    HardwareKeyboard.instance.addHandler(_handleKeyEvent);
  }

  @override
  void dispose() {
    if (_isFocused) {
      _postHotkeyRecording(false);
    }
    _globalHotkeySubscription?.cancel();
    HardwareKeyboard.instance.removeHandler(_handleKeyEvent);
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant WoxHotkeyRecorder oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.hotkey?.toStr() != widget.hotkey?.toStr()) {
      _hotKey = widget.hotkey;
    }
  }

  // Reports recorder focus so core can forward Wox-owned global hotkey presses to this recorder instead of executing them.
  void _postHotkeyRecording(bool isRecording) {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, "Hotkey recorder posts recording state: isRecording=$isRecording");
    WoxApi.instance
        .onHotkeyRecording(traceId, isRecording)
        .then((_) {
          Logger.instance.info(traceId, "Hotkey recorder recording state accepted by core: isRecording=$isRecording");
        })
        .catchError((error) {
          Logger.instance.warn(traceId, "Failed to update hotkey recording state: $error");
        });
  }

  bool _handleKeyEvent(KeyEvent keyEvent) {
    if (_isFocused == false) return false;

    Logger.instance.info(const UuidV4().generate(), "Hotkey recorder received Flutter key event: $keyEvent");

    // backspace to clear hotkey
    if (keyEvent.logicalKey == LogicalKeyboardKey.backspace) {
      _hotKey = null;
      _availabilityMessage = "";
      widget.onHotKeyRecorded("");
      setState(() {});
      return true;
    }

    // Process the key event
    final hotkeyStr = _tracker.processKeyEvent(keyEvent);
    if (hotkeyStr == null) {
      Logger.instance.debug(const UuidV4().generate(), "Hotkey recorder did not parse a hotkey from event: $keyEvent");
      return false;
    }

    _recordHotkey(hotkeyStr);
    return true;
  }

  void _recordHotkey(String hotkeyStr) {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, "Hotkey recorder checks availability: hotkey=$hotkeyStr recordUnavailable=${widget.recordUnavailableHotkey}");
    WoxApi.instance
        .checkHotkeyAvailability(traceId, hotkeyStr)
        .then((availability) {
          Logger.instance.info(
            traceId,
            "Hotkey recorder availability result: hotkey=$hotkeyStr available=${availability.available} conflictType=${availability.conflictType} conflictValue=${availability.conflictValue}",
          );
          if (!mounted) {
            return false;
          }
          if (!availability.available) {
            _hotKey = WoxHotkey.parseHotkeyFromString(hotkeyStr);
            if (widget.recordUnavailableHotkey) {
              Logger.instance.info(traceId, "Hotkey recorder records unavailable hotkey for parent validation: hotkey=$hotkeyStr");
              _availabilityMessage = "";
              widget.onUnavailableHotKeyRecorded?.call(hotkeyStr);
            } else {
              _availabilityMessage = _buildAvailabilityMessage(availability);
              Logger.instance.warn(traceId, "Hotkey recorder rejected unavailable hotkey without parent callback: hotkey=$hotkeyStr");
            }
            setState(() {});
            return false;
          }

          _hotKey = WoxHotkey.parseHotkeyFromString(hotkeyStr);
          _availabilityMessage = "";
          widget.onHotKeyRecorded(hotkeyStr);
          setState(() {});
          return true;
        })
        .catchError((error) {
          Logger.instance.warn(traceId, "Hotkey recorder availability check failed: hotkey=$hotkeyStr error=$error");
          return false;
        });
  }

  String _buildAvailabilityMessage(HotkeyAvailability availability) {
    switch (availability.conflictType) {
      case "main":
        return tr("ui_hotkey_conflict_main");
      case "selection":
        return tr("ui_hotkey_conflict_selection");
      case "query":
        return tr("ui_hotkey_conflict_query").replaceAll("{query}", availability.conflictValue);
      case "system":
        return tr("ui_hotkey_conflict_system");
      default:
        return tr("ui_hotkey_unavailable");
    }
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
    Widget content;
    if (!_isFocused) {
      content = recorderBox;
    } else if (widget.tipPosition == WoxHotkeyRecorderTipPosition.right) {
      // Dense table-edit rows have their labels below the control area, so the recording hint stays to the right to avoid covering the row content.
      content = Row(mainAxisSize: MainAxisSize.min, children: [recorderBox, Padding(padding: const EdgeInsets.only(left: 8.0), child: _buildFocusedHint())]);
    } else {
      // General settings align the recorder itself to the right edge. The left hint is painted outside the recorder's layout box
      // so focusing the control does not push the keycaps away from their idle position.
      content = Stack(
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

    if (_availabilityMessage.isEmpty) {
      return content;
    }

    final alignAvailabilityToControlStart = widget.tipPosition == WoxHotkeyRecorderTipPosition.right;
    return Column(
      crossAxisAlignment: alignAvailabilityToControlStart ? CrossAxisAlignment.start : CrossAxisAlignment.end,
      children: [
        content,
        const SizedBox(height: 6),
        Text(_availabilityMessage, textAlign: alignAvailabilityToControlStart ? TextAlign.left : TextAlign.right, style: const TextStyle(color: Colors.red, fontSize: 12)),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Focus(
      focusNode: _focusNode,
      onFocusChange: (value) {
        Logger.instance.info(const UuidV4().generate(), "Hotkey recorder focus changed: focused=$value");
        _isFocused = value;
        if (_isFocused) {
          _tracker.reset();
        }
        _postHotkeyRecording(_isFocused);

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
