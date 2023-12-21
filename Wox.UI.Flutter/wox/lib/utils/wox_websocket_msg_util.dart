import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/utils/log.dart';

class WoxWebsocketMsgUtil {
  WoxWebsocketMsgUtil._privateConstructor();

  static final WoxWebsocketMsgUtil _instance = WoxWebsocketMsgUtil._privateConstructor();

  static WoxWebsocketMsgUtil get instance => _instance;

  WebSocketChannel? _channel;

  late Uri uri;

  late Function onMessageReceived;

  int connectionAttempts = 1;

  final Map<String, Completer> _completers = {};

  void _connect() {
    _channel?.sink.close();
    _channel = null;

    _channel = WebSocketChannel.connect(uri);
    _channel!.stream.listen(
      (event) {
        var msg = WoxWebsocketMsg.fromJson(jsonDecode(event));
        if (msg.success == false) {
          Logger.instance.error("Received error message: ${msg.toJson()}");
          return;
        }

        if (_completers.containsKey(msg.id)) {
          _completers[msg.id]!.complete(msg);
          _completers.remove(msg.id);
          return;
        }

        onMessageReceived(msg);
      },
      onDone: () {
        _reconnect();
      },
    );
  }

  void _reconnect() {
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
  Future<dynamic> sendMessage(WoxWebsocketMsg msg) async {
    // if query message, send it directly, no need to wait for response
    // because query result may return multiple times
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
      _channel?.sink.add(jsonEncode(msg));
      return;
    }

    Completer completer = Completer();
    _completers[msg.id] = completer;
    _channel?.sink.add(jsonEncode(msg));
    var responseMsg = await completer.future as WoxWebsocketMsg;
    return responseMsg.data;
  }
}
