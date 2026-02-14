import 'package:wox/entity/wox_image.dart';

class WoxSetting {
  late bool enableAutostart;
  late String mainHotkey;
  late String selectionHotkey;
  late String logLevel;
  late bool usePinYin;
  late bool switchInputMethodABC;
  late bool hideOnStart;
  late bool hideOnLostFocus;
  late bool showTray;
  late String langCode;
  late List<QueryHotkey> queryHotkeys;
  late List<QueryShortcut> queryShortcuts;
  late List<TrayQuery> trayQueries;
  late String launchMode;
  late String startPage;
  late String showPosition;
  late List<AIProvider> aiProviders;
  late bool enableMCPServer;
  late int mcpServerPort;
  late int appWidth;
  late int maxResultCount;
  late String themeId;
  late String appFontFamily;
  late bool httpProxyEnabled;
  late String httpProxyUrl;
  late bool enableAutoBackup;
  late bool enableAutoUpdate;
  late String customPythonPath;
  late String customNodejsPath;
  late List<String> cloudSyncDisabledPlugins;

  WoxSetting({
    required this.enableAutostart,
    required this.mainHotkey,
    required this.selectionHotkey,
    required this.logLevel,
    required this.usePinYin,
    required this.switchInputMethodABC,
    required this.hideOnStart,
    required this.hideOnLostFocus,
    required this.showTray,
    required this.langCode,
    required this.queryHotkeys,
    required this.queryShortcuts,
    required this.trayQueries,
    required this.launchMode,
    required this.startPage,
    required this.showPosition,
    required this.aiProviders,
    required this.enableMCPServer,
    required this.mcpServerPort,
    required this.appWidth,
    required this.maxResultCount,
    required this.themeId,
    required this.appFontFamily,
    required this.httpProxyEnabled,
    required this.httpProxyUrl,
    required this.enableAutoBackup,
    required this.enableAutoUpdate,
    required this.customPythonPath,
    required this.customNodejsPath,
    required this.cloudSyncDisabledPlugins,
  });

  WoxSetting.fromJson(Map<String, dynamic> json) {
    enableAutostart = json['EnableAutostart'] ?? false;
    mainHotkey = json['MainHotkey'];
    selectionHotkey = json['SelectionHotkey'];
    logLevel = json['LogLevel'] ?? 'INFO';
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
    if (json['TrayQueries'] != null) {
      trayQueries = <TrayQuery>[];
      json['TrayQueries'].forEach((v) {
        trayQueries.add(TrayQuery.fromJson(v));
      });
    } else {
      trayQueries = <TrayQuery>[];
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

    enableMCPServer = json['EnableMCPServer'] ?? false;
    mcpServerPort = json['MCPServerPort'] ?? 9867;

    appWidth = json['AppWidth'];
    maxResultCount = json['MaxResultCount'];
    themeId = json['ThemeId'];
    appFontFamily = json['AppFontFamily'] ?? '';
    httpProxyEnabled = json['HttpProxyEnabled'] ?? false;
    httpProxyUrl = json['HttpProxyUrl'] ?? '';
    enableAutoBackup = json['EnableAutoBackup'] ?? false;
    enableAutoUpdate = json['EnableAutoUpdate'] ?? true;
    customPythonPath = json['CustomPythonPath'] ?? '';
    customNodejsPath = json['CustomNodejsPath'] ?? '';
    if (json['CloudSyncDisabledPlugins'] != null) {
      cloudSyncDisabledPlugins = List<String>.from(json['CloudSyncDisabledPlugins']);
    } else {
      cloudSyncDisabledPlugins = <String>[];
    }
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['EnableAutostart'] = enableAutostart;
    data['MainHotkey'] = mainHotkey;
    data['SelectionHotkey'] = selectionHotkey;
    data['LogLevel'] = logLevel;
    data['UsePinYin'] = usePinYin;
    data['SwitchInputMethodABC'] = switchInputMethodABC;
    data['HideOnStart'] = hideOnStart;
    data['HideOnLostFocus'] = hideOnLostFocus;
    data['ShowTray'] = showTray;
    data['LangCode'] = langCode;
    data['QueryHotkeys'] = queryHotkeys;
    data['QueryShortcuts'] = queryShortcuts;
    data['TrayQueries'] = trayQueries;
    data['LaunchMode'] = launchMode;
    data['StartPage'] = startPage;
    data['ShowPosition'] = showPosition;
    data['AIProviders'] = aiProviders;
    data['EnableMCPServer'] = enableMCPServer;
    data['MCPServerPort'] = mcpServerPort;
    data['AppWidth'] = appWidth;
    data['MaxResultCount'] = maxResultCount;
    data['ThemeId'] = themeId;
    data['AppFontFamily'] = appFontFamily;
    data['HttpProxyEnabled'] = httpProxyEnabled;
    data['HttpProxyUrl'] = httpProxyUrl;
    data['EnableAutoBackup'] = enableAutoBackup;
    data['EnableAutoUpdate'] = enableAutoUpdate;
    data['CustomPythonPath'] = customPythonPath;
    data['CustomNodejsPath'] = customNodejsPath;
    data['CloudSyncDisabledPlugins'] = cloudSyncDisabledPlugins;
    return data;
  }
}

class QueryHotkey {
  late String hotkey;

  late String query; // Support plugin.QueryVariable

  late bool isSilentExecution;

  late bool disabled;

  QueryHotkey({required this.hotkey, required this.query, required this.isSilentExecution, required this.disabled});

  QueryHotkey.fromJson(Map<String, dynamic> json) {
    hotkey = json['Hotkey'];
    query = json['Query'];
    isSilentExecution = json['IsSilentExecution'] ?? false;
    disabled = json['Disabled'] ?? false;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Hotkey'] = hotkey;
    data['Query'] = query;
    data['IsSilentExecution'] = isSilentExecution;
    data['Disabled'] = disabled;
    return data;
  }
}

class QueryShortcut {
  late String shortcut;

  late String query;

  late bool disabled;

  QueryShortcut({required this.shortcut, required this.query, required this.disabled});

  QueryShortcut.fromJson(Map<String, dynamic> json) {
    shortcut = json['Shortcut'];
    query = json['Query'];
    disabled = json['Disabled'] ?? false;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Shortcut'] = shortcut;
    data['Query'] = query;
    data['Disabled'] = disabled;
    return data;
  }
}

class TrayQuery {
  late WoxImage icon;

  late String query;
  late String width;

  late bool disabled;

  TrayQuery({required this.icon, required this.query, required this.width, required this.disabled});

  TrayQuery.fromJson(Map<String, dynamic> json) {
    if (json['Icon'] is Map<String, dynamic>) {
      icon = WoxImage.fromJson(json['Icon']);
    } else if (json['Icon'] is String) {
      icon = WoxImage.parse(json['Icon']) ?? WoxImage.empty();
    } else {
      icon = WoxImage.empty();
    }
    query = json['Query'];
    if (json['Width'] == null) {
      width = "";
    } else {
      width = json['Width'].toString();
    }
    disabled = json['Disabled'] ?? false;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Icon'] = icon;
    data['Query'] = query;
    data['Width'] = width;
    data['Disabled'] = disabled;
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
  late String alias;
  late String apiKey;

  late String host;

  AIProvider({required this.name, required this.alias, required this.apiKey, required this.host});

  AIProvider.fromJson(Map<String, dynamic> json) {
    name = json['Name'];
    alias = json['Alias'] ?? '';
    apiKey = json['ApiKey'];
    host = json['Host'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['Alias'] = alias;
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
