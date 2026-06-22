import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxSettingAboutView extends WoxSettingBaseView {
  const WoxSettingAboutView({super.key});

  @override
  Widget build(BuildContext context) {
    // About already has a brand-focused hero block, so the standard settings
    // page header would duplicate the page identity and push the content down.
    return form(
      children: [
        Center(
          child: Container(
            constraints: const BoxConstraints(maxWidth: 600),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const SizedBox(height: 40),
                // Logo
                WoxImageView(woxImage: WoxImage.newBase64(WOX_ICON), width: 100, height: 100),
                const SizedBox(height: 30),
                // Version
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                  decoration: BoxDecoration(color: getThemeActiveBackgroundColor(), borderRadius: BorderRadius.circular(16)),
                  child: Text(
                    controller.woxVersion.value,
                    style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor), fontWeight: FontWeight.w500),
                  ),
                ),
                const SizedBox(height: 30),
                // Description
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 24),
                  child: Text(controller.tr('ui_about_description'), textAlign: TextAlign.center, style: TextStyle(color: getThemeTextColor(), fontSize: 16, height: 1.5)),
                ),
                const SizedBox(height: 40),
                // Links
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    _buildOnboardingButton(),
                    const SizedBox(width: 30),
                    _buildLinkButton('ui_about_docs', 'https://wox-launcher.github.io/Wox/#/', Icons.description),
                    const SizedBox(width: 30),
                    _buildLinkButton('ui_about_github', 'https://github.com/Wox-launcher/Wox', Icons.code),
                  ],
                ),
                const SizedBox(height: 40),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildOnboardingButton() {
    return WoxButton.text(
      key: const ValueKey('about-open-onboarding-button'),
      text: controller.tr('ui_about_onboarding'),
      icon: Icon(Icons.auto_stories_outlined, size: 18, color: getThemeTextColor()),
      onPressed: () {
        // Manual reopening uses the same onboarding window path as startup so
        // About stays a link row instead of growing a second guide surface.
        Get.find<WoxLauncherController>().openOnboarding(const UuidV4().generate());
      },
    );
  }

  Widget _buildLinkButton(String labelKey, String url, IconData icon) {
    return WoxButton.text(
      text: controller.tr(labelKey),
      icon: Icon(icon, size: 18, color: getThemeTextColor()),
      onPressed: () async {
        final uri = Uri.parse(url);
        if (await canLaunchUrl(uri)) {
          await launchUrl(uri);
        }
      },
    );
  }
}
