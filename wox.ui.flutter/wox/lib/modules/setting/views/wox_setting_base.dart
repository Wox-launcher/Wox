import 'package:get/get.dart';
import 'package:flutter/material.dart';
import 'package:wox/components/wox_setting_form_field.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';

abstract class WoxSettingBaseView extends GetView<WoxSettingController> {
  const WoxSettingBaseView({super.key});

  Widget form({double width = 960, required List<Widget> children}) {
    return Align(
      alignment: Alignment.topLeft,
      child: SingleChildScrollView(
        child: Padding(
          padding: const EdgeInsets.only(left: 20, right: 40, bottom: 20, top: 20),
          child: Column(crossAxisAlignment: CrossAxisAlignment.start, mainAxisSize: MainAxisSize.min, children: [...children.map((e) => SizedBox(width: width, child: e))]),
        ),
      ),
    );
  }

  Widget formField({required String label, required Widget child, String? tips, Widget? tipsWidget, double labelWidth = GENERAL_SETTING_LABEL_WIDTH}) {
    final resolvedTips = tipsWidget ?? (tips == null ? null : Text(tips, style: TextStyle(color: getThemeSubTextColor(), fontSize: SETTING_TOOLTIP_DEFAULT_SIZE)));
    return WoxSettingFormField(label: label, tips: resolvedTips, labelWidth: labelWidth, labelGap: 20, bottomSpacing: 20, tipsTopSpacing: 2, child: child);
  }
}
