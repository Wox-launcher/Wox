import 'package:flutter_test/flutter_test.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/utils/windows/window_manager.dart';

import 'smoke_test_helper.dart';

void registerLauncherStartupSmokeTests() {
  group('T1: Startup Smoke Tests', () {
    testWidgets('T1-01: Startup shows launcher UI within N seconds', (tester) async {
      final result = await launchLauncherAppAndMeasureStartup(tester, timeout: const Duration(seconds: 5));

      expect(result.elapsed, lessThanOrEqualTo(const Duration(seconds: 3)));
      expect(find.byType(WoxLauncherView), findsOneWidget);
      expect(await windowManager.isVisible(), isTrue);
      expect(result.controller.isQueryBoxVisible.value, isTrue);
    });
  });
}
