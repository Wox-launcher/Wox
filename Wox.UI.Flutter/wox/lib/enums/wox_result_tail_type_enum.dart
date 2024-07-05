typedef WoxQueryResultTailType = String;

enum WoxQueryResultTailTypeEnum {
  WOX_QUERY_RESULT_TAIL_TYPE_TEXT("text", "text"),
  WOX_QUERY_RESULT_TAIL_TYPE_IMAGE("image", "image");

  final String code;
  final String value;

  const WoxQueryResultTailTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxQueryResultTailTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
