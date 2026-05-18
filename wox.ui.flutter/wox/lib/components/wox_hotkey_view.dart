import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/wox_hotkey_display_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_text_measure_util.dart';

class WoxHotkeyView extends StatelessWidget {
  final HotkeyX hotkey;
  final Color backgroundColor;
  final Color borderColor;
  final Color textColor;

  const WoxHotkeyView({super.key, required this.hotkey, required this.backgroundColor, required this.borderColor, required this.textColor});

  static TextStyle keyTextStyle({required Color color}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return TextStyle(fontFamily: 'SFProDisplay', fontSize: metrics.tailHotkeyFontSize, fontWeight: FontWeight.w500, color: color);
  }

  static double measureSingleKeyWidth(BuildContext context, String key) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final minWidth = metrics.scaledSpacing(28);
    final horizontalPadding = metrics.scaledSpacing(7);
    final textWidth = WoxTextMeasureUtil.measureTextWidth(context: context, text: key, style: keyTextStyle(color: Colors.transparent));
    // Feature fix: platform-specific text labels can be wider than the old
    // macOS glyphs, so hotkey chips must grow
    // with their label instead of clipping inside a fixed 28 px box.
    return math.max(minWidth, textWidth + horizontalPadding * 2);
  }

  static double measureHotkeyWidth(BuildContext context, HotkeyX hotkey) {
    final labels = WoxHotkeyDisplayUtil.labelsFromHotkey(hotkey);
    if (labels.isEmpty) {
      return 0;
    }

    final spacing = WoxInterfaceSizeUtil.instance.current.toolbarHotkeyKeySpacing;
    return labels.map((label) => measureSingleKeyWidth(context, label)).fold(0.0, (total, width) => total + width) + (labels.length - 1) * spacing;
  }

  Widget buildSingleKey(BuildContext context, String key) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final keyWidth = measureSingleKeyWidth(context, key);
    final keyHeight = metrics.scaledSpacing(22);

    return Container(
      constraints: BoxConstraints.tight(Size(keyWidth, keyHeight)),
      decoration: BoxDecoration(
        color: backgroundColor,
        border: Border.all(color: borderColor),
        borderRadius: BorderRadius.circular(4),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.1), blurRadius: 2, offset: const Offset(0, 1))],
      ),
      child: Center(child: Text(key, style: keyTextStyle(color: textColor))),
    );
  }

  @override
  Widget build(BuildContext context) {
    var hotkeyWidgets = <Widget>[];
    hotkeyWidgets.addAll(WoxHotkeyDisplayUtil.labelsFromHotkey(hotkey).map((key) => buildSingleKey(context, key)));

    return Wrap(
      // Hotkey chips appear in result tails and the toolbar, so their physical
      // key boxes follow density along with the text instead of staying locked
      // to the previous normal-size literals.
      spacing: WoxInterfaceSizeUtil.instance.current.scaledSpacing(4),
      children: hotkeyWidgets,
    );
  }
}
