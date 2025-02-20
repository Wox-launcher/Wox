import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/material.dart' as base;
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_theme_icon_view.dart';
import 'package:wox/components/wox_theme_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/strings.dart';

class WoxSettingThemeView extends GetView<WoxSettingController> {
  const WoxSettingThemeView({super.key});

  Widget themeList() {
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
                placeholder: Strings.format(controller.tr('setting_theme_search_placeholder'), [controller.filteredThemeList.length]),
                padding: const EdgeInsets.all(10),
                suffix: const Padding(
                  padding: EdgeInsets.only(right: 8.0),
                  child: Icon(FluentIcons.search),
                ),
                onChanged: (value) => {controller.onFilterThemes(value)},
              );
            }),
          ),
        ),
        Expanded(
          child: base.Scrollbar(
            child: Obx(() {
              return ListView.builder(
                itemCount: controller.filteredThemeList.length,
                itemBuilder: (context, index) {
                  final theme = controller.filteredThemeList[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8.0),
                    child: Obx(() {
                      final isActive = controller.activeTheme.value.themeId == theme.themeId;
                      return Container(
                        decoration: BoxDecoration(
                          color: isActive ? SettingPrimaryColor : base.Colors.transparent,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: GestureDetector(
                          behavior: HitTestBehavior.translucent,
                          onTap: () {
                            controller.activeTheme.value = theme;
                          },
                          child: base.ListTile(
                            leading: WoxThemeIconView(theme: theme, width: 32, height: 32),
                            title: Text(theme.themeName,
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
                                  theme.version,
                                  maxLines: 1, // Limiting the description to two lines
                                  overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                                  style: TextStyle(
                                    color: isActive ? base.Colors.white : base.Colors.grey,
                                    fontSize: 12,
                                  ),
                                ),
                                const base.SizedBox(width: 10),
                                Text(
                                  theme.themeAuthor,
                                  maxLines: 1, // Limiting the description to two lines
                                  overflow: TextOverflow.ellipsis, // Add ellipsis for overflow
                                  style: TextStyle(
                                    color: isActive ? base.Colors.white : base.Colors.grey,
                                    fontSize: 12,
                                  ),
                                ),
                              ],
                            ),
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

  Widget themeDetail() {
    return Expanded(
      child: Obx(() {
        final theme = controller.activeTheme.value;
        return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Padding(
            padding: const EdgeInsets.only(bottom: 8.0, left: 10),
            child: Row(
              children: [
                Padding(
                  padding: const EdgeInsets.only(left: 8.0),
                  child: Text(
                    theme.themeName,
                    style: const TextStyle(
                      fontSize: 20,
                    ),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 10.0),
                  child: Text(
                    theme.version,
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
                  theme.themeAuthor,
                  style: const TextStyle(
                    color: base.Colors.grey,
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 18.0),
                  child: HyperlinkButton(
                    onPressed: () {
                      controller.openPluginWebsite(theme.themeUrl);
                    },
                    child: base.Row(
                      children: [
                        Text(
                          controller.tr('setting_theme_website'),
                          style: const TextStyle(
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
                if (theme.isInstalled && !theme.isSystem)
                  base.Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.uninstallTheme(theme);
                      },
                      child: Text(controller.tr('setting_theme_uninstall')),
                    ),
                  ),
                if (!theme.isInstalled)
                  base.Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: Button(
                      onPressed: () {
                        controller.installTheme(theme);
                      },
                      child: Text(controller.tr('setting_theme_install')),
                    ),
                  ),
              ],
            ),
          ),
          Expanded(
            child: base.DefaultTabController(
              length: 2,
              child: Column(
                children: [
                  base.TabBar(
                    isScrollable: true,
                    tabAlignment: base.TabAlignment.start,
                    labelColor: SettingPrimaryColor,
                    indicatorColor: SettingPrimaryColor,
                    tabs: [
                      base.Tab(
                        child: Text(controller.tr('setting_theme_preview')),
                      ),
                      base.Tab(
                        child: Text(controller.tr('setting_theme_description')),
                      ),
                    ],
                  ),
                  Expanded(
                    child: base.TabBarView(
                      children: [
                        themeTabPreview(theme),
                        themeTabDescription(),
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

  Widget themeTabPreview(WoxTheme theme) {
    if (theme.themeId.isEmpty) {
      return Text(controller.tr('setting_theme_empty_data'));
    }

    return WoxThemePreview(theme: theme);
  }

  Widget themeTabDescription() {
    return base.Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            controller.activeTheme.value.description,
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
            child: themeList(),
          ),
          // This is your divider
          Container(
            width: 1,
            height: double.infinity,
            color: Colors.grey[30],
            margin: const EdgeInsets.only(right: 10, left: 10),
          ),
          themeDetail(),
        ],
      ),
    );
  }
}
