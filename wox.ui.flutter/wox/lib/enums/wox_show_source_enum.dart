typedef WoxShowSource = String;

enum WoxShowSourceEnum {
  WOX_SHOW_SOURCE_DEFAULT('default', 'default'),
  WOX_SHOW_SOURCE_QUERY_HOTKEY('query_hotkey', 'query_hotkey'),
  WOX_SHOW_SOURCE_SELECTION('selection', 'selection'),
  WOX_SHOW_SOURCE_TRAY_QUERY('tray_query', 'tray_query'),
  WOX_SHOW_SOURCE_EXPLORER('explorer', 'explorer');

  final String code;
  final String value;

  const WoxShowSourceEnum(this.code, this.value);
}
