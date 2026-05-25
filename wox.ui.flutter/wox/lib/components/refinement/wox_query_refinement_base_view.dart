import 'package:flutter/material.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_hotkey_display_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

abstract class WoxQueryRefinementBaseView extends StatelessWidget {
  final WoxQueryRefinement refinement;
  final List<String> selectedValues;
  final ValueChanged<List<String>> onChanged;
  final WoxLauncherController launcherController;

  const WoxQueryRefinementBaseView({super.key, required this.launcherController, required this.refinement, required this.selectedValues, required this.onChanged});

  String tr(String key) {
    return launcherController.tr(key);
  }

  double get controlHeight => WoxInterfaceSizeUtil.instance.current.scaledSpacing(26);

  double get dropdownWidth => WoxInterfaceSizeUtil.instance.current.scaledSpacing(128);

  int get chipOptionLimit => 5;

  String optionLabel(WoxQueryRefinementOption option) {
    final label = tr(option.title);
    if (option.count == null) {
      return label;
    }

    return "$label (${option.count})";
  }

  Widget? optionLeading(WoxQueryRefinementOption option) {
    if (option.icon.imageData.isEmpty) {
      return null;
    }

    return WoxImageView(woxImage: option.icon, width: 16, height: 16);
  }

  List<WoxDropdownItem<String>> dropdownItems() {
    return refinement.options.map((option) => WoxDropdownItem<String>(value: option.value, label: optionLabel(option), leading: optionLeading(option))).toList();
  }

  String hotkeyLabel() {
    // Feature fix: inline refinement hints should use the same platform-aware
    // modifier labels as result and toolbar hotkey chips, not a separate
    // Cmd/Alt/Ctrl formatter that reads wrong on Windows/Linux.
    return WoxHotkeyDisplayUtil.labelFromHotkeyString(refinement.hotkey);
  }

  String hotkeyDisplayLabel() {
    if (refinement.hotkey.trim().isEmpty) {
      return "";
    }

    final label = hotkeyLabel();
    // Visual refinement: only chorded shortcuts are shown inline. Single-key
    // hints read like values instead of launcher shortcuts, so they stay hidden
    // until a plugin provides a fuller key chord.
    if (!label.contains("+")) {
      return "";
    }

    return label;
  }

  Widget? buildInlineHotkeyHint() {
    final label = hotkeyDisplayLabel();
    if (label.isEmpty) {
      return null;
    }

    final metrics = WoxInterfaceSizeUtil.instance.current;
    return Text(label, style: TextStyle(color: getThemeSubTextColor().withValues(alpha: 0.58), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700));
  }

  Widget buildChip({required String label, required bool selected, required VoidCallback onTap, Widget? leading}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final activeColor = getThemeActiveBackgroundColor();
    final textColor = getThemeTextColor();
    final backgroundColor = selected ? activeColor.withValues(alpha: 0.22) : Colors.transparent;

    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(5),
        child: InkWell(
          onTap: onTap,
          splashFactory: NoSplash.splashFactory,
          overlayColor: WidgetStateProperty.all(textColor.withValues(alpha: 0.06)),
          child: AnimatedContainer(
            // Visual refinement: each option is now a segment inside one shared
            // filter group. Removing per-chip outlines avoids the row of
            // separate buttons that felt visually heavier than launcher filters.
            duration: const Duration(milliseconds: 90),
            curve: Curves.easeOut,
            height: metrics.scaledSpacing(22),
            constraints: BoxConstraints(maxWidth: metrics.scaledSpacing(118)),
            padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(10)),
            decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(5)),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                if (leading != null) ...[leading, SizedBox(width: metrics.scaledSpacing(5))],
                Flexible(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: selected ? textColor : textColor.withValues(alpha: 0.82),
                      fontSize: metrics.smallLabelFontSize,
                      fontWeight: selected ? FontWeight.w700 : FontWeight.w600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget buildChipRow(List<Widget> chips) {
    final gap = WoxInterfaceSizeUtil.instance.current.scaledSpacing(1);
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        for (var index = 0; index < chips.length; index++) ...[if (index > 0) SizedBox(width: gap), chips[index]],
      ],
    );
  }

  Widget buildShell({required Widget child}) {
    // Visual refinement: one refinement renders as one capsule group. This
    // makes the label, value segments, and hotkey read as a single filter
    // control instead of a loose form row between the query box and results.
    final inlineHotkeyHint = buildInlineHotkeyHint();
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();
    // Visual refinement: the visible trailing hotkey replaces the old hover
    // tooltip. Keyboard-launcher controls need discoverability while scanning,
    // not an extra hover-only layer around every refinement.
    return Container(
      height: controlHeight,
      margin: EdgeInsets.only(right: metrics.scaledSpacing(10)),
      decoration: BoxDecoration(color: textColor.withValues(alpha: 0.035), borderRadius: BorderRadius.circular(7), border: Border.all(color: subTextColor.withValues(alpha: 0.12))),
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(3)),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: EdgeInsets.only(left: metrics.scaledSpacing(7), right: metrics.scaledSpacing(7)),
              child: Text(
                tr(refinement.title),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: subTextColor.withValues(alpha: 0.68), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700),
              ),
            ),
            Container(width: 1, height: metrics.scaledSpacing(14), color: subTextColor.withValues(alpha: 0.13)),
            SizedBox(width: metrics.scaledSpacing(3)),
            child,
            if (inlineHotkeyHint != null) ...[
              SizedBox(width: metrics.scaledSpacing(7)),
              Container(width: 1, height: metrics.scaledSpacing(14), color: subTextColor.withValues(alpha: 0.11)),
              SizedBox(width: metrics.scaledSpacing(7)),
              inlineHotkeyHint,
              SizedBox(width: metrics.scaledSpacing(4)),
            ],
          ],
        ),
      ),
    );
  }
}
