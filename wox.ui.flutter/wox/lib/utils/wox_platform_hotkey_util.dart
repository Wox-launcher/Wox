import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

class WoxPlatformHotkeyUtil {
  static String get primaryModifier => Platform.isMacOS ? "cmd" : "ctrl";

  static String get primaryModifierLabel => Platform.isMacOS ? "Cmd" : "Ctrl";

  // Builds the real hotkey string used by parsers and persisted API payloads.
  static String primaryHotkey(String key) {
    final normalizedKey = key.trim();
    if (normalizedKey.isEmpty) {
      return primaryModifier;
    }
    return "$primaryModifier+$normalizedKey";
  }

  // Builds the display label for a Wox-defined primary-modifier shortcut.
  static String primaryHotkeyLabel(String key) {
    final normalizedKey = key.trim();
    if (normalizedKey.isEmpty) {
      return primaryModifierLabel;
    }
    return "$primaryModifierLabel+${_labelKeySequence(normalizedKey)}";
  }

  // Builds a Flutter shortcut activator that follows the primary modifier.
  static SingleActivator primaryActivator(LogicalKeyboardKey key, {bool shift = false, bool alt = false}) {
    return SingleActivator(key, control: !Platform.isMacOS, meta: Platform.isMacOS, shift: shift, alt: alt);
  }

  // Checks whether the platform primary modifier is currently pressed.
  static bool get isPrimaryModifierPressed => Platform.isMacOS ? HardwareKeyboard.instance.isMetaPressed : HardwareKeyboard.instance.isControlPressed;

  static String _labelKeySequence(String key) {
    return key.split("+").map(_labelKey).join("+");
  }

  static String _labelKey(String key) {
    final normalizedKey = key.trim();
    if (normalizedKey.isEmpty) {
      return normalizedKey;
    }
    switch (normalizedKey.toLowerCase()) {
      case "shift":
        return "Shift";
      case "ctrl":
      case "control":
        return "Ctrl";
      case "cmd":
      case "command":
        return "Cmd";
      case "alt":
        return "Alt";
      case "option":
        return "Option";
      case "enter":
        return "Enter";
      case "space":
        return "Space";
      default:
        return normalizedKey.length == 1 ? normalizedKey.toUpperCase() : normalizedKey;
    }
  }
}
