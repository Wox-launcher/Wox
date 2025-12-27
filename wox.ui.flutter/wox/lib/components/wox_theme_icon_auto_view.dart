import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/components/wox_theme_icon_view.dart';
import 'package:wox/controllers/wox_setting_controller.dart';

class WoxThemeIconAutoView extends StatelessWidget {
  final WoxTheme theme;
  final double? width;
  final double? height;

  const WoxThemeIconAutoView({
    super.key,
    required this.theme,
    this.width,
    this.height,
  });

  @override
  Widget build(BuildContext context) {
    // Find light and dark themes from installed themes
    final settingController = Get.find<WoxSettingController>();
    WoxTheme? lightTheme;
    WoxTheme? darkTheme;

    try {
      lightTheme = settingController.installedThemesList.firstWhere((t) => t.themeId == theme.lightThemeId);
      darkTheme = settingController.installedThemesList.firstWhere((t) => t.themeId == theme.darkThemeId);
    } catch (e) {
      // If themes not found, use fallback empty themes
    }

    // Fallback to default themes if not found
    lightTheme ??= _createFallbackLightTheme();
    darkTheme ??= _createFallbackDarkTheme();

    return ClipRRect(
      borderRadius: BorderRadius.circular(8),
      child: SizedBox(
        width: width,
        height: height,
        child: Stack(
          children: [
            // Light theme (top-left triangle)
            Positioned.fill(
              child: ClipPath(
                clipper: _TopLeftDiagonalClipper(),
                child: WoxThemeIconView(theme: lightTheme),
              ),
            ),
            // Dark theme (bottom-right triangle)
            Positioned.fill(
              child: ClipPath(
                clipper: _BottomRightDiagonalClipper(),
                child: WoxThemeIconView(theme: darkTheme),
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
      ),
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
      resultItemActiveBackgroundColor: '#D8D8D8',
    )
      ..appPaddingLeft = 4
      ..appPaddingTop = 4
      ..appPaddingRight = 4
      ..appPaddingBottom = 4
      ..queryBoxBorderRadius = 4
      ..resultItemBorderRadius = 2;
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
      resultItemActiveBackgroundColor: '#4A4A4A',
    )
      ..appPaddingLeft = 4
      ..appPaddingTop = 4
      ..appPaddingRight = 4
      ..appPaddingBottom = 4
      ..queryBoxBorderRadius = 4
      ..resultItemBorderRadius = 2;
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
      ..strokeWidth = 1.5;
    canvas.drawLine(
      Offset(size.width, 0),
      Offset(0, size.height),
      paint,
    );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
