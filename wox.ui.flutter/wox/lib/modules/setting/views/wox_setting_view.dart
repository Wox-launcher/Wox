import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart' as material;
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/modules/setting/views/wox_setting_ui_view.dart';
import 'package:wox/modules/setting/views/wox_setting_ai_view.dart';
import 'package:wox/modules/setting/views/wox_setting_data_view.dart';
import 'package:wox/modules/setting/views/wox_setting_theme_view.dart';
import 'package:wox/modules/setting/views/wox_setting_about_view.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_theme_util.dart';

import 'wox_setting_plugin_view.dart';
import 'wox_setting_general_view.dart';
import 'wox_setting_network_view.dart';
import 'wox_setting_runtime_view.dart';

class WoxSettingView extends GetView<WoxSettingController> {
  const WoxSettingView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return Focus(
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
          child: material.Scaffold(
              backgroundColor: fromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor),
              body: FluentApp(
                debugShowCheckedModeBanner: false,
                theme: FluentThemeData(
                  accentColor: AccentColor.swatch({
                    'normal': getThemeActiveBackgroundColor(),
                  }),
                  visualDensity: VisualDensity.standard,
                  brightness: getThemeBackgroundColor().computeLuminance() < 0.5 ? Brightness.dark : Brightness.light,
                  scaffoldBackgroundColor: Colors.transparent,
                  micaBackgroundColor: Colors.transparent,
                  acrylicBackgroundColor: Colors.transparent,
                  cardColor: getThemeCardBackgroundColor(),
                  shadowColor: getThemeTextColor().withAlpha(50),
                  inactiveBackgroundColor: getThemeBackgroundColor(),
                  inactiveColor: getThemeSubTextColor(),
                  // typography: Typography.raw(caption: TextStyle(color: getThemeSubTextColor())),
                  focusTheme: FocusThemeData(
                    glowColor: getThemeActiveBackgroundColor().withAlpha(25),
                    primaryBorder: BorderSide(color: getThemeActiveBackgroundColor(), width: 2),
                  ),
                  navigationPaneTheme: const NavigationPaneThemeData(
                    backgroundColor: Colors.transparent,
                  ),
                ),
                home: NavigationView(
                  transitionBuilder: (child, animation) {
                    return SuppressPageTransition(child: child);
                  },
                  pane: NavigationPane(
                    selected: controller.activePaneIndex.value,
                    onChanged: (index) => controller.activePaneIndex.value = index,
                    header: const SizedBox(height: 10),
                    displayMode: PaneDisplayMode.open,
                    size: const NavigationPaneSize(openWidth: 200),
                    items: [
                      PaneItem(
                        icon: const Icon(FluentIcons.settings),
                        title: Text(controller.tr('ui_general')),
                        body: const WoxSettingGeneralView(),
                      ),
                      PaneItem(
                        icon: const Icon(FluentIcons.color),
                        title: Text(controller.tr('ui_ui')),
                        body: const WoxSettingUIView(),
                      ),
                      PaneItem(
                        icon: const Icon(FluentIcons.code),
                        title: Text(controller.tr('ui_ai')),
                        body: const WoxSettingAIView(),
                      ),
                      PaneItem(
                        icon: const Icon(FluentIcons.globe),
                        title: Text(controller.tr('ui_network')),
                        body: const WoxSettingNetworkView(),
                      ),
                      PaneItem(
                        icon: const Icon(FluentIcons.database),
                        title: Text(controller.tr('ui_data')),
                        body: const WoxSettingDataView(),
                      ),
                      PaneItemExpander(
                        icon: const Icon(FluentIcons.app_icon_default_add),
                        title: Text(controller.tr('ui_plugins')),
                        body: const WoxSettingPluginView(),
                        initiallyExpanded: true,
                        items: [
                          PaneItem(
                            icon: const Icon(FluentIcons.office_store_logo),
                            title: Text(controller.tr('ui_store_plugins')),
                            body: const WoxSettingPluginView(),
                            onTap: () async {
                              await controller.switchToPluginList(const UuidV4().generate(), true);
                            },
                          ),
                          PaneItem(
                            icon: const Icon(FluentIcons.installation),
                            title: Text(controller.tr('ui_installed_plugins')),
                            body: const WoxSettingPluginView(),
                            onTap: () async {
                              await controller.switchToPluginList(const UuidV4().generate(), false);
                            },
                          ),
                          PaneItem(
                            icon: const Icon(FluentIcons.code),
                            title: Text(controller.tr('ui_runtime_settings')),
                            body: WoxSettingRuntimeView(),
                          ),
                        ],
                      ),
                      PaneItemExpander(
                        icon: const Icon(FluentIcons.color),
                        title: Text(controller.tr('ui_themes')),
                        body: const WoxSettingThemeView(),
                        initiallyExpanded: true,
                        items: [
                          PaneItem(
                            icon: const Icon(FluentIcons.mail),
                            title: Text(controller.tr('ui_store_themes')),
                            body: const WoxSettingThemeView(),
                            onTap: () async {
                              await controller.switchToThemeList(true);
                            },
                          ),
                          PaneItem(
                            icon: const Icon(FluentIcons.installation),
                            title: Text(controller.tr('ui_installed_themes')),
                            body: const WoxSettingThemeView(),
                            onTap: () async {
                              await controller.switchToThemeList(false);
                            },
                          ),
                        ],
                      ),
                      PaneItem(
                        icon: const Icon(FluentIcons.info),
                        title: Text(controller.tr('ui_about')),
                        body: const WoxSettingAboutView(),
                      ),
                    ],
                    footerItems: [
                      PaneItem(
                        icon: const Icon(FluentIcons.back),
                        title: Text(controller.tr('ui_back')),
                        body: Text(controller.tr('ui_back')),
                        onTap: () => controller.hideWindow(),
                      ),
                    ],
                  ),
                ),
              )));
    });
  }
}
