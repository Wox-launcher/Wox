import 'package:flutter/material.dart';
import 'package:wox/components/onboarding/wox_onboarding_style.dart';
import 'package:wox/utils/colors.dart';

class WoxOnboardingStepLayout extends StatelessWidget {
  const WoxOnboardingStepLayout({super.key, required this.content, required this.demo, required this.previewKey});

  final Widget content;
  final Widget demo;
  final Key previewKey;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Refactor boundary: individual onboarding steps now own their content
        // and demo, while the view owns the surrounding title/sidebar/footer
        // layout. Keeping this shared column here prevents each step from
        // reimplementing the same settings-to-demo spacing.
        Align(alignment: Alignment.topLeft, child: content),
        const SizedBox(height: 18),
        Expanded(
          child: ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: TweenAnimationBuilder<double>(
              key: previewKey,
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeOutCubic,
              tween: Tween(begin: 0, end: 1),
              builder: (context, value, child) {
                return Opacity(opacity: value, child: Transform.translate(offset: Offset(0, 8 * (1 - value)), child: child));
              },
              child: Container(
                padding: const EdgeInsets.all(10),
                // The preview shares the remaining onboarding height. A fixed
                // minimum overflows on Windows CI when About reopens onboarding
                // from the smaller management-view transition state.
                decoration: BoxDecoration(
                  color: WoxOnboardingGlassStyle.surface(0.052),
                  border: Border.all(color: WoxOnboardingGlassStyle.outline(0.10)),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: demo,
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class WoxOnboardingSettingsPanel extends StatelessWidget {
  const WoxOnboardingSettingsPanel({super.key, required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(22),
      decoration: BoxDecoration(
        color: WoxOnboardingGlassStyle.surface(),
        border: Border.all(color: WoxOnboardingGlassStyle.outline(0.13)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: WoxOnboardingGlassStyle.panelShadow(0.10),
      ),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: children),
    );
  }
}

class WoxOnboardingInfoPanel extends StatelessWidget {
  const WoxOnboardingInfoPanel({super.key, this.title, required this.body, this.badge});

  final String? title;
  final String body;
  final String? badge;

  @override
  Widget build(BuildContext context) {
    // Readability fix: when an info panel has no separate title, its body text
    // is the primary instruction for the step. The previous subtitle color made
    // these standalone descriptions look faded on dark translucent onboarding
    // panels, especially over the subtle acrylic backdrop.
    final standaloneBodyStyle = TextStyle(color: getThemeTextColor(), fontSize: 14, height: 1.5);
    final detailBodyStyle = TextStyle(color: getThemeSubTextColor(), fontSize: 14, height: 1.5);
    final badgeWidget =
        badge != null
            ? Container(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
              decoration: BoxDecoration(
                color: WoxOnboardingGlassStyle.activeSurface(0.12),
                border: Border.all(color: WoxOnboardingGlassStyle.outline(0.14)),
                borderRadius: BorderRadius.circular(16),
              ),
              child: Text(badge!, style: TextStyle(color: getThemeActiveBackgroundColor(), fontSize: 12, fontWeight: FontWeight.w600)),
            )
            : null;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(22),
      decoration: BoxDecoration(
        color: WoxOnboardingGlassStyle.surface(),
        border: Border.all(color: WoxOnboardingGlassStyle.outline(0.13)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: WoxOnboardingGlassStyle.panelShadow(0.10),
      ),
      child:
          title != null
              ? Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Expanded(child: Text(title!, style: TextStyle(color: getThemeTextColor(), fontSize: 17, fontWeight: FontWeight.w600))),
                      if (badgeWidget != null) ...[const SizedBox(width: 14), badgeWidget],
                    ],
                  ),
                  const SizedBox(height: 12),
                  Text(body, style: detailBodyStyle),
                ],
              )
              : badgeWidget != null
              ? Row(crossAxisAlignment: CrossAxisAlignment.center, children: [Expanded(child: Text(body, style: standaloneBodyStyle)), const SizedBox(width: 14), badgeWidget])
              : Text(body, style: standaloneBodyStyle),
    );
  }
}
