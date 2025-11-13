typedef WoxStartPage = String;

enum WoxStartPageEnum {
  WOX_START_PAGE_BLANK("blank", "blank"),
  WOX_START_PAGE_MRU("mru", "mru");

  final String code;
  final String value;

  const WoxStartPageEnum(this.code, this.value);

  static String getValue(String code) => WoxStartPageEnum.values.firstWhere((page) => page.code == code).value;
}

