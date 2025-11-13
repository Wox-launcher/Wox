typedef WoxLaunchMode = String;

enum WoxLaunchModeEnum {
  WOX_LAUNCH_MODE_FRESH("fresh", "fresh"),
  WOX_LAUNCH_MODE_CONTINUE("continue", "continue");

  final String code;
  final String value;

  const WoxLaunchModeEnum(this.code, this.value);

  static String getValue(String code) => WoxLaunchModeEnum.values.firstWhere((mode) => mode.code == code).value;
}

