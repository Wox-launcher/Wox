typedef WoxImageType = String;

enum WoxImageTypeEnum {
  WOX_IMAGE_TYPE_ABSOLUTE_PATH("absolute", "absolute"),
  WOX_IMAGE_TYPE_RELATIVE_PATH("relative", "relative"),
  WOX_IMAGE_TYPE_BASE64("base64", "base64"),
  WOX_IMAGE_TYPE_SVG("svg", "svg"),
  WOX_IMAGE_TYPE_LOTTIE("lottie", "lottie"),
  WOX_IMAGE_TYPE_EMOJI("emoji", "emoji"),
  WOX_IMAGE_TYPE_URL("url", "url");

  final String code;
  final String value;

  const WoxImageTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxImageTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
