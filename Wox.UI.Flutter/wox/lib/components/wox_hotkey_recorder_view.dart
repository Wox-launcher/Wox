import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_hotkey.dart';
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

  bool _handleKeyEvent(KeyEvent keyEvent) {
    // Logger.instance.debug(const UuidV4().generate(), "Hotkey: ${keyEvent}");
    if (_isFocused == false) return false;

    // backspace to clear hotkey
    if (keyEvent.logicalKey == LogicalKeyboardKey.backspace) {
      _hotKey = null;
      widget.onHotKeyRecorded("");
      setState(() {});
      return true;
    }

    var newHotkey = WoxHotkey.parseHotkeyFromEvent(keyEvent);
    if (newHotkey == null) {
      return false;
    }

    var hotkeyStr = WoxHotkey.toStr(newHotkey);
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
