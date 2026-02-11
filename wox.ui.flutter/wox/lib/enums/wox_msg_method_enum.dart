// ignore_for_file: constant_identifier_names
typedef WoxMsgMethod = String;

enum WoxMsgMethodEnum {
  WOX_MSG_METHOD_Log("Log", "Log"),
  WOX_MSG_METHOD_QUERY("Query", "Query"),
  WOX_MSG_METHOD_QUERY_MRU("QueryMRU", "Query MRU"),
  WOX_MSG_METHOD_ACTION("Action", "Action"),
  WOX_MSG_METHOD_FORM_ACTION("FormAction", "Form action"),
  WOX_MSG_METHOD_VISIBILITY_CHANGED("VisibilityChanged", "Visibility changed"),
  WOX_MSG_METHOD_TERMINAL_SUBSCRIBE("TerminalSubscribe", "Terminal subscribe"),
  WOX_MSG_METHOD_TERMINAL_UNSUBSCRIBE("TerminalUnsubscribe", "Terminal unsubscribe"),
  WOX_MSG_METHOD_TERMINAL_SEARCH("TerminalSearch", "Terminal search"),
  WOX_MSG_METHOD_TERMINAL_CHUNK("TerminalChunk", "Terminal chunk"),
  WOX_MSG_METHOD_TERMINAL_STATE("TerminalState", "Terminal state");

  final String code;
  final String value;

  const WoxMsgMethodEnum(this.code, this.value);

  static String getValue(String code) => WoxMsgMethodEnum.values.firstWhere((activity) => activity.code == code).value;
}
