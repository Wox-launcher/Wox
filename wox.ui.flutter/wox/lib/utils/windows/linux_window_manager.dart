import 'package:flutter/services.dart';
import 'package:uuid/v4.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/base_window_manager.dart';

/// Linux implementation of the window manager
class LinuxWindowManager extends BaseWindowManager {
  static const _channel = MethodChannel('com.wox.linux_window_manager');

  static final LinuxWindowManager instance = LinuxWindowManager._();

  LinuxWindowManager._() {
    // Set up method call handler for events from native
    _channel.setMethodCallHandler(_handleMethodCall);
  }

  /// Handle method calls from native code
  Future<dynamic> _handleMethodCall(MethodCall call) async {
    switch (call.method) {
      case 'onWindowBlur':
        notifyWindowBlur();
        break;
      default:
        Logger.instance.warn(
          const UuidV4().generate(),
          "Unhandled method call: ${call.method}",
        );
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
      Logger.instance.error(
        const UuidV4().generate(),
        "Error setting window size: $e",
      );
    }
  }

  @override
  Future<void> setBounds(Offset position, Size size) async {
    try {
      await _channel.invokeMethod('setBounds', {
        'x': position.dx,
        'y': position.dy,
        'width': size.width,
        'height': size.height,
      });
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error setting window bounds: $e",
      );
    }
  }

  @override
  Future<Offset> getPosition() async {
    try {
      final Map<dynamic, dynamic> result = await _channel.invokeMethod(
        'getPosition',
      );
      return Offset(
        double.parse(result['x'].toString()),
        double.parse(result['y'].toString()),
      );
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error getting position: $e",
      );
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
      Logger.instance.error(
        const UuidV4().generate(),
        "Error setting position: $e",
      );
    }
  }

  @override
  Future<void> center(double width, double height) async {
    try {
      await _channel.invokeMethod('center', {'width': width, 'height': height});
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error centering window: $e",
      );
    }
  }

  @override
  Future<void> show() async {
    try {
      await _channel.invokeMethod('show');
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error showing window: $e",
      );
    }
  }

  @override
  Future<void> hide() async {
    try {
      await _channel.invokeMethod('hide');
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error hiding window: $e",
      );
    }
  }

  @override
  Future<void> focus() async {
    try {
      await _channel.invokeMethod('focus');
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error focusing window: $e",
      );
    }
  }

  @override
  Future<bool> isVisible() async {
    try {
      return await _channel.invokeMethod('isVisible');
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error checking visibility: $e",
      );
      return false;
    }
  }

  @override
  Future<void> setAlwaysOnTop(bool alwaysOnTop) async {
    try {
      await _channel.invokeMethod('setAlwaysOnTop', alwaysOnTop);
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error setting always on top: $e",
      );
    }
  }

  @override
  Future<void> setAppearance(String appearance) async {
    // Not implemented for Linux
  }

  @override
  Future<void> startDragging() async {
    try {
      await _channel.invokeMethod('startDragging');
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error starting window drag: $e",
      );
    }
  }

  @override
  Future<void> waitUntilReadyToShow() async {
    try {
      await _channel.invokeMethod('waitUntilReadyToShow');
    } catch (e) {
      Logger.instance.error(
        const UuidV4().generate(),
        "Error waiting until ready to show: $e",
      );
    }
  }
}
