import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Wox dropdown button with theme-aware styling
class WoxDropdownButton<T> extends StatelessWidget {
  final List<DropdownMenuItem<T>> items;
  final T? value;
  final ValueChanged<T?>? onChanged;
  final bool isExpanded;
  final double fontSize;
  final Color? dropdownColor;
  final double? menuMaxHeight;
  final Widget? hint;
  final Widget? icon;
  final double? iconSize;
  final AlignmentGeometry alignment;
  final double? itemHeight;
  final double? width;
  final Widget? underline;

  const WoxDropdownButton({
    super.key,
    required this.items,
    required this.value,
    required this.onChanged,
    this.isExpanded = true,
    this.fontSize = 13,
    this.dropdownColor,
    this.menuMaxHeight,
    this.hint,
    this.icon,
    this.iconSize,
    this.alignment = AlignmentDirectional.centerStart,
    this.itemHeight,
    this.width,
    this.underline,
  });

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final activeTextColor = getThemeActiveTextColor();
    final dropdownBg = dropdownColor ?? getThemeActiveBackgroundColor().withValues(alpha: 0.95);

    final dropdown = DropdownButton<T>(
      items: items,
      value: value,
      onChanged: onChanged,
      isExpanded: isExpanded,
      // This style applies to dropdown menu items (when expanded)
      style: TextStyle(color: activeTextColor, fontSize: fontSize),
      // This builder customizes how the selected value appears when dropdown is closed
      selectedItemBuilder: (BuildContext context) {
        return items.map<Widget>((DropdownMenuItem<T> item) {
          return Align(
            alignment: alignment,
            child: DefaultTextStyle(
              style: TextStyle(color: textColor, fontSize: fontSize),
              child: item.child,
            ),
          );
        }).toList();
      },
      dropdownColor: dropdownBg,
      iconEnabledColor: textColor,
      iconDisabledColor: textColor.withValues(alpha: 0.5),
      hint: hint,
      icon: icon,
      iconSize: iconSize ?? 24.0,
      menuMaxHeight: menuMaxHeight,
      alignment: alignment,
      itemHeight: itemHeight,
      underline: underline,
    );

    // Always wrap with SizedBox to ensure consistent width
    // Default width is 300 if not specified
    return SizedBox(
      width: width ?? 300.0,
      child: dropdown,
    );
  }
}
