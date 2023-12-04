typedef WoxWebsocketMsgType = String;

enum WoxWebsocketMsgTypeEnum {
  WOX_WEBSOCKET_MSG_TYPE_REQUEST("WebsocketMsgTypeRequest", "WebsocketMsgTypeRequest"),
  WOX_WEBSOCKET_MSG_TYPE_RESPONSE("WebsocketMsgTypeResponse", "WebsocketMsgTypeResponse");

  final String code;
  final String value;

  const WoxWebsocketMsgTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxWebsocketMsgTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
