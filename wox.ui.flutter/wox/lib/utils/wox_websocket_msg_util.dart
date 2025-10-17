import 'dart:async';
import 'dart:convert';

import 'package:uuid/v4.dart';
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

  bool isConnecting = false;

  final Map<String, Completer> _completers = {};

  void _connect() {
    _channel?.sink.close();
    _channel = null;

    _channel = WebSocketChannel.connect(uri);
    _channel!.stream.listen(
      (event) {
        isConnecting = false;
        var msg = WoxWebsocketMsg.fromJson(jsonDecode(event));
        if (msg.success == false) {
          Logger.instance.error(msg.traceId, "Received error websocket message: ${msg.toJson()}");
          return;
        }

        if (_completers.containsKey(msg.requestId)) {
          _completers[msg.requestId]!.complete(msg);
          _completers.remove(msg.requestId);
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
      Logger.instance.info(const UuidV4().generate(), "Attempting to reconnect to WebSocket... $connectionAttempts");
      isConnecting = true;
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

  bool isConnected() {
    return _channel != null && isConnecting == false;
  }

  // send message to websocket server
  Future<dynamic> sendMessage(WoxWebsocketMsg msg) async {
    // if query message, send it directly, no need to wait for response
    // because query result may return multiple times
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
      msg.sendTimestamp = DateTime.now().millisecondsSinceEpoch;
      _channel?.sink.add(jsonEncode(msg));
      return;
    }

    Completer completer = Completer();
    _completers[msg.requestId] = completer;
    _channel?.sink.add(jsonEncode(msg));
    var responseMsg = await completer.future as WoxWebsocketMsg;
    return responseMsg.data;
  }
}
