import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';

class WoxSettingController extends GetxController {
  final activePaneIndex = 0.obs;
  final woxSetting = WoxSettingUtil.instance.currentSetting.obs;
  final userDataLocation = "".obs;
  final backups = <WoxBackup>[].obs;

  //plugins
  final pluginList = <PluginDetail>[];
  final storePlugins = <PluginDetail>[];
  final installedPlugins = <PluginDetail>[];
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

  @override
  void onInit() {
    super.onInit();
    refreshThemeList();
    loadUserDataLocation();
    refreshBackups();
  }

  void hideWindow() {
    Get.find<WoxLauncherController>().isInSettingView.value = false;
  }

  Future<void> updateConfig(String key, String value) async {
    await WoxApi.instance.updateSetting(key, value);
    await reloadSetting();
    Logger.instance.info(const UuidV4().generate(), 'Setting updated: $key=$value');


    if (key == "AIProviders") {
      Get.find<WoxLauncherController>().reloadAIModels();
    }
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
    final storePluginsFromAPI = await WoxApi.instance.findStorePlugins();
    storePluginsFromAPI.sort((a, b) => a.name.compareTo(b.name));
    storePlugins.clear();
    storePlugins.addAll(storePluginsFromAPI);
  }

  Future<void> loadInstalledPlugins() async {
    final installedPluginsFromAPI = await WoxApi.instance.findInstalledPlugins();
    installedPluginsFromAPI.sort((a, b) => a.name.compareTo(b.name));
    installedPlugins.clear();
    installedPlugins.addAll(installedPluginsFromAPI);
  }

  Future<void> refreshPlugin(String pluginId, String refreshType /* update / add / remove */) async {
    Logger.instance.info(const UuidV4().generate(), 'Refreshing plugin: $pluginId, refreshType: $refreshType');
    if (refreshType == "add") {
      PluginDetail updatedPlugin = await WoxApi.instance.getPluginDetail(pluginId);
      if (updatedPlugin.id.isEmpty) {
        Logger.instance.info(const UuidV4().generate(), 'Plugin not found: $pluginId');
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
      if (activePaneIndex.value == 6) {
        pluginList.removeWhere((p) => p.id == pluginId);
        filteredPluginList.removeWhere((p) => p.id == pluginId);
      }
      // if is in store plugin view, update the installed property
      if (activePaneIndex.value == 5) {
        pluginList.firstWhere((p) => p.id == pluginId).isInstalled = false;
        filteredPluginList.firstWhere((p) => p.id == pluginId).isInstalled = false;
      }
      if (activePlugin.value.id == pluginId) {
        activePlugin.value = installedPlugins.isNotEmpty ? installedPlugins[0] : PluginDetail.empty();
      }
    } else if (refreshType == "update") {
      PluginDetail updatedPlugin = await WoxApi.instance.getPluginDetail(pluginId);
      if (updatedPlugin.id.isEmpty) {
        Logger.instance.info(const UuidV4().generate(), 'Plugin not found: $pluginId');
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
  }

  Future<void> switchToPluginList(bool isStorePlugin) async {
    if (isStorePlugin) {
      pluginList.clear();
      pluginList.addAll(storePlugins);
    } else {
      pluginList.clear();
      pluginList.addAll(installedPlugins);
    }

    activePaneIndex.value = isStorePlugin ? 6 : 7;
    isStorePluginList.value = isStorePlugin;
    activePlugin.value = PluginDetail.empty();
    filterPluginKeywordController.text = "";

    filterPlugins();

    //active plugin
    if (activePlugin.value.id.isNotEmpty) {
      activePlugin.value = filteredPluginList.firstWhere((element) => element.id == activePlugin.value.id,
          orElse: () => filteredPluginList.isNotEmpty ? filteredPluginList[0] : PluginDetail.empty());
    } else {
      setFirstFilteredPluginDetailActive();
    }
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
      await refreshPlugin(plugin.id, "add");
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
    await refreshPlugin(plugin.id, "update");
  }

  Future<void> enablePlugin(PluginDetail plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'enabling plugin: ${plugin.name}');
    await WoxApi.instance.enablePlugin(plugin.id);
    await refreshPlugin(plugin.id, "update");
  }

  Future<void> uninstallPlugin(PluginDetail plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'uninstalling plugin: ${plugin.name}');
    await WoxApi.instance.uninstallPlugin(plugin.id);
    await refreshPlugin(plugin.id, "remove");
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
    final activeTabIndex = activePluginTabController.index;

    await WoxApi.instance.updatePluginSetting(pluginId, key, value);
    await refreshPlugin(pluginId, "update");
    Logger.instance.info(const UuidV4().generate(), 'plugin setting updated: $key=$value');

    // switch to the tab that was active before the update
    WidgetsBinding.instance.addPostFrameCallback((timeStamp) {
      if (activePluginTabController.index != activeTabIndex) {
        activePluginTabController.index = activeTabIndex;
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
    await reloadSetting();
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
    activePaneIndex.value = isStoreTheme ? 9 : 10;
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

  Future<void> backupNow() async {
    await WoxApi.instance.backupNow();
    refreshBackups();
  }

  Future<void> refreshBackups() async {
    final result = await WoxApi.instance.getAllBackups();
    backups.assignAll(result);
  }

  Future<void> openFolder(String path) async {
    await WoxApi.instance.open(path);
  }

  Future<void> restoreBackup(String id) async {
    await WoxApi.instance.restoreBackup(id);
    await reloadSetting();
  }

  Future<void> reloadSetting() async {
    await WoxSettingUtil.instance.loadSetting();
    woxSetting.value = WoxSettingUtil.instance.currentSetting;
    Logger.instance.info(const UuidV4().generate(), 'Setting reloaded');
  }
}
