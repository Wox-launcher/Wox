typedef WoxLayoutMode = String;

enum WoxLayoutModeEnum {
  WOX_LAYOUT_MODE_DEFAULT("default", "default"),
  WOX_LAYOUT_MODE_EXPLORER("explorer", "explorer"),
  WOX_LAYOUT_MODE_TRAY_QUERY("tray_query", "tray_query");

  final String code;
  final String value;

  const WoxLayoutModeEnum(this.code, this.value);

  static String getValue(String code) => WoxLayoutModeEnum.values.firstWhere((mode) => mode.code == code).value;
}
