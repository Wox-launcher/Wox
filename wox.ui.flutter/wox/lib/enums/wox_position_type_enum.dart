typedef WoxPositionType = String;

enum WoxPositionTypeEnum {
  POSITION_TYPE_MOUSE_SCREEN("mouse_screen", "MouseScreen"),
  POSITION_TYPE_ACTIVE_SCREEN("active_screen", "ActiveScreen"),
  POSITION_TYPE_LAST_LOCATION("last_location", "LastLocation");

  final String code;
  final String value;

  const WoxPositionTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxPositionTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
