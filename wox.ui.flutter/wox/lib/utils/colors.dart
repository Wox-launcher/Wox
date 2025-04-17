import 'dart:ui';
import 'package:from_css_color/from_css_color.dart';
import 'package:wox/utils/wox_theme_util.dart';

Color getThemeActiveBackgroundColor() {
  return fromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor);
}

Color getThemeTextColor() {
  return fromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor);
}

Color getThemeSubTextColor() {
  return fromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor);
}

Color getThemeBackgroundColor() {
  return fromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor);
}

Color getThemeDividerColor() {
  return fromCssColor(WoxThemeUtil.instance.currentTheme.value.previewSplitLineColor);
}

Color getThemeActionItemActiveColor() {
  return fromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor);
}

Color getThemeCardBackgroundColor() {
  Color baseColor = getThemeBackgroundColor();
  bool isDarkTheme = baseColor.computeLuminance() < 0.5;
  if (isDarkTheme) {
    return Color.fromRGBO(
      (baseColor.r + 20 > 255 ? 255 : baseColor.r + 20).toInt(),
      (baseColor.g + 20 > 255 ? 255 : baseColor.g + 20).toInt(),
      (baseColor.b + 20 > 255 ? 255 : baseColor.b + 20).toInt(),
      1.0,
    );
  } else {
    return Color.fromRGBO(
      (baseColor.r - 20 < 0 ? 0 : baseColor.r - 20).toInt(),
      (baseColor.g - 20 < 0 ? 0 : baseColor.g - 20).toInt(),
      (baseColor.b - 20 < 0 ? 0 : baseColor.b - 20).toInt(),
      1.0,
    );
  }
}
