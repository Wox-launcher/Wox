import 'package:flutter/foundation.dart';
import 'package:wox/utils/windows/window_manager_interface.dart';

/// Base implementation of WindowManagerInterface with common functionality
abstract class BaseWindowManager implements WindowManagerInterface {
  final ObserverList<WindowListener> _listeners = ObserverList<WindowListener>();

  @override
  void addListener(WindowListener listener) {
    _listeners.add(listener);
  }

  @override
  void removeListener(WindowListener listener) {
    _listeners.remove(listener);
  }

  /// Get a copy of the current listeners
  List<WindowListener> get listeners {
    final List<WindowListener> localListeners = List<WindowListener>.from(_listeners);
    return localListeners;
  }

  /// Notify all listeners of window blur
  void notifyWindowBlur() {
    for (final WindowListener listener in listeners) {
      listener.onWindowBlur();
    }
  }
}
