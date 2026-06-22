part of 'wox_demo.dart';

class WoxDemoActionPanel extends StatelessWidget {
  const WoxDemoActionPanel({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.94),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: getThemeTextColor().withValues(alpha: 0.07)),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.18), blurRadius: 28, offset: const Offset(0, 16))],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Actions', style: TextStyle(color: getThemeTextColor(), fontSize: 15, fontWeight: FontWeight.w700)),
          const SizedBox(height: 9),
          Container(height: 1, color: getThemeTextColor().withValues(alpha: 0.54)),
          const SizedBox(height: 8),
          _MiniActionRow(accent: accent, icon: Icons.play_arrow_rounded, title: 'Execute', selected: true),
          const SizedBox(height: 8),
          _MiniActionRow(accent: accent, icon: Icons.push_pin_outlined, title: tr('onboarding_action_panel_copy')),
          const SizedBox(height: 8),
          _MiniActionRow(accent: accent, icon: Icons.more_horiz, title: tr('onboarding_action_panel_more')),
        ],
      ),
    );
  }
}

class _MiniActionRow extends StatelessWidget {
  const _MiniActionRow({required this.accent, required this.icon, required this.title, this.selected = false});

  final Color accent;
  final IconData icon;
  final String title;
  final bool selected;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 32,
      padding: const EdgeInsets.symmetric(horizontal: 9),
      decoration: BoxDecoration(color: selected ? accent.withValues(alpha: 0.82) : getThemeTextColor().withValues(alpha: 0.055), borderRadius: BorderRadius.circular(7)),
      child: Row(
        children: [
          Icon(icon, size: 17, color: selected ? Colors.white : accent),
          const SizedBox(width: 9),
          Expanded(
            child: Text(
              title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: selected ? Colors.white : getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w700),
            ),
          ),
        ],
      ),
    );
  }
}
