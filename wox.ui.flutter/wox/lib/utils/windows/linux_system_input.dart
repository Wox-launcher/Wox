import 'dart:ui';

import 'package:flutter/services.dart';
import 'package:uuid/v4.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/system_input_interface.dart';

class LinuxSystemInput extends SystemInputInterface {
  static const MethodChannel _channel = MethodChannel('com.wox.linux_window_manager');

  static final LinuxSystemInput instance = LinuxSystemInput._();

  LinuxSystemInput._();

  @override
  Future<void> keyDown(String key) async {
    try {
      await _channel.invokeMethod('inputKeyDown', {'key': key});
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), 'Error sending key down for $key: $e');
      rethrow;
    }
  }

  @override
  Future<void> keyUp(String key) async {
    try {
      await _channel.invokeMethod('inputKeyUp', {'key': key});
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), 'Error sending key up for $key: $e');
      rethrow;
    }
  }

  @override
  Future<void> moveMouse(Offset position) async {
    try {
      await _channel.invokeMethod('inputMouseMove', {'x': position.dx, 'y': position.dy});
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), 'Error moving mouse to $position: $e');
      rethrow;
    }
  }

  @override
  Future<void> mouseDown(SystemMouseButton button) async {
    try {
      await _channel.invokeMethod('inputMouseDown', {'button': button.name});
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), 'Error sending mouse down for ${button.name}: $e');
      rethrow;
    }
  }

  @override
  Future<void> mouseUp(SystemMouseButton button) async {
    try {
      await _channel.invokeMethod('inputMouseUp', {'button': button.name});
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), 'Error sending mouse up for ${button.name}: $e');
      rethrow;
    }
  }
}
