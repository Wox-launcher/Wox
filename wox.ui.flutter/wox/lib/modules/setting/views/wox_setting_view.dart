import 'package:fluent_ui/fluent_ui.dart';
import 'dart:io' show Platform;
import 'package:flutter/material.dart' as material;
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/modules/setting/views/wox_setting_ui_view.dart';
import 'package:wox/modules/setting/views/wox_setting_ai_view.dart';
import 'package:wox/modules/setting/views/wox_setting_data_view.dart';
import 'package:wox/modules/setting/views/wox_setting_theme_view.dart';
import 'package:wox/modules/setting/views/wox_setting_about_view.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/log.dart';

import 'wox_setting_plugin_view.dart';
import 'wox_setting_general_view.dart';
import 'wox_setting_network_view.dart';
import 'wox_setting_runtime_view.dart';

class WoxSettingView extends StatefulWidget {
  const WoxSettingView({super.key});

  @override
  State<WoxSettingView> createState() => _WoxSettingViewState();
}

class _WoxSettingViewState extends State<WoxSettingView> {
  final FocusNode _focusNode = FocusNode(debugLabel: 'WoxSettingView-PlatformFocus');

  @override
  void initState() {
    super.initState();
    _focusNode.addListener(() {
      final hasFocus = _focusNode.hasFocus;
      final primaryFocus = FocusManager.instance.primaryFocus;
      final whoHasFocus = primaryFocus?.debugLabel ?? 'unknown';
      Logger.instance.debug(
          const UuidV4().generate(),
          "[KEYLOG][FLUTTER-SETTING] Focus changed: $hasFocus, "
          "primary focus is now: $whoHasFocus");
    });
  }

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  WoxSettingController get controller => Get.find<WoxSettingController>();

  @override
  Widget build(BuildContext context) {
    // Request focus on every build to ensure we always have focus
    WidgetsBinding.instance.addPostFrameCallback((_) {
      // Debug: print current focus information
      final primaryFocus = FocusManager.instance.primaryFocus;
      Logger.instance.debug(
          const UuidV4().generate(),
          "[KEYLOG][FLUTTER-SETTING] Current primary focus: ${primaryFocus?.debugLabel ?? 'null'}, "
          "hasFocus: ${primaryFocus?.hasFocus ?? false}, "
          "our focus node hasFocus: ${_focusNode.hasFocus}");

      if (!_focusNode.hasFocus) {
        _focusNode.requestFocus();
        Logger.instance.debug(const UuidV4().generate(), "[KEYLOG][FLUTTER-SETTING] Requested focus in build");
      }
    });

    return Obx(() {
      return material.Scaffold(
        backgroundColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor),
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
            // Unify fonts on Windows to avoid mixed Segoe/YaHei rendering
            // that makes Chinese text look inconsistent in settings.
            fontFamily: Platform.isWindows ? 'Microsoft YaHei UI' : null,
            typography: Platform.isWindows
                ? Typography.raw(
                    display: TextStyle(
                      color: getThemeTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                    titleLarge: TextStyle(
                      color: getThemeTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                    title: TextStyle(
                      color: getThemeTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                    subtitle: TextStyle(
                      color: getThemeTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                    bodyLarge: TextStyle(
                      color: getThemeTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                    bodyStrong: TextStyle(
                      color: getThemeTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                    body: TextStyle(
                      color: getThemeTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                    caption: TextStyle(
                      color: getThemeSubTextColor(),
                      fontFamily: 'Microsoft YaHei UI',
                      fontFamilyFallback: const [
                        'Microsoft YaHei',
                        'Segoe UI',
                        'Noto Sans CJK SC',
                        'PingFang SC',
                      ],
                    ),
                  )
                : null,
            focusTheme: FocusThemeData(
              glowColor: getThemeActiveBackgroundColor().withAlpha(25),
              primaryBorder: BorderSide(color: getThemeActiveBackgroundColor(), width: 2),
            ),
            navigationPaneTheme: const NavigationPaneThemeData(
              backgroundColor: Colors.transparent,
            ),
          ),
          home: WoxPlatformFocus(
            focusNode: _focusNode,
            autofocus: true,
            onKeyEvent: (FocusNode node, KeyEvent event) {
              Logger.instance.debug(const UuidV4().generate(), "[KEYLOG][FLUTTER-SETTING] Received key event: ${event.logicalKey.keyLabel}");
              if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
                final traceId = const UuidV4().generate();
                Logger.instance.info(traceId, "[KEYLOG][FLUTTER-SETTING] ESC key pressed, hiding window");
                controller.hideWindow();
                return KeyEventResult.handled;
              }
              return KeyEventResult.ignored;
            },
            child: NavigationView(
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
            ), // NavigationView
          ), // WoxPlatformFocus
        ),
      ); // FluentApp
    });
  }
}
