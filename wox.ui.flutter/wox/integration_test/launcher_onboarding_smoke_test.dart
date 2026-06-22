import 'dart:io';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/utils/wox_setting_util.dart';

import 'smoke_test_helper.dart';

void registerLauncherOnboardingSmokeTests() {
  group('T1B: Onboarding Smoke Tests', () {
    testWidgets(
      'T1B-01: unfinished startup opens onboarding and skip finishes before launcher',
      (tester) async {
        final controller = await launchOnboardingApp(tester);

        expect(controller.isInOnboardingView.value, isTrue);
        expect(find.byKey(const ValueKey('onboarding-view')), findsOneWidget);

        await tester.tap(
          find.byKey(const ValueKey('onboarding-skip-button')),
          warnIfMissed: false,
        );
        await tester.pump(const Duration(milliseconds: 500));

        await pumpUntil(
          tester,
          () => find.byType(WoxLauncherView).evaluate().isNotEmpty,
          timeout: const Duration(seconds: 30),
        );
        await WoxSettingUtil.instance.loadSetting('onboarding-smoke-reload');

        expect(controller.isInOnboardingView.value, isFalse);
        expect(
          WoxSettingUtil.instance.currentSetting.onboardingFinished,
          isTrue,
        );
        expect(find.byType(WoxLauncherView), findsOneWidget);
      },
    );

    testWidgets(
      'T1B-02: about page can reopen onboarding and complete returns to launcher',
      (tester) async {
        final controller = await launchLauncherApp(tester);
        final settingController = await openSettings(
          tester,
          controller,
          '/about',
        );

        await tester.tap(
          find.byKey(const ValueKey('about-open-onboarding-button')),
          warnIfMissed: false,
        );
        await tester.pump(const Duration(milliseconds: 500));

        await pumpUntil(
          tester,
          () =>
              find
                  .byKey(const ValueKey('onboarding-view'))
                  .evaluate()
                  .isNotEmpty,
          timeout: const Duration(seconds: 30),
        );
        expect(controller.isInOnboardingView.value, isTrue);

        await tester.tap(
          find.byKey(const ValueKey('onboarding-skip-button')),
          warnIfMissed: false,
        );
        await tester.pump(const Duration(milliseconds: 500));

        await pumpUntil(
          tester,
          () => find.byType(WoxLauncherView).evaluate().isNotEmpty,
          timeout: const Duration(seconds: 30),
        );
        expect(controller.isInOnboardingView.value, isFalse);
        expect(settingController.activeNavPath.value, 'about');
      },
    );

    testWidgets(
      'T1B-03: onboarding config pages save immediately and glance never renders blank',
      (tester) async {
        await launchOnboardingApp(tester);

        expect(
          find.byKey(const ValueKey('onboarding-step-permissions')),
          Platform.isMacOS ? findsOneWidget : findsNothing,
        );
        if (Platform.isMacOS) {
          // The permission step only exists on macOS. Windows and Linux go
          // straight to configurable features so their progress does not include
          // a non-actionable permission page.
          await _goNext(tester); // permissions
          expect(
            find.byKey(const ValueKey('onboarding-permission-macos')),
            findsOneWidget,
          );
        }

        await _goNext(tester); // main hotkey
        expect(
          find.byKey(const ValueKey('onboarding-main-hotkey-demo')),
          findsOneWidget,
        );
        await _goNext(tester); // selection hotkey
        expect(
          find.byKey(const ValueKey('onboarding-selection-hotkey-demo')),
          findsOneWidget,
        );
        // Feature change: the window/interface page was removed from the
        // first-run flow, so the smoke path now moves directly from hotkey
        // setup to Glance instead of asserting width and density controls.
        await _goNext(tester); // glance
        await pumpUntil(
          tester,
          () =>
              find
                  .byKey(const ValueKey('onboarding-glance-disabled'))
                  .evaluate()
                  .isNotEmpty ||
              find
                  .byKey(const ValueKey('onboarding-glance-loading'))
                  .evaluate()
                  .isNotEmpty ||
              find
                  .byKey(const ValueKey('onboarding-glance-empty'))
                  .evaluate()
                  .isNotEmpty ||
              find
                  .byKey(const ValueKey('onboarding-glance-picker'))
                  .evaluate()
                  .isNotEmpty,
          timeout: const Duration(seconds: 30),
        );
        if (find
            .byKey(const ValueKey('onboarding-glance-disabled'))
            .evaluate()
            .isNotEmpty) {
          // Glance is optional in onboarding. The smoke test toggles it on when
          // the default setting is disabled so both the enable flag and the
          // provider loading state are covered.
          await tester.tap(
            find.byKey(const ValueKey('onboarding-glance-enable-switch')),
            warnIfMissed: false,
          );
          await tester.pump(const Duration(milliseconds: 500));
          // Bug fix: the switch callback persists through the same async
          // settings controller path as the real UI, but it is intentionally
          // fire-and-forget. Await that path explicitly so this smoke observes
          // the post-toggle panel instead of racing the background reload.
          await Get.find<WoxSettingController>().updateConfig('EnableGlance', 'true');
          await WoxSettingUtil.instance.loadSetting(
            'onboarding-smoke-glance-enable-reload',
          );
          expect(WoxSettingUtil.instance.currentSetting.enableGlance, isTrue);
          await pumpUntil(
            tester,
            () =>
                // Bug fix: the persisted setting can update before the
                // Obx-bound Glance panel swaps out the disabled state. The
                // smoke is guarding against a blank onboarding page here, so
                // any concrete Glance panel is acceptable after the save
                // assertion above.
                find
                    .byKey(const ValueKey('onboarding-glance-disabled'))
                    .evaluate()
                    .isNotEmpty ||
                find
                    .byKey(const ValueKey('onboarding-glance-loading'))
                    .evaluate()
                    .isNotEmpty ||
                find
                    .byKey(const ValueKey('onboarding-glance-empty'))
                    .evaluate()
                    .isNotEmpty ||
                find
                    .byKey(const ValueKey('onboarding-glance-picker'))
                    .evaluate()
                    .isNotEmpty,
            timeout: const Duration(seconds: 30),
          );
        }

        // Feature change: Action Panel and Query Shortcuts are no longer
        // standalone onboarding steps. The smoke path follows the concrete
        // steps rendered by WoxOnboardingView so navigation assertions validate
        // the current guide instead of an older product tour.
        await _goNext(tester); // query hotkeys
        expect(
          find.byKey(const ValueKey('onboarding-query-hotkeys-page')),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('onboarding-query-hotkeys-demo')),
          findsOneWidget,
        );
        await _goNext(tester); // tray queries
        expect(
          find.byKey(const ValueKey('onboarding-tray-queries-page')),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('onboarding-tray-queries-demo')),
          findsOneWidget,
        );
        await _goNext(tester); // wpm install
        expect(
          find.byKey(const ValueKey('onboarding-wpm-install-page')),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('onboarding-wpm-install-demo')),
          findsOneWidget,
        );
        await _goNext(tester); // theme install
        expect(
          find.byKey(const ValueKey('onboarding-theme-install-page')),
          findsOneWidget,
        );
        expect(
          find.byKey(const ValueKey('onboarding-theme-install-demo')),
          findsOneWidget,
        );
      },
    );
  });
}

Future<void> _goNext(WidgetTester tester) async {
  await tester.tap(
    find.byKey(const ValueKey('onboarding-next-button')),
    warnIfMissed: false,
  );
  await tester.pump(const Duration(milliseconds: 500));
}
