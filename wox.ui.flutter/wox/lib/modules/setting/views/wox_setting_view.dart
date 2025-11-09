import 'package:flutter/material.dart';
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
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
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
  WoxSettingController get controller => Get.find<WoxSettingController>();

  // Flatten the tree to get all items with their semantic IDs
  List<_FlatNavItem> _flattenNavItems(List<_NavItem> items, {int depth = 0}) {
    List<_FlatNavItem> result = [];
    for (final item in items) {
      result.add(_FlatNavItem(item: item, depth: depth, path: item.id));
      if (item.isExpanded && item.children.isNotEmpty) {
        result.addAll(_flattenNavItems(item.children, depth: depth + 1));
      }
    }
    return result;
  }

  Widget? _findBodyByPath(List<_NavItem> items, String path) {
    // First try to find in top level
    for (final item in items) {
      if (item.id == path) {
        return item.body;
      }
      // Then search in children
      for (final child in item.children) {
        if (child.id == path) {
          return child.body;
        }
      }
    }
    return null;
  }

  List<Widget> _buildNavTree(List<_NavItem> items) {
    final flatItems = _flattenNavItems(items);
    return flatItems.map((flatItem) {
      final item = flatItem.item;
      final isSelected = controller.activeNavPath.value == flatItem.path;
      final isParent = item.isParent;

      return GestureDetector(
        onTap: () {
          setState(() {
            if (isParent) {
              item.isExpanded = !item.isExpanded;
            } else {
              controller.activeNavPath.value = flatItem.path;
              if (item.onTap != null) {
                item.onTap!();
              }
            }
          });
        },
        child: Container(
          padding: EdgeInsets.only(
            left: 16.0 + (flatItem.depth * 16.0),
            right: 16.0,
            top: 10.0,
            bottom: 10.0,
          ),
          decoration: BoxDecoration(
            color: isSelected ? getThemeActiveBackgroundColor().withOpacity(0.15) : Colors.transparent,
            borderRadius: BorderRadius.circular(6),
          ),
          margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
          child: Row(
            children: [
              Icon(item.icon, color: getThemeTextColor(), size: 18),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  item.title,
                  style: TextStyle(
                    color: getThemeTextColor(),
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w500 : FontWeight.normal,
                  ),
                ),
              ),
              if (isParent)
                Icon(
                  item.isExpanded ? Icons.expand_more : Icons.chevron_right,
                  color: getThemeTextColor().withOpacity(0.6),
                  size: 18,
                ),
            ],
          ),
        ),
      );
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      // Define navigation items with tree structure
      // This needs to be inside Obx so it rebuilds when language changes
      final List<_NavItem> navItems = [
        _NavItem(id: 'general', icon: Icons.settings_outlined, title: controller.tr('ui_general'), body: const WoxSettingGeneralView()),
        _NavItem(id: 'ui', icon: Icons.palette_outlined, title: controller.tr('ui_ui'), body: const WoxSettingUIView()),
        _NavItem(id: 'ai', icon: Icons.psychology_outlined, title: controller.tr('ui_ai'), body: const WoxSettingAIView()),
        _NavItem(id: 'network', icon: Icons.public_outlined, title: controller.tr('ui_network'), body: const WoxSettingNetworkView()),
        _NavItem(id: 'data', icon: Icons.folder_outlined, title: controller.tr('ui_data'), body: const WoxSettingDataView()),
        _NavItem(
          id: 'plugins',
          icon: Icons.extension_outlined,
          title: controller.tr('ui_plugins'),
          isExpanded: true,
          children: [
            _NavItem(
                id: 'plugins.store',
                icon: Icons.shopping_bag_outlined,
                title: controller.tr('ui_store_plugins'),
                body: const WoxSettingPluginView(),
                onTap: () async {
                  await controller.switchToPluginList(const UuidV4().generate(), true);
                }),
            _NavItem(
                id: 'plugins.installed',
                icon: Icons.widgets_outlined,
                title: controller.tr('ui_installed_plugins'),
                body: const WoxSettingPluginView(),
                onTap: () async {
                  await controller.switchToPluginList(const UuidV4().generate(), false);
                }),
            _NavItem(id: 'plugins.runtime', icon: Icons.terminal_outlined, title: controller.tr('ui_runtime_settings'), body: WoxSettingRuntimeView()),
          ],
        ),
        _NavItem(
          id: 'themes',
          icon: Icons.color_lens_outlined,
          title: controller.tr('ui_themes'),
          isExpanded: true,
          children: [
            _NavItem(
                id: 'themes.store',
                icon: Icons.shopping_bag_outlined,
                title: controller.tr('ui_store_themes'),
                body: const WoxSettingThemeView(),
                onTap: () async {
                  await controller.switchToThemeList(true);
                }),
            _NavItem(
                id: 'themes.installed',
                icon: Icons.brush_outlined,
                title: controller.tr('ui_installed_themes'),
                body: const WoxSettingThemeView(),
                onTap: () async {
                  await controller.switchToThemeList(false);
                }),
          ],
        ),
        _NavItem(id: 'about', icon: Icons.info_outline, title: controller.tr('ui_about'), body: const WoxSettingAboutView()),
      ];

      return WoxPlatformFocus(
        focusNode: controller.settingFocusNode,
        autofocus: true,
        onKeyEvent: (FocusNode node, KeyEvent event) {
          Logger.instance.debug(const UuidV4().generate(), "[KEYLOG][FLUTTER-SETTING] WoxPlatformFocus received key event: ${event.logicalKey.keyLabel}");
          if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
            final traceId = const UuidV4().generate();
            Logger.instance.info(traceId, "[KEYLOG][FLUTTER-SETTING] ESC key pressed, hiding window");
            controller.hideWindow();
            return KeyEventResult.handled;
          }
          return KeyEventResult.ignored;
        },
        child: Scaffold(
          backgroundColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor),
          body: Row(
            children: [
              // Navigation rail
              Container(
                width: 220,
                decoration: BoxDecoration(
                  color: getThemeTextColor().withOpacity(0.03),
                  border: Border(
                    right: BorderSide(
                      color: getThemeTextColor().withOpacity(0.08),
                      width: 1,
                    ),
                  ),
                ),
                child: Column(
                  children: [
                    const SizedBox(height: 16),
                    Expanded(
                      child: ListView(
                        padding: const EdgeInsets.symmetric(vertical: 4),
                        children: _buildNavTree(navItems),
                      ),
                    ),
                    Container(
                      margin: const EdgeInsets.all(8),
                      child: GestureDetector(
                        onTap: () => controller.hideWindow(),
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                          decoration: BoxDecoration(
                            borderRadius: BorderRadius.circular(6),
                          ),
                          child: Row(
                            children: [
                              Icon(Icons.arrow_back, color: getThemeTextColor(), size: 18),
                              const SizedBox(width: 12),
                              Text(
                                controller.tr('ui_back'),
                                style: TextStyle(color: getThemeTextColor(), fontSize: 13),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(height: 8),
                  ],
                ),
              ),
              // Content area
              Expanded(
                child: _findBodyByPath(navItems, controller.activeNavPath.value) ?? navItems[0].body!,
              ),
            ],
          ),
        ), // Scaffold
      ); // WoxPlatformFocus
    });
  }
}

// Helper class for navigation items
class _NavItem {
  final IconData icon;
  final String title;
  final String id; // Semantic ID for this nav item
  final Widget? body;
  final VoidCallback? onTap;
  final List<_NavItem> children;
  bool isExpanded;

  _NavItem({
    required this.icon,
    required this.title,
    required this.id,
    this.body,
    this.onTap,
    this.children = const [],
    this.isExpanded = false,
  });

  bool get isParent => children.isNotEmpty;
}

// Helper class for flattened navigation items
class _FlatNavItem {
  final _NavItem item;
  final int depth;
  final String path;

  _FlatNavItem({
    required this.item,
    required this.depth,
    required this.path,
  });
}
