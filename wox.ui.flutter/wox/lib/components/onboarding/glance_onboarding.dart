import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/utils/colors.dart';

class WoxGlanceOnboarding extends StatelessWidget {
  const WoxGlanceOnboarding({
    super.key,
    required this.accent,
    required this.tr,
    required this.enabled,
    required this.isLoading,
    required this.isLoadFailed,
    required this.items,
    required this.currentValue,
    required this.label,
    required this.value,
    required this.icon,
    required this.onEnableChanged,
    required this.onPrimaryGlanceChanged,
  });

  final Color accent;
  final String Function(String key) tr;
  final bool enabled;
  final bool isLoading;
  final bool isLoadFailed;
  final List<WoxDropdownItem<String>> items;
  final String currentValue;
  final String label;
  final String value;
  final WoxImage? icon;
  final void Function(bool value) onEnableChanged;
  final void Function(String encodedRef) onPrimaryGlanceChanged;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: ValueKey('onboarding-media-glance-$enabled-$label-$value-${icon?.imageType}-${icon?.imageData}'),
      content: _buildContent(),
      demo: WoxGlanceDemo(accent: accent, enabled: enabled, label: label, value: value, icon: icon, tr: tr),
    );
  }

  Widget _buildContent() {
    final children = <Widget>[
      Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(tr('ui_glance_enable'), style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w700)),
                const SizedBox(height: 7),
                Text(tr('ui_glance_enable_tips'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.45)),
              ],
            ),
          ),
          const SizedBox(width: 18),
          WoxSwitch(
            key: const ValueKey('onboarding-glance-enable-switch'),
            value: enabled,
            onChanged: (value) {
              // Step extraction: the switch owns the Glance UI state, but the
              // parent still persists it through the same settings controller
              // path used elsewhere.
              onEnableChanged(value);
            },
          ),
        ],
      ),
    ];

    if (!enabled) {
      return WoxOnboardingSettingsPanel(key: const ValueKey('onboarding-glance-disabled'), children: children);
    }

    if (isLoading) {
      children.addAll([
        const SizedBox(height: 22),
        WoxOnboardingInfoPanel(
          key: const ValueKey('onboarding-glance-loading'),
          title: tr('onboarding_glance_loading_title'),
          body: tr('onboarding_glance_loading_body'),
          badge: tr('onboarding_loading'),
        ),
      ]);
      return WoxOnboardingSettingsPanel(children: children);
    }

    if (isLoadFailed || items.isEmpty) {
      children.addAll([
        const SizedBox(height: 22),
        WoxOnboardingInfoPanel(
          key: const ValueKey('onboarding-glance-empty'),
          title: tr('onboarding_glance_empty_title'),
          body: tr('onboarding_glance_empty_body'),
          badge: tr('onboarding_can_skip'),
        ),
      ]);
      return WoxOnboardingSettingsPanel(children: children);
    }

    return WoxOnboardingSettingsPanel(
      key: const ValueKey('onboarding-glance-picker'),
      children: [
        ...children,
        const SizedBox(height: 24),
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Text(tr('onboarding_glance_picker_label'), style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w600)),
            const SizedBox(width: 18),
            Expanded(
              child: Align(
                alignment: Alignment.centerRight,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 420),
                  child: WoxDropdownButton<String>(
                    value: currentValue,
                    items: items,
                    onChanged: (value) {
                      if (value == null) return;
                      final ref = _parseGlanceKey(value);
                      onPrimaryGlanceChanged(jsonEncode(ref.toJson()));
                    },
                    isExpanded: true,
                  ),
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }

  GlanceRef _parseGlanceKey(String key) {
    final parts = key.split('\x00');
    if (parts.length != 2) {
      return GlanceRef.empty();
    }
    return GlanceRef(pluginId: parts[0], glanceId: parts[1]);
  }
}
