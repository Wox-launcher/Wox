import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/utils/log.dart';

class WoxWebsocketMsgUtil {
  WoxWebsocketMsgUtil._privateConstructor();

  static final WoxWebsocketMsgUtil _instance = WoxWebsocketMsgUtil._privateConstructor();

  static WoxWebsocketMsgUtil get instance => _instance;

  WebSocketChannel? _channel;

  bool connecting = false;

  late Uri uri;

  late Function onMessageReceived;

  int connectionAttempts = 1;

  bool _isConnected() {
    return _channel != null && _channel!.closeCode == null;
  }

  void _connect() {
    _channel?.sink.close();
    _channel = null;

    _channel = WebSocketChannel.connect(uri);
    _channel!.stream.listen(
      (event) {
        onMessageReceived(event);
      },
      onDone: () {
        if (!connecting && !_isConnected()) {
          _reconnect();
        }
      },
    );
    connecting = false;
  }

  void _reconnect() {
    connecting = true;
    Future.delayed(Duration(milliseconds: 200 * (connectionAttempts > 5 ? 5 : connectionAttempts)), () {
      Logger.instance.info("Attempting to reconnect to WebSocket... $connectionAttempts");
      connectionAttempts++;
      _connect();
    });
  }

  // before calling other methods, make sure to call initialize() first
  Future<void> initialize(Uri uri, {required Function onMessageReceived}) async {
    this.uri = uri;
    this.onMessageReceived = onMessageReceived;
    _connect();
  }

  // send message to websocket server
  void sendMessage(WoxWebsocketMsg msg) {
    _channel?.sink.add(jsonEncode(msg));
  }
}
