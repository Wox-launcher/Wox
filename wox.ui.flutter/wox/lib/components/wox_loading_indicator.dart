import 'package:flutter/material.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxLoadingIndicator extends StatelessWidget {
  final double size;
  final double strokeWidth;
  final Color? color;

  const WoxLoadingIndicator({super.key, this.size = 16, this.strokeWidth = 2, this.color});

  @override
  Widget build(BuildContext context) {
    final currentTheme = WoxThemeUtil.instance.currentTheme.value;

    return SizedBox(
      width: size,
      height: size,
      child: CircularProgressIndicator(
        strokeWidth: strokeWidth,
        strokeCap: StrokeCap.round,
        color: color ?? currentTheme.resultItemActiveBackgroundColorParsed,
        backgroundColor: currentTheme.resultItemTitleColorParsed.withValues(alpha: 0.15),
      ),
    );
  }
}
