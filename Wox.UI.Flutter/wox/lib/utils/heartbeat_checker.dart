import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';

class HeartbeatChecker {
  int failedAttempts = 0;
  static const int maxFailedAttempts = 3;

  void startChecking() {
    Timer.periodic(const Duration(seconds: 1), (Timer timer) async {
      bool isAlive = await checkHeartbeat();

      if (!isAlive) {
        failedAttempts++;
        if (failedAttempts >= maxFailedAttempts) {
          timer.cancel();
          exit(0);
        }
      } else {
        failedAttempts = 0;
      }
    });
  }

  Future<bool> checkHeartbeat() async {
    try {
      var res = await Dio().get("http://localhost:${Env.serverPort}/ping");
      if (res.statusCode == 200) {
        return true;
      }
    } catch (e) {
      Logger.instance.info("Failed to check heartbeat: $e");
      return false;
    }

    return false;
  }
}
