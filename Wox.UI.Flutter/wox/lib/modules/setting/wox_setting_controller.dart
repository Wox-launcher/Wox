import 'package:get/get.dart';
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
  final pluginDetails = <PluginDetail>[];
  final filteredPluginDetails = <PluginDetail>[].obs;
  final activePluginDetail = PluginDetail.empty().obs;
  var rawStoreThemes = <WoxTheme>[];
  var rawInstalledThemes = <WoxTheme>[];
  final storeThemes = <WoxTheme>[].obs;
  final installedThemes = <WoxTheme>[].obs;

  void hideWindow() {
    Get.find<WoxLauncherController>().isInSettingView.value = false;
  }

  Future<void> updateConfig(String key, String value) async {
    await WoxApi.instance.updateSetting(key, value);
    await WoxSettingUtil.instance.loadSetting();
    woxSetting.value = WoxSettingUtil.instance.currentSetting;
    Logger.instance.info(const UuidV4().generate(), 'Setting updated: $key=$value');
  }

  void loadStorePlugins() async {
    final rawStorePlugins = await WoxApi.instance.findStorePlugins();
    pluginDetails.clear();
    for (var plugin in rawStorePlugins) {
      pluginDetails.add(PluginDetail.fromStorePlugin(plugin));
    }
    filteredPluginDetails.clear();
    filteredPluginDetails.addAll(pluginDetails);
    if (filteredPluginDetails.isNotEmpty) {
      activePluginDetail.value = filteredPluginDetails[0];
    }
  }

  void loadInstalledPlugins() async {
    final installedPlugin = await WoxApi.instance.findInstalledPlugins();
    pluginDetails.clear();
    for (var plugin in installedPlugin) {
      pluginDetails.add(PluginDetail.fromInstalledPlugin(plugin));
    }
    filteredPluginDetails.clear();
    filteredPluginDetails.addAll(pluginDetails);
    if (filteredPluginDetails.isNotEmpty) {
      activePluginDetail.value = filteredPluginDetails[0];
    }
  }

  Future<void> install(StorePlugin plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'Installing plugin: ${plugin.id}');
    await WoxApi.instance.installPlugin(plugin.id);
    loadStorePlugins();
  }

  Future<void> uninstall(InstalledPlugin plugin) async {
    Logger.instance.info(const UuidV4().generate(), 'Uninstalling plugin: ${plugin.id}');
    await WoxApi.instance.uninstallPlugin(plugin.id);
    loadInstalledPlugins();
  }

  onFilterPlugins(String filter) {
    filteredPluginDetails.clear();
    filteredPluginDetails.addAll(pluginDetails.where((element) => element.name.toLowerCase().contains(filter.toLowerCase())));
  }

  void loadStoreThemes() async {
    rawStoreThemes = await WoxApi.instance.findStoreThemes();
  }

  void loadInstalledThemes() async {
    rawInstalledThemes = await WoxApi.instance.findInstalledThemes();
  }

  Future<void> installTheme(WoxTheme theme) async {
    Logger.instance.info(const UuidV4().generate(), 'Installing theme: ${theme.themeId}');
    await WoxApi.instance.installTheme(theme.themeId);
    loadStoreThemes();
  }

  Future<void> uninstallTheme(WoxTheme theme) async {
    Logger.instance.info(const UuidV4().generate(), 'Uninstalling theme: ${theme.themeId}');
    await WoxApi.instance.uninstallTheme(theme.themeId);
    loadInstalledThemes();
  }

  onFilterStoreThemes(String filter) {
    storeThemes.clear();
    storeThemes.addAll(rawStoreThemes.where((element) => element.themeName.toLowerCase().contains(filter.toLowerCase())));
  }

  onFilterInstalledThemes(String filter) {
    installedThemes.clear();
    installedThemes.addAll(rawInstalledThemes.where((element) => element.themeName.toLowerCase().contains(filter.toLowerCase())));
  }
}
