import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_theme_icon_view.dart';
import 'package:wox/components/wox_theme_preview.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/strings.dart';

class WoxSettingThemeView extends GetView<WoxSettingController> {
  const WoxSettingThemeView({super.key});

  Widget themeList() {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 20),
          child: Obx(() {
            return WoxTextField(
              autofocus: true,
              hintText: Strings.format(controller.tr('ui_setting_theme_search_placeholder'), [controller.filteredThemeList.length]),
              suffixIcon: Padding(
                padding: const EdgeInsets.only(right: 8.0),
                child: Icon(Icons.search, color: getThemeTextColor()),
              ),
              onChanged: (value) => controller.onFilterThemes(value),
            );
          }),
        ),
        Expanded(
          child: Scrollbar(
            child: Obx(() {
              if (controller.filteredThemeList.isEmpty) {
                return Center(
                  child: Text(
                    controller.tr('ui_setting_theme_empty_data'),
                    style: TextStyle(
                      color: getThemeSubTextColor(),
                    ),
                  ),
                );
              }

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
                          color: isActive ? getThemeActiveBackgroundColor() : Colors.transparent,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: GestureDetector(
                          behavior: HitTestBehavior.translucent,
                          onTap: () {
                            controller.activeTheme.value = theme;
                          },
                          child: ListTile(
                            contentPadding: const EdgeInsets.only(left: 6, right: 6),
                            leading: WoxThemeIconView(theme: theme, width: 32, height: 32),
                            title: Text(theme.themeName,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(
                                  fontSize: 15,
                                  color: isActive ? getThemeActionItemActiveColor() : getThemeTextColor(),
                                )),
                            subtitle: Row(
                              mainAxisAlignment: MainAxisAlignment.start,
                              crossAxisAlignment: CrossAxisAlignment.center,
                              children: [
                                Text(
                                  theme.version,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(),
                                    fontSize: 12,
                                  ),
                                ),
                                const SizedBox(width: 10),
                                Text(
                                  theme.themeAuthor,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(
                                    color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(),
                                    fontSize: 12,
                                  ),
                                ),
                              ],
                            ),
                            trailing: themeTrailIcon(theme, isActive),
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

  Widget themeTrailIcon(WoxTheme theme, bool isActive) {
    if (controller.isStoreThemeList.value) {
      if (theme.isInstalled) {
        return Padding(
          padding: const EdgeInsets.only(right: 6),
          child: Icon(Icons.check_circle, size: 20, color: isActive ? getThemeActionItemActiveColor() : Colors.green),
        );
      }
    } else {
      if (theme.isSystem) {
        return Container(
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3),
            border: Border.all(
              color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(),
              width: 0.5,
            ),
          ),
          child: Text(
            controller.tr('ui_setting_theme_system_tag'),
            style: TextStyle(
              color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(),
              fontSize: 11,
              height: 1.1,
            ),
          ),
        );
      }
    }

    return const SizedBox();
  }

  Widget themeDetail() {
    return Expanded(
      child: Obx(() {
        final theme = controller.activeTheme.value;
        if (theme.themeId.isEmpty) {
          return Center(
            child: Text(
              controller.tr('ui_setting_theme_empty_data'),
              style: TextStyle(
                color: getThemeSubTextColor(),
              ),
            ),
          );
        }
        return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
          Padding(
            padding: const EdgeInsets.only(bottom: 8.0, left: 10),
            child: Row(
              children: [
                Padding(
                  padding: const EdgeInsets.only(left: 8.0),
                  child: Text(
                    theme.themeName,
                    style: TextStyle(
                      fontSize: 20,
                      color: getThemeTextColor(),
                    ),
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 10.0),
                  child: Text(
                    theme.version,
                    style: TextStyle(
                      color: getThemeSubTextColor(),
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
                Row(
                  children: [
                    Text(
                      theme.themeAuthor,
                      style: TextStyle(
                        color: getThemeSubTextColor(),
                      ),
                    ),
                  ],
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 18.0),
                  child: WoxButton.text(
                    text: controller.tr('ui_setting_theme_website'),
                    icon: Icon(
                      Icons.open_in_new,
                      size: 12,
                      color: getThemeTextColor(),
                    ),
                    onPressed: () {
                      controller.openPluginWebsite(theme.themeUrl);
                    },
                  ),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.only(bottom: 8.0, left: 16),
            child: Row(
              children: [
                if (!theme.isInstalled)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: WoxButton.primary(
                      text: controller.tr('ui_setting_theme_install'),
                      onPressed: () {
                        controller.installTheme(theme);
                      },
                    ),
                  ),
                if (theme.isInstalled || theme.isSystem)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: WoxButton.primary(
                      text: controller.tr('ui_setting_theme_apply'),
                      onPressed: controller.woxSetting.value.themeId == theme.themeId
                          ? null
                          : () {
                              controller.applyTheme(theme);
                            },
                    ),
                  ),
                if (theme.isInstalled && !theme.isSystem)
                  Padding(
                    padding: const EdgeInsets.only(right: 8.0),
                    child: WoxButton.primary(
                      text: controller.tr('ui_setting_theme_uninstall'),
                      onPressed: () {
                        controller.uninstallTheme(theme);
                      },
                    ),
                  ),
              ],
            ),
          ),
          Expanded(
            child: DefaultTabController(
              length: 2,
              child: Column(
                children: [
                  TabBar(
                    isScrollable: true,
                    tabAlignment: TabAlignment.start,
                    labelColor: getThemeTextColor(),
                    unselectedLabelColor: getThemeTextColor(),
                    indicatorColor: getThemeActiveBackgroundColor(),
                    tabs: [
                      Tab(
                        child: Text(controller.tr('ui_setting_theme_preview'), style: TextStyle(color: getThemeTextColor())),
                      ),
                      Tab(
                        child: Text(controller.tr('ui_setting_theme_description'), style: TextStyle(color: getThemeTextColor())),
                      ),
                    ],
                  ),
                  Expanded(
                    child: TabBarView(
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
      return Text(controller.tr('ui_setting_theme_empty_data'));
    }

    return WoxThemePreview(theme: theme);
  }

  Widget themeTabDescription() {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            controller.activeTheme.value.description,
            style: TextStyle(
              color: getThemeTextColor(),
            ),
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
            width: 260,
            child: themeList(),
          ),
          // This is your divider
          Container(
            width: 1,
            height: double.infinity,
            color: getThemeDividerColor(),
            margin: const EdgeInsets.only(right: 10, left: 10),
          ),
          themeDetail(),
        ],
      ),
    );
  }
}
