import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';

class WoxSettingController extends GetxController {
  final activePaneIndex = 0.obs;
  final woxSetting = WoxSettingUtil.instance.currentSetting.obs;
  final storePlugins = <StorePlugin>[].obs;

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
    storePlugins.value = await WoxApi.instance.findStorePlugins();
  }
}
