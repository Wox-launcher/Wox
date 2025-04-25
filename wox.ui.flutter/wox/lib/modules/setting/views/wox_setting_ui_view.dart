import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

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
            return ComboBox<String>(
              items: [
                ComboBoxItem(
                  value: "mouse_screen",
                  child: Text(controller.tr("ui_show_position_mouse_screen")),
                ),
                ComboBoxItem(
                  value: "active_screen",
                  child: Text(controller.tr("ui_show_position_active_screen")),
                ),
                ComboBoxItem(
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
            );
          }),
        ),
        formField(
          label: controller.tr("ui_show_tray"),
          tips: controller.tr("ui_show_tray_tips"),
          child: Obx(() {
            return ToggleSwitch(
              checked: controller.woxSetting.value.showTray,
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
            return Row(
              children: [
                Expanded(
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
                const SizedBox(width: 16),
                Text('${controller.woxSetting.value.appWidth}'),
              ],
            );
          }),
        ),
        formField(
          label: controller.tr("ui_max_result_count"),
          tips: controller.tr("ui_max_result_count_tips"),
          child: Obx(() {
            return ComboBox<int>(
              value: controller.woxSetting.value.maxResultCount,
              items: List.generate(11, (index) => index + 5)
                  .map(
                    (count) => ComboBoxItem<int>(
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
            );
          }),
        ),
      ]);
    });
  }
}
