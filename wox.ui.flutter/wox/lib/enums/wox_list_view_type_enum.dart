typedef WoxListViewType = String;

enum WoxListViewTypeEnum {
  WOX_LIST_VIEW_TYPE_RESULT("result", "for query result list view"),
  WOX_LIST_VIEW_TYPE_ACTION("action", "for result action list view");

  final String code;
  final String value;

  const WoxListViewTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxListViewTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
