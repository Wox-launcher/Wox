import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

class WoxSettingProxyView extends WoxSettingBaseView {
  const WoxSettingProxyView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return form(children: [
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
      ]);
    });
  }
}
