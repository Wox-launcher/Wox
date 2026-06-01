import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';

/// Modern plugin detail view component
/// Used in both query preview panel and settings page
class WoxPluginDetailView extends StatelessWidget {
  final String pluginDetailJson;

  const WoxPluginDetailView({super.key, required this.pluginDetailJson});

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  WoxImage _runtimeIcon(String runtime) {
    switch (runtime.toUpperCase()) {
      case 'PYTHON':
        return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: PYTHON_ICON);
      case 'NODEJS':
        return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: NODEJS_ICON);
      case 'SCRIPT':
      case 'GO':
      default:
        return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: SCRIPT_ICON);
    }
  }

  String _runtimeLabel(String runtime) {
    switch (runtime.toUpperCase()) {
      case 'NODEJS':
        return 'NodeJS';
      case 'PYTHON':
        return 'Python';
      case 'SCRIPT':
        return 'Script';
      case 'GO':
        return 'Go';
      default:
        return runtime;
    }
  }

  bool _isGithubWebsite(String website) {
    final uri = Uri.tryParse(website);
    return uri != null && (uri.host == 'github.com' || uri.host.endsWith('.github.com'));
  }

  Future<void> _openWebsite(String website) async {
    final uri = Uri.tryParse(website);
    if (uri == null || !uri.hasScheme) {
      return;
    }

    await launchUrl(uri);
  }

  @override
  Widget build(BuildContext context) {
    final Map<String, dynamic> pluginData = jsonDecode(pluginDetailJson);
    final String name = pluginData['Name'] ?? '';
    final String description = pluginData['Description'] ?? '';
    final String author = pluginData['Author'] ?? '';
    final String version = pluginData['Version'] ?? '';
    final String website = pluginData['Website'] ?? '';
    final String runtime = pluginData['Runtime'] ?? '';
    final WoxImage? pluginIcon = pluginData['Icon'] is Map<String, dynamic> ? WoxImage.fromJson(pluginData['Icon']) : null;
    final List<String> screenshotUrls = (pluginData['ScreenshotUrls'] as List<dynamic>?)?.map((e) => e.toString()).toList() ?? [];
    final RxInt currentPage = 0.obs;

    // Visual refresh: plugin-detail preview is now an identity-first display
    // surface. The older bottom metadata card repeated low-priority fields and
    // reduced screenshot space, so metadata is folded into compact header chips.
    final Color baseBackground = getThemeBackgroundColor();
    final bool isDarkTheme = baseBackground.computeLuminance() < 0.5;
    final Color panelColor = getThemePanelBackgroundColor();
    final Color outlineColor = getThemeDividerColor().withValues(alpha: isDarkTheme ? 0.45 : 0.25);

    return Container(
      padding: const EdgeInsets.all(24),
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildHeader(
              name: name,
              description: description,
              author: author,
              version: version,
              website: website,
              runtime: runtime,
              pluginIcon: pluginIcon,
              panelColor: panelColor,
              outlineColor: outlineColor,
            ),

            // Screenshots section
            if (screenshotUrls.isNotEmpty) ...[
              const SizedBox(height: 24),
              if (screenshotUrls.length == 1)
                _buildScreenshotImage(screenshotUrls.first)
              else
                Obx(() {
                  final idx = currentPage.value;
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      GestureDetector(
                        onHorizontalDragEnd: (details) {
                          final v = details.primaryVelocity ?? 0;
                          if (v < 0) {
                            // swipe left -> next
                            currentPage.value = (currentPage.value + 1) % screenshotUrls.length;
                          } else if (v > 0) {
                            // swipe right -> prev
                            currentPage.value = (currentPage.value - 1 + screenshotUrls.length) % screenshotUrls.length;
                          }
                        },
                        child: _buildScreenshotImage(screenshotUrls[idx]),
                      ),
                      const SizedBox(height: 8),
                      Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: List.generate(screenshotUrls.length, (i) {
                          final active = i == idx;
                          return GestureDetector(
                            onTap: () => currentPage.value = i,
                            child: AnimatedContainer(
                              duration: const Duration(milliseconds: 150),
                              margin: const EdgeInsets.symmetric(horizontal: 3),
                              width: active ? 8 : 6,
                              height: active ? 8 : 6,
                              decoration: BoxDecoration(color: active ? getThemeTextColor() : getThemeSubTextColor(), shape: BoxShape.circle),
                            ),
                          );
                        }),
                      ),
                    ],
                  );
                }),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildScreenshotImage(String screenshotUrl) {
    return ClipRRect(
      borderRadius: BorderRadius.circular(8),
      child: Image.network(
        screenshotUrl,
        width: double.infinity,
        fit: BoxFit.fitWidth,
        gaplessPlayback: true,
        // Loading fix: screenshot URLs do not have intrinsic dimensions until
        // Flutter receives image headers, so the old direct Image.network left a
        // blank gap and then popped the screenshot in. The loading state stays
        // icon-only because a large placeholder panel looks like empty content
        // instead of a lightweight progress hint.
        loadingBuilder: (context, child, loadingProgress) {
          if (loadingProgress == null) {
            return child;
          }

          return Center(child: WoxLoadingIndicator(size: 24, color: getThemeActiveBackgroundColor()));
        },
        errorBuilder: (context, error, stackTrace) {
          return Center(child: Icon(Icons.broken_image, size: 48, color: getThemeSubTextColor()));
        },
      ),
    );
  }

  Widget _buildHeader({
    required String name,
    required String description,
    required String author,
    required String version,
    required String website,
    required String runtime,
    required WoxImage? pluginIcon,
    required Color panelColor,
    required Color outlineColor,
  }) {
    final Color textColor = getThemeTextColor();
    final Color iconBackgroundColor = getThemeTextColor().withValues(alpha: isThemeDark() ? 0.10 : 0.05);
    final bool isGithubWebsite = _isGithubWebsite(website);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (pluginIcon != null) ...[
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              // The icon block anchors the display-only preview. Keeping it in
              // the detail payload avoids borrowing the selected-list icon from
              // another layer and keeps query preview/settings rendering aligned.
              color: iconBackgroundColor,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: outlineColor),
            ),
            child: Center(child: WoxImageView(woxImage: pluginIcon, width: 32, height: 32)),
          ),
          const SizedBox(width: 18),
        ],
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Expanded(child: Text(name, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 16, fontWeight: FontWeight.w600))),
                ],
              ),
              const SizedBox(height: 8),
              _buildDescriptionLine(description: description, author: author, textColor: textColor),
              if (version.isNotEmpty || runtime.isNotEmpty || website.isNotEmpty) ...[
                const SizedBox(height: 10),
                _buildMetadataRow(version: version, runtime: runtime, website: website, isGithubWebsite: isGithubWebsite, panelColor: panelColor, outlineColor: outlineColor),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildDescriptionLine({required String description, required String author, required Color textColor}) {
    final TextStyle descriptionStyle = TextStyle(color: textColor, fontSize: 13, height: 1.4);
    final TextStyle authorStyle = TextStyle(color: getThemeSubTextColor(), fontSize: 13, height: 1.4);

    return Text.rich(
      TextSpan(
        children: [
          TextSpan(text: description, style: descriptionStyle),
          if (author.isNotEmpty) ...[
            // Density fix: author is supporting text for the description, not a
            // chip. Keeping it inline avoids a wasted row while preserving a full
            // metadata row for version/runtime/website chips.
            TextSpan(text: ' · ', style: authorStyle),
            TextSpan(text: author, style: authorStyle),
          ],
        ],
      ),
      maxLines: 2,
      overflow: TextOverflow.ellipsis,
    );
  }

  Widget _buildMetadataRow({
    required String version,
    required String runtime,
    required String website,
    required bool isGithubWebsite,
    required Color panelColor,
    required Color outlineColor,
  }) {
    return Wrap(
      // Layout fix: chips get their own content-width row. Mixing author text
      // into this row made long or medium author names steal chip space and
      // pushed GitHub onto a visually awkward second line.
      spacing: 8,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        if (version.isNotEmpty) _buildChip(label: 'v$version', panelColor: panelColor, outlineColor: outlineColor),
        if (runtime.isNotEmpty) _buildChip(label: _runtimeLabel(runtime), icon: _runtimeIcon(runtime), panelColor: panelColor, outlineColor: outlineColor),
        if (website.isNotEmpty)
          _buildChip(
            label: isGithubWebsite ? 'GitHub ↗' : '${tr('ui_plugin_website')} ↗',
            icon:
                isGithubWebsite
                    // GitHub gets a brand mark because users recognize it as
                    // repository identity. Other websites stay text-only so
                    // the preview does not imply a GitHub-hosted project.
                    ? WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: GITHUB_ICON)
                    : null,
            panelColor: panelColor,
            outlineColor: outlineColor,
            onPressed: () {
              _openWebsite(website);
            },
          ),
      ],
    );
  }

  Widget _buildChip({required String label, required Color panelColor, required Color outlineColor, WoxImage? icon, VoidCallback? onPressed}) {
    final Color chipBackground = panelColor.a < 1 ? Color.alphaBlend(panelColor, getThemeBackgroundColor()) : panelColor;

    final chip = Container(
      height: 28,
      padding: EdgeInsets.only(left: icon == null ? 12 : 8, right: 12),
      decoration: BoxDecoration(
        // Website chips are intentionally lightweight links: the preview stays
        // mostly display-oriented, but the visible external-link affordance must
        // open the same URL instead of behaving like inert metadata.
        color: isThemeDark() ? chipBackground.lighter(6) : chipBackground.darker(4),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: outlineColor),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[WoxImageView(woxImage: icon, width: 15, height: 15), const SizedBox(width: 6)],
          Text(label, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 12)),
        ],
      ),
    );

    if (onPressed == null) {
      return chip;
    }

    return MouseRegion(cursor: SystemMouseCursors.click, child: GestureDetector(onTap: onPressed, child: chip));
  }
}
