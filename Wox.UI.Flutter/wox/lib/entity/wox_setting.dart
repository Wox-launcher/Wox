class WoxSetting {
  late String mainHotkey;
  late String selectionHotkey;
  late bool usePinYin;
  late bool switchInputMethodABC;
  late bool hideOnStart;
  late bool hideOnLostFocus;
  late bool showTray;
  late String langCode;
  late List<QueryHotkey> queryHotkeys;
  late List<QueryShortcut> queryShortcuts;
  late String lastQueryMode;
  late int appWidth;
  late String themeId;

  WoxSetting({
    required this.mainHotkey,
    required this.selectionHotkey,
    required this.usePinYin,
    required this.switchInputMethodABC,
    required this.hideOnStart,
    required this.hideOnLostFocus,
    required this.showTray,
    required this.langCode,
    required this.queryHotkeys,
    required this.queryShortcuts,
    required this.lastQueryMode,
    required this.appWidth,
    required this.themeId,
  });

  WoxSetting.fromJson(Map<String, dynamic> json) {
    mainHotkey = json['MainHotkey'];
    selectionHotkey = json['SelectionHotkey'];
    usePinYin = json['UsePinYin'];
    switchInputMethodABC = json['SwitchInputMethodABC'];
    hideOnStart = json['HideOnStart'];
    hideOnLostFocus = json['HideOnLostFocus'];
    showTray = json['ShowTray'];
    langCode = json['LangCode'];

    if (json['QueryHotkeys'] != null) {
      queryHotkeys = <QueryHotkey>[];
      json['QueryHotkeys'].forEach((v) {
        queryHotkeys.add(QueryHotkey.fromJson(v));
      });
    } else {
      queryHotkeys = <QueryHotkey>[];
    }

    if (json['QueryShortcuts'] != null) {
      queryShortcuts = <QueryShortcut>[];
      json['QueryShortcuts'].forEach((v) {
        queryShortcuts.add(QueryShortcut.fromJson(v));
      });
    } else {
      queryShortcuts = <QueryShortcut>[];
    }

    lastQueryMode = json['LastQueryMode'];
    appWidth = json['AppWidth'];
    themeId = json['ThemeId'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['MainHotkey'] = mainHotkey;
    data['SelectionHotkey'] = selectionHotkey;
    data['UsePinYin'] = usePinYin;
    data['SwitchInputMethodABC'] = switchInputMethodABC;
    data['HideOnStart'] = hideOnStart;
    data['HideOnLostFocus'] = hideOnLostFocus;
    data['ShowTray'] = showTray;
    data['LangCode'] = langCode;
    data['QueryHotkeys'] = queryHotkeys;
    data['QueryShortcuts'] = queryShortcuts;
    data['LastQueryMode'] = lastQueryMode;
    data['AppWidth'] = appWidth;
    data['ThemeId'] = themeId;
    return data;
  }
}

class QueryHotkey {
  late String hotkey;

  late String query; // Support plugin.QueryVariable

  QueryHotkey({required this.hotkey, required this.query});

  QueryHotkey.fromJson(Map<String, dynamic> json) {
    hotkey = json['Hotkey'];
    query = json['Query'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Hotkey'] = hotkey;
    data['Query'] = query;
    return data;
  }
}

class QueryShortcut {
  late String shortcut;

  late String query;

  QueryShortcut({required this.shortcut, required this.query});

  QueryShortcut.fromJson(Map<String, dynamic> json) {
    shortcut = json['Shortcut'];
    query = json['Query'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Shortcut'] = shortcut;
    data['Query'] = query;
    return data;
  }
}

class SettingWindowContext {
  late String path;
  late String param;

  SettingWindowContext({required this.path, required this.param});

  SettingWindowContext.fromJson(Map<String, dynamic> json) {
    path = json['Path'];
    param = json['Param'];
  }
}
