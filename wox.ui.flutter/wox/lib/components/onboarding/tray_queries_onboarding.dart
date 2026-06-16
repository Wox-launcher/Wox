import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';

class WoxTrayQueriesOnboarding extends StatelessWidget {
  const WoxTrayQueriesOnboarding({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: const ValueKey('onboarding-media-trayQueries'),
      content: WoxOnboardingInfoPanel(key: const ValueKey('onboarding-tray-queries-page'), body: tr('onboarding_tray_queries_body')),
      demo: WoxTrayQueriesDemo(accent: accent, tr: tr),
    );
  }
}
