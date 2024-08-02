import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';

class WoxHotkeyView extends StatelessWidget {
  final HotKey hotkey;

  const WoxHotkeyView({super.key, required this.hotkey});

  Widget buildSingleView(String key) {
    return Container(
      constraints: BoxConstraints.tight(const Size(24, 24)),
      decoration: BoxDecoration(color: Colors.grey[200], border: Border.all(color: Colors.grey[400]!), borderRadius: BorderRadius.circular(5)),
      child: Center(
        child: Text(
          key,
          style: const TextStyle(fontSize: 10),
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
      hotkeyWidgets.addAll(hotkey.modifiers!.map((o) => buildSingleView(getModifierName(o))));
    }
    hotkeyWidgets.add(buildSingleView(getKeyName(hotkey.key)));

    return Row(children: [
      for (final widget in hotkeyWidgets)
        Padding(
          padding: const EdgeInsets.only(left: 10.0),
          child: widget,
        )
    ]);
  }
}
