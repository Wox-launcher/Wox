part of 'wox_demo.dart';

class WoxThemeInstallDemo extends StatelessWidget {
  const WoxThemeInstallDemo({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    return _ThemeStoreDemo(
      demoKey: const ValueKey('onboarding-theme-install-demo'),
      accent: accent,
      icon: Icons.palette_outlined,
      title: tr('onboarding_theme_install_title'),
      hintFrom: 'theme',
      hintTo: tr('onboarding_theme_install_hint_target'),
      queryStages: const ['', 't', 'theme', 'theme ocean', 'theme ocean dark'],
      installLabel: tr('plugin_theme_install_theme'),
      installingLabel: tr('plugin_wpm_installing'),
      installedLabel: tr('ui_setting_theme_apply'),
      primaryTitle: 'Ocean Dark',
      primarySubtitle: tr('onboarding_theme_install_result_subtitle'),
      primaryIcon: const _ThemeSwatchIcon(background: Color(0xFF0F172A), accent: Color(0xFF38BDF8), highlight: Color(0xFF22C55E)),
      secondaryResults: [
        WoxDemoResult(
          title: 'Aurora',
          subtitle: tr('plugin_theme_group_store'),
          icon: const _ThemeSwatchIcon(background: Color(0xFF261A3D), accent: Color(0xFFE879F9), highlight: Color(0xFFFACC15)),
          tail: tr('plugin_theme_install_theme'),
        ),
        WoxDemoResult(
          title: 'Default Dark',
          subtitle: tr('plugin_theme_group_current'),
          icon: const _ThemeSwatchIcon(background: Color(0xFF1F2937), accent: Color(0xFF60A5FA), highlight: Color(0xFF94A3B8)),
          tail: tr('ui_setting_theme_system_tag'),
        ),
      ],
      // Theme install feature: after the animation applies Ocean Dark, the demo
      // window switches to these colors so users see the real visual effect of
      // the theme change rather than the launcher staying in the current theme.
      appliedTheme: const _DemoThemeData(
        background: Color(0xFF0F172A),
        accent: Color(0xFF38BDF8),
        queryBarBackground: Color(0xFF1E293B),
        queryBarText: Color(0xFFE0F2FE),
        resultTitleColor: Color(0xFFCBD5E1),
        resultSubtitleColor: Color(0xFF64748B),
        resultActiveBackground: Color(0xFF1E3A5F),
        resultActiveTitleColor: Color(0xFFFFFFFF),
        resultActiveSubtitleColor: Color(0xFF7DD3FC),
        tailColor: Color(0xFF475569),
        activeTailColor: Color(0xFF38BDF8),
        textColor: Color(0xFF94A3B8),
      ),
    );
  }
}
