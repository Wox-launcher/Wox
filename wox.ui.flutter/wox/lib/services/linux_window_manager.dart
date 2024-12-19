import 'package:flutter/services.dart';
import 'package:wox/utils/log.dart';

class LinuxWindowManager {
  static const _channel = MethodChannel('com.wox.window_manager');
  
  static final LinuxWindowManager instance = LinuxWindowManager._();
  
  LinuxWindowManager._();

  Future<void> setSize(double width, double height) async {
    try {
      Logger.instance.info("LinuxWindowManager", "Setting size to: $width x $height");
      final Map<String, dynamic> arguments = {
        'width': width,
        'height': height,
      };
      await _channel.invokeMethod<void>('setSize', arguments);
    } catch (e) {
      Logger.instance.error("LinuxWindowManager", "Error setting window size: $e");
    }
  }
} 