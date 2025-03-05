import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/base_window_manager.dart';
import 'package:wox/utils/windows/window_manager_interface.dart';

/// macOS implementation of the window manager
class MacOSWindowManager extends BaseWindowManager {
  static const _channel = MethodChannel('com.wox.macos_window_manager');

  static final MacOSWindowManager instance = MacOSWindowManager._();

  MacOSWindowManager._();

  @override
  Future<void> ensureInitialized() async {
    try {
      await _channel.invokeMethod('ensureInitialized');
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error initializing: $e");
    }
  }

  @override
  Future<void> setSize(Size size) async {
    try {
      await _channel.invokeMethod('setSize', {
        'width': size.width,
        'height': size.height,
      });
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error setting window size: $e");
    }
  }

  @override
  Future<Offset> getPosition() async {
    try {
      final Map<dynamic, dynamic> result = await _channel.invokeMethod('getPosition');
      return Offset(result['x'], result['y']);
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error getting position: $e");
      return Offset.zero;
    }
  }

  @override
  Future<void> setPosition(Offset position) async {
    try {
      await _channel.invokeMethod('setPosition', {
        'x': position.dx,
        'y': position.dy,
      });
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error setting position: $e");
    }
  }

  @override
  Future<void> center() async {
    try {
      await _channel.invokeMethod('center');
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error centering window: $e");
    }
  }

  @override
  Future<void> show() async {
    try {
      await _channel.invokeMethod('show');
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error showing window: $e");
    }
  }

  @override
  Future<void> hide() async {
    try {
      await _channel.invokeMethod('hide');
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error hiding window: $e");
    }
  }

  @override
  Future<void> focus() async {
    try {
      await _channel.invokeMethod('focus');
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error focusing window: $e");
    }
  }

  @override
  Future<bool> isVisible() async {
    try {
      return await _channel.invokeMethod('isVisible');
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error checking visibility: $e");
      return false;
    }
  }

  @override
  Future<void> setAlwaysOnTop(bool alwaysOnTop) async {
    try {
      await _channel.invokeMethod('setAlwaysOnTop', alwaysOnTop);
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error setting always on top: $e");
    }
  }

  @override
  Future<void> waitUntilReadyToShow() async {
    try {
      await _channel.invokeMethod('waitUntilReadyToShow');
    } catch (e) {
      Logger.instance.error("MacOSWindowManager", "Error waiting until ready to show: $e");
    }
  }
}
