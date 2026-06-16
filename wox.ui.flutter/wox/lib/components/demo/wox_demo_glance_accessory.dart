part of 'wox_demo.dart';

class WoxDemoLogoMark extends StatelessWidget {
  const WoxDemoLogoMark({super.key});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 28,
      height: 28,
      decoration: BoxDecoration(color: Colors.white, borderRadius: BorderRadius.circular(6)),
      alignment: Alignment.center,
      child: Text('W', style: TextStyle(color: Colors.black.withValues(alpha: 0.92), fontSize: 20, fontWeight: FontWeight.w900, height: 1)),
    );
  }
}

class WoxDemoGlanceAccessory extends StatelessWidget {
  const WoxDemoGlanceAccessory({super.key, required this.label, required this.value, required this.icon});

  final String label;
  final String value;
  final WoxImage? icon;

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final baseTextColor = WoxThemeUtil.instance.currentTheme.value.queryBoxFontColorParsed;
    final accessoryColor = baseTextColor.withValues(alpha: 0.8);

    // Feature refinement: the real launcher renders Glance as a lightweight
    // inline query accessory, not as a bordered badge. Matching that shape here
    // keeps the onboarding preview aligned with the production window chrome.
    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: metrics.queryBoxGlanceMaxWidth),
      child: Container(
        height: metrics.scaledSpacing(30),
        padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(8)),
        decoration: BoxDecoration(color: Colors.transparent, borderRadius: BorderRadius.circular(5)),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            WoxDemoGlanceInlineIcon(icon: icon, fallback: Icons.schedule_outlined, color: accessoryColor, size: metrics.scaledSpacing(16)),
            SizedBox(width: metrics.scaledSpacing(5)),
            Flexible(
              child: Text(
                value.isEmpty ? label : value,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: accessoryColor, fontSize: metrics.queryBoxGlanceFontSize),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class WoxDemoGlanceInlineIcon extends StatelessWidget {
  const WoxDemoGlanceInlineIcon({super.key, required this.icon, required this.fallback, required this.color, required this.size});

  final WoxImage? icon;
  final IconData fallback;
  final Color color;
  final double size;

  @override
  Widget build(BuildContext context) {
    if (icon != null && icon!.imageData.isNotEmpty) {
      // Feature change: inline Glance previews render the icon returned by the
      // API, which preserves state-specific glyphs while retaining the fallback
      // eye for missing or not-yet-loaded responses.
      return WoxImageView(woxImage: icon!, width: size, height: size, svgColor: color);
    }

    return Icon(fallback, color: color, size: size);
  }
}
