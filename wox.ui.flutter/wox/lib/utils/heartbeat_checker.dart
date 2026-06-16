import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:uuid/v4.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';

class HeartbeatChecker {
  HeartbeatChecker._privateConstructor();

  static final HeartbeatChecker _instance = HeartbeatChecker._privateConstructor();

  factory HeartbeatChecker() => _instance;

  int failedAttempts = 0;
  static const int maxFailedAttempts = 3;
  Timer? _timer;

  void startChecking() {
    init();
    final traceId = const UuidV4().generate();
    _timer = Timer.periodic(const Duration(seconds: 1), (Timer timer) async {
      bool isAlive = await checkHeartbeat(traceId);

      if (!isAlive) {
        failedAttempts++;
        if (failedAttempts >= maxFailedAttempts) {
          Logger.instance.error(traceId, "Server is not alive, exiting...");
          timer.cancel();
          _timer = null;
          exit(0);
        }
      } else {
        failedAttempts = 0;
      }
    });
  }

  void init() {
    _timer?.cancel();
    _timer = null;
    failedAttempts = 0;
  }

  Future<bool> checkHeartbeat(String traceId) async {
    if (Env.isDev) {
      return true;
    }

    try {
      var res = await Dio().get("http://127.0.0.1:${Env.serverPort}/ping");
      if (res.statusCode == 200) {
        return true;
      }
    } catch (e) {
      Logger.instance.error(traceId, "Failed to check heartbeat: $e");
      return false;
    }

    return false;
  }
}
