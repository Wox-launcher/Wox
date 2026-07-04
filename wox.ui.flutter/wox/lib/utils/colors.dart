import 'dart:ui';
import 'package:flutter/material.dart' show Colors, HSLColor;
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

Color getThemeActiveBackgroundColor() {
  return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveBackgroundColor);
}

Color getThemeActiveTextColor() {
  return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor);
}

Color getThemeTextColor() {
  return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemTitleColor);
}

Color getThemeSubTextColor() {
  return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor);
}

Color getThemeBackgroundColor() {
  return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor, defaultColor: const Color(0xFF1F1F1F));
}

bool isThemeDark() {
  return getThemeBackgroundColor().computeLuminance() < 0.5;
}

Color getThemeDividerColor() {
  return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.previewSplitLineColor);
}

Color getThemeSettingDividerColor() {
  // Settings panes previously mixed raw divider colors, dimmed divider colors,
  // and Material defaults, so adjacent separators had visibly different weight.
  // Use the unmodified theme divider token here to match the plugin pane splitter.
  return getThemeDividerColor();
}

Color getThemeActionItemActiveColor() {
  return safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor);
}

Color getThemePanelBackgroundColor() {
  final panelColor = WoxThemeUtil.instance.currentTheme.value.actionContainerBackgroundColor;
  return safeFromCssColor(panelColor, defaultColor: getThemeCardBackgroundColor());
}

Color getThemePopupSurfaceColor() {
  return isThemeDark() ? const Color(0xFF242424) : Colors.white;
}

Color getThemePopupBarrierColor() {
  return Colors.black.withValues(alpha: isThemeDark() ? 0.58 : 0.36);
}

Color getThemePopupOutlineColor() {
  return getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.22 : 0.15);
}

Color getThemeCardBackgroundColor() {
  Color baseColor = getThemeBackgroundColor();
  bool isDarkTheme = baseColor.computeLuminance() < 0.5;
  // Flutter exposes Color channels as normalized doubles, so convert them
  // before applying the legacy 20-step RGB offset.
  int shiftChannel(double channel, int offset) {
    return ((channel * 255).round() + offset).clamp(0, 255).toInt();
  }

  if (isDarkTheme) {
    return Color.fromRGBO(shiftChannel(baseColor.r, 20), shiftChannel(baseColor.g, 20), shiftChannel(baseColor.b, 20), 1.0);
  } else {
    return Color.fromRGBO(shiftChannel(baseColor.r, -20), shiftChannel(baseColor.g, -20), shiftChannel(baseColor.b, -20), 1.0);
  }
}

extension ColorsExtension on Color {
  /// Returns a darker version of the color.
  ///
  /// [weight] is a value between 0 and 100 that determines how much darker the color should be.
  /// A weight of 0 returns the original color, while a weight of 100 returns black.
  Color darker([int weight = 10]) {
    weight = weight.clamp(0, 100);
    final hslColor = HSLColor.fromColor(this);
    final newLightness = (hslColor.lightness * (100 - weight) / 100).clamp(0.0, 1.0);
    return hslColor.withLightness(newLightness).toColor();
  }

  /// Returns a lighter version of the color.
  ///
  /// [weight] is a value between 0 and 100 that determines how much lighter the color should be.
  /// A weight of 0 returns the original color, while a weight of 100 returns white.
  Color lighter([int weight = 10]) {
    weight = weight.clamp(0, 100);
    final hslColor = HSLColor.fromColor(this);
    final newLightness = (hslColor.lightness + (1 - hslColor.lightness) * weight / 100).clamp(0.0, 1.0);
    return hslColor.withLightness(newLightness).toColor();
  }
}
