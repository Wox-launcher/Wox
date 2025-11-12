typedef WoxMsgMethod = String;

enum WoxMsgMethodEnum {
  WOX_MSG_METHOD_Log("Log", "Log"),
  WOX_MSG_METHOD_QUERY("Query", "Query"),
  WOX_MSG_METHOD_ACTION("Action", "Action"),
  WOX_MSG_METHOD_VISIBILITY_CHANGED("VisibilityChanged", "Visibility changed");

  final String code;
  final String value;

  const WoxMsgMethodEnum(this.code, this.value);

  static String getValue(String code) => WoxMsgMethodEnum.values.firstWhere((activity) => activity.code == code).value;
}
