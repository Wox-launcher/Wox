import 'package:flutter_test/flutter_test.dart';
import 'smoke_test_helper.dart';

void registerLauncherKeyFunctionalitySmokeTests() {
  group('T3: Key Functionality Smoke Tests', () {
    testWidgets('T3-01: Theme settings accessible', (tester) async {
      final launcherController = await launchAndShowLauncher(tester);
      final settingController = await openSettings(tester, launcherController, 'general');

      await tapSettingNavItem(tester, settingController, 'ui');

      expectSettingsWindowOpen(launcherController);

      await closeSettings(tester, settingController, launcherController);
    });

    testWidgets('T3-02: Data backup entry accessible', (tester) async {
      final launcherController = await launchAndShowLauncher(tester);
      final settingController = await openSettings(tester, launcherController, 'general');

      await tapSettingNavItem(tester, settingController, 'data');

      expectSettingsWindowOpen(launcherController);

      await closeSettings(tester, settingController, launcherController);
    });

    testWidgets('T3-03: Usage and About pages load', (tester) async {
      final launcherController = await launchAndShowLauncher(tester);
      final settingController = await openSettings(tester, launcherController, 'general');

      await tapSettingNavItem(tester, settingController, 'usage');

      expectSettingsWindowOpen(launcherController);

      await tapSettingNavItem(tester, settingController, 'about');
      expectSettingsWindowOpen(launcherController);

      await closeSettings(tester, settingController, launcherController);
    });
  });
}
