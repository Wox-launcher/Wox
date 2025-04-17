import 'package:flutter/services.dart';
import 'package:uuid/v4.dart';
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
      case 'onWindowBlur':
        notifyWindowBlur();
        break;
      case 'log':
        // Log messages from native code
        final message = call.arguments as String;
        Logger.instance.info(const UuidV4().generate(), " [NATIVE] $message");
        break;
      default:
        Logger.instance.warn(const UuidV4().generate(), "Unhandled method call: ${call.method}");
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
      Logger.instance.error(const UuidV4().generate(), "Error setting window size: $e");
      rethrow;
    }
  }

  @override
  Future<Offset> getPosition() async {
    try {
      final Map<dynamic, dynamic> result = await _channel.invokeMethod('getPosition');
      return Offset(result['x'], result['y']);
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error getting position: $e");
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
      Logger.instance.error(const UuidV4().generate(), "Error setting position: $e");
      rethrow;
    }
  }

  @override
  Future<void> center(double? width, double height) async {
    try {
      await _channel.invokeMethod('center', {
        'width': width,
        'height': height,
      });
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error centering window: $e");
      rethrow;
    }
  }

  @override
  Future<void> show() async {
    try {
      await _channel.invokeMethod('show');
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error showing window: $e");
      rethrow;
    }
  }

  @override
  Future<void> hide() async {
    try {
      // add a delay to avoid the keyboard state issue on windows (let flutter handle the keyboard state as quickly as possible)
      // if we hide the window immediately, the keyboard state will be inconsistent
      // E.g. if you press the esc key to hide the window, the keyup event will be suppressed until the next show, so next time you press the esc key, the previous keyup event will be triggered, which in our case will not trigger the hide app action. you may need to press the esc key twice
      Future.delayed(const Duration(milliseconds: 100), () {
        Logger.instance.info(const UuidV4().generate(), "[HideWindow] Delayed execution after 100ms");
        _channel.invokeMethod('hide');
      });
    } catch (e) {
      rethrow;
    }
  }

  @override
  Future<void> focus() async {
    try {
      await _channel.invokeMethod('focus');
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error focusing window: $e");
      rethrow;
    }
  }

  @override
  Future<bool> isVisible() async {
    try {
      return await _channel.invokeMethod('isVisible');
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error checking visibility: $e");
      return false;
    }
  }

  @override
  Future<void> setAlwaysOnTop(bool alwaysOnTop) async {
    try {
      await _channel.invokeMethod('setAlwaysOnTop', alwaysOnTop);
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error setting always on top: $e");
      rethrow;
    }
  }

  @override
  Future<void> startDragging() async {
    try {
      await _channel.invokeMethod('startDragging');
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error starting window drag: $e");
    }
  }

  @override
  Future<void> waitUntilReadyToShow() async {
    try {
      await _channel.invokeMethod('waitUntilReadyToShow');
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error waiting until ready to show: $e");
      rethrow;
    }
  }
}
