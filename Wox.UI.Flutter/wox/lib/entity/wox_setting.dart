class WoxSetting {
  late String mainHotkey;
  late String selectionHotkey;
  late bool usePinYin;
  late bool switchInputMethodABC;
  late bool hideOnStart;
  late bool hideOnLostFocus;
  late bool showTray;
  late String langCode;
  late String queryHotkeys;
  late String queryShortcuts;
  late String lastQueryMode;
  late int appWidth;
  late String themeId;

  WoxSetting(
      {required this.mainHotkey,
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
      required this.themeId});

  WoxSetting.fromJson(Map<String, dynamic> json) {
    mainHotkey = json['MainHotkey'];
    selectionHotkey = json['SelectionHotkey'];
    usePinYin = json['UsePinYin'];
    switchInputMethodABC = json['SwitchInputMethodABC'];
    hideOnStart = json['HideOnStart'];
    hideOnLostFocus = json['HideOnLostFocus'];
    showTray = json['ShowTray'];
    langCode = json['LangCode'];
    queryHotkeys = json['QueryHotkeys']?.isEmpty ?? '';
    queryShortcuts = json['QueryShortcuts']?.isEmpty ?? '';
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
