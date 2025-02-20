import 'dart:convert';

import 'package:dynamic_tabbar/dynamic_tabbar.dart' as dt;
import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart' as material;
import 'package:flutter/services.dart';
import 'package:flutter_image_slideshow/flutter_image_slideshow.dart';
import 'package:get/get.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_ai_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_image_view.dart';
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
import 'package:wox/modules/setting/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/strings.dart';

class WoxSettingPluginView extends GetView<WoxSettingController> {
  const WoxSettingPluginView({super.key});

  Widget pluginList() {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 20),
          child: Focus(
            autofocus: true,
            onKeyEvent: (FocusNode node, KeyEvent event) {
              if (event is KeyDownEvent) {
                switch (event.logicalKey) {
                  case LogicalKeyboardKey.escape:
                    controller.hideWindow();
                    return KeyEventResult.handled;
                }
              }

              return KeyEventResult.ignored;
            },
            child: Obx(() {
              return TextBox(
                autofocus: true,
                controller: controller.filterPluginKeywordController,
                placeholder: Strings.format(controller.tr('search_plugins'), [controller.filteredPluginDetails.length]),
                padding: const EdgeInsets.all(10),
                suffix: const Padding(
                  padding: EdgeInsets.only(right: 8.0),
                  child: Icon(FluentIcons.search),
                ),
                onChanged: (value) {
                  controller.filterPlugins();
                  controller.setFirstFilteredPluginDetailActive();
                },
              );
            }),
          ),
        ),
        Expanded(
          child: Scrollbar(
            thumbVisibility: false,
            child: Obx(() {
              return ListView.builder(
                primary: true,
                itemCount: controller.filteredPluginDetails.length,
                itemBuilder: (context, index) {
                  final plugin = controller.filteredPluginDetails[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8.0),
                    child: Obx(() {
                      final isActive = controller.activePluginDetail.value.id == plugin.id;
                      return Container(
                        decoration: BoxDecoration(
                          color: isActive ? SettingPrimaryColor : Colors.transparent,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: GestureDetector(
                          behavior: HitTestBehavior.translucent,
                          onTap: () {
                            controller.activePluginDetail.value = plugin;
                          },
                          // fluent listTile is not clickable
                          child: material.ListTile(
                            contentPadding: const EdgeInsets.only(left: 6, right: 6),
                            leading: WoxImageView(woxImage: plugin.icon, width: 32),
                            title: Text(plugin.name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(
                                  fontSize: 15,
                                  color: isActive ? Colors.white : Colors.black,
                                )),
                            subtitle: Row(
                              mainAxisAlignment: MainAxisAlignment.start,
                              children: [
                                Text(
                                  plugin.version,
                                  maxLines: 1, // Limiting the description to two lines
                                  overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                                  style: TextStyle(
                                    color: isActive ? Colors.white : Colors.grey,
                                    fontSize: 12,
                                  ),
                                ),
                                const SizedBox(width: 10),
                                Text(
                                  plugin.author,
                                  maxLines: 1, // Limiting the description to two lines
                                  overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                                  style: TextStyle(
                                    color: isActive ? Colors.white : Colors.grey,
                                    fontSize: 12,
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
    if (controller.isStorePluginList.value) {
      if (plugin.isInstalled) {
        return Icon(FluentIcons.skype_circle_check, color: isActive ? Colors.white : Colors.green);
      }
    }
    return const SizedBox();
  }

  Widget pluginDetail() {
    return Expanded(
      child: Obx(() {
        final plugin = controller.activePluginDetail.value;
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
                    style: const TextStyle(
                      fontSize: 20,
                    ),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 10.0),
                  child: Text(
                    plugin.version,
                    style: const TextStyle(
                      color: Colors.grey,
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
                        color: SettingWarningColor,
                        border: Border.all(color: SettingWarningColor),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(
                        controller.tr('plugin_dev_tag'),
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
                  style: const TextStyle(
                    color: Colors.grey,
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
                          controller.tr('plugin_website'),
                          style: TextStyle(
                            color: Colors.blue,
                          ),
                        ),
                        Padding(
                          padding: const EdgeInsets.only(left: 4.0),
                          child: Icon(
                            FluentIcons.open_in_new_tab,
                            size: 12,
                            color: Colors.blue,
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
                      onPressed: () {
                        controller.uninstallPlugin(plugin);
                      },
                      child: Text(controller.tr('plugin_uninstall')),
                    ),
                  ),
                if (!plugin.isInstalled)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Obx(() => Button(
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
                                    Text(controller.tr("plugin_installing")),
                                  ],
                                )
                              : Text(controller.tr('plugin_install')),
                        )),
                  ),
                if (plugin.isInstalled && !plugin.isDisable)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.disablePlugin(plugin);
                      },
                      child: Text(controller.tr('plugin_disable')),
                    ),
                  ),
                if (plugin.isInstalled && plugin.isDisable)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.enablePlugin(plugin);
                      },
                      child: Text(controller.tr('plugin_enable')),
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
              tabAlignment: material.TabAlignment.start,
              onAddTabMoveTo: dt.MoveToTab.idol,
              labelColor: SettingPrimaryColor,
              indicatorColor: SettingPrimaryColor,
              dynamicTabs: [
                if (controller.shouldShowSettingTab())
                  dt.TabData(
                    index: 0,
                    title: material.Tab(
                      child: Text(controller.tr('plugin_tab_settings')),
                    ),
                    content: pluginTabSetting(),
                  ),
                dt.TabData(
                  index: 1,
                  title: material.Tab(
                    child: Text(controller.tr('plugin_tab_trigger_keywords')),
                  ),
                  content: pluginTabTriggerKeywords(),
                ),
                dt.TabData(
                  index: 2,
                  title: material.Tab(
                    child: Text(controller.tr('plugin_tab_commands')),
                  ),
                  content: pluginTabCommand(),
                ),
                dt.TabData(
                  index: 3,
                  title: material.Tab(
                    child: Text(controller.tr('plugin_tab_description')),
                  ),
                  content: pluginTabDescription(),
                ),
                dt.TabData(
                  index: 4,
                  title: material.Tab(
                    child: Text(controller.tr('plugin_tab_privacy')),
                  ),
                  content: pluginTabPrivacy(),
                ),
              ],
              onTabControllerUpdated: (tabController) {
                controller.activePluginTabController = tabController;
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
            controller.activePluginDetail.value.description,
          ),
          const SizedBox(height: 20),
          controller.activePluginDetail.value.screenshotUrls.isNotEmpty
              ? ImageSlideshow(
                  width: double.infinity,
                  height: 400,
                  indicatorColor: SettingPrimaryColor,
                  children: [
                    ...controller.activePluginDetail.value.screenshotUrls.map((e) => Image.network(e)),
                  ],
                )
              : const SizedBox(),
        ],
      ),
    );
  }

  Widget pluginTabSetting() {
    return Obx(() {
      var plugin = controller.activePluginDetail.value;
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
                        controller.refreshPluginList();
                      },
                    );
                  }
                  if (e.type == "textbox") {
                    return WoxSettingPluginTextBox(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueTextBox,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                        controller.refreshPluginList();
                      },
                    );
                  }
                  if (e.type == "newline") {
                    return WoxSettingPluginNewLine(
                      value: "",
                      item: e.value as PluginSettingValueNewLine,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                        controller.refreshPluginList();
                      },
                    );
                  }
                  if (e.type == "select") {
                    return WoxSettingPluginSelect(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueSelect,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                        controller.refreshPluginList();
                      },
                    );
                  }
                  if (e.type == "selectAIModel") {
                    return WoxSettingPluginSelectAIModel(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueSelectAIModel,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                        controller.refreshPluginList();
                      },
                    );
                  }
                  if (e.type == "head") {
                    return WoxSettingPluginHead(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueHead,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                        controller.refreshPluginList();
                      },
                    );
                  }
                  if (e.type == "label") {
                    return WoxSettingPluginLabel(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueLabel,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                        controller.refreshPluginList();
                      },
                    );
                  }
                  if (e.type == "table") {
                    return WoxSettingPluginTable(
                      value: plugin.setting.settings[e.value.key] ?? "",
                      item: e.value as PluginSettingValueTable,
                      onUpdate: (key, value) async {
                        await controller.updatePluginSetting(plugin.id, key, value);
                        controller.refreshPluginList();
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
    var plugin = controller.activePluginDetail.value;
    if (plugin.triggerKeywords.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(controller.tr('plugin_no_trigger_keywords')),
          ],
        ),
      );
    }

    return WoxSettingPluginTable(
      value: json.encode(plugin.triggerKeywords.map((e) => {"keyword": e}).toList()),
      tableWidth: 680,
      item: PluginSettingValueTable.fromJson({
        "Key": "_triggerKeywords",
        "Columns": [
          {
            "Key": "keyword",
            "Label": controller.tr('plugin_trigger_keyword_column'),
            "Tooltip": controller.tr('plugin_trigger_keyword_tooltip'),
            "Type": "text",
            "TextMaxLines": 1,
            "Validators": [
              {"Type": "not_empty"}
            ],
          },
        ],
        "SortColumnKey": "keyword"
      }),
      onUpdate: (key, value) {
        final List<String> triggerKeywords = [];
        for (var item in json.decode(value)) {
          triggerKeywords.add(item["keyword"]);
        }
        plugin.triggerKeywords = triggerKeywords;
        controller.updatePluginTriggerKeywords(plugin.id, triggerKeywords);
        controller.refreshPluginList();
      },
    );
  }

  Widget pluginTabCommand() {
    var plugin = controller.activePluginDetail.value;
    if (plugin.commands.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(controller.tr('plugin_no_commands')),
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
            "Label": controller.tr('plugin_command_name_column'),
            "Width": 120,
            "Type": "text",
            "TextMaxLines": 1,
            "Validators": [
              {"Type": "not_empty"}
            ],
          },
          {
            "Key": "Description",
            "Label": controller.tr('plugin_command_desc_column'),
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
    var plugin = controller.activePluginDetail.value;
    var noDataAccess = Padding(
      padding: EdgeInsets.all(16),
      child: Text(controller.tr('plugin_no_data_access')),
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
            Text(controller.tr('plugin_data_access_title')),
            ...params.map((e) {
              if (e == "requireActiveWindowName") {
                return privacyItem(
                  material.Icons.window,
                  controller.tr('plugin_privacy_window_name'),
                  controller.tr('plugin_privacy_window_name_desc'),
                );
              }
              if (e == "requireActiveWindowPid") {
                return privacyItem(
                  material.Icons.window,
                  controller.tr('plugin_privacy_window_pid'),
                  controller.tr('plugin_privacy_window_pid_desc'),
                );
              }
              if (e == "requireActiveBrowserUrl") {
                return privacyItem(
                  material.Icons.web_sharp,
                  controller.tr('plugin_privacy_browser_url'),
                  controller.tr('plugin_privacy_browser_url_desc'),
                );
              }
              if (e == "llm") {
                return privacyItem(
                  material.Icons.chat,
                  controller.tr('plugin_privacy_llm'),
                  controller.tr('plugin_privacy_llm_desc'),
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
              Icon(icon),
              SizedBox(width: 10),
              Text(title),
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
                    color: Colors.grey[100],
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
            width: 250,
            child: pluginList(),
          ),
          // This is your divider
          Container(
            width: 1,
            height: double.infinity,
            color: Colors.grey[30],
            margin: const EdgeInsets.only(right: 10, left: 10),
          ),
          pluginDetail(),
        ],
      ),
    );
  }
}
