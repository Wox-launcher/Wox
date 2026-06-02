import 'dart:io';

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';

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

    testWidgets('T3-04: Main hotkey recorder saves a safe shortcut', (tester) async {
      final launcherController = await launchAndShowLauncher(tester);
      final settingController = await openSettings(tester, launcherController, 'general');
      final originalMainHotkey = settingController.woxSetting.value.mainHotkey;
      final originalSelectionHotkey = settingController.woxSetting.value.selectionHotkey;
      final initialMainHotkey = _smokeHotkeyForKey('u');
      final initialSelectionHotkey = _smokeHotkeyForKey('o');
      final recordedHotkey = _smokeHotkeyForKey('p');

      try {
        await settingController.updateConfig('MainHotkey', initialMainHotkey);
        await settingController.updateConfig('SelectionHotkey', initialSelectionHotkey);
        await pumpUntil(tester, () => settingController.woxSetting.value.mainHotkey == initialMainHotkey, timeout: const Duration(seconds: 10));
        await pumpUntil(tester, () => settingController.woxSetting.value.selectionHotkey == initialSelectionHotkey, timeout: const Duration(seconds: 10));

        final mainHotkeyFieldFinder = find.byKey(settingController.getBuiltInSettingKey('MainHotkey'));
        expect(mainHotkeyFieldFinder, findsOneWidget);
        await Scrollable.ensureVisible(tester.element(mainHotkeyFieldFinder), duration: Duration.zero, alignment: 0.35);
        await tester.pump(const Duration(milliseconds: 100));

        final mainRecorderFinder = find.descendant(of: mainHotkeyFieldFinder, matching: find.byType(WoxHotkeyRecorder));
        expect(mainRecorderFinder, findsOneWidget);
        await tester.tap(mainRecorderFinder, warnIfMissed: false);
        await tester.pump(const Duration(milliseconds: 300));

        await _recordSafeHotkey(tester);
        await pumpUntil(tester, () => settingController.woxSetting.value.mainHotkey == recordedHotkey, timeout: const Duration(seconds: 10));

        expect(settingController.woxSetting.value.mainHotkey, recordedHotkey);
      } finally {
        if (launcherController.isSettingWindowOpen.value) {
          await closeSettings(tester, settingController, launcherController);
        }
        await updateSettingDirect('MainHotkey', originalMainHotkey);
        await updateSettingDirect('SelectionHotkey', originalSelectionHotkey);
      }
    });
  });
}

String _smokeHotkeyForKey(String key) {
  return Platform.isMacOS ? 'ctrl+shift+option+$key' : 'ctrl+shift+alt+$key';
}

Future<void> _recordSafeHotkey(WidgetTester tester) async {
  await tester.sendKeyDownEvent(LogicalKeyboardKey.controlLeft);
  await tester.sendKeyDownEvent(LogicalKeyboardKey.shiftLeft);
  await tester.sendKeyDownEvent(LogicalKeyboardKey.altLeft);
  await tester.sendKeyDownEvent(LogicalKeyboardKey.keyP);
  await tester.sendKeyUpEvent(LogicalKeyboardKey.keyP);
  await tester.sendKeyUpEvent(LogicalKeyboardKey.altLeft);
  await tester.sendKeyUpEvent(LogicalKeyboardKey.shiftLeft);
  await tester.sendKeyUpEvent(LogicalKeyboardKey.controlLeft);
  await tester.pump(const Duration(milliseconds: 300));
}
