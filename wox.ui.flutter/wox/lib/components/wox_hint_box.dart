import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

class WoxHintBox extends StatelessWidget {
  final String text;
  final IconData icon;
  final EdgeInsetsGeometry padding;

  const WoxHintBox({
    super.key,
    required this.text,
    this.icon = Icons.info_outline,
    this.padding = const EdgeInsets.all(12),
  });

  @override
  Widget build(BuildContext context) {
    final Color baseBackground = getThemeBackgroundColor();
    final bool isDarkTheme = baseBackground.computeLuminance() < 0.5;

    final Color accentColor =
        isDarkTheme ? Colors.lightBlueAccent : Colors.blue;
    final Color backgroundColor =
        accentColor.withValues(alpha: isDarkTheme ? 0.14 : 0.10);
    final Color borderColor =
        accentColor.withValues(alpha: isDarkTheme ? 0.35 : 0.30);

    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: borderColor),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 16, color: accentColor),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              text,
              style: TextStyle(
                  color: getThemeTextColor(), fontSize: 13, height: 1.35),
            ),
          ),
        ],
      ),
    );
  }
}
