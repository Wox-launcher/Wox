typedef PositionType = String;

enum PositionTypeEnum {
  POSITION_TYPE_MOUSE_SCREEN("MouseScreen", "MouseScreen"),
  POSITION_TYPE_LAST_LOCATION("LastLocation", "LastLocation");

  final String code;
  final String value;

  const PositionTypeEnum(this.code, this.value);

  static String getValue(String code) => PositionTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
