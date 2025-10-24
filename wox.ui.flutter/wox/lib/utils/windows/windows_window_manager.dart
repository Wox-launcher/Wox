import 'package:flutter/services.dart';
import 'package:uuid/v4.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/windows/base_window_manager.dart';

/// Modifier key states from Windows
class WindowsModifierKeyStates {
  final bool isShiftPressed;
  final bool isControlPressed;
  final bool isAltPressed;
  final bool isMetaPressed;

  WindowsModifierKeyStates({
    required this.isShiftPressed,
    required this.isControlPressed,
    required this.isAltPressed,
    required this.isMetaPressed,
  });
}

/// Callback type for keyboard events from Windows message loop
typedef WindowsKeyboardEventCallback = void Function(
  String eventType,
  int keyCode,
  int scanCode,
  WindowsModifierKeyStates modifierStates,
);

/// Windows implementation of the window manager
class WindowsWindowManager extends BaseWindowManager {
  static const _channel = MethodChannel('com.wox.windows_window_manager');

  static final WindowsWindowManager instance = WindowsWindowManager._();

  /// Keyboard event listeners
  final List<WindowsKeyboardEventCallback> _keyboardEventListeners = [];

  /// Current modifier key states (updated from Windows message loop)
  WindowsModifierKeyStates _currentModifierStates = WindowsModifierKeyStates(
    isShiftPressed: false,
    isControlPressed: false,
    isAltPressed: false,
    isMetaPressed: false,
  );

  /// Get current modifier key states
  WindowsModifierKeyStates get currentModifierStates => _currentModifierStates;

  WindowsWindowManager._() {
    // Set up method call handler for events from native
    _channel.setMethodCallHandler(_handleMethodCall);
  }

  /// Add a keyboard event listener
  void addKeyboardEventListener(WindowsKeyboardEventCallback callback) {
    _keyboardEventListeners.add(callback);
  }

  /// Remove a keyboard event listener
  void removeKeyboardEventListener(WindowsKeyboardEventCallback callback) {
    _keyboardEventListeners.remove(callback);
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
      case 'onKeyboardEvent':
        // Handle keyboard events from Windows message loop
        final eventData = call.arguments as Map<dynamic, dynamic>;
        final eventType = eventData['type'] as String;
        final keyCode = eventData['keyCode'] as int;
        final scanCode = eventData['scanCode'] as int;

        // Get modifier key states and update current state
        _currentModifierStates = WindowsModifierKeyStates(
          isShiftPressed: eventData['isShiftPressed'] as bool? ?? false,
          isControlPressed: eventData['isControlPressed'] as bool? ?? false,
          isAltPressed: eventData['isAltPressed'] as bool? ?? false,
          isMetaPressed: eventData['isMetaPressed'] as bool? ?? false,
        );

        // Notify all listeners
        for (final listener in _keyboardEventListeners) {
          listener(eventType, keyCode, scanCode, _currentModifierStates);
        }
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
      await _channel.invokeMethod('hide');
    } catch (e) {
      Logger.instance.error(const UuidV4().generate(), "Error hiding window: $e");
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
