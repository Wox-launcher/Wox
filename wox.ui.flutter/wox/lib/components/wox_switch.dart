import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Custom Switch widget that matches Fluent UI style
class WoxSwitch extends StatelessWidget {
  final bool value;
  final ValueChanged<bool>? onChanged;

  const WoxSwitch({
    super.key,
    required this.value,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    final activeColor = getThemeActiveBackgroundColor();

    return SizedBox(
      height: 24, // Fixed height to ensure consistent spacing
      child: FittedBox(
        fit: BoxFit.contain,
        child: Switch(
          value: value,
          onChanged: onChanged,
          thumbColor: WidgetStateProperty.all(Colors.white),
          trackColor: WidgetStateProperty.resolveWith<Color>(
            (Set<WidgetState> states) {
              if (states.contains(WidgetState.selected)) {
                return activeColor;
              }
              return getThemeTextColor().withValues(alpha: 0.3);
            },
          ),
          materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
        ),
      ),
    );
  }
}
