import 'dart:async';
import 'dart:io';

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_cloud_sync.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_setting_search.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_update_channel_version.dart';
import 'package:wox/entity/wox_usage_stats.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_plugin_runtime_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_fuzzy_match_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/windows/linux_window_manager.dart';

// WoxThemeEditorDraftSession carries the original active theme, saved baseline, and current unsaved draft.
class WoxThemeEditorDraftSession {
  final WoxTheme restoreTheme;
  final WoxTheme sourceTheme;
  final WoxTheme draftTheme;

  const WoxThemeEditorDraftSession({required this.restoreTheme, required this.sourceTheme, required this.draftTheme});
}

class WoxSettingController extends GetxController with WidgetsBindingObserver {
  static const double _settingSearchResultEstimatedRowExtent = 52.0;
  static const Duration _accountBillingPollInterval = Duration(seconds: 5);
  static const Duration _accountBillingWaitTimeout = Duration(minutes: 5);
  static const Duration _cloudSyncStatusPollInterval = Duration(seconds: 2);
  static const Duration _cloudSyncStatusWaitTimeout = Duration(minutes: 5);

  final activeNavPath = 'general'.obs;
  final woxSetting = WoxSettingUtil.instance.currentSetting.obs;
  final userDataLocation = "".obs;
  final backups = <WoxBackup>[].obs;
  // Manual backup can take noticeable time. Keep the in-flight state in the
  // controller so the settings UI can disable duplicate clicks, and so calls
  // that arrive before the widget rebuilds are still ignored in one place.
  final isBackingUp = false.obs;
  final woxVersion = "".obs;
  final runtimeStatuses = <WoxRuntimeStatus>[].obs;
  final isRuntimeStatusLoading = false.obs;
  final runtimeStatusError = ''.obs;
  final restartingRuntime = ''.obs;
  final isClearingLogs = false.obs;
  final isUpdatingLogLevel = false.obs;
  final updateChannelVersions = <WoxUpdateChannelVersion>[].obs;
  final cloudSyncStatus = WoxCloudSyncStatus.empty().obs;
  final accountStatus = WoxAccountStatus.empty().obs;
  final cloudSyncBillingPlan = WoxBillingPlan.empty().obs;
  final cloudSyncBillingPlanLoaded = false.obs;
  final cloudSyncBillingPlanError = ''.obs;
  final cloudSyncDeviceList = WoxCloudSyncDeviceList.empty().obs;
  final isCloudSyncStatusLoading = false.obs;
  final cloudSyncStatusError = ''.obs;
  final isCloudSyncActionLoading = false.obs;
  final cloudSyncActionError = ''.obs;
  final accountActionError = ''.obs;
  final accountSubscriptionError = ''.obs;
  final isAccountBillingWaiting = false.obs;
  final accountBillingWaitingMessageKey = ''.obs;
  // Installed plugin details are stored in a plain list, so this signal lets views
  // rebuild after background refreshes without showing a transient loading state.
  final installedPluginListRevision = 0.obs;

  final usageStats = WoxUsageStats.empty().obs;
  final isUsageStatsLoading = false.obs;
  final usageStatsError = ''.obs;
  final usageStatsPeriod = '30d'.obs;
  final systemFontFamilies = <String>[].obs;
  final settingGlancePreviewItems = <String, GlanceItem>{}.obs;
  bool _isRefreshingSettingGlancePreviews = false;
  bool _hasRefreshedSettingGlancePreviewsForUIEntry = false;

  //plugins
  final pluginList = <PluginDetail>[];
  final storePlugins = <PluginDetail>[];
  final installedPlugins = <PluginDetail>[];
  final filterPluginKeywordController = TextEditingController();
  final filteredPluginList = <PluginDetail>[].obs;
  final filterEnabledPluginsOnly = false.obs;
  final filterDisabledPluginsOnly = false.obs;
  final filterUpgradablePluginsOnly = false.obs;
  final filterUninstalledPluginsOnly = false.obs;
  final filterThirdPartyPluginsOnly = false.obs;
  final filterRuntimeNodejsOnly = false.obs;
  final filterRuntimePythonOnly = false.obs;
  final filterRuntimeScriptOnly = false.obs;
  final filterRuntimeScriptNodejsOnly = false.obs;
  final filterRuntimeScriptPythonOnly = false.obs;
  final activePlugin = PluginDetail.empty().obs;
  final isStorePluginList = true.obs;
  String pendingInstalledPluginFocusRef = '';
  final pluginListScrollController = ScrollController();
  final Map<String, GlobalKey> pluginListItemKeys = <String, GlobalKey>{};
  TabController? activePluginTabController;

  final isRefreshingPluginList = false.obs;

  // Whether the active Wayland compositor supports wlr-layer-shell, enabling
  // precise launcher placement (Hyprland/sway). Populated from the GTK backend
  // on Linux so the UI settings can re-expose the ShowPosition selector.
  final linuxLayerShellSupported = false.obs;

  //themes
  final themeList = <WoxTheme>[];
  final storeThemesList = <WoxTheme>[];
  final installedThemesList = <WoxTheme>[]; // All installed themes for auto theme lookup
  final filteredThemeList = <WoxTheme>[].obs;
  final activeTheme = WoxTheme.empty().obs;
  final isStoreThemeList = true.obs;
  final themeListScrollController = ScrollController();
  final filterThemeKeywordController = TextEditingController();
  WoxTheme? _themeEditorRestoreTheme;
  WoxTheme? _themeEditorSourceTheme;
  WoxTheme? _themeEditorDraftTheme;
  Future<void>? _storeThemesLoadFuture;
  bool _hasLoadedStoreThemes = false;

  //lang
  var langMap = <String, String>{}.obs;

  final isInstallingPlugin = false.obs;
  final isUpgradingPlugin = false.obs;
  final pluginInstallError = ''.obs;
  final FocusNode settingFocusNode = FocusNode();
  final TextEditingController settingSearchTextController = TextEditingController();
  final FocusNode settingSearchFocusNode = FocusNode();
  final ScrollController settingSearchResultScrollController = ScrollController();
  final settingSearchResults = <WoxSettingSearchResult>[].obs;
  final settingSearchPanelVisible = false.obs;
  final selectedSettingSearchResultIndex = 0.obs;
  final highlightedSettingTargetId = ''.obs;
  final Map<String, GlobalKey> generalSectionKeys = <String, GlobalKey>{};
  final Map<String, GlobalKey> builtInSettingKeys = <String, GlobalKey>{};
  final Map<String, GlobalKey> pluginSettingItemKeys = <String, GlobalKey>{};
  final RxnInt pendingTrayQueryEditRowIndex = RxnInt();
  Future<WoxAIModelSelectorResources>? _aiModelSelectorResourcesFuture;
  WoxAIModelSelectorResources? _aiModelSelectorResources;
  bool _hasPreloadedSettingViewData = false;
  String _pendingGeneralSectionAnchor = '';
  String _pendingBuiltInSettingKey = '';
  String _pendingPluginSettingKey = '';
  Timer? _settingHighlightTimer;
  Timer? _accountBillingPollTimer;
  Timer? _cloudSyncStatusPollTimer;
  DateTime? _accountBillingWaitDeadline;
  DateTime? _cloudSyncStatusWaitDeadline;
  String _accountBillingWaitTimeoutMessageKey = '';
  bool Function(WoxAccountStatus status)? _accountBillingWaitComplete;
  bool _isAccountBillingPolling = false;
  bool _isCloudSyncStatusPolling = false;
  bool _isRefreshingAccountStatusOnResume = false;

  @override
  void onInit() {
    super.onInit();
    WidgetsBinding.instance.addObserver(this);
    ever<String>(activeNavPath, _handleActiveNavPathChanged);
    if (Platform.isLinux) {
      _refreshLinuxLayerShellSupported();
    }
  }

  Future<void> _refreshLinuxLayerShellSupported() async {
    if (!Platform.isLinux) {
      return;
    }
    try {
      final info = await LinuxWindowManager.instance.getBackendInfo();
      final supported = info["supportsLayerShell"] == true || info["supportsLayerShell"].toString().toLowerCase() == "true";
      linuxLayerShellSupported.value = supported;
    } catch (_) {
      // Keep the default (false); the settings UI will hide ShowPosition.
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) {
      return;
    }
    unawaited(_refreshCloudSyncAccountStatusOnResume());
  }

  void _handleActiveNavPathChanged(String path) {
    if (path != 'data.cloudsync') {
      _stopCloudSyncStatusWaiting();
    }

    if (path != 'ui') {
      // The Glance preview belongs to the UI settings page only. Clearing the
      // entry flag on navigation keeps the next visit a fresh real API snapshot
      // without running background refreshes while the page is not visible.
      _hasRefreshedSettingGlancePreviewsForUIEntry = false;
      settingGlancePreviewItems.clear();
      return;
    }

    unawaited(refreshSettingGlancePreviewsForUIEntry(const UuidV4().generate()));
  }

  void preloadSettingViewData(String traceId, {bool forceRefresh = false}) {
    if (_hasPreloadedSettingViewData && !forceRefresh) {
      return;
    }

    _hasPreloadedSettingViewData = true;
    unawaited(loadSystemFontFamilies());
    unawaited(loadUserDataLocation());
    unawaited(refreshBackups());
    unawaited(loadWoxVersion());
    unawaited(loadUpdateChannelVersions());
    unawaited(refreshRuntimeStatuses());
    unawaited(refreshUsageStats());
    unawaited(refreshCloudSyncBillingPlan());
    unawaited(reloadPlugins(traceId));
    unawaited(loadAIModelSelectorResources(traceId: traceId, forceRefresh: forceRefresh));
  }

  /// Loads and caches AI model selector data for the current settings session.
  Future<WoxAIModelSelectorResources> loadAIModelSelectorResources({String? traceId, bool forceRefresh = false}) async {
    if (forceRefresh && _aiModelSelectorResources == null) {
      final running = _aiModelSelectorResourcesFuture;
      if (running != null) {
        return running;
      }
    }

    if (!forceRefresh) {
      final cached = _aiModelSelectorResources;
      if (cached != null) {
        return cached;
      }

      final running = _aiModelSelectorResourcesFuture;
      if (running != null) {
        return running;
      }
    }

    final effectiveTraceId = traceId ?? const UuidV4().generate();
    final future = _fetchAIModelSelectorResources(effectiveTraceId);
    _aiModelSelectorResourcesFuture = future;

    try {
      final resources = await future;
      _aiModelSelectorResources = resources;
      return resources;
    } finally {
      if (identical(_aiModelSelectorResourcesFuture, future)) {
        _aiModelSelectorResourcesFuture = null;
      }
    }
  }

  /// Clears stale AI model selector data before the next provider-driven refresh.
  void invalidateAIModelSelectorResources() {
    _aiModelSelectorResources = null;
    _aiModelSelectorResourcesFuture = null;
  }

  Future<WoxAIModelSelectorResources> _fetchAIModelSelectorResources(String traceId) async {
    final results = await Future.wait([WoxApi.instance.findAIModels(traceId), WoxApi.instance.findAIProviders(traceId)]);
    return WoxAIModelSelectorResources(models: results[0] as List<AIModel>, providers: results[1] as List<AIProviderInfo>);
  }

  GlobalKey getGeneralSectionKey(String sectionId) {
    return generalSectionKeys.putIfAbsent(sectionId, () => GlobalKey(debugLabel: 'settings-general-$sectionId'));
  }

  GlobalKey getBuiltInSettingKey(String settingKey) {
    return builtInSettingKeys.putIfAbsent(settingKey, () => GlobalKey(debugLabel: 'settings-built-in-$settingKey'));
  }

  GlobalKey getPluginSettingItemKey(String pluginId, String settingKey) {
    final key = '$pluginId\x00$settingKey';
    return pluginSettingItemKeys.putIfAbsent(key, () => GlobalKey(debugLabel: 'settings-plugin-setting-$pluginId-$settingKey'));
  }

  bool isSettingTargetHighlighted(String targetId) {
    return highlightedSettingTargetId.value == targetId;
  }

  void handleSettingSearchChanged() {
    refreshSettingSearchResults();
    settingSearchPanelVisible.value = settingSearchTextController.text.trim().isNotEmpty;
  }

  void closeSettingSearchPanel() {
    settingSearchPanelVisible.value = false;
  }

  void clearSettingSearch() {
    settingSearchTextController.clear();
    settingSearchResults.clear();
    settingSearchPanelVisible.value = false;
    selectedSettingSearchResultIndex.value = 0;
  }

  void refreshSettingSearchResults() {
    final keyword = settingSearchTextController.text.trim();
    if (keyword.isEmpty) {
      settingSearchResults.clear();
      selectedSettingSearchResultIndex.value = 0;
      return;
    }

    final results = <WoxSettingSearchResult>[];
    for (final candidate in [..._buildBuiltInSettingSearchResults(), ..._buildInstalledPluginSearchResults(), ..._buildPluginSettingSearchResults()]) {
      final score = _matchSettingSearchCandidate(candidate.searchTexts, keyword);
      if (score <= 0) {
        continue;
      }
      results.add(
        WoxSettingSearchResult(
          type: candidate.type,
          id: candidate.id,
          title: candidate.title,
          subtitle: candidate.subtitle,
          navPath: candidate.navPath,
          pluginId: candidate.pluginId,
          settingKey: candidate.settingKey,
          icon: candidate.icon,
          searchTexts: candidate.searchTexts,
          score: score,
        ),
      );
    }

    results.sort((a, b) {
      final scoreCompare = b.score.compareTo(a.score);
      if (scoreCompare != 0) {
        return scoreCompare;
      }
      // Feature refinement: plugin settings inherit their plugin name as
      // searchable text. When that ties with the plugin row itself, prefer the
      // higher-level destination so typing a plugin name opens the plugin first.
      final typeCompare = _settingSearchTypePriority(a.type).compareTo(_settingSearchTypePriority(b.type));
      if (typeCompare != 0) {
        return typeCompare;
      }
      return a.title.toLowerCase().compareTo(b.title.toLowerCase());
    });
    settingSearchResults.assignAll(results.take(8));
    // Feature: every search refresh starts with a deterministic keyboard target
    // so Enter can immediately open the best match without requiring a mouse.
    selectedSettingSearchResultIndex.value = 0;
    _scheduleSelectedSettingSearchResultVisible(immediate: true);
  }

  void selectSettingSearchResult(int index) {
    if (settingSearchResults.isEmpty) {
      selectedSettingSearchResultIndex.value = 0;
      return;
    }

    selectedSettingSearchResultIndex.value = index.clamp(0, settingSearchResults.length - 1).toInt();
    _scheduleSelectedSettingSearchResultVisible();
  }

  void moveSettingSearchSelection(int delta) {
    if (settingSearchResults.isEmpty) {
      selectedSettingSearchResultIndex.value = 0;
      return;
    }

    final nextIndex = (selectedSettingSearchResultIndex.value + delta).clamp(0, settingSearchResults.length - 1).toInt();
    selectedSettingSearchResultIndex.value = nextIndex;
    _scheduleSelectedSettingSearchResultVisible(immediate: true);
  }

  void _scheduleSelectedSettingSearchResultVisible({bool immediate = false}) {
    if (settingSearchResultScrollController.hasClients) {
      _scrollSelectedSettingSearchResultVisible(immediate: immediate);
      return;
    }

    WidgetsBinding.instance.addPostFrameCallback((_) {
      _scrollSelectedSettingSearchResultVisible(immediate: immediate);
    });
  }

  void _scrollSelectedSettingSearchResultVisible({bool immediate = false}) {
    if (!settingSearchResultScrollController.hasClients || settingSearchResults.isEmpty) {
      return;
    }

    final position = settingSearchResultScrollController.position;
    final selectedIndex = selectedSettingSearchResultIndex.value.clamp(0, settingSearchResults.length - 1).toInt();
    final itemTop = selectedIndex * _settingSearchResultEstimatedRowExtent;
    final itemBottom = itemTop + _settingSearchResultEstimatedRowExtent;
    final viewportTop = position.pixels;
    final viewportBottom = viewportTop + position.viewportDimension;
    double? targetOffset;
    // Bug fix: keyboard selection used to move only the highlighted row. The
    // floating list is height-limited, so rows past the viewport stayed hidden.
    // Keyboard repeat can cancel short animations before they visibly move, so
    // arrow navigation uses an immediate jump while search refreshes can still
    // defer until the list has a mounted scroll position.
    if (itemTop < viewportTop) {
      targetOffset = itemTop;
    } else if (itemBottom > viewportBottom) {
      targetOffset = itemBottom - position.viewportDimension;
    }
    if (targetOffset == null) {
      return;
    }

    final clampedOffset = targetOffset.clamp(position.minScrollExtent, position.maxScrollExtent).toDouble();
    if ((clampedOffset - position.pixels).abs() < 0.5) {
      return;
    }
    if (immediate) {
      settingSearchResultScrollController.jumpTo(clampedOffset);
      return;
    }
    unawaited(settingSearchResultScrollController.animateTo(clampedOffset, duration: const Duration(milliseconds: 120), curve: Curves.easeOutCubic));
  }

  Future<void> activateSelectedSettingSearchResult() async {
    if (settingSearchResults.isEmpty) {
      return;
    }

    await activateSettingSearchResult(settingSearchResults[selectedSettingSearchResultIndex.value.clamp(0, settingSearchResults.length - 1).toInt()]);
  }

  Future<void> activateSettingSearchResult(WoxSettingSearchResult result) async {
    settingSearchPanelVisible.value = false;

    switch (result.type) {
      case WoxSettingSearchTargetType.builtInSetting:
        if (result.navPath == 'data.cloudsync') {
          await switchToCloudSyncView(const UuidV4().generate());
        } else {
          activeNavPath.value = result.navPath;
        }
        _pendingBuiltInSettingKey = result.settingKey;
        _highlightSettingTarget(result.highlightTargetId);
        _schedulePendingBuiltInSettingFocus();
        break;
      case WoxSettingSearchTargetType.installedPlugin:
        await switchToPluginList(const UuidV4().generate(), false);
        focusInstalledPlugin(result.pluginId);
        _highlightSettingTarget(result.highlightTargetId);
        break;
      case WoxSettingSearchTargetType.pluginSetting:
        await switchToPluginList(const UuidV4().generate(), false);
        final focused = focusInstalledPlugin(result.pluginId);
        if (!focused) {
          return;
        }
        _pendingPluginSettingKey = result.settingKey;
        // Search jumps always land on the Settings tab because the result
        // points to a concrete plugin setting, not just the plugin detail pane.
        if (shouldShowSettingTab()) {
          activePluginTabController?.index = 0;
        }
        _highlightSettingTarget(result.highlightTargetId);
        _schedulePendingPluginSettingFocus();
        break;
    }
  }

  void notifyBuiltInSettingViewReady() {
    if (_pendingBuiltInSettingKey.isEmpty) {
      return;
    }
    _schedulePendingBuiltInSettingFocus();
  }

  void notifyPluginSettingViewReady() {
    if (_pendingPluginSettingKey.isEmpty) {
      return;
    }
    _schedulePendingPluginSettingFocus();
  }

  void _highlightSettingTarget(String targetId) {
    highlightedSettingTargetId.value = targetId;
    _settingHighlightTimer?.cancel();
    _settingHighlightTimer = Timer(const Duration(milliseconds: 1500), () {
      if (highlightedSettingTargetId.value == targetId) {
        highlightedSettingTargetId.value = '';
      }
    });
  }

  void _schedulePendingBuiltInSettingFocus({int attempt = 0}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_pendingBuiltInSettingKey.isEmpty) {
        return;
      }

      final targetKey = builtInSettingKeys[_pendingBuiltInSettingKey];
      final targetContext = targetKey?.currentContext;
      if (targetContext == null) {
        if (attempt >= 10) {
          return;
        }
        Future.delayed(const Duration(milliseconds: 80), () {
          _schedulePendingBuiltInSettingFocus(attempt: attempt + 1);
        });
        return;
      }

      Scrollable.ensureVisible(targetContext, duration: const Duration(milliseconds: 220), curve: Curves.easeOutCubic, alignment: 0.14);
      _pendingBuiltInSettingKey = '';
    });
  }

  void _schedulePendingPluginSettingFocus({int attempt = 0}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_pendingPluginSettingKey.isEmpty || activePlugin.value.id.isEmpty) {
        return;
      }

      final targetKey = pluginSettingItemKeys['${activePlugin.value.id}\x00$_pendingPluginSettingKey'];
      final targetContext = targetKey?.currentContext;
      if (targetContext == null) {
        if (attempt >= 10) {
          return;
        }
        Future.delayed(const Duration(milliseconds: 80), () {
          _schedulePendingPluginSettingFocus(attempt: attempt + 1);
        });
        return;
      }

      Scrollable.ensureVisible(targetContext, duration: const Duration(milliseconds: 220), curve: Curves.easeOutCubic, alignment: 0.14);
      _pendingPluginSettingKey = '';
    });
  }

  List<WoxSettingSearchResult> _buildBuiltInSettingSearchResults() {
    return _builtInSettingSearchDefinitions.where(_isBuiltInSettingSearchDefinitionVisible).map((definition) {
      final title = tr(definition.titleKey);
      final subtitle = definition.subtitleKey.isEmpty ? tr(_settingNavTitleKey(definition.navPath)) : tr(definition.subtitleKey);
      // Search refinement: subtitles are explanatory result text, not stable
      // lookup terms. Keeping them out prevents broad helper copy from pulling
      // unrelated settings into short keyword searches.
      final texts = <String>[definition.settingKey, title, tr(_settingNavTitleKey(definition.navPath)), ...definition.searchKeywords.map(tr)];
      return WoxSettingSearchResult(
        type: WoxSettingSearchTargetType.builtInSetting,
        id: 'builtInSetting:${definition.settingKey}',
        title: title,
        subtitle: subtitle,
        navPath: definition.navPath,
        pluginId: '',
        settingKey: definition.settingKey,
        searchTexts: _normalizeSettingSearchTexts(texts),
        score: 0,
      );
    }).toList();
  }

  bool _isBuiltInSettingSearchDefinitionVisible(_BuiltInSettingSearchDefinition definition) {
    // Keep search aligned with runtime-disabled Wayland settings so it does not
    // navigate to controls hidden from their settings pages.
    if (!woxSetting.value.isLinuxWaylandSession) {
      return true;
    }
    // wlr-layer-shell restores precise window placement (Hyprland/sway), so the
    // ShowPosition selector stays reachable there. Selection/tray-query capture
    // and ignored-hotkey-apps remain unavailable on native Wayland regardless.
    final showPositionAvailable = linuxLayerShellSupported.value;
    return (definition.settingKey != 'ShowPosition' || showPositionAvailable) &&
        definition.settingKey != 'SelectionHotkey' &&
        definition.settingKey != 'IgnoredHotkeyApps' &&
        definition.settingKey != 'TrayQueries';
  }

  List<WoxSettingSearchResult> _buildInstalledPluginSearchResults() {
    return installedPlugins.map((plugin) {
      return WoxSettingSearchResult(
        type: WoxSettingSearchTargetType.installedPlugin,
        id: 'installedPlugin:${plugin.id}',
        title: plugin.name,
        subtitle: plugin.description.isNotEmpty ? plugin.description : plugin.id,
        navPath: 'plugins.installed',
        pluginId: plugin.id,
        settingKey: '',
        icon: plugin.icon,
        searchTexts: _normalizeSettingSearchTexts([plugin.id, plugin.name, plugin.nameEn, plugin.author, plugin.runtime, ...plugin.triggerKeywords]),
        score: 0,
      );
    }).toList();
  }

  List<WoxSettingSearchResult> _buildPluginSettingSearchResults() {
    final results = <WoxSettingSearchResult>[];
    for (final plugin in installedPlugins) {
      for (final definition in plugin.settingDefinitions) {
        final extracted = _extractPluginSettingSearchData(definition);
        final title = tr(extracted.title).trim();
        if (extracted.settingKey.isEmpty || title.isEmpty) {
          continue;
        }
        results.add(
          WoxSettingSearchResult(
            type: WoxSettingSearchTargetType.pluginSetting,
            id: 'pluginSetting:${plugin.id}:${extracted.settingKey}',
            title: title,
            subtitle: plugin.name,
            navPath: 'plugins.installed',
            pluginId: plugin.id,
            settingKey: extracted.settingKey,
            icon: plugin.icon,
            // Search refinement: the plugin name is the result subtitle for
            // plugin settings. Match the setting's own visible title/key instead
            // so searching a plugin does not fan out into every setting it owns.
            searchTexts: _normalizeSettingSearchTexts([extracted.settingKey, title, ...extracted.searchTexts]),
            score: 0,
          ),
        );
      }
    }
    return results;
  }

  int _matchSettingSearchCandidate(List<String> searchTexts, String keyword) {
    var bestScore = 0;
    for (final text in searchTexts) {
      if (text.isEmpty) {
        continue;
      }
      final match = WoxFuzzyMatchUtil.match(text: text, pattern: keyword, usePinYin: WoxSettingUtil.instance.currentSetting.usePinYin);
      if (match.isMatch && match.score > bestScore) {
        bestScore = match.score;
      }
    }
    return bestScore;
  }

  int _settingSearchTypePriority(WoxSettingSearchTargetType type) {
    switch (type) {
      case WoxSettingSearchTargetType.builtInSetting:
        return 0;
      case WoxSettingSearchTargetType.installedPlugin:
        return 1;
      case WoxSettingSearchTargetType.pluginSetting:
        return 2;
    }
  }

  List<String> _normalizeSettingSearchTexts(List<String> rawTexts) {
    final seen = <String>{};
    final texts = <String>[];
    void addText(String text) {
      final trimmed = text.trim();
      if (trimmed.isEmpty) {
        return;
      }
      final key = trimmed.toLowerCase();
      if (seen.add(key)) {
        texts.add(trimmed);
      }
    }

    for (final text in rawTexts) {
      addText(text);
      // Feature: plugin settings often define visible labels as i18n keys.
      // Search stores both the raw definition text and the rendered text so SDKs
      // do not need a separate keyword field just to make settings discoverable.
      addText(tr(text));
    }
    return texts;
  }

  String _settingNavTitleKey(String navPath) {
    switch (navPath) {
      case 'general':
        return 'ui_general';
      case 'ui':
        return 'ui_ui';
      case 'ai':
        return 'ui_ai';
      case 'network':
        return 'ui_network';
      case 'data':
        return 'ui_data';
      case 'data.backup':
        return 'ui_data_backup_restore_nav';
      case 'data.cloudsync':
        return 'ui_cloud_sync';
      case 'plugins.runtime':
        return 'ui_runtime_settings';
      case 'themes.store':
        return 'ui_store_themes';
      case 'themes.installed':
        return 'ui_installed_themes';
      case 'themes.edit':
        return 'ui_theme_editor_title';
      case 'debug':
        return 'ui_debug';
      case 'update':
        return 'ui_update';
      case 'privacy':
        return 'ui_privacy';
      default:
        return 'ui_settings';
    }
  }

  _PluginSettingSearchData _extractPluginSettingSearchData(PluginSettingDefinitionItem definition) {
    final value = definition.value;
    // Feature refinement: plugin setting search now stays on visible titles,
    // labels, option text, and stable keys. Tooltip text is hover help and made
    // broad words surface unrelated settings too often.
    if (value is PluginSettingValueCheckBox) {
      return _PluginSettingSearchData(settingKey: value.key, title: value.label, searchTexts: [value.label, value.key]);
    }
    if (value is PluginSettingValueTextBox) {
      return _PluginSettingSearchData(settingKey: value.key, title: value.label, searchTexts: [value.label, value.key, value.suffix]);
    }
    if (value is PluginSettingValueSelect) {
      return _PluginSettingSearchData(
        settingKey: value.key,
        title: value.label,
        searchTexts: [
          value.label,
          value.key,
          value.suffix,
          ...value.options.expand((option) => [option.label, option.value]),
        ],
      );
    }
    if (value is PluginSettingValueSelectAIModel) {
      return _PluginSettingSearchData(settingKey: value.key, title: value.label, searchTexts: [value.label, value.key, value.suffix]);
    }
    if (value is PluginSettingValueTable) {
      final title = value.title.trim().isNotEmpty ? value.title : value.key;
      return _PluginSettingSearchData(
        settingKey: value.key,
        title: title,
        searchTexts: [
          title,
          value.key,
          ...value.columns.expand((column) => [column.key, column.label]),
        ],
      );
    }
    if (value is PluginSettingValueHead) {
      return _PluginSettingSearchData(settingKey: '', title: value.content, searchTexts: [value.content]);
    }
    if (value is PluginSettingValueLabel) {
      return _PluginSettingSearchData(settingKey: '', title: value.content, searchTexts: [value.content]);
    }
    return const _PluginSettingSearchData(settingKey: '', title: '', searchTexts: []);
  }

  void focusGeneralSection(String sectionId) {
    final request = _parseGeneralSectionFocusRequest(sectionId);
    if (request.sectionId.isEmpty) {
      return;
    }

    _pendingGeneralSectionAnchor = request.sectionId;
    pendingTrayQueryEditRowIndex.value = request.trayQueryEditRowIndex;
    _schedulePendingGeneralSectionFocus();
  }

  int? consumePendingTrayQueryEditRowIndex() {
    final rowIndex = pendingTrayQueryEditRowIndex.value;
    pendingTrayQueryEditRowIndex.value = null;
    return rowIndex;
  }

  void notifyGeneralViewReady() {
    if (_pendingGeneralSectionAnchor.isEmpty) {
      return;
    }

    _schedulePendingGeneralSectionFocus();
  }

  void _schedulePendingGeneralSectionFocus({int attempt = 0}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_pendingGeneralSectionAnchor.isEmpty || activeNavPath.value != 'general') {
        return;
      }

      final targetKey = generalSectionKeys[_pendingGeneralSectionAnchor];
      final targetContext = targetKey?.currentContext;
      if (targetContext == null) {
        if (attempt >= 10) {
          return;
        }

        Future.delayed(const Duration(milliseconds: 80), () {
          _schedulePendingGeneralSectionFocus(attempt: attempt + 1);
        });
        return;
      }

      Scrollable.ensureVisible(targetContext, duration: const Duration(milliseconds: 220), curve: Curves.easeOutCubic, alignment: 0.1);
      _pendingGeneralSectionAnchor = '';
    });
  }

  _GeneralSectionFocusRequest _parseGeneralSectionFocusRequest(String rawRequest) {
    final normalizedRequest = rawRequest.trim();
    if (normalizedRequest.isEmpty) {
      return const _GeneralSectionFocusRequest(sectionId: '');
    }

    final separatorIndex = normalizedRequest.indexOf(':');
    if (separatorIndex < 0) {
      return _GeneralSectionFocusRequest(sectionId: normalizedRequest);
    }

    final sectionId = normalizedRequest.substring(0, separatorIndex).trim();
    final rawRowIndex = normalizedRequest.substring(separatorIndex + 1).trim();
    final rowIndex = int.tryParse(rawRowIndex);
    return _GeneralSectionFocusRequest(sectionId: sectionId, trayQueryEditRowIndex: rowIndex);
  }

  Future<void> loadLang(String langCode) async {
    final traceId = const UuidV4().generate();
    langMap.value = await WoxApi.instance.getLangJson(traceId, langCode);
  }

  Future<void> loadWoxVersion() async {
    final traceId = const UuidV4().generate();
    try {
      final version = await WoxApi.instance.getWoxVersion(traceId);
      woxVersion.value = version;
    } catch (e) {
      woxVersion.value = '';
      Logger.instance.error(traceId, 'Failed to load Wox version: $e');
    }
  }

  /// Loads latest manifest versions for stable and beta update channels.
  Future<void> loadUpdateChannelVersions() async {
    final traceId = const UuidV4().generate();
    try {
      final versions = await WoxApi.instance.getUpdateChannelVersions(traceId);
      updateChannelVersions.assignAll(versions);
    } catch (e) {
      updateChannelVersions.clear();
      Logger.instance.error(traceId, 'Failed to load update channel versions: $e');
    }
  }

  /// Formats a channel version for compact display inside the update channel dropdown.
  String getUpdateChannelVersionText(String channel) {
    for (final version in updateChannelVersions) {
      if (version.channel == channel && version.latestVersion.trim().isNotEmpty) {
        final latestVersion = version.latestVersion.trim();
        return latestVersion.startsWith('v') ? latestVersion : 'v$latestVersion';
      }
    }
    return '';
  }

  Future<void> refreshRuntimeStatuses() async {
    isRuntimeStatusLoading.value = true;
    runtimeStatusError.value = '';
    final traceId = const UuidV4().generate();
    try {
      final statuses = await WoxApi.instance.getRuntimeStatuses(traceId);
      runtimeStatuses.assignAll(statuses);
      Logger.instance.info(traceId, 'Runtime statuses loaded, count: ${statuses.length}');
    } catch (e) {
      runtimeStatuses.clear();
      runtimeStatusError.value = e.toString();
      Logger.instance.error(traceId, 'Failed to load runtime statuses: $e');
    } finally {
      isRuntimeStatusLoading.value = false;
    }
  }

  WoxRuntimeStatus? getRuntimeStatus(String runtime) {
    final normalizedRuntime = runtime.toUpperCase();
    for (final status in runtimeStatuses) {
      if (status.runtime.toUpperCase() == normalizedRuntime) {
        return status;
      }
    }
    return null;
  }

  WoxRuntimeStatus? getActionableRuntimeStatusForPlugin(PluginDetail plugin) {
    if (!WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.NODEJS) && !WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.PYTHON)) {
      return null;
    }

    final status = getRuntimeStatus(plugin.runtime);
    if (status == null || !status.isActionableFailure) {
      return null;
    }

    return status;
  }

  Future<void> restartRuntime(WoxRuntimeStatus status) async {
    if (!status.canRestart || restartingRuntime.value.isNotEmpty) {
      return;
    }

    final traceId = const UuidV4().generate();
    final runtime = status.runtime.toUpperCase();
    try {
      // Feature: restarting a runtime host from settings makes path fixes usable
      // immediately and keeps users out of the old "restart Wox and retry" loop.
      runtimeStatusError.value = '';
      restartingRuntime.value = runtime;
      await WoxApi.instance.restartRuntime(traceId, runtime);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to restart runtime $runtime: $e');
      runtimeStatusError.value = e.toString();
    } finally {
      restartingRuntime.value = '';
      await refreshRuntimeStatuses();
    }
  }

  Future<void> openRuntimeInstallUrl(WoxRuntimeStatus status) async {
    if (status.installUrl.isEmpty) {
      return;
    }

    final uri = Uri.tryParse(status.installUrl);
    if (uri == null) {
      return;
    }
    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  Future<void> refreshUsageStats({String? period}) async {
    if (period != null) {
      usageStatsPeriod.value = period;
    }

    isUsageStatsLoading.value = true;
    usageStatsError.value = '';
    final traceId = const UuidV4().generate();
    try {
      final stats = await WoxApi.instance.getUsageStats(traceId, period: usageStatsPeriod.value);
      usageStats.value = stats;
      Logger.instance.info(traceId, 'Usage stats loaded for period ${usageStatsPeriod.value}');
    } catch (e) {
      usageStats.value = WoxUsageStats.empty();
      usageStatsError.value = e.toString();
      Logger.instance.error(traceId, 'Failed to load usage stats: $e');
    } finally {
      isUsageStatsLoading.value = false;
    }
  }

  Future<void> loadSystemFontFamilies() async {
    final traceId = const UuidV4().generate();
    try {
      final families = await WoxApi.instance.getSystemFontFamilies(traceId);
      final normalized = families.map((family) => family.trim()).where((family) => family.isNotEmpty).toSet().toList()..sort((a, b) => a.toLowerCase().compareTo(b.toLowerCase()));
      systemFontFamilies.assignAll(normalized);
      Logger.instance.info(traceId, 'System font families loaded, count: ${normalized.length}');
    } catch (e) {
      systemFontFamilies.clear();
      Logger.instance.error(traceId, 'Failed to load system font families: $e');
    }
  }

  void hideWindow(String traceId) {
    Get.find<WoxLauncherController>().exitSetting(traceId);
  }

  Future<void> updateConfig(String key, String value) async {
    final traceId = const UuidV4().generate();
    await WoxApi.instance.updateSetting(traceId, key, value);
    await reloadSetting(traceId);
    Logger.instance.info(traceId, 'Setting updated: $key=$value');

    if (key == "AIProviders") {
      invalidateAIModelSelectorResources();
      unawaited(loadAIModelSelectorResources(traceId: traceId, forceRefresh: true));
    }

    // If user switches to last_location, save current window position immediately
    if (key == "ShowPosition" && value == WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code) {
      try {
        final launcherController = Get.find<WoxLauncherController>();
        launcherController.saveWindowPositionIfNeeded(reason: "setting-switch");
        Logger.instance.info(traceId, 'Saved current window position when switching to last_location');
      } catch (e) {
        Logger.instance.error(traceId, 'Failed to save window position when switching to last_location: $e');
      }
    }

    // Sync lastLaunchMode immediately so hideApp uses the correct mode
    // without waiting for the next show cycle from the backend.
    if (key == "LaunchMode") {
      try {
        final launcherController = Get.find<WoxLauncherController>();
        launcherController.lastLaunchMode = value;
        Logger.instance.info(traceId, 'Synced lastLaunchMode to $value');
      } catch (e) {
        Logger.instance.error(traceId, 'Failed to sync lastLaunchMode: $e');
      }
    }
  }


  // updateLang persists the new language code. The language json and plugin
  // translation refresh is handled by reloadSetting, which updateConfig invokes
  // internally after detecting the LangCode change. Calling _applyLangResourcesChange
  // here as well would duplicate the work, so it is intentionally omitted.
  Future<void> updateLang(String langCode) async {
    await updateConfig("LangCode", langCode);
  }

  // _applyLangResourcesChange reloads the language json and refreshes plugin
  // translations. It is shared by the manual language switch (updateLang) and
  // by reloadSetting when a remote change (e.g. cloud sync bootstrap restore)
  // updates LangCode without going through updateConfig, so the UI keeps
  // langMap and plugin translations in sync with the persisted value.
  Future<void> _applyLangResourcesChange(String langCode) async {
    final traceId = const UuidV4().generate();
    langMap.value = await WoxApi.instance.getLangJson(traceId, langCode);

    // Refresh all loaded plugins to update translations
    // Reload installed plugins list
    if (installedPlugins.isNotEmpty) {
      await loadInstalledPlugins(traceId);
    }

    // Reload store plugins list if loaded
    if (storePlugins.isNotEmpty) {
      await loadStorePlugins(traceId);
    }

    // Refresh current view
    if (activeNavPath.value == 'plugins.installed' || activeNavPath.value == 'plugins.store') {
      await switchToPluginList(traceId, isStorePluginList.value);
    }

    // Refresh active plugin detail if one is selected
    if (activePlugin.value.id.isNotEmpty) {
      await refreshPlugin(activePlugin.value.id, "update");
    }
  }

  // get translation
  String tr(String key) {
    if (key.startsWith("i18n:")) {
      key = key.substring(5);
    }

    return langMap[key] ?? key;
  }

  // ---------- Plugins ----------

  Future<void> loadStorePlugins(String traceId) async {
    try {
      var start = DateTime.now();
      final storePluginsFromAPI = await WoxApi.instance.findStorePlugins(traceId);
      storePluginsFromAPI.sort((a, b) => a.name.compareTo(b.name));
      storePlugins.clear();
      storePlugins.addAll(storePluginsFromAPI);
      Logger.instance.info(traceId, 'Store plugins loaded, cost ${DateTime.now().difference(start).inMilliseconds} ms');
    } catch (e) {
      storePlugins.clear();
      Logger.instance.error(traceId, 'Failed to load store plugins: $e');
    }
  }

  Future<void> loadInstalledPlugins(String traceId) async {
    try {
      var start = DateTime.now();
      final installedPluginsFromAPI = await WoxApi.instance.findInstalledPlugins(traceId);
      installedPluginsFromAPI.sort((a, b) => a.name.compareTo(b.name));
      installedPlugins.clear();
      installedPlugins.addAll(installedPluginsFromAPI);
      Logger.instance.info(traceId, 'Installed plugins loaded, cost ${DateTime.now().difference(start).inMilliseconds} ms');
    } catch (e) {
      installedPlugins.clear();
      Logger.instance.error(traceId, 'Failed to load installed plugins: $e');
    } finally {
      installedPluginListRevision.value++;
    }
  }

  /// Preload both plugin lists at startup without awaiting to avoid blocking UI.
  void preloadPlugins(String traceId) {
    unawaited(loadInstalledPlugins(traceId));
    unawaited(loadStorePlugins(traceId));
  }

  /// Drop store-side caches (plugins + themes) so hidden window memory is released.
  /// Both lists are lazily reloaded by their respective load methods when the
  /// settings view is opened again.
  void clearStoreCache() {
    storePlugins.clear();
    storeThemesList.clear();
    _hasLoadedStoreThemes = false;
  }

  Future<void> refreshSettingGlancePreviewsForUIEntry(String traceId) async {
    if (_hasRefreshedSettingGlancePreviewsForUIEntry) {
      return;
    }

    _hasRefreshedSettingGlancePreviewsForUIEntry = true;
    settingGlancePreviewItems.clear();
    if (installedPlugins.isEmpty) {
      await loadInstalledPlugins(traceId);
    }
    await refreshSettingGlancePreviews(traceId);
  }

  Future<void> refreshSettingGlancePreviews(String traceId) async {
    final refs = <GlanceRef>[];
    for (final plugin in installedPlugins) {
      for (final glance in plugin.glances) {
        refs.add(GlanceRef(pluginId: plugin.id, glanceId: glance.id));
      }
    }

    if (refs.isEmpty) {
      settingGlancePreviewItems.clear();
      return;
    }

    if (_isRefreshingSettingGlancePreviews) {
      return;
    }

    _isRefreshingSettingGlancePreviews = true;
    try {
      // The UI settings page asks the same Glance API once when it becomes
      // visible, so the dropdown is a real snapshot without a background cache.
      final items = await WoxApi.instance.getGlanceItems(traceId, refs, "manualRefresh");
      final nextItems = <String, GlanceItem>{};
      for (final item in items) {
        if (!item.isEmpty) {
          nextItems[GlanceRef(pluginId: item.pluginId, glanceId: item.id).key] = item;
        }
      }
      settingGlancePreviewItems.assignAll(nextItems);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to refresh setting glance previews: $e');
    } finally {
      _isRefreshingSettingGlancePreviews = false;
    }
  }

  Future<void> reloadPlugins(String traceId) async {
    final currentActivePluginId = activePlugin.value.id;

    await Future.wait([loadInstalledPlugins(traceId), loadStorePlugins(traceId)]);

    if (activeNavPath.value != 'plugins.installed' && activeNavPath.value != 'plugins.store') {
      return;
    }

    filterPlugins();

    if (pendingInstalledPluginFocusRef.isNotEmpty) {
      final focused = focusInstalledPlugin(pendingInstalledPluginFocusRef, keepPendingFocus: true);
      if (focused) {
        pendingInstalledPluginFocusRef = '';
        return;
      }
    }

    if (currentActivePluginId.isEmpty) {
      setFirstFilteredPluginDetailActive();
      return;
    }

    syncActivePluginWithFilteredList(currentActivePluginId: currentActivePluginId);
  }

  Future<void> refreshPlugin(String pluginId, String refreshType /* update / add / remove */) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'Refreshing plugin: $pluginId, refreshType: $refreshType');
    if (refreshType == "add") {
      PluginDetail updatedPlugin = await WoxApi.instance.getPluginDetail(traceId, pluginId);
      if (updatedPlugin.id.isEmpty) {
        Logger.instance.info(traceId, 'Plugin not found: $pluginId');
        return;
      }

      int storeIndex = storePlugins.indexWhere((p) => p.id == pluginId);
      if (storeIndex >= 0) {
        storePlugins[storeIndex] = updatedPlugin;
      }
      int installedIndex = installedPlugins.indexWhere((p) => p.id == pluginId);
      if (installedIndex >= 0) {
        installedPlugins[installedIndex] = updatedPlugin;
      } else {
        installedPlugins.add(updatedPlugin);
      }
      int pluginListIndex = pluginList.indexWhere((p) => p.id == pluginId);
      if (pluginListIndex >= 0) {
        pluginList[pluginListIndex] = updatedPlugin;
      } else if (activeNavPath.value == 'plugins.installed') {
        pluginList.add(updatedPlugin);
      }
      int filteredPluginListIndex = filteredPluginList.indexWhere((p) => p.id == pluginId);
      if (filteredPluginListIndex >= 0) {
        filteredPluginList[filteredPluginListIndex] = updatedPlugin;
      } else {
        filteredPluginList.add(updatedPlugin);
      }
      if (activePlugin.value.id == pluginId) {
        activePlugin.value = updatedPlugin;
      }
    } else if (refreshType == "remove") {
      installedPlugins.removeWhere((p) => p.id == pluginId);
      int storeIndex = storePlugins.indexWhere((p) => p.id == pluginId);
      if (storeIndex >= 0) {
        storePlugins[storeIndex].isInstalled = false;
      }
      // if is in installed plugin view, remove from plugin list
      if (activeNavPath.value == 'plugins.installed') {
        pluginList.removeWhere((p) => p.id == pluginId);
        filteredPluginList.removeWhere((p) => p.id == pluginId);
      }
      // if is in store plugin view, update the installed property
      if (activeNavPath.value == 'plugins.store') {
        pluginList.firstWhere((p) => p.id == pluginId).isInstalled = false;
        filteredPluginList.firstWhere((p) => p.id == pluginId).isInstalled = false;
      }
      if (activePlugin.value.id == pluginId) {
        activePlugin.value = installedPlugins.isNotEmpty ? installedPlugins[0] : PluginDetail.empty();
      }
    } else if (refreshType == "update") {
      PluginDetail updatedPlugin = await WoxApi.instance.getPluginDetail(traceId, pluginId);
      if (updatedPlugin.id.isEmpty) {
        Logger.instance.info(traceId, 'Plugin not found: $pluginId');
        return;
      }

      int installedIndex = installedPlugins.indexWhere((p) => p.id == pluginId);
      if (installedIndex >= 0) {
        installedPlugins[installedIndex] = updatedPlugin;
      }
      int storeIndex = storePlugins.indexWhere((p) => p.id == pluginId);
      if (storeIndex >= 0) {
        storePlugins[storeIndex] = updatedPlugin;
      }
      int pluginListIndex = pluginList.indexWhere((p) => p.id == pluginId);
      if (pluginListIndex >= 0) {
        pluginList[pluginListIndex] = updatedPlugin;
      }
      int filteredPluginListIndex = filteredPluginList.indexWhere((p) => p.id == pluginId);
      if (filteredPluginListIndex >= 0) {
        filteredPluginList[filteredPluginListIndex] = updatedPlugin;
      }
      if (activePlugin.value.id == pluginId) {
        activePlugin.value = updatedPlugin;
      }
    }

    filterPlugins();
    syncActivePluginWithFilteredList();
  }

  Future<void> switchToPluginList(String traceId, bool isStorePlugin) async {
    Logger.instance.info(traceId, 'Switching to plugin list: $isStorePlugin');
    if (isStorePlugin) {
      pluginList.clear();
      pluginList.addAll(storePlugins);
    } else {
      pluginList.clear();
      pluginList.addAll(installedPlugins);
    }

    activeNavPath.value = isStorePlugin ? 'plugins.store' : 'plugins.installed';
    isStorePluginList.value = isStorePlugin;
    activePlugin.value = PluginDetail.empty();
    filterPluginKeywordController.text = "";

    filterPlugins();

    //active plugin
    if (activePlugin.value.id.isNotEmpty) {
      activePlugin.value = filteredPluginList.firstWhere(
        (element) => element.id == activePlugin.value.id,
        orElse: () => filteredPluginList.isNotEmpty ? filteredPluginList[0] : PluginDetail.empty(),
      );
    } else {
      setFirstFilteredPluginDetailActive();
    }
  }

  Future<void> switchToDataView(String traceId) async {
    await switchToBackupView(traceId);
  }

  Future<void> switchToBackupView(String traceId) async {
    activeNavPath.value = 'data.backup';
    await refreshBackups();
  }

  Future<void> switchToCloudSyncView(String traceId) async {
    activeNavPath.value = 'data.cloudsync';
    await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus(), refreshCloudSyncBillingPlan(), loadInstalledPlugins(traceId)]);
    if (accountStatus.value.loggedIn) {
      await refreshCloudSyncDevices(showLoading: false);
    }
    _updateCloudSyncStatusWaiting();
  }

  GlobalKey getPluginListItemKey(String pluginId) {
    return pluginListItemKeys.putIfAbsent(pluginId, () => GlobalKey());
  }

  Future<void> ensurePluginVisible(String pluginId) async {
    final targetIndex = filteredPluginList.indexWhere((plugin) => plugin.id == pluginId);
    if (targetIndex < 0) {
      return;
    }
    final itemKey = pluginListItemKeys[pluginId];

    WidgetsBinding.instance.addPostFrameCallback((_) async {
      if (pluginListScrollController.hasClients) {
        const estimatedItemExtent = 88.0;
        final targetOffset = targetIndex * estimatedItemExtent;
        final maxExtent = pluginListScrollController.position.maxScrollExtent;
        final clampedOffset = targetOffset.clamp(0.0, maxExtent);

        if ((pluginListScrollController.offset - clampedOffset).abs() > 4) {
          await pluginListScrollController.animateTo(clampedOffset, duration: const Duration(milliseconds: 180), curve: Curves.easeOutCubic);
        }
      }

      WidgetsBinding.instance.addPostFrameCallback((_) {
        final itemContext = itemKey?.currentContext;
        if (itemContext != null) {
          Scrollable.ensureVisible(itemContext, duration: const Duration(milliseconds: 120), curve: Curves.easeOutCubic, alignment: 0.5);
        }
      });
    });
  }

  void resetPluginFilters() {
    filterPluginKeywordController.text = "";
    filterEnabledPluginsOnly.value = false;
    filterDisabledPluginsOnly.value = false;
    filterUpgradablePluginsOnly.value = false;
    filterUninstalledPluginsOnly.value = false;
    filterThirdPartyPluginsOnly.value = false;
    filterRuntimeNodejsOnly.value = false;
    filterRuntimePythonOnly.value = false;
    filterRuntimeScriptOnly.value = false;
    filterRuntimeScriptNodejsOnly.value = false;
    filterRuntimeScriptPythonOnly.value = false;
  }

  bool focusInstalledPlugin(String pluginRef, {bool keepPendingFocus = false}) {
    if (!keepPendingFocus) {
      pendingInstalledPluginFocusRef = pluginRef;
    }

    resetPluginFilters();
    filterPlugins();

    if (pluginRef.isEmpty) {
      setFirstFilteredPluginDetailActive();
      ensurePluginVisible(activePlugin.value.id);
      return activePlugin.value.id.isNotEmpty;
    }

    final exactPluginIndex = filteredPluginList.indexWhere((plugin) => plugin.id == pluginRef);
    if (exactPluginIndex >= 0) {
      activePlugin.value = filteredPluginList[exactPluginIndex];
      ensurePluginVisible(activePlugin.value.id);
      return true;
    }

    filterPluginKeywordController.text = pluginRef;
    filterPlugins();
    setFirstFilteredPluginDetailActive();
    ensurePluginVisible(activePlugin.value.id);
    return activePlugin.value.id.isNotEmpty;
  }

  void setFirstFilteredPluginDetailActive() {
    if (filteredPluginList.isNotEmpty) {
      activePlugin.value = filteredPluginList[0];
      return;
    }

    activePlugin.value = PluginDetail.empty();
  }

  void syncActivePluginWithFilteredList({String? currentActivePluginId, bool preserveActivePlugin = false}) {
    if (filteredPluginList.isEmpty) {
      if (preserveActivePlugin) {
        return;
      }

      activePlugin.value = PluginDetail.empty();
      return;
    }

    final targetPluginId = currentActivePluginId ?? activePlugin.value.id;
    if (targetPluginId.isEmpty) {
      setFirstFilteredPluginDetailActive();
      return;
    }

    final idx = filteredPluginList.indexWhere((plugin) => plugin.id == targetPluginId);
    if (idx >= 0) {
      final matchedPlugin = filteredPluginList[idx];
      if (currentActivePluginId == null && activePlugin.value.id == matchedPlugin.id) {
        return;
      }

      activePlugin.value = matchedPlugin;
      return;
    }

    // Search filtering should not switch the detail pane while the user is
    // typing. The old fallback selected the first matching plugin on every
    // filter miss, rebuilding the right-side settings and causing the root
    // settings focus node to interrupt text input.
    if (preserveActivePlugin) {
      return;
    }

    setFirstFilteredPluginDetailActive();
  }

  void handlePluginSearchChanged() {
    filterPlugins();
    syncActivePluginWithFilteredList(preserveActivePlugin: true);
  }

  bool get hasInstalledRuntimePluginFilterApplied =>
      filterRuntimeNodejsOnly.value || filterRuntimePythonOnly.value || filterRuntimeScriptNodejsOnly.value || filterRuntimeScriptPythonOnly.value;

  bool get hasStoreRuntimePluginFilterApplied => filterRuntimeNodejsOnly.value || filterRuntimePythonOnly.value || filterRuntimeScriptOnly.value;

  bool get hasInstalledPluginFilterApplied =>
      filterEnabledPluginsOnly.value ||
      filterDisabledPluginsOnly.value ||
      filterUpgradablePluginsOnly.value ||
      filterThirdPartyPluginsOnly.value ||
      hasInstalledRuntimePluginFilterApplied;

  bool get hasStorePluginFilterApplied => filterUninstalledPluginsOnly.value || filterThirdPartyPluginsOnly.value || hasStoreRuntimePluginFilterApplied;

  bool get hasPluginFilterApplied => isStorePluginList.value ? hasStorePluginFilterApplied : hasInstalledPluginFilterApplied;

  void updatePluginFilters({
    bool? enabledOnly,
    bool? disabledOnly,
    bool? upgradableOnly,
    bool? uninstalledOnly,
    bool? thirdPartyOnly,
    bool? runtimeNodejsOnly,
    bool? runtimePythonOnly,
    bool? runtimeScriptOnly,
    bool? runtimeScriptNodejsOnly,
    bool? runtimeScriptPythonOnly,
  }) {
    if (enabledOnly != null) {
      filterEnabledPluginsOnly.value = enabledOnly;
    }
    if (disabledOnly != null) {
      filterDisabledPluginsOnly.value = disabledOnly;
    }
    if (upgradableOnly != null) {
      filterUpgradablePluginsOnly.value = upgradableOnly;
    }
    if (uninstalledOnly != null) {
      filterUninstalledPluginsOnly.value = uninstalledOnly;
    }
    if (thirdPartyOnly != null) {
      filterThirdPartyPluginsOnly.value = thirdPartyOnly;
    }
    if (runtimeNodejsOnly != null) {
      filterRuntimeNodejsOnly.value = runtimeNodejsOnly;
    }
    if (runtimePythonOnly != null) {
      filterRuntimePythonOnly.value = runtimePythonOnly;
    }
    if (runtimeScriptOnly != null) {
      filterRuntimeScriptOnly.value = runtimeScriptOnly;
    }
    if (runtimeScriptNodejsOnly != null) {
      filterRuntimeScriptNodejsOnly.value = runtimeScriptNodejsOnly;
    }
    if (runtimeScriptPythonOnly != null) {
      filterRuntimeScriptPythonOnly.value = runtimeScriptPythonOnly;
    }

    filterPlugins();
    syncActivePluginWithFilteredList();
  }

  bool isScriptNodejsPlugin(PluginDetail plugin) {
    if (!WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.SCRIPT)) {
      return false;
    }

    return plugin.entry.toLowerCase().endsWith('.js');
  }

  bool isScriptPythonPlugin(PluginDetail plugin) {
    if (!WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.SCRIPT)) {
      return false;
    }

    return plugin.entry.toLowerCase().endsWith('.py');
  }

  Future<void> installPlugin(PluginDetail plugin) async {
    try {
      pluginInstallError.value = '';
      isInstallingPlugin.value = true;
      final traceId = const UuidV4().generate();
      Logger.instance.info(traceId, 'installing plugin: ${plugin.name}');
      await WoxApi.instance.installPlugin(traceId, plugin.id);
      await refreshPlugin(plugin.id, "add");
    } catch (e) {
      final traceId = const UuidV4().generate();
      Logger.instance.error(traceId, 'Failed to install plugin ${plugin.name}: $e');
      pluginInstallError.value = e.toString();
      // Bug fix: install errors used to display only a raw exception. Refreshing
      // runtime status lets the detail pane show whether Node.js/Python is
      // missing, unsupported, or failed to launch, plus the recovery action.
      if (WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.NODEJS) || WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.PYTHON)) {
        await refreshRuntimeStatuses();
      }
    } finally {
      isInstallingPlugin.value = false;
    }
  }

  Future<void> upgradePlugin(PluginDetail plugin) async {
    if (isUpgradingPlugin.value) {
      return;
    }

    try {
      pluginInstallError.value = '';
      isUpgradingPlugin.value = true;
      final traceId = const UuidV4().generate();
      Logger.instance.info(traceId, 'upgrading plugin: ${plugin.name}');
      // Keep the same upgrade path as WPM plugin: install from store by plugin id.
      await WoxApi.instance.installPlugin(traceId, plugin.id);
      await refreshPlugin(plugin.id, "update");
    } catch (e) {
      final traceId = const UuidV4().generate();
      Logger.instance.error(traceId, 'Failed to upgrade plugin ${plugin.name}: $e');
      pluginInstallError.value = e.toString();
      if (WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.NODEJS) || WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.PYTHON)) {
        await refreshRuntimeStatuses();
      }
    } finally {
      isUpgradingPlugin.value = false;
    }
  }

  Future<void> disablePlugin(PluginDetail plugin) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'disabling plugin: ${plugin.name}');
    await WoxApi.instance.disablePlugin(traceId, plugin.id);
    await refreshPlugin(plugin.id, "update");
  }

  Future<void> enablePlugin(PluginDetail plugin) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'enabling plugin: ${plugin.name}');
    await WoxApi.instance.enablePlugin(traceId, plugin.id);
    await refreshPlugin(plugin.id, "update");
  }

  Future<void> uninstallPlugin(PluginDetail plugin) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'uninstalling plugin: ${plugin.name}');
    await WoxApi.instance.uninstallPlugin(traceId, plugin.id);
    await refreshPlugin(plugin.id, "remove");
  }

  void filterPlugins() {
    final keyword = filterPluginKeywordController.text;
    final isStoreView = isStorePluginList.value;
    final enabledOnly = !isStoreView && filterEnabledPluginsOnly.value;
    final disabledOnly = !isStoreView && filterDisabledPluginsOnly.value;
    final upgradableOnly = !isStoreView && filterUpgradablePluginsOnly.value;
    final uninstalledOnly = isStoreView && filterUninstalledPluginsOnly.value;
    final thirdPartyOnly = filterThirdPartyPluginsOnly.value;
    final runtimeNodejsOnly = filterRuntimeNodejsOnly.value;
    final runtimePythonOnly = filterRuntimePythonOnly.value;
    final runtimeScriptOnly = filterRuntimeScriptOnly.value;
    final runtimeScriptNodejsOnly = filterRuntimeScriptNodejsOnly.value;
    final runtimeScriptPythonOnly = filterRuntimeScriptPythonOnly.value;

    bool matchesKeyword(PluginDetail plugin) {
      if (keyword.isEmpty) {
        return true;
      }

      bool match = WoxFuzzyMatchUtil.isFuzzyMatch(text: plugin.name, pattern: keyword, usePinYin: WoxSettingUtil.instance.currentSetting.usePinYin);
      if (match) {
        return true;
      }

      if (plugin.nameEn.isNotEmpty) {
        match = WoxFuzzyMatchUtil.isFuzzyMatch(text: plugin.nameEn, pattern: keyword, usePinYin: false);
        if (match) {
          return true;
        }
      }

      if (plugin.description.toLowerCase().contains(keyword.toLowerCase())) {
        return true;
      }

      if (plugin.descriptionEn.toLowerCase().contains(keyword.toLowerCase())) {
        return true;
      }

      return false;
    }

    bool matchesAdvancedFilter(PluginDetail plugin) {
      if (enabledOnly && plugin.isDisable) {
        return false;
      }
      if (disabledOnly && !plugin.isDisable) {
        return false;
      }
      if (upgradableOnly && !plugin.isUpgradable) {
        return false;
      }
      if (uninstalledOnly && plugin.isInstalled) {
        return false;
      }
      // Feature: third-party filtering should use the existing IsSystem marker
      // instead of inferring ownership from author text, because official plugin
      // authors can vary while IsSystem is the stable contract from core/store data.
      if (thirdPartyOnly && plugin.isSystem) {
        return false;
      }

      final runtimeFilterApplied = runtimeNodejsOnly || runtimePythonOnly || (isStoreView ? runtimeScriptOnly : (runtimeScriptNodejsOnly || runtimeScriptPythonOnly));
      if (!runtimeFilterApplied) {
        return true;
      }

      final matchRuntimeNodejs = runtimeNodejsOnly && WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.NODEJS);
      final matchRuntimePython = runtimePythonOnly && WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.PYTHON);
      final matchRuntimeScript = isStoreView && runtimeScriptOnly && WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.SCRIPT);
      final matchRuntimeScriptNodejs = runtimeScriptNodejsOnly && isScriptNodejsPlugin(plugin);
      final matchRuntimeScriptPython = runtimeScriptPythonOnly && isScriptPythonPlugin(plugin);

      if (!(matchRuntimeNodejs || matchRuntimePython || matchRuntimeScript || matchRuntimeScriptNodejs || matchRuntimeScriptPython)) {
        return false;
      }

      return true;
    }

    final filtered = pluginList.where((plugin) => matchesKeyword(plugin) && matchesAdvancedFilter(plugin)).toList();
    filteredPluginList.assignAll(filtered);
  }

  Future<void> openPluginWebsite(String website) async {
    await launchUrl(Uri.parse(website));
  }

  Future<void> openPluginDirectory(PluginDetail plugin) async {
    final directory = plugin.pluginDirectory;
    if (directory.isEmpty) {
      return;
    }
    await openFolder(directory);
  }

  Future<String?> updatePluginSetting(String pluginId, String key, String value) async {
    final traceId = const UuidV4().generate();
    final activeTabIndex = activePluginTabController?.index ?? 0;
    final previousValue = getPluginSettingValue(pluginId, key);
    applyPluginSettingOptimistically(pluginId, key, value);

    final saveStart = DateTime.now();
    try {
      await WoxApi.instance.updatePluginSetting(traceId, pluginId, key, value);
      Logger.instance.info(traceId, 'plugin setting saved: $key=$value, cost ${DateTime.now().difference(saveStart).inMilliseconds} ms');
    } catch (e) {
      Logger.instance.error(traceId, 'failed to save plugin setting: $key=$value, error: $e');
      restorePluginSetting(pluginId, key, previousValue);
      return e.toString().replaceFirst('Exception: ', '');
    }

    unawaited(refreshPluginAfterSettingUpdate(pluginId, activeTabIndex, traceId));
    return null;
  }

  String? getPluginSettingValue(String pluginId, String key) {
    PluginDetail? target;

    if (activePlugin.value.id == pluginId) {
      target = activePlugin.value;
    } else {
      for (final plugin in [...installedPlugins, ...storePlugins, ...pluginList, ...filteredPluginList]) {
        if (plugin.id == pluginId) {
          target = plugin;
          break;
        }
      }
    }

    if (target == null) {
      return null;
    }

    if (!target.setting.settings.containsKey(key)) {
      return null;
    }

    return target.setting.settings[key];
  }

  // Optimistically update the plugin setting in all relevant lists to provide instant feedback in the UI,
  // instead of waiting for the API response.
  void applyPluginSettingOptimistically(String pluginId, String key, String value) {
    bool updatePlugin(List<PluginDetail> plugins) {
      var updated = false;
      for (final plugin in plugins) {
        if (plugin.id != pluginId) {
          continue;
        }
        plugin.setting.settings[key] = value;
        updated = true;
      }
      return updated;
    }

    final active = activePlugin.value;
    if (active.id == pluginId) {
      active.setting.settings[key] = value;
      activePlugin.refresh();
    }

    updatePlugin(installedPlugins);
    updatePlugin(storePlugins);
    updatePlugin(pluginList);
    if (updatePlugin(filteredPluginList)) {
      filteredPluginList.refresh();
    }
  }

  void restorePluginSetting(String pluginId, String key, String? previousValue) {
    bool updatePlugin(List<PluginDetail> plugins) {
      var updated = false;
      for (final plugin in plugins) {
        if (plugin.id != pluginId) {
          continue;
        }

        if (previousValue == null) {
          plugin.setting.settings.remove(key);
        } else {
          plugin.setting.settings[key] = previousValue;
        }
        updated = true;
      }
      return updated;
    }

    final active = activePlugin.value;
    if (active.id == pluginId) {
      if (previousValue == null) {
        active.setting.settings.remove(key);
      } else {
        active.setting.settings[key] = previousValue;
      }
      activePlugin.refresh();
    }

    updatePlugin(installedPlugins);
    updatePlugin(storePlugins);
    updatePlugin(pluginList);
    if (updatePlugin(filteredPluginList)) {
      filteredPluginList.refresh();
    }
  }

  Future<void> refreshPluginAfterSettingUpdate(String pluginId, int activeTabIndex, String traceId) async {
    final refreshStart = DateTime.now();
    await refreshPlugin(pluginId, "update");
    Logger.instance.info(traceId, 'plugin detail refreshed after setting update, cost ${DateTime.now().difference(refreshStart).inMilliseconds} ms');

    // switch to the tab that was active before the update
    WidgetsBinding.instance.addPostFrameCallback((timeStamp) {
      final tabController = activePluginTabController;
      if (tabController != null && tabController.index != activeTabIndex) {
        tabController.index = activeTabIndex;
      }
    });
  }

  Future<void> updatePluginTriggerKeywords(String pluginId, List<String> triggerKeywords) async {}

  bool shouldShowSettingTab() {
    return activePlugin.value.isInstalled && activePlugin.value.settingDefinitions.isNotEmpty;
  }

  void switchToPluginSettingTab() {
    if (shouldShowSettingTab()) {
      // buggy, ref https://github.com/alihaider78222/dynamic_tabbar/issues/6
      // activePluginTabController.animateTo(1, duration: Duration.zero);
    }
  }

  // ---------- Themes ----------

  WoxTheme _cloneTheme(WoxTheme theme) {
    return WoxTheme.fromJson(Map<String, dynamic>.from(theme.toJson()));
  }

  // Return the in-memory theme editor session so settings can be closed and reopened without dropping unsaved color changes.
  WoxThemeEditorDraftSession getOrCreateThemeEditorDraftSession({required WoxTheme requestedTheme, required WoxTheme currentTheme}) {
    final cachedRestoreTheme = _themeEditorRestoreTheme;
    final cachedSourceTheme = _themeEditorSourceTheme;
    final cachedDraftTheme = _themeEditorDraftTheme;
    final requestedThemeId = requestedTheme.themeId;
    if (cachedRestoreTheme != null &&
        cachedSourceTheme != null &&
        cachedDraftTheme != null &&
        (requestedThemeId.isEmpty || requestedThemeId == cachedSourceTheme.themeId || requestedThemeId == cachedDraftTheme.themeId)) {
      return WoxThemeEditorDraftSession(restoreTheme: _cloneTheme(cachedRestoreTheme), sourceTheme: _cloneTheme(cachedSourceTheme), draftTheme: _cloneTheme(cachedDraftTheme));
    }

    final restoreTheme = _cloneTheme(currentTheme);
    final sourceTheme = _cloneTheme(requestedTheme.themeId.isEmpty ? currentTheme : requestedTheme);
    final draftTheme = _cloneTheme(sourceTheme);
    _themeEditorRestoreTheme = _cloneTheme(restoreTheme);
    _themeEditorSourceTheme = _cloneTheme(sourceTheme);
    _themeEditorDraftTheme = _cloneTheme(draftTheme);
    return WoxThemeEditorDraftSession(restoreTheme: restoreTheme, sourceTheme: sourceTheme, draftTheme: draftTheme);
  }

  // Keep the latest draft in the controller because the editor widget can be disposed when settings closes.
  void updateThemeEditorDraft(WoxTheme draftTheme) {
    if (_themeEditorDraftTheme == null) {
      return;
    }
    _themeEditorDraftTheme = _cloneTheme(draftTheme);
  }

  // Clear the draft session only when the user explicitly discards the edited theme.
  WoxTheme discardThemeEditorDraft() {
    final restoreTheme = _cloneTheme(_themeEditorRestoreTheme ?? WoxTheme.empty());
    _themeEditorRestoreTheme = null;
    _themeEditorSourceTheme = null;
    _themeEditorDraftTheme = null;
    return restoreTheme;
  }

  // Reset the draft baseline after save so discard is disabled until the next edit.
  void commitThemeEditorDraft(WoxTheme savedTheme) {
    final savedThemeClone = _cloneTheme(savedTheme);
    _themeEditorRestoreTheme = _cloneTheme(savedThemeClone);
    _themeEditorSourceTheme = _cloneTheme(savedThemeClone);
    _themeEditorDraftTheme = _cloneTheme(savedThemeClone);
  }

  // Preload the theme store once per app session so settings tab switches can reuse the in-memory list.
  Future<void> preloadThemeStore(String traceId) async {
    await _ensureStoreThemesLoaded(traceId);
  }

  // Share an in-flight store request so preload and tab selection do not duplicate the same local API call.
  Future<void> _ensureStoreThemesLoaded(String traceId, {bool forceRefresh = false}) {
    if (!forceRefresh && _hasLoadedStoreThemes) {
      return Future.value();
    }

    final existingLoad = _storeThemesLoadFuture;
    if (!forceRefresh && existingLoad != null) {
      return existingLoad;
    }

    late final Future<void> loadFuture;
    loadFuture = _loadStoreThemesIntoCache(traceId).whenComplete(() {
      if (identical(_storeThemesLoadFuture, loadFuture)) {
        _storeThemesLoadFuture = null;
      }
    });
    _storeThemesLoadFuture = loadFuture;
    return loadFuture;
  }

  // Load store data into the controller cache without changing whichever theme list is currently visible.
  Future<void> _loadStoreThemesIntoCache(String traceId) async {
    try {
      final start = DateTime.now();
      final storeThemes = await WoxApi.instance.findStoreThemes(traceId);
      storeThemes.sort((a, b) => a.themeName.compareTo(b.themeName));
      if (storeThemes.isEmpty) {
        // Core loads the remote manifest in the background during startup. An early settings entry can see an empty store before that finishes.
        _hasLoadedStoreThemes = false;
        Logger.instance.warn(traceId, 'Theme store cache skipped because API returned no themes');
        return;
      }

      storeThemesList.clear();
      storeThemesList.addAll(storeThemes);
      _hasLoadedStoreThemes = true;
      await _loadInstalledThemesForLookup();
      Logger.instance.info(traceId, 'Store themes cached, cost ${DateTime.now().difference(start).inMilliseconds} ms');
    } catch (e) {
      _hasLoadedStoreThemes = false;
      Logger.instance.error(traceId, 'Failed to cache store themes: $e');
    }
  }

  Future<void> loadStoreThemes({bool forceRefresh = false}) async {
    final traceId = const UuidV4().generate();
    await _ensureStoreThemesLoaded(traceId, forceRefresh: forceRefresh);
    if (!forceRefresh && !_hasLoadedStoreThemes && storeThemesList.isEmpty) {
      await _ensureStoreThemesLoaded(traceId, forceRefresh: true);
    }
    _replaceVisibleThemes(storeThemesList);
  }

  void _replaceVisibleThemes(Iterable<WoxTheme> themes) {
    themeList.clear();
    themeList.addAll(themes);
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList);
  }

  Future<void> loadInstalledThemes() async {
    final traceId = const UuidV4().generate();
    final installThemes = await WoxApi.instance.findInstalledThemes(traceId);
    installThemes.sort((a, b) => a.themeName.compareTo(b.themeName));
    installedThemesList.clear();
    installedThemesList.addAll(installThemes);
    _replaceVisibleThemes(installThemes);
  }

  Future<void> _loadInstalledThemesForLookup() async {
    final traceId = const UuidV4().generate();
    final installThemes = await WoxApi.instance.findInstalledThemes(traceId);
    installedThemesList.clear();
    installedThemesList.addAll(installThemes);
  }

  Future<void> installTheme(WoxTheme theme) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'Installing theme: ${theme.themeId}');
    await WoxApi.instance.installTheme(traceId, theme.themeId);
    _updateCachedStoreThemeInstallState(theme.themeId, true);
    await _loadInstalledThemesForLookup();
    await refreshThemeList();
  }

  Future<void> uninstallTheme(WoxTheme theme) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'Uninstalling theme: ${theme.themeId}');
    await WoxApi.instance.uninstallTheme(traceId, theme.themeId);
    _updateCachedStoreThemeInstallState(theme.themeId, false);
    await _loadInstalledThemesForLookup();
    await refreshThemeList();
  }

  Future<void> applyTheme(WoxTheme theme) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'Applying theme: ${theme.themeId}');
    await WoxApi.instance.applyTheme(traceId, theme.themeId);
    await refreshThemeList();
    await reloadSetting(traceId);
  }

  void _updateCachedStoreThemeInstallState(String themeId, bool isInstalled) {
    for (final theme in storeThemesList) {
      if (theme.themeId == themeId) {
        theme.isInstalled = isInstalled;
      }
    }
  }

  // Save edited theme drafts through core so the new theme is installed and applied consistently.
  Future<WoxTheme> saveThemeAs(WoxTheme theme, String name, {bool switchToInstalledThemes = false, bool overwrite = false}) async {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, 'Saving theme: $name, overwrite=$overwrite');
    final savedTheme = await WoxApi.instance.saveTheme(traceId, name, theme, overwrite: overwrite);
    if (switchToInstalledThemes) {
      activeNavPath.value = 'themes.installed';
      isStoreThemeList.value = false;
    }
    if (!isStoreThemeList.value) {
      await loadInstalledThemes();
    } else {
      await _loadInstalledThemesForLookup();
    }
    await reloadSetting(traceId);
    activeTheme.value = savedTheme;
    return savedTheme;
  }

  void onFilterThemes(String filter) {
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList.where((element) => element.themeName.toLowerCase().contains(filter.toLowerCase())));
  }

  void ensureThemeVisible(String themeId) {
    final index = filteredThemeList.indexWhere((theme) => theme.themeId == themeId);
    if (index < 0) {
      return;
    }

    const rowExtent = 72.0;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!themeListScrollController.hasClients) {
        return;
      }
      final maxExtent = themeListScrollController.position.maxScrollExtent;
      final targetOffset = (index * rowExtent).clamp(0.0, maxExtent).toDouble();
      themeListScrollController.animateTo(targetOffset, duration: const Duration(milliseconds: 180), curve: Curves.easeOutCubic);
    });
  }

  Future<void> locateCurrentTheme() async {
    if (isStoreThemeList.value) {
      await switchToThemeList(false);
    } else if (themeList.isEmpty) {
      await refreshThemeList();
    }

    final currentThemeId = woxSetting.value.themeId;
    if (currentThemeId.isEmpty) {
      return;
    }

    filterThemeKeywordController.clear();
    onFilterThemes('');
    final matchedTheme = filteredThemeList.firstWhere((theme) => theme.themeId == currentThemeId, orElse: () => WoxTheme.empty());
    if (matchedTheme.themeId.isEmpty) {
      return;
    }
    activeTheme.value = matchedTheme;
    ensureThemeVisible(currentThemeId);
  }

  void setFirstFilteredThemeActive() {
    if (filteredThemeList.isNotEmpty) {
      activeTheme.value = filteredThemeList[0];
    }
  }

  Future<void> refreshThemeList() async {
    if (isStoreThemeList.value) {
      await loadStoreThemes();
    } else {
      await loadInstalledThemes();
    }

    //active theme
    if (filteredThemeList.isEmpty) {
      activeTheme.value = WoxTheme.empty();
      return;
    }

    if (activeTheme.value.themeId.isNotEmpty) {
      activeTheme.value = filteredThemeList.firstWhere((element) => element.themeId == activeTheme.value.themeId, orElse: () => filteredThemeList[0]);
    } else {
      setFirstFilteredThemeActive();
    }
  }

  Future<void> switchToThemeList(bool isStoreTheme) async {
    activeNavPath.value = isStoreTheme ? 'themes.store' : 'themes.installed';
    isStoreThemeList.value = isStoreTheme;
    activeTheme.value = WoxTheme.empty();
    await refreshThemeList();
    setFirstFilteredThemeActive();
  }

  Future<void> loadUserDataLocation() async {
    final traceId = const UuidV4().generate();
    try {
      userDataLocation.value = await WoxApi.instance.getUserDataLocation(traceId);
    } catch (e) {
      userDataLocation.value = '';
      Logger.instance.error(traceId, 'Failed to load user data location: $e');
    }
  }

  Future<void> updateUserDataLocation(String newLocation) async {
    final traceId = const UuidV4().generate();
    await WoxApi.instance.updateUserDataLocation(traceId, newLocation);
    userDataLocation.value = newLocation;
  }

  Future<void> backupNow() async {
    if (isBackingUp.value) {
      return;
    }

    final traceId = const UuidV4().generate();
    isBackingUp.value = true;
    try {
      await WoxApi.instance.backupNow(traceId);
      await refreshBackups();
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to create manual backup: $e');
    } finally {
      isBackingUp.value = false;
    }
  }

  Future<void> refreshBackups() async {
    final traceId = const UuidV4().generate();
    try {
      final result = await WoxApi.instance.getAllBackups(traceId);
      backups.assignAll(result);
    } catch (e) {
      backups.clear();
      Logger.instance.error(traceId, 'Failed to load backups: $e');
    }
  }

  Future<void> refreshCloudSyncStatus({bool showLoading = true}) async {
    final traceId = const UuidV4().generate();
    if (showLoading) {
      isCloudSyncStatusLoading.value = true;
    }
    cloudSyncStatusError.value = '';
    try {
      final status = await WoxApi.instance.getCloudSyncStatus(traceId);
      cloudSyncStatus.value = status;
      Logger.instance.info(traceId, 'Cloud sync status loaded');
    } catch (e) {
      cloudSyncStatus.value = WoxCloudSyncStatus.empty();
      cloudSyncStatusError.value = e.toString();
      Logger.instance.error(traceId, 'Failed to load cloud sync status: $e');
    } finally {
      if (showLoading) {
        isCloudSyncStatusLoading.value = false;
      }
    }
  }

  void applyCloudSyncProgress(Map<String, dynamic> data) {
    final progress = WoxCloudSyncProgress.fromJson(data);
    cloudSyncStatus.value = cloudSyncStatus.value.withProgress(progress.active ? progress : null);
  }

  Future<void> refreshAccountStatus() async {
    final traceId = const UuidV4().generate();
    try {
      accountStatus.value = await WoxApi.instance.getAccountStatus(traceId);
      Logger.instance.info(traceId, 'Account status loaded');
    } catch (e) {
      accountStatus.value = WoxAccountStatus.empty();
      Logger.instance.error(traceId, 'Failed to load account status: $e');
    }
  }

  Future<void> refreshCloudSyncBillingPlan() async {
    final traceId = const UuidV4().generate();
    cloudSyncBillingPlanError.value = '';
    try {
      cloudSyncBillingPlan.value = await WoxApi.instance.accountBillingPlan(traceId);
      cloudSyncBillingPlanLoaded.value = true;
      Logger.instance.info(traceId, 'Cloud sync billing plan loaded');
    } catch (e) {
      cloudSyncBillingPlanLoaded.value = true;
      cloudSyncBillingPlanError.value = e.toString();
      Logger.instance.error(traceId, 'Failed to load cloud sync billing plan: $e');
    }
  }

  Future<void> refreshCloudSyncDevices({bool showLoading = true}) async {
    if (!accountStatus.value.loggedIn) {
      cloudSyncDeviceList.value = WoxCloudSyncDeviceList.empty();
      return;
    }
    final traceId = const UuidV4().generate();
    if (showLoading) {
      isCloudSyncActionLoading.value = true;
    }
    cloudSyncActionError.value = '';
    try {
      cloudSyncDeviceList.value = await WoxApi.instance.cloudSyncDevicesList(traceId);
      Logger.instance.info(traceId, 'Cloud sync devices loaded');
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Failed to load cloud sync devices: $e');
    } finally {
      if (showLoading) {
        isCloudSyncActionLoading.value = false;
      }
    }
  }

  Future<void> cloudSyncRevokeDevice(String targetDeviceId) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncDeviceRevoke(traceId, targetDeviceId);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus(showLoading: false), refreshCloudSyncDevices(showLoading: false)]);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync device revoke failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> cloudSyncJoinDevice() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncDeviceJoin(traceId);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus(showLoading: false), refreshCloudSyncDevices(showLoading: false)]);
      _updateCloudSyncStatusWaiting();
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync device join failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  // Maps Wox locale identifiers to the sync account API language set.
  String accountRequestLang() {
    final langCode = woxSetting.value.langCode.toLowerCase().replaceAll('_', '-');
    if (langCode.startsWith('zh')) {
      return 'zh';
    }
    return 'en';
  }

  Future<WoxAccountActionResult> accountRegister(String email, String password) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      final result = await WoxApi.instance.accountRegister(traceId, email, password, accountRequestLang());
      await refreshAccountStatus();
      return result;
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account register failed: $e');
      return WoxAccountActionResult.empty();
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<WoxAccountActionResult> accountVerifyEmail(String email, String code) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      final result = await WoxApi.instance.accountVerifyEmail(traceId, email, code, accountRequestLang());
      if (result.isOk) {
        await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
        await refreshCloudSyncDevices(showLoading: false);
      }
      return result;
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account verify email failed: $e');
      return WoxAccountActionResult.empty();
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<WoxAccountActionResult> accountLogin(String email, String password) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      final result = await WoxApi.instance.accountLogin(traceId, email, password, accountRequestLang());
      if (result.isOk) {
        await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
        await refreshCloudSyncDevices(showLoading: false);
      } else {
        await refreshAccountStatus();
      }
      return result;
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account login failed: $e');
      return WoxAccountActionResult.empty();
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> accountLogout() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      _stopAccountBillingWaiting();
      await WoxApi.instance.accountLogout(traceId);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
      cloudSyncDeviceList.value = WoxCloudSyncDeviceList.empty();
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account logout failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> accountResendVerification(String email) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.accountResendVerification(traceId, email, accountRequestLang());
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account resend verification failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<bool> accountPasswordResetRequest(String email) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.accountPasswordResetRequest(traceId, email, accountRequestLang());
      return true;
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account password reset request failed: $e');
      return false;
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> accountPasswordResetConfirm(String token, String password) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.accountPasswordResetConfirm(traceId, token, password, accountRequestLang());
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account password reset confirm failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<bool> accountChangePassword(String currentPassword, String newPassword) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.accountChangePassword(traceId, currentPassword, newPassword, accountRequestLang());
      return true;
    } catch (e) {
      accountActionError.value = e.toString();
      Logger.instance.error(traceId, 'Account change password failed: $e');
      return false;
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> accountOpenCheckout() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      _stopAccountBillingWaiting();
      final session = await WoxApi.instance.accountBillingCheckout(traceId);
      await _openExternalBillingUrl(traceId, session.url);
      _startAccountBillingWaiting(
        messageKey: "ui_cloud_sync_subscription_waiting_payment",
        timeoutMessageKey: "ui_cloud_sync_subscription_payment_timeout",
        complete: (status) => status.isPro,
      );
    } catch (e) {
      _stopAccountBillingWaiting();
      accountSubscriptionError.value = e.toString();
      Logger.instance.error(traceId, 'Open billing checkout failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> accountOpenBillingPortal() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      _stopAccountBillingWaiting();
      final initialStatusFingerprint = _accountBillingStatusFingerprint(accountStatus.value);
      final session = await WoxApi.instance.accountBillingPortal(traceId);
      await _openExternalBillingUrl(traceId, session.url);
      _startAccountBillingWaiting(
        messageKey: "ui_cloud_sync_subscription_waiting_update",
        timeoutMessageKey: "ui_cloud_sync_subscription_update_timeout",
        complete: (status) => _accountBillingStatusFingerprint(status) != initialStatusFingerprint,
      );
    } catch (e) {
      _stopAccountBillingWaiting();
      accountSubscriptionError.value = e.toString();
      Logger.instance.error(traceId, 'Open billing portal failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  // Pulls the latest subscription state from the account server and refreshes dependent sync status.
  Future<void> accountRefreshSubscriptionStatus() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    accountActionError.value = '';
    accountSubscriptionError.value = '';
    cloudSyncActionError.value = '';
    try {
      _stopAccountBillingWaiting();
      accountStatus.value = await WoxApi.instance.accountRefresh(traceId);
      await refreshCloudSyncStatus();
    } catch (e) {
      await refreshAccountStatus();
      _updateCloudSyncStatusWaiting();
      accountSubscriptionError.value = e.toString();
      Logger.instance.error(traceId, 'Refresh account subscription status failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  // Refreshes subscription state when users return from external billing pages that did not trigger a deeplink callback.
  Future<void> _refreshCloudSyncAccountStatusOnResume() async {
    if (activeNavPath.value != 'data.cloudsync' ||
        !accountStatus.value.loggedIn ||
        isAccountBillingWaiting.value ||
        isCloudSyncActionLoading.value ||
        _isRefreshingAccountStatusOnResume) {
      return;
    }

    _isRefreshingAccountStatusOnResume = true;
    final traceId = const UuidV4().generate();
    try {
      accountStatus.value = await WoxApi.instance.accountRefresh(traceId);
      await refreshCloudSyncStatus(showLoading: false);
      _updateCloudSyncStatusWaiting();
      Logger.instance.info(traceId, 'Account subscription status refreshed after app resume');
    } catch (e) {
      await refreshAccountStatus();
      _updateCloudSyncStatusWaiting();
      Logger.instance.error(traceId, 'Failed to refresh account subscription status after app resume: $e');
    } finally {
      _isRefreshingAccountStatusOnResume = false;
    }
  }

  Future<void> _openExternalBillingUrl(String traceId, String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null) {
      throw Exception('invalid billing url');
    }
    final opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!opened) {
      throw Exception('failed to open billing url');
    }
  }

  // Starts a bounded wait after a Stripe page opens so webhook-driven account changes are pulled into the UI.
  void _startAccountBillingWaiting({required String messageKey, required String timeoutMessageKey, required bool Function(WoxAccountStatus status) complete}) {
    _accountBillingPollTimer?.cancel();
    _accountBillingWaitDeadline = DateTime.now().add(_accountBillingWaitTimeout);
    _accountBillingWaitTimeoutMessageKey = timeoutMessageKey;
    _accountBillingWaitComplete = complete;
    isAccountBillingWaiting.value = true;
    accountBillingWaitingMessageKey.value = messageKey;
    accountSubscriptionError.value = '';
    unawaited(_pollAccountBillingStatus());
    _accountBillingPollTimer = Timer.periodic(_accountBillingPollInterval, (_) => unawaited(_pollAccountBillingStatus()));
  }

  void _stopAccountBillingWaiting() {
    _accountBillingPollTimer?.cancel();
    _accountBillingPollTimer = null;
    _accountBillingWaitDeadline = null;
    _accountBillingWaitTimeoutMessageKey = '';
    _accountBillingWaitComplete = null;
    isAccountBillingWaiting.value = false;
    accountBillingWaitingMessageKey.value = '';
  }

  // Polls account status until the expected billing change is visible or the bounded wait expires.
  Future<void> _pollAccountBillingStatus() async {
    if (!isAccountBillingWaiting.value || _isAccountBillingPolling) {
      return;
    }
    _isAccountBillingPolling = true;
    try {
      final traceId = const UuidV4().generate();
      try {
        accountStatus.value = await WoxApi.instance.accountRefresh(traceId);
        Logger.instance.info(traceId, 'Account billing status refreshed');
      } catch (e) {
        Logger.instance.error(traceId, 'Failed to refresh account billing status: $e');
      }

      final complete = _accountBillingWaitComplete;
      if (complete != null && complete(accountStatus.value)) {
        _stopAccountBillingWaiting();
        accountSubscriptionError.value = '';
        await refreshCloudSyncStatus();
        return;
      }

      final deadline = _accountBillingWaitDeadline;
      if (deadline != null && DateTime.now().isAfter(deadline)) {
        final timeoutMessageKey = _accountBillingWaitTimeoutMessageKey;
        _stopAccountBillingWaiting();
        accountSubscriptionError.value = timeoutMessageKey.isEmpty ? '' : tr(timeoutMessageKey);
      }
    } finally {
      _isAccountBillingPolling = false;
    }
  }

  String _accountBillingStatusFingerprint(WoxAccountStatus status) {
    return '${status.plan}|${status.syncEligible}|${status.syncEnabled}|${status.sessionExpired}|${status.deviceCount}';
  }

  Future<WoxCloudSyncBootstrapStatus?> cloudSyncBootstrapStatus() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      return await WoxApi.instance.cloudSyncBootstrapStatus(traceId);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync bootstrap status failed: $e');
      return null;
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<bool> cloudSyncBootstrapStart(String recoveryCode) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncBootstrapStart(traceId, recoveryCode);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
      _updateCloudSyncStatusWaiting();
      return true;
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync bootstrap start failed: $e');
      return false;
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  // Starts a bounded status poll while first-time bootstrap work continues in the backend.
  void _updateCloudSyncStatusWaiting() {
    if (!_shouldPollCloudSyncStatus(accountStatus.value, cloudSyncStatus.value)) {
      _stopCloudSyncStatusWaiting();
      return;
    }
    if (_cloudSyncStatusPollTimer != null) {
      return;
    }

    _cloudSyncStatusWaitDeadline = DateTime.now().add(_cloudSyncStatusWaitTimeout);
    unawaited(_pollCloudSyncStatus());
    _cloudSyncStatusPollTimer = Timer.periodic(_cloudSyncStatusPollInterval, (_) => unawaited(_pollCloudSyncStatus()));
  }

  // Keeps polling scoped to the bootstrap-pending state, which completes in the backend without a UI push event.
  bool _shouldPollCloudSyncStatus(WoxAccountStatus account, WoxCloudSyncStatus status) {
    final state = status.state;
    return activeNavPath.value == 'data.cloudsync' &&
        account.loggedIn &&
        !account.sessionExpired &&
        account.syncEligible &&
        account.syncEnabled &&
        status.keyStatus.available &&
        state != null &&
        !state.bootstrapped &&
        state.lastError.isEmpty;
  }

  void _stopCloudSyncStatusWaiting() {
    _cloudSyncStatusPollTimer?.cancel();
    _cloudSyncStatusPollTimer = null;
    _cloudSyncStatusWaitDeadline = null;
  }

  Future<void> _pollCloudSyncStatus() async {
    if (_isCloudSyncStatusPolling) {
      return;
    }
    _isCloudSyncStatusPolling = true;
    try {
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus(showLoading: false)]);
      if (!_shouldPollCloudSyncStatus(accountStatus.value, cloudSyncStatus.value)) {
        _stopCloudSyncStatusWaiting();
        return;
      }

      final deadline = _cloudSyncStatusWaitDeadline;
      if (deadline != null && DateTime.now().isAfter(deadline)) {
        _stopCloudSyncStatusWaiting();
      }
    } finally {
      _isCloudSyncStatusPolling = false;
    }
  }

  Future<void> cloudSyncEnable() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncEnable(traceId);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync enable failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> cloudSyncDisable() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncDisable(traceId);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync disable failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> cloudSyncPush() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncPush(traceId);
      await refreshCloudSyncStatus(showLoading: false);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync push failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
      await refreshCloudSyncStatus(showLoading: false);
    }
  }

  Future<void> cloudSyncPull() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncPull(traceId);
      await refreshCloudSyncStatus(showLoading: false);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync pull failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
      await refreshCloudSyncStatus(showLoading: false);
    }
  }

  Future<void> cloudSyncSyncNow() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncPush(traceId);
      await WoxApi.instance.cloudSyncPull(traceId);
      await refreshCloudSyncStatus(showLoading: false);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync now failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
      await refreshCloudSyncStatus(showLoading: false);
    }
  }

  Future<String?> cloudSyncGenerateRecoveryCode() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      return await WoxApi.instance.cloudSyncRecoveryCode(traceId);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync recovery code failed: $e');
      return null;
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> cloudSyncInitKey(String recoveryCode, String deviceName) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncKeyInit(traceId, recoveryCode, deviceName);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
      await refreshCloudSyncDevices(showLoading: false);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync key init failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> cloudSyncFetchKey(String recoveryCode) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncKeyFetch(traceId, recoveryCode);
      await Future.wait([refreshAccountStatus(), refreshCloudSyncStatus()]);
      await refreshCloudSyncDevices(showLoading: false);
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync key fetch failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<String?> cloudSyncPrepareReset() async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      final resp = await WoxApi.instance.cloudSyncKeyResetPrepare(traceId);
      final token = resp['reset_token'];
      if (token is String) {
        return token;
      }
      return null;
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync reset prepare failed: $e');
      return null;
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> cloudSyncReset(String resetToken) async {
    final traceId = const UuidV4().generate();
    isCloudSyncActionLoading.value = true;
    cloudSyncActionError.value = '';
    try {
      await WoxApi.instance.cloudSyncKeyReset(traceId, resetToken);
      await refreshCloudSyncStatus();
    } catch (e) {
      cloudSyncActionError.value = e.toString();
      Logger.instance.error(traceId, 'Cloud sync reset failed: $e');
    } finally {
      isCloudSyncActionLoading.value = false;
    }
  }

  Future<void> updateCloudSyncDisabledPlugins(List<String> pluginIds) async {
    await updateConfig("CloudSyncDisabledPlugins", jsonEncode(pluginIds));
  }

  Future<void> updateCloudSyncServerUrl(String url) async {
    final currentUrl = woxSetting.value.cloudSyncServerUrl.trim();
    final nextUrl = url.trim();
    if (currentUrl == nextUrl) {
      return;
    }
    await updateConfig("CloudSyncServerUrl", nextUrl);
    // A sync endpoint change invalidates account-scoped local state. Reuse the
    // normal logout path so the Cloud Sync page immediately reflects that the
    // current account session is no longer active for this endpoint.
    await accountLogout();
  }

  Future<void> clearLogs() async {
    if (isClearingLogs.value) {
      return;
    }

    final traceId = const UuidV4().generate();
    isClearingLogs.value = true;
    try {
      await WoxApi.instance.clearLogs(traceId);
      Logger.instance.info(traceId, 'Logs cleared');
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to clear logs: $e');
    } finally {
      isClearingLogs.value = false;
    }
  }

  Future<void> updateLogLevel(String level) async {
    if (isUpdatingLogLevel.value) {
      return;
    }

    final previous = woxSetting.value.logLevel;
    woxSetting.value.logLevel = level;
    woxSetting.refresh();
    Logger.instance.setLogLevel(level);

    final traceId = const UuidV4().generate();
    isUpdatingLogLevel.value = true;
    try {
      await WoxApi.instance.updateSetting(traceId, "LogLevel", level);
      await reloadSetting(traceId);
      Logger.instance.info(traceId, 'LogLevel updated: $level');
    } catch (e) {
      woxSetting.value.logLevel = previous;
      woxSetting.refresh();
      Logger.instance.setLogLevel(previous);
      Logger.instance.error(traceId, 'Failed to update LogLevel: $e');
    } finally {
      isUpdatingLogLevel.value = false;
    }
  }

  Future<void> openLogFile() async {
    final traceId = const UuidV4().generate();
    await WoxApi.instance.openLogFile(traceId);
  }

  Future<void> openFolder(String path) async {
    await WoxApi.instance.open(const UuidV4().generate(), path);
  }

  Future<void> restoreBackup(String id) async {
    final traceId = const UuidV4().generate();
    await WoxApi.instance.restoreBackup(traceId, id);
    await reloadSetting(traceId);
  }

  Future<void> reloadSetting(String traceId) async {
    final previousLangCode = woxSetting.value.langCode;
    await WoxSettingUtil.instance.loadSetting(traceId);
    woxSetting.value = WoxSettingUtil.instance.currentSetting;
    // Cloud sync (e.g. bootstrap restore) and other remote sources can change
    // LangCode without going through updateLang, so reload the language json
    // and plugin translations to keep the UI in sync with the persisted value.
    if (woxSetting.value.langCode != previousLangCode) {
      await _applyLangResourcesChange(woxSetting.value.langCode);
    }
    WoxInterfaceSizeUtil.instance.refreshFromDensity(woxSetting.value.uiDensity);
    Logger.instance.setLogLevel(woxSetting.value.logLevel);
    if (Get.isRegistered<WoxLauncherController>()) {
      final launcherController = Get.find<WoxLauncherController>();
      if (!woxSetting.value.enableQueryCompletionHint) {
        launcherController.clearQueryCompletionHint();
      }
      unawaited(launcherController.refreshGlance(traceId, "settingsChanged"));
      // Interface size changes are launcher-only; the settings view keeps its
      // fixed sizing while the launcher recalculates dimensions from metrics as
      // soon as settings reload.
      unawaited(launcherController.resizeHeight(traceId: traceId, reason: "settings density changed"));
    }
    Logger.instance.info(traceId, 'Setting reloaded');
  }

  @override
  void onClose() {
    WidgetsBinding.instance.removeObserver(this);
    _settingHighlightTimer?.cancel();
    _accountBillingPollTimer?.cancel();
    _cloudSyncStatusPollTimer?.cancel();
    pluginListScrollController.dispose();
    themeListScrollController.dispose();
    filterThemeKeywordController.dispose();
    settingFocusNode.dispose();
    settingSearchTextController.dispose();
    settingSearchFocusNode.dispose();
    settingSearchResultScrollController.dispose();
    super.onClose();
  }
}

class WoxAIModelSelectorResources {
  final List<AIModel> models;
  final List<AIProviderInfo> providers;

  const WoxAIModelSelectorResources({required this.models, required this.providers});
}

class _GeneralSectionFocusRequest {
  final String sectionId;
  final int? trayQueryEditRowIndex;

  const _GeneralSectionFocusRequest({required this.sectionId, this.trayQueryEditRowIndex});
}

class _BuiltInSettingSearchDefinition {
  final String settingKey;
  final String navPath;
  final String titleKey;
  final String subtitleKey;
  final List<String> searchKeywords;

  const _BuiltInSettingSearchDefinition({required this.settingKey, required this.navPath, required this.titleKey, this.subtitleKey = '', this.searchKeywords = const []});
}

class _PluginSettingSearchData {
  final String settingKey;
  final String title;
  final List<String> searchTexts;

  const _PluginSettingSearchData({required this.settingKey, required this.title, required this.searchTexts});
}

const List<_BuiltInSettingSearchDefinition> _builtInSettingSearchDefinitions = [
  _BuiltInSettingSearchDefinition(
    settingKey: 'EnableAutostart',
    navPath: 'general',
    titleKey: 'ui_autostart',
    subtitleKey: 'ui_autostart_tips',
    searchKeywords: ['startup', 'auto start'],
  ),
  _BuiltInSettingSearchDefinition(settingKey: 'HideOnStart', navPath: 'general', titleKey: 'ui_hide_on_start', subtitleKey: 'ui_hide_on_start_tips'),
  _BuiltInSettingSearchDefinition(
    settingKey: 'EnableAutoUpdate',
    navPath: 'update',
    titleKey: 'ui_enable_auto_update',
    subtitleKey: 'ui_enable_auto_update_tips',
    searchKeywords: ['update'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'ReleaseChannel',
    navPath: 'update',
    titleKey: 'ui_release_channel',
    subtitleKey: 'ui_release_channel_tips',
    searchKeywords: ['update channel', 'release channel', 'stable', 'beta', 'stable channel', 'beta channel', 'prerelease'],
  ),
  _BuiltInSettingSearchDefinition(settingKey: 'MainHotkey', navPath: 'general', titleKey: 'ui_hotkey', subtitleKey: 'ui_hotkey_tips', searchKeywords: ['shortcut', 'main hotkey']),
  _BuiltInSettingSearchDefinition(
    settingKey: 'SelectionHotkey',
    navPath: 'general',
    titleKey: 'ui_selection_hotkey',
    subtitleKey: 'ui_selection_hotkey_tips',
    searchKeywords: ['selection shortcut'],
  ),
  _BuiltInSettingSearchDefinition(settingKey: 'LaunchMode', navPath: 'general', titleKey: 'ui_launch_mode', subtitleKey: 'ui_launch_mode_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'StartPage', navPath: 'general', titleKey: 'ui_start_page', subtitleKey: 'ui_start_page_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'HideOnLostFocus', navPath: 'general', titleKey: 'ui_hide_on_lost_focus', subtitleKey: 'ui_hide_on_lost_focus_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'UsePinYin', navPath: 'general', titleKey: 'ui_use_pinyin', subtitleKey: 'ui_use_pinyin_tips', searchKeywords: ['pinyin']),
  _BuiltInSettingSearchDefinition(settingKey: 'SwitchInputMethodABC', navPath: 'general', titleKey: 'ui_switch_input_method_abc', subtitleKey: 'ui_switch_input_method_abc_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'LangCode', navPath: 'general', titleKey: 'ui_lang', searchKeywords: ['language']),
  _BuiltInSettingSearchDefinition(settingKey: 'IgnoredHotkeyApps', navPath: 'general', titleKey: 'ui_hotkey_ignore_apps', subtitleKey: 'ui_hotkey_ignore_apps_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'QueryHotkeys', navPath: 'general', titleKey: 'ui_query_hotkeys', subtitleKey: 'ui_query_hotkeys_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'QueryShortcuts', navPath: 'general', titleKey: 'ui_query_shortcuts', subtitleKey: 'ui_query_shortcuts_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'TrayQueries', navPath: 'general', titleKey: 'ui_tray_queries', subtitleKey: 'ui_tray_queries_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'ShowPosition', navPath: 'ui', titleKey: 'ui_show_position', subtitleKey: 'ui_show_position_tips', searchKeywords: ['position']),
  _BuiltInSettingSearchDefinition(settingKey: 'ShowTray', navPath: 'ui', titleKey: 'ui_show_tray', subtitleKey: 'ui_show_tray_tips', searchKeywords: ['tray']),
  _BuiltInSettingSearchDefinition(settingKey: 'AppWidth', navPath: 'ui', titleKey: 'ui_app_width', subtitleKey: 'ui_app_width_tips', searchKeywords: ['width']),
  _BuiltInSettingSearchDefinition(
    settingKey: 'UiDensity',
    navPath: 'ui',
    titleKey: 'ui_interface_size',
    subtitleKey: 'ui_interface_size_tips',
    searchKeywords: ['Interface Size', 'density', 'compact', 'comfortable'],
  ),
  _BuiltInSettingSearchDefinition(settingKey: 'AppFontFamily', navPath: 'ui', titleKey: 'ui_app_font_family', subtitleKey: 'ui_app_font_family_tips', searchKeywords: ['font']),
  _BuiltInSettingSearchDefinition(
    settingKey: 'EnableQueryCompletionHint',
    navPath: 'ui',
    titleKey: 'ui_query_completion_hint',
    subtitleKey: 'ui_query_completion_hint_tips',
    searchKeywords: ['completion', 'hint', 'autocomplete', 'inline completion'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'MaxResultCount',
    navPath: 'ui',
    titleKey: 'ui_max_result_count',
    subtitleKey: 'ui_max_result_count_tips',
    searchKeywords: ['result count'],
  ),
  _BuiltInSettingSearchDefinition(settingKey: 'EnableGlance', navPath: 'ui', titleKey: 'ui_glance_enable', subtitleKey: 'ui_glance_enable_tips', searchKeywords: ['glance']),
  _BuiltInSettingSearchDefinition(settingKey: 'HideGlanceIcon', navPath: 'ui', titleKey: 'ui_glance_hide_icon', subtitleKey: 'ui_glance_hide_icon_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'PrimaryGlance', navPath: 'ui', titleKey: 'ui_glance_primary', subtitleKey: 'ui_glance_primary_tips'),
  _BuiltInSettingSearchDefinition(settingKey: 'AIProviders', navPath: 'ai', titleKey: 'ui_ai_model', searchKeywords: ['ai provider', 'api key', 'model']),
  _BuiltInSettingSearchDefinition(settingKey: 'AIMCPServers', navPath: 'ai', titleKey: 'ui_ai_mcp_servers', searchKeywords: ['mcp', 'tool', 'server']),
  _BuiltInSettingSearchDefinition(settingKey: 'AISkills', navPath: 'ai', titleKey: 'ui_ai_skills', searchKeywords: ['skill', 'repo', 'path']),
  _BuiltInSettingSearchDefinition(settingKey: 'HttpProxyEnabled', navPath: 'network', titleKey: 'ui_proxy_enabled', searchKeywords: ['proxy']),
  _BuiltInSettingSearchDefinition(settingKey: 'HttpProxyUrl', navPath: 'network', titleKey: 'ui_proxy_url', searchKeywords: ['proxy url']),
  _BuiltInSettingSearchDefinition(
    settingKey: 'EnableAutoBackup',
    navPath: 'data',
    titleKey: 'ui_data_backup_auto_title',
    subtitleKey: 'ui_data_backup_auto_tips',
    searchKeywords: ['backup'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'UserDataLocation',
    navPath: 'data',
    titleKey: 'ui_data_config_location',
    subtitleKey: 'ui_data_config_location_tips',
    searchKeywords: ['data location'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CustomPythonPath',
    navPath: 'plugins.runtime',
    titleKey: 'ui_runtime_python_path',
    subtitleKey: 'ui_runtime_python_path_tips',
    searchKeywords: ['python'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CustomNodejsPath',
    navPath: 'plugins.runtime',
    titleKey: 'ui_runtime_nodejs_path',
    subtitleKey: 'ui_runtime_nodejs_path_tips',
    searchKeywords: ['nodejs', 'node'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CloudSyncServerUrl',
    navPath: 'debug',
    titleKey: 'ui_cloud_sync_server_url',
    subtitleKey: 'ui_cloud_sync_server_url_tips',
    searchKeywords: ['cloud sync', 'sync server', 'server url'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CloudSyncAccount',
    navPath: 'data.cloudsync',
    titleKey: 'ui_cloud_sync_account',
    searchKeywords: ['cloud sync', 'account', 'login', 'register', 'email', 'password'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CloudSyncSubscriptionStatus',
    navPath: 'data.cloudsync',
    titleKey: 'ui_cloud_sync_plan_status',
    searchKeywords: ['cloud sync', 'subscription', 'billing', 'subscribe', 'plan', 'free', 'pro'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CloudSyncBillingHelp',
    navPath: 'data.cloudsync',
    titleKey: 'ui_cloud_sync_billing_help',
    subtitleKey: 'ui_cloud_sync_billing_help_tips',
    searchKeywords: ['cloud sync', 'billing', 'support', 'contact support', 'customer service'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CloudSyncStatus',
    navPath: 'data.cloudsync',
    titleKey: 'ui_cloud_sync_sync_status',
    searchKeywords: ['cloud sync', 'sync status', 'sync now'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CloudSyncDisabledPlugins',
    navPath: 'data.cloudsync',
    titleKey: 'ui_cloud_sync_plugin_exclusions',
    subtitleKey: 'ui_cloud_sync_plugin_exclusions_tips',
    searchKeywords: ['cloud sync', 'plugin sync', 'plugin exclusions', 'exclude plugin'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'CloudSyncConfigNotes',
    navPath: 'data.cloudsync',
    titleKey: 'ui_cloud_sync_config_notes',
    subtitleKey: 'ui_cloud_sync_config_notes_tips',
    searchKeywords: ['cloud sync', 'sync notes', 'platform sync', 'partial sync'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'ShowScoreTail',
    navPath: 'debug',
    titleKey: 'ui_debug_show_score_tail',
    subtitleKey: 'ui_debug_show_score_tail_tips',
    searchKeywords: ['score'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'ShowPerformanceTail',
    navPath: 'debug',
    titleKey: 'ui_debug_show_performance_tail',
    subtitleKey: 'ui_debug_show_performance_tail_tips',
    searchKeywords: ['performance'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'ShowPerformanceTailBatch',
    navPath: 'debug',
    titleKey: 'ui_debug_show_performance_tail_batch',
    subtitleKey: 'ui_debug_show_performance_tail_batch_tips',
    searchKeywords: ['performance', 'batch'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'ShowPerformanceTailPluginQuery',
    navPath: 'debug',
    titleKey: 'ui_debug_show_performance_tail_plugin_query',
    subtitleKey: 'ui_debug_show_performance_tail_plugin_query_tips',
    searchKeywords: ['performance', 'plugin', 'query'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'ShowPerformanceTailBackendPrepared',
    navPath: 'debug',
    titleKey: 'ui_debug_show_performance_tail_backend_prepared',
    subtitleKey: 'ui_debug_show_performance_tail_backend_prepared_tips',
    searchKeywords: ['performance', 'backend', 'send'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'ShowPerformanceTailUiReceived',
    navPath: 'debug',
    titleKey: 'ui_debug_show_performance_tail_ui_received',
    subtitleKey: 'ui_debug_show_performance_tail_ui_received_tips',
    searchKeywords: ['performance', 'ui', 'received'],
  ),
  _BuiltInSettingSearchDefinition(
    settingKey: 'EnableAnonymousUsageStats',
    navPath: 'privacy',
    titleKey: 'ui_privacy_anonymous_stats_title',
    subtitleKey: 'ui_privacy_anonymous_stats_description',
    searchKeywords: ['privacy', 'telemetry'],
  ),
];
