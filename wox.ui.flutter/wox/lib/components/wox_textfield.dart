import 'package:flutter/material.dart';
import 'package:wox/utils/colors.dart';

class WoxTextField extends StatelessWidget {
  final TextEditingController? controller;
  final String? hintText;
  final bool enabled;
  final ValueChanged<String>? onChanged;
  final VoidCallback? onEditingComplete;
  final ValueChanged<String>? onSubmitted;
  final int maxLines;
  final int? minLines;
  final bool autofocus;
  final TextStyle? style;
  final TextStyle? hintStyle;
  final double? width;
  final Widget? suffixIcon;
  final EdgeInsetsGeometry? contentPadding;
  final FocusNode? focusNode;

  const WoxTextField({
    super.key,
    this.controller,
    this.hintText,
    this.enabled = true,
    this.onChanged,
    this.onEditingComplete,
    this.onSubmitted,
    this.maxLines = 1,
    this.minLines,
    this.autofocus = false,
    this.style,
    this.hintStyle,
    this.width,
    this.suffixIcon,
    this.contentPadding,
    this.focusNode,
  });

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final borderColor = getThemeSubTextColor();

    final textField = TextField(
      controller: controller,
      enabled: enabled,
      onChanged: onChanged,
      onEditingComplete: onEditingComplete,
      onSubmitted: onSubmitted,
      maxLines: maxLines,
      minLines: minLines,
      autofocus: autofocus,
      focusNode: focusNode,
      textAlignVertical: TextAlignVertical.center,
      style: style ?? TextStyle(color: textColor, fontSize: 13),
      decoration: InputDecoration(
        hintText: hintText,
        hintStyle: hintStyle ?? TextStyle(color: textColor.withValues(alpha: 0.5), fontSize: 13),
        contentPadding: contentPadding ?? const EdgeInsets.symmetric(horizontal: 8, vertical: 10),
        suffixIcon: suffixIcon,
        border: InputBorder.none,
        enabledBorder: InputBorder.none,
        focusedBorder: InputBorder.none,
        disabledBorder: InputBorder.none,
        isDense: true,
      ),
    );

    // Wrap with Container to add border, similar to hotkey recorder and dropdown
    return SizedBox(
      width: width ?? 300.0,
      child: Container(
        decoration: BoxDecoration(
          border: Border.all(color: borderColor),
          borderRadius: BorderRadius.circular(4),
        ),
        child: textField,
      ),
    );
  }
}
