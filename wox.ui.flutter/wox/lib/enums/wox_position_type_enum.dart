typedef WoxPositionType = String;

enum WoxPositionTypeEnum {
  POSITION_TYPE_MOUSE_SCREEN("MouseScreen", "MouseScreen"),
  POSITION_TYPE_LAST_LOCATION("LastLocation", "LastLocation");

  final String code;
  final String value;

  const WoxPositionTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxPositionTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
