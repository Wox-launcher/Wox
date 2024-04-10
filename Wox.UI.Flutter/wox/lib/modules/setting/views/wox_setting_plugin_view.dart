import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart' as material;
import 'package:flutter/services.dart';
import 'package:flutter_image_slideshow/flutter_image_slideshow.dart';
import 'package:get/get.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/wox_plugin_setting_head.dart';
import 'package:wox/entity/wox_plugin_setting_label.dart';
import 'package:wox/entity/wox_plugin_setting_newline.dart';
import 'package:wox/entity/wox_plugin_setting_select.dart';
import 'package:wox/entity/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_plugin_setting_textbox.dart';
import 'package:wox/components/plugin/wox_setting_plugin_checkbox_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_textbox_view.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingPluginView extends GetView<WoxSettingController> {
  const WoxSettingPluginView({super.key});

  Widget pluginList() {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 20),
          child: RawKeyboardListener(
            focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
              if (event is RawKeyDownEvent) {
                switch (event.logicalKey) {
                  case LogicalKeyboardKey.escape:
                    controller.hideWindow();
                    return KeyEventResult.handled;
                }
              }

              return KeyEventResult.ignored;
            }),
            child: Obx(() {
              return TextBox(
                autofocus: true,
                placeholder: 'Search ${controller.filteredPluginDetails.length} plugins',
                padding: const EdgeInsets.all(10),
                suffix: const Padding(
                  padding: EdgeInsets.only(right: 8.0),
                  child: Icon(FluentIcons.search),
                ),
                onChanged: (value) => {controller.onFilterPlugins(value)},
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
                            trailing: pluginTrailIcon(plugin),
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

  Widget pluginTrailIcon(PluginDetail plugin) {
    if (controller.isStorePluginList.value) {
      if (plugin.isInstalled) {
        return Icon(FluentIcons.skype_circle_check, color: Colors.green);
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
                    style: TextStyle(
                      color: Colors.grey,
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
                          "website",
                          style: TextStyle(
                            color: Colors.blue,
                          ),
                        ),
                        Padding(
                          padding: EdgeInsets.only(left: 4.0),
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
                      child: const Text('Uninstall'),
                    ),
                  ),
                if (!plugin.isInstalled)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.installPlugin(plugin);
                      },
                      child: const Text('Install'),
                    ),
                  ),
                if (plugin.isInstalled && !plugin.isDisable)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.disablePlugin(plugin);
                      },
                      child: const Text('Disable'),
                    ),
                  ),
                if (plugin.isInstalled && plugin.isDisable)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.enablePlugin(plugin);
                      },
                      child: const Text('Enable'),
                    ),
                  ),
              ],
            ),
          ),
          Expanded(
            child: material.DefaultTabController(
              length: shouldShowSettingTab() ? 2 : 1,
              child: Column(
                children: [
                  material.TabBar(
                    isScrollable: true,
                    tabAlignment: material.TabAlignment.start,
                    labelColor: SettingPrimaryColor,
                    indicatorColor: SettingPrimaryColor,
                    tabs: [
                      const material.Tab(
                        child: Text('Description'),
                      ),
                      if (shouldShowSettingTab())
                        const material.Tab(
                          child: Text('Settings'),
                        )
                    ],
                  ),
                  Expanded(
                    child: material.TabBarView(
                      children: [
                        pluginTabDescription(),
                        if (shouldShowSettingTab()) pluginTabSetting(),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        ]);
      }),
    );
  }

  bool shouldShowSettingTab() {
    return controller.activePluginDetail.value.isInstalled && controller.activePluginDetail.value.settingDefinitions.isNotEmpty;
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
                    return WoxSettingPluginCheckbox(plugin, e.value as PluginSettingValueCheckBox, (key, value) {
                      controller.refreshPluginList();
                    });
                  }
                  if (e.type == "textbox") {
                    return WoxSettingPluginTextBox(plugin, e.value as PluginSettingValueTextBox, (key, value) {
                      controller.refreshPluginList();
                    });
                  }
                  if (e.type == "newline") {
                    return WoxSettingPluginNewLine(plugin, e.value as PluginSettingValueNewLine, (key, value) {
                      controller.refreshPluginList();
                    });
                  }
                  if (e.type == "select") {
                    return WoxSettingPluginSelect(plugin, e.value as PluginSettingValueSelect, (key, value) {
                      controller.refreshPluginList();
                    });
                  }
                  if (e.type == "head") {
                    return WoxSettingPluginHead(plugin, e.value as PluginSettingValueHead, (key, value) {
                      controller.refreshPluginList();
                    });
                  }
                  if (e.type == "label") {
                    return WoxSettingPluginLabel(plugin, e.value as PluginSettingValueLabel, (key, value) {
                      controller.refreshPluginList();
                    });
                  }
                  if (e.type == "table") {
                    return WoxSettingPluginTable(plugin, e.value as PluginSettingValueTable, (key, value) {
                      controller.refreshPluginList();
                    });
                  }

                  return Text(e.type + " not suppr");
                },
              )
            ],
          ),
        ),
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(20),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 300,
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
