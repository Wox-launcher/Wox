import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:wox/entity/screenshot_session.dart';
import 'package:wox/utils/screenshot/screenshot_platform_bridge.dart';
import 'package:wox/utils/windows/window_manager.dart';

import 'smoke_test_helper.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  registerLauncherScreenshotWindowRestoreSmokeTests();
}

void registerLauncherScreenshotWindowRestoreSmokeTests() {
  group('T11A: Screenshot Window Restore Smoke Tests', () {
    testWidgets('T11A-01: Native screenshot presentation restores the launcher window chrome after dismiss', (tester) async {
      if (!Platform.isMacOS) {
        return;
      }

      await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final bridge = MethodChannelScreenshotPlatformBridge();
      final position = await windowManager.getPosition();
      final size = await windowManager.getSize();
      final workspaceBounds = ScreenshotRect(x: position.dx, y: position.dy, width: size.width, height: size.height);

      final before = await bridge.debugCaptureWorkspaceState();
      expect(before['closeButtonHidden'], isTrue);
      expect(before['miniaturizeButtonHidden'], isTrue);
      expect(before['zoomButtonHidden'], isTrue);

      await bridge.presentCaptureWorkspace(workspaceBounds);
      await tester.pump(const Duration(milliseconds: 250));
      final active = await bridge.debugCaptureWorkspaceState();

      expect(active['isCapturePresentationActive'], isTrue);

      await bridge.dismissCaptureWorkspacePresentation();
      await tester.pump(const Duration(milliseconds: 250));
      final restored = await bridge.debugCaptureWorkspaceState();

      expect(restored['isCapturePresentationActive'], isFalse);
      expect(restored['styleMask'], equals(before['styleMask']));
      expect(restored['titleVisibility'], equals(before['titleVisibility']));
      expect(restored['titlebarAppearsTransparent'], equals(before['titlebarAppearsTransparent']));
      expect(restored['closeButtonHidden'], equals(before['closeButtonHidden']));
      expect(restored['miniaturizeButtonHidden'], equals(before['miniaturizeButtonHidden']));
      expect(restored['zoomButtonHidden'], equals(before['zoomButtonHidden']));
    });
  });
}
