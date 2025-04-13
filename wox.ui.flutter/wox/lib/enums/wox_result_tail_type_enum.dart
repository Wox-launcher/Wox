typedef WoxListItemTailType = String;

enum WoxListItemTailTypeEnum {
  WOX_LIST_ITEM_TAIL_TYPE_TEXT("text", "text"),
  WOX_LIST_ITEM_TAIL_TYPE_IMAGE("image", "image"),
  WOX_LIST_ITEM_TAIL_TYPE_HOTKEY("hotkey", "hotkey");

  final String code;
  final String value;

  const WoxListItemTailTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxListItemTailTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
