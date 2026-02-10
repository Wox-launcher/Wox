import 'package:flutter/material.dart';

/// Listener for window events
mixin WindowListener {
  void onWindowBlur() {}
}

/// Interface for window manager implementations
abstract class WindowManagerInterface {
  /// Set the window size
  Future<void> setSize(Size size);

  /// Set window position and size at once
  Future<void> setBounds(Offset position, Size size);

  /// Get the window position
  Future<Offset> getPosition();

  /// Get the window size
  Future<Size> getSize();

  /// Set the window position
  Future<void> setPosition(Offset position);

  /// Center the window on the screen
  Future<void> center(double width, double height);

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

  /// Set window appearance (light/dark)
  /// appearance: "light" or "dark"
  Future<void> setAppearance(String appearance);

  /// Wait until the window is ready to show
  Future<void> waitUntilReadyToShow();

  /// Start dragging the window
  Future<void> startDragging();

  /// Add a window event listener
  void addListener(WindowListener listener);

  /// Remove a window event listener
  void removeListener(WindowListener listener);
}
