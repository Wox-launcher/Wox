import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window.dart';
import 'package:wox/utils/windows/window_manager.dart';

abstract class WoxWindowDriver {
  bool get isPrimary;

  Future<int?> getNativeHandle();

  Future<bool> isVisible();

  Future<void> show();

  Future<void> hide();

  Future<void> focus();

  Future<void> close();

  Future<void> setBounds(Offset position, Size size);

  Future<void> setSize(Size size);

  Future<Size> getSize();

  Future<Offset> getPosition();

  Future<void> setAlwaysOnTop(bool value);

  void startDragging();
}

class WoxPrimaryWindowDriver implements WoxWindowDriver {
  @override
  bool get isPrimary => true;

  @override
  Future<int?> getNativeHandle() => windowManager.getNativeHandle();

  @override
  Future<bool> isVisible() => windowManager.isVisible();

  @override
  Future<void> show() => windowManager.show();

  @override
  Future<void> hide() => windowManager.hide();

  @override
  Future<void> focus() => windowManager.focus();

  @override
  Future<void> close() => windowManager.hide();

  @override
  Future<void> setBounds(Offset position, Size size) => windowManager.setBounds(position, size);

  @override
  Future<void> setSize(Size size) => windowManager.setSize(size);

  @override
  Future<Size> getSize() => windowManager.getSize();

  @override
  Future<Offset> getPosition() => windowManager.getPosition();

  @override
  Future<void> setAlwaysOnTop(bool value) => windowManager.setAlwaysOnTop(value);

  @override
  void startDragging() {
    windowManager.startDragging();
  }
}

class WoxSecondaryWindowDriver implements WoxWindowDriver {
  WoxSecondaryWindowDriver({required Size initialSize}) : _size = initialSize;

  static const Duration _focusMonitorInterval = Duration(milliseconds: 120);
  static const Duration _focusMonitorGrace = Duration(milliseconds: 350);

  WoxMultipleWindowHandle? _handle;
  bool _visible = true;
  Offset _position = Offset.zero;
  Size _size;
  Timer? _focusMonitor;
  int _focusMonitorToken = 0;
  Future<void> Function()? _onBlur;

  void attachHandle(WoxMultipleWindowHandle handle) {
    _handle = handle;
  }

  void setOnBlur(Future<void> Function() onBlur) {
    _onBlur = onBlur;
  }

  @override
  bool get isPrimary => false;

  @override
  Future<int?> getNativeHandle() async => _handle?.nativeHandle;

  @override
  Future<bool> isVisible() async => _visible;

  @override
  Future<void> show() async {
    _visible = true;
    await _handle?.show();
    _startFocusMonitor();
  }

  @override
  Future<void> hide() async {
    // Secondary windows are transient query-owned surfaces. Hiding one ends
    // that instance so the next named open gets a fresh sessionId.
    await close();
  }

  @override
  Future<void> focus() async {
    _visible = true;
    await _handle?.focus();
    _startFocusMonitor();
  }

  @override
  Future<void> close() async {
    _visible = false;
    _stopFocusMonitor();
    await _handle?.close();
  }

  @override
  Future<void> setBounds(Offset position, Size size) async {
    _position = position;
    _size = size;
    await _handle?.setBounds(position, size);
  }

  @override
  Future<void> setSize(Size size) async {
    _size = size;
    await _handle?.setSize(size);
  }

  @override
  Future<Size> getSize() async => await _handle?.getSize() ?? _size;

  @override
  Future<Offset> getPosition() async => await _handle?.getPosition() ?? _position;

  @override
  Future<void> setAlwaysOnTop(bool value) async {
    await _handle?.setAlwaysOnTop(value);
  }

  @override
  void startDragging() {
    _handle?.startDragging();
  }

  void _startFocusMonitor() {
    if (!Platform.isWindows) {
      return;
    }

    final handle = _handle;
    final onBlur = _onBlur;
    if (handle == null || onBlur == null) {
      return;
    }

    _focusMonitor?.cancel();
    final token = ++_focusMonitorToken;
    final monitorStartedAt = DateTime.now();
    var hasObservedForeground = false;
    _focusMonitor = Timer.periodic(_focusMonitorInterval, (timer) {
      if (!_visible || token != _focusMonitorToken) {
        timer.cancel();
        return;
      }

      final isForeground = handle.isForegroundWindowOrChild;
      if (isForeground) {
        hasObservedForeground = true;
        return;
      }

      if (DateTime.now().difference(monitorStartedAt) < _focusMonitorGrace) {
        return;
      }

      if (!hasObservedForeground) {
        return;
      }

      timer.cancel();
      unawaited(onBlur());
    });
  }

  void _stopFocusMonitor() {
    _focusMonitorToken++;
    _focusMonitor?.cancel();
    _focusMonitor = null;
  }
}
