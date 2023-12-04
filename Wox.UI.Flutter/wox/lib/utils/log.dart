import 'dart:io';

import 'package:logger/logger.dart' as xlogger;
import 'package:path/path.dart' as path;

class Logger {
  late xlogger.Logger _logger;

  Logger._internal() {
    var logPath = path.join(path.absolute(Platform.environment['HOME']!, ".wox", "log", "ui.log"));
    _logger = xlogger.Logger(
      printer: xlogger.SimplePrinter(),
      output: xlogger.FileOutput(file: File(logPath)),
    );
  }

  static final Logger _instance = Logger._internal();

  static Logger get instance => _instance;

  void info(String msg) {
    _logger.i(msg);
  }
}
