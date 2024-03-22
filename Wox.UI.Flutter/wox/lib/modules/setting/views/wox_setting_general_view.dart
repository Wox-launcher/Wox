import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingGeneralView extends GetView<WoxSettingController> {
  const WoxSettingGeneralView({super.key});

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
    return Padding(
        padding: EdgeInsets.only(left: 10, top: 24),
        child: form(width: 800, children: [
          formField(
            label: "Hotkey",
            child: Text("data"),
          ),
          formField(
              label: "Themes",
              child: ComboBox<String>(
                placeholder: Text('select a theme'),
                value: "empty",
                items: [
                  ComboBoxItem<String>(child: Text('Light'), value: 'light'),
                  ComboBoxItem<String>(child: Text('Dark'), value: 'dark'),
                ],
                onChanged: (value) {
                  print(value);
                },
              )),
          formField(
              label: "Themes",
              tips: "In 'Preserve' model, it will show last query result, when you reopen wox launcher.",
              child: ComboBox<String>(
                placeholder: Text('select a theme'),
                value: "empty",
                items: [
                  ComboBoxItem<String>(child: Text('Light'), value: 'light'),
                  ComboBoxItem<String>(child: Text('Dark'), value: 'dark'),
                ],
                onChanged: (value) {
                  print(value);
                },
              )),
          formField(
            label: "Use PinYin",
            tips: "If selected, When searching, it converts Chinese into Pinyin and matches it.",
            child: Obx(() {
              return ToggleSwitch(
                checked: controller.woxSetting.value.usePinYin,
                onChanged: (bool value) {
                  controller.updateConfig("UsePinYin", value.toString());
                },
              );
            }),
          ),
          formField(
            label: "Hide On Lost Focus",
            tips: "If selected, When wox lost focus, it will be hidden.",
            child: ToggleSwitch(
              checked: true,
              onChanged: (bool value) {},
            ),
          ),
          formField(
            label: "Hide On Start",
            tips: "If selected, When wox start, it will be hidden.",
            child: ToggleSwitch(
              checked: true,
              onChanged: (bool value) {},
            ),
          ),
          formField(
            label: "Show Tray",
            tips: "If selected, When wox start, icon will be shown on tray.",
            child: ToggleSwitch(
              checked: true,
              onChanged: (bool value) {},
            ),
          ),
          formField(
            label: "Switch Input Method",
            tips: "If selected, input method will be switched to english, when enter input field.",
            child: ToggleSwitch(
              checked: true,
              onChanged: (bool value) {},
            ),
          ),
        ]));
  }
}
