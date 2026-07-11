import 'dart:io';

import 'package:flutter/services.dart';

enum HotKeyModifier {
  alt([PhysicalKeyboardKey.altLeft, PhysicalKeyboardKey.altRight]),
  capsLock([PhysicalKeyboardKey.capsLock]),
  control([PhysicalKeyboardKey.controlLeft, PhysicalKeyboardKey.controlRight]),
  fn([PhysicalKeyboardKey.fn]),
  meta([PhysicalKeyboardKey.metaLeft, PhysicalKeyboardKey.metaRight]),
  shift([PhysicalKeyboardKey.shiftLeft, PhysicalKeyboardKey.shiftRight]);

  const HotKeyModifier(this.physicalKeys);

  final List<PhysicalKeyboardKey> physicalKeys;
}

enum HotKeyScope { system, inapp }

class HotKey {
  final KeyboardKey key;
  final List<HotKeyModifier>? modifiers;
  final HotKeyScope scope;

  const HotKey({required this.key, this.modifiers, this.scope = HotKeyScope.system});
}

extension WoxKeyboardKeyExt on KeyboardKey {
  String get keyLabel {
    if (this is LogicalKeyboardKey) {
      final logicalKey = this as LogicalKeyboardKey;
      return logicalKey.keyLabel.isNotEmpty ? logicalKey.keyLabel : logicalKey.debugName ?? "Unknown";
    }

    final physicalKey = this is PhysicalKeyboardKey ? this as PhysicalKeyboardKey : null;
    return WoxHotkey.physicalKeyLabel(physicalKey) ?? physicalKey?.debugName ?? "Unknown";
  }
}

class HotkeyX {
  String raw;
  HotKey? normalHotkey; // normal hotkey, E.g. "ctrl+shift+a"
  KeyboardKey? capsLockHotkey; // Caps Lock combination, E.g. "capslock+a"
  HotKeyModifier? doubleHotkey; // double hotkey, E.g. "ctrl+ctrl"
  List<String>? modifierChord; // left/right modifier chord, E.g. "left_shift+left_cmd"
  List<String>? holdModifiers; // hold-mode left/right modifier chord, E.g. "hold:left_alt"

  HotkeyX(this.raw, {this.normalHotkey, this.capsLockHotkey, this.doubleHotkey, this.modifierChord, this.holdModifiers});

  bool get isNormalHotkey => normalHotkey != null;

  bool get isCapsLockHotkey => capsLockHotkey != null;

  bool get isDoubleHotkey => doubleHotkey != null;

  bool get isModifierChord => modifierChord != null && modifierChord!.isNotEmpty;

  bool get isHoldModifier => holdModifiers != null && holdModifiers!.isNotEmpty;

  List<String>? get displayModifierChord => isHoldModifier ? holdModifiers : modifierChord;

  String get kind {
    if (isNormalHotkey) {
      return WoxHotkey.kindNormalCombo;
    }
    if (isCapsLockHotkey) {
      return WoxHotkey.kindCapsLockCombo;
    }
    if (isDoubleHotkey) {
      return WoxHotkey.kindDoubleModifier;
    }
    if (isHoldModifier) {
      return WoxHotkey.kindHoldModifier;
    }
    if (isModifierChord) {
      return WoxHotkey.kindPressModifier;
    }
    return "";
  }

  String toStr() {
    return raw;
  }
}

class HotkeyRecordingCapability {
  final bool rawRecorderAvailable;
  final List<String> fallbackAllowedKinds;
  final String unavailableReason;

  HotkeyRecordingCapability({required this.rawRecorderAvailable, required this.fallbackAllowedKinds, required this.unavailableReason});

  factory HotkeyRecordingCapability.fromJson(Map<String, dynamic>? json) {
    final fallback = json?["FallbackAllowedKinds"];
    return HotkeyRecordingCapability(
      rawRecorderAvailable: json?["RawRecorderAvailable"] == true,
      fallbackAllowedKinds: fallback is List ? fallback.map((item) => item.toString()).toList() : <String>[],
      unavailableReason: json?["UnavailableReason"]?.toString() ?? "",
    );
  }
}

/// Hotkey availability result returned by core, including the owner of Wox-managed conflicts.
class HotkeyAvailability {
  final bool available;
  final String conflictType;
  final String conflictValue;

  HotkeyAvailability({required this.available, this.conflictType = "", this.conflictValue = ""});

  factory HotkeyAvailability.fromJson(Map<String, dynamic>? json) {
    return HotkeyAvailability(
      available: json?["Available"] == true,
      conflictType: json?["ConflictType"]?.toString() ?? "",
      conflictValue: json?["ConflictValue"]?.toString() ?? "",
    );
  }
}

/// A hotkey in Wox at least consists of a modifier and a key.
class WoxHotkey {
  static const String kindNormalCombo = "normalCombo";
  static const String kindDoubleModifier = "doubleModifier";
  static const String kindCapsLockCombo = "capsLockCombo";
  static const String kindHoldModifier = "holdModifier";
  static const String kindPressModifier = "pressModifier";
  static const String holdModifierPrefix = "hold:";

  static HotkeyX? parseHotkeyFromString(String value) {
    final trimmedValue = value.trim();
    if (trimmedValue.startsWith(holdModifierPrefix)) {
      final innerValue = trimmedValue.substring(holdModifierPrefix.length).trim();
      final innerHotkey = parseHotkeyFromString(innerValue);
      if (innerHotkey == null) {
        return null;
      }
      final holdModifiers = innerHotkey.displayModifierChord;
      if (holdModifiers == null || holdModifiers.isEmpty) {
        return null;
      }
      return HotkeyX(trimmedValue, holdModifiers: holdModifiers);
    }

    final modifiers = <HotKeyModifier>[];
    var isCapsLockCombo = false;
    LogicalKeyboardKey? key;
    final tokens = trimmedValue.split("+").map((element) => element.trim().toLowerCase()).where((element) => element.isNotEmpty).toList();
    final modifierChord = _parseModifierChord(tokens);
    if (modifierChord != null) {
      return HotkeyX(trimmedValue, modifierChord: modifierChord);
    }

    for (final e in tokens) {
      if ((e == "capslock" || e == "caps_lock" || e == "caps lock") && tokens.length > 1) {
        isCapsLockCombo = true;
      } else if (e == "alt" || e == "option") {
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
      } else if (e == "backquote" || e == "tilde" || e == "~" || e == "`") {
        key = LogicalKeyboardKey.backquote;
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
      } else if (e == "up" || e == "arrowup") {
        key = LogicalKeyboardKey.arrowUp;
      } else if (e == "down" || e == "arrowdown") {
        key = LogicalKeyboardKey.arrowDown;
      } else if (e == "left" || e == "arrowleft") {
        key = LogicalKeyboardKey.arrowLeft;
      } else if (e == "right" || e == "arrowright") {
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
    }

    // double hotkey
    if (key == null && modifiers.length == 2 && modifiers[0] == modifiers[1]) {
      return HotkeyX(trimmedValue, doubleHotkey: modifiers[0]);
    }

    if (isCapsLockCombo && key != null) {
      return HotkeyX(trimmedValue, capsLockHotkey: key);
    }

    // normal hotkey
    if (key != null && modifiers.isNotEmpty) {
      return HotkeyX(trimmedValue, normalHotkey: HotKey(key: key, modifiers: modifiers));
    }

    return null;
  }

  // recordedHotkeyToString folds backend recorder kind into the persisted hotkey string.
  static String recordedHotkeyToString(String hotkey, String kind) {
    final trimmedHotkey = hotkey.trim();
    if (trimmedHotkey.isEmpty) {
      return "";
    }
    if (kind == kindHoldModifier && !trimmedHotkey.startsWith(holdModifierPrefix)) {
      return "$holdModifierPrefix$trimmedHotkey";
    }
    return trimmedHotkey;
  }

  static List<String>? _parseModifierChord(List<String> tokens) {
    if (tokens.isEmpty || tokens.length > 2) {
      return null;
    }

    final normalized = <String>[];
    final seen = <String>{};
    for (final token in tokens) {
      final part = _normalizeSpecificModifierPart(token);
      if (part == null || !seen.add(part)) {
        return null;
      }
      normalized.add(part);
    }

    normalized.sort((a, b) => _specificModifierOrder(a).compareTo(_specificModifierOrder(b)));
    return normalized;
  }

  static String? _normalizeSpecificModifierPart(String token) {
    final parts = token.split("_");
    if (parts.length != 2) {
      return null;
    }

    final side = parts[0];
    if (side != "left" && side != "right") {
      return null;
    }

    return switch (parts[1]) {
      "ctrl" || "control" => "${side}_ctrl",
      "shift" => "${side}_shift",
      "alt" || "option" => "${side}_alt",
      "cmd" || "command" || "super" || "win" || "windows" => Platform.isMacOS ? "${side}_cmd" : "${side}_win",
      _ => null,
    };
  }

  static int _specificModifierOrder(String part) {
    const order = {"left_ctrl": 0, "right_ctrl": 1, "left_shift": 2, "right_shift": 3, "left_alt": 4, "right_alt": 5, "left_cmd": 6, "right_cmd": 7, "left_win": 6, "right_win": 7};
    return order[part] ?? 100;
  }

  static HotKey? parseNormalHotkeyFromEvent(KeyEvent event) {
    if (event is KeyUpEvent) return null;

    if (!WoxHotkey.isAllowedKey(event.physicalKey)) {
      // Not an allowed key for hotkey combinations (e.g., modifier keys alone)
      // This is normal behavior, not an error
      return null;
    }

    List<HotKeyModifier> modifiers = [];

    if (HardwareKeyboard.instance.isControlPressed) {
      modifiers.add(HotKeyModifier.control);
    }
    if (HardwareKeyboard.instance.isShiftPressed) {
      modifiers.add(HotKeyModifier.shift);
    }
    if (HardwareKeyboard.instance.isAltPressed) {
      modifiers.add(HotKeyModifier.alt);
    }
    if (HardwareKeyboard.instance.isMetaPressed) {
      modifiers.add(HotKeyModifier.meta);
    }

    if (modifiers.isEmpty) {
      return null;
    }

    return HotKey(key: event.physicalKey, modifiers: modifiers, scope: HotKeyScope.system);
  }

  static bool isAnyModifierPressed() {
    return HardwareKeyboard.instance.physicalKeysPressed.any((element) => HotKeyModifier.values.any((e) => e.physicalKeys.contains(element)));
  }

  static bool isModifierKey(PhysicalKeyboardKey key) {
    return HotKeyModifier.values.any((e) => e.physicalKeys.contains(key));
  }

  static List<HotKeyModifier> getPressedModifiers() {
    final modifiers = <HotKeyModifier>[];

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
      PhysicalKeyboardKey.backquote,
    ];

    return allowedKeys.contains(key);
  }

  static String? physicalKeyLabel(PhysicalKeyboardKey? key) {
    return switch (key) {
      PhysicalKeyboardKey.keyA => "A",
      PhysicalKeyboardKey.keyB => "B",
      PhysicalKeyboardKey.keyC => "C",
      PhysicalKeyboardKey.keyD => "D",
      PhysicalKeyboardKey.keyE => "E",
      PhysicalKeyboardKey.keyF => "F",
      PhysicalKeyboardKey.keyG => "G",
      PhysicalKeyboardKey.keyH => "H",
      PhysicalKeyboardKey.keyI => "I",
      PhysicalKeyboardKey.keyJ => "J",
      PhysicalKeyboardKey.keyK => "K",
      PhysicalKeyboardKey.keyL => "L",
      PhysicalKeyboardKey.keyM => "M",
      PhysicalKeyboardKey.keyN => "N",
      PhysicalKeyboardKey.keyO => "O",
      PhysicalKeyboardKey.keyP => "P",
      PhysicalKeyboardKey.keyQ => "Q",
      PhysicalKeyboardKey.keyR => "R",
      PhysicalKeyboardKey.keyS => "S",
      PhysicalKeyboardKey.keyT => "T",
      PhysicalKeyboardKey.keyU => "U",
      PhysicalKeyboardKey.keyV => "V",
      PhysicalKeyboardKey.keyW => "W",
      PhysicalKeyboardKey.keyX => "X",
      PhysicalKeyboardKey.keyY => "Y",
      PhysicalKeyboardKey.keyZ => "Z",
      PhysicalKeyboardKey.digit0 => "0",
      PhysicalKeyboardKey.digit1 => "1",
      PhysicalKeyboardKey.digit2 => "2",
      PhysicalKeyboardKey.digit3 => "3",
      PhysicalKeyboardKey.digit4 => "4",
      PhysicalKeyboardKey.digit5 => "5",
      PhysicalKeyboardKey.digit6 => "6",
      PhysicalKeyboardKey.digit7 => "7",
      PhysicalKeyboardKey.digit8 => "8",
      PhysicalKeyboardKey.digit9 => "9",
      PhysicalKeyboardKey.f1 => "F1",
      PhysicalKeyboardKey.f2 => "F2",
      PhysicalKeyboardKey.f3 => "F3",
      PhysicalKeyboardKey.f4 => "F4",
      PhysicalKeyboardKey.f5 => "F5",
      PhysicalKeyboardKey.f6 => "F6",
      PhysicalKeyboardKey.f7 => "F7",
      PhysicalKeyboardKey.f8 => "F8",
      PhysicalKeyboardKey.f9 => "F9",
      PhysicalKeyboardKey.f10 => "F10",
      PhysicalKeyboardKey.f11 => "F11",
      PhysicalKeyboardKey.f12 => "F12",
      PhysicalKeyboardKey.enter => "Enter",
      PhysicalKeyboardKey.escape => "Escape",
      PhysicalKeyboardKey.backspace => "Backspace",
      PhysicalKeyboardKey.tab => "Tab",
      PhysicalKeyboardKey.space => "Space",
      PhysicalKeyboardKey.delete => "Delete",
      PhysicalKeyboardKey.arrowLeft => "Arrow Left",
      PhysicalKeyboardKey.arrowDown => "Arrow Down",
      PhysicalKeyboardKey.arrowRight => "Arrow Right",
      PhysicalKeyboardKey.arrowUp => "Arrow Up",
      PhysicalKeyboardKey.home => "Home",
      PhysicalKeyboardKey.end => "End",
      PhysicalKeyboardKey.pageUp => "Page Up",
      PhysicalKeyboardKey.pageDown => "Page Down",
      PhysicalKeyboardKey.insert => "Insert",
      PhysicalKeyboardKey.capsLock => "CapsLock",
      PhysicalKeyboardKey.shiftLeft || PhysicalKeyboardKey.shiftRight => "Shift",
      PhysicalKeyboardKey.controlLeft || PhysicalKeyboardKey.controlRight => "Control",
      PhysicalKeyboardKey.altLeft || PhysicalKeyboardKey.altRight => "Alt",
      PhysicalKeyboardKey.metaLeft || PhysicalKeyboardKey.metaRight => "Meta",
      PhysicalKeyboardKey.backquote => "~",
      _ => null,
    };
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

    final keyStr = keyToStr(hotKey.key);

    return "${modifiers.join("+")}+$keyStr";
  }

  static String capsLockHotkeyToStr(KeyboardKey key) {
    return "capslock+${keyToStr(key)}";
  }

  static String keyToStr(KeyboardKey key) {
    var keyStr = key.keyLabel.toLowerCase();
    if (keyStr.startsWith("key ")) {
      keyStr = keyStr.substring(4);
    }
    if (key == PhysicalKeyboardKey.space || key == LogicalKeyboardKey.space) {
      keyStr = "space";
    } else if (key == PhysicalKeyboardKey.enter || key == LogicalKeyboardKey.enter) {
      keyStr = "enter";
    } else if (key == PhysicalKeyboardKey.backspace || key == LogicalKeyboardKey.backspace) {
      keyStr = "backspace";
    } else if (key == PhysicalKeyboardKey.delete || key == LogicalKeyboardKey.delete) {
      keyStr = "delete";
    } else if (key == PhysicalKeyboardKey.arrowLeft || key == LogicalKeyboardKey.arrowLeft) {
      keyStr = "left";
    } else if (key == PhysicalKeyboardKey.arrowDown || key == LogicalKeyboardKey.arrowDown) {
      keyStr = "down";
    } else if (key == PhysicalKeyboardKey.arrowRight || key == LogicalKeyboardKey.arrowRight) {
      keyStr = "right";
    } else if (key == PhysicalKeyboardKey.arrowUp || key == LogicalKeyboardKey.arrowUp) {
      keyStr = "up";
    } else if (key == PhysicalKeyboardKey.backquote || key == LogicalKeyboardKey.backquote) {
      keyStr = "~";
    }
    return keyStr;
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
