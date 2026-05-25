import 'package:flutter/material.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window.dart';
import 'package:wox/utils/windows/window_manager.dart';

abstract class WoxWindowDriver {
  bool get isPrimary;

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

  WoxMultipleWindowHandle? _handle;
  bool _visible = true;
  Offset _position = Offset.zero;
  Size _size;

  void attachHandle(WoxMultipleWindowHandle handle) {
    _handle = handle;
  }

  @override
  bool get isPrimary => false;

  @override
  Future<bool> isVisible() async => _visible;

  @override
  Future<void> show() async {
    _visible = true;
    await _handle?.show();
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
  }

  @override
  Future<void> close() async {
    _visible = false;
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
}
