// ignore_for_file: constant_identifier_names
typedef WoxPreviewType = String;

enum WoxPreviewTypeEnum {
  WOX_PREVIEW_TYPE_MARKDOWN("markdown", "markdown"),
  WOX_PREVIEW_TYPE_TEXT("text", "text"),
  WOX_PREVIEW_TYPE_IMAGE("image", "image"),
  WOX_PREVIEW_TYPE_URL("url", "url"),
  WOX_PREVIEW_TYPE_FILE("file", "file"),
  WOX_PREVIEW_TYPE_LIST("list", "list"),
  WOX_PREVIEW_TYPE_REMOTE("remote", "remote"),
  WOX_PREVIEW_TYPE_TERMINAL("terminal", "terminal"),
  WOX_PREVIEW_TYPE_WEBVIEW("webview", "webview"),
  WOX_PREVIEW_TYPE_PLUGIN_DETAIL("plugin_detail", "plugin_detail"),
  WOX_PREVIEW_TYPE_CHAT("chat", "chat"),
  WOX_PREVIEW_TYPE_UPDATE("update", "update"),
  WOX_PREVIEW_TYPE_AI_STREAM("ai_stream", "ai_stream"),
  WOX_PREVIEW_TYPE_QUERY_REQUIREMENT_SETTINGS("query_requirement_settings", "query_requirement_settings"),
  WOX_PREVIEW_TYPE_THEME_EDIT("theme_edit", "theme_edit"),
  WOX_PREVIEW_TYPE_TRIGGER_KEYWORD_CONFLICT("trigger_keyword_conflict", "trigger_keyword_conflict"),
  WOX_PREVIEW_TYPE_HOTKEY_OVERVIEW("hotkey_overview", "hotkey_overview");

  final String code;
  final String value;

  const WoxPreviewTypeEnum(this.code, this.value);

  static String getValue(String code) => WoxPreviewTypeEnum.values.firstWhere((activity) => activity.code == code).value;
}
