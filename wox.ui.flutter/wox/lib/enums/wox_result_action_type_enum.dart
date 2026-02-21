typedef WoxResultActionType = String;

enum WoxResultActionTypeEnum {
  WOX_RESULT_ACTION_TYPE_EXECUTE("execute", "execute action directly"),
  WOX_RESULT_ACTION_TYPE_FORM("form", "open form action panel"),
  WOX_RESULT_ACTION_TYPE_LOCAL("local", "execute local action");

  final String code;
  final String value;

  const WoxResultActionTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxResultActionTypeEnum.values.firstWhere((item) => item.code == code).value;
}
