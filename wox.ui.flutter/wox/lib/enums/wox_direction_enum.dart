typedef WoxDirection = String;

enum WoxDirectionEnum {
  WOX_DIRECTION_UP("up", "up"),
  WOX_DIRECTION_DOWN("down", "down"),
  WOX_DIRECTION_LEFT("left", "left"),
  WOX_DIRECTION_RIGHT("right", "right");

  final String code;
  final String value;

  const WoxDirectionEnum(this.code, this.value);

  static String getValue(String code) => WoxDirectionEnum.values.firstWhere((activity) => activity.code == code).value;
}
