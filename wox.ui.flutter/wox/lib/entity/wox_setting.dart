import 'package:wox/entity/wox_image.dart';

class WoxSetting {
  late bool enableAutostart;
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
  late String launchMode;
  late String startPage;
  late String showPosition;
  late List<AIProvider> aiProviders;
  late int appWidth;
  late int maxResultCount;
  late String themeId;
  late bool httpProxyEnabled;
  late String httpProxyUrl;
  late bool enableAutoBackup;
  late bool enableAutoUpdate;
  late String customPythonPath;
  late String customNodejsPath;

  WoxSetting({
    required this.enableAutostart,
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
    required this.launchMode,
    required this.startPage,
    required this.showPosition,
    required this.aiProviders,
    required this.appWidth,
    required this.maxResultCount,
    required this.themeId,
    required this.httpProxyEnabled,
    required this.httpProxyUrl,
    required this.enableAutoBackup,
    required this.enableAutoUpdate,
    required this.customPythonPath,
    required this.customNodejsPath,
  });

  WoxSetting.fromJson(Map<String, dynamic> json) {
    enableAutostart = json['EnableAutostart'] ?? false;
    mainHotkey = json['MainHotkey'];
    selectionHotkey = json['SelectionHotkey'];
    usePinYin = json['UsePinYin'] ?? false;
    switchInputMethodABC = json['SwitchInputMethodABC'] ?? false;
    hideOnStart = json['HideOnStart'] ?? false;
    hideOnLostFocus = json['HideOnLostFocus'];
    showTray = json['ShowTray'] ?? false;
    langCode = json['LangCode'];
    showPosition = json['ShowPosition'] ?? 'mouse_screen';

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

    launchMode = json['LaunchMode'] ?? 'continue';
    startPage = json['StartPage'] ?? 'mru';

    if (json['AIProviders'] != null) {
      aiProviders = <AIProvider>[];
      json['AIProviders'].forEach((v) {
        aiProviders.add(AIProvider.fromJson(v));
      });
    } else {
      aiProviders = <AIProvider>[];
    }

    appWidth = json['AppWidth'];
    maxResultCount = json['MaxResultCount'];
    themeId = json['ThemeId'];
    httpProxyEnabled = json['HttpProxyEnabled'] ?? false;
    httpProxyUrl = json['HttpProxyUrl'] ?? '';
    enableAutoBackup = json['EnableAutoBackup'] ?? false;
    enableAutoUpdate = json['EnableAutoUpdate'] ?? true;
    customPythonPath = json['CustomPythonPath'] ?? '';
    customNodejsPath = json['CustomNodejsPath'] ?? '';
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['EnableAutostart'] = enableAutostart;
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
    data['LaunchMode'] = launchMode;
    data['StartPage'] = startPage;
    data['ShowPosition'] = showPosition;
    data['AIProviders'] = aiProviders;
    data['AppWidth'] = appWidth;
    data['MaxResultCount'] = maxResultCount;
    data['ThemeId'] = themeId;
    data['HttpProxyEnabled'] = httpProxyEnabled;
    data['HttpProxyUrl'] = httpProxyUrl;
    data['EnableAutoBackup'] = enableAutoBackup;
    data['EnableAutoUpdate'] = enableAutoUpdate;
    data['CustomPythonPath'] = customPythonPath;
    data['CustomNodejsPath'] = customNodejsPath;
    return data;
  }
}

class QueryHotkey {
  late String hotkey;

  late String query; // Support plugin.QueryVariable

  late bool isSilentExecution;

  QueryHotkey({required this.hotkey, required this.query, required this.isSilentExecution});

  QueryHotkey.fromJson(Map<String, dynamic> json) {
    hotkey = json['Hotkey'];
    query = json['Query'];
    isSilentExecution = json['IsSilentExecution'] ?? false;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Hotkey'] = hotkey;
    data['Query'] = query;
    data['IsSilentExecution'] = isSilentExecution;
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

class AIProvider {
  late String name;
  late String apiKey;

  late String host;

  AIProvider({required this.name, required this.apiKey, required this.host});

  AIProvider.fromJson(Map<String, dynamic> json) {
    name = json['Name'];
    apiKey = json['ApiKey'];
    host = json['Host'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['ApiKey'] = apiKey;
    data['Host'] = host;
    return data;
  }
}

class AIProviderInfo {
  late String name;
  late WoxImage icon;

  AIProviderInfo({required this.name, required this.icon});

  AIProviderInfo.fromJson(Map<String, dynamic> json) {
    name = json['Name'];
    icon = WoxImage.fromJson(json['Icon']);
  }
}
