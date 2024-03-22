import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/views/wox_setting_store_plugin_view.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

import 'wox_setting_general_view.dart';
import 'wox_setting_installed_plugin_view.dart';

class WoxSettingView extends GetView<WoxSettingController> {
  const WoxSettingView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return RawKeyboardListener(
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
          child: Scaffold(
            body: FluentApp(
              debugShowCheckedModeBanner: false,
              home: NavigationView(
                transitionBuilder: (child, animation) {
                  return SuppressPageTransition(child: child);
                },
                pane: NavigationPane(
                  header: const SizedBox(height: 10),
                  selected: controller.activePaneIndex.value,
                  onChanged: (index) => controller.activePaneIndex.value = index,
                  displayMode: PaneDisplayMode.open,
                  size: const NavigationPaneSize(openWidth: 250),
                  items: [
                    PaneItem(
                      icon: const Icon(FluentIcons.settings),
                      title: const Text('General'),
                      body: const WoxSettingGeneralView(),
                    ),
                    PaneItemExpander(
                        icon: const Icon(FluentIcons.app_icon_default_add),
                        title: const Text('Plugins'),
                        body: const WoxSettingStorePluginView(),
                        onTap: () {
                          controller.loadStorePlugins();
                          controller.activePaneIndex.value = 2;
                        },
                        initiallyExpanded: true,
                        items: [
                          PaneItem(
                            icon: const Icon(FluentIcons.office_store_logo),
                            title: const Text('Store Plugins'),
                            body: const WoxSettingStorePluginView(),
                            onTap: () => controller.loadStorePlugins(),
                          ),
                          PaneItem(
                            icon: const Icon(FluentIcons.installation),
                            title: const Text('Installed Plugins'),
                            body: const WoxSettingInstalledPluginView(),
                            onTap: () => controller.loadInstalledPlugins(),
                          ),
                        ]),
                    PaneItemExpander(
                      icon: const Icon(FluentIcons.color),
                      title: const Text('Themes'),
                      body: const Text('Themes'),
                      onTap: () => controller.activePaneIndex.value = 5,
                      initiallyExpanded: true,
                      items: [
                        PaneItem(
                          icon: const Icon(FluentIcons.mail),
                          title: const Text('Store Themes'),
                          body: const Text('Track orders'),
                        ),
                        PaneItem(
                          icon: const Icon(FluentIcons.installation),
                          title: const Text('Installed Themes'),
                          body: const Text('Track orders'),
                        ),
                      ],
                    ),
                  ],
                  footerItems: [
                    PaneItem(
                      icon: const Icon(FluentIcons.back),
                      title: const Text('Back'),
                      body: const Text('Back'),
                      onTap: () => controller.hideWindow(),
                    ),
                  ],
                ),
              ),
            ),
          ));
    });
  }
}
