import 'package:flutter/material.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/utils/colors.dart';

/// Compact checkbox + label tile with Wox styles
class WoxCheckboxTile extends StatelessWidget {
  final bool value;
  final ValueChanged<bool> onChanged;
  final String title;
  final bool enabled;
  final EdgeInsetsGeometry? padding; // outer padding around the row

  const WoxCheckboxTile({
    super.key,
    required this.value,
    required this.onChanged,
    required this.title,
    this.enabled = true,
    this.padding,
  });

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();

    return InkWell(
      onTap: enabled ? () => onChanged(!value) : null,
      borderRadius: BorderRadius.circular(4),
      child: Padding(
        padding: padding ?? const EdgeInsets.symmetric(vertical: 2.0, horizontal: 4.0),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            WoxCheckbox(
              value: value,
              onChanged: (v) => onChanged(v ?? false),
              enabled: enabled,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: textColor, fontSize: 13),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

