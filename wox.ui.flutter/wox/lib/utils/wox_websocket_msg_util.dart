import 'dart:async';
import 'dart:convert';

import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
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
          Get.find<WoxLauncherController>().showToolbarMsg(
              msg.traceId,
              ToolbarMsg(
                icon: WoxImage(
                    imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code,
                    imageData:
                        '<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24"><path fill="#f21818" d="M12 17q.425 0 .713-.288T13 16t-.288-.712T12 15t-.712.288T11 16t.288.713T12 17m-1-4h2V7h-2zm1 9q-2.075 0-3.9-.788t-3.175-2.137T2.788 15.9T2 12t.788-3.9t2.137-3.175T8.1 2.788T12 2t3.9.788t3.175 2.137T21.213 8.1T22 12t-.788 3.9t-2.137 3.175t-3.175 2.138T12 22"/></svg>'),
                text: msg.data,
              ));
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
