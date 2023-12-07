typedef WoxMsgMethod = String;

enum WoxMsgMethodEnum {
  WOX_MSG_METHOD_PING("Ping", "Ping"),
  WOX_MSG_METHOD_QUERY("Query", "Query"),
  WOX_MSG_METHOD_ACTION("Action", "Action"),
  WOX_MSG_METHOD_REFRESH("Refresh", "Refresh"),
  WOX_MSG_METHOD_REGISTER_MAIN_HOTKEY("RegisterMainHotkey", "Register Main Hotkey"),
  WOX_MSG_METHOD_VISIBILITY_CHANGED("VisibilityChanged", "Visibility changed"),
  WOX_MSG_METHOD_LOST_FOCUS("LostFocus", "Lost focus");

  final String code;
  final String value;

  const WoxMsgMethodEnum(this.code, this.value);

  static String getValue(String code) => WoxMsgMethodEnum.values.firstWhere((activity) => activity.code == code).value;
}
