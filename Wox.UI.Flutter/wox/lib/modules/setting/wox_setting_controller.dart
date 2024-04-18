import 'package:fluent_ui/fluent_ui.dart';
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

class WoxSettingController extends GetxController {
  final activePaneIndex = 0.obs;
  final woxSetting = WoxSettingUtil.instance.currentSetting.obs;

  //plugins
  final pluginDetails = <PluginDetail>[];
  final filterPluginKeywordController = TextEditingController();
  final filteredPluginDetails = <PluginDetail>[].obs;
  final activePluginDetail = PluginDetail.empty().obs;
  final isStorePluginList = true.obs;
  late TabController activePluginTabController;

  //themes
  final themeList = <WoxSettingTheme>[];
  final filteredThemeList = <WoxSettingTheme>[].obs;
  final activeTheme = WoxSettingTheme.empty().obs;
  final isStoreThemeList = true.obs;

  void hideWindow() {
    Get.find<WoxLauncherController>().isInSettingView.value = false;
  }

  Future<void> updateConfig(String key, String value) async {
    await WoxApi.instance.updateSetting(key, value);
    await WoxSettingUtil.instance.loadSetting();
    woxSetting.value = WoxSettingUtil.instance.currentSetting;
    Logger.instance.info(const UuidV4().generate(), 'Setting updated: $key=$value');
  }

  // ---------- Plugins ----------

  Future<void> loadStorePlugins() async {
    final storePlugins = await WoxApi.instance.findStorePlugins();
    storePlugins.sort((a, b) => a.name.compareTo(b.name));
    pluginDetails.clear();
    pluginDetails.addAll(storePlugins);
  }

  Future<void> loadInstalledPlugins() async {
    final installedPlugin = await WoxApi.instance.findInstalledPlugins();
    installedPlugin.sort((a, b) => a.name.compareTo(b.name));
    pluginDetails.clear();
    pluginDetails.addAll(installedPlugin);
  }

  Future<void> refreshPluginList() async {
    if (isStorePluginList.value) {
      await loadStorePlugins();
    } else {
      await loadInstalledPlugins();
    }

    filterPlugins();

    //active plugin
    if (activePluginDetail.value.id.isNotEmpty) {
      activePluginDetail.value = filteredPluginDetails.firstWhere((element) => element.id == activePluginDetail.value.id, orElse: () => filteredPluginDetails[0]);
    } else {
      setFirstFilteredPluginDetailActive();
    }
  }

  Future<void> switchToPluginList(bool isStorePlugin) async {
    activePaneIndex.value = isStorePlugin ? 2 : 3;
    isStorePluginList.value = isStorePlugin;
    activePluginDetail.value = PluginDetail.empty();
    filterPluginKeywordController.text = "";
    await refreshPluginList();
    setFirstFilteredPluginDetailActive();
  }

  void setFirstFilteredPluginDetailActive() {
    if (filteredPluginDetails.isNotEmpty) {
      activePluginDetail.value = filteredPluginDetails[0];
    }
  }

  Future<void> installPlugin(PluginDetail plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'installing plugin: ${plugin.name}');
    await WoxApi.instance.installPlugin(plugin.id);
    await refreshPluginList();
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
    await refreshPluginList();
  }

  filterPlugins() {
    filteredPluginDetails.clear();

    if (filterPluginKeywordController.text.isEmpty) {
      filteredPluginDetails.addAll(pluginDetails);
    } else {
      filteredPluginDetails.addAll(pluginDetails.where((element) => element.name.toLowerCase().contains(filterPluginKeywordController.text.toLowerCase())));
    }
  }

  Future<void> openPluginWebsite(String website) async {
    await launchUrl(Uri.parse(website));
  }

  Future<void> updatePluginSetting(String pluginId, String key, String value) async {
    await WoxApi.instance.updatePluginSetting(pluginId, key, value);
    Logger.instance.info(const UuidV4().generate(), 'plugin setting updated: $key=$value');
  }

  bool shouldShowSettingTab() {
    return activePluginDetail.value.isInstalled && activePluginDetail.value.settingDefinitions.isNotEmpty;
  }

  void switchToPluginSettingTab() {
    if (shouldShowSettingTab()) {
      activePluginTabController.animateTo(1, duration: Duration.zero);
    }
  }

  // ---------- Themes ----------

  Future<void> loadStoreThemes() async {
    final storeThemes = await WoxApi.instance.findStoreThemes();
    storeThemes.sort((a, b) => a.themeName.compareTo(b.themeName));
    themeList.clear();
    for (var theme in storeThemes) {
      themeList.add(WoxSettingTheme.fromWoxSettingTheme(theme));
    }
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList);
  }

  Future<void> loadInstalledThemes() async {
    final installThemes = await WoxApi.instance.findInstalledThemes();
    installThemes.sort((a, b) => a.themeName.compareTo(b.themeName));
    themeList.clear();
    for (var theme in installThemes) {
      themeList.add(WoxSettingTheme.fromWoxSettingTheme(theme));
    }
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList);
  }

  Future<void> installTheme(WoxSettingTheme theme) async {
    Logger.instance.info(const UuidV4().generate(), 'Installing theme: ${theme.themeId}');
    await WoxApi.instance.installTheme(theme.themeId);
    loadStoreThemes();
  }

  Future<void> uninstallTheme(WoxSettingTheme theme) async {
    Logger.instance.info(const UuidV4().generate(), 'Uninstalling theme: ${theme.themeId}');
    await WoxApi.instance.uninstallTheme(theme.themeId);
    loadInstalledThemes();
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
    activePaneIndex.value = isStoreTheme ? 5 : 6;
    isStoreThemeList.value = isStoreTheme;
    activeTheme.value = WoxSettingTheme.empty();
    await refreshThemeList();
    setFirstFilteredThemeActive();
  }
}
