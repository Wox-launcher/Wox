import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingProxyView extends GetView<WoxSettingController> {
  
  const WoxSettingProxyView({super.key});

  Widget form({required double width, required List<Widget> children}) {
    return Column(
      children: [
        ...children.map((e) => SizedBox(
              width: width,
              child: e,
            )),
      ],
    );
  }

  Widget formField({required String label, required Widget child, String? tips, double labelWidth = 140}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 18),
      child: Column(
        children: [
          Row(
            children: [
              Padding(
                padding: const EdgeInsets.only(right: 20),
                child: SizedBox(width: labelWidth, child: Text(label, textAlign: TextAlign.right)),
              ),
              child,
            ],
          ),
          if (tips != null)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Row(
                children: [
                  Padding(
                    padding: const EdgeInsets.only(right: 20),
                    child: SizedBox(width: labelWidth, child: const Text("")),
                  ),
                  Flexible(
                    child: Text(
                      tips,
                      style: TextStyle(color: Colors.grey[90], fontSize: 13),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      child: Obx(() {
        return Padding(
          padding: const EdgeInsets.all(20),
          child: form(
            width: 850,
            children: [
              formField(
                label: controller.tr("ui_proxy_enabled"),
                child: ToggleSwitch(
                  checked: controller.woxSetting.value.httpProxyEnabled,
                  onChanged: (value) => controller.updateConfig('HttpProxyEnabled', value.toString()),
                ),
              ),
              formField(
                label: controller.tr("ui_proxy_url"),
                tips: controller.tr("ui_proxy_url_tips"),
                child: SizedBox(
                  width: 400,
                  child: TextBox(
                    enabled: controller.woxSetting.value.httpProxyEnabled,
                    controller: TextEditingController(text: controller.woxSetting.value.httpProxyUrl),
                    onChanged: (value) => controller.updateConfig('HttpProxyUrl', value),
                  ),
                ),
              ),
            ],
          ),
        );
      }),
    );
  }
}
