import 'dart:convert';
import 'dart:math' as math;

import 'package:dynamic_tabbar/dynamic_tabbar.dart' as dt;
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/wox_plugin_detail_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_ai_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/wox_hint_box.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/components/wox_label.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_newline.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/components/plugin/wox_setting_plugin_checkbox_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_textbox_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/strings.dart';
import 'package:wox/utils/wox_text_measure_util.dart';
import 'package:wox/enums/wox_plugin_runtime_enum.dart';

class WoxSettingPluginView extends GetView<WoxSettingController> {
  const WoxSettingPluginView({super.key});
  // Local refreshing state for showing loading spinner on refresh button
  static final RxBool _refreshing = false.obs;
  static final GlobalKey _pluginFilterIconKey = GlobalKey();
  static const double _pluginLabelMinWidth = 0;
  static const double _pluginTableDefaultWidth = 626;
  static const double _pluginFilterPanelDefaultWidth = 660;

  String _extractSettingLabelText(dynamic settingDefinitionValue, String settingType) {
    if (settingType == "checkbox") {
      return (settingDefinitionValue as PluginSettingValueCheckBox).label;
    }
    if (settingType == "textbox") {
      return (settingDefinitionValue as PluginSettingValueTextBox).label;
    }
    if (settingType == "select") {
      return (settingDefinitionValue as PluginSettingValueSelect).label;
    }
    if (settingType == "selectAIModel") {
      return (settingDefinitionValue as PluginSettingValueSelectAIModel).label;
    }
    if (settingType == "table") {
      return (settingDefinitionValue as PluginSettingValueTable).title;
    }

    return "";
  }

  double _measureLabelWidthByText(BuildContext context, String label, {double minWidth = _pluginLabelMinWidth}) {
    final trimmedLabel = label.trim();
    final preferredWidth = WoxTextMeasureUtil.measureTextWidth(context: context, text: trimmedLabel, style: const TextStyle(fontSize: 13)) + 8;
    return preferredWidth.clamp(minWidth, PLUGIN_SETTING_LABEL_MAX_WIDTH).toDouble();
  }

  double _calculateUniformPluginLabelWidth(BuildContext context, PluginDetail plugin) {
    double maxLabelWidth = 0;

    for (final definition in plugin.settingDefinitions) {
      final rawLabel = _extractSettingLabelText(definition.value, definition.type).trim();
      if (rawLabel.isEmpty) {
        continue;
      }

      final translatedLabel = controller.tr(rawLabel).trim();
      if (translatedLabel.isEmpty) {
        continue;
      }

      maxLabelWidth = math.max(maxLabelWidth, _measureLabelWidthByText(context, translatedLabel, minWidth: 0));
    }

    return maxLabelWidth.clamp(_pluginLabelMinWidth, PLUGIN_SETTING_LABEL_MAX_WIDTH).toDouble();
  }

  Future<void> _showPluginFilterPanel(BuildContext context) async {
    final filterIconContext = _pluginFilterIconKey.currentContext;
    if (filterIconContext == null) {
      return;
    }

    final RenderBox overlay = Overlay.of(context).context.findRenderObject() as RenderBox;
    final RenderBox button = filterIconContext.findRenderObject() as RenderBox;
    final Offset buttonTopLeft = button.localToGlobal(Offset.zero, ancestor: overlay);

    final Size screenSize = overlay.size;
    const double panelMinWidth = 360;
    double left = buttonTopLeft.dx;
    double panelWidth = math.min(_pluginFilterPanelDefaultWidth, screenSize.width - left - 12);
    if (panelWidth < panelMinWidth) {
      panelWidth = math.min(_pluginFilterPanelDefaultWidth, screenSize.width - 24);
      left = left.clamp(12.0, screenSize.width - panelWidth - 12.0);
    }

    final double panelHeight = 190;

    double top = buttonTopLeft.dy + button.size.height + 8;
    top = top.clamp(12.0, screenSize.height - panelHeight - 12.0);

    await showGeneralDialog(
      context: context,
      barrierDismissible: true,
      barrierLabel: 'plugin_filter_panel',
      barrierColor: Colors.transparent,
      transitionDuration: Duration.zero,
      pageBuilder: (dialogContext, animation, secondaryAnimation) {
        return Material(
          color: Colors.transparent,
          child: Stack(
            children: [
              Positioned.fill(child: GestureDetector(behavior: HitTestBehavior.translucent, onTap: () => Navigator.of(dialogContext).maybePop(), child: const SizedBox.expand())),
              Positioned(left: left, top: top, child: _PluginFilterPanel(controller: controller, width: panelWidth)),
            ],
          ),
        );
      },
    );
  }

  Widget pluginList(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: 20),
          child: Obx(() {
            return WoxTextField(
              autofocus: true,
              controller: controller.filterPluginKeywordController,
              hintText: Strings.format(controller.tr('ui_search_plugins'), [controller.filteredPluginList.length]),
              contentPadding: const EdgeInsets.all(10),
              suffixIcon: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Obx(() {
                    if (_refreshing.value) {
                      return const Padding(padding: EdgeInsets.symmetric(horizontal: 4.0), child: WoxLoadingIndicator(size: 16));
                    }

                    final Color iconColor = controller.hasPluginFilterApplied ? getThemeActiveBackgroundColor() : getThemeSubTextColor();
                    return Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Padding(
                          padding: const EdgeInsets.only(right: 2),
                          child: GestureDetector(
                            key: _pluginFilterIconKey,
                            onTap: () => _showPluginFilterPanel(context),
                            child: Padding(padding: const EdgeInsets.symmetric(horizontal: 4.0), child: Icon(Icons.filter_alt_outlined, color: iconColor)),
                          ),
                        ),
                        GestureDetector(
                          onTap: () async {
                            _refreshing.value = true;
                            try {
                              final traceId = const UuidV4().generate();
                              final preserveKeyword = controller.filterPluginKeywordController.text;
                              final preserveActiveId = controller.activePlugin.value.id;
                              final isStore = controller.isStorePluginList.value;

                              if (isStore) {
                                await controller.loadStorePlugins(traceId);
                                await controller.switchToPluginList(traceId, true);
                              } else {
                                await controller.loadInstalledPlugins(traceId);
                                await controller.switchToPluginList(traceId, false);
                              }

                              // restore filter keyword and re-filter
                              controller.filterPluginKeywordController.text = preserveKeyword;
                              controller.filterPlugins();
                              controller.syncActivePluginWithFilteredList(currentActivePluginId: preserveActiveId);
                            } finally {
                              _refreshing.value = false;
                            }
                          },
                          child: Padding(padding: const EdgeInsets.symmetric(horizontal: 4.0), child: Icon(Icons.refresh, color: getThemeSubTextColor())),
                        ),
                      ],
                    );
                  }),
                ],
              ),
              onChanged: (value) {
                controller.filterPlugins();
                controller.syncActivePluginWithFilteredList();
              },
            );
          }),
        ),
        Expanded(
          child: Scrollbar(
            thumbVisibility: false,
            child: Obx(() {
              if (controller.filteredPluginList.isEmpty) {
                return Center(child: Text(controller.tr('ui_setting_plugin_empty_data'), style: TextStyle(color: getThemeSubTextColor())));
              }

              return ListView.builder(
                primary: true,
                itemCount: controller.filteredPluginList.length,
                itemBuilder: (context, index) {
                  final plugin = controller.filteredPluginList[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8.0),
                    child: Obx(() {
                      final isActive = controller.activePlugin.value.id == plugin.id;
                      return Container(
                        decoration: BoxDecoration(color: isActive ? getThemeActiveBackgroundColor() : Colors.transparent, borderRadius: BorderRadius.circular(4)),
                        child: GestureDetector(
                          behavior: HitTestBehavior.translucent,
                          onTap: () {
                            controller.activePlugin.value = plugin;
                          },
                          child: ListTile(
                            contentPadding: const EdgeInsets.only(left: 6, right: 6),
                            leading: WoxImageView(woxImage: plugin.icon, width: 32),
                            title: Text(
                              plugin.name,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(fontSize: 15, color: isActive ? getThemeActionItemActiveColor() : getThemeTextColor()),
                            ),
                            subtitle: Row(
                              mainAxisAlignment: MainAxisAlignment.start,
                              crossAxisAlignment: CrossAxisAlignment.center,
                              children: [
                                Text(
                                  plugin.version,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: TextStyle(color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(), fontSize: 12),
                                ),
                                const SizedBox(width: 10),
                                Flexible(
                                  child: Text(
                                    plugin.author,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: TextStyle(color: isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor(), fontSize: 12),
                                  ),
                                ),
                              ],
                            ),
                            trailing: pluginTrailIcon(plugin, isActive),
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

  Widget pluginTrailIcon(PluginDetail plugin, bool isActive) {
    // align tags/icons to the right of the tile
    final Color borderColor = isActive ? getThemeActionItemActiveColor() : getThemeSubTextColor();

    final List<Widget> rightItems = [];

    void addTag(String text) {
      rightItems.add(
        Container(
          margin: EdgeInsets.only(left: rightItems.isEmpty ? 0 : 8),
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 1),
          decoration: BoxDecoration(borderRadius: BorderRadius.circular(3), border: Border.all(color: borderColor, width: 0.5)),
          child: Text(text, style: TextStyle(color: borderColor, fontSize: 11, height: 1.1)),
        ),
      );
    }

    // Script tag (non-system script plugins)
    if (!plugin.isSystem && WoxPluginRuntimeEnum.equals(plugin.runtime, WoxPluginRuntimeEnum.SCRIPT)) {
      addTag(controller.tr('ui_setting_plugin_script_tag'));
    }

    // System tag
    if (plugin.isSystem) {
      addTag(controller.tr('ui_setting_plugin_system_tag'));
    }

    // Installed list tags
    if (!controller.isStorePluginList.value && plugin.isUpgradable) {
      addTag(controller.tr('plugin_wpm_upgrade'));
    }
    if (!controller.isStorePluginList.value && plugin.isDisable) {
      addTag(controller.tr('ui_disabled'));
    }

    // Store list: show installed check icon
    if (controller.isStorePluginList.value && plugin.isInstalled) {
      rightItems.add(
        Padding(padding: const EdgeInsets.only(right: 6, left: 4), child: Icon(Icons.check_circle, size: 20, color: isActive ? getThemeActionItemActiveColor() : Colors.green)),
      );
    }

    if (rightItems.isEmpty) {
      return const SizedBox();
    }

    return Row(mainAxisSize: MainAxisSize.min, children: rightItems);
  }

  Widget pluginDetail(BuildContext context) {
    return Expanded(
      child: Obx(() {
        if (controller.activePlugin.value.id.isEmpty) {
          return Center(child: Text(controller.tr('ui_setting_plugin_empty_data'), style: TextStyle(color: getThemeSubTextColor())));
        }

        final plugin = controller.activePlugin.value;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.only(bottom: 8.0, left: 10),
              child: Row(
                children: [
                  WoxImageView(woxImage: plugin.icon, width: 32),
                  Padding(padding: const EdgeInsets.only(left: 8.0), child: Text(plugin.name, style: TextStyle(fontSize: 20, color: getThemeTextColor()))),
                  Padding(padding: const EdgeInsets.only(left: 10.0), child: Text(plugin.version, style: TextStyle(color: getThemeSubTextColor()))),
                  if (plugin.isDev)
                    // dev tag, warning color with warning border
                    Padding(
                      padding: const EdgeInsets.only(left: 10.0),
                      child: Container(
                        padding: const EdgeInsets.all(4),
                        decoration: BoxDecoration(color: getThemeSubTextColor(), border: Border.all(color: getThemeSubTextColor()), borderRadius: BorderRadius.circular(4)),
                        child: Text(controller.tr('ui_plugin_dev_tag'), style: const TextStyle(color: Colors.white, fontSize: 12)),
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
                  Text(plugin.author, style: TextStyle(color: getThemeSubTextColor())),
                  Padding(
                    padding: const EdgeInsets.only(left: 18.0),
                    child: WoxButton.text(
                      text: controller.tr('ui_plugin_website'),
                      icon: Icon(Icons.open_in_new, size: 12, color: getThemeTextColor()),
                      onPressed: () {
                        controller.openPluginWebsite(plugin.website);
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
                  if (plugin.isInstalled && plugin.isUpgradable)
                    Padding(
                      padding: const EdgeInsets.only(right: 8.0),
                      child: Obx(
                        () => WoxButton.secondary(
                          text: controller.tr('plugin_wpm_upgrade'),
                          icon:
                              controller.isUpgradingPlugin.value
                                  ? WoxLoadingIndicator(size: 16, color: getThemeActionItemActiveColor())
                                  : Icon(Icons.system_update_alt, size: 14, color: getThemeTextColor()),
                          onPressed:
                              controller.isUpgradingPlugin.value
                                  ? null
                                  : () {
                                    controller.upgradePlugin(plugin);
                                  },
                        ),
                      ),
                    ),
                  if (plugin.isInstalled && !plugin.isSystem)
                    Padding(
                      padding: const EdgeInsets.only(right: 8.0),
                      child: WoxButton.secondary(
                        text: controller.tr('ui_plugin_uninstall'),
                        onPressed: () {
                          controller.uninstallPlugin(plugin);
                        },
                      ),
                    ),
                  if (!plugin.isInstalled)
                    Padding(
                      padding: const EdgeInsets.only(right: 8.0),
                      child: Obx(
                        () => WoxButton.secondary(
                          text: controller.isInstallingPlugin.value ? controller.tr("ui_plugin_installing") : controller.tr('ui_plugin_install'),
                          icon: controller.isInstallingPlugin.value ? WoxLoadingIndicator(size: 16, color: getThemeActionItemActiveColor()) : null,
                          onPressed:
                              controller.isInstallingPlugin.value
                                  ? null
                                  : () {
                                    controller.installPlugin(plugin);
                                  },
                        ),
                      ),
                    ),
                  if (plugin.isInstalled && !plugin.isDisable)
                    Padding(
                      padding: const EdgeInsets.only(right: 8.0),
                      child: WoxButton.secondary(
                        text: controller.tr('ui_plugin_disable'),
                        onPressed: () {
                          controller.disablePlugin(plugin);
                        },
                      ),
                    ),
                  if (plugin.isInstalled && plugin.isDisable)
                    Padding(
                      padding: const EdgeInsets.only(right: 8.0),
                      child: WoxButton.secondary(
                        text: controller.tr('ui_plugin_enable'),
                        onPressed: () {
                          controller.enablePlugin(plugin);
                        },
                      ),
                    ),
                  if (plugin.isInstalled && !plugin.isSystem)
                    Padding(
                      padding: const EdgeInsets.only(right: 8.0),
                      child: WoxButton.secondary(
                        text: controller.tr('ui_plugin_open_directory'),
                        icon: Icon(Icons.folder_open, size: 14, color: getThemeTextColor()),
                        onPressed:
                            plugin.pluginDirectory.isEmpty
                                ? null
                                : () {
                                  controller.openPluginDirectory(plugin);
                                },
                      ),
                    ),
                ],
              ),
            ),
            Obx(() {
              if (controller.pluginInstallError.value.isNotEmpty) {
                return Container(
                  margin: const EdgeInsets.only(left: 16, right: 16, top: 8),
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Colors.red.withValues(alpha: 0.1),
                    border: Border.all(color: Colors.red.withValues(alpha: 0.3)),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Row(
                    children: [
                      Icon(Icons.error_outline, color: Colors.red, size: 20),
                      const SizedBox(width: 8),
                      Expanded(child: Text(controller.pluginInstallError.value, style: TextStyle(color: Colors.red, fontSize: 13))),
                      IconButton(
                        icon: Icon(Icons.close, size: 18, color: Colors.red),
                        padding: EdgeInsets.zero,
                        constraints: BoxConstraints(),
                        onPressed: () {
                          controller.pluginInstallError.value = '';
                        },
                      ),
                    ],
                  ),
                );
              }
              return const SizedBox.shrink();
            }),
            Expanded(
              child: dt.DynamicTabBarWidget(
                isScrollable: true,
                showBackIcon: false,
                showNextIcon: false,
                physics: const NeverScrollableScrollPhysics(),
                physicsTabBarView: const NeverScrollableScrollPhysics(),
                tabAlignment: TabAlignment.start,
                onAddTabMoveTo: dt.MoveToTab.idol,
                labelColor: getThemeTextColor(),
                unselectedLabelColor: getThemeTextColor(),
                indicatorColor: getThemeActiveBackgroundColor(),
                dynamicTabs:
                    controller.activeNavPath.value == 'plugins.installed'
                        ? [
                          dt.TabData(
                            index: 0,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_settings'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabSetting(context),
                          ),
                          dt.TabData(
                            index: 1,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_trigger_keywords'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabTriggerKeywords(),
                          ),
                          dt.TabData(
                            index: 2,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_commands'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabCommand(),
                          ),
                          dt.TabData(
                            index: 3,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_description'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabDescription(),
                          ),
                          dt.TabData(
                            index: 4,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_privacy'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabPrivacy(),
                          ),
                        ]
                        : [
                          // For uninstalled plugins: Description tab first
                          dt.TabData(
                            index: 0,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_description'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabDescription(),
                          ),
                          dt.TabData(
                            index: 1,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_trigger_keywords'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabTriggerKeywords(),
                          ),
                          dt.TabData(
                            index: 2,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_commands'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabCommand(),
                          ),
                          dt.TabData(
                            index: 3,
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_privacy'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabPrivacy(),
                          ),
                        ],
                onTabControllerUpdated: (tabController) {
                  controller.activePluginTabController = tabController;
                  tabController.index = 0;
                },
                onTabChanged: (index) {},
              ),
            ),
          ],
        );
      }),
    );
  }

  Widget pluginTabDescription() {
    // Convert PluginDetail to JSON format expected by WoxPluginDetailView
    final pluginData = {
      'Id': controller.activePlugin.value.id,
      'Name': controller.activePlugin.value.name,
      'Description': controller.activePlugin.value.description,
      'Author': controller.activePlugin.value.author,
      'Version': controller.activePlugin.value.version,
      'Website': controller.activePlugin.value.website,
      'Runtime': controller.activePlugin.value.runtime,
      'ScreenshotUrls': controller.activePlugin.value.screenshotUrls,
    };

    return WoxPluginDetailView(pluginDetailJson: jsonEncode(pluginData));
  }

  Widget pluginTabSetting(BuildContext context) {
    return Obx(() {
      var plugin = controller.activePlugin.value;
      final uniformLabelWidth = _calculateUniformPluginLabelWidth(context, plugin);

      // Show empty state if no settings
      if (plugin.settingDefinitions.isEmpty) {
        return Padding(
          padding: const EdgeInsets.all(16),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(controller.tr('ui_plugin_no_settings'), style: TextStyle(color: getThemeTextColor()))]),
        );
      }

      return Padding(
        padding: const EdgeInsets.all(16.0),
        child: SingleChildScrollView(
          child: Wrap(
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              ...plugin.settingDefinitions.map((e) {
                if (e.type == "checkbox") {
                  return WoxSettingPluginCheckbox(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueCheckBox,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                }
                if (e.type == "textbox") {
                  return WoxSettingPluginTextBox(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueTextBox,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                }
                if (e.type == "newline") {
                  return WoxSettingPluginNewLine(value: "", item: e.value as PluginSettingValueNewLine, labelWidth: uniformLabelWidth, onUpdate: (key, value) async => null);
                }
                if (e.type == "select") {
                  return WoxSettingPluginSelect(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueSelect,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                }
                if (e.type == "selectAIModel") {
                  return WoxSettingPluginSelectAIModel(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueSelectAIModel,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                }
                if (e.type == "head") {
                  return WoxSettingPluginHead(value: "", item: e.value as PluginSettingValueHead, labelWidth: uniformLabelWidth, onUpdate: (key, value) async => null);
                }
                if (e.type == "label") {
                  return WoxSettingPluginLabel(value: "", item: e.value as PluginSettingValueLabel, labelWidth: uniformLabelWidth, onUpdate: (key, value) async => null);
                }
                if (e.type == "table") {
                  return WoxSettingPluginTable(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueTable,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                }

                return Text(e.type);
              }),
            ],
          ),
        ),
      );
    });
  }

  Widget pluginTabTriggerKeywords() {
    var plugin = controller.activePlugin.value;
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          WoxHintBox(text: controller.tr('ui_plugin_trigger_keywords_tip')),
          const SizedBox(height: 12),
          if (plugin.triggerKeywords.isEmpty)
            Text(controller.tr('ui_plugin_no_trigger_keywords'), style: TextStyle(color: getThemeTextColor()))
          else
            WoxSettingPluginTable(
              value: json.encode(plugin.triggerKeywords.map((e) => {"keyword": e}).toList()),
              tableWidth: _pluginTableDefaultWidth,
              item: PluginSettingValueTable.fromJson({
                "Key": "_triggerKeywords",
                "Columns": [
                  {
                    "Key": "keyword",
                    "Label": controller.tr('ui_plugin_trigger_keyword_column'),
                    "Tooltip": controller.tr('ui_plugin_trigger_keyword_tooltip'),
                    "Type": "text",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"},
                    ],
                  },
                ],
                "SortColumnKey": "keyword",
              }),
              onUpdate: (key, value) async {
                final List<String> triggerKeywords = [];
                for (var item in json.decode(value)) {
                  triggerKeywords.add(item["keyword"]);
                }
                plugin.triggerKeywords = triggerKeywords;
                return controller.updatePluginSetting(plugin.id, "TriggerKeywords", triggerKeywords.join(","));
              },
            ),
        ],
      ),
    );
  }

  Widget pluginTabCommand() {
    var plugin = controller.activePlugin.value;
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          WoxHintBox(text: controller.tr('ui_plugin_commands_tip')),
          const SizedBox(height: 12),
          if (plugin.commands.isEmpty)
            Text(controller.tr('ui_plugin_no_commands'), style: TextStyle(color: getThemeTextColor()))
          else
            WoxSettingPluginTable(
              value: json.encode(plugin.commands),
              tableWidth: _pluginTableDefaultWidth,
              readonly: true,
              labelWidth: 160,
              item: PluginSettingValueTable.fromJson({
                "Key": "_commands",
                "Columns": [
                  {
                    "Key": "Command",
                    "Label": controller.tr('ui_plugin_command_name_column'),
                    "Width": 120,
                    "Type": "text",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"},
                    ],
                  },
                  {
                    "Key": "Description",
                    "Label": controller.tr('ui_plugin_command_desc_column'),
                    "Type": "text",
                    "TextMaxLines": 1,
                    "Validators": [
                      {"Type": "not_empty"},
                    ],
                  },
                ],
                "SortColumnKey": "Command",
              }),
              onUpdate: (key, value) async => null,
            ),
        ],
      ),
    );
  }

  Widget pluginTabPrivacy() {
    var plugin = controller.activePlugin.value;
    var noDataAccess = Padding(padding: const EdgeInsets.all(16), child: Text(controller.tr('ui_plugin_no_data_access'), style: TextStyle(color: getThemeTextColor())));

    if (plugin.features.isEmpty) {
      return noDataAccess;
    }

    List<String> params = [];

    //check if "queryEnv" feature is exist and list it's params
    var queryEnv = plugin.features.where((element) => element.name == "queryEnv").toList();
    if (queryEnv.isNotEmpty) {
      queryEnv.first.params.forEach((key, value) {
        if (value is bool && value) {
          params.add(key);
        }
        if (value is String && value.isNotEmpty && value.toLowerCase() == "true") {
          params.add(key);
        }
      });
    }

    // check if llmChat feature is exist
    var llmChat = plugin.features.where((element) => element.name == "llm").toList();
    if (llmChat.isNotEmpty) {
      params.add("llm");
    }

    if (params.isEmpty) {
      return noDataAccess;
    }

    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Expanded(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(controller.tr('ui_plugin_data_access_title'), style: TextStyle(color: getThemeTextColor())),
            ...params.map((e) {
              if (e == "requireActiveWindowName") {
                return privacyItem(Icons.window, controller.tr('ui_plugin_privacy_window_name'), controller.tr('ui_plugin_privacy_window_name_desc'));
              }
              if (e == "requireActiveWindowPid") {
                return privacyItem(Icons.window, controller.tr('ui_plugin_privacy_window_pid'), controller.tr('ui_plugin_privacy_window_pid_desc'));
              }
              if (e == "requireActiveBrowserUrl") {
                return privacyItem(Icons.web_sharp, controller.tr('ui_plugin_privacy_browser_url'), controller.tr('ui_plugin_privacy_browser_url_desc'));
              }
              if (e == "llm") {
                return privacyItem(Icons.chat, controller.tr('ui_plugin_privacy_llm'), controller.tr('ui_plugin_privacy_llm_desc'));
              }
              return Text(e);
            }),
          ],
        ),
      ),
    );
  }

  Widget privacyItem(IconData icon, String title, String description) {
    return Padding(
      padding: const EdgeInsets.only(top: 20.0),
      child: Column(
        children: [
          Row(children: [Icon(icon, color: getThemeTextColor()), const SizedBox(width: 10), Text(title, style: TextStyle(color: getThemeTextColor()))]),
          const SizedBox(height: 6),
          Row(children: [const SizedBox(width: 30), Flexible(child: Text(description, style: TextStyle(color: getThemeSubTextColor())))]),
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
          SizedBox(width: 260, child: pluginList(context)),
          Container(width: 1, height: double.infinity, color: getThemeDividerColor(), margin: const EdgeInsets.only(right: 10, left: 10)),
          pluginDetail(context),
        ],
      ),
    );
  }
}

class _PluginFilterPanel extends StatefulWidget {
  final WoxSettingController controller;
  final double width;

  const _PluginFilterPanel({required this.controller, required this.width});

  @override
  State<_PluginFilterPanel> createState() => _PluginFilterPanelState();
}

class _PluginFilterPanelState extends State<_PluginFilterPanel> {
  static const double _labelColumnMinWidth = 50;
  static const double _labelColumnMaxWidth = 180;

  late final FocusNode _focusNode;
  bool _focusReady = false;

  @override
  void initState() {
    super.initState();
    _focusNode = FocusNode();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _focusNode.requestFocus();
        _focusReady = true;
      }
    });
  }

  @override
  void dispose() {
    _focusNode.dispose();
    super.dispose();
  }

  double _measureLabelWidth(BuildContext context, String label) {
    return WoxTextMeasureUtil.measureTextWidth(context: context, text: label, style: TextStyle(color: getThemeTextColor(), fontSize: 13));
  }

  double _calculateLabelColumnWidth(BuildContext context, {required bool isStorePluginList}) {
    final labels = <String>[
      if (isStorePluginList) widget.controller.tr('ui_not_installed'),
      if (!isStorePluginList) widget.controller.tr('ui_plugin_filter_disabled_only'),
      if (!isStorePluginList) widget.controller.tr('ui_plugin_filter_enabled_only'),
      if (!isStorePluginList) widget.controller.tr('ui_plugin_filter_upgradable'),
      widget.controller.tr('ui_runtime_status'),
    ];

    double width = _labelColumnMinWidth;
    for (final label in labels) {
      width = math.max(width, _measureLabelWidth(context, label));
    }

    return width.clamp(_labelColumnMinWidth, _labelColumnMaxWidth).toDouble();
  }

  Widget _buildBooleanFilterRow({required String label, required double labelWidth, required bool value, required ValueChanged<bool?> onChanged}) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        WoxLabel(label: label, width: labelWidth, textAlign: TextAlign.left, style: TextStyle(color: getThemeTextColor(), fontSize: 13)),
        const SizedBox(width: 10),
        WoxCheckbox(value: value, onChanged: onChanged, size: 18),
      ],
    );
  }

  Widget _buildRuntimeFilterOption({required bool value, required String label, required ValueChanged<bool?> onChanged}) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [WoxCheckbox(value: value, onChanged: onChanged, size: 18), const SizedBox(width: 4), Text(label, style: TextStyle(color: getThemeTextColor(), fontSize: 13))],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final isStorePluginList = widget.controller.isStorePluginList.value;
      final labelColumnWidth = _calculateLabelColumnWidth(context, isStorePluginList: isStorePluginList);

      return Material(
        color: getThemeBackgroundColor().withAlpha(255),
        elevation: 8,
        borderRadius: BorderRadius.circular(8),
        child: Container(
          width: widget.width,
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
          decoration: BoxDecoration(color: getThemeBackgroundColor().withAlpha(255), borderRadius: BorderRadius.circular(8), border: Border.all(color: getThemeDividerColor())),
          child: Focus(
            focusNode: _focusNode,
            autofocus: true,
            onFocusChange: (hasFocus) {
              if (_focusReady && !hasFocus && mounted) {
                Navigator.of(context).maybePop();
              }
            },
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (isStorePluginList)
                  _buildBooleanFilterRow(
                    label: widget.controller.tr('ui_not_installed'),
                    labelWidth: labelColumnWidth,
                    value: widget.controller.filterUninstalledPluginsOnly.value,
                    onChanged: (value) => widget.controller.updatePluginFilters(uninstalledOnly: value ?? false),
                  ),
                if (!isStorePluginList)
                  _buildBooleanFilterRow(
                    label: widget.controller.tr('ui_plugin_filter_disabled_only'),
                    labelWidth: labelColumnWidth,
                    value: widget.controller.filterDisabledPluginsOnly.value,
                    onChanged: (value) => widget.controller.updatePluginFilters(disabledOnly: value ?? false),
                  ),
                if (!isStorePluginList) const SizedBox(height: 10),
                if (!isStorePluginList)
                  _buildBooleanFilterRow(
                    label: widget.controller.tr('ui_plugin_filter_enabled_only'),
                    labelWidth: labelColumnWidth,
                    value: widget.controller.filterEnabledPluginsOnly.value,
                    onChanged: (value) => widget.controller.updatePluginFilters(enabledOnly: value ?? false),
                  ),
                if (!isStorePluginList) const SizedBox(height: 10),
                if (!isStorePluginList)
                  _buildBooleanFilterRow(
                    label: widget.controller.tr('ui_plugin_filter_upgradable'),
                    labelWidth: labelColumnWidth,
                    value: widget.controller.filterUpgradablePluginsOnly.value,
                    onChanged: (value) => widget.controller.updatePluginFilters(upgradableOnly: value ?? false),
                  ),
                const SizedBox(height: 10),
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    WoxLabel(
                      label: widget.controller.tr('ui_runtime_status'),
                      width: labelColumnWidth,
                      textAlign: TextAlign.left,
                      style: TextStyle(color: getThemeTextColor(), fontSize: 13),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: SingleChildScrollView(
                        scrollDirection: Axis.horizontal,
                        child: Row(
                          children: [
                            _buildRuntimeFilterOption(
                              value: widget.controller.filterRuntimeNodejsOnly.value,
                              label: widget.controller.tr('ui_runtime_name_nodejs'),
                              onChanged: (value) => widget.controller.updatePluginFilters(runtimeNodejsOnly: value ?? false),
                            ),
                            const SizedBox(width: 14),
                            _buildRuntimeFilterOption(
                              value: widget.controller.filterRuntimePythonOnly.value,
                              label: widget.controller.tr('ui_runtime_name_python'),
                              onChanged: (value) => widget.controller.updatePluginFilters(runtimePythonOnly: value ?? false),
                            ),
                            const SizedBox(width: 14),
                            if (isStorePluginList)
                              _buildRuntimeFilterOption(
                                value: widget.controller.filterRuntimeScriptOnly.value,
                                label: widget.controller.tr('ui_runtime_name_script'),
                                onChanged: (value) => widget.controller.updatePluginFilters(runtimeScriptOnly: value ?? false),
                              ),
                            if (!isStorePluginList)
                              _buildRuntimeFilterOption(
                                value: widget.controller.filterRuntimeScriptNodejsOnly.value,
                                label: widget.controller.tr('plugin_wpm_script_template_nodejs'),
                                onChanged: (value) => widget.controller.updatePluginFilters(runtimeScriptNodejsOnly: value ?? false),
                              ),
                            if (!isStorePluginList) const SizedBox(width: 14),
                            if (!isStorePluginList)
                              _buildRuntimeFilterOption(
                                value: widget.controller.filterRuntimeScriptPythonOnly.value,
                                label: widget.controller.tr('plugin_wpm_script_template_python'),
                                onChanged: (value) => widget.controller.updatePluginFilters(runtimeScriptPythonOnly: value ?? false),
                              ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      );
    });
  }
}
