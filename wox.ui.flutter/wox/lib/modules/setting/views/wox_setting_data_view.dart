import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/picker.dart';

class WoxSettingDataView extends WoxSettingBaseView {
  const WoxSettingDataView({super.key});

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      child: Padding(
        padding: const EdgeInsets.all(20),
        child: form(children: [
          formField(
            label: controller.tr("ui_data_config_location"),
            child: Obx(() {
              return TextBox(
                controller: TextEditingController(text: controller.userDataLocation.value),
                readOnly: true,
                suffix: Button(
                  child: Text(controller.tr("ui_data_config_location_change")),
                  onPressed: () async {
                    // Store the context before async operation
                    final currentContext = context;
                    final result = await FileSelector.pick(
                      const UuidV4().generate(),
                      FileSelectorParams(isDirectory: true),
                    );
                    if (result.isNotEmpty) {
                      if (currentContext.mounted) {
                        showDialog(
                          context: currentContext,
                          builder: (context) {
                            return ContentDialog(
                              content: Text(controller.tr("ui_data_config_location_change_confirm").replaceAll("{0}", result[0])),
                              actions: [
                                Button(
                                  child: Text(controller.tr("ui_data_config_location_change_cancel")),
                                  onPressed: () {
                                    Navigator.pop(context);
                                  },
                                ),
                                FilledButton(
                                  child: Text(controller.tr("ui_data_config_location_change_confirm_button")),
                                  onPressed: () {
                                    Navigator.pop(context);
                                    controller.updateUserDataLocation(result[0]);
                                  },
                                ),
                              ],
                            );
                          },
                        );
                      }
                    }
                  },
                ),
              );
            }),
            tips: controller.tr("ui_data_config_location_tips"),
          ),
        ]),
      ),
    );
  }
}
