import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_plugin.dart';
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
        padding: const EdgeInsets.all(20),
        child: form(width: 850, children: [
          formField(
            label: "Hotkey",
            tips: "Hotkeys to open or hide Wox.",
            child: WoxHotkeyRecorder(
              hotkey: WoxHotkey.parseHotkey(controller.woxSetting.value.mainHotkey),
              onHotKeyRecorded: (hotkey) {
                controller.updateConfig("MainHotkey", hotkey);
              },
            ),
          ),
          formField(
            label: "Selection Hotkey",
            tips: "Hotkeys to do actions on selected text or files.",
            child: WoxHotkeyRecorder(
              hotkey: WoxHotkey.parseHotkey(controller.woxSetting.value.selectionHotkey),
              onHotKeyRecorded: (hotkey) {
                controller.updateConfig("SelectionHotkey", hotkey);
              },
            ),
          ),
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
            child: Obx(() {
              return ToggleSwitch(
                checked: controller.woxSetting.value.hideOnLostFocus,
                onChanged: (bool value) {
                  controller.updateConfig("HideOnLostFocus", value.toString());
                },
              );
            }),
          ),
          formField(
            label: "Hide On Start",
            tips: "If selected, When wox start, it will be hidden.",
            child: Obx(() {
              return ToggleSwitch(
                checked: controller.woxSetting.value.hideOnStart,
                onChanged: (bool value) {
                  controller.updateConfig("HideOnStart", value.toString());
                },
              );
            }),
          ),
          formField(
            label: "Show Tray",
            tips: "If selected, When wox start, icon will be shown on tray.",
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
            label: "Switch Input Method",
            tips: "If selected, input method will be switched to english, when enter input field.",
            child: Obx(() {
              return ToggleSwitch(
                checked: controller.woxSetting.value.switchInputMethodABC,
                onChanged: (bool value) {
                  controller.updateConfig("SwitchInputMethodABC", value.toString());
                },
              );
            }),
          ),
          formField(
            label: "Query Hotkeys",
            child: Obx(() {
              PluginDetail fakePluginDetail = PluginDetail.empty();
              fakePluginDetail.setting = PluginSetting.empty();
              fakePluginDetail.setting.settings["QueryHotkeys"] = json.encode(controller.woxSetting.value.queryHotkeys);
              PluginSettingValueTable fakePluginSettingValueTable = PluginSettingValueTable.fromJson({
                "Key": "QueryHotkeys",
                "Columns": [
                  {
                    "Key": "Hotkey",
                    "Label": "Hotkey",
                    "Tooltip": "The hotkey to trigger the query.",
                    "Width": 120,
                    "Type": "hotkey",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"}
                    ],
                  },
                  {
                    "Key": "Query",
                    "Label": "Query",
                    "Tooltip": "The query when the hotkey is triggered.",
                    "Type": "text",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"}
                    ],
                  }
                ],
                "SortColumnKey": "Query"
              });
              return WoxSettingPluginTable(fakePluginDetail, fakePluginSettingValueTable, (key, value) {
                controller.updateConfig("QueryHotkeys", value);
              });
            }),
          ),
        ]));
  }
}
