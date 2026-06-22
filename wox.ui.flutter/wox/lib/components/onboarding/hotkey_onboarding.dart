import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/colors.dart';

class WoxMainHotkeyOnboarding extends StatelessWidget {
  const WoxMainHotkeyOnboarding({super.key, required this.accent, required this.hotkey, required this.tr, required this.onHotkeyChanged});

  final Color accent;
  final String hotkey;
  final String Function(String key) tr;
  final void Function(String hotkey) onHotkeyChanged;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: ValueKey('onboarding-media-mainHotkey-$hotkey'),
      content: _HotkeyContent(
        accent: accent,
        title: tr('onboarding_main_hotkey_title'),
        body: tr('onboarding_main_hotkey_description'),
        hotkey: hotkey,
        onHotkeyChanged: onHotkeyChanged,
      ),
      demo: WoxMainHotkeyDemo(accent: accent, hotkey: hotkey, tr: tr),
    );
  }
}

class WoxSelectionHotkeyOnboarding extends StatelessWidget {
  const WoxSelectionHotkeyOnboarding({super.key, required this.accent, required this.hotkey, required this.tr, required this.onHotkeyChanged});

  final Color accent;
  final String hotkey;
  final String Function(String key) tr;
  final void Function(String hotkey) onHotkeyChanged;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: ValueKey('onboarding-media-selectionHotkey-$hotkey'),
      content: _HotkeyContent(
        accent: accent,
        title: tr('onboarding_selection_hotkey_title'),
        body: tr('onboarding_selection_hotkey_description'),
        hotkey: hotkey,
        onHotkeyChanged: onHotkeyChanged,
      ),
      demo: WoxSelectionHotkeyDemo(accent: accent, hotkey: hotkey, tr: tr),
    );
  }
}

class _HotkeyContent extends StatelessWidget {
  const _HotkeyContent({required this.accent, required this.title, required this.body, required this.hotkey, required this.onHotkeyChanged});

  final Color accent;
  final String title;
  final String body;
  final String hotkey;
  final void Function(String hotkey) onHotkeyChanged;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();

    // Hotkey redesign: mirror the permissions checklist layout so onboarding
    // steps use a consistent left context / right control rhythm. The previous
    // vertical stack made the recorder feel detached from its explanation.
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 16),
      decoration: BoxDecoration(color: textColor.withValues(alpha: 0.038), border: Border.all(color: subTextColor.withValues(alpha: 0.18)), borderRadius: BorderRadius.circular(8)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 16, fontWeight: FontWeight.w700)),
                const SizedBox(height: 6),
                Text(body, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(color: subTextColor, fontSize: 13, height: 1.35)),
              ],
            ),
          ),
          const SizedBox(width: 18),
          Align(
            alignment: Alignment.centerRight,
            child: WoxHotkeyRecorder(
              hotkey: WoxHotkey.parseHotkeyFromString(hotkey),
              onHotKeyRecorded: (value) {
                // Step extraction: the recorder UI is reusable for both hotkey
                // steps, but the parent still decides which setting key is saved.
                onHotkeyChanged(value);
              },
            ),
          ),
        ],
      ),
    );
  }
}
