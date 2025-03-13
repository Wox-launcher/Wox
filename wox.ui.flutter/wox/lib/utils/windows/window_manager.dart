import 'dart:io';
import 'package:flutter/material.dart';
import 'package:wox/utils/windows/linux_window_manager.dart';
import 'package:wox/utils/windows/macos_window_manager.dart';
import 'package:wox/utils/windows/window_manager_interface.dart';
import 'package:wox/utils/windows/windows_window_manager.dart';

/// Main window manager class that delegates to platform-specific implementations
class WindowManager implements WindowManagerInterface {
  static final WindowManager instance = WindowManager._();

  late final WindowManagerInterface _platformImpl;

  WindowManager._() {
    if (Platform.isLinux) {
      _platformImpl = LinuxWindowManager.instance;
    } else if (Platform.isMacOS) {
      _platformImpl = MacOSWindowManager.instance;
    } else if (Platform.isWindows) {
      _platformImpl = WindowsWindowManager.instance;
    } else {
      throw UnsupportedError('Unsupported platform: ${Platform.operatingSystem}');
    }
  }

  @override
  Future<void> setSize(Size size) {
    return _platformImpl.setSize(size);
  }

  @override
  Future<Offset> getPosition() {
    return _platformImpl.getPosition();
  }

  @override
  Future<void> setPosition(Offset position) {
    return _platformImpl.setPosition(position);
  }

  @override
  Future<void> center(double width, double height) {
    return _platformImpl.center(width, height);
  }

  @override
  Future<void> show() {
    return _platformImpl.show();
  }

  @override
  Future<void> hide() {
    return _platformImpl.hide();
  }

  @override
  Future<void> focus() {
    return _platformImpl.focus();
  }

  @override
  Future<bool> isVisible() {
    return _platformImpl.isVisible();
  }

  @override
  Future<void> setAlwaysOnTop(bool alwaysOnTop) {
    return _platformImpl.setAlwaysOnTop(alwaysOnTop);
  }

  @override
  Future<void> startDragging() {
    return _platformImpl.startDragging();
  }

  @override
  Future<void> waitUntilReadyToShow() {
    return _platformImpl.waitUntilReadyToShow();
  }

  @override
  void addListener(WindowListener listener) {
    _platformImpl.addListener(listener);
  }

  @override
  void removeListener(WindowListener listener) {
    _platformImpl.removeListener(listener);
  }
}

/// Global instance of the window manager for easy access
final windowManager = WindowManager.instance;
