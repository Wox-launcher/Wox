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
  final HotkeyX? hotkey;

  const WoxHotkeyRecorder({super.key, required this.onHotKeyRecorded, required this.hotkey});

  @override
  State<WoxHotkeyRecorder> createState() => _WoxHotkeyRecorderState();
}

class _WoxHotkeyRecorderState extends State<WoxHotkeyRecorder> {
  HotkeyX? _hotKey;
  bool _isFocused = false;
  late FocusNode _focusNode;
  final Map<LogicalKeyboardKey, int> _lastKeyUpTimestamp = {};
  static const int _doubleClickThreshold = 500; // milliseconds

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

    Logger.instance.debug(const UuidV4().generate(), "Hotkey: ${keyEvent}");

    // backspace to clear hotkey
    if (keyEvent.logicalKey == LogicalKeyboardKey.backspace) {
      _hotKey = null;
      widget.onHotKeyRecorded("");
      setState(() {});
      return true;
    }

    // Handle double click modifier keys
    if (keyEvent is KeyUpEvent && WoxHotkey.isModifierKey(keyEvent.physicalKey)) {
      final now = DateTime.now().millisecondsSinceEpoch;
      final lastPress = _lastKeyUpTimestamp[keyEvent.logicalKey] ?? 0;

      if (now - lastPress <= _doubleClickThreshold) {
        // Double click detected
        final modifierStr = WoxHotkey.getModifierStr(WoxHotkey.convertToModifier(keyEvent.physicalKey)!);
        final hotkeyStr = "$modifierStr+$modifierStr";
        WoxApi.instance.isHotkeyAvailable(hotkeyStr).then((isAvailable) {
          Logger.instance.debug(const UuidV4().generate(), "Double click hotkey available: $isAvailable");
          if (!isAvailable) {
            return false;
          }

          _hotKey = HotkeyX(hotkeyStr, doubleHotkey: WoxHotkey.convertToModifier(keyEvent.physicalKey));
          widget.onHotKeyRecorded(hotkeyStr);
          setState(() {});
          return true;
        });
        _lastKeyUpTimestamp.remove(keyEvent.logicalKey);
        return true;
      }

      _lastKeyUpTimestamp[keyEvent.logicalKey] = now;
      return true;
    }

    // Handle normal hotkeys
    var newHotkey = WoxHotkey.parseNormalHotkeyFromEvent(keyEvent);
    if (newHotkey == null) {
      return false;
    }

    var hotkeyStr = WoxHotkey.normalHotkeyToStr(newHotkey);
    Logger.instance.debug(const UuidV4().generate(), "Hotkey str: $hotkeyStr");
    WoxApi.instance.isHotkeyAvailable(hotkeyStr).then((isAvailable) {
      Logger.instance.debug(const UuidV4().generate(), "Hotkey available: $isAvailable");
      if (!isAvailable) {
        return false;
      }

      _hotKey = HotkeyX(hotkeyStr, normalHotkey: newHotkey);
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
            color: Colors.black.withOpacity(0.3),
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
          _lastKeyUpTimestamp.clear();
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
                child: _hotKey == null
                    ? SizedBox(
                        width: 80,
                        height: 18,
                        child: Text(
                          _isFocused ? "Recording..." : "Click to set",
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
                  "Press any key to set hotkey, or double click modifier keys",
                  style: TextStyle(color: Colors.grey[500], fontSize: 13),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
