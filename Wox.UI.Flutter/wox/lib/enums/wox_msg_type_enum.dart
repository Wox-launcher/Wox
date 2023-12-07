typedef WoxMsgType = String;

enum WoxMsgTypeEnum {
  WOX_MSG_TYPE_REQUEST("WebsocketMsgTypeRequest", "Request"),
  WOX_MSG_TYPE_RESPONSE("WebsocketMsgTypeResponse", "Response");

  final String code;
  final String value;

  const WoxMsgTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxMsgTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
