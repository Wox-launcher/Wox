import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingUIView extends WoxSettingBaseView {
  const WoxSettingUIView({super.key});

  List<WoxDropdownItem<int>> _buildWindowWidthItems(int currentWidth) {
    final values = List<int>.generate(21, (index) => 600 + (index * 50));
    if (!values.contains(currentWidth)) {
      values.add(currentWidth);
      values.sort();
    }

    return values.map((width) => WoxDropdownItem<int>(value: width, label: width.toString())).toList();
  }

  List<WoxDropdownItem<String>> _buildInterfaceSizeItems() {
    return [
      WoxDropdownItem(value: "compact", label: controller.tr("ui_interface_size_compact")),
      WoxDropdownItem(value: "normal", label: controller.tr("ui_interface_size_normal")),
      WoxDropdownItem(value: "comfortable", label: controller.tr("ui_interface_size_comfortable")),
    ];
  }

  List<WoxDropdownItem<String>> _buildGlanceItems() {
    final items = <WoxDropdownItem<String>>[];
    final iconColor = getThemeTextColor();
    for (final plugin in controller.installedPlugins) {
      for (final glance in plugin.glances) {
        final key = GlanceRef(pluginId: plugin.id, glanceId: glance.id).key;
        final previewItem = controller.settingGlancePreviewItems[key];
        // Feature change: live Glance responses can expose state-specific icons
        // that metadata cannot know yet, so the picker uses the API icon first
        // and keeps metadata as the loading fallback.
        final icon = previewItem != null && previewItem.icon.imageData.isNotEmpty ? previewItem.icon : WoxImage.parse(glance.icon);
        // Glance choices are user-facing metadata, so show the actual glance first and keep the provider as supporting context instead of forcing users to parse "plugin / item" strings.
        items.add(
          WoxDropdownItem(
            value: key,
            label: glance.name,
            leading: icon == null ? null : WoxImageView(woxImage: icon, width: 18, height: 18, svgColor: iconColor),
            trailing: _buildGlancePreviewValue(previewItem),
          ),
        );
      }
    }
    return items;
  }

  Widget _buildGlancePreviewValue(GlanceItem? item) {
    final text = item == null || item.text.trim().isEmpty ? "—" : item.text.trim();
    final isEmptyPreview = item == null || item.text.trim().isEmpty;
    final color = isEmptyPreview ? getThemeSubTextColor().withValues(alpha: 0.65) : getThemeTextColor();

    return ConstrainedBox(
      constraints: const BoxConstraints(maxWidth: 110),
      // The settings picker previews the real Glance response so users choose
      // the item by its live output, not by metadata that can only approximate it.
      child: Text(text, maxLines: 1, overflow: TextOverflow.ellipsis, textAlign: TextAlign.right, style: TextStyle(color: color, fontSize: 13)),
    );
  }

  GlanceRef _parseGlanceKey(String key) {
    final parts = key.split('\x00');
    if (parts.length != 2) {
      return GlanceRef.empty();
    }
    return GlanceRef(pluginId: parts[0], glanceId: parts[1]);
  }

  Future<void> _updateGlanceSlot(String settingKey, String value) async {
    final ref = _parseGlanceKey(value);
    await controller.updateConfig(settingKey, jsonEncode(ref.toJson()));
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        controller.refreshSettingGlancePreviewsForUIEntry(const UuidV4().generate());
      });

      return form(
        title: controller.tr("ui_ui"),
        description: controller.tr("ui_ui_description"),
        children: [
          formSection(
            title: controller.tr("ui_ui_section_launcher"),
            children: [
              if (!controller.woxSetting.value.isLinuxWaylandSession)
                // Wayland compositors own top-level window placement, so Wox cannot honor
                // launcher position preferences there.
                formField(
                  settingKey: "ShowPosition",
                  label: controller.tr("ui_show_position"),
                  tips: controller.tr("ui_show_position_tips"),
                  child: Obx(() {
                    return WoxDropdownButton<String>(
                      items: [
                        WoxDropdownItem(value: "mouse_screen", label: controller.tr("ui_show_position_mouse_screen")),
                        WoxDropdownItem(value: "active_screen", label: controller.tr("ui_show_position_active_screen")),
                        WoxDropdownItem(value: "last_location", label: controller.tr("ui_show_position_last_location")),
                      ],
                      value: controller.woxSetting.value.showPosition,
                      onChanged: (v) {
                        if (v != null) {
                          controller.updateConfig("ShowPosition", v);
                        }
                      },
                      isExpanded: true,
                    );
                  }),
                ),
              formField(
                settingKey: "ShowTray",
                label: controller.tr("ui_show_tray"),
                tips: controller.tr("ui_show_tray_tips"),
                child: Obx(() {
                  return WoxSwitch(
                    value: controller.woxSetting.value.showTray,
                    onChanged: (bool value) {
                      controller.updateConfig("ShowTray", value.toString());
                    },
                  );
                }),
              ),
              formField(
                settingKey: "AppWidth",
                label: controller.tr("ui_app_width"),
                tips: controller.tr("ui_app_width_tips"),
                child: Obx(() {
                  final currentWidth = controller.woxSetting.value.appWidth;

                  return WoxDropdownButton<int>(
                    // Width is a preset-style preference, so a dropdown avoids the imprecise slider interaction and keeps the UI aligned with other setting rows.
                    value: currentWidth,
                    items: _buildWindowWidthItems(currentWidth),
                    onChanged: (value) {
                      if (value != null) {
                        controller.updateConfig("AppWidth", value.toString());
                      }
                    },
                    isExpanded: true,
                  );
                }),
              ),
              formField(
                settingKey: "UiDensity",
                label: controller.tr("ui_interface_size"),
                tips: controller.tr("ui_interface_size_tips"),
                child: Obx(() {
                  final items = _buildInterfaceSizeItems();
                  final currentDensity = controller.woxSetting.value.uiDensity;
                  final selectedDensity = items.any((item) => item.value == currentDensity) ? currentDensity : "normal";

                  return WoxDropdownButton<String>(
                    // Interface size only writes the density key; the settings page
                    // itself keeps its fixed layout while the launcher observes the
                    // reloaded setting and recomputes density metrics.
                    value: selectedDensity,
                    items: items,
                    onChanged: (value) {
                      if (value != null) {
                        controller.updateConfig("UiDensity", value);
                      }
                    },
                    isExpanded: true,
                  );
                }),
              ),
              formField(
                settingKey: "AppFontFamily",
                label: controller.tr("ui_app_font_family"),
                tips: controller.tr("ui_app_font_family_tips"),
                child: Obx(() {
                  final currentFontFamily = controller.woxSetting.value.appFontFamily;
                  final fontFamilies = List<String>.from(controller.systemFontFamilies);
                  if (currentFontFamily.isNotEmpty && !fontFamilies.contains(currentFontFamily)) {
                    fontFamilies.insert(0, currentFontFamily);
                  }

                  final items = <WoxDropdownItem<String>>[
                    WoxDropdownItem(value: "", label: controller.tr("ui_app_font_family_system_default")),
                    ...fontFamilies.map((family) => WoxDropdownItem<String>(value: family, label: family)),
                  ];

                  final selectedValue = items.any((item) => item.value == currentFontFamily) ? currentFontFamily : "";

                  return WoxDropdownButton<String>(
                    value: selectedValue,
                    items: items,
                    onChanged: (value) {
                      if (value != null) {
                        controller.updateConfig("AppFontFamily", value);
                      }
                    },
                    isExpanded: true,
                    enableFilter: true,
                    filterHintText: controller.tr("ui_filter_placeholder"),
                    menuMaxHeight: 360,
                  );
                }),
              ),
              formField(
                settingKey: "EnableQueryCompletionHint",
                label: controller.tr("ui_query_completion_hint"),
                tips: controller.tr("ui_query_completion_hint_tips"),
                child: Obx(() {
                  return WoxSwitch(
                    value: controller.woxSetting.value.enableQueryCompletionHint,
                    onChanged: (bool value) {
                      controller.updateConfig("EnableQueryCompletionHint", value.toString());
                    },
                  );
                }),
              ),
            ],
          ),
          formSection(
            title: controller.tr("ui_ui_section_results"),
            children: [
              formField(
                settingKey: "MaxResultCount",
                label: controller.tr("ui_max_result_count"),
                tips: controller.tr("ui_max_result_count_tips"),
                child: Obx(() {
                  return WoxDropdownButton<int>(
                    value: controller.woxSetting.value.maxResultCount,
                    items: List.generate(11, (index) => index + 5).map((count) => WoxDropdownItem<int>(value: count, label: count.toString())).toList(),
                    onChanged: (v) {
                      if (v != null) {
                        controller.updateConfig("MaxResultCount", v.toString());
                      }
                    },
                  );
                }),
              ),
            ],
          ),
          formSection(
            title: controller.tr("ui_ui_section_glance"),
            children: [
              formField(
                settingKey: "EnableGlance",
                label: controller.tr("ui_glance_enable"),
                tips: controller.tr("ui_glance_enable_tips"),
                child: Obx(() {
                  return WoxSwitch(
                    value: controller.woxSetting.value.enableGlance,
                    onChanged: (bool value) {
                      controller.updateConfig("EnableGlance", value.toString());
                    },
                  );
                }),
              ),
              formField(
                settingKey: "HideGlanceIcon",
                label: controller.tr("ui_glance_hide_icon"),
                tips: controller.tr("ui_glance_hide_icon_tips"),
                child: Obx(() {
                  return WoxSwitch(
                    value: controller.woxSetting.value.hideGlanceIcon,
                    onChanged: (bool value) {
                      // HideGlanceIcon is separated from EnableGlance so users
                      // can keep the same selected item and only choose whether
                      // the query-box accessory spends space on the icon.
                      controller.updateConfig("HideGlanceIcon", value.toString());
                    },
                  );
                }),
              ),
              formField(
                settingKey: "PrimaryGlance",
                label: controller.tr("ui_glance_primary"),
                tips: controller.tr("ui_glance_primary_tips"),
                child: Obx(() {
                  final setting = controller.woxSetting.value;
                  final items = _buildGlanceItems();
                  final selected = items.any((item) => item.value == setting.primaryGlance.key) ? setting.primaryGlance.key : (items.isNotEmpty ? items.first.value : "");
                  return WoxDropdownButton<String>(
                    value: selected,
                    items: items,
                    onChanged: (value) {
                      if (value != null && value.isNotEmpty) {
                        _updateGlanceSlot("PrimaryGlance", value);
                      }
                    },
                    isExpanded: true,
                  );
                }),
              ),
            ],
          ),
        ],
      );
    });
  }
}
