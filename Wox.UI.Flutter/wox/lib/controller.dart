import 'dart:convert';

import 'package:flutter/widgets.dart';
import 'package:get/get.dart';
import 'package:logger/logger.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entities/show_app_params.dart';
import 'package:wox/entities/websocket_msg.dart';

class WoxController extends GetxController {
  final query = "".obs;
  final queryTextFieldController = TextEditingController();
  late final WebSocketChannel channel;

  void connect() {
    var channel = WebSocketChannel.connect(Uri.parse("ws://localhost:34987/ws"));
    channel.stream.listen((event) {
      var msg = WebsocketMsg.fromJson(jsonDecode(event));
      if (msg.type == "ToggleApp") {
        toggleApp(msg.data);
      }
    });
  }

  Future<void> toggleApp(ShowAppParams params) async {
    Logger().i("Toggle app");
    var isVisible = await windowManager.isVisible();
    if (isVisible) {
      hide();
    } else {
      show(params);
    }
  }

  void hide() {
    windowManager.hide();
  }

  void show(ShowAppParams params) {
    windowManager.show();
    windowManager.focus();
    if (params.position != null) {
      windowManager.setPosition(Offset(params.position!.x as double, params.position!.y as double));
    }
  }

  void onQueryChanged(String value) {
    query.value = value;
    Logger().i("Query changed: $value");

    windowManager.setSize(Size(800, (value.length + 1) * 100), animate: false);
  }
}
