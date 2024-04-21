import 'package:flutter/services.dart';
import 'package:hotkey_manager/hotkey_manager.dart';

class WoxHotkey {
  static HotKey? parseHotkey(String value) {
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

    if (modifiers.isEmpty || key == null) {
      return null;
    }

    return HotKey(
      key: key!,
      modifiers: modifiers,
    );
  }
}
