import 'dart:io';

import 'package:logger/logger.dart' as xlogger;
import 'package:path/path.dart' as path;

class Logger {
  late xlogger.Logger _logger;

  Logger._privateConstructor();

  Future<void> initLogger() async {
    var logPath = path.join(path.absolute(Platform.environment['HOME']!, ".wox", "log", "ui.log"));
    var file = await File(logPath).create(recursive: true);
    _logger = xlogger.Logger(
      printer: xlogger.SimplePrinter(),
      output: xlogger.FileOutput(file: file),
    );
  }

  static final Logger _instance = Logger._privateConstructor();

  static Logger get instance => _instance;

  void info(String msg) {
    _logger.i(msg);
  }
}
