import 'dart:async';

import 'package:uuid/v4.dart';
import 'package:wox/utils/log.dart';

class HotkeyRecordingResult {
  final String hotkey;
  final String kind;

  const HotkeyRecordingResult({required this.hotkey, required this.kind});
}

class WoxHotkeyRecordingBus {
  WoxHotkeyRecordingBus._privateConstructor();

  static final WoxHotkeyRecordingBus instance = WoxHotkeyRecordingBus._privateConstructor();

  final StreamController<HotkeyRecordingResult> _controller = StreamController<HotkeyRecordingResult>.broadcast();

  Stream<HotkeyRecordingResult> get stream => _controller.stream;

  void emit(String hotkey, String kind) {
    if (hotkey.trim().isEmpty) {
      Logger.instance.warn(const UuidV4().generate(), "Hotkey recording bus ignored empty hotkey event");
      return;
    }
    Logger.instance.info(const UuidV4().generate(), "Hotkey recording bus emits hotkey=$hotkey kind=$kind");
    _controller.add(HotkeyRecordingResult(hotkey: hotkey, kind: kind));
  }
}
