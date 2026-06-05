import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/components/wox_query_hotkey_dialog.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/enums/wox_launch_mode_enum.dart';
import 'package:wox/enums/wox_start_page_enum.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';

class WoxSettingGeneralView extends WoxSettingBaseView {
  const WoxSettingGeneralView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        controller.notifyGeneralViewReady();
      });

      return form(
        title: controller.tr("ui_general"),
        description: controller.tr("ui_general_description"),
        children: [
          formSection(
            title: controller.tr("ui_general_section_startup"),
            children: [
              formField(
                settingKey: "EnableAutostart",
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
                settingKey: "HideOnStart",
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
                settingKey: "EnableAutoUpdate",
                label: controller.tr("ui_enable_auto_update"),
                labelWidth: GENERAL_SETTING_WIDE_LABEL_WIDTH,
                tips: controller.tr("ui_enable_auto_update_tips"),
                child: Obx(() {
                  return WoxSwitch(
                    value: controller.woxSetting.value.enableAutoUpdate,
                    onChanged: (bool value) {
                      controller.updateConfig("EnableAutoUpdate", value.toString());

                      // The backend refreshes update metadata asynchronously, so keep the delayed doctor refresh after changing this setting.
                      Future.delayed(const Duration(seconds: 2), () {
                        Get.find<WoxLauncherController>().doctorCheck();
                      });
                    },
                  );
                }),
              ),
              formField(
                settingKey: "ReleaseChannel",
                label: controller.tr("ui_release_channel"),
                labelWidth: GENERAL_SETTING_WIDE_LABEL_WIDTH,
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

                        // The backend clears cached update state when the channel changes, then checks metadata asynchronously.
                        Future.delayed(const Duration(seconds: 2), () {
                          Get.find<WoxLauncherController>().doctorCheck();
                        });
                      }
                    },
                    isExpanded: true,
                  );
                }),
              ),
            ],
          ),
          formSection(
            title: controller.tr("ui_general_section_launch"),
            children: [
              formField(
                settingKey: "MainHotkey",
                label: controller.tr("ui_hotkey"),
                tips: controller.tr("ui_hotkey_tips"),
                controlMaxWidth: 520,
                child: WoxHotkeyRecorder(
                  hotkey: WoxHotkey.parseHotkeyFromString(controller.woxSetting.value.mainHotkey),
                  onHotKeyRecorded: (hotkey) {
                    controller.updateConfig("MainHotkey", hotkey);
                  },
                ),
              ),
              formField(
                settingKey: "SelectionHotkey",
                label: controller.tr("ui_selection_hotkey"),
                tips: controller.tr("ui_selection_hotkey_tips"),
                controlMaxWidth: 520,
                child: WoxHotkeyRecorder(
                  hotkey: WoxHotkey.parseHotkeyFromString(controller.woxSetting.value.selectionHotkey),
                  onHotKeyRecorded: (hotkey) {
                    controller.updateConfig("SelectionHotkey", hotkey);
                  },
                ),
              ),
              formField(
                settingKey: "LaunchMode",
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
                settingKey: "StartPage",
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
                      WoxDropdownItem(value: WoxStartPageEnum.WOX_START_PAGE_MRU.code, label: controller.tr("ui_start_page_mru"), tooltip: controller.tr("ui_start_page_mru_tips")),
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
                settingKey: "HideOnLostFocus",
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
                settingKey: "UsePinYin",
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
                settingKey: "SwitchInputMethodABC",
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
            ],
          ),
          formSection(
            title: controller.tr("ui_general_section_language"),
            children: [
              formField(
                settingKey: "LangCode",
                label: controller.tr("ui_lang"),
                child: FutureBuilder(
                  future: WoxApi.instance.getAllLanguages(const UuidV4().generate()),
                  builder: (context, snapshot) {
                    if (snapshot.connectionState == ConnectionState.done) {
                      final languages = snapshot.data as List<WoxLang>;
                      return Obx(() {
                        return WoxDropdownButton<String>(
                          items:
                              languages.map((e) {
                                return WoxDropdownItem(value: e.code, label: e.name);
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
                  },
                ),
              ),
            ],
          ),
          formSection(
            title: controller.tr("ui_general_section_hotkeys"),
            children: [
              settingTarget(
                settingKey: "IgnoredHotkeyApps",
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 24),
                  child: Obx(() {
                    final rows = controller.woxSetting.value.ignoredHotkeyApps.map((app) => <String, dynamic>{"App": app.toJson()}).toList();

                    return WoxSettingPluginTable(
                      inlineTitleActions: true,
                      tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                      value: json.encode(rows),
                      item: PluginSettingValueTable.fromJson({
                        "Key": "IgnoredHotkeyAppsTable",
                        "Title": "i18n:ui_hotkey_ignore_apps",
                        "Tooltip": "i18n:ui_hotkey_ignore_apps_tips",
                        "MaxHeight": 220,
                        "Columns": [
                          {
                            "Key": "App",
                            "Label": "i18n:ui_hotkey_ignore_apps_app",
                            "Tooltip": "i18n:ui_hotkey_ignore_apps_tips",
                            "Type": "app",
                            "Width": 420,
                            "Validators": [
                              {"Type": "not_empty"},
                            ],
                          },
                        ],
                        "SortColumnKey": "",
                      }),
                      onUpdate: (key, value) async {
                        final decodedRows = json.decode(value) as List<dynamic>;
                        final apps = decodedRows.map((row) => row is Map<String, dynamic> ? row["App"] : null).whereType<Map<String, dynamic>>().toList();

                        await controller.updateConfig("IgnoredHotkeyApps", json.encode(apps));
                        return null;
                      },
                    );
                  }),
                ),
              ),
              settingTarget(
                settingKey: "QueryHotkeys",
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 24),
                  child: Obx(() {
                    return WoxSettingPluginTable(
                      inlineTitleActions: true,
                      tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                      customCreateDialogBuilder: (context, saveRow) => showWoxQueryHotkeyDialog(context: context, onSave: saveRow),
                      customEditDialogBuilder: (context, row, saveRow) => showWoxQueryHotkeyDialog(context: context, initialRow: row, onSave: saveRow),
                      titleActions: [
                        _buildDemoTitleAction(
                          triggerKey: 'settings-query-hotkeys-demo-trigger',
                          popoverKey: 'wox-demo-popover-queryHotkeys',
                          demo: WoxQueryHotkeysDemo(accent: const Color(0xFFF43F5E), tr: controller.tr),
                        ),
                      ],
                      value: json.encode(controller.woxSetting.value.queryHotkeys),
                      item: PluginSettingValueTable.fromJson({
                        "Key": "QueryHotkeys",
                        "Title": "i18n:ui_query_hotkeys",
                        "Tooltip": "i18n:ui_query_hotkeys_tips",
                        "UpdateDialogWidth": 760,
                        "Columns": [
                          {"Key": "Name", "Label": "i18n:ui_query_hotkeys_name", "Tooltip": "i18n:ui_query_hotkeys_name_tooltip", "Type": "text", "Width": 140, "TextMaxLines": 1},
                          {
                            "Key": "Hotkey",
                            "Label": "i18n:ui_query_hotkeys_hotkey",
                            "Tooltip": "i18n:ui_query_hotkeys_hotkey_tooltip",
                            "Width": 120,
                            "Type": "hotkey",
                            "TextMaxLines": 1,
                            "Validators": [
                              {"Type": "not_empty"},
                            ],
                          },
                          {
                            "Key": "Query",
                            "Label": "i18n:ui_query_hotkeys_query",
                            "Tooltip": "i18n:ui_query_hotkeys_query_tooltip",
                            "Type": PluginSettingValueType.pluginSettingValueTableColumnTypeQueryHotkeyQuery,
                            "TextMaxLines": 1,
                            "Validators": [
                              {"Type": "not_empty"},
                            ],
                          },
                          {
                            "Key": "Position",
                            "Label": "i18n:ui_query_hotkeys_position",
                            "Tooltip": "i18n:ui_query_hotkeys_position_tooltip",
                            "Type": "select",
                            "Width": 120,
                            "HideInTable": true,
                            "SelectOptions": [
                              {"Label": controller.tr("ui_query_position_system_default"), "Value": "system_default"},
                              {"Label": controller.tr("ui_query_position_top_left"), "Value": "top_left"},
                              {"Label": controller.tr("ui_query_position_top_center"), "Value": "top_center"},
                              {"Label": controller.tr("ui_query_position_top_right"), "Value": "top_right"},
                              {"Label": controller.tr("ui_query_position_center"), "Value": "center"},
                              {"Label": controller.tr("ui_query_position_bottom_left"), "Value": "bottom_left"},
                              {"Label": controller.tr("ui_query_position_bottom_center"), "Value": "bottom_center"},
                              {"Label": controller.tr("ui_query_position_bottom_right"), "Value": "bottom_right"},
                            ],
                          },
                          {
                            "Key": "HideQueryBox",
                            "Label": "i18n:ui_query_hotkeys_hide_query_box",
                            "Tooltip": "i18n:ui_query_hotkeys_hide_query_box_tooltip",
                            "Width": 80,
                            "HideInTable": true,
                            "Type": "checkbox",
                          },
                          {
                            "Key": "HideToolbar",
                            "Label": "i18n:ui_query_hotkeys_hide_toolbar",
                            "Tooltip": "i18n:ui_query_hotkeys_hide_toolbar_tooltip",
                            "Width": 80,
                            "HideInTable": true,
                            "Type": "checkbox",
                          },
                          {
                            "Key": "Width",
                            "Label": "i18n:ui_query_hotkeys_width",
                            "Tooltip": "i18n:ui_query_hotkeys_width_tooltip",
                            "Type": "text",
                            "Width": 50,
                            "HideInTable": true,
                            "TextMaxLines": 1,
                          },
                          {
                            "Key": "MaxResultCount",
                            "Label": "i18n:ui_query_hotkeys_max_result_count",
                            "Tooltip": "i18n:ui_query_hotkeys_max_result_count_tooltip",
                            "Type": "text",
                            "Width": 90,
                            "HideInTable": true,
                            "TextMaxLines": 1,
                          },
                          {
                            "Key": "IsSilentExecution",
                            "Label": "i18n:ui_query_hotkeys_silent",
                            "Tooltip": "i18n:ui_query_hotkeys_silent_tooltip",
                            "Width": 40,
                            "HideInTable": true,
                            "Type": "checkbox",
                          },
                          {"Key": "Disabled", "Label": "i18n:ui_disabled", "Tooltip": "i18n:ui_disabled_tooltip", "Width": 60, "Type": "checkbox"},
                        ],
                        "SortColumnKey": "Query",
                      }),
                      onUpdate: (key, value) async {
                        await controller.updateConfig("QueryHotkeys", value);
                        return null;
                      },
                    );
                  }),
                ),
              ),
              settingTarget(
                settingKey: "QueryShortcuts",
                child: Padding(
                  padding: const EdgeInsets.only(bottom: 24),
                  child: Obx(() {
                    return WoxSettingPluginTable(
                      inlineTitleActions: true,
                      tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                      titleActions: [
                        _buildDemoTitleAction(
                          triggerKey: 'settings-query-shortcuts-demo-trigger',
                          popoverKey: 'wox-demo-popover-queryShortcuts',
                          demo: WoxQueryShortcutsDemo(accent: const Color(0xFFA78BFA), tr: controller.tr),
                        ),
                      ],
                      value: json.encode(controller.woxSetting.value.queryShortcuts),
                      item: PluginSettingValueTable.fromJson({
                        "Key": "QueryShortcuts",
                        "Title": "i18n:ui_query_shortcuts",
                        "Tooltip": "i18n:ui_query_shortcuts_tips",
                        "Columns": [
                          {
                            "Key": "Shortcut",
                            "Label": "i18n:ui_query_shortcuts_shortcut",
                            "Tooltip": "i18n:ui_query_shortcuts_shortcut_tooltip",
                            "Width": 120,
                            "Type": "text",
                            "TextMaxLines": 1,
                            "Validators": [
                              {"Type": "not_empty"},
                            ],
                          },
                          {
                            "Key": "Query",
                            "Label": "i18n:ui_query_shortcuts_query",
                            "Tooltip": "i18n:ui_query_shortcuts_query_tooltip",
                            "Type": "text",
                            "TextMaxLines": 1,
                            "Validators": [
                              {"Type": "not_empty"},
                            ],
                          },
                          {"Key": "Disabled", "Label": "i18n:ui_disabled", "Tooltip": "i18n:ui_disabled_tooltip", "Width": 60, "Type": "checkbox"},
                        ],
                        "SortColumnKey": "Query",
                      }),
                      onUpdate: (key, value) async {
                        await controller.updateConfig("QueryShortcuts", value);
                        return null;
                      },
                    );
                  }),
                ),
              ),
              settingTarget(
                settingKey: "TrayQueries",
                child: Padding(
                  key: controller.getGeneralSectionKey('tray_queries'),
                  padding: const EdgeInsets.only(bottom: 24),
                  child: Obx(() {
                    return WoxSettingPluginTable(
                      inlineTitleActions: true,
                      tableWidth: GENERAL_SETTING_TABLE_WIDTH,
                      titleActions: [
                        _buildDemoTitleAction(
                          triggerKey: 'settings-tray-queries-demo-trigger',
                          popoverKey: 'wox-demo-popover-trayQueries',
                          demo: WoxTrayQueriesDemo(accent: const Color(0xFF22C55E), tr: controller.tr),
                        ),
                      ],
                      value: json.encode(controller.woxSetting.value.trayQueries),
                      autoOpenEditRowIndex: controller.pendingTrayQueryEditRowIndex.value,
                      item: PluginSettingValueTable.fromJson({
                        "Key": "TrayQueries",
                        "Title": "i18n:ui_tray_queries",
                        "Tooltip": "i18n:ui_tray_queries_tips",
                        "Columns": [
                          {"Key": "Icon", "Label": "i18n:ui_tray_queries_icon", "Tooltip": "i18n:ui_tray_queries_icon_tooltip", "Type": "woxImage", "Width": 40},
                          {
                            "Key": "Query",
                            "Label": "i18n:ui_tray_queries_query",
                            "Tooltip": "i18n:ui_tray_queries_query_tooltip",
                            "Type": "text",
                            "TextMaxLines": 1,
                            "Validators": [
                              {"Type": "not_empty"},
                            ],
                          },
                          {
                            "Key": "HideQueryBox",
                            "Label": "i18n:ui_tray_queries_hide_query_box",
                            "Tooltip": "i18n:ui_tray_queries_hide_query_box_tooltip",
                            "Width": 80,
                            "HideInTable": true,
                            "Type": "checkbox",
                          },
                          {
                            "Key": "HideToolbar",
                            "Label": "i18n:ui_tray_queries_hide_toolbar",
                            "Tooltip": "i18n:ui_tray_queries_hide_toolbar_tooltip",
                            "Width": 80,
                            "HideInTable": true,
                            "Type": "checkbox",
                          },
                          {
                            "Key": "Width",
                            "Label": "i18n:ui_tray_queries_width",
                            "Tooltip": "i18n:ui_tray_queries_width_tooltip",
                            "Type": "text",
                            "Width": 40,
                            "HideInTable": true,
                            "TextMaxLines": 1,
                          },
                          {
                            "Key": "MaxResultCount",
                            "Label": "i18n:ui_tray_queries_max_result_count",
                            "Tooltip": "i18n:ui_tray_queries_max_result_count_tooltip",
                            "Type": "text",
                            "Width": 90,
                            "HideInTable": true,
                            "TextMaxLines": 1,
                          },
                          {"Key": "Disabled", "Label": "i18n:ui_disabled", "Tooltip": "i18n:ui_disabled_tooltip", "Width": 50, "Type": "checkbox"},
                        ],
                        "SortColumnKey": "",
                      }),
                      onUpdate: (key, value) async {
                        await controller.updateConfig("TrayQueries", value);
                        return null;
                      },
                    );
                  }),
                ),
              ),
            ],
          ),
        ],
      );
    });
  }

  Widget _buildDemoTitleAction({required String triggerKey, required String popoverKey, required Widget demo}) {
    final foreground = getThemeTextColor();

    return WoxDemoPopover(
      key: ValueKey(triggerKey),
      popoverKey: ValueKey(popoverKey),
      demo: demo,
      width: 680,
      height: 460,
      child: Semantics(
        label: controller.tr("ui_demo_preview"),
        button: true,
        child: MouseRegion(
          cursor: SystemMouseCursors.help,
          child: SizedBox(
            width: 22,
            height: 22,
            // Feature refinement: the demo trigger now behaves like a title-side affordance, so it uses the same color as the title and avoids a separate text tooltip that would compete with the preview popover.
            child: Icon(Icons.play_circle_outline_rounded, color: foreground.withValues(alpha: 0.88), size: 18),
          ),
        ),
      ),
    );
  }

  Widget? _buildUpdateChannelVersion(String version) {
    if (version.isEmpty) {
      return null;
    }

    return Text(version, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12));
  }
}
