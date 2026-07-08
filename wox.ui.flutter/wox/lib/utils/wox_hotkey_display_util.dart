import 'dart:io';

import 'package:flutter/services.dart';
import 'package:wox/entity/wox_hotkey.dart';

class WoxHotkeyDisplayUtil {
  static String modifierLabel(HotKeyModifier modifier) {
    // Feature fix: modifier chips now prefer short text over platform glyphs.
    // The old macOS-style symbols were compact, but they made Windows/Linux
    // shortcuts harder to scan and were inconsistent with real key legends.
    return switch (modifier) {
      HotKeyModifier.meta =>
        Platform.isMacOS
            ? "Cmd"
            : Platform.isWindows
            ? "Win"
            : "Super",
      HotKeyModifier.alt => Platform.isMacOS ? "Option" : "Alt",
      HotKeyModifier.control => "Ctrl",
      HotKeyModifier.shift => "Shift",
      _ => modifier.name,
    };
  }

  static String keyLabel(KeyboardKey key) {
    if (key == LogicalKeyboardKey.space || key == PhysicalKeyboardKey.space) {
      return "Space";
    } else if (key == LogicalKeyboardKey.enter) {
      return "⏎";
    } else if (key == LogicalKeyboardKey.arrowUp) {
      return "↑";
    } else if (key == LogicalKeyboardKey.arrowDown) {
      return "↓";
    } else if (key == LogicalKeyboardKey.arrowLeft) {
      return "←";
    } else if (key == LogicalKeyboardKey.arrowRight) {
      return "→";
    } else if (key == LogicalKeyboardKey.pageUp) {
      return "PageUp";
    } else if (key == LogicalKeyboardKey.pageDown) {
      return "PageDown";
    } else if (key == LogicalKeyboardKey.home) {
      return "Home";
    } else if (key == LogicalKeyboardKey.end) {
      return "End";
    } else if (key == LogicalKeyboardKey.capsLock) {
      return "CapsLock";
    } else if (key == LogicalKeyboardKey.insert) {
      return "Insert";
    } else if (key == LogicalKeyboardKey.numLock) {
      return "NumLock";
    } else if (key == LogicalKeyboardKey.scrollLock) {
      return "ScrollLock";
    } else if (key == LogicalKeyboardKey.pause) {
      return "Pause";
    } else if (key == LogicalKeyboardKey.printScreen) {
      return "PrintScreen";
    } else if (key == LogicalKeyboardKey.backquote || key == PhysicalKeyboardKey.backquote) {
      return "~";
    }

    final label = key.keyLabel;
    return label.length <= 3 ? label : _shortTextKeyLabel(label);
  }

  static String _shortTextKeyLabel(String keyLabel) {
    final normalized = keyLabel.trim().toLowerCase().replaceAll(" ", "");
    return switch (normalized) {
      "escape" || "esc" => "Esc",
      "backspace" => "Bsp",
      "delete" || "del" => "Del",
      "tab" => "Tab",
      "space" => "Space",
      "pageup" => "PageUp",
      "pagedown" => "PageDown",
      "home" => "Home",
      "end" => "End",
      "insert" || "ins" => "Insert",
      "capslock" => "CapsLock",
      "numlock" => "NumLock",
      "scrolllock" => "ScrollLock",
      "pause" => "Pause",
      "printscreen" => "PrintScreen",
      _ => normalized.length <= 3 ? keyLabel : "${normalized.substring(0, 1).toUpperCase()}${normalized.substring(1, 3)}",
    };
  }

  static List<String> labelsFromHotkey(HotkeyX hotkey) {
    if (hotkey.isNormalHotkey) {
      return [for (final modifier in hotkey.normalHotkey!.modifiers ?? <HotKeyModifier>[]) modifierLabel(modifier), keyLabel(hotkey.normalHotkey!.key)];
    }

    if (hotkey.isCapsLockHotkey) {
      return ["CapsLock", keyLabel(hotkey.capsLockHotkey!)];
    }

    if (hotkey.isDoubleHotkey) {
      final label = modifierLabel(hotkey.doubleHotkey!);
      return [label, label];
    }

    final modifierChord = hotkey.displayModifierChord;
    if (modifierChord != null && modifierChord.isNotEmpty) {
      return modifierChord.map(labelFromRawPart).toList();
    }

    return [];
  }

  static String labelFromHotkeyString(String hotkey) {
    final rawHotkey = hotkey.trim();
    if (rawHotkey.isEmpty) {
      return "";
    }

    final parsedHotkey = WoxHotkey.parseHotkeyFromString(rawHotkey);
    if (parsedHotkey != null) {
      final parsedLabels = labelsFromHotkey(parsedHotkey);
      if (parsedLabels.isNotEmpty) {
        return parsedLabels.join("+");
      }
    }

    // Compatibility fallback: plugin-provided shortcuts can include keys that
    // the strict parser does not know yet. Normalize the common modifiers while
    // preserving the remaining key label instead of hiding the hint entirely.
    return rawHotkey.split("+").where((part) => part.trim().isNotEmpty).map(labelFromRawPart).join("+");
  }

  static String labelFromRawPart(String part) {
    final normalized = part.trim().toLowerCase();
    return switch (normalized) {
      "alt" => Platform.isMacOS ? "Option" : "Alt",
      "option" => "Option",
      "control" => "Ctrl",
      "ctrl" => "Ctrl",
      "shift" => "Shift",
      "meta" =>
        Platform.isMacOS
            ? "Cmd"
            : Platform.isWindows
            ? "Win"
            : "Super",
      "command" =>
        Platform.isMacOS
            ? "Cmd"
            : Platform.isWindows
            ? "Win"
            : "Super",
      "cmd" =>
        Platform.isMacOS
            ? "Cmd"
            : Platform.isWindows
            ? "Win"
            : "Super",
      "windows" => Platform.isWindows ? "Win" : "Super",
      "win" => Platform.isWindows ? "Win" : "Super",
      "super" => "Super",
      "left_ctrl" => "Left Ctrl",
      "right_ctrl" => "Right Ctrl",
      "left_shift" => "Left Shift",
      "right_shift" => "Right Shift",
      "left_alt" => Platform.isMacOS ? "Left Option" : "Left Alt",
      "right_alt" => Platform.isMacOS ? "Right Option" : "Right Alt",
      "left_cmd" => Platform.isMacOS ? "Left Cmd" : "Left Super",
      "right_cmd" => Platform.isMacOS ? "Right Cmd" : "Right Super",
      "left_win" => Platform.isWindows ? "Left Win" : "Left Super",
      "right_win" => Platform.isWindows ? "Right Win" : "Right Super",
      "space" => "Space",
      "enter" => "⏎",
      "escape" || "esc" => "Esc",
      "backspace" => "Backspace",
      "delete" => "Delete",
      "tab" => "Tab",
      "backquote" || "tilde" => "~",
      "arrowup" || "up" => "↑",
      "arrowdown" || "down" => "↓",
      "arrowleft" || "left" => "←",
      "arrowright" || "right" => "→",
      "pageup" => "PageUp",
      "pagedown" => "PageDown",
      "home" => "Home",
      "end" => "End",
      "insert" => "Insert",
      "capslock" => "CapsLock",
      "numlock" => "NumLock",
      "scrolllock" => "ScrollLock",
      "pause" => "Pause",
      "printscreen" => "PrintScreen",
      _ => normalized.length == 1 ? normalized.toUpperCase() : _shortTextKeyLabel(normalized),
    };
  }
}
