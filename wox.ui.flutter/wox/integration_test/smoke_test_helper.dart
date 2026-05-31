import 'dart:async';
import 'dart:io';

import 'package:extended_text_field/extended_text_field.dart';
import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
// The latency helpers return the concrete tail entity, so this test helper
// must import the tail model directly instead of relying on wox_query.dart.
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_launch_mode_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_result_tail_text_category_enum.dart';
import 'package:wox/enums/wox_start_page_enum.dart';
import 'package:wox/main.dart' as app;
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/modules/setting/views/wox_setting_view.dart';
import 'package:wox/utils/wox_http_util.dart';
import 'package:wox/utils/heartbeat_checker.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';
import 'package:wox/utils/test/wox_test_config.dart';
import 'package:wox/utils/windows/window_manager.dart';

const Size smokeLargeWindowSize = Size(1200, 900);
const double smokeWindowPositionTolerance = 1;
const int _windowsAltVirtualKey = 18;
const int _windowsAltScanCode = 56;
const Size _smokeBootstrapWindowSize = Size(800, 600);

class SmokeLaunchResult {
  const SmokeLaunchResult({required this.controller, required this.elapsed});

  final WoxLauncherController controller;
  final Duration elapsed;
}

class ScreenWorkArea {
  const ScreenWorkArea({required this.x, required this.y, required this.width, required this.height});

  final int x;
  final int y;
  final int width;
  final int height;

  factory ScreenWorkArea.fromJson(Map<String, dynamic> json) {
    return ScreenWorkArea(
      x: (json['x'] ?? json['X']) as int,
      y: (json['y'] ?? json['Y']) as int,
      width: (json['width'] ?? json['Width']) as int,
      height: (json['height'] ?? json['Height']) as int,
    );
  }
}

Future<void> resetSmokeAppState() async {
  HeartbeatChecker().init();
  await WoxWebsocketMsgUtil.instance.init();
  Get.reset();
}

void registerLauncherTestCleanup(WidgetTester tester, WoxLauncherController controller) {
  addTearDown(() async {
    await controller.resetForIntegrationTest();

    await restoreSmokeWindowStateForNextTest();

    // Hide the window so the backend resets its visibility state.  Use
    // windowManager.hide() directly — controller.hideApp() involves async API
    // calls that may hang during tearDown.
    if (await windowManager.isVisible()) {
      await windowManager.hide();
    }

    // On Windows, fully unmount the previous app tree so that focus listeners
    // are disposed during teardown instead of surviving until the next key event.
    // This is NOT needed on macOS, and calling pumpWidget during teardown on macOS can cause
    // issues with the hidden window's vsync signals, causing pump() to block indefinitely.  Only do this on Windows
    if (Platform.isWindows) {
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump();
    }
  });
}

Future<void> ensureSmokeWindowReadyForFirstPump() async {
  // Bug fix: Flutter's macOS test attach only best-effort foregrounds the app
  // with `open`, and that can fail while the window still reports visible. A
  // visible but non-frontmost macOS panel can stop the first tester.pump() from
  // receiving vsync, so force the native show path on macOS instead of relying
  // on visibility alone. The native show implementation is used because it also
  // calls makeKeyAndOrderFront and NSApp.activate.
  if (Platform.isMacOS || !await windowManager.isVisible()) {
    await windowManager.show();
  }
}

Future<void> startSmokeAppBeforeFirstPump({required Duration timeout, Future<void> Function()? beforeRunApp}) async {
  await ensureSmokeWindowReadyForFirstPump();

  try {
    // Bug fix: onboarding smoke must seed the durable first-run setting before
    // runApp schedules the /on/ready callback. Calling app.main as one block
    // let the backend read the previous test's OnboardingFinished value, so
    // dedicated first-run cases could open the launcher and wait forever for
    // onboarding. Keeping the production startup order here, with one smoke-only
    // hook after services are initialized, makes initial backend routing
    // deterministic without adding a product-only test branch.
    await app.initialServices([WoxTestConfig.serverPort.toString(), '-1', 'true']).timeout(timeout);
    await beforeRunApp?.call();
    await app.initWindow().timeout(timeout);
    await app.initDeepLink().timeout(timeout);
    runApp(const app.MyApp());
  } on TimeoutException {
    fail('Wox app main did not complete before the first pump within $timeout.');
  }

  // waitUntilReadyToShow can adjust macOS panel flags after the earlier show
  // call. Re-activate once runApp has been scheduled so the first pump has a
  // foreground window to drive vsync.
  await ensureSmokeWindowReadyForFirstPump();
}

Future<void> seedOnboardingFinishedBeforeReady(bool finished) async {
  // Smoke tests usually validate launcher behavior, not the first-run wizard.
  // Seed the new onboarding flag after Env is initialized but before the first
  // frame notifies Go via /on/ready, so existing startup tests keep measuring
  // launcher readiness while dedicated onboarding tests opt out explicitly.
  final traceId = const UuidV4().generate();
  await WoxApi.instance.updateSetting(traceId, 'OnboardingFinished', finished.toString());
  await WoxSettingUtil.instance.loadSetting(traceId);
  if (Get.isRegistered<WoxSettingController>()) {
    Get.find<WoxSettingController>().woxSetting.value = WoxSettingUtil.instance.currentSetting;
  }
}

Future<WoxLauncherController> launchLauncherApp(WidgetTester tester, {bool onboardingFinished = true}) async {
  // Ensure the window is visible before any pump() call.  On macOS, a hidden
  // window stops delivering vsync signals, which causes pump() to block.
  // The previous test's tearDown hides the window for backend state cleanup.
  await resetSmokeAppState();
  await startSmokeAppBeforeFirstPump(timeout: const Duration(seconds: 30), beforeRunApp: () => seedOnboardingFinishedBeforeReady(onboardingFinished));

  final launcherFinder = find.byType(WoxLauncherView);
  await pumpUntil(tester, () => launcherFinder.evaluate().isNotEmpty, timeout: const Duration(seconds: 30));
  expect(launcherFinder, findsOneWidget);

  final controller = Get.find<WoxLauncherController>();
  registerLauncherTestCleanup(tester, controller);
  return controller;
}

Future<SmokeLaunchResult> launchLauncherAppAndMeasureStartup(WidgetTester tester, {Duration timeout = const Duration(seconds: 5)}) async {
  await resetSmokeAppState();

  final stopwatch = Stopwatch()..start();
  await startSmokeAppBeforeFirstPump(timeout: timeout, beforeRunApp: () => seedOnboardingFinishedBeforeReady(true));

  final launcherFinder = find.byType(WoxLauncherView);
  await pumpUntil(tester, () => launcherFinder.evaluate().isNotEmpty, timeout: timeout);

  final remaining = timeout - stopwatch.elapsed;
  if (remaining.isNegative) {
    fail('Launcher widget appeared after ${stopwatch.elapsed}, exceeding timeout $timeout.');
  }

  await waitForWindowVisibility(tester, true, timeout: remaining);
  stopwatch.stop();

  expect(launcherFinder, findsOneWidget);
  final controller = Get.find<WoxLauncherController>();
  registerLauncherTestCleanup(tester, controller);
  return SmokeLaunchResult(controller: controller, elapsed: stopwatch.elapsed);
}

Future<WoxLauncherController> launchOnboardingApp(WidgetTester tester) async {
  await resetSmokeAppState();
  await startSmokeAppBeforeFirstPump(timeout: const Duration(seconds: 30), beforeRunApp: () => seedOnboardingFinishedBeforeReady(false));

  final onboardingFinder = find.byKey(const ValueKey('onboarding-view'));
  await tester.pump(const Duration(milliseconds: 500));
  if (onboardingFinder.evaluate().isEmpty) {
    // Bug fix: the backend handles /on/ready only once per smoke process, so
    // onboarding cases cannot depend on startup routing after the startup smoke
    // has consumed it. Explicitly opening the guide keeps these cases focused
    // on onboarding behavior in both filtered and full-suite runs.
    await Get.find<WoxLauncherController>().openOnboarding(const UuidV4().generate());
  }
  await pumpUntil(tester, () => onboardingFinder.evaluate().isNotEmpty, timeout: const Duration(seconds: 30));
  expect(onboardingFinder, findsOneWidget);

  final controller = Get.find<WoxLauncherController>();
  registerLauncherTestCleanup(tester, controller);
  return controller;
}

Future<WoxLauncherController> launchAndShowLauncher(WidgetTester tester, {Size? windowSize}) async {
  final controller = await launchLauncherApp(tester);

  await updateSettingDirect('LangCode', 'en_US');
  await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code);
  await updateSettingDirect('StartPage', WoxStartPageEnum.WOX_START_PAGE_BLANK.code);
  await updateSettingDirect('ShowPerformanceTail', 'true');
  await triggerBackendShowApp(tester);
  // Use a bounded pump instead of pumpAndSettle because showApp() calls
  // focusQueryBox() which starts the cursor blink timer. The periodic blink
  // keeps scheduling frames, preventing pumpAndSettle from ever settling.
  await tester.pump(const Duration(milliseconds: 500));

  if (windowSize != null) {
    await ensureWindowSize(tester, windowSize);
  }

  expect(await windowManager.isVisible(), isTrue);
  return controller;
}

Future<void> hideLauncherIfVisible(WidgetTester tester, WoxLauncherController controller) async {
  if (!await windowManager.isVisible()) {
    return;
  }

  await hideLauncherByEscape(tester, controller);
}

Future<void> waitForWindowVisibility(WidgetTester tester, bool visible, {Duration timeout = const Duration(seconds: 30)}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    if (await windowManager.isVisible() == visible) {
      return;
    }
    // When waiting for the window to become hidden, use Future.delayed instead
    // of tester.pump() because macOS stops delivering vsync signals for hidden
    // windows, causing pump() to block indefinitely.
    if (!visible) {
      await Future.delayed(const Duration(milliseconds: 200));
    } else {
      await tester.pump(const Duration(milliseconds: 200));
    }
  }

  fail('Window visibility did not become $visible within $timeout.');
}

Future<void> updateSettingDirect(String key, String value) async {
  final traceId = const UuidV4().generate();
  await WoxApi.instance.updateSetting(traceId, key, value);
  await WoxSettingUtil.instance.loadSetting(traceId);

  // Keep lastLaunchMode in sync so hideApp uses the correct mode immediately,
  // matching the behavior of WoxSettingController.updateConfig.
  if (key == 'LaunchMode') {
    final controller = Get.find<WoxLauncherController>();
    controller.lastLaunchMode = value;
  }
}

Future<void> updatePluginSettingDirect(String pluginId, String key, String value) async {
  // Speed smoke tests need deterministic plugin fixtures. Existing helpers only
  // cover global Wox settings, which was not enough to seed plugin-local data
  // such as shell aliases or custom app directories before querying.
  await WoxApi.instance.updatePluginSetting(const UuidV4().generate(), pluginId, key, value);
}

Future<void> saveLastWindowPosition(int x, int y) async {
  await WoxApi.instance.saveWindowPosition(const UuidV4().generate(), x, y);
}

Future<void> restoreSmokeWindowStateForNextTest() async {
  await updateSettingDirect('ShowPosition', WoxPositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code);
  await saveLastWindowPosition(-1, -1);

  if (!Platform.isWindows && !Platform.isMacOS) {
    return;
  }

  final screen = await getMouseScreenWorkArea();
  final position = getCenteredTopLeftForWindowSize(screen, _smokeBootstrapWindowSize);
  await windowManager.setBounds(position, _smokeBootstrapWindowSize);
}

Future<void> triggerBackendShowApp(WidgetTester tester) async {
  await WoxHttpUtil.instance.postData<String>(const UuidV4().generate(), '/show', null);
  await waitForWindowVisibility(tester, true);
}

Future<void> triggerTestQueryHotkey(WidgetTester tester, String query, {bool isSilentExecution = false}) async {
  await WoxHttpUtil.instance.postData<String>(const UuidV4().generate(), '/test/trigger/query_hotkey', {'Query': query, 'IsSilentExecution': isSilentExecution});
  await waitForWindowVisibility(tester, true);
}

Future<void> triggerTestOpenSetting(WidgetTester tester, {String path = '', String param = '', String source = ''}) async {
  await WoxHttpUtil.instance.postData<String>(const UuidV4().generate(), '/test/trigger/open_setting', {'Path': path, 'Param': param, 'Source': source});
  await tester.pump(const Duration(milliseconds: 500));
}

Future<void> triggerTestOpenOnboarding(WidgetTester tester) async {
  await WoxHttpUtil.instance.postData<String>(const UuidV4().generate(), '/test/trigger/open_onboarding', null);
  await tester.pump(const Duration(milliseconds: 500));
}

Future<void> triggerTestSelectionHotkey(WidgetTester tester, {required String type, String text = '', List<String> filePaths = const []}) async {
  await WoxHttpUtil.instance.postData<String>(const UuidV4().generate(), '/test/trigger/selection_hotkey', {'Type': type, 'Text': text, 'FilePaths': filePaths});
  await waitForWindowVisibility(tester, true);
}

Future<void> triggerTestTrayQuery(
  WidgetTester tester, {
  required String query,
  bool hideQueryBox = false,
  bool hideToolbar = false,
  int width = 0,
  int x = 200,
  int y = 40,
  int rectWidth = 40,
  int rectHeight = 40,
}) async {
  await WoxHttpUtil.instance.postData<String>(const UuidV4().generate(), '/test/trigger/tray_query', {
    'Query': query,
    'Width': width,
    'HideQueryBox': hideQueryBox,
    'HideToolbar': hideToolbar,
    'Rect': {'X': x, 'Y': y, 'Width': rectWidth, 'Height': rectHeight},
  });
  await waitForWindowVisibility(tester, true);
}

Future<Offset> waitForWindowPosition(
  WidgetTester tester,
  Offset expected, {
  double tolerance = smokeWindowPositionTolerance,
  Duration timeout = const Duration(seconds: 30),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 200));
    final actual = await windowManager.getPosition();
    if (isOffsetClose(actual, expected, tolerance: tolerance)) {
      return actual;
    }
  }

  final actual = await windowManager.getPosition();
  fail('Window position did not reach expected $expected within $timeout. Actual: $actual');
}

Offset getCenteredTopLeftForWindowSize(ScreenWorkArea screen, Size windowSize) {
  final expectedX = screen.x + ((screen.width - windowSize.width) / 2).round();
  final expectedY = screen.y + ((screen.height - windowSize.height) / 2).round();
  return Offset(expectedX.toDouble(), expectedY.toDouble());
}

bool isOffsetClose(Offset actual, Offset expected, {double tolerance = smokeWindowPositionTolerance}) {
  return (actual.dx - expected.dx).abs() <= tolerance && (actual.dy - expected.dy).abs() <= tolerance;
}

Future<ScreenWorkArea> getMouseScreenWorkArea() async {
  final response = await WoxHttpUtil.instance.getData<Map<String, dynamic>>(const UuidV4().generate(), '/test/screen/mouse');
  return ScreenWorkArea.fromJson(response);
}

Future<Offset> getExpectedMouseScreenCenterTopLeft() async {
  final screen = await getMouseScreenWorkArea();
  final setting = WoxSettingUtil.instance.currentSetting;
  final theme = WoxThemeUtil.instance.currentTheme.value;

  final queryBoxHeight = 55 + theme.appPaddingTop + theme.appPaddingBottom;
  final resultItemHeight = 50 + theme.resultItemPaddingTop + theme.resultItemPaddingBottom;
  final resultListViewHeight = resultItemHeight * (setting.maxResultCount == 0 ? 10 : setting.maxResultCount);
  final resultContainerHeight = resultListViewHeight + theme.resultContainerPaddingTop + theme.resultContainerPaddingBottom;
  final maxWindowHeight = queryBoxHeight + resultContainerHeight + 40;

  final expectedX = screen.x + (screen.width - setting.appWidth) ~/ 2;
  final expectedY = screen.y + (screen.height - maxWindowHeight) ~/ 2;
  return Offset(expectedX.toDouble(), expectedY.toDouble());
}

Future<void> ensureWindowSize(WidgetTester tester, Size size) async {
  await windowManager.setSize(size);
  // pumpAndSettle is safe here because this is called during launcher setup,
  // before any text input that would start cursor blink timers.
  await tester.pumpAndSettle();
}

Future<void> hideLauncherByEscape(WidgetTester tester, WoxLauncherController controller, {Duration timeout = const Duration(seconds: 30)}) async {
  // Do NOT use systemInput.keyPress(escape) here.  Native OS-level key events
  // travel through the macOS event pipeline asynchronously — the KeyUpEvent
  // for Escape can arrive after Get.reset() clears Flutter's HardwareKeyboard
  // state in the next test, triggering a "physical key is not pressed"
  // assertion failure.  Calling hideApp directly is reliable and avoids the
  // async keyboard state mismatch.
  await controller.hideApp(const UuidV4().generate());
  await waitForWindowVisibility(tester, false, timeout: timeout);
}

Future<void> enterQueryTextAndWait(WidgetTester tester, WoxLauncherController controller, String query, {Duration timeout = const Duration(seconds: 30)}) async {
  final extendedTextFieldFinder = find.byType(ExtendedTextField);
  expect(extendedTextFieldFinder, findsOneWidget);

  await tester.tap(extendedTextFieldFinder);
  await tester.pump(const Duration(milliseconds: 200));

  tester.testTextInput.enterText(query);
  await tester.pump();

  // Keep smoke tests on the real text-input path so paste and formatter regressions
  // are caught before query-result assertions look at plugin behavior.
  await pumpUntil(tester, () => controller.queryBoxTextFieldController.text == query && controller.currentQuery.value.queryText == query, timeout: timeout);
}

Future<void> enterQueryAndWaitForResults(WidgetTester tester, WoxLauncherController controller, String query, {Duration timeout = const Duration(seconds: 30)}) async {
  await enterQueryTextAndWait(tester, controller, query, timeout: timeout);
  final currentQueryId = controller.currentQuery.value.queryId;

  // Suppress transient overflow errors that occur during the window resize
  // transition when results first appear and the layout hasn't settled yet.
  final oldHandler = FlutterError.onError;
  FlutterError.onError = (details) {
    if (details.exception is FlutterError && details.exception.toString().contains('overflowed')) {
      return;
    }
    oldHandler?.call(details);
  };

  await pumpUntil(tester, () => controller.activeResultViewController.items.any((item) => item.value.data.queryId == currentQueryId), timeout: timeout);

  // Pump a few more frames to let the resize settle before restoring the error handler.
  // Bug fix: screenshot smoke can leave the macOS panel visible but non-frontmost
  // before later system-plugin queries run. Re-activate before the settling pump
  // so this shared helper does not block forever waiting for hidden/non-frontmost
  // vsync after results have already arrived.
  await ensureSmokeWindowReadyForFirstPump();
  await tester.pump(const Duration(milliseconds: 500));
  FlutterError.onError = oldHandler;
}

String normalizeSmokeText(String value) {
  return value.trim().toLowerCase();
}

List<WoxQueryResult> getActiveResults(WoxLauncherController controller) {
  return controller.activeResultViewController.items.map((item) => item.value.data).toList();
}

WoxQueryResult expectActiveResult(WoxLauncherController controller) {
  final activeResults = getActiveResults(controller);
  expect(activeResults, isNotEmpty);
  return controller.activeResultViewController.activeItem.data;
}

WoxQueryResult? findActiveResultWhere(WoxLauncherController controller, bool Function(WoxQueryResult result) predicate) {
  for (final result in getActiveResults(controller)) {
    if (predicate(result)) {
      return result;
    }
  }

  return null;
}

WoxQueryResult expectActiveResultWhere(WoxLauncherController controller, bool Function(WoxQueryResult result) predicate, {String? description}) {
  final result = findActiveResultWhere(controller, predicate);
  expect(result, isNotNull, reason: description ?? 'Expected at least one visible result matching the provided predicate.');
  return result!;
}

Future<WoxQueryResult> queryAndWaitForResultWhere(
  WidgetTester tester,
  WoxLauncherController controller,
  String query,
  bool Function(WoxQueryResult result) predicate, {
  String? description,
  Duration timeout = const Duration(seconds: 30),
}) async {
  await queryAndWaitForResults(tester, controller, query, timeout: timeout);
  await pumpUntil(tester, () => findActiveResultWhere(controller, predicate) != null, timeout: timeout);
  return expectActiveResultWhere(controller, predicate, description: description);
}

List<WoxResultAction> findResultActionsByName(WoxQueryResult result, String actionName, {bool exactMatch = false}) {
  final normalizedActionName = normalizeSmokeText(actionName);
  return result.actions.where((action) {
    final normalizedName = normalizeSmokeText(action.name);
    if (exactMatch) {
      return normalizedName == normalizedActionName;
    }
    return normalizedName.contains(normalizedActionName);
  }).toList();
}

bool isSmokeDebugTextTail(WoxListItemTail tail) {
  final text = tail.text;
  if (text == null) {
    return false;
  }

  return RegExp(r'^P\d+$').hasMatch(text) || RegExp(r'^\d+ms$').hasMatch(text) || text.startsWith('score:');
}

List<WoxListItemTail> getSmokeBusinessTails(WoxQueryResult result) {
  // Development smoke runs append batch/latency/score diagnostics after the
  // plugin-provided tails. Strip those debug-only annotations here so content
  // assertions keep validating the plugin payload instead of a moving tail count.
  return result.tails.where((tail) => !isSmokeDebugTextTail(tail)).toList();
}

WoxResultAction expectResultActionByName(WoxQueryResult result, String actionName, {bool exactMatch = false}) {
  final actions = findResultActionsByName(result, actionName, exactMatch: exactMatch);
  expect(actions, isNotEmpty, reason: 'Expected action "$actionName" in result "${result.title}".');
  return actions.first;
}

WoxListItemTail expectQueryLatencyTail(WoxQueryResult result, {String? description}) {
  // These speed smoke tests gate on the same debug latency tail the launcher
  // renders in development builds. Looking up the tail here keeps every test
  // aligned with the actual yellow/red indicator shown to users.
  for (final tail in result.tails) {
    final text = tail.text;
    if (text != null && RegExp(r'^\d+ms$').hasMatch(text)) {
      return tail;
    }
  }

  fail(description ?? 'Expected a query latency tail on result "${result.title}", but none was found.');
}

int expectQueryLatencyWithinThreshold(WoxQueryResult result, {int maxMs = 10, bool allowWarning = false, bool allowDanger = false}) {
  final latencyTail = expectQueryLatencyTail(result);
  final latencyMs = int.parse(latencyTail.text!.replaceAll('ms', ''));
  final disallowedCategories =
      allowDanger
          ? <String>[]
          : allowWarning
          ? [woxListItemTailTextCategoryDanger]
          : [woxListItemTailTextCategoryWarning, woxListItemTailTextCategoryDanger];
  final expectedLatencyBand =
      allowDanger
          ? 'bounded by the explicit latency ceiling'
          : allowWarning
          ? 'non-danger'
          : 'neutral';

  expect(latencyMs, lessThanOrEqualTo(maxMs), reason: 'Expected "${result.title}" to return within ${maxMs}ms, got ${latencyMs}ms.');
  if (disallowedCategories.isNotEmpty) {
    expect(
      latencyTail.textCategory,
      isNot(anyOf(disallowedCategories)),
      reason: 'Expected "${result.title}" to keep the latency tail $expectedLatencyBand, got ${latencyTail.textCategory}.',
    );
  }

  return latencyMs;
}

Future<void> sendWindowsKeyboardEvent({required String type, required bool isAltPressed}) async {
  if (!Platform.isWindows) {
    return;
  }

  final data = const StandardMethodCodec().encodeMethodCall(
    MethodCall('onKeyboardEvent', {
      'type': type,
      'keyCode': _windowsAltVirtualKey,
      'scanCode': _windowsAltScanCode,
      'isShiftPressed': false,
      'isControlPressed': false,
      'isAltPressed': isAltPressed,
      'isMetaPressed': false,
    }),
  );

  await TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger.handlePlatformMessage('com.wox.windows_window_manager', data, (_) {});
}

Future<void> holdQuickSelectModifier(WidgetTester tester, {Duration holdDuration = const Duration(milliseconds: 350)}) async {
  if (Platform.isWindows) {
    // Keep the Windows bridge in sync while still driving Flutter's keyboard
    // pipeline, because quick select listens through onKeyEvent and
    // HardwareKeyboard.
    await sendWindowsKeyboardEvent(type: 'keydown', isAltPressed: true);
  }

  await tester.sendKeyDownEvent(LogicalKeyboardKey.altLeft);
  await tester.pump(holdDuration);
}

Future<void> releaseQuickSelectModifier(WidgetTester tester) async {
  await tester.sendKeyUpEvent(LogicalKeyboardKey.altLeft);

  if (Platform.isWindows) {
    await sendWindowsKeyboardEvent(type: 'keyup', isAltPressed: false);
  }

  await tester.pump(const Duration(milliseconds: 200));
}

Future<WoxSettingController> openSettings(WidgetTester tester, WoxLauncherController launcherController, String path) async {
  await triggerTestOpenSetting(tester, path: path);

  await pumpUntil(tester, () => launcherController.isInSettingView.value && find.byType(WoxSettingView).evaluate().isNotEmpty, timeout: const Duration(seconds: 30));

  expect(launcherController.isInSettingView.value, isTrue);
  expect(find.byType(WoxSettingView), findsOneWidget);
  return Get.find<WoxSettingController>();
}

Future<void> closeSettings(WidgetTester tester, WoxSettingController settingController, WoxLauncherController launcherController) async {
  final backButtonFinder = find.byKey(const ValueKey('settings-back-button'));
  expect(backButtonFinder, findsOneWidget);
  // Avoid tester.ensureVisible which calls pumpAndSettle (10-min timeout).
  // If the cursor blink timer is still active, pumpAndSettle never settles.
  // The back button is always visible at the bottom of the fixed sidebar.
  await tester.pump();
  await tester.tap(backButtonFinder, warnIfMissed: false);
  await tester.pump(const Duration(milliseconds: 500));

  final fallbackDeadline = DateTime.now().add(const Duration(seconds: 2));
  while (DateTime.now().isBefore(fallbackDeadline)) {
    await tester.pump(const Duration(milliseconds: 200));
    if (!launcherController.isInSettingView.value) {
      return;
    }
  }

  settingController.hideWindow(const UuidV4().generate());
  await pumpUntil(tester, () => launcherController.isInSettingView.value == false, timeout: const Duration(seconds: 30));
}

Future<void> tapSettingNavItem(WidgetTester tester, WoxSettingController settingController, String navPath, {Duration timeout = const Duration(seconds: 30)}) async {
  final navItemFinder = find.byKey(ValueKey('settings-nav-$navPath'));
  if (navItemFinder.evaluate().isEmpty) {
    final navScrollable = find.descendant(of: find.byKey(const ValueKey('settings-nav-list')), matching: find.byType(Scrollable));
    await tester.scrollUntilVisible(navItemFinder, 120, scrollable: navScrollable, duration: const Duration(milliseconds: 100), continuous: true);
  }
  expect(navItemFinder, findsOneWidget);
  // Avoid tester.ensureVisible which calls pumpAndSettle (10-min timeout).
  // If the cursor blink timer is still active from the query box, pumpAndSettle
  // will never settle. Nav items are always visible in the fixed sidebar.
  await tester.pump();
  await tester.tap(navItemFinder, warnIfMissed: false);
  await tester.pump(const Duration(milliseconds: 500));
  await pumpUntil(tester, () => settingController.activeNavPath.value == navPath, timeout: timeout);
}

Future<void> queryAndWaitForResults(WidgetTester tester, WoxLauncherController controller, String query, {Duration timeout = const Duration(seconds: 30)}) async {
  await enterQueryAndWaitForResults(tester, controller, query, timeout: timeout);
}

Future<WoxQueryResult> queryAndWaitForActiveResult(WidgetTester tester, WoxLauncherController controller, String query, {Duration timeout = const Duration(seconds: 30)}) async {
  await queryAndWaitForResults(tester, controller, query, timeout: timeout);
  return expectActiveResult(controller);
}

Future<void> waitForActiveResults(WidgetTester tester, WoxLauncherController controller, {Duration timeout = const Duration(seconds: 30)}) async {
  await pumpUntil(tester, () => controller.activeResultViewController.items.isNotEmpty, timeout: timeout);
}

Future<void> waitForQueryBoxFocus(WidgetTester tester, WoxLauncherController controller, {Duration timeout = const Duration(seconds: 30)}) async {
  await pumpUntil(tester, () => controller.queryBoxFocusNode.hasFocus, timeout: timeout);
}

Future<void> waitForQueryBoxText(WidgetTester tester, WoxLauncherController controller, String expectedText, {Duration timeout = const Duration(seconds: 30)}) async {
  await pumpUntil(tester, () => controller.queryBoxTextFieldController.text == expectedText, timeout: timeout);
}

Future<void> waitForNoResults(WidgetTester tester, WoxLauncherController controller, {Duration timeout = const Duration(seconds: 30)}) async {
  await pumpUntil(tester, () => controller.resultListViewController.items.isEmpty && controller.resultGridViewController.items.isEmpty, timeout: timeout);
}

Future<void> waitForNoActiveResults(WidgetTester tester, WoxLauncherController controller, {Duration timeout = const Duration(seconds: 30)}) async {
  await pumpUntil(tester, () => controller.activeResultViewController.items.isEmpty, timeout: timeout);
}

Future<void> waitForWindowHeightToMatchController(
  WidgetTester tester,
  WoxLauncherController controller, {
  double tolerance = 2,
  Duration timeout = const Duration(seconds: 10),
  Duration step = const Duration(milliseconds: 200),
}) async {
  final deadline = DateTime.now().add(timeout);
  Size? lastActual;
  double? lastExpected;
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(step);
    final actual = await windowManager.getSize();
    final expected = controller.calculateWindowHeight();
    lastActual = actual;
    lastExpected = expected;
    if ((actual.height - expected).abs() <= tolerance) {
      return;
    }
  }

  // Keep the failing smoke actionable: resize regressions are timing sensitive,
  // and the last sampled window/controller heights show whether the native
  // resize never happened, happened too late, or settled outside tolerance.
  fail(
    'Window height did not match controller.calculateWindowHeight() within $timeout. '
    'Last actual: $lastActual, last expected height: $lastExpected, tolerance: $tolerance, '
    'items: ${controller.activeResultViewController.items.length}, '
    'preview: ${controller.isShowPreviewPanel.value}, action: ${controller.isShowActionPanel.value}, '
    'formAction: ${controller.isShowFormActionPanel.value}, grid: ${controller.isInGridMode()}, '
    'previewType: ${controller.currentPreview.value.previewType}, '
    'placeholder: ${controller.isShowingPendingResultPlaceholder}.',
  );
}

Future<void> pumpUntil(WidgetTester tester, bool Function() condition, {required Duration timeout}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 200));
    if (condition()) {
      return;
    }
  }

  fail('Condition not met within $timeout.');
}

Future<Map<String, dynamic>> triggerTestScreenshot() async {
  return await WoxHttpUtil.instance.postData<Map<String, dynamic>>(const UuidV4().generate(), '/test/trigger/screenshot', {});
}
