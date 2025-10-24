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

    return Transform.scale(
      scale: 0.8,
      child: Switch(
        value: value,
        onChanged: onChanged,
        activeColor: Colors.white,
        activeTrackColor: activeColor,
        inactiveThumbColor: Colors.white,
        inactiveTrackColor: getThemeTextColor().withOpacity(0.3),
        materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
      ),
    );
  }
}

