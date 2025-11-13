import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';

/// Modern plugin detail view component
/// Used in both query preview panel and settings page
class WoxPluginDetailView extends StatelessWidget {
  final String pluginDetailJson;

  const WoxPluginDetailView({
    super.key,
    required this.pluginDetailJson,
  });

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
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
    final List<String> screenshotUrls = (pluginData['ScreenshotUrls'] as List<dynamic>?)?.map((e) => e.toString()).toList() ?? [];
    final RxInt currentPage = 0.obs;

    // Calculate panel color similar to runtime settings
    final Color baseBackground = getThemeBackgroundColor();
    final bool isDarkTheme = baseBackground.computeLuminance() < 0.5;
    final Color panelColor = getThemePanelBackgroundColor();
    Color cardColor = panelColor.a < 1 ? Color.alphaBlend(panelColor, baseBackground) : panelColor;
    cardColor = isDarkTheme ? cardColor.lighter(6) : cardColor.darker(4);
    final Color outlineColor = getThemeDividerColor().withValues(alpha: isDarkTheme ? 0.45 : 0.25);

    return Container(
      padding: const EdgeInsets.all(24),
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Plugin name and version
            Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Expanded(
                  child: Text(
                    name,
                    style: TextStyle(
                      color: getThemeTextColor(),
                      fontSize: 24,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
                Text(
                  'v$version',
                  style: TextStyle(
                    color: getThemeSubTextColor(),
                    fontSize: 14,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 20),

            // Description
            Text(
              description,
              style: TextStyle(
                color: getThemeTextColor(),
                fontSize: 14,
                height: 1.6,
              ),
            ),

            // Screenshots section
            if (screenshotUrls.isNotEmpty) ...[
              const SizedBox(height: 24),
              if (screenshotUrls.length == 1)
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: Image.network(
                    screenshotUrls.first,
                    width: double.infinity,
                    fit: BoxFit.fitWidth,
                    errorBuilder: (context, error, stackTrace) {
                      return Center(
                        child: Icon(
                          Icons.broken_image,
                          size: 48,
                          color: getThemeSubTextColor(),
                        ),
                      );
                    },
                  ),
                )
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
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(8),
                          child: Image.network(
                            screenshotUrls[idx],
                            width: double.infinity,
                            fit: BoxFit.fitWidth,
                            errorBuilder: (context, error, stackTrace) {
                              return Center(
                                child: Icon(
                                  Icons.broken_image,
                                  size: 48,
                                  color: getThemeSubTextColor(),
                                ),
                              );
                            },
                          ),
                        ),
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
                              decoration: BoxDecoration(
                                color: active ? getThemeTextColor() : getThemeSubTextColor(),
                                shape: BoxShape.circle,
                              ),
                            ),
                          );
                        }),
                      ),
                    ],
                  );
                }),
            ],

            // Metadata panel at the bottom
            const SizedBox(height: 24),
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: cardColor,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: outlineColor),
              ),
              child: Column(
                children: [
                  _buildInfoRow(tr('ui_plugin_author'), author),
                  if (runtime.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    _buildInfoRow(tr('ui_plugin_runtime'), runtime.toUpperCase()),
                  ],
                  if (website.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    _buildWebsiteRow(website),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoRow(String label, String value) {
    return Row(
      children: [
        Text(
          label,
          style: TextStyle(
            color: getThemeSubTextColor(),
            fontSize: 13,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            value,
            style: TextStyle(
              color: getThemeTextColor(),
              fontSize: 13,
            ),
            textAlign: TextAlign.right,
          ),
        ),
      ],
    );
  }

  Widget _buildWebsiteRow(String website) {
    return Row(
      children: [
        Text(
          tr('ui_plugin_website'),
          style: TextStyle(
            color: getThemeSubTextColor(),
            fontSize: 13,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: InkWell(
            onTap: () async {
              final uri = Uri.parse(website);
              if (await canLaunchUrl(uri)) {
                await launchUrl(uri);
              }
            },
            child: Text(
              website,
              style: TextStyle(
                color: getThemeTextColor(),
                fontSize: 13,
                decoration: TextDecoration.underline,
              ),
              textAlign: TextAlign.right,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ),
      ],
    );
  }
}
