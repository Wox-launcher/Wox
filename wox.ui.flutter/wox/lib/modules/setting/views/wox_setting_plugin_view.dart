import 'dart:convert';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/plugin/wox_ai_command_template_dialog.dart';
import 'package:wox/components/plugin/wox_setting_plugin_dictation_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_item_view.dart';
import 'package:wox/components/wox_plugin_detail_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_ai_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/plugin/wox_window_manager_groups_setting.dart';
import 'package:wox/components/wox_hint_box.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
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
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_dictation_hotkey.dart';
import 'package:wox/entity/setting/wox_plugin_setting_dictation_model.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_newline.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/components/plugin/wox_setting_plugin_checkbox_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_textbox_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/strings.dart';
import 'package:wox/utils/wox_setting_focus_util.dart';
import 'package:wox/utils/wox_text_measure_util.dart';
import 'package:wox/enums/wox_plugin_runtime_enum.dart';

class WoxSettingPluginView extends GetView<WoxSettingController> {
  const WoxSettingPluginView({super.key});
  static const String _triggerKeywordColumnKey = "keyword";
  static const String _triggerKeywordOriginalColumnKey = "_wox_original_trigger_keyword";
  static const String _globalTriggerKeyword = "*";
  static const String _aiCommandPluginId = "c9910664-1c28-47ae-bad6-e7332a02d471";
  static const String _windowManagerPluginId = "5b7d9f22-4d87-4c0f-a2c1-8e2b50c8bca0";
  static const String _windowManagerGroupsSettingKey = "windowGroups";
  static const String _selectionPluginId = "d9e557ed-89bd-4b8b-bd64-2a7632cf3483";
  static const String _selectionSpaceQuickLookSettingKey = "enableSpaceQuickLook";
  static const double _pluginSettingLabelActionWidth = 28.0;
  static const List<WoxHotkeyRecorderKind> _dictationHotkeyKinds = [
    WoxHotkeyRecorderKind.normalCombo,
    WoxHotkeyRecorderKind.doubleModifier,
    WoxHotkeyRecorderKind.capsLockCombo,
    WoxHotkeyRecorderKind.pressModifier,
    WoxHotkeyRecorderKind.holdModifier,
  ];
  // Local refreshing state for showing loading spinner on refresh button
  static final RxBool _refreshing = false.obs;
  static final GlobalKey _pluginFilterIconKey = GlobalKey();

  String _runtimeDisplayName(String runtime) {
    switch (runtime.toUpperCase()) {
      case 'PYTHON':
        return controller.tr("ui_runtime_name_python");
      case 'NODEJS':
        return controller.tr("ui_runtime_name_nodejs");
      default:
        return runtime;
    }
  }

  String _runtimeStatusLabel(String statusCode) {
    switch (statusCode) {
      case 'executable_missing':
        return controller.tr("ui_runtime_status_executable_missing");
      case 'unsupported_version':
        return controller.tr("ui_runtime_status_unsupported_version");
      case 'start_failed':
        return controller.tr("ui_runtime_status_start_failed");
      default:
        return controller.tr("ui_runtime_status_stopped");
    }
  }

  String _pluginInstallRuntimeStatusDetail(PluginDetail plugin, WoxRuntimeStatus status) {
    final runtimeName = _runtimeDisplayName(status.runtime);
    switch (status.statusCode) {
      case 'executable_missing':
        return controller.tr("ui_plugin_install_runtime_missing_detail").replaceAll("{plugin}", plugin.name).replaceAll("{runtime}", runtimeName);
      case 'unsupported_version':
        return controller.tr("ui_plugin_install_runtime_unsupported_detail").replaceAll("{plugin}", plugin.name).replaceAll("{runtime}", runtimeName);
      case 'start_failed':
        return status.lastStartError.isNotEmpty ? status.lastStartError : controller.tr("ui_plugin_install_runtime_start_failed_detail").replaceAll("{runtime}", runtimeName);
      default:
        return status.statusMessage;
    }
  }

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
    if (settingType == "dictationHotkey") {
      return (settingDefinitionValue as PluginSettingValueDictationHotkey).label;
    }
    if (settingType == "dictationModel") {
      return (settingDefinitionValue as PluginSettingValueDictationModel).label;
    }

    return "";
  }

  double _measureLabelWidthByText(BuildContext context, String label, {double minWidth = PLUGIN_SETTING_LABEL_MIN_WIDTH}) {
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

      final labelActionWidth = _pluginSettingLabelActionWidthFor(plugin: plugin, definition: definition);
      maxLabelWidth = math.max(maxLabelWidth, _measureLabelWidthByText(context, translatedLabel, minWidth: 0) + labelActionWidth);
    }

    return maxLabelWidth.clamp(PLUGIN_SETTING_LABEL_MIN_WIDTH, PLUGIN_SETTING_LABEL_MAX_WIDTH).toDouble();
  }

  double _pluginSettingLabelActionWidthFor({required PluginDetail plugin, required PluginSettingDefinitionItem definition}) {
    if (plugin.id == _selectionPluginId &&
        definition.value is PluginSettingValueCheckBox &&
        (definition.value as PluginSettingValueCheckBox).key == _selectionSpaceQuickLookSettingKey) {
      return _pluginSettingLabelActionWidth;
    }

    if (plugin.id == _windowManagerPluginId && definition.value is PluginSettingValueTable && (definition.value as PluginSettingValueTable).key == _windowManagerGroupsSettingKey) {
      return _pluginSettingLabelActionWidth;
    }

    return 0;
  }

  String _extractStablePluginSettingKey(PluginSettingDefinitionItem definition) {
    final value = definition.value;
    if (value is PluginSettingValueCheckBox) {
      return value.key;
    }
    if (value is PluginSettingValueTextBox) {
      return value.key;
    }
    if (value is PluginSettingValueSelect) {
      return value.key;
    }
    if (value is PluginSettingValueSelectAIModel) {
      return value.key;
    }
    if (value is PluginSettingValueTable) {
      return value.key;
    }
    if (value is PluginSettingValueDictationHotkey) {
      return value.key;
    }
    if (value is PluginSettingValueDictationModel) {
      return value.key;
    }
    return "";
  }

  Widget _buildPluginSettingTarget({required PluginDetail plugin, required PluginSettingDefinitionItem definition, required Widget child}) {
    final settingKey = _extractStablePluginSettingKey(definition).trim();
    if (settingKey.isEmpty) {
      return child;
    }

    return Container(
      key: controller.getPluginSettingItemKey(plugin.id, settingKey),
      child: Obx(() {
        final isHighlighted = controller.isSettingTargetHighlighted('plugin-setting-${plugin.id}-$settingKey');
        final wrappedChild = isHighlighted ? KeyedSubtree(key: ValueKey('settings-highlight-plugin-setting-${plugin.id}-$settingKey'), child: child) : child;

        // Feature: plugin settings are indexed from existing definitions, so each rendered
        // setting row owns the same stable key used by search to scroll and flash the target.
        return AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          curve: Curves.easeOutCubic,
          decoration: BoxDecoration(
            color: isHighlighted ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.18 : 0.10) : Colors.transparent,
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: isHighlighted ? getThemeActiveBackgroundColor().withValues(alpha: 0.45) : Colors.transparent, width: 1),
          ),
          child: wrappedChild,
        );
      }),
    );
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
    double panelWidth = math.min(PLUGIN_SETTING_FILTER_PANEL_WIDTH, screenSize.width - left - 12);
    if (panelWidth < panelMinWidth) {
      panelWidth = math.min(PLUGIN_SETTING_FILTER_PANEL_WIDTH, screenSize.width - 24);
      left = left.clamp(12.0, screenSize.width - panelWidth - 12.0);
    }

    final double panelHeight = 230;

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
    WoxSettingFocusUtil.restoreIfInSettingView();
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

                    // Shape, not color, carries the applied-filter state so the cue stays consistent across themes whose active colors have different contrast semantics.
                    final IconData filterIcon = controller.hasPluginFilterApplied ? Icons.filter_alt : Icons.filter_alt_outlined;
                    final Color iconColor = getThemeSubTextColor();
                    return Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Padding(
                          padding: const EdgeInsets.only(right: 2),
                          child: GestureDetector(
                            key: _pluginFilterIconKey,
                            onTap: () => _showPluginFilterPanel(context),
                            child: Padding(padding: const EdgeInsets.symmetric(horizontal: 4.0), child: Icon(filterIcon, color: iconColor)),
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
                controller.handlePluginSearchChanged();
              },
            );
          }),
        ),
        Expanded(
          child: Scrollbar(
            thumbVisibility: false,
            controller: controller.pluginListScrollController,
            child: Obx(() {
              if (controller.filteredPluginList.isEmpty) {
                return SingleChildScrollView(
                  controller: controller.pluginListScrollController,
                  child: Center(child: Text(controller.tr('ui_setting_plugin_empty_data'), style: TextStyle(color: getThemeSubTextColor()))),
                );
              }

              return ListView.builder(
                controller: controller.pluginListScrollController,
                itemCount: controller.filteredPluginList.length,
                itemBuilder: (context, index) {
                  final plugin = controller.filteredPluginList[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8.0),
                    child: Obx(() {
                      final isActive = controller.activePlugin.value.id == plugin.id;
                      final isHighlighted = controller.isSettingTargetHighlighted('plugin-${plugin.id}');
                      final tile = GestureDetector(
                        behavior: HitTestBehavior.translucent,
                        onTap: () {
                          controller.activePlugin.value = plugin;
                        },
                        child: ListTile(
                          contentPadding: const EdgeInsets.only(left: 6, right: 6),
                          leading: WoxImageView(woxImage: plugin.icon, width: 32, height: 32),
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
                      );
                      final row = Container(
                        key: controller.getPluginListItemKey(plugin.id),
                        decoration: BoxDecoration(
                          // Feature: settings search can jump to an installed plugin row.
                          // Keep the selected fill dominant while using a border as the
                          // temporary search cue so active and highlighted states do not fight.
                          color:
                              isActive
                                  ? getThemeActiveBackgroundColor()
                                  : (isHighlighted ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.16 : 0.08) : Colors.transparent),
                          borderRadius: BorderRadius.circular(4),
                          border: Border.all(color: isHighlighted ? getThemeActiveBackgroundColor().withValues(alpha: 0.48) : Colors.transparent, width: 1),
                        ),
                        child: tile,
                      );
                      return isHighlighted ? KeyedSubtree(key: ValueKey('settings-highlight-plugin-${plugin.id}'), child: row) : row;
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
                  WoxImageView(woxImage: plugin.icon, width: 32, height: 32),
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
                final runtimeStatus = controller.getActionableRuntimeStatusForPlugin(plugin);
                final bool isRestarting = runtimeStatus != null && controller.restartingRuntime.value == runtimeStatus.runtime.toUpperCase();
                final String errorTitle =
                    runtimeStatus == null ? controller.pluginInstallError.value : '${_runtimeDisplayName(runtimeStatus.runtime)}: ${_runtimeStatusLabel(runtimeStatus.statusCode)}';
                final String errorDetail = runtimeStatus == null ? '' : _pluginInstallRuntimeStatusDetail(plugin, runtimeStatus);
                return Container(
                  margin: const EdgeInsets.only(left: 16, right: 16, top: 8),
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: Colors.red.withValues(alpha: 0.1),
                    border: Border.all(color: Colors.red.withValues(alpha: 0.3)),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Icon(Icons.error_outline, color: Colors.red, size: 20),
                          const SizedBox(width: 8),
                          // Bug fix: when core can identify a runtime problem, hide the wrapped
                          // install exception chain and show one localized recovery message. The
                          // raw chain is still logged by core and remains available when no
                          // structured runtime diagnosis exists.
                          Expanded(
                            child: Text(errorTitle, style: TextStyle(color: Colors.red, fontSize: 13, fontWeight: runtimeStatus == null ? FontWeight.normal : FontWeight.w600)),
                          ),
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
                      if (runtimeStatus != null) ...[
                        const SizedBox(height: 8),
                        Text(errorDetail, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
                        const SizedBox(height: 10),
                        Row(
                          children: [
                            if (runtimeStatus.installUrl.isNotEmpty && (runtimeStatus.statusCode == 'executable_missing' || runtimeStatus.statusCode == 'unsupported_version')) ...[
                              WoxButton.secondary(
                                text: controller
                                    .tr(runtimeStatus.statusCode == 'unsupported_version' ? "ui_runtime_upgrade_runtime" : "ui_runtime_install_runtime")
                                    .replaceAll("{runtime}", _runtimeDisplayName(runtimeStatus.runtime)),
                                icon: Icon(Icons.open_in_new, size: 14, color: getThemeTextColor()),
                                onPressed: () {
                                  controller.openRuntimeInstallUrl(runtimeStatus);
                                },
                              ),
                              const SizedBox(width: 8),
                            ],
                            if (runtimeStatus.canRestart)
                              WoxButton.secondary(
                                text: isRestarting ? controller.tr("ui_runtime_restarting_host") : controller.tr("ui_runtime_restart_host"),
                                icon:
                                    isRestarting
                                        ? WoxLoadingIndicator(size: 14, color: getThemeActionItemActiveColor())
                                        : Icon(Icons.restart_alt, size: 14, color: getThemeTextColor()),
                                onPressed:
                                    isRestarting
                                        ? null
                                        : () {
                                          controller.restartRuntime(runtimeStatus);
                                        },
                              ),
                          ],
                        ),
                      ],
                    ],
                  ),
                );
              }
              return const SizedBox.shrink();
            }),
            Expanded(
              child: _InstantPluginTabView(
                labelColor: getThemeTextColor(),
                unselectedLabelColor: getThemeTextColor(),
                indicatorColor: getThemeActiveBackgroundColor(),
                dividerColor: getThemeSettingDividerColor(),
                tabs:
                    controller.activeNavPath.value == 'plugins.installed'
                        ? [
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_settings'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabSetting(context),
                          ),
                          // Installed plugins keep the description directly after Settings so users can review plugin context before changing keywords or commands; the previous order buried this basic context behind operational tabs.
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_description'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabDescription(),
                          ),
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_trigger_keywords'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabTriggerKeywords(),
                          ),
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_commands'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabCommand(context),
                          ),
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_privacy'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabPrivacy(context),
                          ),
                        ]
                        : [
                          // For uninstalled plugins: Description tab first
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_description'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabDescription(),
                          ),
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_trigger_keywords'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabTriggerKeywords(),
                          ),
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_commands'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabCommand(context),
                          ),
                          _PluginTabData(
                            title: Tab(child: Text(controller.tr('ui_plugin_tab_privacy'), style: TextStyle(color: getThemeTextColor()))),
                            content: pluginTabPrivacy(context),
                          ),
                        ],
                onTabControllerUpdated: (tabController) {
                  controller.activePluginTabController = tabController;
                  tabController.index = 0;
                },
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

  String _normalizeTriggerKeyword(dynamic value) {
    return value?.toString().trim() ?? "";
  }

  List<Map<String, String>> _buildTriggerKeywordRows(PluginDetail plugin) {
    // Keep the original keyword in a hidden row field so edit validation can
    // distinguish "saving this same keyword again" from "adding a duplicate".
    // The hidden field is stripped by the tab save handler before persisting.
    return plugin.triggerKeywords.map((keyword) => {_triggerKeywordColumnKey: keyword, _triggerKeywordOriginalColumnKey: keyword}).toList();
  }

  Future<List<PluginSettingTableValidationError>> _validateTriggerKeywordUpdate(PluginDetail plugin, Map<String, dynamic> rowValues) async {
    final keyword = _normalizeTriggerKeyword(rowValues[_triggerKeywordColumnKey]);
    if (keyword.isEmpty) {
      return const [];
    }
    if (keyword == _globalTriggerKeyword) {
      // "*" is a shared global-query marker rather than an exclusive route.
      // The previous duplicate checks treated it like a normal keyword, which
      // blocked a second plugin from opting into global queries even though the
      // core can fan out empty-trigger input to every plugin that declares "*".
      return const [];
    }

    final originalKeyword = _normalizeTriggerKeyword(rowValues[_triggerKeywordOriginalColumnKey]);
    // Duplicate checks need to ignore the row being edited. Counting only the
    // typed value made a normal save of the original keyword look like a conflict,
    // so the hidden original-keyword marker is used to subtract that row once.
    var samePluginMatchCount = plugin.triggerKeywords.where((item) => _normalizeTriggerKeyword(item) == keyword).length;
    if (originalKeyword == keyword && samePluginMatchCount > 0) {
      samePluginMatchCount--;
    }
    if (samePluginMatchCount > 0) {
      return const [PluginSettingTableValidationError(key: _triggerKeywordColumnKey, errorMsg: "ui_plugin_trigger_keyword_duplicate_in_plugin")];
    }

    // Cross-plugin conflicts must be checked against installed plugins, because
    // store-only plugin metadata can share defaults without affecting launcher
    // routing. Blocking here keeps the setting dialog open with a field-level hint.
    for (final item in controller.installedPlugins) {
      if (item.id == plugin.id) {
        continue;
      }
      if (item.triggerKeywords.any((triggerKeyword) => _normalizeTriggerKeyword(triggerKeyword) == keyword)) {
        // Include the conflicting plugin name in the validation error so users
        // know which existing route must be changed before this keyword can be saved.
        final conflictPluginName = item.name.trim().isNotEmpty ? item.name.trim() : item.id;
        final message = Strings.format(controller.tr("ui_plugin_trigger_keyword_duplicate_in_other_plugin"), [conflictPluginName]);
        return [PluginSettingTableValidationError(key: _triggerKeywordColumnKey, errorMsg: message)];
      }
    }

    return const [];
  }

  Widget? _buildTriggerKeywordCell(PluginSettingValueTableColumn column, Map<String, dynamic> row) {
    if (column.key != _triggerKeywordColumnKey || row[column.key] != _globalTriggerKeyword) {
      return null;
    }

    final accentColor = getThemeActiveBackgroundColor();
    // Keep the persisted global keyword as "*" for compatibility while presenting
    // it as a readable pill in the table; raw symbols looked like placeholder data.
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: accentColor.withValues(alpha: 0.1),
        border: Border.all(color: accentColor.withValues(alpha: 0.22)),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.public_rounded, size: 14, color: accentColor),
          const SizedBox(width: 5),
          Text(controller.tr('ui_plugin_trigger_keyword_global'), style: TextStyle(color: accentColor, fontSize: 12, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }

  Widget _buildPluginEmptyState(BuildContext context, {required IconData icon, required String title, required String description}) {
    final accentColor = getThemeActiveBackgroundColor();
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();

    return LayoutBuilder(
      builder: (context, constraints) {
        final contentHeight = constraints.hasBoundedHeight ? math.max(260.0, constraints.maxHeight - 32) : 360.0;

        return Padding(
          padding: const EdgeInsets.all(16),
          child: SizedBox(
            width: double.infinity,
            height: contentHeight,
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 430),
                // Empty plugin detail panes used to render as a single left-aligned line,
                // which looked unfinished in a large area. A shared centered state keeps
                // empty settings and privacy tabs visually consistent without adding noise.
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      width: 58,
                      height: 58,
                      decoration: BoxDecoration(
                        color: accentColor.withValues(alpha: 0.1),
                        border: Border.all(color: accentColor.withValues(alpha: 0.18)),
                        borderRadius: BorderRadius.circular(18),
                      ),
                      child: Icon(icon, color: accentColor, size: 28),
                    ),
                    const SizedBox(height: 18),
                    Text(title, textAlign: TextAlign.center, style: TextStyle(color: textColor, fontSize: 18, fontWeight: FontWeight.w700)),
                    const SizedBox(height: 8),
                    Text(description, textAlign: TextAlign.center, style: TextStyle(color: subTextColor, fontSize: 13, height: 1.45)),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget pluginTabSetting(BuildContext context) {
    return Obx(() {
      var plugin = controller.activePlugin.value;
      final uniformLabelWidth = _calculateUniformPluginLabelWidth(context, plugin);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        controller.notifyPluginSettingViewReady();
      });

      // Show empty state if no settings
      if (plugin.settingDefinitions.isEmpty) {
        return _buildPluginEmptyState(
          context,
          icon: Icons.tune_rounded,
          title: controller.tr('ui_plugin_no_settings'),
          description: controller.tr('ui_plugin_no_settings_subtitle'),
        );
      }

      final useInlineTableActions = plugin.settingDefinitions.every((definition) => definition.type == "table");

      return Padding(
        padding: const EdgeInsets.all(16.0),
        child: SingleChildScrollView(
          child: Column(
            // Plugin settings use a stacked rhythm because the detail pane shares space with the plugin list; the top-level two-column form made descriptions wrap too aggressively here.
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              ...plugin.settingDefinitions.map((e) {
                late final Widget settingWidget;
                if (e.type == "checkbox") {
                  final checkboxValue = e.value as PluginSettingValueCheckBox;
                  settingWidget = WoxSettingPluginCheckbox(
                    value: plugin.setting.settings[checkboxValue.key] ?? "",
                    item: checkboxValue,
                    labelWidth: uniformLabelWidth,
                    labelActions: _buildPluginSettingLabelActions(plugin: plugin, settingKey: checkboxValue.key),
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "textbox") {
                  settingWidget = WoxSettingPluginTextBox(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueTextBox,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "newline") {
                  settingWidget = WoxSettingPluginNewLine(
                    value: "",
                    item: e.value as PluginSettingValueNewLine,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async => null,
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "select") {
                  settingWidget = WoxSettingPluginSelect(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueSelect,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "selectAIModel") {
                  settingWidget = WoxSettingPluginSelectAIModel(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: e.value as PluginSettingValueSelectAIModel,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "head") {
                  settingWidget = WoxSettingPluginHead(value: "", item: e.value as PluginSettingValueHead, labelWidth: uniformLabelWidth, onUpdate: (key, value) async => null);
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "label") {
                  settingWidget = WoxSettingPluginLabel(value: "", item: e.value as PluginSettingValueLabel, labelWidth: uniformLabelWidth, onUpdate: (key, value) async => null);
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "dictationHotkey") {
                  final hotkeyValue = e.value as PluginSettingValueDictationHotkey;
                  final savedHotkeyValue = (plugin.setting.settings[hotkeyValue.key] ?? "").trim();
                  settingWidget = WoxSettingPluginItem.layoutFor(
                    label: hotkeyValue.label,
                    style: hotkeyValue.style,
                    tooltip: hotkeyValue.tooltip,
                    labelWidth: uniformLabelWidth,
                    translator: controller.tr,
                    child: WoxHotkeyRecorder(
                      hotkey: savedHotkeyValue.isNotEmpty ? WoxHotkey.parseHotkeyFromString(savedHotkeyValue) : null,
                      purpose: WoxHotkeyRecorderPurpose.dictation,
                      allowedKinds: _dictationHotkeyKinds,
                      tipPosition: WoxHotkeyRecorderTipPosition.right,
                      onHotKeyRecorded: (result) {
                        controller.updatePluginSetting(plugin.id, hotkeyValue.key, result.hotkey);
                      },
                      onUnavailableHotKeyRecorded: (_) {
                        controller.updatePluginSetting(plugin.id, hotkeyValue.key, "");
                      },
                    ),
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "dictationModel") {
                  final modelValue = e.value as PluginSettingValueDictationModel;
                  settingWidget = WoxSettingPluginDictationModel(
                    value: plugin.setting.settings[modelValue.key] ?? "",
                    item: modelValue,
                    labelWidth: uniformLabelWidth,
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }
                if (e.type == "table") {
                  final tableValue = e.value as PluginSettingValueTable;
                  if (plugin.id == _windowManagerPluginId && tableValue.key == _windowManagerGroupsSettingKey) {
                    settingWidget = WoxWindowManagerGroupsSetting(
                      value: plugin.setting.settings[tableValue.key] ?? "",
                      labelWidth: uniformLabelWidth,
                      onUpdate: (key, value) async {
                        return controller.updatePluginSetting(plugin.id, key, value);
                      },
                    );
                    return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                  }

                  final isAICommandCommandsTable = plugin.id == _aiCommandPluginId && tableValue.key == "commands";
                  settingWidget = WoxSettingPluginTable(
                    value: plugin.setting.settings[e.value.key] ?? "",
                    item: tableValue,
                    labelWidth: uniformLabelWidth,
                    inlineTitleActions: useInlineTableActions,
                    trailingActions: isAICommandCommandsTable ? [_buildAICommandTemplateAction(context, plugin, plugin.setting.settings[tableValue.key] ?? "")] : const [],
                    onUpdate: (key, value) async {
                      return controller.updatePluginSetting(plugin.id, key, value);
                    },
                  );
                  return _buildPluginSettingTarget(plugin: plugin, definition: e, child: settingWidget);
                }

                return Text(e.type);
              }),
            ],
          ),
        ),
      );
    });
  }

  Widget _buildAICommandTemplateAction(BuildContext context, PluginDetail plugin, String tableValue) {
    return WoxButton.secondary(
      text: controller.tr("ui_ai_command_template_add_from_store"),
      icon: Icon(Icons.storefront_outlined, color: getThemeSubTextColor(), size: 16),
      height: 30,
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      onPressed: () async {
        List<dynamic> rows = [];
        try {
          final decoded = json.decode(tableValue.trim().isEmpty ? "[]" : tableValue);
          if (decoded is List) {
            rows = decoded;
          }
        } catch (_) {
          rows = [];
        }

        await showAICommandTemplateDialog(context: context, pluginId: plugin.id, currentRows: rows, triggerKeyword: _resolvePrimaryTriggerKeyword(plugin));
        WoxSettingFocusUtil.restoreIfInSettingView();
      },
    );
  }

  List<Widget> _buildPluginSettingLabelActions({required PluginDetail plugin, required String settingKey}) {
    if (plugin.id != _selectionPluginId || settingKey != _selectionSpaceQuickLookSettingKey) {
      return const [];
    }

    return [
      _buildPluginSettingDemoAction(
        triggerKey: 'settings-selection-space-quick-look-demo-trigger',
        popoverKey: 'wox-demo-popover-selectionSpaceQuickLook',
        demo: WoxSelectionSpaceQuickLookDemo(accent: const Color(0xFF10B981), tr: controller.tr),
      ),
    ];
  }

  Widget _buildPluginSettingDemoAction({required String triggerKey, required String popoverKey, required Widget demo}) {
    final foreground = getThemeTextColor();

    return WoxDemoPopover(
      key: ValueKey(triggerKey),
      popoverKey: ValueKey(popoverKey),
      demo: demo,
      width: 680,
      height: 460,
      child: Semantics(
        label: controller.tr("ui_demo_preview"),
        button: true,
        child: MouseRegion(
          cursor: SystemMouseCursors.help,
          child: SizedBox(width: 22, height: 22, child: Icon(Icons.play_circle_outline_rounded, color: foreground.withValues(alpha: 0.88), size: 18)),
        ),
      ),
    );
  }

  String _resolvePrimaryTriggerKeyword(PluginDetail plugin) {
    for (final keyword in plugin.triggerKeywords) {
      final normalized = keyword.trim();
      if (normalized.isNotEmpty) {
        return normalized;
      }
    }

    return "ai";
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
          // Trigger keywords are required to make an installed plugin reachable.
          // Always render the editable table so bad or legacy empty data can be fixed
          // instead of showing a static empty label with no recovery path.
          WoxSettingPluginTable(
            value: json.encode(_buildTriggerKeywordRows(plugin)),
            tableWidth: PLUGIN_SETTING_TABLE_WIDTH,
            minimumRowCount: 1,
            minimumRowDeleteMessage: "ui_plugin_trigger_keyword_keep_one",
            customCellBuilder: _buildTriggerKeywordCell,
            onUpdateValidate: (rowValues) => _validateTriggerKeywordUpdate(plugin, rowValues),
            item: PluginSettingValueTable.fromJson({
              "Key": "_triggerKeywords",
              "Columns": [
                {
                  "Key": _triggerKeywordColumnKey,
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
                triggerKeywords.add(_normalizeTriggerKeyword(item[_triggerKeywordColumnKey]));
              }
              plugin.triggerKeywords = triggerKeywords;
              return controller.updatePluginSetting(plugin.id, "TriggerKeywords", triggerKeywords.join(","));
            },
          ),
        ],
      ),
    );
  }

  Widget pluginTabCommand(BuildContext context) {
    var plugin = controller.activePlugin.value;
    return Padding(
      padding: const EdgeInsets.all(16.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          WoxHintBox(text: controller.tr('ui_plugin_commands_tip')),
          const SizedBox(height: 12),
          if (plugin.commands.isEmpty)
            Expanded(
              child: _buildPluginEmptyState(
                context,
                icon: Icons.terminal_rounded,
                title: controller.tr('ui_plugin_no_commands'),
                description: controller.tr('ui_plugin_no_commands_subtitle'),
              ),
            )
          else
            WoxSettingPluginTable(
              value: json.encode(plugin.commands),
              tableWidth: PLUGIN_SETTING_TABLE_WIDTH,
              readonly: true,
              labelWidth: PLUGIN_SETTING_LABEL_WIDTH,
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

  Widget pluginTabPrivacy(BuildContext context) {
    var plugin = controller.activePlugin.value;
    final noDataAccess = _buildPluginEmptyState(
      context,
      icon: Icons.verified_user_outlined,
      title: controller.tr('ui_plugin_no_data_access'),
      description: controller.tr('ui_plugin_no_data_access_subtitle'),
    );

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
      child: SingleChildScrollView(
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
              if (e == "requireActiveWindowId") {
                return privacyItem(Icons.window, controller.tr('ui_plugin_privacy_window_id'), controller.tr('ui_plugin_privacy_window_id_desc'));
              }
              if (e == "requireActiveWindowIcon") {
                return privacyItem(Icons.window, controller.tr('ui_plugin_privacy_window_icon'), controller.tr('ui_plugin_privacy_window_icon_desc'));
              }
              if (e == "requireActiveWindowIsOpenSaveDialog") {
                return privacyItem(Icons.folder_open, controller.tr('ui_plugin_privacy_open_save_dialog'), controller.tr('ui_plugin_privacy_open_save_dialog_desc'));
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
          // Keep this pane splitter as the reference settings divider. Other
          // settings separators now reuse the same token instead of local alpha
          // variants or framework defaults.
          Container(width: 1, height: double.infinity, color: getThemeSettingDividerColor(), margin: const EdgeInsets.only(right: 10, left: 10)),
          pluginDetail(context),
        ],
      ),
    );
  }
}

class _PluginTabData {
  final Tab title;
  final Widget content;

  const _PluginTabData({required this.title, required this.content});
}

class _InstantPluginTabView extends StatefulWidget {
  final List<_PluginTabData> tabs;
  final ValueChanged<TabController> onTabControllerUpdated;
  final Color labelColor;
  final Color unselectedLabelColor;
  final Color indicatorColor;
  final Color dividerColor;

  const _InstantPluginTabView({
    required this.tabs,
    required this.onTabControllerUpdated,
    required this.labelColor,
    required this.unselectedLabelColor,
    required this.indicatorColor,
    required this.dividerColor,
  });

  @override
  State<_InstantPluginTabView> createState() => _InstantPluginTabViewState();
}

class _InstantPluginTabViewState extends State<_InstantPluginTabView> with TickerProviderStateMixin {
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = _createController();
    widget.onTabControllerUpdated(_tabController);
  }

  @override
  void didUpdateWidget(covariant _InstantPluginTabView oldWidget) {
    super.didUpdateWidget(oldWidget);

    if (oldWidget.tabs.length != widget.tabs.length) {
      _tabController.dispose();
      _tabController = _createController();
    }

    // Plugin detail tabs should feel like a settings pane switch, not a page
    // carousel. The old dynamic tab widget used the default TabController
    // animation, which added click feedback and a horizontal content slide.
    // A zero-duration controller preserves TabBar semantics while making every
    // rebuild and tab click switch panes immediately.
    widget.onTabControllerUpdated(_tabController);
  }

  TabController _createController() {
    return TabController(length: widget.tabs.length, animationDuration: Duration.zero, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        TabBar(
          isScrollable: true,
          controller: _tabController,
          tabAlignment: TabAlignment.start,
          labelColor: widget.labelColor,
          unselectedLabelColor: widget.unselectedLabelColor,
          indicatorColor: widget.indicatorColor,
          // Match the tab strip rule to the settings pane splitter. The default
          // Material divider used a different neutral, which made this horizontal
          // separator stand apart from the vertical settings separators.
          dividerColor: widget.dividerColor,
          dividerHeight: 1,
          // The plugin detail tab strip should not flash a pressed color; the
          // selected underline is the only state cue needed in this compact UI.
          splashFactory: NoSplash.splashFactory,
          overlayColor: WidgetStateProperty.all(Colors.transparent),
          enableFeedback: false,
          tabs: widget.tabs.map((tab) => tab.title).toList(),
        ),
        Expanded(child: TabBarView(controller: _tabController, physics: const NeverScrollableScrollPhysics(), children: widget.tabs.map((tab) => tab.content).toList())),
      ],
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
      widget.controller.tr('ui_plugin_filter_third_party_only'),
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
                if (isStorePluginList) const SizedBox(height: 10),
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
                if (!isStorePluginList) const SizedBox(height: 10),
                _buildBooleanFilterRow(
                  label: widget.controller.tr('ui_plugin_filter_third_party_only'),
                  labelWidth: labelColumnWidth,
                  value: widget.controller.filterThirdPartyPluginsOnly.value,
                  // Feature: third-party filtering stays in the same advanced
                  // filter group so it can be combined with status and runtime
                  // filters without adding a separate ownership filter model.
                  onChanged: (value) => widget.controller.updatePluginFilters(thirdPartyOnly: value ?? false),
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
