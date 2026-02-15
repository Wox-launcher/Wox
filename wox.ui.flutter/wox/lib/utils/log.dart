import 'dart:io';

import 'package:logger/logger.dart' as xlogger;
import 'package:path/path.dart' as path;
import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/enums/wox_msg_type_enum.dart';

import 'wox_websocket_msg_util.dart';

class Logger {
  late xlogger.Logger _logger;
  String logLevel = "INFO";

  Logger._privateConstructor();

  Future<void> initLogger() async {
    _logger = xlogger.Logger(printer: xlogger.SimplePrinter(printTime: true, colors: false), output: WoxFileOutput());
  }

  static final Logger _instance = Logger._privateConstructor();

  static Logger get instance => _instance;

  void info(String traceId, String msg) {
    log(traceId, "info", msg);
  }

  void error(String traceId, String msg) {
    log(traceId, "error", msg);
  }

  void warn(String traceId, String msg) {
    log(traceId, "warn", msg);
  }

  void debug(String traceId, String msg) {
    log(traceId, "debug", msg);
  }

  void log(String traceId, String level, String message) {
    if (!shouldLog(level)) {
      return;
    }

    _logger.i("$traceId [$level] $message");

    try {
      sendLog(traceId, level, message);
    } catch (e) {
      _logger.e("$traceId [$level] Failed to send log: $e");
    }
  }

  void setLogLevel(String level) {
    final normalized = normalizeLogLevel(level);
    logLevel = normalized;
    _logger.i("[LOGGER] log level set to $normalized");
  }

  String normalizeLogLevel(String level) {
    final normalized = level.trim().toUpperCase();
    if (normalized == "DEBUG") {
      return "DEBUG";
    }
    return "INFO";
  }

  bool shouldLog(String level) {
    final threshold = logLevel == "DEBUG" ? 10 : 20;
    return logPriority(level) >= threshold;
  }

  int logPriority(String level) {
    switch (level.trim().toLowerCase()) {
      case "debug":
        return 10;
      case "info":
        return 20;
      case "warn":
        return 30;
      case "error":
        return 40;
      default:
        return 20;
    }
  }

  void sendLog(String traceId, String level, String message) {
    if (WoxWebsocketMsgUtil.instance.isConnected()) {
      final msg = WoxWebsocketMsg(
        requestId: const UuidV4().generate(),
        traceId: traceId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code,
        method: WoxMsgMethodEnum.WOX_MSG_METHOD_Log.code,
        data: {"traceId": traceId, "level": level, "message": message},
      );
      WoxWebsocketMsgUtil.instance.sendMessage(msg);
    }
  }
}

class LoggerSwitch {
  static bool enablePaintLog = false;
  static bool enableSizeAndPositionLog = false;
}

class WoxFileOutput extends xlogger.LogOutput {
  late IOSink sink;

  WoxFileOutput() {
    var logFile = File(path.join(getHomeDir(), ".wox", "log", 'ui.log'));
    logFile.createSync(recursive: true);
    sink = logFile.openWrite(mode: FileMode.append);
  }

  String getHomeDir() {
    if (Platform.isWindows) {
      return Platform.environment['UserProfile']!;
    } else {
      return Platform.environment['HOME']!;
    }
  }

  @override
  void output(xlogger.OutputEvent event) {
    for (var element in event.lines) {
      sink.writeln(element);
    }
  }
}
