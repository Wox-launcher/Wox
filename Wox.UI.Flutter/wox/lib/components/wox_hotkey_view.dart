import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';

class WoxHotkeyView extends StatelessWidget {
  final HotKey hotkey;
  final Color backgroundColor;
  final Color borderColor;
  final Color textColor;

  const WoxHotkeyView({
    super.key,
    required this.hotkey,
    required this.backgroundColor,
    required this.borderColor,
    required this.textColor,
  });

  Widget buildSingleKey(String key) {
    return Container(
      constraints: BoxConstraints.tight(const Size(28, 22)),
      decoration: BoxDecoration(
        color: backgroundColor,
        border: Border.all(color: borderColor),
        borderRadius: BorderRadius.circular(4),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.1),
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Center(
        child: Text(
          key,
          style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w500,
            color: textColor,
          ),
        ),
      ),
    );
  }

  String getModifierName(HotKeyModifier modifier) {
    if (modifier == HotKeyModifier.meta) {
      return "⌘";
    } else if (modifier == HotKeyModifier.alt) {
      return "⌥";
    } else if (modifier == HotKeyModifier.control) {
      return "⌃";
    } else if (modifier == HotKeyModifier.shift) {
      return "⇧";
    }

    return modifier.name;
  }

  String getKeyName(KeyboardKey key) {
    if (key == LogicalKeyboardKey.enter) {
      return "⏎";
    } else if (key == LogicalKeyboardKey.escape) {
      return "⎋";
    } else if (key == LogicalKeyboardKey.backspace) {
      return "⌫";
    } else if (key == LogicalKeyboardKey.delete) {
      return "⌦";
    } else if (key == LogicalKeyboardKey.arrowUp) {
      return "↑";
    } else if (key == LogicalKeyboardKey.arrowDown) {
      return "↓";
    } else if (key == LogicalKeyboardKey.arrowLeft) {
      return "←";
    } else if (key == LogicalKeyboardKey.arrowRight) {
      return "→";
    } else if (key == LogicalKeyboardKey.pageUp) {
      return "⇞";
    } else if (key == LogicalKeyboardKey.pageDown) {
      return "⇟";
    } else if (key == LogicalKeyboardKey.home) {
      return "↖";
    } else if (key == LogicalKeyboardKey.end) {
      return "↘";
    } else if (key == LogicalKeyboardKey.tab) {
      return "⇥";
    } else if (key == LogicalKeyboardKey.capsLock) {
      return "⇪";
    } else if (key == LogicalKeyboardKey.insert) {
      return "⌅";
    } else if (key == LogicalKeyboardKey.numLock) {
      return "⇭";
    } else if (key == LogicalKeyboardKey.scrollLock) {
      return "⇳";
    } else if (key == LogicalKeyboardKey.pause) {
      return "⎉";
    } else if (key == LogicalKeyboardKey.printScreen) {
      return "⎙";
    } else if (key == LogicalKeyboardKey.f1) {
      return "F1";
    } else if (key == LogicalKeyboardKey.f2) {
      return "F2";
    } else if (key == LogicalKeyboardKey.f3) {
      return "F3";
    } else if (key == LogicalKeyboardKey.f4) {
      return "F4";
    } else if (key == LogicalKeyboardKey.f5) {
      return "F5";
    }

    return key.keyLabel;
  }

  @override
  Widget build(BuildContext context) {
    var hotkeyWidgets = <Widget>[];
    if (hotkey.modifiers != null) {
      hotkeyWidgets.addAll(hotkey.modifiers!.map((o) => buildSingleKey(getModifierName(o))));
    }
    hotkeyWidgets.add(buildSingleKey(getKeyName(hotkey.key)));

    return Wrap(
      spacing: 4,
      children: hotkeyWidgets,
    );
  }
}
