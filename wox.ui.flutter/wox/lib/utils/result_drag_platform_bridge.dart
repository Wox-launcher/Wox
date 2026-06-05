import 'dart:io';

import 'package:flutter/services.dart';
import 'package:wox/utils/log.dart';

enum ResultDragStatus {
  success,
  cancel,
  cancelInSource,
  error;

  static ResultDragStatus fromString(String? value) {
    switch (value) {
      case 'success':
        return ResultDragStatus.success;
      case 'cancel':
        return ResultDragStatus.cancel;
      case 'cancel_in_source':
        return ResultDragStatus.cancelInSource;
      default:
        return ResultDragStatus.error;
    }
  }
}

class ResultDragPlatformBridge {
  static final ResultDragPlatformBridge instance = ResultDragPlatformBridge._();

  static const MethodChannel _channel = MethodChannel('com.wox.result_drag');

  ResultDragPlatformBridge._();

  Future<ResultDragStatus> startFileDrag(String traceId, List<String> files) async {
    if ((!Platform.isWindows && !Platform.isMacOS) || files.isEmpty) {
      return ResultDragStatus.error;
    }

    try {
      final result = await _channel.invokeMethod<Map<dynamic, dynamic>>('startFileDrag', {'traceId': traceId, 'files': files});
      return ResultDragStatus.fromString(result?['status']?.toString());
    } on MissingPluginException {
      Logger.instance.warn(traceId, 'Result drag is not implemented on this platform');
      return ResultDragStatus.error;
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to start result file drag: $e');
      return ResultDragStatus.error;
    }
  }
}
