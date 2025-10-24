import 'dart:convert';

import 'package:dynamic_tabbar/dynamic_tabbar.dart' as dt;
import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart' as material;
import 'package:flutter/services.dart';
import 'package:flutter_image_slideshow/flutter_image_slideshow.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_ai_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_newline.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/components/plugin/wox_setting_plugin_checkbox_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_textbox_view.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/strings.dart';
import 'package:wox/enums/wox_plugin_runtime_enum.dart';

class WoxSettingPluginView extends GetView<WoxSettingController> {
  const WoxSettingPluginView({super.key});
  // Local refreshing state for showing loading spinner on refresh button
  static final RxBool _refreshing = false.obs;

  Widget pluginList() {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 20),
          child: Obx(() {
            return TextBox(
              autofocus: true,
              controller: controller.filterPluginKeywordController,
              placeholder: Strings.format(controller.tr('ui_search_plugins'), [controller.filteredPluginList.length]),
              padding: const EdgeInsets.all(10),
              suffix: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Obx(() {
                    if (_refreshing.value) {
                      return const Padding(
                        padding: EdgeInsets.symmetric(horizontal: 4.0),
                        child: SizedBox(width: 16, height: 16, child: ProgressRing()),
                      );
                    }
                    return GestureDetector(
                      onTap: () async {
                        _refreshing.value = true;
                        try {
                          final traceId = const UuidV4().generate();
                          final preserveKeyword = controller.filterPluginKeywordController.text;
                          final preserveActiveId = controller.activePlugin.value.id;
                          final isStore = controller.isStorePluginList.value;

                          if (isStore) {
                            await controller.loadStorePlugins(traceId);
                            await controller.switchToPluginList(traceId, true);
                          } else {
                            await controller.loadInstalledPlugins(traceId);
                            await controller.switchToPluginList(traceId, false);
                          }

                          // restore filter keyword and re-filter
                          controller.filterPluginKeywordController.text = preserveKeyword;
                          controller.filterPlugins();

                          // try restore previous active selection if still present
                          final idx = controller.filteredPluginList.indexWhere((p) => p.id == preserveActiveId);
                          if (idx >= 0) {
                            controller.activePlugin.value = controller.filteredPluginList[idx];
                          } else {
                            controller.setFirstFilteredPluginDetailActive();
                          }
                        } finally {
                          _refreshing.value = false;
                        }
                      },
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 4.0),
                        child: Icon(FluentIcons.refresh, color: getThemeSubTextColor()),
                      ),
                    );
                  }),
                ],
              ),
              onChanged: (value) {
                controller.filterPlugins();
                controller.setFirstFilteredPluginDetailActive();
              },
            );
          }),
        ),
        Expanded(
          child: Scrollbar(
            thumbVisibility: false,
            child: Obx(() {
              if (controller.filteredPluginList.isEmpty) {
                return Center(
                  child: Text(
                    controller.tr('ui_setting_plugin_empty_data'),
                    style: TextStyle(
                      color: getThemeSubTextColor(),
                    ),
                  ),
                );
              }

              return ListView.builder(
                primary: true,
                itemCount: controller.filteredPluginList.length,
                itemBuilder: (context, index) {
                  final plugin = controller.filteredPluginList[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8.0),
                    child: Obx(() {
                      final isActive = controller.activePlugin.value.id == plugin.id;
                      return Container(
                        decoration: BoxDecoration(
                          color: isActive ? getThemeActiveBackgroundColor() : Colors.transparent,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: GestureDetector(
                          behavior: HitTestBehavior.translucent,
                          onTap: () {
                            controller.activePlugin.value = plugin;
                          },
                          child: material.ListTile(
                            contentPadding: const EdgeInsets.only(left: 6, right: 0),
                            leading: WoxImageView(woxImage: plugin.icon, width: 32),
                            title: Text(plugin.name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(
                                  fontSize: 15,
                                  color: isActive ? getThemeActionItemActiveColor() : getThemeTextColor(),
                                )),
                            subtitle: Row(
                              mainAxisAlignment: MainAxisAlignment.start,
                              crossAxisAlignment: CrossAxisAlignment.center,
                              children: [
                                Text(
                                  plugin.version,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(),
                                    fontSize: 12,
                                  ),
                                ),
                                const SizedBox(width: 10),
                                Flexible(
                                  child: Text(
                                    plugin.author,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: TextStyle(
                                      color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(),
                                      fontSize: 12,
                                    ),
                                  ),
                                ),
                              ],
                            ),
                            trailing: pluginTrailIcon(plugin, isActive),
                          ),
                        ),
                      );
                    }),
                  );
                },
              );
            }),
          ),
        ),
      ],
    );
  }

  Widget pluginTrailIcon(PluginDetail plugin, bool isActive) {
    // align tags/icons to the right of the tile
    final Color borderColor = isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor();

    List<Widget> rightItems = [];

    // Script tag (non-system script plugins)
    if (!plugin.isSystem && WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.SCRIPT)) {
      rightItems.add(Container(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3),
          border: Border.all(color: borderColor, width: 0.5),
        ),
        child: Text(
          controller.tr('ui_setting_plugin_script_tag'),
          style: TextStyle(color: borderColor, fontSize: 11, height: 1.1),
        ),
      ));
    }

    // System tag
    if (plugin.isSystem) {
      rightItems.add(Container(
        margin: const EdgeInsets.only(left: 8),
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3),
          border: Border.all(color: borderColor, width: 0.5),
        ),
        child: Text(
          controller.tr('ui_setting_plugin_system_tag'),
          style: TextStyle(color: borderColor, fontSize: 11, height: 1.1),
        ),
      ));
    }

    // Store list: show installed check icon
    if (controller.isStorePluginList.value && plugin.isInstalled) {
      rightItems.add(Padding(
        padding: const EdgeInsets.only(left: 8.0),
        child: Icon(FluentIcons.skype_circle_check, color: isActive ? getThemeActionItemActiveColor() : Colors.green),
      ));
    }

    if (rightItems.isEmpty) {
      return const SizedBox();
    }

    return Row(mainAxisSize: MainAxisSize.min, children: rightItems);
  }

  Widget pluginDetail() {
    return Expanded(
      child: Obx(() {
        if (controller.activePlugin.value.id.isEmpty) {
          return Center(
            child: Text(
              controller.tr('ui_setting_plugin_empty_data'),
              style: TextStyle(
                color: getThemeSubTextColor(),
              ),
            ),
          );
        }

        final plugin = controller.activePlugin.value;
        return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Padding(
            padding: const EdgeInsets.only(bottom: 8.0, left: 10),
            child: Row(
              children: [
                WoxImageView(woxImage: plugin.icon, width: 32),
                Padding(
                  padding: const EdgeInsets.only(left: 8.0),
                  child: Text(
                    plugin.name,
                    style: TextStyle(
                      fontSize: 20,
                      color: getThemeTextColor(),
                    ),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 10.0),
                  child: Text(
                    plugin.version,
                    style: TextStyle(
                      color: getThemeSubTextColor(),
                    ),
                  ),
                ),
                if (plugin.isDev)
                  // dev tag, warning color with warning border
                  Padding(
                    padding: const EdgeInsets.only(left: 10.0),
                    child: Container(
                      padding: const EdgeInsets.all(4),
                      decoration: BoxDecoration(
                        color: getThemeSubTextColor(),
                        border: Border.all(color: getThemeSubTextColor()),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(
                        controller.tr('ui_plugin_dev_tag'),
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 12,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.only(bottom: 8.0, left: 16),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  plugin.author,
                  style: TextStyle(
                    color: getThemeSubTextColor(),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 18.0),
                  child: HyperlinkButton(
                    onPressed: () {
                      controller.openPluginWebsite(plugin.website);
                    },
                    child: Row(
                      children: [
                        Text(
                          controller.tr('ui_plugin_website'),
                          style: TextStyle(
                            color: getThemeTextColor(),
                          ),
                        ),
                        Padding(
                          padding: const EdgeInsets.only(left: 4.0),
                          child: Icon(
                            FluentIcons.open_in_new_tab,
                            size: 12,
                            color: getThemeTextColor(),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.only(bottom: 8.0, left: 16),
            child: Row(
              children: [
                if (plugin.isInstalled && !plugin.isSystem)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      style: ButtonStyle(
                        foregroundColor: WidgetStateProperty.all(getThemeTextColor()),
                      ),
                      onPressed: () {
                        controller.uninstallPlugin(plugin);
                      },
                      child: Text(controller.tr('ui_plugin_uninstall')),
                    ),
                  ),
                if (!plugin.isInstalled)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Obx(() => Button(
                          style: ButtonStyle(
                            foregroundColor: WidgetStateProperty.all(getThemeTextColor()),
                          ),
                          onPressed: controller.isInstallingPlugin.value
                              ? null
                              : () {
                                  controller.installPlugin(plugin);
                                },
                          child: controller.isInstallingPlugin.value
                              ? Row(
                                  children: [
                                    const SizedBox(
                                      width: 16,
                                      height: 16,
                                      child: ProgressRing(),
                                    ),
                                    const SizedBox(width: 8),
                                    Text(controller.tr("ui_plugin_installing")),
                                  ],
                                )
                              : Text(controller.tr('ui_plugin_install')),
                        )),
                  ),
                if (plugin.isInstalled && !plugin.isDisable)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      style: ButtonStyle(
                        foregroundColor: WidgetStateProperty.all(getThemeTextColor()),
                      ),
                      onPressed: () {
                        controller.disablePlugin(plugin);
                      },
                      child: Text(controller.tr('ui_plugin_disable')),
                    ),
                  ),
                if (plugin.isInstalled && plugin.isDisable)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      style: ButtonStyle(
                        foregroundColor: WidgetStateProperty.all(getThemeTextColor()),
                      ),
                      onPressed: () {
                        controller.enablePlugin(plugin);
                      },
                      child: Text(controller.tr('ui_plugin_enable')),
                    ),
                  ),
              ],
            ),
          ),
          Expanded(
            child: dt.DynamicTabBarWidget(
              isScrollable: true,
              showBackIcon: false,
              showNextIcon: false,
              physics: const NeverScrollableScrollPhysics(),
              physicsTabBarView: const NeverScrollableScrollPhysics(),
              tabAlignment: material.TabAlignment.start,
              onAddTabMoveTo: dt.MoveToTab.idol,
              labelColor: getThemeTextColor(),
              unselectedLabelColor: getThemeTextColor(),
              indicatorColor: getThemeActiveBackgroundColor(),
              dynamicTabs: [
                if (controller.shouldShowSettingTab())
                  dt.TabData(
                    index: 0,
                    title: material.Tab(
                      child: Text(controller.tr('ui_plugin_tab_settings')),
                    ),
                    content: pluginTabSetting(),
                  ),
                dt.TabData(
                  index: 1,
                  title: material.Tab(
                    child: Text(controller.tr('ui_plugin_tab_trigger_keywords')),
                  ),
                  content: pluginTabTriggerKeywords(),
                ),
                dt.TabData(
                  index: 2,
                  title: material.Tab(
                    child: Text(controller.tr('ui_plugin_tab_commands')),
                  ),
                  content: pluginTabCommand(),
                ),
                dt.TabData(
                  index: 3,
                  title: material.Tab(
                    child: Text(controller.tr('ui_plugin_tab_description')),
                  ),
                  content: pluginTabDescription(),
                ),
                dt.TabData(
                  index: 4,
                  title: material.Tab(
                    child: Text(controller.tr('ui_plugin_tab_privacy')),
                  ),
                  content: pluginTabPrivacy(),
                ),
              ],
              onTabControllerUpdated: (tabController) {
                controller.activePluginTabController = tabController;
                tabController.index = 0;
              },
              onTabChanged: (index) {},
            ),
          ),
        ]);
      }),
    );
  }

  Widget pluginTabDescription() {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            controller.activePlugin.value.description,
          ),
          const SizedBox(height: 20),
          controller.activePlugin.value.screenshotUrls.isNotEmpty
              ? ImageSlideshow(
                  width: double.infinity,
                  height: 400,
                  indicatorColor: getThemeActiveBackgroundColor(),
                  children: [
                    ...controller.activePlugin.value.screenshotUrls.map((e) => Image.network(e)),
                  ],
                )
              : const SizedBox(),
        ],
      ),
    );
  }

  Widget pluginTabSetting() {
    return Obx(() {
      var plugin = controller.activePlugin.value;
      return Padding(
        padding: const EdgeInsets.all(16.0),
        child: SingleChildScrollView(
          child: Wrap(
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              ...plugin.settingDefinitions.map(
                (e) {
                  if (e.type == "checkbox") {
                    return WoxSettingPluginCheckbox(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueCheckBox,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }
                  if (e.type == "textbox") {
                    return WoxSettingPluginTextBox(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueTextBox,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }
                  if (e.type == "newline") {
                    return WoxSettingPluginNewLine(
                      value: "",
                      item: e.value as PluginSettingValueNewLine,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }
                  if (e.type == "select") {
                    return WoxSettingPluginSelect(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueSelect,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }
                  if (e.type == "selectAIModel") {
                    return WoxSettingPluginSelectAIModel(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueSelectAIModel,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }
                  if (e.type == "head") {
                    return WoxSettingPluginHead(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueHead,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }
                  if (e.type == "label") {
                    return WoxSettingPluginLabel(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueLabel,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }
                  if (e.type == "table") {
                    return WoxSettingPluginTable(
                      tableWidth: 640,
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueTable,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                  }

                  return Text(e.type);
                },
              )
            ],
          ),
        ),
      );
    });
  }

  Widget pluginTabTriggerKeywords() {
    var plugin = controller.activePlugin.value;
    if (plugin.triggerKeywords.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(controller.tr('ui_plugin_no_trigger_keywords'), style: TextStyle(color: getThemeTextColor())),
          ],
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: WoxSettingPluginTable(
        value: json.encode(plugin.triggerKeywords.map((e) => {"keyword": e}).toList()),
        tableWidth: 640,
        item: PluginSettingValueTable.fromJson({
          "Key": "_triggerKeywords",
          "Columns": [
            {
              "Key": "keyword",
              "Label": controller.tr('ui_plugin_trigger_keyword_column'),
              "Tooltip": controller.tr('ui_plugin_trigger_keyword_tooltip'),
              "Type": "text",
              "TextMaxLines": 1,
              "Validators": [
                {"Type": "not_empty"}
              ],
            },
          ],
          "SortColumnKey": "keyword"
        }),
        onUpdate: (key, value) async {
          final List<String> triggerKeywords = [];
          for (var item in json.decode(value)) {
            triggerKeywords.add(item["keyword"]);
          }
          plugin.triggerKeywords = triggerKeywords;
          await controller.updatePluginSetting(plugin.id, "TriggerKeywords", triggerKeywords.join(","));
        },
      ),
    );
  }

  Widget pluginTabCommand() {
    var plugin = controller.activePlugin.value;
    if (plugin.commands.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(controller.tr('ui_plugin_no_commands'), style: TextStyle(color: getThemeTextColor())),
          ],
        ),
      );
    }

    return WoxSettingPluginTable(
      value: json.encode(plugin.commands),
      tableWidth: 680,
      readonly: true,
      item: PluginSettingValueTable.fromJson({
        "Key": "_commands",
        "Columns": [
          {
            "Key": "Command",
            "Label": controller.tr('ui_plugin_command_name_column'),
            "Width": 120,
            "Type": "text",
            "TextMaxLines": 1,
            "Validators": [
              {"Type": "not_empty"}
            ],
          },
          {
            "Key": "Description",
            "Label": controller.tr('ui_plugin_command_desc_column'),
            "Type": "text",
            "TextMaxLines": 1,
            "Validators": [
              {"Type": "not_empty"}
            ],
          }
        ],
        "SortColumnKey": "Command"
      }),
      onUpdate: (key, value) {},
    );
  }

  Widget pluginTabPrivacy() {
    var plugin = controller.activePlugin.value;
    var noDataAccess = Padding(
      padding: const EdgeInsets.all(16),
      child: Text(controller.tr('ui_plugin_no_data_access'), style: TextStyle(color: getThemeTextColor())),
    );

    if (plugin.features.isEmpty) {
      return noDataAccess;
    }

    List<String> params = [];

    //check if "queryEnv" feature is exist and list it's params
    var queryEnv = plugin.features.where((element) => element.name == "queryEnv").toList();
    if (queryEnv.isNotEmpty) {
      queryEnv.first.params.forEach((key, value) {
        if (value == "true") {
          params.add(key);
        }
      });
    }

    // check if llmChat feature is exist
    var llmChat = plugin.features.where((element) => element.name == "llm").toList();
    if (llmChat.isNotEmpty) {
      params.add("llm");
    }

    if (params.isEmpty) {
      return noDataAccess;
    }

    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(controller.tr('ui_plugin_data_access_title'), style: TextStyle(color: getThemeTextColor())),
            ...params.map((e) {
              if (e == "requireActiveWindowName") {
                return privacyItem(
                  material.Icons.window,
                  controller.tr('ui_plugin_privacy_window_name'),
                  controller.tr('ui_plugin_privacy_window_name_desc'),
                );
              }
              if (e == "requireActiveWindowPid") {
                return privacyItem(
                  material.Icons.window,
                  controller.tr('ui_plugin_privacy_window_pid'),
                  controller.tr('ui_plugin_privacy_window_pid_desc'),
                );
              }
              if (e == "requireActiveBrowserUrl") {
                return privacyItem(
                  material.Icons.web_sharp,
                  controller.tr('ui_plugin_privacy_browser_url'),
                  controller.tr('ui_plugin_privacy_browser_url_desc'),
                );
              }
              if (e == "llm") {
                return privacyItem(
                  material.Icons.chat,
                  controller.tr('ui_plugin_privacy_llm'),
                  controller.tr('ui_plugin_privacy_llm_desc'),
                );
              }
              return Text(e);
            }),
          ],
        ),
      ),
    );
  }

  Widget privacyItem(IconData icon, String title, String description) {
    return Padding(
      padding: const EdgeInsets.only(top: 20.0),
      child: Column(
        children: [
          Row(
            children: [
              Icon(icon, color: getThemeTextColor()),
              const SizedBox(width: 10),
              Text(title, style: TextStyle(color: getThemeTextColor())),
            ],
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              const SizedBox(width: 30),
              Flexible(
                child: Text(
                  description,
                  style: TextStyle(
                    color: getThemeSubTextColor(),
                  ),
                ),
              ),
            ],
          )
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 260,
            child: pluginList(),
          ),
          Container(
            width: 1,
            height: double.infinity,
            color: getThemeDividerColor(),
            margin: const EdgeInsets.only(right: 10, left: 10),
          ),
          pluginDetail(),
        ],
      ),
    );
  }
}
