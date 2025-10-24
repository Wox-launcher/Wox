import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/components/wox_switch.dart';

class WoxSettingNetworkView extends WoxSettingBaseView {
  const WoxSettingNetworkView({super.key});

  @override
  Widget build(BuildContext context) {
    final TextEditingController proxyUrlController = TextEditingController(
      text: controller.woxSetting.value.httpProxyUrl,
    );

    return Obx(() {
      return form(
        children: [
          formField(
            label: controller.tr("ui_proxy_enabled"),
            child: WoxSwitch(
              value: controller.woxSetting.value.httpProxyEnabled,
              onChanged: (value) => controller.updateConfig(
                'HttpProxyEnabled',
                value.toString(),
              ),
            ),
          ),
          formField(
            label: controller.tr("ui_proxy_url"),
            tips: controller.tr("ui_proxy_url_tips"),
            child: SizedBox( 
              width: 400,
              child: TextField(
                enabled: controller.woxSetting.value.httpProxyEnabled,
                controller: proxyUrlController,
                onChanged: (value) => controller.updateConfig('HttpProxyUrl', value),
              ),
            ),
          ),
        ],
      );
    });
  }
}
