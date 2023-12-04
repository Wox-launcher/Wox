typedef WoxSelectionType = String;

enum WoxSelectionTypeEnum {
  WOX_SELECTION_TYPE_TEXT("text", "text"),
  WOX_SELECTION_TYPE_FILE("file", "file");

  final String code;
  final String value;

  const WoxSelectionTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxSelectionTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
