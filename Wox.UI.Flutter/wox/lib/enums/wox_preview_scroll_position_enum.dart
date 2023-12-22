typedef WoxPreviewScrollPosition = String;

enum WoxPreviewScrollPositionEnum {
  WOX_PREVIEW_SCROLL_POSITION_BOTTOM("bottom", "bottom");

  final String code;
  final String value;

  const WoxPreviewScrollPositionEnum(this.code, this.value);

  static String getValue(String code) => WoxPreviewScrollPositionEnum.values.firstWhere((activity) => activity.code == code).value;
}
