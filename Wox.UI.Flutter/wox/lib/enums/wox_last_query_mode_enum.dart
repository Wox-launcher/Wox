typedef WoxLastQueryMode = String;

enum WoxLastQueryModeEnum {
  WOX_LAST_QUERY_MODE_PRESERVE("preserve", "preserve"),
  WOX_LAST_QUERY_MODE_EMPTY("empty", "empty");

  final String code;
  final String value;

  const WoxLastQueryModeEnum(this.code, this.value);

  static String getValue(String code) => WoxLastQueryModeEnum.values.firstWhere((activity) => activity.code == code).value;
}
