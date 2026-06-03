import 'dart:async';
import 'dart:convert';

import 'package:uuid/v4.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';

class WoxWebsocketMsgUtil {
  WoxWebsocketMsgUtil._privateConstructor();

  static final WoxWebsocketMsgUtil _instance = WoxWebsocketMsgUtil._privateConstructor();

  static WoxWebsocketMsgUtil get instance => _instance;

  static const String _coreSessionPrefix = "core-";

  WebSocketChannel? _channel;
  StreamSubscription? _subscription;
  Timer? _reconnectTimer;

  late Uri uri;

  late Function onMessageReceived;

  int connectionAttempts = 1;

  bool isConnecting = false;
  bool _isDisposed = false;

  final Map<String, Completer> _completers = {};

  void _connect() {
    _reconnectTimer?.cancel();
    _subscription?.cancel();
    _channel?.sink.close();
    _channel = null;

    if (_isDisposed) {
      return;
    }

    _channel = WebSocketChannel.connect(uri);
    _subscription = _channel!.stream.listen(
      (event) {
        final eventReceivedMs = DateTime.now().millisecondsSinceEpoch;
        final eventReceivedUs = DateTime.now().microsecondsSinceEpoch;
        final payloadChars = event is String ? event.length : 0;
        isConnecting = false;
        connectionAttempts = 1;
        final jsonDecodeStartUs = DateTime.now().microsecondsSinceEpoch;
        final decoded = jsonDecode(event);
        final jsonDecodeUs = DateTime.now().microsecondsSinceEpoch - jsonDecodeStartUs;
        final fromJsonStartUs = DateTime.now().microsecondsSinceEpoch;
        var msg = WoxWebsocketMsg.fromJson(decoded);
        final fromJsonUs = DateTime.now().microsecondsSinceEpoch - fromJsonStartUs;
        if (Env.isDev && msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
          final backendToStreamMs = msg.sendTimestamp > 0 ? eventReceivedMs - msg.sendTimestamp : -1;
          Logger.instance.debug(
            msg.traceId,
            "query_timing source=ui stage=ui_websocket_stream_receive traceId=${msg.traceId} method=${msg.method} payloadChars=$payloadChars backendToStreamMs=$backendToStreamMs jsonDecodeUs=$jsonDecodeUs fromJsonUs=$fromJsonUs streamParseUs=${DateTime.now().microsecondsSinceEpoch - eventReceivedUs}",
          );
        }
        if (msg.sessionId.isNotEmpty && msg.sessionId != Env.sessionId && !msg.sessionId.startsWith(_coreSessionPrefix)) {
          return;
        }
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
        if (!_isDisposed) {
          _reconnect();
        }
      },
      onError: (_) {
        if (!_isDisposed) {
          _reconnect();
        }
      },
    );
  }

  void _reconnect() {
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(Duration(milliseconds: 200 * (connectionAttempts > 5 ? 5 : connectionAttempts)), () {
      if (_isDisposed) {
        return;
      }
      Logger.instance.info(const UuidV4().generate(), "Attempting to reconnect to WebSocket... $connectionAttempts");
      isConnecting = true;
      connectionAttempts++;
      _connect();
    });
  }

  // before calling other methods, make sure to call initialize() first
  Future<void> initialize(Uri uri, {required Function onMessageReceived}) async {
    await init();
    _isDisposed = false;
    this.uri = uri;
    this.onMessageReceived = onMessageReceived;
    _connect();
  }

  Future<void> init() async {
    _isDisposed = true;
    isConnecting = false;
    connectionAttempts = 1;

    _reconnectTimer?.cancel();
    _reconnectTimer = null;

    await _subscription?.cancel();
    _subscription = null;

    await _channel?.sink.close();
    _channel = null;

    _completers.clear();
  }

  bool isConnected() {
    return _channel != null && isConnecting == false;
  }

  // send message to websocket server
  Future<dynamic> sendMessage(WoxWebsocketMsg msg) async {
    msg.sessionId = Env.sessionId;
    final payload = jsonEncode(msg);

    // if query message, send it directly, no need to wait for response
    // because query result may return multiple times
    if (msg.method == WoxMsgMethodEnum.WOX_MSG_METHOD_QUERY.code) {
      msg.sendTimestamp = DateTime.now().millisecondsSinceEpoch;
      _channel?.sink.add(payload);
      return;
    }

    Completer completer = Completer();
    _completers[msg.requestId] = completer;
    _channel?.sink.add(payload);
    var responseMsg = await completer.future as WoxWebsocketMsg;
    return responseMsg.data;
  }
}
