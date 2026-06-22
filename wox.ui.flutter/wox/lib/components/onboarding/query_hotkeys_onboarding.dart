import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';

class WoxQueryHotkeysOnboarding extends StatelessWidget {
  const WoxQueryHotkeysOnboarding({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: const ValueKey('onboarding-media-queryHotkeys'),
      content: WoxOnboardingInfoPanel(key: const ValueKey('onboarding-query-hotkeys-page'), body: tr('onboarding_query_hotkeys_body')),
      demo: WoxQueryHotkeysDemo(accent: accent, tr: tr),
    );
  }
}
