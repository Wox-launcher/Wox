import 'dart:async';

import 'package:uuid/v4.dart';
import 'package:wox/utils/log.dart';

class WoxHotkeyRecordingBus {
  WoxHotkeyRecordingBus._privateConstructor();

  static final WoxHotkeyRecordingBus instance = WoxHotkeyRecordingBus._privateConstructor();

  final StreamController<String> _controller = StreamController<String>.broadcast();

  Stream<String> get stream => _controller.stream;

  void emit(String hotkey) {
    if (hotkey.trim().isEmpty) {
      Logger.instance.warn(const UuidV4().generate(), "Hotkey recording bus ignored empty hotkey event");
      return;
    }
    Logger.instance.info(const UuidV4().generate(), "Hotkey recording bus emits hotkey=$hotkey");
    _controller.add(hotkey);
  }
}
