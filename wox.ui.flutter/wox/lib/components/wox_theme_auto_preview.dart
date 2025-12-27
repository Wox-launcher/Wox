import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxThemeAutoPreview extends StatelessWidget {
  final WoxTheme theme;

  const WoxThemeAutoPreview({super.key, required this.theme});

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  Widget build(BuildContext context) {
    final settingController = Get.find<WoxSettingController>();
    WoxTheme? lightTheme;
    WoxTheme? darkTheme;

    try {
      lightTheme = settingController.installedThemesList.firstWhere((t) => t.themeId == theme.lightThemeId);
      darkTheme = settingController.installedThemesList.firstWhere((t) => t.themeId == theme.darkThemeId);
    } catch (e) {
      // If themes not found, show fallback
    }

    // Fallback to default themes if not found
    lightTheme ??= _createFallbackLightTheme();
    darkTheme ??= _createFallbackDarkTheme();

    final List<String> previewTexts = [
      tr("ui_theme_preview_text_1"),
      tr("ui_theme_preview_text_2"),
      tr("ui_theme_preview_text_3"),
      tr("ui_theme_preview_text_4"),
      tr("ui_theme_preview_text_5"),
    ];

    return Container(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(8),
        color: safeFromCssColor(lightTheme.appBackgroundColor),
      ),
      clipBehavior: Clip.antiAlias,
      child: Stack(
        children: [
          // Light theme (top-left triangle)
          Positioned.fill(
            child: ClipPath(
              clipper: _TopLeftDiagonalClipper(),
              child: Container(
                color: safeFromCssColor(lightTheme.appBackgroundColor),
                child: _buildPreviewContent(lightTheme, previewTexts),
              ),
            ),
          ),
          // Dark theme (bottom-right triangle)
          Positioned.fill(
            child: ClipPath(
              clipper: _BottomRightDiagonalClipper(),
              child: Container(
                color: safeFromCssColor(darkTheme.appBackgroundColor),
                child: _buildPreviewContent(darkTheme, previewTexts),
              ),
            ),
          ),
          // Diagonal line
          Positioned.fill(
            child: CustomPaint(
              painter: _DiagonalLinePainter(),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPreviewContent(WoxTheme theme, List<String> previewTexts) {
    Color queryBoxColor = safeFromCssColor(theme.queryBoxBackgroundColor);
    Color resultItemActiveColor = safeFromCssColor(theme.resultItemActiveBackgroundColor);
    Color resultItemColor = safeFromCssColor(theme.appBackgroundColor);

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(10, 10, 10, 10),
          child: Container(
            height: 40,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(theme.queryBoxBorderRadius.toDouble()),
              color: queryBoxColor,
            ),
            child: Align(
              alignment: Alignment.centerLeft,
              child: Padding(
                padding: const EdgeInsets.only(left: 10),
                child: Text(
                  tr("ui_theme_preview_title"),
                  style: TextStyle(color: safeFromCssColor(theme.queryBoxFontColor)),
                ),
              ),
            ),
          ),
        ),
        Expanded(
          child: ListView(
            children: List.generate(5, (index) {
              bool isActive = index == 1;
              return Container(
                height: 60,
                margin: const EdgeInsets.symmetric(vertical: 0, horizontal: 10),
                decoration: BoxDecoration(
                  color: isActive ? resultItemActiveColor : resultItemColor,
                ),
                child: ListTile(
                  leading: WoxImageView(
                    woxImage: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: QUERY_ICON_SELECTION_FILE),
                    width: 30,
                  ),
                  title: Text(
                    previewTexts[index],
                    style: TextStyle(
                      color: safeFromCssColor(isActive ? theme.resultItemActiveTitleColor : theme.resultItemTitleColor),
                    ),
                  ),
                  subtitle: Text(
                    tr("ui_theme_preview_subtitle").replaceAll("{index}", "${index + 1}"),
                    style: TextStyle(
                      color: safeFromCssColor(isActive ? theme.resultItemActiveSubTitleColor : theme.resultItemSubTitleColor),
                    ),
                  ),
                ),
              );
            }),
          ),
        ),
        Container(
          height: WoxThemeUtil.instance.getToolbarHeight(),
          decoration: BoxDecoration(
            color: safeFromCssColor(theme.toolbarBackgroundColor.isEmpty ? theme.appBackgroundColor : theme.toolbarBackgroundColor),
            border: Border(
              top: BorderSide(
                color: safeFromCssColor(theme.toolbarFontColor).withOpacity(0.1),
                width: 1,
              ),
            ),
          ),
          child: Padding(
            padding: EdgeInsets.symmetric(
              horizontal: theme.toolbarPaddingLeft.toDouble(),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                const Row(
                  children: [],
                ),
                Row(
                  children: [
                    Text(
                      tr("ui_theme_preview_open"),
                      style: TextStyle(color: safeFromCssColor(theme.toolbarFontColor)),
                    ),
                    const SizedBox(width: 8),
                    WoxHotkeyView(
                      hotkey: WoxHotkey.parseHotkeyFromString("Enter")!,
                      backgroundColor: safeFromCssColor(theme.toolbarBackgroundColor),
                      borderColor: safeFromCssColor(theme.toolbarFontColor),
                      textColor: safeFromCssColor(theme.toolbarFontColor),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  WoxTheme _createFallbackLightTheme() {
    return WoxTheme(
      themeId: 'fallback-light',
      themeName: 'Fallback Light',
      themeAuthor: 'System',
      themeUrl: '',
      version: '1.0.0',
      description: '',
      isSystem: true,
      isInstalled: true,
      isUpgradable: false,
      appBackgroundColor: '#F5F5F5',
      queryBoxBackgroundColor: '#E8E8E8',
      queryBoxFontColor: '#000000',
      resultItemActiveBackgroundColor: '#D8D8D8',
      resultItemTitleColor: '#000000',
      resultItemActiveTitleColor: '#000000',
      resultItemSubTitleColor: '#666666',
      resultItemActiveSubTitleColor: '#666666',
      toolbarBackgroundColor: '#F5F5F5',
      toolbarFontColor: '#000000',
      appPaddingLeft: 20,
      appPaddingTop: 20,
      appPaddingRight: 20,
      appPaddingBottom: 20,
      queryBoxBorderRadius: 8,
      resultItemBorderRadius: 4,
      toolbarPaddingLeft: 16,
    );
  }

  WoxTheme _createFallbackDarkTheme() {
    return WoxTheme(
      themeId: 'fallback-dark',
      themeName: 'Fallback Dark',
      themeAuthor: 'System',
      themeUrl: '',
      version: '1.0.0',
      description: '',
      isSystem: true,
      isInstalled: true,
      isUpgradable: false,
      appBackgroundColor: '#2B2B2B',
      queryBoxBackgroundColor: '#3D3D3D',
      queryBoxFontColor: '#FFFFFF',
      resultItemActiveBackgroundColor: '#4A4A4A',
      resultItemTitleColor: '#FFFFFF',
      resultItemActiveTitleColor: '#FFFFFF',
      resultItemSubTitleColor: '#AAAAAA',
      resultItemActiveSubTitleColor: '#AAAAAA',
      toolbarBackgroundColor: '#2B2B2B',
      toolbarFontColor: '#FFFFFF',
      appPaddingLeft: 20,
      appPaddingTop: 20,
      appPaddingRight: 20,
      appPaddingBottom: 20,
      queryBoxBorderRadius: 8,
      resultItemBorderRadius: 4,
      toolbarPaddingLeft: 16,
    );
  }
}

class _TopLeftDiagonalClipper extends CustomClipper<Path> {
  @override
  Path getClip(Size size) {
    final path = Path()
      ..moveTo(0, 0)
      ..lineTo(size.width, 0)
      ..lineTo(0, size.height)
      ..close();
    return path;
  }

  @override
  bool shouldReclip(covariant CustomClipper<Path> oldClipper) => false;
}

class _BottomRightDiagonalClipper extends CustomClipper<Path> {
  @override
  Path getClip(Size size) {
    final path = Path()
      ..moveTo(size.width, 0)
      ..lineTo(size.width, size.height)
      ..lineTo(0, size.height)
      ..close();
    return path;
  }

  @override
  bool shouldReclip(covariant CustomClipper<Path> oldClipper) => false;
}

class _DiagonalLinePainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = Colors.black.withOpacity(0.15)
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2;
    canvas.drawLine(
      Offset(size.width, 0),
      Offset(0, size.height),
      paint,
    );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
