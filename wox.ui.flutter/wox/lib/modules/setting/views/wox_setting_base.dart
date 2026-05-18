import 'package:get/get.dart';
import 'package:flutter/material.dart';
import 'package:wox/components/wox_setting_form_field.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';

abstract class WoxSettingBaseView extends GetView<WoxSettingController> {
  const WoxSettingBaseView({super.key});

  Widget form({double width = GENERAL_SETTING_FORM_WIDTH, String? title, String? description, required List<Widget> children}) {
    return Align(
      alignment: Alignment.topLeft,
      child: SingleChildScrollView(
        child: Padding(
          // The settings surface now uses a wider, page-like content column so controls can align to the right while descriptions stay readable.
          padding: const EdgeInsets.only(left: 38, right: 44, bottom: 28, top: 34),
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: width),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [if (title != null) pageHeader(title: title, description: description), ...children],
            ),
          ),
        ),
      ),
    );
  }

  Widget pageHeader({required String title, String? description}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: TextStyle(color: getThemeTextColor(), fontSize: 22, fontWeight: FontWeight.w700)),
          if (description != null && description.trim().isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(description, style: TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.35)),
          ],
        ],
      ),
    );
  }

  Widget formSection({required String title, required List<Widget> children}) {
    return Padding(
      padding: const EdgeInsets.only(top: 2, bottom: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Section dividers are part of the same settings layout grid as pane
          // splitters and tab rules. The old local alpha made group separators
          // look unrelated to the rest of the settings chrome.
          Container(height: 1, color: getThemeSettingDividerColor()),
          const SizedBox(height: 14),
          Text(title.toUpperCase(), style: TextStyle(color: getThemeSubTextColor(), fontSize: 11, fontWeight: FontWeight.w600)),
          const SizedBox(height: 12),
          ...children,
        ],
      ),
    );
  }

  Widget formField({
    Key? key,
    required String label,
    required Widget child,
    String? tips,
    Widget? tipsWidget,
    double labelWidth = GENERAL_SETTING_WIDE_LABEL_WIDTH,
    bool fullWidth = false,
    double? controlMaxWidth,
    double bottomSpacing = 18,
  }) {
    final resolvedTips = tipsWidget ?? (tips == null ? null : Text(tips, style: TextStyle(color: getThemeSubTextColor(), fontSize: SETTING_TOOLTIP_DEFAULT_SIZE, height: 1.35)));
    return WoxSettingFormField(
      key: key,
      label: label,
      tips: resolvedTips,
      labelWidth: labelWidth,
      labelGap: 32,
      bottomSpacing: bottomSpacing,
      tipsTopSpacing: 4,
      fullWidth: fullWidth,
      controlMaxWidth: fullWidth ? null : controlMaxWidth,
      child: child,
    );
  }
}
