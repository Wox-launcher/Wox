import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingUIView extends WoxSettingBaseView {
  const WoxSettingUIView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return form(children: [
        formField(
          label: controller.tr("ui_show_position"),
          tips: controller.tr("ui_show_position_tips"),
          child: Obx(() {
            return SizedBox(
              width: 200,
              child: DropdownButton<String>(
                items: [
                  DropdownMenuItem(
                    value: "mouse_screen",
                    child: Text(controller.tr("ui_show_position_mouse_screen")),
                  ),
                  DropdownMenuItem(
                    value: "active_screen",
                    child: Text(controller.tr("ui_show_position_active_screen")),
                  ),
                  DropdownMenuItem(
                    value: "last_location",
                    child: Text(controller.tr("ui_show_position_last_location")),
                  ),
                ],
                value: controller.woxSetting.value.showPosition,
                onChanged: (v) {
                  if (v != null) {
                    controller.updateConfig("ShowPosition", v);
                  }
                },
                isExpanded: true,
                style: TextStyle(color: getThemeTextColor(), fontSize: 13),
                dropdownColor: getThemeActiveBackgroundColor().withOpacity(0.95),
                iconEnabledColor: getThemeTextColor(),
              ),
            );
          }),
        ),
        formField(
          label: controller.tr("ui_show_tray"),
          tips: controller.tr("ui_show_tray_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.showTray,
              onChanged: (bool value) {
                controller.updateConfig("ShowTray", value.toString());
              },
            );
          }),
        ),
        formField(
          label: controller.tr("ui_app_width"),
          tips: controller.tr("ui_app_width_tips"),
          child: Obx(() {
            return Transform.translate(
              offset: const Offset(-20, 0),
              child: Row(
                children: [
                  Expanded(
                    child: SliderTheme(
                      data: SliderThemeData(
                        activeTrackColor: getThemeActiveBackgroundColor(),
                        inactiveTrackColor: getThemeTextColor().withOpacity(0.3),
                        thumbColor: getThemeActiveBackgroundColor(),
                        overlayColor: getThemeActiveBackgroundColor().withOpacity(0.2),
                        valueIndicatorColor: getThemeActiveBackgroundColor(),
                        valueIndicatorTextStyle: TextStyle(color: getThemeTextColor()),
                      ),
                      child: Slider(
                        value: controller.woxSetting.value.appWidth.toDouble(),
                        min: 600,
                        max: 1600,
                        divisions: 20,
                        onChanged: (double value) {
                          controller.updateConfig("AppWidth", value.toInt().toString());
                        },
                      ),
                    ),
                  ),
                  const SizedBox(width: 16),
                  Text(
                    '${controller.woxSetting.value.appWidth}',
                    style: TextStyle(color: getThemeTextColor(), fontSize: 13),
                  ),
                ],
              ),
            );
          }),
        ),
        formField(
          label: controller.tr("ui_max_result_count"),
          tips: controller.tr("ui_max_result_count_tips"),
          child: Obx(() {
            return DropdownButton<int>(
              value: controller.woxSetting.value.maxResultCount,
              items: List.generate(11, (index) => index + 5)
                  .map(
                    (count) => DropdownMenuItem<int>(
                      value: count,
                      child: Text(count.toString()),
                    ),
                  )
                  .toList(),
              onChanged: (v) {
                if (v != null) {
                  controller.updateConfig("MaxResultCount", v.toString());
                }
              },
              style: TextStyle(color: getThemeTextColor(), fontSize: 13),
              dropdownColor: getThemeActiveBackgroundColor().withOpacity(0.95),
              iconEnabledColor: getThemeTextColor(),
            );
          }),
        ),
      ]);
    });
  }
}
