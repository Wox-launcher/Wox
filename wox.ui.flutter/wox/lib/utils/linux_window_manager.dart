import 'package:flutter/services.dart';
import 'package:wox/utils/log.dart';

class LinuxWindowManager {
  static const _channel = MethodChannel('com.wox.window_manager');
  
  static final LinuxWindowManager instance = LinuxWindowManager._();
  
  LinuxWindowManager._();

  Future<void> setSize(double width, double height) async {
    try {
      await _channel.invokeMethod('setSize', {
        'width': width,
        'height': height,
      });
    } catch (e) {
      Logger.instance.error("LinuxWindowManager", "Error setting window size: $e");
    }
  }
} 