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
import 'package:wox/modules/setting/views/wox_setting_usage_view.dart';
import 'package:wox/modules/setting/views/wox_setting_privacy_view.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/log.dart';

import 'wox_setting_plugin_view.dart';
import 'wox_setting_general_view.dart';
import 'wox_setting_network_view.dart';
import 'wox_setting_runtime_view.dart';
import 'wox_setting_debug_view.dart';

class WoxSettingView extends StatefulWidget {
  const WoxSettingView({super.key});

  @override
  State<WoxSettingView> createState() => _WoxSettingViewState();
}

class _WoxSettingViewState extends State<WoxSettingView> {
  static const String navItemKeyPrefix = 'settings-nav-';
  static const String navListKey = 'settings-nav-list';
  static const String backButtonKey = 'settings-back-button';

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
        key: ValueKey('$navItemKeyPrefix${flatItem.path}'),
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
          // The navigation rail now uses a calmer selected state with a small border; the previous filled block made the sidebar heavier than the content.
          padding: EdgeInsets.only(left: 16.0 + (flatItem.depth * 18.0), right: 16.0, top: 12.0, bottom: 12.0),
          decoration: BoxDecoration(
            color: isSelected ? getThemeActiveBackgroundColor().withValues(alpha: 0.16) : Colors.transparent,
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: isSelected ? getThemeActiveBackgroundColor().withValues(alpha: 0.32) : Colors.transparent),
          ),
          margin: const EdgeInsets.symmetric(horizontal: 14, vertical: 3),
          child: Row(
            children: [
              Icon(item.icon, color: isSelected ? getThemeTextColor() : getThemeTextColor().withValues(alpha: 0.78), size: 18),
              const SizedBox(width: 12),
              Expanded(child: Text(item.title, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal))),
              if (isParent) Icon(item.isExpanded ? Icons.expand_more : Icons.chevron_right, color: getThemeTextColor().withValues(alpha: 0.6), size: 18),
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
              },
            ),
            _NavItem(
              id: 'plugins.installed',
              icon: Icons.widgets_outlined,
              title: controller.tr('ui_installed_plugins'),
              body: const WoxSettingPluginView(),
              onTap: () async {
                await controller.switchToPluginList(const UuidV4().generate(), false);
              },
            ),
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
              },
            ),
            _NavItem(
              id: 'themes.installed',
              icon: Icons.brush_outlined,
              title: controller.tr('ui_installed_themes'),
              body: const WoxSettingThemeView(),
              onTap: () async {
                await controller.switchToThemeList(false);
              },
            ),
          ],
        ),
        _NavItem(id: 'usage', icon: Icons.query_stats_outlined, title: controller.tr('ui_usage'), body: const WoxSettingUsageView()),
        if (Env.isDev)
          // New dev-only settings entry: these controls expose backend debug
          // tails without leaking internal instrumentation switches into
          // packaged user builds.
          _NavItem(id: 'debug', icon: Icons.bug_report_outlined, title: controller.tr('ui_debug'), body: const WoxSettingDebugView()),
        _NavItem(id: 'privacy', icon: Icons.privacy_tip_outlined, title: controller.tr('ui_privacy'), body: const WoxSettingPrivacyView()),
        _NavItem(id: 'about', icon: Icons.info_outline, title: controller.tr('ui_about'), body: const WoxSettingAboutView()),
      ];

      return WoxPlatformFocus(
        focusNode: controller.settingFocusNode,
        autofocus: true,
        onKeyEvent: (FocusNode node, KeyEvent event) {
          Logger.instance.debug(const UuidV4().generate(), "[KEYLOG][FLUTTER-SETTING] WoxPlatformFocus received key event: ${event.logicalKey.keyLabel}");
          if (event.logicalKey == LogicalKeyboardKey.escape && (event is KeyDownEvent || event is KeyRepeatEvent)) {
            // Bug fix: Escape can arrive as a down/repeat/up sequence. The old
            // KeyDown handler exited settings immediately, so holding Escape could
            // also leak follow-up events into the launcher or hide path. Consume
            // the press here and perform the route transition on KeyUp only.
            return KeyEventResult.handled;
          }
          if (event is KeyUpEvent && event.logicalKey == LogicalKeyboardKey.escape) {
            final traceId = const UuidV4().generate();
            Logger.instance.info(traceId, "[KEYLOG][FLUTTER-SETTING] ESC key pressed, hiding window");
            controller.hideWindow(traceId);
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
                width: 250,
                decoration: BoxDecoration(
                  // A slightly wider, quieter rail matches the refined content rhythm and gives nested items room to breathe.
                  color: getThemeTextColor().withValues(alpha: 0.035),
                  // Settings separators should use one visual token. The old dimmed
                  // sidebar border was weaker than the plugin/detail splitter, so it
                  // made the three-pane layout look like separate components.
                  border: Border(right: BorderSide(color: getThemeSettingDividerColor(), width: 1)),
                ),
                child: Column(
                  children: [
                    const SizedBox(height: 26),
                    Expanded(child: ListView(key: const ValueKey(navListKey), padding: const EdgeInsets.symmetric(vertical: 4), children: _buildNavTree(navItems))),
                    Container(
                      margin: const EdgeInsets.fromLTRB(14, 8, 14, 16),
                      child: GestureDetector(
                        key: const ValueKey(backButtonKey),
                        onTap: () => controller.hideWindow(const UuidV4().generate()),
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                          decoration: BoxDecoration(borderRadius: BorderRadius.circular(6)),
                          child: Row(
                            children: [
                              Icon(Icons.arrow_back, color: getThemeTextColor().withValues(alpha: 0.86), size: 18),
                              const SizedBox(width: 12),
                              Text(controller.tr('ui_back'), style: TextStyle(color: getThemeTextColor(), fontSize: 13)),
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
              Expanded(child: _findBodyByPath(navItems, controller.activeNavPath.value) ?? navItems[0].body!),
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

  _NavItem({required this.icon, required this.title, required this.id, this.body, this.onTap, this.children = const [], this.isExpanded = false});

  bool get isParent => children.isNotEmpty;
}

// Helper class for flattened navigation items
class _FlatNavItem {
  final _NavItem item;
  final int depth;
  final String path;

  _FlatNavItem({required this.item, required this.depth, required this.path});
}
