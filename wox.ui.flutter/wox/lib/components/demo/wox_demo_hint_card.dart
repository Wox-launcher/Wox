part of 'wox_demo.dart';

class _ExpansionBadge extends StatelessWidget {
  const _ExpansionBadge({required this.accent, required this.from, required this.to});

  final Color accent;
  final String from;
  final String to;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.94),
        border: Border.all(color: accent.withValues(alpha: 0.32)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.12), blurRadius: 20, offset: const Offset(0, 10))],
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(from, style: TextStyle(color: accent, fontSize: 12, fontWeight: FontWeight.w800)),
          Padding(padding: const EdgeInsets.symmetric(horizontal: 8), child: Icon(Icons.arrow_forward_rounded, color: getThemeSubTextColor(), size: 16)),
          Text(to, style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w700)),
        ],
      ),
    );
  }
}

class WoxDemoHintCard extends StatelessWidget {
  // progress parameter is kept for API compatibility but no longer affects
  // visibility. The badge is always shown so users can immediately read the
  // from→to mapping without waiting for the animation to reach it.
  const WoxDemoHintCard({super.key, required this.accent, required this.icon, required this.title, required this.from, required this.to, this.progress = 1});

  final Color accent;
  final IconData icon;
  final String title;
  final String from;
  final String to;
  final double progress;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 11),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.92),
        border: Border.all(color: getThemeTextColor().withValues(alpha: 0.10)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.12), blurRadius: 22, offset: const Offset(0, 10))],
      ),
      // Left-right split: title occupies the left half, badge the right half.
      // Both sides are Expanded so the dividing point stays at the center
      // regardless of text length or container width.
      child: Row(
        children: [
          Expanded(
            child: Row(
              children: [
                Icon(icon, color: accent, size: 20),
                const SizedBox(width: 8),
                Flexible(child: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w700))),
              ],
            ),
          ),
          Expanded(child: Align(alignment: Alignment.centerRight, child: _ExpansionBadge(accent: accent, from: from, to: to))),
        ],
      ),
    );
  }
}
