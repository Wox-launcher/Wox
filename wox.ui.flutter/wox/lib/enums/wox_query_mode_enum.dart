typedef WoxQueryMode = String;

enum WoxQueryModeEnum {
  WOX_QUERY_MODE_PRESERVE("preserve", "preserve"),
  WOX_QUERY_MODE_EMPTY("empty", "empty"),
  WOX_QUERY_MODE_MRU("mru", "mru");

  final String code;
  final String value;

  const WoxQueryModeEnum(this.code, this.value);

  static String getValue(String code) => WoxQueryModeEnum.values.firstWhere((activity) => activity.code == code).value;
}
