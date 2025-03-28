import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';

class WoxDoubleHotkey {
  static const String doubleClickPrefix = "double_";

  static bool isDoubleClickHotkey(String hotkeyStr) {
    return hotkeyStr.startsWith(doubleClickPrefix);
  }

  static String toDoubleClickStr(HotKeyModifier modifier) {
    String modifierStr = "";
    switch (modifier) {
      case HotKeyModifier.alt:
        modifierStr = "alt";
        break;
      case HotKeyModifier.control:
        modifierStr = "control";
        break;
      case HotKeyModifier.shift:
        modifierStr = "shift";
        break;
      case HotKeyModifier.meta:
        modifierStr = "meta";
        break;
      default:
        throw Exception("Unsupported modifier for double click");
    }
    return "${doubleClickPrefix}$modifierStr";
  }

  static HotKeyModifier? parseModifierFromDoubleClickStr(String hotkeyStr) {
    if (!isDoubleClickHotkey(hotkeyStr)) return null;

    String modifierStr = hotkeyStr.substring(doubleClickPrefix.length);
    switch (modifierStr) {
      case "alt":
        return HotKeyModifier.alt;
      case "control":
        return HotKeyModifier.control;
      case "shift":
        return HotKeyModifier.shift;
      case "meta":
        return HotKeyModifier.meta;
      default:
        return null;
    }
  }
}
