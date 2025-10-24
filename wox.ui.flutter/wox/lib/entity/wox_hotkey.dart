import 'dart:io';

import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:wox/utils/windows/windows_window_manager.dart';

class HotkeyX {
  String raw;
  HotKey? normalHotkey; // normal hotkey, E.g. "ctrl+shift+a"
  HotKeyModifier? doubleHotkey; // double hotkey, E.g. "ctrl+ctrl"
  LogicalKeyboardKey? singleHotkey; // single hotkey, E.g. "enter", usually used for default action hotkey

  HotkeyX(this.raw, {this.normalHotkey, this.doubleHotkey, this.singleHotkey});

  bool get isNormalHotkey => normalHotkey != null;

  bool get isDoubleHotkey => doubleHotkey != null;

  bool get isSingleHotkey => singleHotkey != null;

  String toStr() {
    return raw;
  }
}

/// A hotkey in Wox at least consists of a modifier and a key.
class WoxHotkey {
  static HotkeyX? parseHotkeyFromString(String value) {
    final modifiers = <HotKeyModifier>[];
    LogicalKeyboardKey? key;
    value.split("+").forEach((element) {
      final e = element.toLowerCase();
      if (e == "alt" || e == "option") {
        modifiers.add(HotKeyModifier.alt);
      } else if (e == "control" || e == "ctrl") {
        modifiers.add(HotKeyModifier.control);
      } else if (e == "shift") {
        modifiers.add(HotKeyModifier.shift);
      } else if (e == "meta" || e == "command" || e == "cmd") {
        modifiers.add(HotKeyModifier.meta);
      } else if (e == "windows" || e == "win") {
        modifiers.add(HotKeyModifier.meta);
      } else if (e == "a") {
        key = LogicalKeyboardKey.keyA;
      } else if (e == "b") {
        key = LogicalKeyboardKey.keyB;
      } else if (e == "c") {
        key = LogicalKeyboardKey.keyC;
      } else if (e == "d") {
        key = LogicalKeyboardKey.keyD;
      } else if (e == "e") {
        key = LogicalKeyboardKey.keyE;
      } else if (e == "f") {
        key = LogicalKeyboardKey.keyF;
      } else if (e == "g") {
        key = LogicalKeyboardKey.keyG;
      } else if (e == "h") {
        key = LogicalKeyboardKey.keyH;
      } else if (e == "i") {
        key = LogicalKeyboardKey.keyI;
      } else if (e == "j") {
        key = LogicalKeyboardKey.keyJ;
      } else if (e == "k") {
        key = LogicalKeyboardKey.keyK;
      } else if (e == "l") {
        key = LogicalKeyboardKey.keyL;
      } else if (e == "m") {
        key = LogicalKeyboardKey.keyM;
      } else if (e == "n") {
        key = LogicalKeyboardKey.keyN;
      } else if (e == "o") {
        key = LogicalKeyboardKey.keyO;
      } else if (e == "p") {
        key = LogicalKeyboardKey.keyP;
      } else if (e == "q") {
        key = LogicalKeyboardKey.keyQ;
      } else if (e == "r") {
        key = LogicalKeyboardKey.keyR;
      } else if (e == "s") {
        key = LogicalKeyboardKey.keyS;
      } else if (e == "t") {
        key = LogicalKeyboardKey.keyT;
      } else if (e == "u") {
        key = LogicalKeyboardKey.keyU;
      } else if (e == "v") {
        key = LogicalKeyboardKey.keyV;
      } else if (e == "w") {
        key = LogicalKeyboardKey.keyW;
      } else if (e == "x") {
        key = LogicalKeyboardKey.keyX;
      } else if (e == "y") {
        key = LogicalKeyboardKey.keyY;
      } else if (e == "z") {
        key = LogicalKeyboardKey.keyZ;
      } else if (e == "0") {
        key = LogicalKeyboardKey.digit0;
      } else if (e == "1") {
        key = LogicalKeyboardKey.digit1;
      } else if (e == "2") {
        key = LogicalKeyboardKey.digit2;
      } else if (e == "3") {
        key = LogicalKeyboardKey.digit3;
      } else if (e == "4") {
        key = LogicalKeyboardKey.digit4;
      } else if (e == "5") {
        key = LogicalKeyboardKey.digit5;
      } else if (e == "6") {
        key = LogicalKeyboardKey.digit6;
      } else if (e == "7") {
        key = LogicalKeyboardKey.digit7;
      } else if (e == "8") {
        key = LogicalKeyboardKey.digit8;
      } else if (e == "9") {
        key = LogicalKeyboardKey.digit9;
      } else if (e == "f1") {
        key = LogicalKeyboardKey.f1;
      } else if (e == "f2") {
        key = LogicalKeyboardKey.f2;
      } else if (e == "f3") {
        key = LogicalKeyboardKey.f3;
      } else if (e == "f4") {
        key = LogicalKeyboardKey.f4;
      } else if (e == "f5") {
        key = LogicalKeyboardKey.f5;
      } else if (e == "f6") {
        key = LogicalKeyboardKey.f6;
      } else if (e == "f7") {
        key = LogicalKeyboardKey.f7;
      } else if (e == "f8") {
        key = LogicalKeyboardKey.f8;
      } else if (e == "f9") {
        key = LogicalKeyboardKey.f9;
      } else if (e == "f10") {
        key = LogicalKeyboardKey.f10;
      } else if (e == "f11") {
        key = LogicalKeyboardKey.f11;
      } else if (e == "f12") {
        key = LogicalKeyboardKey.f12;
      } else if (e == "f13") {
        key = LogicalKeyboardKey.f13;
      } else if (e == "f14") {
        key = LogicalKeyboardKey.f14;
      } else if (e == "space") {
        key = LogicalKeyboardKey.space;
      } else if (e == "enter") {
        key = LogicalKeyboardKey.enter;
      } else if (e == "backspace") {
        key = LogicalKeyboardKey.backspace;
      } else if (e == "delete") {
        key = LogicalKeyboardKey.delete;
      } else if (e == "escape") {
        key = LogicalKeyboardKey.escape;
      } else if (e == "tab") {
        key = LogicalKeyboardKey.tab;
      } else if (e == "capslock") {
        key = LogicalKeyboardKey.capsLock;
      } else if (e == "shiftleft") {
        key = LogicalKeyboardKey.shiftLeft;
      } else if (e == "shiftright") {
        key = LogicalKeyboardKey.shiftRight;
      } else if (e == "controlleft") {
        key = LogicalKeyboardKey.controlLeft;
      } else if (e == "controlright") {
        key = LogicalKeyboardKey.controlRight;
      } else if (e == "altleft") {
        key = LogicalKeyboardKey.altLeft;
      } else if (e == "altright") {
        key = LogicalKeyboardKey.altRight;
      } else if (e == "metaleft") {
        key = LogicalKeyboardKey.metaLeft;
      } else if (e == "metaright") {
        key = LogicalKeyboardKey.metaRight;
      } else if (e == "arrowup") {
        key = LogicalKeyboardKey.arrowUp;
      } else if (e == "arrowdown") {
        key = LogicalKeyboardKey.arrowDown;
      } else if (e == "arrowleft") {
        key = LogicalKeyboardKey.arrowLeft;
      } else if (e == "arrowright") {
        key = LogicalKeyboardKey.arrowRight;
      } else if (e == "pageup") {
        key = LogicalKeyboardKey.pageUp;
      } else if (e == "pagedown") {
        key = LogicalKeyboardKey.pageDown;
      } else if (e == "home") {
        key = LogicalKeyboardKey.home;
      } else if (e == "end") {
        key = LogicalKeyboardKey.end;
      } else if (e == "insert") {
        key = LogicalKeyboardKey.insert;
      }
    });

    // double hotkey
    if (key == null && modifiers.length == 2 && modifiers[0] == modifiers[1]) {
      return HotkeyX(value, doubleHotkey: modifiers[0]);
    }

    // normal hotkey
    if (key != null && modifiers.isNotEmpty) {
      return HotkeyX(value, normalHotkey: HotKey(key: key!, modifiers: modifiers));
    }

    return HotkeyX(value, singleHotkey: key);
  }

  static HotKey? parseNormalHotkeyFromEvent(KeyEvent event) {
    if (event is KeyUpEvent) return null;

    if (!WoxHotkey.isAllowedKey(event.physicalKey)) {
      // Not an allowed key for hotkey combinations (e.g., modifier keys alone)
      // This is normal behavior, not an error
      return null;
    }

    List<HotKeyModifier> modifiers = [];

    // On Windows, use WindowsWindowManager's modifier key states (more reliable)
    if (Platform.isWindows) {
      final states = WindowsWindowManager.instance.currentModifierStates;
      if (states.isAltPressed) {
        modifiers.add(HotKeyModifier.alt);
      }
      if (states.isControlPressed) {
        modifiers.add(HotKeyModifier.control);
      }
      if (states.isShiftPressed) {
        modifiers.add(HotKeyModifier.shift);
      }
      if (states.isMetaPressed) {
        modifiers.add(HotKeyModifier.meta);
      }
    } else {
      // On other platforms, use HardwareKeyboard
      if (HardwareKeyboard.instance.isAltPressed) {
        modifiers.add(HotKeyModifier.alt);
      }
      if (HardwareKeyboard.instance.isControlPressed) {
        modifiers.add(HotKeyModifier.control);
      }
      if (HardwareKeyboard.instance.isShiftPressed) {
        modifiers.add(HotKeyModifier.shift);
      }
      if (HardwareKeyboard.instance.isMetaPressed) {
        modifiers.add(HotKeyModifier.meta);
      }
    }

    if (modifiers.isEmpty) {
      return null;
    }

    return HotKey(key: event.physicalKey, modifiers: modifiers, scope: HotKeyScope.system);
  }

  static bool isAnyModifierPressed() {
    // On Windows, use WindowsWindowManager's modifier key states (more reliable)
    if (Platform.isWindows) {
      final states = WindowsWindowManager.instance.currentModifierStates;
      return states.isShiftPressed || states.isControlPressed || states.isAltPressed || states.isMetaPressed;
    }

    // On other platforms, use HardwareKeyboard
    return HardwareKeyboard.instance.physicalKeysPressed.any((element) => HotKeyModifier.values.any((e) => e.physicalKeys.contains(element)));
  }

  static bool isModifierKey(PhysicalKeyboardKey key) {
    return HotKeyModifier.values.any((e) => e.physicalKeys.contains(key));
  }

  static List<HotKeyModifier> getPressedModifiers() {
    final modifiers = <HotKeyModifier>[];

    // On Windows, use WindowsWindowManager's modifier key states (more reliable)
    if (Platform.isWindows) {
      final states = WindowsWindowManager.instance.currentModifierStates;
      if (states.isAltPressed) {
        modifiers.add(HotKeyModifier.alt);
      }
      if (states.isControlPressed) {
        modifiers.add(HotKeyModifier.control);
      }
      if (states.isShiftPressed) {
        modifiers.add(HotKeyModifier.shift);
      }
      if (states.isMetaPressed) {
        modifiers.add(HotKeyModifier.meta);
      }
      return modifiers;
    }

    // On other platforms, use HardwareKeyboard
    if (HardwareKeyboard.instance.isAltPressed) {
      modifiers.add(HotKeyModifier.alt);
    }
    if (HardwareKeyboard.instance.isControlPressed) {
      modifiers.add(HotKeyModifier.control);
    }
    if (HardwareKeyboard.instance.isShiftPressed) {
      modifiers.add(HotKeyModifier.shift);
    }
    if (HardwareKeyboard.instance.isMetaPressed) {
      modifiers.add(HotKeyModifier.meta);
    }

    return modifiers;
  }

  static bool isAllowedKey(PhysicalKeyboardKey key) {
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

  static bool equals(HotKey? a, HotKey? b) {
    if (a == null || b == null) {
      return false;
    }

    return a.key.keyLabel == b.key.keyLabel && isModifiersEquals(a.modifiers, b.modifiers);
  }

  static bool isModifiersEquals(List<HotKeyModifier>? a, List<HotKeyModifier>? b) {
    if (a == null || b == null) {
      return false;
    }

    if (a.length != b.length) {
      return false;
    }

    // check if all elements in a are in b
    // and all elements in b are in a
    return a.every((element) => b.map((o) => o.name).contains(element.name)) && b.every((element) => a.map((o) => o.name).contains(element.name));
  }

  static String normalHotkeyToStr(HotKey hotKey) {
    var modifiers = [];
    if (hotKey.modifiers != null) {
      for (var modifier in hotKey.modifiers!) {
        modifiers.add(getModifierStr(modifier));
      }
    }

    var keyStr = hotKey.key.keyLabel.toLowerCase();
    if (keyStr.startsWith("key ")) {
      keyStr = keyStr.substring(4);
    }
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

  static HotKeyModifier? convertToModifier(PhysicalKeyboardKey key) {
    if (key == PhysicalKeyboardKey.shiftLeft || key == PhysicalKeyboardKey.shiftRight) {
      return HotKeyModifier.shift;
    } else if (key == PhysicalKeyboardKey.controlLeft || key == PhysicalKeyboardKey.controlRight) {
      return HotKeyModifier.control;
    } else if (key == PhysicalKeyboardKey.altLeft || key == PhysicalKeyboardKey.altRight) {
      return HotKeyModifier.alt;
    } else if (key == PhysicalKeyboardKey.metaLeft || key == PhysicalKeyboardKey.metaRight) {
      return HotKeyModifier.meta;
    }

    return null;
  }

  static String getModifierStr(HotKeyModifier modifier) {
    if (modifier == HotKeyModifier.shift) {
      return "shift";
    } else if (modifier == HotKeyModifier.control) {
      return "ctrl";
    } else if (modifier == HotKeyModifier.alt) {
      if (Platform.isMacOS) {
        return "option";
      } else {
        return "alt";
      }
    } else if (modifier == HotKeyModifier.meta) {
      if (Platform.isMacOS) {
        return "cmd";
      } else {
        return "win";
      }
    }

    return "";
  }
}
