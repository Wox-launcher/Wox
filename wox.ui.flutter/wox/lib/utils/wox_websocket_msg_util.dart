import 'dart:async';
import 'dart:convert';

import 'package:uuid/v4.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_msg_method_enum.dart';
import 'package:wox/enums/wox_msg_type_enum.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';

typedef WoxWebsocketMsgHandler = FutureOr<void> Function(WoxWebsocketMsg msg);

class WoxWebsocketMsgUtil {
  WoxWebsocketMsgUtil._privateConstructor();

  static final WoxWebsocketMsgUtil _instance = WoxWebsocketMsgUtil._privateConstructor();

  static WoxWebsocketMsgUtil get instance => _instance;

  static const String _coreSessionPrefix = "core-";

  WebSocketChannel? _channel;
  StreamSubscription? _subscription;
  Timer? _reconnectTimer;

  late Uri uri;

  WoxWebsocketMsgHandler? _coreHandler;

  int connectionAttempts = 1;

  bool isConnecting = false;
  bool _isDisposed = false;

  final Map<String, Completer> _completers = {};
  final Map<String, WoxWebsocketMsgHandler> _sessionHandlers = {};

  void registerSession(String sessionId, WoxWebsocketMsgHandler handler) {
    _sessionHandlers[sessionId] = handler;
  }

  void unregisterSession(String sessionId) {
    _sessionHandlers.remove(sessionId);
  }

  void setCoreHandler(WoxWebsocketMsgHandler handler) {
    _coreHandler = handler;
  }

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
        isConnecting = false;
        connectionAttempts = 1;
        var msg = WoxWebsocketMsg.fromJson(jsonDecode(event));

        if (_completers.containsKey(msg.requestId)) {
          _completers[msg.requestId]!.complete(msg);
          _completers.remove(msg.requestId);
          return;
        }

        if (msg.success == false) {
          Logger.instance.error(msg.traceId, "Received error websocket message: ${msg.toJson()}");
          return;
        }

        _dispatchMessage(msg);
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

  void _dispatchMessage(WoxWebsocketMsg msg) {
    if (msg.sessionId.isNotEmpty && !msg.sessionId.startsWith(_coreSessionPrefix)) {
      final handler = _sessionHandlers[msg.sessionId];
      if (handler == null) {
        Logger.instance.debug(msg.traceId, "No websocket session handler for ${msg.sessionId}, method=${msg.method}");
        return;
      }
      handler(msg);
      return;
    }

    final handler = _coreHandler;
    if (handler == null) {
      Logger.instance.debug(msg.traceId, "No websocket core handler, method=${msg.method}");
      return;
    }
    handler(msg);
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
  Future<void> initialize(Uri uri, {required WoxWebsocketMsgHandler onMessageReceived}) async {
    // The app runtime registers the primary instance before opening the
    // transport. Reinitializing the socket must keep live session routes,
    // otherwise primary query responses are dropped by the session router.
    await _resetTransport(clearSessionHandlers: false);
    _isDisposed = false;
    this.uri = uri;
    _coreHandler = onMessageReceived;
    _connect();
  }

  Future<void> init() async {
    await _resetTransport(clearSessionHandlers: true);
  }

  Future<void> _resetTransport({required bool clearSessionHandlers}) async {
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
    if (clearSessionHandlers) {
      _sessionHandlers.clear();
    }
  }

  bool isConnected() {
    return _channel != null && isConnecting == false;
  }

  // send message to websocket server
  Future<dynamic> sendMessage(WoxWebsocketMsg msg, {String? sessionId}) async {
    if (msg.sessionId.isEmpty) {
      msg.sessionId = sessionId ?? Env.sessionId;
    }
    final payload = jsonEncode(msg);

    if (msg.type == WoxMsgTypeEnum.WOX_MSG_TYPE_RESPONSE.code) {
      _channel?.sink.add(payload);
      return;
    }

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
