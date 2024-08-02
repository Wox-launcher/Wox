typedef WoxQueryResultTailType = String;

enum WoxQueryResultTailTypeEnum {
  WOX_QUERY_RESULT_TAIL_TYPE_TEXT("text", "text"),
  WOX_QUERY_RESULT_TAIL_TYPE_IMAGE("image", "image"),
  WOX_QUERY_RESULT_TAIL_TYPE_HOTKEY("hotkey", "hotkey");

  final String code;
  final String value;

  const WoxQueryResultTailTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxQueryResultTailTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
