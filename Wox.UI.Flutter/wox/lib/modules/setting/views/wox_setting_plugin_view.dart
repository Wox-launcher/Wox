import 'package:flutter/material.dart' as base;
import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/services.dart';
import 'package:flutter_image_slideshow/flutter_image_slideshow.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_plugin.dart';
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
          child: base.Scrollbar(
            child: Obx(() {
              return ListView.builder(
                itemCount: controller.filteredPluginDetails.length,
                itemBuilder: (context, index) {
                  final plugin = controller.filteredPluginDetails[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8.0),
                    child: Obx(() {
                      final isActive = controller.activePluginDetail.value.id == plugin.id;
                      return Container(
                        decoration: BoxDecoration(
                          color: isActive ? SettingPrimaryColor : base.Colors.transparent,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: GestureDetector(
                          behavior: HitTestBehavior.translucent,
                          onTap: () {
                            controller.activePluginDetail.value = plugin;
                          },
                          child: base.ListTile(
                            leading: WoxImageView(woxImage: plugin.icon, width: 32),
                            //ellipsis: true,
                            title: Text(plugin.name,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(
                                  fontSize: 15,
                                  color: isActive ? base.Colors.white : base.Colors.black,
                                )),
                            subtitle: base.Row(
                              mainAxisAlignment: MainAxisAlignment.start,
                              children: [
                                Text(
                                  "${plugin.version}",
                                  maxLines: 1, // Limiting the description to two lines
                                  overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                                  style: TextStyle(
                                    color: isActive ? base.Colors.white : base.Colors.grey,
                                    fontSize: 12,
                                  ),
                                ),
                                const base.SizedBox(width: 10),
                                Text(
                                  "${plugin.author}",
                                  maxLines: 1, // Limiting the description to two lines
                                  overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                                  style: TextStyle(
                                    color: isActive ? base.Colors.white : base.Colors.grey,
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
        return const Icon(FluentIcons.skype_circle_check, color: base.Colors.green);
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
                      color: base.Colors.grey,
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
                    color: base.Colors.grey,
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 18.0),
                  child: HyperlinkButton(
                    onPressed: () {
                      controller.openPluginWebsite(plugin.website);
                    },
                    child: const base.Row(
                      children: [
                        Text(
                          "website",
                          style: TextStyle(
                            color: base.Colors.blue,
                          ),
                        ),
                        Padding(
                          padding: EdgeInsets.only(left: 4.0),
                          child: Icon(
                            FluentIcons.open_in_new_tab,
                            size: 12,
                            color: base.Colors.blue,
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
                  base.Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.uninstallPlugin(plugin);
                      },
                      child: const Text('Uninstall'),
                    ),
                  ),
                if (!plugin.isInstalled)
                  base.Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.installPlugin(plugin);
                      },
                      child: const Text('Install'),
                    ),
                  ),
                if (plugin.isInstalled && !plugin.isDisable)
                  base.Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.disablePlugin(plugin);
                      },
                      child: const Text('Disable'),
                    ),
                  ),
                if (plugin.isInstalled && plugin.isDisable)
                  base.Padding(
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
            child: base.DefaultTabController(
              length: plugin.isInstalled ? 2 : 1,
              child: Column(
                children: [
                  base.TabBar(
                    isScrollable: true,
                    tabAlignment: base.TabAlignment.start,
                    labelColor: SettingPrimaryColor,
                    indicatorColor: SettingPrimaryColor,
                    tabs: [
                      const base.Tab(
                        child: Text('Description'),
                      ),
                      if (plugin.isInstalled)
                        const base.Tab(
                          child: Text('Settings'),
                        )
                    ],
                  ),
                  Expanded(
                    child: base.TabBarView(
                      children: [
                        pluginTabDescription(),
                        controller.activePluginDetail.value.isInstalled ? pluginTabSetting() : const SizedBox(),
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

  Widget pluginTabDescription() {
    return base.Padding(
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
                    ...controller.activePluginDetail.value.screenshotUrls.map((e) => base.Image.network(e)),
                  ],
                )
              : const SizedBox(),
        ],
      ),
    );
  }

  Widget pluginTabSetting() {
    return const base.Padding(
      padding: EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Settings',
          ),
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
