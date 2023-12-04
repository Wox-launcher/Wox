import 'dart:io';

import 'package:logger/logger.dart' as xlogger;
import 'package:path/path.dart' as path;

class Logger {
  late xlogger.Logger _logger;

  Logger._privateConstructor();

  Future<void> initLogger() async {
    _logger = xlogger.Logger(
      printer: xlogger.SimplePrinter(printTime: true, colors: false),
      output: WoxFileOutput(),
    );
  }

  static final Logger _instance = Logger._privateConstructor();

  static Logger get instance => _instance;

  void info(String msg) {
    _logger.i(msg);
  }

  void error(String msg) {
    _logger.e(msg);
  }
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
