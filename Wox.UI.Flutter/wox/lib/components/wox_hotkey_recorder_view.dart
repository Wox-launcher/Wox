import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:wox/utils/colors.dart';

class WoxHotkeyRecorder extends StatefulWidget {
  final ValueChanged<HotKey> onHotKeyRecorded;
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
    if (_isFocused == false) return false;
    if (keyEvent is KeyUpEvent) return false;

    final physicalKeysPressed = HardwareKeyboard.instance.physicalKeysPressed;
    PhysicalKeyboardKey? key = keyEvent.physicalKey;
    List<HotKeyModifier>? modifiers = HotKeyModifier.values.where((e) => e.physicalKeys.any(physicalKeysPressed.contains)).toList();
    if (modifiers.isNotEmpty) {
      // Remove the key from the modifiers list if it is a modifier
      modifiers = modifiers.where((e) => !e.physicalKeys.contains(key)).toList();
    }

    if (modifiers.isEmpty) {
      return false;
    }
    // ignore tab, arrow keys
    if (keyEvent.physicalKey == PhysicalKeyboardKey.tab && modifiers.isEmpty) {
      return false;
    }
    if (keyEvent.physicalKey == PhysicalKeyboardKey.tab && modifiers.length == 1 && modifiers.contains(HotKeyModifier.shift)) {
      return false;
    }
    if (keyEvent.physicalKey == PhysicalKeyboardKey.arrowLeft && modifiers.isEmpty) {
      return false;
    }
    if (keyEvent.physicalKey == PhysicalKeyboardKey.arrowDown && modifiers.isEmpty) {
      return false;
    }
    if (keyEvent.physicalKey == PhysicalKeyboardKey.arrowRight && modifiers.isEmpty) {
      return false;
    }
    if (keyEvent.physicalKey == PhysicalKeyboardKey.arrowUp && modifiers.isEmpty) {
      return false;
    }

    _hotKey = HotKey(key: keyEvent.physicalKey, modifiers: modifiers, scope: HotKeyScope.system);
    widget.onHotKeyRecorded(_hotKey!);
    setState(() {});
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
                child: widget.hotkey == null ? const Text("") : HotKeyVirtualView(hotKey: _hotKey!),
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
