import 'package:flutter/services.dart';
import 'package:uuid/uuid.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/base_window_manager.dart';

/// macOS implementation of the window manager
class MacOSWindowManager extends BaseWindowManager {
  static const _channel = MethodChannel('com.wox.macos_window_manager');

  static final MacOSWindowManager instance = MacOSWindowManager._();

  MacOSWindowManager._() {
    // Set up method call handler for receiving events from native side
    _channel.setMethodCallHandler(_handleMethodCall);
  }

  // Handle method calls from the native side
  Future<dynamic> _handleMethodCall(MethodCall call) async {
    switch (call.method) {
      case 'onWindowBlur':
        Logger.instance.debug(const Uuid().v4(), "Window blur event received");
        notifyWindowBlur();
        break;
      default:
        Logger.instance.debug(const Uuid().v4(), "Unhandled method call: ${call.method}");
    }
    return null;
  }

  @override
  Future<void> setSize(Size size) async {
    try {
      await _channel.invokeMethod('setSize', {'width': size.width, 'height': size.height});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error setting window size: $e");
    }
  }

  @override
  Future<void> setBounds(Offset position, Size size) async {
    try {
      await _channel.invokeMethod('setBounds', {'x': position.dx, 'y': position.dy, 'width': size.width, 'height': size.height});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error setting window bounds: $e");
    }
  }

  @override
  Future<Offset> getPosition() async {
    try {
      final Map<dynamic, dynamic> result = await _channel.invokeMethod('getPosition');
      return Offset(result['x'], result['y']);
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error getting position: $e");
      return Offset.zero;
    }
  }

  @override
  Future<Size> getSize() async {
    try {
      final Map<dynamic, dynamic> result = await _channel.invokeMethod('getSize');
      return Size(result['width'], result['height']);
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error getting size: $e");
      return Size.zero;
    }
  }

  @override
  Future<void> setPosition(Offset position) async {
    try {
      await _channel.invokeMethod('setPosition', {'x': position.dx, 'y': position.dy});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error setting position: $e");
    }
  }

  @override
  Future<void> center(double width, double height) async {
    try {
      await _channel.invokeMethod('center', {'width': width, 'height': height});
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error centering window: $e");
    }
  }

  @override
  Future<void> show() async {
    try {
      await _channel.invokeMethod('show');
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error showing window: $e");
    }
  }

  @override
  Future<void> hide() async {
    try {
      await _channel.invokeMethod('hide');
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error hiding window: $e");
    }
  }

  @override
  Future<void> focus() async {
    try {
      await _channel.invokeMethod('focus');
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error focusing window: $e");
    }
  }

  @override
  Future<bool> isVisible() async {
    try {
      return await _channel.invokeMethod('isVisible');
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error checking visibility: $e");
      return false;
    }
  }

  @override
  Future<void> setAlwaysOnTop(bool alwaysOnTop) async {
    try {
      await _channel.invokeMethod('setAlwaysOnTop', alwaysOnTop);
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error setting always on top: $e");
    }
  }

  @override
  Future<void> setAppearance(String appearance) async {
    try {
      await _channel.invokeMethod('setAppearance', appearance);
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error setting appearance: $e");
    }
  }

  @override
  Future<void> startDragging() async {
    try {
      await _channel.invokeMethod('startDragging');
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error starting window drag: $e");
    }
  }

  @override
  Future<void> waitUntilReadyToShow() async {
    try {
      await _channel.invokeMethod('waitUntilReadyToShow');
    } catch (e) {
      Logger.instance.error(const Uuid().v4(), "Error waiting until ready to show: $e");
    }
  }
}
