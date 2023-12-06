typedef WoxDirection = String;

enum WoxDirectionEnum {
  WOX_DIRECTION_UP("up", "up"),
  WOX_DIRECTION_DOWN("down", "down");

  final String code;
  final String value;

  const WoxDirectionEnum(this.code, this.value);

  static String getValue(String code) => WoxDirectionEnum.values.firstWhere((activity) => activity.code == code).value;
}
