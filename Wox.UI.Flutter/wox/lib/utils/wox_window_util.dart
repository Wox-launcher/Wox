import 'dart:convert';
import 'dart:ui';

import 'package:desktop_multi_window/desktop_multi_window.dart';
import 'package:wox/utils/env.dart';

class WoxWindowUtil {
  late final Map<String, WindowController> _windowMap = {};
  late final List<int> _windowIdList = [];

  WoxWindowUtil._privateConstructor();

  static final WoxWindowUtil _instance = WoxWindowUtil._privateConstructor();

  static WoxWindowUtil get instance => _instance;

  Future<void> createWindow(String name, Map<String, dynamic> params) async {
    params["serverPort"] = Env.serverPort;
    params["serverPid"] = Env.serverPid;
    params["isDev"] = Env.isDev;
    final window = await DesktopMultiWindow.createWindow(jsonEncode(params));
    _windowMap[name] = window;
    _windowIdList.add(window.windowId);
  }

  void showWindow(String name, String title, Size size) {
    if (_windowMap.containsKey(name) && _windowMap[name] != null) {
      _windowMap[name]
        ?..setFrame(const Offset(0, 0) & size)
        ..center()
        ..setTitle(title)
        ..resizable(false)
        ..show();
    }
  }

  bool isMultiWindow(List<String> arguments) {
    if (arguments.isEmpty) {
      return false;
    }
    return arguments[0] == "multi_window";
  }
}
