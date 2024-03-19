import 'package:get/get.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';

class WoxSettingController extends GetxController {
  final activePaneIndex = 0.obs;

  void hideWindow() {
    Get.find<WoxLauncherController>().isInSettingView.value = false;
  }
}
