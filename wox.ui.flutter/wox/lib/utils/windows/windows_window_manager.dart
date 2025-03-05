import 'package:flutter/services.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/base_window_manager.dart';

/// Windows implementation of the window manager
class WindowsWindowManager extends BaseWindowManager {
  static const _channel = MethodChannel('com.wox.windows_window_manager');

  static final WindowsWindowManager instance = WindowsWindowManager._();

  WindowsWindowManager._() {
    // Set up method call handler for events from native
    _channel.setMethodCallHandler(_handleMethodCall);
  }

  /// Handle method calls from native code
  Future<dynamic> _handleMethodCall(MethodCall call) async {
    switch (call.method) {
      case 'onWindowFocus':
        notifyWindowFocus();
        break;
      case 'onWindowBlur':
        notifyWindowBlur();
        break;
      case 'onWindowMaximize':
        notifyWindowMaximize();
        break;
      case 'onWindowMinimize':
        notifyWindowMinimize();
        break;
      case 'onWindowResize':
        notifyWindowResize();
        break;
      case 'onWindowMove':
        notifyWindowMove();
        break;
      case 'onWindowClose':
        notifyWindowClose();
        break;
      default:
        Logger.instance.warn("WindowsWindowManager", "Unhandled method call: ${call.method}");
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
      Logger.instance.error("WindowsWindowManager", "Error setting window size: $e");
      rethrow;
    }
  }

  @override
  Future<Offset> getPosition() async {
    try {
      final Map<dynamic, dynamic> result = await _channel.invokeMethod('getPosition');
      return Offset(result['x'], result['y']);
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error getting position: $e");
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
      Logger.instance.error("WindowsWindowManager", "Error setting position: $e");
      rethrow;
    }
  }

  @override
  Future<void> center() async {
    try {
      await _channel.invokeMethod('center');
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error centering window: $e");
      rethrow;
    }
  }

  @override
  Future<void> show() async {
    try {
      await _channel.invokeMethod('show');
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error showing window: $e");
      rethrow;
    }
  }

  @override
  Future<void> hide() async {
    try {
      await _channel.invokeMethod('hide');
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error hiding window: $e");
      rethrow;
    }
  }

  @override
  Future<void> focus() async {
    try {
      await _channel.invokeMethod('focus');
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error focusing window: $e");
      rethrow;
    }
  }

  @override
  Future<bool> isVisible() async {
    try {
      return await _channel.invokeMethod('isVisible');
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error checking visibility: $e");
      return false;
    }
  }

  @override
  Future<void> setAlwaysOnTop(bool alwaysOnTop) async {
    try {
      await _channel.invokeMethod('setAlwaysOnTop', alwaysOnTop);
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error setting always on top: $e");
      rethrow;
    }
  }

  @override
  Future<void> waitUntilReadyToShow() async {
    try {
      await _channel.invokeMethod('waitUntilReadyToShow', {
        'width': 800.0,
        'height': 600.0,
        'center': true,
        'alwaysOnTop': false,
      });
    } catch (e) {
      Logger.instance.error("WindowsWindowManager", "Error waiting until ready to show: $e");
      rethrow;
    }
  }
}
