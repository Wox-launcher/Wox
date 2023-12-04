typedef WoxQueryType = String;

enum WoxQueryTypeEnum {
  WOX_QUERY_TYPE_INPUT("input", "input"),
  WOX_QUERY_TYPE_SELECTION("selection", "selection");

  final String code;
  final String value;

  const WoxQueryTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxQueryTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
