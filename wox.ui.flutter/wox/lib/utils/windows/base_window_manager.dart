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

  /// Notify all listeners of window focus
  void notifyWindowFocus() {
    for (final WindowListener listener in listeners) {
      listener.onWindowFocus();
    }
  }

  /// Notify all listeners of window blur
  void notifyWindowBlur() {
    for (final WindowListener listener in listeners) {
      listener.onWindowBlur();
    }
  }

  /// Notify all listeners of window maximize
  void notifyWindowMaximize() {
    for (final WindowListener listener in listeners) {
      listener.onWindowMaximize();
    }
  }

  /// Notify all listeners of window unmaximize
  void notifyWindowUnmaximize() {
    for (final WindowListener listener in listeners) {
      listener.onWindowUnmaximize();
    }
  }

  /// Notify all listeners of window minimize
  void notifyWindowMinimize() {
    for (final WindowListener listener in listeners) {
      listener.onWindowMinimize();
    }
  }

  /// Notify all listeners of window restore
  void notifyWindowRestore() {
    for (final WindowListener listener in listeners) {
      listener.onWindowRestore();
    }
  }

  /// Notify all listeners of window resize
  void notifyWindowResize() {
    for (final WindowListener listener in listeners) {
      listener.onWindowResize();
    }
  }

  /// Notify all listeners of window move
  void notifyWindowMove() {
    for (final WindowListener listener in listeners) {
      listener.onWindowMove();
    }
  }

  /// Notify all listeners of window close
  void notifyWindowClose() {
    for (final WindowListener listener in listeners) {
      listener.onWindowClose();
    }
  }
}
