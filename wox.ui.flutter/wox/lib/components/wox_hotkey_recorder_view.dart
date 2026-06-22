import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
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

/// Carries both the parsed hotkey and whether the original event should be consumed by the recorder.
class _HotkeyTrackerResult {
  final String? hotkey;
  final bool handled;

  const _HotkeyTrackerResult({this.hotkey, this.handled = false});
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
///   1. Tracking real and synthesized modifier downs separately
///   2. Ignoring synthesized modifier ups when a real down is still active
///   3. Keeping recently released synthesized modifiers briefly active because macOS can report Cmd up immediately before Space down for cmd+space
///   4. Buffering a Space key that arrives just before a synthesized Cmd down in macOS cmd+space sequences
///   5. When a non-modifier key is pressed, we check our own _pressedModifiers instead of HardwareKeyboard.instance
///
/// This approach:
///   - Works correctly even when OS intercepts key combinations
///   - Handles both normal hotkeys (cmd+space) and double-click hotkeys (cmd+cmd)
///   - Is cross-platform compatible (synthesized events occur on macOS, Linux, and potentially Windows)
class _HotkeyTracker {
  final Set<PhysicalKeyboardKey> _pressedModifiers = {};
  final Set<PhysicalKeyboardKey> _realPressedModifiers = {};
  final Set<PhysicalKeyboardKey> _synthesizedPressedModifiers = {};
  final Map<PhysicalKeyboardKey, int> _synthesizedModifierReleaseTimestamp = {};
  final Map<HotKeyModifier, int> _lastModifierTapTimestamp = {};
  final Set<HotKeyModifier> _invalidModifierTaps = {};
  bool _capsPressed = false;
  PhysicalKeyboardKey? _pendingOutOfOrderKey;
  int? _pendingOutOfOrderKeyTimestamp;
  static const int _doubleClickThreshold = 500; // milliseconds
  static const int _synthesizedModifierReleaseGrace = 120; // milliseconds
  static const int _pendingOutOfOrderKeyGrace = 120; // milliseconds

  void reset() {
    _pressedModifiers.clear();
    _realPressedModifiers.clear();
    _synthesizedPressedModifiers.clear();
    _synthesizedModifierReleaseTimestamp.clear();
    _lastModifierTapTimestamp.clear();
    _invalidModifierTaps.clear();
    _capsPressed = false;
    _clearPendingOutOfOrderKey();
  }

  bool get isCapsPressed => _capsPressed;

  /// Rebuild the active modifier snapshot from real and synthesized sources.
  void _syncPressedModifiers() {
    _pressedModifiers
      ..clear()
      ..addAll(_realPressedModifiers)
      ..addAll(_synthesizedPressedModifiers);
  }

  /// Remove synthesized modifier releases after the short reconciliation window expires.
  void _pruneExpiredSynthesizedModifiers() {
    final now = DateTime.now().millisecondsSinceEpoch;
    final expiredKeys = _synthesizedModifierReleaseTimestamp.entries.where((entry) => now - entry.value > _synthesizedModifierReleaseGrace).map((entry) => entry.key).toList();

    for (final key in expiredKeys) {
      _synthesizedModifierReleaseTimestamp.remove(key);
      if (!_realPressedModifiers.contains(key)) {
        _synthesizedPressedModifiers.remove(key);
      }
    }

    if (expiredKeys.isNotEmpty) {
      _syncPressedModifiers();
    }
  }

  void _clearPendingOutOfOrderKey() {
    _pendingOutOfOrderKey = null;
    _pendingOutOfOrderKeyTimestamp = null;
  }

  /// Returns the currently pressed modifier categories, collapsing left/right physical keys.
  Set<HotKeyModifier> _pressedModifierTypes() {
    final modifiers = <HotKeyModifier>{};
    for (final key in _pressedModifiers) {
      final modifier = WoxHotkey.convertToModifier(key);
      if (modifier != null) {
        modifiers.add(modifier);
      }
    }
    return modifiers;
  }

  String _debugPhysicalKeys(Set<PhysicalKeyboardKey> keys) {
    final labels = keys.map((key) => "${key.keyLabel}/${key.usbHidUsage}").toList()..sort();
    return "[${labels.join(",")}]";
  }

  String _debugModifierTypes(Set<HotKeyModifier> modifiers) {
    final labels = modifiers.map((modifier) => modifier.name).toList()..sort();
    return "[${labels.join(",")}]";
  }

  String debugState() {
    return "capsPressed=$_capsPressed "
        "pressed=${_debugPhysicalKeys(_pressedModifiers)} "
        "real=${_debugPhysicalKeys(_realPressedModifiers)} "
        "synth=${_debugPhysicalKeys(_synthesizedPressedModifiers)} "
        "modifierTypes=${_debugModifierTypes(_pressedModifierTypes())} "
        "invalidModifierTaps=${_debugModifierTypes(_invalidModifierTaps)} "
        "pendingOutOfOrderKey=${_pendingOutOfOrderKey == null ? "" : "${_pendingOutOfOrderKey!.keyLabel}/${_pendingOutOfOrderKey!.usbHidUsage}"}";
  }

  /// Marks any held modifiers as part of a combination and clears pending pure-tap state.
  void _invalidateActiveModifierTaps() {
    final modifiers = _pressedModifierTypes();
    _invalidModifierTaps.addAll(modifiers);
    _lastModifierTapTimestamp.clear();
  }

  /// Keep a short-lived non-modifier key only for macOS cmd+space, where Flutter can report Space before synthesized Cmd.
  bool _stagePendingOutOfOrderKey(KeyEvent keyEvent) {
    if (!Platform.isMacOS || keyEvent is! KeyDownEvent || keyEvent.physicalKey != PhysicalKeyboardKey.space) {
      return false;
    }

    _pendingOutOfOrderKey = keyEvent.physicalKey;
    _pendingOutOfOrderKeyTimestamp = DateTime.now().millisecondsSinceEpoch;
    return true;
  }

  void _pruneExpiredPendingOutOfOrderKey() {
    final timestamp = _pendingOutOfOrderKeyTimestamp;
    if (timestamp == null) {
      return;
    }

    final now = DateTime.now().millisecondsSinceEpoch;
    if (now - timestamp > _pendingOutOfOrderKeyGrace) {
      _clearPendingOutOfOrderKey();
    }
  }

  String? _consumePendingOutOfOrderHotkey(KeyEvent keyEvent) {
    _pruneExpiredPendingOutOfOrderKey();
    final pendingKey = _pendingOutOfOrderKey;
    if (pendingKey == null || !Platform.isMacOS || keyEvent is! KeyDownEvent || !keyEvent.synthesized) {
      return null;
    }

    final modifier = WoxHotkey.convertToModifier(keyEvent.physicalKey);
    if (modifier != HotKeyModifier.meta) {
      return null;
    }

    _clearPendingOutOfOrderKey();
    final hotkey = HotKey(key: pendingKey, modifiers: [HotKeyModifier.meta], scope: HotKeyScope.system);
    return WoxHotkey.normalHotkeyToStr(hotkey);
  }

  /// Process a keyboard event and report both the detected hotkey and whether the event was handled.
  bool _isCapsLockKeyEvent(KeyEvent keyEvent) {
    return keyEvent.physicalKey == PhysicalKeyboardKey.capsLock || keyEvent.logicalKey == LogicalKeyboardKey.capsLock;
  }

  _HotkeyTrackerResult processKeyEvent(KeyEvent keyEvent) {
    _pruneExpiredSynthesizedModifiers();
    _pruneExpiredPendingOutOfOrderKey();

    if (_isCapsLockKeyEvent(keyEvent)) {
      if (keyEvent is KeyDownEvent) {
        _capsPressed = true;
      } else if (keyEvent is KeyUpEvent) {
        _capsPressed = false;
      }
      return const _HotkeyTrackerResult(handled: true);
    }

    // Track modifier key states manually (more reliable than HardwareKeyboard.instance
    // which gets corrupted by synthesized events)
    if (WoxHotkey.isModifierKey(keyEvent.physicalKey)) {
      final modifier = WoxHotkey.convertToModifier(keyEvent.physicalKey);
      if (modifier == null) {
        return const _HotkeyTrackerResult();
      }

      if (keyEvent is KeyDownEvent) {
        // On Linux/Wayland, Flutter's HardwareKeyboard can deliver duplicate
        // KeyDownEvents for the same physical modifier press. If the key is
        // already tracked as pressed, skip all processing to avoid
        // false-positive combo invalidation that would prevent double-tap
        // detection. This is gated to Linux so Windows/macOS behavior is
        // unaffected.
        if (Platform.isLinux &&
            !keyEvent.synthesized &&
            _realPressedModifiers.contains(keyEvent.physicalKey)) {
          return const _HotkeyTrackerResult();
        }

        final activeModifiersBeforeDown = _pressedModifierTypes();
        if (!keyEvent.synthesized && activeModifiersBeforeDown.isNotEmpty) {
          _invalidModifierTaps
            ..addAll(activeModifiersBeforeDown)
            ..add(modifier);
        }
        _lastModifierTapTimestamp.removeWhere((tapModifier, _) => tapModifier != modifier);

        _synthesizedModifierReleaseTimestamp.remove(keyEvent.physicalKey);
        if (keyEvent.synthesized) {
          _synthesizedPressedModifiers.add(keyEvent.physicalKey);
        } else {
          _realPressedModifiers.add(keyEvent.physicalKey);
          _synthesizedPressedModifiers.remove(keyEvent.physicalKey);
        }
        _syncPressedModifiers();

        final pendingHotkey = _consumePendingOutOfOrderHotkey(keyEvent);
        if (pendingHotkey != null) {
          return _HotkeyTrackerResult(hotkey: pendingHotkey, handled: true);
        }
      } else if (keyEvent is KeyUpEvent) {
        if (keyEvent.synthesized) {
          if (!_realPressedModifiers.contains(keyEvent.physicalKey)) {
            _synthesizedModifierReleaseTimestamp[keyEvent.physicalKey] = DateTime.now().millisecondsSinceEpoch;
          }
        } else {
          final wasPressed = _realPressedModifiers.contains(keyEvent.physicalKey) || _synthesizedPressedModifiers.contains(keyEvent.physicalKey);
          if (!wasPressed) {
            return const _HotkeyTrackerResult();
          }

          _realPressedModifiers.remove(keyEvent.physicalKey);
          _synthesizedPressedModifiers.remove(keyEvent.physicalKey);
          _synthesizedModifierReleaseTimestamp.remove(keyEvent.physicalKey);
          _lastModifierTapTimestamp.removeWhere((tapModifier, _) => tapModifier != modifier);

          if (_invalidModifierTaps.remove(modifier)) {
            _lastModifierTapTimestamp.remove(modifier);
            _syncPressedModifiers();
            return const _HotkeyTrackerResult();
          }

          // Check for double-click modifier keys
          final now = DateTime.now().millisecondsSinceEpoch;
          final lastPress = _lastModifierTapTimestamp[modifier] ?? 0;

          if (now - lastPress <= _doubleClickThreshold) {
            // Double click detected
            final modifierStr = WoxHotkey.getModifierStr(modifier);
            _lastModifierTapTimestamp.remove(modifier);
            _syncPressedModifiers();
            _clearPendingOutOfOrderKey();
            return _HotkeyTrackerResult(hotkey: "$modifierStr+$modifierStr", handled: true);
          }

          _lastModifierTapTimestamp[modifier] = now;
        }
      }
      _syncPressedModifiers();
      return const _HotkeyTrackerResult();
    }

    // Flutter may synthesize non-modifier keys while reconciling focus state.
    // They are not direct user input, so they should not complete a recording.
    if (keyEvent.synthesized) {
      return const _HotkeyTrackerResult();
    }

    _invalidateActiveModifierTaps();

    // Handle normal hotkeys (modifier + key)
    if (keyEvent is! KeyUpEvent && WoxHotkey.isAllowedKey(keyEvent.physicalKey)) {
      if (_capsPressed) {
        _clearPendingOutOfOrderKey();
        return _HotkeyTrackerResult(hotkey: WoxHotkey.capsLockHotkeyToStr(keyEvent.physicalKey), handled: true);
      }

      if (!Platform.isWindows && _pressedModifiers.isEmpty) {
        return _HotkeyTrackerResult(handled: _stagePendingOutOfOrderKey(keyEvent));
      }

      final modifiers = <HotKeyModifier>[];
      // Use the recorder-local snapshot on every platform. HardwareKeyboard can
      // retain stale Windows modifier state after global-hotkey focus changes,
      // which would make the next recorded key include a modifier the user did
      // not press in this recording session.
      for (var key in _pressedModifiers) {
        final modifier = WoxHotkey.convertToModifier(key);
        if (modifier != null && !modifiers.contains(modifier)) {
          modifiers.add(modifier);
        }
      }

      if (modifiers.isEmpty) {
        return const _HotkeyTrackerResult();
      }

      _clearPendingOutOfOrderKey();
      final hotkey = HotKey(key: keyEvent.physicalKey, modifiers: modifiers, scope: HotKeyScope.system);
      return _HotkeyTrackerResult(hotkey: WoxHotkey.normalHotkeyToStr(hotkey), handled: true);
    }

    if (keyEvent is KeyUpEvent && keyEvent.physicalKey == _pendingOutOfOrderKey) {
      _clearPendingOutOfOrderKey();
    }

    return const _HotkeyTrackerResult();
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

    final traceId = const UuidV4().generate();
    Logger.instance.info(
      traceId,
      "Hotkey recorder event begin: ${_describeKeyEvent(keyEvent)} trackerBefore=${_tracker.debugState()} hardware=${_hardwareKeyboardSnapshot()} current=${_hotKey?.toStr() ?? ""}",
    );

    // backspace to clear hotkey
    if (keyEvent.logicalKey == LogicalKeyboardKey.backspace) {
      _hotKey = null;
      _availabilityMessage = "";
      widget.onHotKeyRecorded("");
      setState(() {});
      Logger.instance.info(traceId, "Hotkey recorder cleared hotkey from Backspace");
      return true;
    }

    // Process the key event
    final result = _tracker.processKeyEvent(keyEvent);
    Logger.instance.info(
      traceId,
      "Hotkey recorder event result: hotkey=${result.hotkey ?? ""} handled=${result.handled} trackerAfter=${_tracker.debugState()} hardware=${_hardwareKeyboardSnapshot()}",
    );
    if (result.hotkey == null) {
      Logger.instance.info(traceId, "Hotkey recorder did not parse a hotkey from event");
      return result.handled;
    }

    _recordHotkey(result.hotkey!);
    return true;
  }

  String _describeKeyEvent(KeyEvent keyEvent) {
    return "type=${keyEvent.runtimeType} "
        "physical=${keyEvent.physicalKey.keyLabel}/${keyEvent.physicalKey.usbHidUsage} "
        "logical=${keyEvent.logicalKey.keyLabel}/${keyEvent.logicalKey.keyId} "
        "character=${keyEvent.character ?? ""} "
        "synthesized=${keyEvent.synthesized}";
  }

  String _hardwareKeyboardSnapshot() {
    final keyboard = HardwareKeyboard.instance;
    final physicalKeys = keyboard.physicalKeysPressed.map((key) => "${key.keyLabel}/${key.usbHidUsage}").toList()..sort();
    return "control=${keyboard.isControlPressed} shift=${keyboard.isShiftPressed} alt=${keyboard.isAltPressed} meta=${keyboard.isMetaPressed} physical=[${physicalKeys.join(",")}]";
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
    final hasAvailabilityError = _availabilityMessage.isNotEmpty;
    return Container(
      // Match the quieter setting control treatment; focus still uses the accent color while idle borders no longer dominate the row.
      decoration: BoxDecoration(
        border: Border.all(color: hasAvailabilityError ? Colors.red : (_isFocused ? getThemeActiveBackgroundColor() : getThemeSubTextColor().withValues(alpha: 0.55))),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Padding(
        padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
        child:
            _hotKey == null || (!_hotKey!.isDoubleHotkey && !_hotKey!.isCapsLockHotkey && !_hotKey!.isNormalHotkey && !_hotKey!.isSingleHotkey)
                ? SizedBox(
                  width: 80,
                  height: 18,
                  child: Text(_isFocused ? tr("ui_hotkey_recording") : tr("ui_hotkey_click_to_set"), style: TextStyle(color: Colors.grey[400], fontSize: 13)),
                )
                : WoxHotkeyView(
                  // Reusing WoxHotkeyView keeps settings and toolbar shortcut
                  // labels platform-consistent.
                  hotkey: _hotKey!,
                  backgroundColor: Theme.of(context).canvasColor,
                  borderColor: Theme.of(context).dividerColor,
                  textColor: Theme.of(context).textTheme.bodyMedium?.color ?? getThemeTextColor(),
                ),
      ),
    );
  }

  Widget _buildFocusedHint({bool singleLine = false}) {
    final hasAvailabilityError = _availabilityMessage.isNotEmpty;
    return Text(
      hasAvailabilityError ? _availabilityMessage : tr("ui_hotkey_press_hint"),
      maxLines: singleLine ? 1 : null,
      softWrap: !singleLine,
      overflow: singleLine ? TextOverflow.visible : TextOverflow.clip,
      style: TextStyle(color: hasAvailabilityError ? Colors.red : Colors.grey[500], fontSize: 13),
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

    if (_availabilityMessage.isEmpty || _isFocused) {
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
      onKeyEvent: (node, event) => _handleKeyEvent(event) ? KeyEventResult.handled : KeyEventResult.ignored,
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
