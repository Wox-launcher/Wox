import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';

class WoxThemeInstallOnboarding extends StatelessWidget {
  const WoxThemeInstallOnboarding({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: const ValueKey('onboarding-media-themeInstall'),
      content: WoxOnboardingInfoPanel(key: const ValueKey('onboarding-theme-install-page'), body: tr('onboarding_theme_install_body')),
      demo: WoxThemeInstallDemo(accent: accent, tr: tr),
    );
  }
}
