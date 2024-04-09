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
  final filteredPluginDetails = <PluginDetail>[].obs;
  final activePluginDetail = PluginDetail.empty().obs;

  final activePluginDetailTab = 0.obs;
  final isStorePluginList = true.obs;

  //themes
  late List<WoxSettingTheme> themeList = <WoxSettingTheme>[];
  final filteredThemeList = <WoxSettingTheme>[].obs;
  final activeTheme = WoxSettingTheme.empty().obs;

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
    filteredPluginDetails.clear();
    filteredPluginDetails.addAll(pluginDetails);
  }

  Future<void> loadInstalledPlugins() async {
    final installedPlugin = await WoxApi.instance.findInstalledPlugins();
    installedPlugin.sort((a, b) => a.name.compareTo(b.name));
    pluginDetails.clear();
    pluginDetails.addAll(installedPlugin);
    filteredPluginDetails.clear();
    filteredPluginDetails.addAll(pluginDetails);
  }

  Future<void> refreshPluginList() async {
    if (isStorePluginList.value) {
      await loadStorePlugins();
    } else {
      await loadInstalledPlugins();
    }

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

  onFilterPlugins(String filter) {
    filteredPluginDetails.clear();
    filteredPluginDetails.addAll(pluginDetails.where((element) => element.name.toLowerCase().contains(filter.toLowerCase())));
    if (filteredPluginDetails.isNotEmpty) {
      activePluginDetail.value = filteredPluginDetails[0];
    }
  }

  Future<void> openPluginWebsite(String website) async {
    await launchUrl(Uri.parse(website));
  }

  // ---------- Themes ----------

  void loadStoreThemes() async {
    themeList = await WoxApi.instance.findStoreThemes();
    filteredThemeList.clear();
    filteredThemeList.addAll(themeList);
  }

  void loadInstalledThemes() async {
    themeList = await WoxApi.instance.findInstalledThemes();
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
}
