import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Wox-styled checkbox with compact hit target and consistent sizing
class WoxCheckbox extends StatelessWidget {
  final bool? value;
  final ValueChanged<bool?>? onChanged;
  final bool enabled;
  final double size; // visual box size
  final EdgeInsetsGeometry? padding; // outer padding if needed
  final Color? activeColor;
  final Color? checkColor;
  final Color? borderColor;

  const WoxCheckbox({
    super.key,
    required this.value,
    required this.onChanged,
    this.enabled = true,
    this.size = 24,
    this.padding,
    this.activeColor,
    this.checkColor,
    this.borderColor,
  });

  @override
  Widget build(BuildContext context) {
    final Color border = borderColor ?? getThemeSubTextColor();
    final Color active = activeColor ?? getThemeActiveBackgroundColor();
    final Color tick = checkColor ?? getThemeActiveTextColor();

    final checkbox = Theme(
      data: Theme.of(context).copyWith(
        materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
        checkboxTheme: CheckboxThemeData(
          materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
          visualDensity: const VisualDensity(horizontal: -4, vertical: -4),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(4)),
          side: BorderSide(color: border),
        ),
      ),
      child: Checkbox(
        value: value,
        onChanged: enabled ? onChanged : null,
        activeColor: active,
        checkColor: tick,
        side: BorderSide(color: border),
      ),
    );

    return Padding(
      padding: padding ?? EdgeInsets.zero,
      child: SizedBox(
        width: size,
        height: size,
        child: Center(child: checkbox),
      ),
    );
  }
}

