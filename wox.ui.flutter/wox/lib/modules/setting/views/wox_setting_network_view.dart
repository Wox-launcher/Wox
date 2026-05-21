import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/components/wox_textfield.dart';

class WoxSettingNetworkView extends WoxSettingBaseView {
  const WoxSettingNetworkView({super.key});

  @override
  Widget build(BuildContext context) {
    final TextEditingController proxyUrlController = TextEditingController(text: controller.woxSetting.value.httpProxyUrl);

    return Obx(() {
      return form(
        title: controller.tr("ui_network"),
        description: controller.tr("ui_network_description"),
        children: [
          formField(
            settingKey: "HttpProxyEnabled",
            label: controller.tr("ui_proxy_enabled"),
            child: WoxSwitch(value: controller.woxSetting.value.httpProxyEnabled, onChanged: (value) => controller.updateConfig('HttpProxyEnabled', value.toString())),
          ),
          formField(
            settingKey: "HttpProxyUrl",
            label: controller.tr("ui_proxy_url"),
            tips: controller.tr("ui_proxy_url_tips"),
            child: WoxTextField(
              enabled: controller.woxSetting.value.httpProxyEnabled,
              controller: proxyUrlController,
              onChanged: (value) => controller.updateConfig('HttpProxyUrl', value),
            ),
          ),
        ],
      );
    });
  }
}
