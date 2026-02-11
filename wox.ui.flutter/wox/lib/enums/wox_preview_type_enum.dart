// ignore_for_file: constant_identifier_names
typedef WoxPreviewType = String;

enum WoxPreviewTypeEnum {
  WOX_PREVIEW_TYPE_MARKDOWN("markdown", "markdown"),
  WOX_PREVIEW_TYPE_TEXT("text", "text"),
  WOX_PREVIEW_TYPE_IMAGE("image", "image"),
  WOX_PREVIEW_TYPE_URL("url", "url"),
  WOX_PREVIEW_TYPE_FILE("file", "file"),
  WOX_PREVIEW_TYPE_REMOTE("remote", "remote"),
  WOX_PREVIEW_TYPE_TERMINAL("terminal", "terminal"),
  WOX_PREVIEW_TYPE_PLUGIN_DETAIL("plugin_detail", "plugin_detail"),
  WOX_PREVIEW_TYPE_CHAT("chat", "chat"),
  WOX_PREVIEW_TYPE_UPDATE("update", "update");

  final String code;
  final String value;

  const WoxPreviewTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxPreviewTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
