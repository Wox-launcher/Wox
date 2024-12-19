import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:wox/entity/wox_theme.dart';

class WoxThemeIconView extends StatelessWidget {
  final WoxTheme theme;
  final double? width;
  final double? height;

  const WoxThemeIconView({super.key, required this.theme, this.width, this.height});

  @override
  Widget build(BuildContext context) {
    Color backgroundColor = fromCssColor(theme.appBackgroundColor);
    Color queryBoxColor = fromCssColor(theme.queryBoxBackgroundColor);
    Color resultItemActiveColor = fromCssColor(theme.resultItemActiveBackgroundColor);

    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(8),
        color: backgroundColor,
      ),
      child: Padding(
        padding: const EdgeInsets.only(left: 4, right: 4, top: 4),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Query box
            Container(
              width: width,
              height: 10,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(4),
                color: queryBoxColor,
              ),
            ),
            const SizedBox(height: 4),
            // Result items
            Column(
              children: [
                Container(
                  height: 5,
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(2),
                    color: resultItemActiveColor,
                  ),
                ),
                const SizedBox(height: 6),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
