import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';

class WoxPluginStoreOnboarding extends StatelessWidget {
  const WoxPluginStoreOnboarding({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: const ValueKey('onboarding-media-wpmInstall'),
      content: WoxOnboardingInfoPanel(key: const ValueKey('onboarding-wpm-install-page'), body: tr('onboarding_wpm_install_body')),
      demo: WoxPluginStoreDemo(accent: accent, tr: tr),
    );
  }
}
