part of 'wox_demo.dart';

class _ThemeSwatchIcon extends StatelessWidget {
  const _ThemeSwatchIcon({required this.background, required this.accent, required this.highlight});

  final Color background;
  final Color accent;
  final Color highlight;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 28,
      height: 28,
      decoration: BoxDecoration(color: background, borderRadius: BorderRadius.circular(7), border: Border.all(color: Colors.white.withValues(alpha: 0.16))),
      child: Stack(
        children: [
          Positioned(left: 5, right: 5, top: 7, child: Container(height: 4, decoration: BoxDecoration(color: accent, borderRadius: BorderRadius.circular(999)))),
          Positioned(left: 5, right: 11, top: 14, child: Container(height: 4, decoration: BoxDecoration(color: highlight, borderRadius: BorderRadius.circular(999)))),
          Positioned(
            left: 5,
            right: 15,
            top: 20,
            child: Container(height: 3, decoration: BoxDecoration(color: Colors.white.withValues(alpha: 0.72), borderRadius: BorderRadius.circular(999))),
          ),
        ],
      ),
    );
  }
}
