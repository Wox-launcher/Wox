import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/onboarding/wox_onboarding_step_layout.dart';
import 'package:wox/components/onboarding/wox_onboarding_style.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/utils/colors.dart';

class WoxWelcomeOnboarding extends StatelessWidget {
  const WoxWelcomeOnboarding({super.key, required this.accent, required this.tr, required this.languagesFuture, required this.currentLangCode, required this.onLangChanged});

  final Color accent;
  final String Function(String key) tr;
  final Future<List<WoxLang>> languagesFuture;
  final String currentLangCode;
  final Future<void> Function(String value) onLangChanged;

  @override
  Widget build(BuildContext context) {
    return WoxOnboardingStepLayout(
      previewKey: const ValueKey('onboarding-media-welcome'),
      content: WoxOnboardingSettingsPanel(
        key: const ValueKey('onboarding-welcome-page'),
        children: [
          // Readability fix: the welcome intro is standalone explanatory text,
          // so it should use the same primary-color, normal-weight style as the
          // other onboarding descriptions instead of the dim subtitle color.
          Text(tr('onboarding_welcome_card_body'), style: TextStyle(color: getThemeTextColor(), fontSize: 14, height: 1.5)),
          const SizedBox(height: 20),
          // Glass-dark refresh: the separator uses the same neutral outline as
          // the surrounding panel instead of a subtitle-derived line, which was
          // too dim against the translucent launcher background.
          Container(height: 1, color: WoxOnboardingGlassStyle.outline(0.11)),
          const SizedBox(height: 18),
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Text(tr('ui_lang'), style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w600)),
              const SizedBox(width: 18),
              Expanded(
                child: Align(
                  alignment: Alignment.centerRight,
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 320),
                    child: FutureBuilder<List<WoxLang>>(
                      future: languagesFuture,
                      builder: (context, snapshot) {
                        if (snapshot.connectionState != ConnectionState.done) {
                          return Text(tr('onboarding_loading'), textAlign: TextAlign.right, style: TextStyle(color: getThemeSubTextColor(), fontSize: 13));
                        }

                        final languages = snapshot.data ?? const <WoxLang>[];
                        if (languages.isEmpty) {
                          return Text(currentLangCode, textAlign: TextAlign.right, style: TextStyle(color: getThemeSubTextColor(), fontSize: 13));
                        }

                        // Step extraction: language selection lives with the
                        // welcome step because only this step presents it. The
                        // parent still owns persistence through onLangChanged.
                        return WoxDropdownButton<String>(
                          key: const ValueKey('onboarding-language-dropdown'),
                          items: languages.map((language) => WoxDropdownItem(value: language.code, label: language.name)).toList(),
                          value: currentLangCode,
                          onChanged: (value) {
                            if (value != null) {
                              unawaited(onLangChanged(value));
                            }
                          },
                          isExpanded: true,
                        );
                      },
                    ),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
      demo: WoxQueryConceptDemo(accent: accent, tr: tr),
    );
  }
}
