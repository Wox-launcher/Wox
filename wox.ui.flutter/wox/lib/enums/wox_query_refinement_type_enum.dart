typedef WoxQueryRefinementType = String;

enum WoxQueryRefinementTypeEnum {
  singleSelect("singleSelect", "single select"),
  multiSelect("multiSelect", "multi select"),
  toggle("toggle", "toggle"),
  sort("sort", "sort");

  final String code;
  final String value;

  const WoxQueryRefinementTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxQueryRefinementTypeEnum.values.firstWhere((item) => item.code == code).value;
}
