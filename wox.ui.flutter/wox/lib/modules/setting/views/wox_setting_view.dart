import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_setting_search.dart';
import 'package:wox/modules/setting/views/wox_setting_ui_view.dart';
import 'package:wox/modules/setting/views/wox_setting_ai_view.dart';
import 'package:wox/modules/setting/views/wox_setting_data_view.dart';
import 'package:wox/modules/setting/views/wox_setting_theme_view.dart';
import 'package:wox/modules/setting/views/wox_setting_theme_editor_view.dart';
import 'package:wox/modules/setting/views/wox_setting_about_view.dart';
import 'package:wox/modules/setting/views/wox_setting_usage_view.dart';
import 'package:wox/modules/setting/views/wox_setting_privacy_view.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/wox_system_wallpaper_util.dart';
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

  WoxSettingController get controller => Get.find<WoxSettingController>();
  final ScrollController _navScrollController = ScrollController();
  final GlobalKey _navViewportKey = GlobalKey(debugLabel: 'settings-nav-viewport');
  final Map<String, GlobalKey> _navItemKeys = <String, GlobalKey>{};
  late final Worker _activeNavPathWorker;
  String _lastQueuedVisibleNavPath = '';
  bool _consumeSearchEscapeKeyUp = false;

  @override
  void initState() {
    super.initState();
    _activeNavPathWorker = ever<String>(controller.activeNavPath, (_) => _scheduleActiveNavItemVisible());
    HardwareKeyboard.instance.addHandler(_handleHardwareKeyboardEvent);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        unawaited(controller.preloadThemeStore(const UuidV4().generate()));
        unawaited(_preloadThemeEditorWallpaper());
      }
    });
  }

  @override
  void dispose() {
    _activeNavPathWorker.dispose();
    _navScrollController.dispose();
    HardwareKeyboard.instance.removeHandler(_handleHardwareKeyboardEvent);
    super.dispose();
  }

  GlobalKey _getNavItemKey(String navPath) {
    return _navItemKeys.putIfAbsent(navPath, () => GlobalKey(debugLabel: 'settings-nav-$navPath'));
  }

  // Preload the desktop wallpaper as soon as settings opens so the theme editor backdrop is ready before the editor page is selected.
  Future<void> _preloadThemeEditorWallpaper() async {
    final traceId = const UuidV4().generate();
    final wallpaperPath = await WoxSystemWallpaperUtil.instance.loadSystemWallpaperPath(traceId: traceId, forceRefresh: true);
    if (!mounted || wallpaperPath == null) {
      return;
    }
    await WoxSystemWallpaperUtil.instance.precacheSystemWallpaperPath(context, wallpaperPath, traceId: traceId);
  }

  void _scheduleActiveNavItemVisible({int attempt = 0}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }

      final targetContext = _navItemKeys[controller.activeNavPath.value]?.currentContext;
      if (targetContext == null) {
        if (attempt >= 8) {
          return;
        }
        Future.delayed(const Duration(milliseconds: 60), () {
          if (mounted) {
            _scheduleActiveNavItemVisible(attempt: attempt + 1);
          }
        });
        return;
      }

      // Bug fix: search activation already changed the active nav path, but the
      // rail kept its old scroll offset. Ensuring the keyed nav row is visible
      // keeps the sidebar state and the content page aligned after jumps.
      if (_isNavTargetFullyVisible(targetContext)) {
        return;
      }

      final targetRenderObject = targetContext.findRenderObject();
      final viewport = RenderAbstractViewport.maybeOf(targetRenderObject);
      if (targetRenderObject != null && viewport != null && _navScrollController.hasClients) {
        final targetOffset = viewport.getOffsetToReveal(targetRenderObject, 0.18).offset;
        final scrollPosition = _navScrollController.position;
        final clampedOffset = targetOffset.clamp(scrollPosition.minScrollExtent, scrollPosition.maxScrollExtent).toDouble();
        // Bug fix: this runs from a post-frame search/navigation update. A
        // short animation can start after the test/user-visible frame and be
        // cancelled by follow-up rebuilds, so jump directly to keep active state
        // and sidebar position in sync.
        _navScrollController.jumpTo(clampedOffset);
        return;
      }

      Scrollable.ensureVisible(targetContext, duration: const Duration(milliseconds: 180), curve: Curves.easeOutCubic, alignment: 0.18);
    });
  }

  bool _isNavTargetFullyVisible(BuildContext targetContext) {
    final targetRenderObject = targetContext.findRenderObject();
    final viewportRenderObject = _navViewportKey.currentContext?.findRenderObject();
    if (targetRenderObject is! RenderBox || viewportRenderObject is! RenderBox) {
      return false;
    }

    const visibilityTolerance = 0.5;
    final targetTop = targetRenderObject.localToGlobal(Offset.zero).dy;
    final targetBottom = targetRenderObject.localToGlobal(Offset(0, targetRenderObject.size.height)).dy;
    final viewportTop = viewportRenderObject.localToGlobal(Offset.zero).dy;
    final viewportBottom = viewportRenderObject.localToGlobal(Offset(0, viewportRenderObject.size.height)).dy;

    // Bug fix: activeNavPath also changes for ordinary user clicks. The search
    // jump behavior still needs to reveal off-screen rows, but visible rows
    // should keep the user's current sidebar scroll offset instead of being
    // re-aligned on every click.
    return targetTop >= viewportTop - visibilityTolerance && targetBottom <= viewportBottom + visibilityTolerance;
  }

  void _queueActiveNavItemVisibleForBuild() {
    final activePath = controller.activeNavPath.value;
    if (_lastQueuedVisibleNavPath == activePath) {
      return;
    }

    _lastQueuedVisibleNavPath = activePath;
    // Bug fix: activeNavPath can change during search-result activation while
    // the nav rail is also rebuilding. Queueing once from the reactive build
    // path makes the scroll-to-active behavior deterministic without forcing
    // the rail back into position on unrelated rebuilds.
    _scheduleActiveNavItemVisible();
  }

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

  Widget _buildSearchBox() {
    return Padding(
      padding: const EdgeInsets.fromLTRB(14, 16, 14, 8),
      child: Shortcuts(
        shortcuts: const <ShortcutActivator, Intent>{
          SingleActivator(LogicalKeyboardKey.arrowDown): _SettingSearchMoveIntent(1),
          SingleActivator(LogicalKeyboardKey.arrowUp): _SettingSearchMoveIntent(-1),
        },
        child: Actions(
          actions: <Type, Action<Intent>>{
            _SettingSearchMoveIntent: CallbackAction<_SettingSearchMoveIntent>(
              onInvoke: (intent) {
                if (controller.settingSearchPanelVisible.value) {
                  // Bug fix: arrow keys navigate the floating result list while
                  // search is active. Capturing them at the search-field
                  // shortcut layer prevents TextField from also moving the
                  // caret to the beginning or end of the input text.
                  controller.moveSettingSearchSelection(intent.delta);
                }
                return null;
              },
            ),
          },
          child: Obx(() {
            final hasSearchText = controller.settingSearchPanelVisible.value && controller.settingSearchTextController.text.trim().isNotEmpty;
            return SizedBox(
              height: 42,
              child: Container(
                decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor()), borderRadius: BorderRadius.circular(4)),
                child: Row(
                  children: [
                    SizedBox(width: 36, height: 42, child: Icon(Icons.search, color: getThemeSubTextColor(), size: 18)),
                    Expanded(
                      child: Padding(
                        padding: EdgeInsets.only(right: hasSearchText ? 4 : 12),
                        child: Center(
                          // Bug fix: Material InputDecoration's default single-line
                          // padding kept the hint and cursor above visual center in
                          // the compact settings rail. A collapsed field inside a
                          // centered, padded row gives text, placeholder, cursor,
                          // and clear icon consistent spacing and vertical anchor.
                          child: TextSelectionTheme(
                            data: TextSelectionThemeData(selectionColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxTextSelectionBackgroundColor)),
                            child: TextField(
                              key: const ValueKey('settings-search-field'),
                              controller: controller.settingSearchTextController,
                              focusNode: controller.settingSearchFocusNode,
                              cursorColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxCursorColor),
                              style: TextStyle(color: getThemeTextColor(), fontSize: 13, height: 1),
                              decoration: InputDecoration.collapsed(
                                hintText: controller.tr('ui_setting_search_placeholder'),
                                hintStyle: TextStyle(color: getThemeTextColor().withValues(alpha: 0.5), fontSize: 13, height: 1),
                              ),
                              onChanged: (_) => controller.handleSettingSearchChanged(),
                              onSubmitted: (_) {
                                unawaited(controller.activateSelectedSettingSearchResult());
                              },
                            ),
                          ),
                        ),
                      ),
                    ),
                    if (hasSearchText)
                      SizedBox(
                        width: 36,
                        height: 42,
                        child: IconButton(
                          key: const ValueKey('settings-search-clear-button'),
                          visualDensity: VisualDensity.compact,
                          constraints: const BoxConstraints.tightFor(width: 36, height: 42),
                          padding: EdgeInsets.zero,
                          icon: Icon(Icons.close, color: getThemeSubTextColor(), size: 18),
                          onPressed: controller.clearSettingSearch,
                        ),
                      ),
                  ],
                ),
              ),
            );
          }),
        ),
      ),
    );
  }

  Widget _buildSidebarNavArea(List<_NavItem> navItems) {
    return Stack(
      children: [
        // Bug fix: search jumps need a mounted GlobalKey for the target nav row.
        // The settings nav is small, so building it as one scrollable column is
        // simpler and makes ensureVisible reliable for off-screen destinations.
        SizedBox.expand(
          key: _navViewportKey,
          child: SingleChildScrollView(
            key: const ValueKey(navListKey),
            controller: _navScrollController,
            padding: const EdgeInsets.symmetric(vertical: 4),
            child: Column(children: _buildNavTree(navItems)),
          ),
        ),
        Obx(() {
          if (!controller.settingSearchPanelVisible.value) {
            return const SizedBox.shrink();
          }

          // Feature: search results are a floating palette over the navigation
          // rail. Keeping the nav list in place avoids layout jumps while users type.
          return Positioned(left: 14, right: 14, top: 0, child: _buildSearchResultPanel());
        }),
      ],
    );
  }

  Widget _buildSearchResultPanel() {
    final results = controller.settingSearchResults;
    return Container(
      key: const ValueKey('settings-search-result-panel'),
      margin: const EdgeInsets.only(top: 2),
      constraints: const BoxConstraints(maxHeight: 280),
      decoration: BoxDecoration(
        // Bug fix: the floating palette sits directly over the navigation rail.
        // Some themes use translucent window backgrounds, so the panel forces
        // full opacity here to keep result text readable over menu items.
        color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor).withValues(alpha: 1),
        border: Border.all(color: getThemeSettingDividerColor()),
        borderRadius: BorderRadius.circular(6),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.12), blurRadius: 12, offset: const Offset(0, 6))],
      ),
      child:
          results.isEmpty
              ? Padding(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 14),
                child: Text(controller.tr('ui_setting_search_empty'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12.5)),
              )
              : ListView.builder(
                controller: controller.settingSearchResultScrollController,
                shrinkWrap: true,
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 6),
                itemCount: results.length,
                itemBuilder: (context, index) {
                  final result = results[index];
                  return Obx(() {
                    final isSelected = controller.selectedSettingSearchResultIndex.value == index;
                    return MouseRegion(
                      // Bug fix: rebuilt search rows can receive onEnter while
                      // the pointer is stationary, which overrides the "first
                      // result selected" reset after typing. Zero-delta hover
                      // events can also be emitted during rebuilds, so only a
                      // real pointer move should switch the active result.
                      onHover: (event) {
                        if (event.delta == Offset.zero) {
                          return;
                        }
                        controller.selectSettingSearchResult(index);
                      },
                      child: GestureDetector(
                        key: ValueKey(result.resultKey),
                        behavior: HitTestBehavior.translucent,
                        onTap: () => controller.activateSettingSearchResult(result),
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                          decoration: BoxDecoration(
                            // Feature: keyboard search now has a visible active row.
                            // The selected fill mirrors nav selection without changing
                            // row height, so arrowing through results stays stable.
                            color: isSelected ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.22 : 0.12) : Colors.transparent,
                            borderRadius: BorderRadius.circular(5),
                          ),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.center,
                            children: [
                              _buildSearchResultIcon(result, isSelected),
                              const SizedBox(width: 8),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(result.title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 12.5)),
                                    const SizedBox(height: 2),
                                    Text(
                                      '${_searchResultTypeLabel(result.type)} · ${result.subtitle}',
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: TextStyle(color: getThemeSubTextColor(), fontSize: 11),
                                    ),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    );
                  });
                },
              ),
    );
  }

  KeyEventResult _handleSearchKeyEvent(KeyEvent event) {
    if (!controller.settingSearchPanelVisible.value || event is! KeyDownEvent && event is! KeyRepeatEvent) {
      return KeyEventResult.ignored;
    }

    if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
      controller.moveSettingSearchSelection(1);
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
      controller.moveSettingSearchSelection(-1);
      return KeyEventResult.handled;
    }
    if (event.logicalKey == LogicalKeyboardKey.enter || event.logicalKey == LogicalKeyboardKey.numpadEnter) {
      unawaited(controller.activateSelectedSettingSearchResult());
      return KeyEventResult.handled;
    }

    return KeyEventResult.ignored;
  }

  bool _handleHardwareKeyboardEvent(KeyEvent event) {
    if (controller.settingSearchFocusNode.hasFocus && (event.logicalKey == LogicalKeyboardKey.arrowDown || event.logicalKey == LogicalKeyboardKey.arrowUp)) {
      return false;
    }
    if (controller.settingSearchFocusNode.hasFocus && event.logicalKey == LogicalKeyboardKey.escape) {
      // Settings search owns Escape while focused so the same key sequence
      // cannot also reach page-level shortcuts after clearing the text.
      return false;
    }
    if (event is KeyUpEvent && event.logicalKey == LogicalKeyboardKey.escape && _consumeSearchEscapeKeyUp) {
      return false;
    }
    return _handleSearchKeyEvent(event) == KeyEventResult.handled;
  }

  Widget _buildSearchResultIcon(WoxSettingSearchResult result, bool isSelected) {
    // Visual refinement: 24px keeps search result icons readable beside the
    // two-line text without making the navigation palette feel heavier.
    const iconSize = 24.0;
    final icon = result.icon;
    if ((result.type == WoxSettingSearchTargetType.installedPlugin || result.type == WoxSettingSearchTargetType.pluginSetting) &&
        icon != null &&
        icon.imageData.trim().isNotEmpty) {
      // Feature refinement: plugin-related search rows use the plugin's own
      // icon instead of a generic category icon, so plugin and plugin-setting
      // hits are visually tied to the destination users will open.
      return ClipRRect(
        key: ValueKey('settings-search-result-plugin-icon-${result.pluginId}-${result.settingKey}'),
        borderRadius: BorderRadius.circular(3),
        child: WoxImageView(woxImage: icon, width: iconSize, height: iconSize),
      );
    }

    return Icon(_searchResultIcon(result.type), color: isSelected ? getThemeTextColor() : getThemeSubTextColor(), size: iconSize);
  }

  bool _isSearchFocusShortcut(KeyEvent event) {
    return event.logicalKey == LogicalKeyboardKey.keyF && (HardwareKeyboard.instance.isControlPressed || HardwareKeyboard.instance.isMetaPressed);
  }

  void _focusSettingSearchFromShortcut() {
    controller.settingSearchFocusNode.requestFocus();
    final textLength = controller.settingSearchTextController.text.length;
    // Feature: Settings follows the common find shortcut. Selecting existing
    // text lets users immediately replace the current search without manually
    // clearing it first.
    controller.settingSearchTextController.selection = TextSelection(baseOffset: 0, extentOffset: textLength);
    controller.settingSearchPanelVisible.value = controller.settingSearchTextController.text.trim().isNotEmpty;
  }

  IconData _searchResultIcon(WoxSettingSearchTargetType type) {
    switch (type) {
      case WoxSettingSearchTargetType.builtInSetting:
        return Icons.tune_outlined;
      case WoxSettingSearchTargetType.installedPlugin:
        return Icons.extension_outlined;
      case WoxSettingSearchTargetType.pluginSetting:
        return Icons.settings_suggest_outlined;
    }
  }

  String _searchResultTypeLabel(WoxSettingSearchTargetType type) {
    switch (type) {
      case WoxSettingSearchTargetType.builtInSetting:
        return controller.tr('ui_setting_search_type_setting');
      case WoxSettingSearchTargetType.installedPlugin:
        return controller.tr('ui_setting_search_type_plugin');
      case WoxSettingSearchTargetType.pluginSetting:
        return controller.tr('ui_setting_search_type_plugin_setting');
    }
  }

  List<Widget> _buildNavTree(List<_NavItem> items) {
    final flatItems = _flattenNavItems(items);
    return flatItems.map((flatItem) {
      final item = flatItem.item;
      final isSelected = controller.activeNavPath.value == flatItem.path;
      final isParent = item.isParent;

      return Container(
        key: _getNavItemKey(flatItem.path),
        child: GestureDetector(
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
                Expanded(child: Text(item.title, style: TextStyle(color: getThemeTextColor(), fontSize: 13))),
                if (isParent) Icon(item.isExpanded ? Icons.expand_more : Icons.chevron_right, color: getThemeTextColor().withValues(alpha: 0.6), size: 18),
              ],
            ),
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
            _NavItem(id: 'themes.edit', icon: Icons.tune_outlined, title: controller.tr('ui_theme_editor_title'), body: const WoxSettingThemeEditorView()),
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
      _queueActiveNavItemVisibleForBuild();

      return WoxPlatformFocus(
        focusNode: controller.settingFocusNode,
        autofocus: true,
        onKeyEvent: (FocusNode node, KeyEvent event) {
          if (_isSearchFocusShortcut(event)) {
            if (event is KeyDownEvent || event is KeyRepeatEvent) {
              _focusSettingSearchFromShortcut();
            }
            return KeyEventResult.handled;
          }
          if (event.logicalKey == LogicalKeyboardKey.escape && controller.settingSearchFocusNode.hasFocus) {
            if ((event is KeyDownEvent || event is KeyRepeatEvent) && controller.settingSearchTextController.text.trim().isNotEmpty) {
              // Bug fix: Escape arrives as down/up. Clearing the text on key
              // down can move focus back to the page before the matching KeyUp,
              // so consume the release with a pending flag.
              _consumeSearchEscapeKeyUp = true;
              controller.clearSettingSearch();
              return KeyEventResult.handled;
            }
            if (event is KeyUpEvent && _consumeSearchEscapeKeyUp) {
              _consumeSearchEscapeKeyUp = false;
              return KeyEventResult.handled;
            }
          }
          if (event is KeyUpEvent && event.logicalKey == LogicalKeyboardKey.escape && _consumeSearchEscapeKeyUp) {
            // Bug fix: clearing settings search can move focus back to the page
            // before the matching KeyUp arrives. Consume that release by the
            // pending search-clear flag instead of requiring the search field to
            // still be focused.
            _consumeSearchEscapeKeyUp = false;
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
                child: Column(children: [_buildSearchBox(), Expanded(child: _buildSidebarNavArea(navItems)), const SizedBox(height: 8)]),
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

class _SettingSearchMoveIntent extends Intent {
  final int delta;

  const _SettingSearchMoveIntent(this.delta);
}
