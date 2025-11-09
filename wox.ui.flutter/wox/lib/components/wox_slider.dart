import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Custom Slider widget with consistent padding and theme-aware styling
class WoxSlider extends StatelessWidget {
  final double value;
  final double min;
  final double max;
  final int? divisions;
  final ValueChanged<double>? onChanged;
  final bool showValue;
  final String? valuePrefix;
  final String? valueSuffix;

  const WoxSlider({
    super.key,
    required this.value,
    required this.min,
    required this.max,
    this.divisions,
    required this.onChanged,
    this.showValue = true,
    this.valuePrefix,
    this.valueSuffix,
  });

  @override
  Widget build(BuildContext context) {
    final slider = SliderTheme(
      data: SliderThemeData(
        activeTrackColor: getThemeActiveBackgroundColor(),
        inactiveTrackColor: getThemeTextColor().withValues(alpha: 0.3),
        thumbColor: getThemeActiveBackgroundColor(),
        overlayColor: getThemeActiveBackgroundColor().withValues(alpha: 0.2),
        valueIndicatorColor: getThemeActiveBackgroundColor(),
        valueIndicatorTextStyle: TextStyle(color: getThemeTextColor()),
        trackHeight: 2.0,
        thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 8.0),
        overlayShape: const RoundSliderOverlayShape(overlayRadius: 16.0),
        // Remove default padding
        trackShape: const CustomTrackShape(),
      ),
      child: Slider(
        value: value,
        min: min,
        max: max,
        divisions: divisions,
        onChanged: onChanged,
      ),
    );

    if (!showValue) {
      return slider;
    }

    return Row(
      children: [
        Expanded(child: slider),
        const SizedBox(width: 16),
        Text(
          '${valuePrefix ?? ''}${value.toInt()}${valueSuffix ?? ''}',
          style: TextStyle(color: getThemeTextColor(), fontSize: 13),
        ),
      ],
    );
  }
}

/// Custom track shape that removes default padding
class CustomTrackShape extends RoundedRectSliderTrackShape {
  const CustomTrackShape();

  @override
  Rect getPreferredRect({
    required RenderBox parentBox,
    Offset offset = Offset.zero,
    required SliderThemeData sliderTheme,
    bool isEnabled = false,
    bool isDiscrete = false,
  }) {
    final double trackHeight = sliderTheme.trackHeight ?? 2.0;
    final double trackLeft = offset.dx;
    final double trackTop = offset.dy + (parentBox.size.height - trackHeight) / 2;
    final double trackWidth = parentBox.size.width;
    return Rect.fromLTWH(trackLeft, trackTop, trackWidth, trackHeight);
  }
}
