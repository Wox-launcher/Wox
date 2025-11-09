import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

/// Button types for WoxButton
enum WoxButtonType {
  /// Primary button with filled background (ElevatedButton style)
  primary,

  /// Secondary button with transparent background (TextButton style)
  secondary,

  /// Text-only button for links
  text,
}

/// Custom Button widget with consistent styling and padding
class WoxButton extends StatelessWidget {
  final String text;
  final VoidCallback? onPressed;
  final WoxButtonType type;
  final Widget? icon;
  final double? width;
  final double? height;
  final double fontSize;
  final EdgeInsetsGeometry? padding;

  const WoxButton({
    super.key,
    required this.text,
    required this.onPressed,
    this.type = WoxButtonType.primary,
    this.icon,
    this.width,
    this.height,
    this.fontSize = 13,
    this.padding,
  });

  /// Create a primary button (filled background)
  const WoxButton.primary({
    super.key,
    required this.text,
    required this.onPressed,
    this.icon,
    this.width,
    this.height,
    this.fontSize = 13,
    this.padding,
  }) : type = WoxButtonType.primary;

  /// Create a secondary button (transparent background)
  const WoxButton.secondary({
    super.key,
    required this.text,
    required this.onPressed,
    this.icon,
    this.width,
    this.height,
    this.fontSize = 13,
    this.padding,
  }) : type = WoxButtonType.secondary;

  /// Create a text button (for links)
  const WoxButton.text({
    super.key,
    required this.text,
    required this.onPressed,
    this.icon,
    this.width,
    this.height,
    this.fontSize = 13,
    this.padding,
  }) : type = WoxButtonType.text;

  @override
  Widget build(BuildContext context) {
    var buttonPadding = padding ?? const EdgeInsets.symmetric(horizontal: 20, vertical: 16);
    if (type == WoxButtonType.text) {
      buttonPadding = const EdgeInsets.symmetric(horizontal: 6, vertical: 4);
    }

    Widget buttonChild = icon != null
        ? Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              icon!,
              const SizedBox(width: 8),
              Text(text),
            ],
          )
        : Text(text);

    Widget button;

    switch (type) {
      case WoxButtonType.primary:
        button = ElevatedButton(
          onPressed: onPressed,
          style: ButtonStyle(
            backgroundColor: WidgetStateProperty.resolveWith<Color>(
              (Set<WidgetState> states) {
                if (states.contains(WidgetState.disabled)) {
                  return getThemeTextColor().withValues(alpha: 0.3);
                }
                return getThemeActiveBackgroundColor();
              },
            ),
            foregroundColor: WidgetStateProperty.resolveWith<Color>(
              (Set<WidgetState> states) {
                if (states.contains(WidgetState.disabled)) {
                  return getThemeTextColor().withValues(alpha: 0.5);
                }
                return getThemeActionItemActiveColor();
              },
            ),
            padding: WidgetStateProperty.all(buttonPadding),
            textStyle: WidgetStateProperty.all(
              TextStyle(fontSize: fontSize, fontWeight: FontWeight.normal),
            ),
            elevation: WidgetStateProperty.all(0),
            shape: WidgetStateProperty.all(
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(4)),
            ),
            minimumSize: WidgetStateProperty.all(Size.zero),
            tapTargetSize: MaterialTapTargetSize.shrinkWrap,
          ),
          child: buttonChild,
        );
        break;

      case WoxButtonType.secondary:
        button = OutlinedButton(
          onPressed: onPressed,
          style: ButtonStyle(
            foregroundColor: WidgetStateProperty.resolveWith<Color>(
              (Set<WidgetState> states) {
                if (states.contains(WidgetState.disabled)) {
                  return getThemeTextColor().withValues(alpha: 0.5);
                }
                return getThemeTextColor();
              },
            ),
            side: WidgetStateProperty.resolveWith<BorderSide>(
              (Set<WidgetState> states) {
                if (states.contains(WidgetState.disabled)) {
                  return BorderSide(color: getThemeTextColor().withValues(alpha: 0.3));
                }
                return BorderSide(color: getThemeTextColor().withValues(alpha: 0.5));
              },
            ),
            padding: WidgetStateProperty.all(buttonPadding),
            textStyle: WidgetStateProperty.all(
              TextStyle(fontSize: fontSize, fontWeight: FontWeight.normal),
            ),
            shape: WidgetStateProperty.all(
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(4)),
            ),
            minimumSize: WidgetStateProperty.all(Size.zero),
            tapTargetSize: MaterialTapTargetSize.shrinkWrap,
          ),
          child: buttonChild,
        );
        break;

      case WoxButtonType.text:
        button = TextButton(
          onPressed: onPressed,
          style: ButtonStyle(
            foregroundColor: WidgetStateProperty.resolveWith<Color>(
              (Set<WidgetState> states) {
                if (states.contains(WidgetState.disabled)) {
                  return getThemeTextColor().withValues(alpha: 0.5);
                }
                return getThemeTextColor();
              },
            ),
            padding: WidgetStateProperty.all(buttonPadding),
            textStyle: WidgetStateProperty.all(
              TextStyle(fontSize: fontSize, fontWeight: FontWeight.normal),
            ),
            minimumSize: WidgetStateProperty.all(Size.zero),
            tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            overlayColor: WidgetStateProperty.all(
              getThemeTextColor().withValues(alpha: 0.1),
            ),
          ),
          child: buttonChild,
        );
        break;
    }

    if (width != null || height != null) {
      return SizedBox(
        width: width,
        height: height,
        child: button,
      );
    }

    return button;
  }
}
