import 'dart:async';
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
  WoxFileOutput? _output;
  bool _isInitialized = false;
  String logLevel = "INFO";

  Logger._privateConstructor();

  Future<void> initLogger() async {
    _output = WoxFileOutput();
    _logger = xlogger.Logger(printer: xlogger.SimplePrinter(printTime: true, colors: false), output: _output);
    _isInitialized = true;
  }

  static final Logger _instance = Logger._privateConstructor();

  static Logger get instance => _instance;

  void info(String traceId, String msg) {
    log(traceId, "info", msg);
  }

  void error(String traceId, String msg) {
    log(traceId, "error", msg);
  }

  void crash(String traceId, String msg) {
    // Bug fix: crash diagnostics are the only logs that should force a sync
    // disk write. Regular error logs can be frequent during plugin failures, so
    // tying every error to writeAsStringSync would still block the UI isolate.
    log(traceId, "error", msg, syncToDisk: true);
  }

  void warn(String traceId, String msg) {
    log(traceId, "warn", msg);
  }

  void debug(String traceId, String msg) {
    log(traceId, "debug", msg);
  }

  void log(String traceId, String level, String message, {bool syncToDisk = false}) {
    if (!_isInitialized) {
      return;
    }

    if (!shouldLog(level)) {
      return;
    }

    if (syncToDisk) {
      _output?.writeNextLogSynchronously();
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

  Future<void> flush() async {
    try {
      await _output?.flush();
    } catch (_) {
      // Crash handlers must not create another uncaught error while trying to persist diagnostics.
    }
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
  late File logFile;
  IOSink? _sink;
  Future<void>? _flushFuture;
  bool _writeNextLogSynchronously = false;

  WoxFileOutput() {
    logFile = File(path.join(getHomeDir(), ".wox", "log", 'ui.log'));
    logFile.createSync(recursive: true);
    _sink = logFile.openWrite(mode: FileMode.append);
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
    final content = "${event.lines.join('\n')}\n";
    if (_writeNextLogSynchronously) {
      // Bug fix: synchronous writes are now opt-in for crash paths instead of
      // inferred from "[error]". Error storms should stay buffered so they do
      // not freeze typing, while crash handlers can still persist one line
      // before the process exits.
      _writeNextLogSynchronously = false;
      logFile.writeAsStringSync(content, mode: FileMode.append, flush: true);
      return;
    }

    // Optimization: buffered IOSink writes keep frequent info/debug logs off the
    // UI isolate's blocking path while preserving the existing ui.log stream for
    // normal diagnostics.
    _sink?.write(content);
  }

  void writeNextLogSynchronously() {
    _writeNextLogSynchronously = true;
  }

  Future<void> flush() async {
    final activeFlush = _flushFuture;
    if (activeFlush != null) {
      return activeFlush;
    }

    final sink = _sink;
    if (sink == null) {
      return;
    }

    late final Future<void> guardedFlush;
    guardedFlush = sink.flush().whenComplete(() {
      if (identical(_flushFuture, guardedFlush)) {
        _flushFuture = null;
      }
    });
    _flushFuture = guardedFlush;
    await guardedFlush;
  }
}
