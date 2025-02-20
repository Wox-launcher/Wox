import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_lang.dart';
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
    return SingleChildScrollView(
      child: Obx(() {
        return Padding(
            padding: const EdgeInsets.all(20),
            child: form(width: 850, children: [
              formField(
                label: controller.tr("autostart"),
                tips: controller.tr("autostart_tips"),
                child: ToggleSwitch(
                  checked: controller.woxSetting.value.enableAutostart,
                  onChanged: (bool value) {
                    controller.updateConfig("EnableAutostart", value.toString());
                  },
                ),
              ),
              formField(
                label: controller.tr("hotkey"),
                tips: controller.tr("hotkey_tips"),
                child: WoxHotkeyRecorder(
                  hotkey: WoxHotkey.parseHotkeyFromString(controller.woxSetting.value.mainHotkey),
                  onHotKeyRecorded: (hotkey) {
                    controller.updateConfig("MainHotkey", hotkey);
                  },
                ),
              ),
              formField(
                label: controller.tr("selection_hotkey"),
                tips: controller.tr("selection_hotkey_tips"),
                child: WoxHotkeyRecorder(
                  hotkey: WoxHotkey.parseHotkeyFromString(controller.woxSetting.value.selectionHotkey),
                  onHotKeyRecorded: (hotkey) {
                    controller.updateConfig("SelectionHotkey", hotkey);
                  },
                ),
              ),
              formField(
                label: controller.tr("use_pinyin"),
                tips: controller.tr("use_pinyin_tips"),
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
                label: controller.tr("hide_on_lost_focus"),
                tips: controller.tr("hide_on_lost_focus_tips"),
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
                label: controller.tr("hide_on_start"),
                tips: controller.tr("hide_on_start_tips"),
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
                label: controller.tr("show_tray"),
                tips: controller.tr("show_tray_tips"),
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
                label: controller.tr("switch_input_method_abc"),
                tips: controller.tr("switch_input_method_abc_tips"),
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
                label: controller.tr("lang"),
                child: FutureBuilder(
                    future: WoxApi.instance.getAllLanguages(),
                    builder: (context, snapshot) {
                      if (snapshot.connectionState == ConnectionState.done) {
                        final languages = snapshot.data as List<WoxLang>;
                        return Obx(() {
                          return ComboBox<String>(
                            items: languages.map((e) {
                              return ComboBoxItem(
                                value: e.code,
                                child: Text(e.name),
                              );
                            }).toList(),
                            value: controller.woxSetting.value.langCode,
                            onChanged: (v) {
                              if (v != null) {
                                controller.updateLang(v);
                              }
                            },
                          );
                        });
                      }
                      return const SizedBox();
                    }),
              ),
              formField(
                label: controller.tr("query_hotkeys"),
                child: Obx(() {
                  return WoxSettingPluginTable(
                    value: json.encode(controller.woxSetting.value.queryHotkeys),
                    item: PluginSettingValueTable.fromJson({
                      "Key": "QueryHotkeys",
                      "Columns": [
                        {
                          "Key": "Hotkey",
                          "Label": "i18n:query_hotkeys_hotkey",
                          "Tooltip": "i18n:query_hotkeys_hotkey_tooltip",
                          "Width": 120,
                          "Type": "hotkey",
                          "TextMaxLines": 1,
                          "Validators": [
                            {"Type": "not_empty"}
                          ],
                        },
                        {
                          "Key": "Query",
                          "Label": "i18n:query_hotkeys_query",
                          "Tooltip": "i18n:query_hotkeys_query_tooltip",
                          "Type": "text",
                          "TextMaxLines": 1,
                          "Validators": [
                            {"Type": "not_empty"}
                          ],
                        },
                        {
                          "Key": "IsSilentExecution",
                          "Label": "i18n:query_hotkeys_silent",
                          "Tooltip": "i18n:query_hotkeys_silent_tooltip",
                          "Width": 60,
                          "Type": "checkbox",
                        }
                      ],
                      "SortColumnKey": "Query"
                    }),
                    onUpdate: (key, value) {
                      controller.updateConfig("QueryHotkeys", value);
                    },
                  );
                }),
              ),
              formField(
                label: controller.tr("query_shortcuts"),
                child: Obx(() {
                  return WoxSettingPluginTable(
                    value: json.encode(controller.woxSetting.value.queryShortcuts),
                    item: PluginSettingValueTable.fromJson({
                      "Key": "QueryShortcuts",
                      "Columns": [
                        {
                          "Key": "Shortcut",
                          "Label": "i18n:query_shortcuts_shortcut",
                          "Tooltip": "i18n:query_shortcuts_shortcut_tooltip",
                          "Width": 120,
                          "Type": "text",
                          "TextMaxLines": 1,
                          "Validators": [
                            {"Type": "not_empty"}
                          ],
                        },
                        {
                          "Key": "Query",
                          "Label": "i18n:query_shortcuts_query",
                          "Tooltip": "i18n:query_shortcuts_query_tooltip",
                          "Type": "text",
                          "TextMaxLines": 1,
                          "Validators": [
                            {"Type": "not_empty"}
                          ],
                        }
                      ],
                      "SortColumnKey": "Query"
                    }),
                    onUpdate: (key, value) {
                      controller.updateConfig("QueryShortcuts", value);
                    },
                  );
                }),
              ),
            ]));
      }),
    );
  }
}
