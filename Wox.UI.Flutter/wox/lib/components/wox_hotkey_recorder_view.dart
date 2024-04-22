import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

class WoxHotkeyRecorder extends StatefulWidget {
  final ValueChanged<String> onHotKeyRecorded;
  final HotKey? hotkey;

  const WoxHotkeyRecorder({super.key, required this.onHotKeyRecorded, required this.hotkey});

  @override
  State<WoxHotkeyRecorder> createState() => _WoxHotkeyRecorderState();
}

class _WoxHotkeyRecorderState extends State<WoxHotkeyRecorder> {
  HotKey? _hotKey;
  bool _isFocused = false;
  late FocusNode _focusNode;

  @override
  void initState() {
    super.initState();

    _focusNode = FocusNode();
    if (widget.hotkey != null) {
      _hotKey = widget.hotkey!;
    }
    HardwareKeyboard.instance.addHandler(_handleKeyEvent);
  }

  @override
  void dispose() {
    super.dispose();

    HardwareKeyboard.instance.removeHandler(_handleKeyEvent);
  }

  String getHotkeyString(HotKey hotKey) {
    var modifiers = [];
    if (hotKey.modifiers != null) {
      for (var modifier in hotKey.modifiers!) {
        if (modifier == HotKeyModifier.shift) {
          modifiers.add("shift");
        } else if (modifier == HotKeyModifier.control) {
          modifiers.add("ctrl");
        } else if (modifier == HotKeyModifier.alt) {
          if (Platform.isMacOS) {
            modifiers.add("option");
          } else {
            modifiers.add("alt");
          }
        } else if (modifier == HotKeyModifier.meta) {
          if (Platform.isMacOS) {
            modifiers.add("cmd");
          } else {
            modifiers.add("win");
          }
        }
      }
    }

    var keyStr = hotKey.key.keyLabel.toLowerCase();
    if (hotKey.key == PhysicalKeyboardKey.space) {
      keyStr = "space";
    } else if (hotKey.key == PhysicalKeyboardKey.enter) {
      keyStr = "enter";
    } else if (hotKey.key == PhysicalKeyboardKey.backspace) {
      keyStr = "backspace";
    } else if (hotKey.key == PhysicalKeyboardKey.delete) {
      keyStr = "delete";
    } else if (hotKey.key == PhysicalKeyboardKey.arrowLeft) {
      keyStr = "left";
    } else if (hotKey.key == PhysicalKeyboardKey.arrowDown) {
      keyStr = "down";
    } else if (hotKey.key == PhysicalKeyboardKey.arrowRight) {
      keyStr = "right";
    } else if (hotKey.key == PhysicalKeyboardKey.arrowUp) {
      keyStr = "up";
    }

    return "${modifiers.join("+")}+$keyStr";
  }

  bool isAllowedKey(PhysicalKeyboardKey key) {
    var allowedKeys = [
      PhysicalKeyboardKey.keyA,
      PhysicalKeyboardKey.keyB,
      PhysicalKeyboardKey.keyC,
      PhysicalKeyboardKey.keyD,
      PhysicalKeyboardKey.keyE,
      PhysicalKeyboardKey.keyF,
      PhysicalKeyboardKey.keyG,
      PhysicalKeyboardKey.keyH,
      PhysicalKeyboardKey.keyI,
      PhysicalKeyboardKey.keyJ,
      PhysicalKeyboardKey.keyK,
      PhysicalKeyboardKey.keyL,
      PhysicalKeyboardKey.keyM,
      PhysicalKeyboardKey.keyN,
      PhysicalKeyboardKey.keyO,
      PhysicalKeyboardKey.keyP,
      PhysicalKeyboardKey.keyQ,
      PhysicalKeyboardKey.keyR,
      PhysicalKeyboardKey.keyS,
      PhysicalKeyboardKey.keyT,
      PhysicalKeyboardKey.keyU,
      PhysicalKeyboardKey.keyV,
      PhysicalKeyboardKey.keyW,
      PhysicalKeyboardKey.keyX,
      PhysicalKeyboardKey.keyY,
      PhysicalKeyboardKey.keyZ,
      PhysicalKeyboardKey.digit1,
      PhysicalKeyboardKey.digit2,
      PhysicalKeyboardKey.digit3,
      PhysicalKeyboardKey.digit4,
      PhysicalKeyboardKey.digit5,
      PhysicalKeyboardKey.digit6,
      PhysicalKeyboardKey.digit7,
      PhysicalKeyboardKey.digit8,
      PhysicalKeyboardKey.digit9,
      PhysicalKeyboardKey.digit0,
      PhysicalKeyboardKey.space,
      PhysicalKeyboardKey.enter,
      PhysicalKeyboardKey.backspace,
      PhysicalKeyboardKey.delete,
      PhysicalKeyboardKey.arrowLeft,
      PhysicalKeyboardKey.arrowDown,
      PhysicalKeyboardKey.arrowRight,
      PhysicalKeyboardKey.arrowUp,
    ];

    return allowedKeys.contains(key);
  }

  bool _handleKeyEvent(KeyEvent keyEvent) {
    // Logger.instance.debug(const UuidV4().generate(), "Hotkey: ${keyEvent}");
    if (_isFocused == false) return false;
    if (keyEvent is KeyUpEvent) return false;

    // backspace to clear hotkey
    if (keyEvent.logicalKey == LogicalKeyboardKey.backspace) {
      _hotKey = null;
      widget.onHotKeyRecorded("");
      setState(() {});
      return true;
    }

    final physicalKeysPressed = HardwareKeyboard.instance.physicalKeysPressed;
    var modifiers = HotKeyModifier.values.where((e) => e.physicalKeys.any(physicalKeysPressed.contains)).toList();
    PhysicalKeyboardKey? key;
    physicalKeysPressed.removeWhere((element) => !isAllowedKey(element));
    if (physicalKeysPressed.isNotEmpty) {
      key = physicalKeysPressed.last;
    }

    if (modifiers.isEmpty || key == null) {
      return false;
    }

    var newHotkey = HotKey(key: key, modifiers: modifiers, scope: HotKeyScope.system);
    var hotkeyStr = getHotkeyString(newHotkey);
    Logger.instance.debug(const UuidV4().generate(), "Hotkey str: $hotkeyStr");
    WoxApi.instance.isHotkeyAvailable(hotkeyStr).then((isAvailable) {
      Logger.instance.debug(const UuidV4().generate(), "Hotkey available: $isAvailable");
      if (!isAvailable) {
        return false;
      }

      _hotKey = newHotkey;
      widget.onHotKeyRecorded(hotkeyStr);
      setState(() {});
      return true;
    });

    return true;
  }

  @override
  Widget build(BuildContext context) {
    return Focus(
      focusNode: _focusNode,
      onFocusChange: (value) {
        _isFocused = value;
        if (_isFocused) {
        } else {}

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
                border: Border.all(color: _isFocused ? SettingPrimaryColor : Colors.grey[600]!),
                borderRadius: BorderRadius.circular(4),
              ),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(8.0, 4.0, 8.0, 4.0),
                child: _hotKey == null
                    ? SizedBox(
                        width: 80,
                        height: 18,
                        child: Text(
                          _isFocused ? "Recording..." : "Click to set",
                          style: TextStyle(color: Colors.grey[400], fontSize: 13),
                        ),
                      )
                    : HotKeyVirtualView(hotKey: _hotKey!),
              ),
            ),
            if (_isFocused)
              Padding(
                padding: const EdgeInsets.only(left: 8.0),
                child: Text(
                  "Press any key to set hotkey",
                  style: TextStyle(color: Colors.grey[500], fontSize: 13),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
