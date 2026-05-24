import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_dialog_util.dart';
import 'package:wox/utils/wox_setting_focus_util.dart';

class WoxSettingPrivacyView extends WoxSettingBaseView {
  const WoxSettingPrivacyView({super.key});

  Future<void> _showDataSampleDialog(BuildContext context) async {
    final sampleData = _buildSamplePayload();
    final jsonString = const JsonEncoder.withIndent('  ').convert(sampleData);

    await showWoxDialog(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (dialogContext) {
        return AlertDialog(
          backgroundColor: getThemePopupSurfaceColor(),
          surfaceTintColor: Colors.transparent,
          elevation: 18,
          insetPadding: const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
          contentPadding: const EdgeInsets.fromLTRB(24, 24, 24, 0),
          actionsPadding: const EdgeInsets.fromLTRB(24, 12, 24, 24),
          actionsAlignment: MainAxisAlignment.end,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20), side: BorderSide(color: getThemePopupOutlineColor())),
          content: SizedBox(
            width: 450,
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(controller.tr("ui_privacy_sample_title"), style: TextStyle(color: getThemeTextColor(), fontSize: 16, fontWeight: FontWeight.w700)),
                  const SizedBox(height: 20),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: getThemeTextColor().withValues(alpha: 0.05),
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: getThemePopupOutlineColor()),
                    ),
                    child: WoxSelectableText(jsonString, style: TextStyle(fontSize: 12, color: getThemeTextColor())),
                  ),
                ],
              ),
            ),
          ),
          actions: [
            WoxButton.secondary(
              text: controller.tr("toolbar_copy"),
              padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12),
              onPressed: () {
                Clipboard.setData(ClipboardData(text: jsonString));
              },
            ),
            const SizedBox(width: 12),
            WoxButton.primary(
              text: controller.tr("ui_ok"),
              padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 12),
              onPressed: () {
                Navigator.pop(dialogContext);
              },
            ),
          ],
        );
      },
    );
    WoxSettingFocusUtil.restoreIfInSettingView();
  }

  Map<String, dynamic> _buildSamplePayload() {
    // Show sample values that represent what would be sent
    return {
      "schema_version": 1,
      "install_hash": "sha256(install_id) - a 64-character hexadecimal string",
      "os_family": _getOSFamilySample(),
      "wox_version": controller.woxVersion.value,
      "sent_at": DateTime.now().millisecondsSinceEpoch,
    };
  }

  String _getOSFamilySample() {
    // Determine OS from Dart platform
    return Theme.of(Get.context!).platform == TargetPlatform.windows
        ? "windows"
        : Theme.of(Get.context!).platform == TargetPlatform.macOS
        ? "darwin"
        : "linux";
  }

  @override
  Widget build(BuildContext context) {
    return form(
      title: controller.tr("ui_privacy"),
      description: controller.tr("ui_privacy_description"),
      children: [
        formField(
          settingKey: "EnableAnonymousUsageStats",
          label: controller.tr("ui_privacy_anonymous_stats_title"),
          labelWidth: GENERAL_SETTING_WIDE_LABEL_WIDTH,
          child: Obx(() {
            return Row(
              // The parent form field right-aligns the control area; keeping this row compact
              // prevents the switch and sample action from expanding back to the left edge.
              mainAxisSize: MainAxisSize.min,
              children: [
                WoxButton.text(
                  text: controller.tr("ui_privacy_view_sample"),
                  padding: const EdgeInsets.only(top: 4, right: 8, bottom: 4),
                  onPressed: () => _showDataSampleDialog(context),
                ),
                SizedBox(width: 10),
                WoxSwitch(
                  value: controller.woxSetting.value.enableAnonymousUsageStats,
                  onChanged: (value) {
                    controller.updateConfig("EnableAnonymousUsageStats", value.toString());
                  },
                ),
              ],
            );
          }),
          tips: controller.tr("ui_privacy_anonymous_stats_description"),
        ),
      ],
    );
  }
}
