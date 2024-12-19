typedef WoxEventDeviceType = String;

enum WoxEventDeviceTypeEnum {
  WOX_EVENT_DEVEICE_TYPE_KEYBOARD("keyboard", "keyboard"),
  WOX_EVENT_DEVEICE_TYPE_MOUSE("mouse", "mouse");

  final String code;
  final String value;

  const WoxEventDeviceTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxEventDeviceTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
