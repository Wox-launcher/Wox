import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

class WoxSettingDataView extends WoxSettingBaseView {
  const WoxSettingDataView({super.key});

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
                label: controller.tr("ui_data_config_location"),
                child: Text("配置位置"),
              ),            
            ],
          ),
        );
      }),
    );
  }
}
