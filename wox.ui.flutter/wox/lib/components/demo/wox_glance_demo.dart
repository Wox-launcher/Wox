part of 'wox_demo.dart';

class WoxGlanceDemo extends StatelessWidget {
  const WoxGlanceDemo({super.key, required this.accent, required this.enabled, required this.label, required this.value, required this.icon, required this.tr});

  final Color accent;
  final bool enabled;
  final String label;
  final String value;
  final WoxImage? icon;
  final String Function(String key) tr;

  Widget? _buildAccessory() {
    if (!enabled) {
      return null;
    }

    return WoxDemoGlanceAccessory(key: ValueKey('mini-glance-pill-$label-$value-${icon?.imageType}-${icon?.imageData}'), label: label, value: value, icon: icon);
  }

  @override
  Widget build(BuildContext context) {
    // Feature extraction: Glance was previously assembled inline in onboarding. Moving the full demo here keeps the reusable API parameter-driven and lets settings pages preview the same label/value/icon behavior later.
    return WoxDemoFramedDesktop(
      accent: accent,
      child: Padding(
        // Symmetric vertical padding centres the Wox window within the desktop
        // frame. The previous 82/44 split was inherited from demos that carry a
        // hint strip at the top; the Glance demo does not use one.
        padding: const EdgeInsets.fromLTRB(48, 44, 52, 44),
        child: WoxDemoWindow(
          accent: accent,
          query: 'wox',
          queryAccessory: _buildAccessory(),
          opaqueBackground: true,
          results: [
            WoxDemoResult(
              title: enabled ? label : tr('ui_glance_enable'),
              subtitle: enabled ? tr('onboarding_glance_description') : tr('ui_glance_enable_tips'),
              icon: WoxDemoGlanceInlineIcon(
                icon: enabled ? icon : null,
                fallback: enabled ? Icons.remove_red_eye_outlined : Icons.visibility_off_outlined,
                color: Colors.white,
                size: 22,
              ),
              selected: true,
              tail: enabled ? value : '',
            ),
            WoxDemoResult(
              title: tr('onboarding_glance_sample_provider'),
              subtitle: tr('onboarding_glance_loading_body'),
              icon: Icon(Icons.bolt_outlined, color: accent, size: 22),
              tail: 'Glance',
            ),
            WoxDemoResult(
              title: tr('ui_glance_primary'),
              subtitle: tr('onboarding_glance_picker_label'),
              icon: const Icon(Icons.push_pin_outlined, color: Color(0xFF60A5FA), size: 22),
              tail: enabled ? label : '',
            ),
          ],
        ),
      ),
    );
  }
}
