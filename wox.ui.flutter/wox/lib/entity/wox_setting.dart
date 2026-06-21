import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_glance.dart';

class WoxSetting {
  late bool enableAutostart;
  late String mainHotkey;
  late String selectionHotkey;
  late List<IgnoredHotkeyApp> ignoredHotkeyApps;
  late String logLevel;
  late bool usePinYin;
  late bool switchInputMethodABC;
  late bool hideOnStart;
  // OnboardingFinished is carried in the normal settings model so the guide
  // can persist skip/finish through the same key-value update path as other
  // first-run choices.
  late bool onboardingFinished;
  late bool hideOnLostFocus;
  late bool showTray;
  late String langCode;
  late List<QueryHotkey> queryHotkeys;
  late List<QueryShortcut> queryShortcuts;
  late List<TrayQuery> trayQueries;
  late String launchMode;
  late String startPage;
  late String showPosition;
  late bool isLinuxWaylandSession;
  late bool isEvdevRawListenerAvailable;
  late List<AIProvider> aiProviders;
  late int appWidth;
  late int maxResultCount;
  // UiDensity is stored as a small enum so Flutter derives visual metrics
  // locally while staying aligned with backend window-height estimates.
  late String uiDensity;
  late String themeId;
  late String appFontFamily;
  late bool enableQueryCompletionHint;
  late bool enableGlance;
  late GlanceRef primaryGlance;
  late bool hideGlanceIcon;
  late bool httpProxyEnabled;
  late String httpProxyUrl;
  late bool enableAutoBackup;
  late bool enableAutoUpdate;
  late String releaseChannel;
  late bool enableAnonymousUsageStats;
  late String customPythonPath;
  late String customNodejsPath;
  late String cloudSyncServerUrl;
  late List<String> cloudSyncDisabledPlugins;
  late bool showScoreTail;
  late bool showPerformanceTail;
  late bool showPerformanceTailBatch;
  late bool showPerformanceTailPluginQuery;
  late bool showPerformanceTailBackendPrepared;
  late bool showPerformanceTailUiReceived;

  WoxSetting({
    required this.enableAutostart,
    required this.mainHotkey,
    required this.selectionHotkey,
    required this.ignoredHotkeyApps,
    required this.logLevel,
    required this.usePinYin,
    required this.switchInputMethodABC,
    required this.hideOnStart,
    required this.onboardingFinished,
    required this.hideOnLostFocus,
    required this.showTray,
    required this.langCode,
    required this.queryHotkeys,
    required this.queryShortcuts,
    required this.trayQueries,
    required this.launchMode,
    required this.startPage,
    required this.showPosition,
    required this.isLinuxWaylandSession,
    required this.isEvdevRawListenerAvailable,
    required this.aiProviders,
    required this.appWidth,
    required this.maxResultCount,
    required this.uiDensity,
    required this.themeId,
    required this.appFontFamily,
    required this.enableQueryCompletionHint,
    required this.enableGlance,
    required this.primaryGlance,
    required this.hideGlanceIcon,
    required this.httpProxyEnabled,
    required this.httpProxyUrl,
    required this.enableAutoBackup,
    required this.enableAutoUpdate,
    required this.releaseChannel,
    required this.enableAnonymousUsageStats,
    required this.customPythonPath,
    required this.customNodejsPath,
    this.cloudSyncServerUrl = '',
    required this.cloudSyncDisabledPlugins,
    required this.showScoreTail,
    required this.showPerformanceTail,
    required this.showPerformanceTailBatch,
    required this.showPerformanceTailPluginQuery,
    required this.showPerformanceTailBackendPrepared,
    required this.showPerformanceTailUiReceived,
  });

  WoxSetting.fromJson(Map<String, dynamic> json) {
    enableAutostart = json['EnableAutostart'] ?? false;
    mainHotkey = json['MainHotkey'];
    selectionHotkey = json['SelectionHotkey'];
    if (json['IgnoredHotkeyApps'] != null) {
      ignoredHotkeyApps = <IgnoredHotkeyApp>[];
      json['IgnoredHotkeyApps'].forEach((v) {
        ignoredHotkeyApps.add(IgnoredHotkeyApp.fromJson(v));
      });
    } else {
      ignoredHotkeyApps = <IgnoredHotkeyApp>[];
    }
    logLevel = json['LogLevel'] ?? 'INFO';
    usePinYin = json['UsePinYin'] ?? false;
    switchInputMethodABC = json['SwitchInputMethodABC'] ?? false;
    hideOnStart = json['HideOnStart'] ?? false;
    onboardingFinished = json['OnboardingFinished'] ?? false;
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
    isLinuxWaylandSession = json['IsLinuxWaylandSession'] ?? false;
    isEvdevRawListenerAvailable = json['IsEvdevRawListenerAvailable'] ?? false;

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
    uiDensity = json['UiDensity'] ?? 'normal';
    themeId = json['ThemeId'];
    appFontFamily = json['AppFontFamily'] ?? '';
    enableQueryCompletionHint = json['EnableQueryCompletionHint'] ?? false;
    enableGlance = json['EnableGlance'] ?? true;
    primaryGlance = GlanceRef.fromJson(json['PrimaryGlance']);
    hideGlanceIcon = json['HideGlanceIcon'] ?? false;
    httpProxyEnabled = json['HttpProxyEnabled'] ?? false;
    httpProxyUrl = json['HttpProxyUrl'] ?? '';
    enableAutoBackup = json['EnableAutoBackup'] ?? false;
    enableAutoUpdate = json['EnableAutoUpdate'] ?? true;
    releaseChannel = json['ReleaseChannel'] ?? 'stable';
    enableAnonymousUsageStats = json['EnableAnonymousUsageStats'] ?? true;
    customPythonPath = json['CustomPythonPath'] ?? '';
    customNodejsPath = json['CustomNodejsPath'] ?? '';
    cloudSyncServerUrl = json['CloudSyncServerUrl'] ?? '';
    if (json['CloudSyncDisabledPlugins'] != null) {
      cloudSyncDisabledPlugins = List<String>.from(json['CloudSyncDisabledPlugins']);
    } else {
      cloudSyncDisabledPlugins = <String>[];
    }
    showScoreTail = json['ShowScoreTail'] ?? false;
    showPerformanceTail = json['ShowPerformanceTail'] ?? false;
    showPerformanceTailBatch = json['ShowPerformanceTailBatch'] ?? true;
    showPerformanceTailPluginQuery = json['ShowPerformanceTailPluginQuery'] ?? true;
    showPerformanceTailBackendPrepared = json['ShowPerformanceTailBackendPrepared'] ?? true;
    showPerformanceTailUiReceived = json['ShowPerformanceTailUiReceived'] ?? true;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['EnableAutostart'] = enableAutostart;
    data['MainHotkey'] = mainHotkey;
    data['SelectionHotkey'] = selectionHotkey;
    data['IgnoredHotkeyApps'] = ignoredHotkeyApps;
    data['LogLevel'] = logLevel;
    data['UsePinYin'] = usePinYin;
    data['SwitchInputMethodABC'] = switchInputMethodABC;
    data['HideOnStart'] = hideOnStart;
    data['OnboardingFinished'] = onboardingFinished;
    data['HideOnLostFocus'] = hideOnLostFocus;
    data['ShowTray'] = showTray;
    data['LangCode'] = langCode;
    data['QueryHotkeys'] = queryHotkeys;
    data['QueryShortcuts'] = queryShortcuts;
    data['TrayQueries'] = trayQueries;
    data['LaunchMode'] = launchMode;
    data['StartPage'] = startPage;
    data['ShowPosition'] = showPosition;
    data['IsLinuxWaylandSession'] = isLinuxWaylandSession;
    data['IsEvdevRawListenerAvailable'] = isEvdevRawListenerAvailable;
    data['AIProviders'] = aiProviders;
    data['AppWidth'] = appWidth;
    data['MaxResultCount'] = maxResultCount;
    data['UiDensity'] = uiDensity;
    data['ThemeId'] = themeId;
    data['AppFontFamily'] = appFontFamily;
    data['EnableQueryCompletionHint'] = enableQueryCompletionHint;
    data['EnableGlance'] = enableGlance;
    data['PrimaryGlance'] = primaryGlance.toJson();
    data['HideGlanceIcon'] = hideGlanceIcon;
    data['HttpProxyEnabled'] = httpProxyEnabled;
    data['HttpProxyUrl'] = httpProxyUrl;
    data['EnableAutoBackup'] = enableAutoBackup;
    data['EnableAutoUpdate'] = enableAutoUpdate;
    data['ReleaseChannel'] = releaseChannel;
    data['EnableAnonymousUsageStats'] = enableAnonymousUsageStats;
    data['CustomPythonPath'] = customPythonPath;
    data['CustomNodejsPath'] = customNodejsPath;
    data['CloudSyncServerUrl'] = cloudSyncServerUrl;
    data['CloudSyncDisabledPlugins'] = cloudSyncDisabledPlugins;
    data['ShowScoreTail'] = showScoreTail;
    data['ShowPerformanceTail'] = showPerformanceTail;
    data['ShowPerformanceTailBatch'] = showPerformanceTailBatch;
    data['ShowPerformanceTailPluginQuery'] = showPerformanceTailPluginQuery;
    data['ShowPerformanceTailBackendPrepared'] = showPerformanceTailBackendPrepared;
    data['ShowPerformanceTailUiReceived'] = showPerformanceTailUiReceived;
    return data;
  }
}

class QueryHotkey {
  late String name;

  late String hotkey;

  late String query; // Support plugin.QueryVariable

  late bool isSilentExecution;
  late bool hideQueryBox;
  late bool hideToolbar;
  late String width;
  late String maxResultCount;
  late String position;

  late bool disabled;

  QueryHotkey({
    required this.name,
    required this.hotkey,
    required this.query,
    required this.isSilentExecution,
    required this.hideQueryBox,
    required this.hideToolbar,
    required this.width,
    required this.maxResultCount,
    required this.position,
    required this.disabled,
  });

  QueryHotkey.fromJson(Map<String, dynamic> json) {
    name = json['Name']?.toString() ?? "";
    hotkey = json['Hotkey'];
    query = json['Query'];
    isSilentExecution = json['IsSilentExecution'] ?? false;
    hideQueryBox = json['HideQueryBox'] ?? false;
    hideToolbar = json['HideToolbar'] ?? false;
    width = json['Width'] == null ? "" : json['Width'].toString();
    maxResultCount = json['MaxResultCount'] == null ? "" : json['MaxResultCount'].toString();
    position = json['Position'] ?? 'system_default';
    disabled = json['Disabled'] ?? false;
  }

  String get displayName => name.trim().isNotEmpty ? name.trim() : query;

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['Hotkey'] = hotkey;
    data['Query'] = query;
    data['IsSilentExecution'] = isSilentExecution;
    data['HideQueryBox'] = hideQueryBox;
    data['HideToolbar'] = hideToolbar;
    data['Width'] = width;
    data['MaxResultCount'] = maxResultCount;
    data['Position'] = position;
    data['Disabled'] = disabled;
    return data;
  }
}

class IgnoredHotkeyApp {
  late String name;
  late String identity;
  late String path;
  late WoxImage icon;

  IgnoredHotkeyApp({required this.name, required this.identity, required this.path, required this.icon});

  IgnoredHotkeyApp.fromJson(Map<String, dynamic> json) {
    name = json['Name'] ?? '';
    identity = json['Identity'] ?? '';
    path = json['Path'] ?? '';
    icon = json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : WoxImage.empty();
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['Identity'] = identity;
    data['Path'] = path;
    data['Icon'] = icon.toJson();
    return data;
  }

  static IgnoredHotkeyApp empty() {
    return IgnoredHotkeyApp(name: '', identity: '', path: '', icon: WoxImage.empty());
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
  late String maxResultCount;
  late bool hideQueryBox;
  late bool hideToolbar;

  late bool disabled;

  TrayQuery({
    required this.icon,
    required this.query,
    required this.width,
    required this.maxResultCount,
    required this.hideQueryBox,
    required this.hideToolbar,
    required this.disabled,
  });

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
    if (json['MaxResultCount'] == null) {
      maxResultCount = "";
    } else {
      maxResultCount = json['MaxResultCount'].toString();
    }
    hideQueryBox = json['HideQueryBox'] ?? false;
    hideToolbar = json['HideToolbar'] ?? false;
    disabled = json['Disabled'] ?? false;
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Icon'] = icon;
    data['Query'] = query;
    data['Width'] = width;
    data['MaxResultCount'] = maxResultCount;
    data['HideQueryBox'] = hideQueryBox;
    data['HideToolbar'] = hideToolbar;
    data['Disabled'] = disabled;
    return data;
  }
}

class SettingWindowContext {
  // Bug fix: keep tray-opened settings distinguishable from launcher-opened
  // settings after the JSON bridge. Visibility can change during the transition,
  // so the opener source is the stable signal for Escape exit behavior.
  static const String sourceTray = 'tray';

  late String path;
  late String param;
  // Source is optional for compatibility with older core messages; empty/default
  // means the settings page should return to the launcher query UI.
  late String source;

  SettingWindowContext({required this.path, required this.param, this.source = ''});

  SettingWindowContext.fromJson(Map<String, dynamic> json) {
    path = json['Path'];
    param = json['Param'];
    source = json['Source'] ?? '';
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
  late String defaultHost;

  AIProviderInfo({required this.name, required this.icon, required this.defaultHost});

  AIProviderInfo.fromJson(Map<String, dynamic> json) {
    name = json['Name'];
    icon = WoxImage.fromJson(json['Icon']);
    defaultHost = json['DefaultHost'] ?? "";
  }
}
