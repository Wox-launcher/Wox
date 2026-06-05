import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/utils/wox_platform_hotkey_util.dart';

class WoxFinishOnboarding extends StatelessWidget {
  const WoxFinishOnboarding({
    super.key,
    required this.accent,
    required this.tr,
    required this.glanceEnabled,
    required this.glanceLabel,
    required this.glanceValue,
    required this.glanceIcon,
  });

  final Color accent;
  final String Function(String key) tr;
  final bool glanceEnabled;
  final String glanceLabel;
  final String glanceValue;
  final WoxImage? glanceIcon;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: ValueKey('onboarding-media-finish-$glanceEnabled-$glanceLabel-$glanceValue-${glanceIcon?.imageType}-${glanceIcon?.imageData}'),
      content: WoxOnboardingInfoPanel(key: const ValueKey('onboarding-finish-page'), body: tr('onboarding_finish_card_body')),
      demo: WoxDemoFramedDesktop(
        accent: accent,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(48, 44, 52, 44),
          child: WoxDemoWindow(
            accent: accent,
            query: 'ready',
            queryAccessory: _buildGlanceAccessory(),
            results: [
              WoxDemoResult(
                title: tr('onboarding_finish_card_title'),
                subtitle: tr('onboarding_finish_card_body'),
                icon: const Icon(Icons.check_rounded, color: Colors.white, size: 24),
                selected: true,
                tail: tr('onboarding_finish_badge'),
              ),
              const WoxDemoResult(title: 'Open Wox Settings', subtitle: r'C:\Users\qianl\AppData\Roaming\Wox', icon: WoxDemoLogoMark()),
              WoxDemoResult(
                title: tr('onboarding_action_panel_title'),
                subtitle: tr('onboarding_action_panel_description'),
                icon: Icon(Icons.play_arrow_rounded, color: accent, size: 23),
                tail: WoxPlatformHotkeyUtil.primaryHotkeyLabel('j'),
              ),
              WoxDemoResult(
                title: tr('onboarding_query_hotkeys_title'),
                subtitle: tr('onboarding_query_shortcuts_title'),
                icon: const Icon(Icons.manage_search_rounded, color: Color(0xFFA78BFA), size: 23),
                tail: tr('ui_tray_queries'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget? _buildGlanceAccessory() {
    if (!glanceEnabled) {
      return null;
    }

    // Step extraction: finish owns the summary preview, including the optional
    // Glance accessory that earlier steps may have enabled.
    return WoxDemoGlanceAccessory(
      key: ValueKey('mini-glance-pill-$glanceLabel-$glanceValue-${glanceIcon?.imageType}-${glanceIcon?.imageData}'),
      label: glanceLabel,
      value: glanceValue,
      icon: glanceIcon,
    );
  }
}
