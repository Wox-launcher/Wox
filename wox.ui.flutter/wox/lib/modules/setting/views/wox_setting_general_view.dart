import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/enums/wox_launch_mode_enum.dart';
import 'package:wox/enums/wox_start_page_enum.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';

class WoxSettingGeneralView extends WoxSettingBaseView {
  const WoxSettingGeneralView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return form(children: [
        formField(
          label: controller.tr("ui_autostart"),
          tips: controller.tr("ui_autostart_tips"),
          child: WoxSwitch(
            value: controller.woxSetting.value.enableAutostart,
            onChanged: (bool value) {
              controller.updateConfig("EnableAutostart", value.toString());
            },
          ),
        ),
        formField(
          label: controller.tr("ui_hotkey"),
          tips: controller.tr("ui_hotkey_tips"),
          child: WoxHotkeyRecorder(
            hotkey: WoxHotkey.parseHotkeyFromString(controller.woxSetting.value.mainHotkey),
            onHotKeyRecorded: (hotkey) {
              controller.updateConfig("MainHotkey", hotkey);
            },
          ),
        ),
        formField(
          label: controller.tr("ui_selection_hotkey"),
          tips: controller.tr("ui_selection_hotkey_tips"),
          child: WoxHotkeyRecorder(
            hotkey: WoxHotkey.parseHotkeyFromString(controller.woxSetting.value.selectionHotkey),
            onHotKeyRecorded: (hotkey) {
              controller.updateConfig("SelectionHotkey", hotkey);
            },
          ),
        ),
        formField(
          label: controller.tr("ui_use_pinyin"),
          tips: controller.tr("ui_use_pinyin_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.usePinYin,
              onChanged: (bool value) {
                controller.updateConfig("UsePinYin", value.toString());
              },
            );
          }),
        ),
        formField(
          label: controller.tr("ui_hide_on_lost_focus"),
          tips: controller.tr("ui_hide_on_lost_focus_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.hideOnLostFocus,
              onChanged: (bool value) {
                controller.updateConfig("HideOnLostFocus", value.toString());
              },
            );
          }),
        ),
        formField(
          label: controller.tr("ui_hide_on_start"),
          tips: controller.tr("ui_hide_on_start_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.hideOnStart,
              onChanged: (bool value) {
                controller.updateConfig("HideOnStart", value.toString());
              },
            );
          }),
        ),
        formField(
          label: controller.tr("ui_switch_input_method_abc"),
          tips: controller.tr("ui_switch_input_method_abc_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.switchInputMethodABC,
              onChanged: (bool value) {
                controller.updateConfig("SwitchInputMethodABC", value.toString());
              },
            );
          }),
        ),
        formField(
          label: controller.tr("ui_enable_auto_update"),
          tips: controller.tr("ui_enable_auto_update_tips"),
          child: Obx(() {
            return WoxSwitch(
              value: controller.woxSetting.value.enableAutoUpdate,
              onChanged: (bool value) {
                controller.updateConfig("EnableAutoUpdate", value.toString());

                // add some delay for backend to update the version check
                Future.delayed(const Duration(seconds: 2), () {
                  Get.find<WoxLauncherController>().doctorCheck();
                });
              },
            );
          }),
        ),
        formField(
          label: controller.tr("ui_launch_mode"),
          tips: controller.tr("ui_launch_mode_tips"),
          child: Obx(() {
            return WoxDropdownButton<String>(
              items: [
                WoxDropdownItem(
                  value: WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code,
                  label: controller.tr("ui_launch_mode_fresh"),
                  tooltip: controller.tr("ui_launch_mode_fresh_tips"),
                ),
                WoxDropdownItem(
                  value: WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code,
                  label: controller.tr("ui_launch_mode_continue"),
                  tooltip: controller.tr("ui_launch_mode_continue_tips"),
                ),
              ],
              value: controller.woxSetting.value.launchMode,
              onChanged: (v) {
                if (v != null) {
                  controller.updateConfig("LaunchMode", v);
                }
              },
              isExpanded: true,
            );
          }),
        ),
        formField(
          label: controller.tr("ui_start_page"),
          tips: controller.tr("ui_start_page_tips"),
          child: Obx(() {
            return WoxDropdownButton<String>(
              items: [
                WoxDropdownItem(
                  value: WoxStartPageEnum.WOX_START_PAGE_BLANK.code,
                  label: controller.tr("ui_start_page_blank"),
                  tooltip: controller.tr("ui_start_page_blank_tips"),
                ),
                WoxDropdownItem(
                  value: WoxStartPageEnum.WOX_START_PAGE_MRU.code,
                  label: controller.tr("ui_start_page_mru"),
                  tooltip: controller.tr("ui_start_page_mru_tips"),
                ),
              ],
              value: controller.woxSetting.value.startPage,
              onChanged: (v) {
                if (v != null) {
                  controller.updateConfig("StartPage", v);
                }
              },
              isExpanded: true,
            );
          }),
        ),
        formField(
          label: controller.tr("ui_lang"),
          child: FutureBuilder(
              future: WoxApi.instance.getAllLanguages(),
              builder: (context, snapshot) {
                if (snapshot.connectionState == ConnectionState.done) {
                  final languages = snapshot.data as List<WoxLang>;
                  return Obx(() {
                    return WoxDropdownButton<String>(
                      items: languages.map((e) {
                        return WoxDropdownItem(
                          value: e.code,
                          label: e.name,
                        );
                      }).toList(),
                      value: controller.woxSetting.value.langCode,
                      onChanged: (v) {
                        if (v != null) {
                          controller.updateLang(v);
                        }
                      },
                      isExpanded: true,
                    );
                  });
                }
                return const SizedBox();
              }),
        ),
        formField(
          label: controller.tr("ui_query_hotkeys"),
          tips: controller.tr("ui_query_hotkeys_tips"),
          child: Obx(() {
            return WoxSettingPluginTable(
              value: json.encode(controller.woxSetting.value.queryHotkeys),
              item: PluginSettingValueTable.fromJson({
                "Key": "QueryHotkeys",
                "Columns": [
                  {
                    "Key": "Hotkey",
                    "Label": "i18n:ui_query_hotkeys_hotkey",
                    "Tooltip": "i18n:ui_query_hotkeys_hotkey_tooltip",
                    "Width": 120,
                    "Type": "hotkey",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"}
                    ],
                  },
                  {
                    "Key": "Query",
                    "Label": "i18n:ui_query_hotkeys_query",
                    "Tooltip": "i18n:ui_query_hotkeys_query_tooltip",
                    "Type": "text",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"}
                    ],
                  },
                  {
                    "Key": "IsSilentExecution",
                    "Label": "i18n:ui_query_hotkeys_silent",
                    "Tooltip": "i18n:ui_query_hotkeys_silent_tooltip",
                    "Width": 54,
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
          label: controller.tr("ui_query_shortcuts"),
          tips: controller.tr("ui_query_shortcuts_tips"),
          child: Obx(() {
            return WoxSettingPluginTable(
              value: json.encode(controller.woxSetting.value.queryShortcuts),
              item: PluginSettingValueTable.fromJson({
                "Key": "QueryShortcuts",
                "Columns": [
                  {
                    "Key": "Shortcut",
                    "Label": "i18n:ui_query_shortcuts_shortcut",
                    "Tooltip": "i18n:ui_query_shortcuts_shortcut_tooltip",
                    "Width": 120,
                    "Type": "text",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"}
                    ],
                  },
                  {
                    "Key": "Query",
                    "Label": "i18n:ui_query_shortcuts_query",
                    "Tooltip": "i18n:ui_query_shortcuts_query_tooltip",
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
      ]);
    });
  }
}
