import 'dart:io';

import 'package:flutter/services.dart';
import 'package:wox/utils/env.dart';

typedef PermissionFlowRefreshListener = void Function();

class MacOSPermissionFlowBridge {
  MacOSPermissionFlowBridge._() {
    if (Platform.isMacOS) {
      _channel.setMethodCallHandler(_handleNativeCall);
    }
  }

  static final MacOSPermissionFlowBridge instance = MacOSPermissionFlowBridge._();
  static const MethodChannel _channel = MethodChannel('com.wox.macos_permission_flow');
  final Set<PermissionFlowRefreshListener> _refreshListeners = {};

  void addRefreshListener(PermissionFlowRefreshListener listener) => _refreshListeners.add(listener);

  void removeRefreshListener(PermissionFlowRefreshListener listener) => _refreshListeners.remove(listener);

  Future<void> open({
    required String permissionType,
    required String title,
    required String rightInstruction,
    required String bottomInstruction,
    required String manualInstruction,
  }) async {
    if (!Platform.isMacOS) return;
    await _channel.invokeMethod<void>('openPermissionFlow', {
      'permissionType': permissionType,
      'corePid': Env.serverPid,
      'title': title,
      'rightInstruction': rightInstruction,
      'bottomInstruction': bottomInstruction,
      'manualInstruction': manualInstruction,
    });
  }

  Future<dynamic> _handleNativeCall(MethodCall call) async {
    if (call.method == 'permissionFlowClosed' || call.method == 'applicationActivated' || call.method == 'permissionStatusRefreshRequested') {
      for (final listener in List<PermissionFlowRefreshListener>.of(_refreshListeners)) {
        listener();
      }
    }
  }
}
