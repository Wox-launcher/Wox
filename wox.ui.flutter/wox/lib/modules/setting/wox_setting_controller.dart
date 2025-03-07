import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxSettingController extends GetxController {
  final activePaneIndex = 0.obs;
  final woxSetting = WoxSettingUtil.instance.currentSetting.obs;
  final userDataLocation = "".obs;

  //plugins
  final pluginList = <PluginDetail>[];
  final filterPluginKeywordController = TextEditingController();
  final filteredPluginList = <PluginDetail>[].obs;
  final activePlugin = PluginDetail.empty().obs;
  final isStorePluginList = true.obs;
  late TabController activePluginTabController;

  //themes
  final themeList = <WoxTheme>[];
  final filteredThemeList = <WoxTheme>[].obs;
  final activeTheme = WoxTheme.empty().obs;
  final isStoreThemeList = true.obs;

  //lang
  var langMap = <String, String>{}.obs;

  final isInstallingPlugin = false.obs;

  // Add loading state and cache
  final isLoadingPlugins = false.obs;
  final _storePluginsCache = <PluginDetail>[];
  final _installedPluginsCache = <PluginDetail>[];
  final _lastStorePluginsFetchTime = DateTime(2000).obs;
  final _lastInstalledPluginsFetchTime = DateTime(2000).obs;
  static const _cacheDuration = Duration(minutes: 5);

  @override
  void onInit() {
    super.onInit();
    refreshThemeList();
    loadUserDataLocation();
  }

  void hideWindow() {
    Get.find<WoxLauncherController>().isInSettingView.value = false;
  }

  Future<void> updateConfig(String key, String value) async {
    await WoxApi.instance.updateSetting(key, value);
    await WoxSettingUtil.instance.loadSetting();
    woxSetting.value = WoxSettingUtil.instance.currentSetting;
    Logger.instance.info(const UuidV4().generate(), 'Setting updated: $key=$value');
  }

  Future<void> updateLang(String langCode) async {
    await updateConfig("LangCode", langCode);
    langMap.value = await WoxApi.instance.getLangJson(langCode);
  }

  // get translation
  String tr(String key) {
    if (key.startsWith("i18n:")) {
      key = key.substring(5);
    }

    return langMap[key] ?? key;
  }

  // ---------- Plugins ----------

  Future<void> loadStorePlugins() async {
    // Check if cache is still valid
    if (_storePluginsCache.isNotEmpty && DateTime.now().difference(_lastStorePluginsFetchTime.value) < _cacheDuration) {
      pluginList.clear();
      pluginList.addAll(_storePluginsCache);
      return;
    }

    final storePlugins = await WoxApi.instance.findStorePlugins();
    storePlugins.sort((a, b) => a.name.compareTo(b.name));
    pluginList.clear();
    pluginList.addAll(storePlugins);

    // Update cache
    _storePluginsCache.clear();
    _storePluginsCache.addAll(storePlugins);
    _lastStorePluginsFetchTime.value = DateTime.now();
  }

  Future<void> loadInstalledPlugins() async {
    // Check if cache is still valid
    if (_installedPluginsCache.isNotEmpty && DateTime.now().difference(_lastInstalledPluginsFetchTime.value) < _cacheDuration) {
      pluginList.clear();
      pluginList.addAll(_installedPluginsCache);
      return;
    }

    final installedPlugin = await WoxApi.instance.findInstalledPlugins();
    installedPlugin.sort((a, b) => a.name.compareTo(b.name));
    pluginList.clear();
    pluginList.addAll(installedPlugin);

    // Update cache
    _installedPluginsCache.clear();
    _installedPluginsCache.addAll(installedPlugin);
    _lastInstalledPluginsFetchTime.value = DateTime.now();
  }

  Future<void> refreshPluginList() async {
    try {
      isLoadingPlugins.value = true;

      if (isStorePluginList.value) {
        await loadStorePlugins();
      } else {
        await loadInstalledPlugins();
      }

      filterPlugins();

      //active plugin
      if (activePlugin.value.id.isNotEmpty) {
        activePlugin.value = filteredPluginList.firstWhere((element) => element.id == activePlugin.value.id,
            orElse: () => filteredPluginList.isNotEmpty ? filteredPluginList[0] : PluginDetail.empty());
      } else {
        setFirstFilteredPluginDetailActive();
      }
    } finally {
      isLoadingPlugins.value = false;
    }
  }

  Future<void> switchToPluginList(bool isStorePlugin) async {
    // If we're already loading, ignore the request
    if (isLoadingPlugins.value) return;

    activePaneIndex.value = isStorePlugin ? 5 : 6;
    isStorePluginList.value = isStorePlugin;
    activePlugin.value = PluginDetail.empty();
    filterPluginKeywordController.text = "";
    await refreshPluginList();
    setFirstFilteredPluginDetailActive();
  }

  void setFirstFilteredPluginDetailActive() {
    if (filteredPluginList.isNotEmpty) {
      activePlugin.value = filteredPluginList[0];
    }
  }

  Future<void> installPlugin(PluginDetail plugin) async {
    try {
      isInstallingPlugin.value = true;
      Logger.instance.info(const UuidV4().generate(), 'installing plugin: ${plugin.name}');
      await WoxApi.instance.installPlugin(plugin.id);
      // Clear both caches when installing a plugin
      _lastStorePluginsFetchTime.value = DateTime(2000);
      _lastInstalledPluginsFetchTime.value = DateTime(2000);
      await refreshPluginList();
    } catch (e) {
      Get.snackbar(
        'Installation Failed',
        e.toString(),
        snackPosition: SnackPosition.TOP,
        backgroundColor: Colors.red,
        colorText: Colors.white,
        duration: const Duration(seconds: 5),
      );
    } finally {
      isInstallingPlugin.value = false;
    }
  }

  Future<void> disablePlugin(PluginDetail plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'disabling plugin: ${plugin.name}');
    await WoxApi.instance.disablePlugin(plugin.id);
    await refreshPluginList();
  }

  Future<void> enablePlugin(PluginDetail plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'enabling plugin: ${plugin.name}');
    await WoxApi.instance.enablePlugin(plugin.id);
    await refreshPluginList();
  }

  Future<void> uninstallPlugin(PluginDetail plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'uninstalling plugin: ${plugin.name}');
    await WoxApi.instance.uninstallPlugin(plugin.id);
    // Clear both caches when uninstalling a plugin
    _lastStorePluginsFetchTime.value = DateTime(2000);
    _lastInstalledPluginsFetchTime.value = DateTime(2000);
    await refreshPluginList();
  }

  filterPlugins() {
    filteredPluginList.clear();

    if (filterPluginKeywordController.text.isEmpty) {
      filteredPluginList.addAll(pluginList);
    } else {
      filteredPluginList.addAll(pluginList.where((element) => element.name.toLowerCase().contains(filterPluginKeywordController.text.toLowerCase())));
    }
  }

  Future<void> openPluginWebsite(String website) async {
    await launchUrl(Uri.parse(website));
  }

  Future<void> updatePluginSetting(String pluginId, String key, String value) async {
    await WoxApi.instance.updatePluginSetting(pluginId, key, value);
    Logger.instance.info(const UuidV4().generate(), 'plugin setting updated: $key=$value');
  }

  Future<void> updatePluginTriggerKeywords(String pluginId, List<String> triggerKeywords) async {
    await updatePluginSetting(pluginId, "TriggerKeywords", triggerKeywords.join(","));
  }

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

  Future<void> loadStoreThemes() async {
    final storeThemes = await WoxApi.instance.findStoreThemes();
    storeThemes.sort((a, b) => a.themeName.compareTo(b.themeName));
    themeList.clear();
    for (var theme in storeThemes) {
      themeList.add(theme);
    }
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList);
  }

  Future<void> loadInstalledThemes() async {
    final installThemes = await WoxApi.instance.findInstalledThemes();
    installThemes.sort((a, b) => a.themeName.compareTo(b.themeName));
    themeList.clear();
    for (var theme in installThemes) {
      themeList.add(theme);
    }
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList);
  }

  Future<void> installTheme(WoxTheme theme) async {
    Logger.instance.info(const UuidV4().generate(), 'Installing theme: ${theme.themeId}');
    await WoxApi.instance.installTheme(theme.themeId);
    await refreshThemeList();
  }

  Future<void> uninstallTheme(WoxTheme theme) async {
    Logger.instance.info(const UuidV4().generate(), 'Uninstalling theme: ${theme.themeId}');
    await WoxApi.instance.uninstallTheme(theme.themeId);
    await refreshThemeList();
  }

  Future<void> applyTheme(WoxTheme theme) async {
    Logger.instance.info(const UuidV4().generate(), 'Applying theme: ${theme.themeId}');
    await WoxApi.instance.applyTheme(theme.themeId);
    await refreshThemeList();
    // refresh wox setting to make the UI updated
    await WoxSettingUtil.instance.loadSetting();
    woxSetting.value = WoxSettingUtil.instance.currentSetting;
  }

  onFilterThemes(String filter) {
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList.where((element) => element.themeName.toLowerCase().contains(filter.toLowerCase())));
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
    if (activeTheme.value.themeId.isNotEmpty) {
      activeTheme.value = filteredThemeList.firstWhere((element) => element.themeId == activeTheme.value.themeId, orElse: () => filteredThemeList[0]);
    } else {
      setFirstFilteredThemeActive();
    }
  }

  Future<void> switchToThemeList(bool isStoreTheme) async {
    activePaneIndex.value = isStoreTheme ? 8 : 9;
    isStoreThemeList.value = isStoreTheme;
    activeTheme.value = WoxTheme.empty();
    await refreshThemeList();
    setFirstFilteredThemeActive();
  }

  Future<void> loadUserDataLocation() async {
    userDataLocation.value = await WoxApi.instance.getUserDataLocation();
  }

  Future<void> updateUserDataLocation(String newLocation) async {
    await WoxApi.instance.updateUserDataLocation(newLocation);
    userDataLocation.value = newLocation;
  }
}
