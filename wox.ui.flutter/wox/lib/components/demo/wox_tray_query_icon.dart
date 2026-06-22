part of 'wox_demo.dart';

class _TrayQueryIcon extends StatelessWidget {
  const _TrayQueryIcon({required this.accent, required this.pressed, this.size = 28, this.menuBarStyle = false});

  final Color accent;
  final bool pressed;
  final double size;
  final bool menuBarStyle;

  @override
  Widget build(BuildContext context) {
    final idleBackground = menuBarStyle ? Colors.transparent : getThemeBackgroundColor().withValues(alpha: 0.88);
    final idleBorderColor = menuBarStyle ? Colors.transparent : getThemeTextColor().withValues(alpha: 0.14);
    final iconSize = menuBarStyle ? size * 0.84 : size * 0.56;

    // Feature refinement: the tray query affordance should read as a real
    // system-tray glyph, not a launcher-sized button. Keeping the size explicit
    // also lets the tray demo anchor Wox geometry to the same visual bounds.
    // Bug fix: macOS menu-bar mode removes the idle button frame and uses a
    // smaller glyph so the status item aligns with the simulated Finder bar.
    return AnimatedContainer(
      duration: const Duration(milliseconds: 140),
      width: size,
      height: size,
      decoration: BoxDecoration(
        color: pressed ? accent.withValues(alpha: 0.90) : idleBackground,
        border: Border.all(color: pressed ? accent : idleBorderColor),
        borderRadius: BorderRadius.circular(menuBarStyle ? 5 : 8),
        boxShadow:
            menuBarStyle
                ? (pressed ? [BoxShadow(color: accent.withValues(alpha: 0.18), blurRadius: 8)] : const [])
                : [BoxShadow(color: accent.withValues(alpha: pressed ? 0.24 : 0.08), blurRadius: pressed ? 14 : 8)],
      ),
      child: Icon(Icons.wb_sunny_outlined, color: pressed ? Colors.white : accent, size: iconSize),
    );
  }
}
