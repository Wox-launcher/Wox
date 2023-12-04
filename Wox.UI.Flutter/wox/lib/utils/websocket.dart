import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/utils/log.dart';

class WoxWebsocket {
  WebSocketChannel? channel;
  final Uri uri;

  Function onMessageReceived;

  WoxWebsocket(this.uri, {required this.onMessageReceived});

  void connect() {
    channel?.sink.close();
    channel = null;

    channel = WebSocketChannel.connect(uri);
    channel!.stream.listen(
      (event) {
        onMessageReceived(event);
      },
      onDone: () {
        _reconnect();
      },
    );
  }

  void sendMessage(WoxWebsocketMsg msg) {
    channel?.sink.add(jsonEncode(msg));
  }

  void _reconnect() {
    Future.delayed(const Duration(seconds: 1), () {
      Logger.instance.info("Attempting to reconnect to WebSocket...");
      connect();
    });
  }
}
