import 'dart:io';
import 'dart:ui';

import 'package:wox/utils/windows/linux_system_input.dart';
import 'package:wox/utils/windows/macos_system_input.dart';
import 'package:wox/utils/windows/system_input_interface.dart';
import 'package:wox/utils/windows/windows_system_input.dart';

class SystemInput extends SystemInputInterface {
  static final SystemInput instance = SystemInput._();

  late final SystemInputInterface _platformImpl;

  SystemInput._() {
    if (Platform.isLinux) {
      _platformImpl = LinuxSystemInput.instance;
    } else if (Platform.isMacOS) {
      _platformImpl = MacOSSystemInput.instance;
    } else if (Platform.isWindows) {
      _platformImpl = WindowsSystemInput.instance;
    } else {
      throw UnsupportedError('Unsupported platform: ${Platform.operatingSystem}');
    }
  }

  @override
  Future<void> keyDown(String key) {
    return _platformImpl.keyDown(key);
  }

  @override
  Future<void> keyUp(String key) {
    return _platformImpl.keyUp(key);
  }

  @override
  Future<void> moveMouse(Offset position) {
    return _platformImpl.moveMouse(position);
  }

  @override
  Future<void> mouseDown(SystemMouseButton button) {
    return _platformImpl.mouseDown(button);
  }

  @override
  Future<void> mouseUp(SystemMouseButton button) {
    return _platformImpl.mouseUp(button);
  }
}

final systemInput = SystemInput.instance;
