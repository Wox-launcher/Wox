import 'dart:ui';

import 'package:flutter/services.dart';
import 'package:uuid/uuid.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/system_input_interface.dart';

class MacOSSystemInput extends SystemInputInterface {
  static const MethodChannel _channel = MethodChannel('com.wox.macos_window_manager');

  static final MacOSSystemInput instance = MacOSSystemInput._();

  MacOSSystemInput._();

  @override
  Future<void> keyDown(String key) async {
    try {
      await _channel.invokeMethod('inputKeyDown', {'key': key});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), 'Error sending key down for $key: $e');
      rethrow;
    }
  }

  @override
  Future<void> keyUp(String key) async {
    try {
      await _channel.invokeMethod('inputKeyUp', {'key': key});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), 'Error sending key up for $key: $e');
      rethrow;
    }
  }

  @override
  Future<void> moveMouse(Offset position) async {
    try {
      await _channel.invokeMethod('inputMouseMove', {'x': position.dx, 'y': position.dy});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), 'Error moving mouse to $position: $e');
      rethrow;
    }
  }

  @override
  Future<void> mouseDown(SystemMouseButton button) async {
    try {
      await _channel.invokeMethod('inputMouseDown', {'button': button.name});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), 'Error sending mouse down for ${button.name}: $e');
      rethrow;
    }
  }

  @override
  Future<void> mouseUp(SystemMouseButton button) async {
    try {
      await _channel.invokeMethod('inputMouseUp', {'button': button.name});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), 'Error sending mouse up for ${button.name}: $e');
      rethrow;
    }
  }
}
