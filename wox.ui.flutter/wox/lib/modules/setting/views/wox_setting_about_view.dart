import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wox/components/wox_image_view.dart';
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
                WoxImageView(
                  woxImage: WoxImage.newBase64(WOX_ICON),
                  width: 100,
                  height: 100,
                ),
                const SizedBox(height: 30),
                // Version
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                  decoration: BoxDecoration(
                    color: getThemeActiveBackgroundColor(),
                    borderRadius: BorderRadius.circular(16),
                  ),
                  child: Text(
                    controller.woxVersion.value,
                    style: TextStyle(
                      color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionItemActiveFontColor),
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ),
                const SizedBox(height: 30),
                // Description
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 24),
                  child: Text(
                    controller.tr('ui_about_description'),
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      color: getThemeTextColor(),
                      fontSize: 16,
                      height: 1.5,
                    ),
                  ),
                ),
                const SizedBox(height: 40),
                // Links
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    _buildLinkButton(
                      'ui_about_docs',
                      'https://wox-launcher.github.io/Wox/#/',
                      Icons.description,
                    ),
                    const SizedBox(width: 30),
                    _buildLinkButton(
                      'ui_about_github',
                      'https://github.com/Wox-launcher/Wox',
                      Icons.code,
                    ),
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

  Widget _buildLinkButton(String labelKey, String url, IconData icon) {
    return TextButton(
      onPressed: () async {
        final uri = Uri.parse(url);
        if (await canLaunchUrl(uri)) {
          await launchUrl(uri);
        }
      },
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 18, color: getThemeTextColor()),
          const SizedBox(width: 8),
          Text(
            controller.tr(labelKey),
            style: TextStyle(
              color: getThemeTextColor(),
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }
}
