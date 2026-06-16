import 'dart:async';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingUpdateView extends WoxSettingBaseView {
  const WoxSettingUpdateView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return form(
        title: controller.tr("ui_update"),
        description: controller.tr("ui_update_description"),
        children: [
          formSection(
            title: controller.tr("ui_update_section_updates"),
            children: [
              formField(
                settingKey: "EnableAutoUpdate",
                label: controller.tr("ui_enable_auto_update"),
                tips: controller.tr("ui_enable_auto_update_tips"),
                child: WoxSwitch(
                  value: controller.woxSetting.value.enableAutoUpdate,
                  onChanged: (bool value) {
                    controller.updateConfig("EnableAutoUpdate", value.toString());
                    _refreshDoctorAfterUpdateSettingChanges();
                  },
                ),
              ),
              formField(
                settingKey: "ReleaseChannel",
                label: controller.tr("ui_release_channel"),
                tips: controller.tr("ui_release_channel_tips"),
                child: Obx(() {
                  final stableVersion = controller.getUpdateChannelVersionText("stable");
                  final betaVersion = controller.getUpdateChannelVersionText("beta");

                  return WoxDropdownButton<String>(
                    items: [
                      WoxDropdownItem(
                        value: "stable",
                        label: controller.tr("ui_release_channel_stable"),
                        tooltip: controller.tr("ui_release_channel_stable_tips"),
                        trailing: _buildUpdateChannelVersion(stableVersion),
                      ),
                      WoxDropdownItem(
                        value: "beta",
                        label: controller.tr("ui_release_channel_beta"),
                        tooltip: controller.tr("ui_release_channel_beta_tips"),
                        trailing: _buildUpdateChannelVersion(betaVersion),
                      ),
                    ],
                    value: controller.woxSetting.value.releaseChannel,
                    onChanged: (v) {
                      if (v != null) {
                        controller.updateConfig("ReleaseChannel", v);
                        _refreshDoctorAfterUpdateSettingChanges();
                      }
                    },
                    isExpanded: true,
                  );
                }),
              ),
            ],
          ),
        ],
      );
    });
  }

  void _refreshDoctorAfterUpdateSettingChanges() {
    // The backend refreshes update metadata asynchronously after update settings change, so delay the doctor refresh until the new state is available.
    Future.delayed(const Duration(seconds: 2), () {
      Get.find<WoxLauncherController>().doctorCheck();
    });
  }

  Widget? _buildUpdateChannelVersion(String version) {
    if (version.isEmpty) {
      return null;
    }

    return Text(version, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12));
  }
}
