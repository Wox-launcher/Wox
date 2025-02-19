import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingProxyView extends GetView<WoxSettingController> {
  const WoxSettingProxyView({super.key});

  @override
  Widget build(BuildContext context) {
    return ScaffoldPage(
      header: const PageHeader(
        title: Text('Network'),
      ),
      content: ListView(
        padding: const EdgeInsets.all(15),
        children: [
          const Text(
            'Proxy Settings',
            style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 10),
          Obx(() => ToggleSwitch(
                checked: controller.woxSetting.value.httpProxyEnabled,
                onChanged: (value) => controller.updateConfig('HttpProxyEnabled', value.toString()),
                content: const Text('Enable HTTP Proxy'),
              )),
          const SizedBox(height: 10),
          Obx(() => TextBox(
                prefix: const Text('Proxy URL'),
                placeholder: 'http://localhost:7890 or socks5://localhost:7890',
                enabled: controller.woxSetting.value.httpProxyEnabled,
                onChanged: (value) => controller.updateConfig('HttpProxyUrl', value),
              )),
          const SizedBox(height: 10),
          const InfoBar(
            title: Text('Note'),
            content: Text('Proxy changes will take effect after restarting the application'),
            severity: InfoBarSeverity.info,
          ),
        ],
      ),
    );
  }
}
