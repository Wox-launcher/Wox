import 'package:flutter/material.dart';

/// Listener for window events
mixin WindowListener {
  void onWindowFocus() {}
  void onWindowBlur() {}
  void onWindowMaximize() {}
  void onWindowUnmaximize() {}
  void onWindowMinimize() {}
  void onWindowRestore() {}
  void onWindowResize() {}
  void onWindowMove() {}
  void onWindowClose() {}
}

/// Interface for window manager implementations
abstract class WindowManagerInterface {
  /// Set the window size
  Future<void> setSize(Size size);

  /// Get the window position
  Future<Offset> getPosition();

  /// Set the window position
  Future<void> setPosition(Offset position);

  /// Center the window on the screen
  Future<void> center();

  /// Show the window
  Future<void> show();

  /// Hide the window
  Future<void> hide();

  /// Focus the window
  Future<void> focus();

  /// Check if the window is visible
  Future<bool> isVisible();

  /// Set whether the window is always on top
  Future<void> setAlwaysOnTop(bool alwaysOnTop);

  /// Wait until the window is ready to show
  Future<void> waitUntilReadyToShow();

  /// Add a window event listener
  void addListener(WindowListener listener);

  /// Remove a window event listener
  void removeListener(WindowListener listener);
}
