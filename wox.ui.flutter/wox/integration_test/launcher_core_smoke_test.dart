import 'dart:io';
import 'dart:convert';
import 'dart:ui' as ui;

import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_preview_list.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_launch_mode_enum.dart';
import 'package:wox/enums/wox_position_type_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_result_tail_text_category_enum.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';
import 'package:wox/enums/wox_start_page_enum.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/modules/setting/views/wox_setting_view.dart';
import 'package:wox/utils/windows/window_manager.dart';

import 'smoke_test_helper.dart';

void registerLauncherCoreSmokeTests() {
  group('T2: Core Smoke Tests', () {
    testWidgets('T2-01: Launch main window and verify UI elements', (tester) async {
      final controller = await launchAndShowLauncher(tester);

      expect(find.byType(WoxLauncherView), findsOneWidget);
      expect(await windowManager.isVisible(), isTrue);
      expect(controller.isQueryBoxVisible.value, isTrue);
    });

    testWidgets('T2-02: ShowPosition mouse_screen centers the launcher on the current screen', (tester) async {
      if (!Platform.isWindows && !Platform.isMacOS) {
        return;
      }

      final controller = await launchLauncherApp(tester);
      await hideLauncherIfVisible(tester, controller);

      await updateSettingDirect('ShowPosition', WoxPositionTypeEnum.POSITION_TYPE_MOUSE_SCREEN.code);
      final expectedPosition = await getExpectedMouseScreenCenterTopLeft();
      await triggerBackendShowApp(tester);

      final actualPosition = await waitForWindowPosition(tester, expectedPosition);
      expect(isOffsetClose(actualPosition, expectedPosition), isTrue);
    });

    testWidgets('T2-03: ShowPosition last_location restores the saved window coordinates exactly', (tester) async {
      final controller = await launchLauncherApp(tester);
      await hideLauncherIfVisible(tester, controller);

      const expectedPosition = Offset(240, 180);
      await updateSettingDirect('ShowPosition', WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code);
      await saveLastWindowPosition(expectedPosition.dx.toInt(), expectedPosition.dy.toInt());

      await triggerBackendShowApp(tester);

      final actualPosition = await waitForWindowPosition(tester, expectedPosition);
      expect(isOffsetClose(actualPosition, expectedPosition), isTrue);
    });

    testWidgets('T2-04: Keyboard navigation works', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      await queryAndWaitForResults(tester, controller, 'wox launcher test xyz123');
      await waitForQueryBoxFocus(tester, controller);

      final initialIndex = controller.activeResultViewController.activeIndex.value;
      final resultCount = controller.activeResultViewController.items.length;
      expect(resultCount, greaterThan(0));

      controller.handleQueryBoxArrowDown();
      await tester.pump();
      // Bug fix: smoke fixtures can produce a single deterministic result on
      // macOS. ArrowDown should move to the next result when one exists and
      // wrap to the same index when there is only one result.
      expect(controller.activeResultViewController.activeIndex.value, equals((initialIndex + 1) % resultCount));

      controller.handleQueryBoxArrowUp();
      await tester.pump();
      expect(controller.activeResultViewController.activeIndex.value, equals(initialIndex));

      await controller.hideApp(const UuidV4().generate());
      await waitForWindowVisibility(tester, false);
      expect(await windowManager.isVisible(), isFalse);
    });

    testWidgets('T2-05: Long press Alt shows quick select labels', (tester) async {
      if (!Platform.isWindows) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      await queryAndWaitForResults(tester, controller, 'wox launcher test xyz123');
      controller.focusQueryBox();
      await tester.pump(const Duration(milliseconds: 200));

      expect(controller.isQuickSelectMode.value, isFalse);
      expect(controller.activeResultViewController.items.any((item) => item.value.isShowQuickSelect), isFalse);

      await holdQuickSelectModifier(tester);

      expect(controller.isQuickSelectMode.value, isTrue);
      final quickSelectItems = controller.activeResultViewController.items.where((item) => item.value.isShowQuickSelect).toList();
      expect(quickSelectItems, isNotEmpty);
      expect(quickSelectItems.first.value.quickSelectNumber, equals('1'));

      await releaseQuickSelectModifier(tester);

      expect(controller.isQuickSelectMode.value, isFalse);
      expect(controller.activeResultViewController.items.any((item) => item.value.isShowQuickSelect), isFalse);
    });

    testWidgets('T2-06: Closing settings returns focus to the launcher query box', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final settingController = await openSettings(tester, controller, 'general');

      expect(controller.isInSettingView.value, isTrue);

      await closeSettings(tester, settingController, controller);
      await waitForQueryBoxFocus(tester, controller);

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.isInSettingView.value, isFalse);
      expect(controller.queryBoxFocusNode.hasFocus, isTrue);
    });

    testWidgets('T2-06a: Holding Escape in settings returns to query box without hiding launcher', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      await openSettings(tester, controller, 'general');

      await tester.sendKeyDownEvent(LogicalKeyboardKey.escape);
      await tester.pump(const Duration(milliseconds: 100));

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.isInSettingView.value, isTrue);
      expect(find.byType(WoxSettingView), findsOneWidget);

      await tester.sendKeyRepeatEvent(LogicalKeyboardKey.escape);
      await tester.pump(const Duration(milliseconds: 100));

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.isInSettingView.value, isTrue);
      expect(find.byType(WoxSettingView), findsOneWidget);

      await tester.sendKeyUpEvent(LogicalKeyboardKey.escape);
      await waitForQueryBoxFocus(tester, controller);

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.isInSettingView.value, isFalse);
      expect(controller.queryBoxFocusNode.hasFocus, isTrue);
      expect(find.byType(WoxLauncherView), findsOneWidget);
    });

    testWidgets('T2-06b: Re-show after a hidden settings route opens the launcher query UI', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      await openSettings(tester, controller, 'general');

      await windowManager.hide();
      await waitForWindowVisibility(tester, false);
      expect(controller.isInSettingView.value, isTrue);

      await triggerBackendShowApp(tester);
      await waitForQueryBoxFocus(tester, controller);

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.isInSettingView.value, isFalse);
      expect(controller.queryBoxFocusNode.hasFocus, isTrue);
      expect(find.byType(WoxLauncherView), findsOneWidget);
    });

    testWidgets('T2-06c: Tray-opened settings closes back to hidden state on Escape', (tester) async {
      final controller = await launchLauncherApp(tester);
      await triggerTestOpenSetting(tester, source: SettingWindowContext.sourceTray);
      await pumpUntil(tester, () => controller.isInSettingView.value && find.byType(WoxSettingView).evaluate().isNotEmpty, timeout: const Duration(seconds: 30));

      await tester.sendKeyDownEvent(LogicalKeyboardKey.escape);
      await tester.pump(const Duration(milliseconds: 100));
      await tester.sendKeyUpEvent(LogicalKeyboardKey.escape);

      await waitForWindowVisibility(tester, false);
      expect(controller.isInSettingView.value, isFalse);
    });

    testWidgets('T2-07: Re-show restores query box focus for immediate typing', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      await waitForQueryBoxFocus(tester, controller);
      expect(controller.queryBoxFocusNode.hasFocus, isTrue);

      await hideLauncherByEscape(tester, controller);

      await triggerBackendShowApp(tester);
      await waitForQueryBoxFocus(tester, controller);

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.queryBoxFocusNode.hasFocus, isTrue);
      expect(controller.isInSettingView.value, isFalse);
    });

    testWidgets('T2-07a: Re-show focus retry does not select text typed during startup', (tester) async {
      final controller = await launchLauncherApp(tester);
      await updateSettingDirect('LangCode', 'en_US');
      await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code);
      await updateSettingDirect('StartPage', WoxStartPageEnum.WOX_START_PAGE_BLANK.code);

      await controller.showApp(
        const UuidV4().generate(),
        ShowAppParams(
          selectAll: true,
          position: Position(type: WoxPositionTypeEnum.POSITION_TYPE_LAST_LOCATION.code, x: 200, y: 200),
          queryHistories: [],
          launchMode: WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code,
          startPage: WoxStartPageEnum.WOX_START_PAGE_BLANK.code,
        ),
      );

      controller.queryBoxTextFieldController.value = const TextEditingValue(text: 'q', selection: TextSelection.collapsed(offset: 1));
      controller.onQueryBoxTextChanged('q');
      await tester.pump(const Duration(milliseconds: 150));

      // Regression coverage: the delayed Windows focus retry must not re-apply
      // SelectAll after the user has already typed the first character. Doing
      // so selects "q", and the next key replaces it, producing "ianlifeng".
      expect(controller.queryBoxTextFieldController.text, equals('q'));
      expect(controller.queryBoxTextFieldController.selection.isCollapsed, isTrue);
      expect(controller.queryBoxTextFieldController.selection.baseOffset, equals(1));
    });

    testWidgets('T2-08: Fresh launch clears stale query when shown from the default source', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      await enterQueryTextAndWait(tester, controller, 'wox launcher test xyz123');
      final staleQueryId = controller.currentQuery.value.queryId;
      // Bug fix: this case only needs a stale query/result before hide+show.
      // queryAndWaitForResults adds an extra visual settle pump for resize
      // assertions, and on macOS that non-essential pump can wait forever when
      // earlier show/hide tests leave the panel visible but not frontmost.
      // Stop once the backend result has reached controller state so the fresh
      // launch behavior stays covered without depending on resize-settle vsync.
      await pumpUntil(tester, () => controller.activeResultViewController.items.any((item) => item.value.data.queryId == staleQueryId), timeout: const Duration(seconds: 30));
      expect(controller.queryBoxTextFieldController.text, equals('wox launcher test xyz123'));

      await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code);

      await hideLauncherByEscape(tester, controller);

      await triggerBackendShowApp(tester);
      await waitForQueryBoxFocus(tester, controller);

      expect(controller.queryBoxTextFieldController.text, isEmpty);
      expect(controller.queryBoxFocusNode.hasFocus, isTrue);
    });

    testWidgets('T2-09: Fresh launch preserves a query-hotkey query source', (tester) async {
      final controller = await launchLauncherApp(tester);
      await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code);

      await triggerTestQueryHotkey(tester, 'wox launcher test xyz123');
      await waitForQueryBoxText(tester, controller, 'wox launcher test xyz123');

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.queryBoxTextFieldController.text, equals('wox launcher test xyz123'));
    });

    testWidgets('T2-10: Fresh launch preserves a selection query source payload', (tester) async {
      final controller = await launchLauncherApp(tester);
      await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code);

      await triggerTestSelectionHotkey(tester, type: WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code, text: 'selected smoke text');
      await pumpUntil(
        tester,
        () =>
            controller.currentQuery.value.queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code &&
            controller.currentQuery.value.querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code &&
            controller.currentQuery.value.querySelection.text == 'selected smoke text',
        timeout: const Duration(seconds: 30),
      );

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.currentQuery.value.queryType, equals(WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code));
      expect(controller.currentQuery.value.querySelection.type, equals(WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code));
      expect(controller.currentQuery.value.querySelection.text, equals('selected smoke text'));
      expect(controller.queryBoxTextFieldController.text, isEmpty);
    });

    testWidgets('T2-11: Fresh launch preserves tray-query query and layout payloads', (tester) async {
      final controller = await launchLauncherApp(tester);
      await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code);

      await triggerTestTrayQuery(tester, query: 'tray smoke query', hideQueryBox: false, hideToolbar: true);
      await waitForQueryBoxText(tester, controller, 'tray smoke query');

      expect(await windowManager.isVisible(), isTrue);
      expect(controller.queryBoxTextFieldController.text, equals('tray smoke query'));
      expect(controller.isQueryBoxVisible.value, isTrue);
      expect(controller.isToolbarHiddenForce.value, isTrue);
    });

    testWidgets('T2-12: Continue launch restores the main query after a query hotkey session', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code);

      await queryAndWaitForResults(tester, controller, 'main query xyz123');
      expect(controller.queryBoxTextFieldController.text, equals('main query xyz123'));

      await hideLauncherByEscape(tester, controller);

      await triggerTestQueryHotkey(tester, 'hotkey query abc456');
      await waitForQueryBoxText(tester, controller, 'hotkey query abc456');
      expect(controller.queryBoxTextFieldController.text, equals('hotkey query abc456'));

      await hideLauncherByEscape(tester, controller);

      await triggerBackendShowApp(tester);
      await waitForQueryBoxText(tester, controller, 'main query xyz123');

      expect(controller.queryBoxTextFieldController.text, equals('main query xyz123'));
    });

    testWidgets('T2-13: Action panel opens with Alt+J', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      await queryAndWaitForResults(tester, controller, 'wox launcher test xyz123');

      controller.openActionPanelForActiveResult(const UuidV4().generate());
      await tester.pump(const Duration(milliseconds: 500));

      expect(controller.isShowActionPanel.value, isTrue);
    }, skip: true);

    testWidgets('T2-14: Settings entry is reachable via openSetting', (tester) async {
      final launcherController = await launchAndShowLauncher(tester);

      await openSettings(tester, launcherController, 'general');

      expect(launcherController.isInSettingView.value, isTrue);
      expect(find.byType(WoxSettingView), findsOneWidget);
    });

    testWidgets('T2-15: Settings page basic navigation', (tester) async {
      final launcherController = await launchAndShowLauncher(tester);
      final settingController = await openSettings(tester, launcherController, 'general');

      expect(find.byType(WoxSettingView), findsOneWidget);

      await tapSettingNavItem(tester, settingController, 'general');
      expect(find.byType(WoxSettingView), findsOneWidget);

      await tapSettingNavItem(tester, settingController, 'ui');
      expect(find.byType(WoxSettingView), findsOneWidget);

      await tapSettingNavItem(tester, settingController, 'data');
      expect(find.byType(WoxSettingView), findsOneWidget);

      await closeSettings(tester, settingController, launcherController);
    });

    testWidgets('T2-15a: General query settings expose reusable demo popovers', (tester) async {
      final launcherController = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final settingController = await openSettings(tester, launcherController, 'general');

      for (final demo in const [
        (triggerKey: 'settings-query-hotkeys-demo-trigger', popoverKey: 'wox-demo-popover-queryHotkeys'),
        (triggerKey: 'settings-query-shortcuts-demo-trigger', popoverKey: 'wox-demo-popover-queryShortcuts'),
        (triggerKey: 'settings-tray-queries-demo-trigger', popoverKey: 'wox-demo-popover-trayQueries'),
      ]) {
        final trigger = find.byKey(ValueKey(demo.triggerKey));
        await tester.scrollUntilVisible(trigger, 260, scrollable: find.byType(Scrollable).first, duration: const Duration(milliseconds: 80), continuous: true);
        expect(trigger, findsOneWidget);

        // Smoke coverage: demo previews are hover-only so table editing keeps keyboard focus; moving the synthetic mouse verifies the trigger without clicking the table header.
        final gesture = await tester.createGesture(kind: ui.PointerDeviceKind.mouse);
        final triggerCenter = tester.getCenter(trigger);
        await gesture.addPointer(location: triggerCenter);
        await gesture.moveTo(triggerCenter);
        await tester.pump(const Duration(milliseconds: 450));
        expect(find.byKey(ValueKey(demo.popoverKey)), findsOneWidget);
        await gesture.removePointer();
        await tester.pump(const Duration(milliseconds: 250));
      }

      await closeSettings(tester, settingController, launcherController);
    });

    testWidgets('T2-16: LaunchMode switch via settings syncs hide and show behavior immediately', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      // Open settings general page.
      final settingController = await openSettings(tester, controller, 'general');

      // --- Phase 1: Switch fresh → continue via the UI dropdown ---
      final freshLabel = settingController.tr('ui_launch_mode_fresh');
      final continueLabel = settingController.tr('ui_launch_mode_continue');

      // Tap the dropdown (currently showing "fresh") to open its menu.
      await tester.tap(find.text(freshLabel));
      await tester.pump(const Duration(milliseconds: 300));

      // Tap the "continue" option in the opened dropdown menu.
      // DropdownButton renders two text widgets for the selected item (one in
      // the button, one in the menu), so use .last to tap the menu item.
      await tester.tap(find.text(continueLabel).last);
      await tester.pump(const Duration(milliseconds: 500));

      await closeSettings(tester, settingController, controller);

      // Verify lastLaunchMode was synced immediately.
      expect(controller.lastLaunchMode, equals(WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code));

      // Query and get results.
      await queryAndWaitForResults(tester, controller, 'wox launcher test xyz123');
      expect(controller.activeResultViewController.items, isNotEmpty);
      final sizeWithResults = await windowManager.getSize();

      // Hide and re-show — continue mode should preserve results and height.
      await hideLauncherByEscape(tester, controller);
      await triggerBackendShowApp(tester);
      await tester.pump(const Duration(milliseconds: 500));

      expect(controller.activeResultViewController.items, isNotEmpty, reason: 'Continue mode should preserve results');
      expect(controller.queryBoxTextFieldController.text, equals('wox launcher test xyz123'));
      final sizeAfterContinueReshow = await windowManager.getSize();
      expect(
        (sizeAfterContinueReshow.height - sizeWithResults.height).abs(),
        lessThanOrEqualTo(2),
        reason: 'Continue mode: window height should match (was ${sizeWithResults.height}, got ${sizeAfterContinueReshow.height})',
      );

      // --- Phase 2: Switch continue → fresh via the UI dropdown ---
      final settingController2 = await openSettings(tester, controller, 'general');

      // Dropdown now shows "continue". Tap it to open the menu.
      await tester.tap(find.text(continueLabel));
      await tester.pump(const Duration(milliseconds: 300));

      // Tap the "fresh" option.
      await tester.tap(find.text(freshLabel).last);
      await tester.pump(const Duration(milliseconds: 500));

      await closeSettings(tester, settingController2, controller);

      // Verify lastLaunchMode was synced back to fresh.
      expect(controller.lastLaunchMode, equals(WoxLaunchModeEnum.WOX_LAUNCH_MODE_FRESH.code));

      // Query and get results again.
      await queryAndWaitForResults(tester, controller, 'wox launcher test xyz123');
      expect(controller.activeResultViewController.items, isNotEmpty);

      // Hide and re-show — fresh mode should clear results.
      await hideLauncherByEscape(tester, controller);
      await triggerBackendShowApp(tester);
      await tester.pump(const Duration(milliseconds: 500));

      expect(controller.activeResultViewController.items, isEmpty, reason: 'Fresh mode should clear results on hide');
      expect(controller.queryBoxTextFieldController.text, isEmpty, reason: 'Fresh mode should clear query text on hide');
    });

    testWidgets('T2-17: Continue launch keeps result actions executable after hide and re-show', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      await updateSettingDirect('LaunchMode', WoxLaunchModeEnum.WOX_LAUNCH_MODE_CONTINUE.code);

      var actionExecuted = false;
      final query = PlainQuery.text('retained action smoke');
      query.queryId = const UuidV4().generate();
      controller.currentQuery.value = query;
      controller.queryBoxTextFieldController.text = query.queryText;

      final result = WoxQueryResult(
        queryId: query.queryId,
        id: 'retained-action-result',
        title: 'Retained Action Result',
        subTitle: 'Synthetic smoke result for continue launch action retention',
        icon: WoxImage.empty(),
        preview: WoxPreview.empty(),
        score: 100,
        group: '',
        groupScore: 0,
        tails: const [],
        actions: [
          WoxResultAction.local(
            id: 'retained-action-execute',
            name: 'Execute',
            hotkey: 'enter',
            isDefault: true,
            // Bug fix: this smoke case validates continue-mode result/action
            // retention, so use a deterministic local action instead of waiting
            // for shared global plugin search to surface a settings command.
            handler: (_) {
              actionExecuted = true;
              return true;
            },
          ),
        ],
        isGroup: false,
      );
      await controller.onReceivedQueryResults(const UuidV4().generate(), query.queryId, [result], isFinal: true);
      expectResultActionByName(result, 'execute');

      await hideLauncherByEscape(tester, controller);
      await triggerBackendShowApp(tester);
      await waitForQueryBoxText(tester, controller, query.queryText);
      expect(controller.activeResultViewController.items, isNotEmpty, reason: 'Continue mode should preserve prior results on re-show');

      final resultIndexAfterReshow = controller.activeResultViewController.items.indexWhere((item) => item.value.data.id == result.id);
      expect(resultIndexAfterReshow, greaterThanOrEqualTo(0));
      controller.activeResultViewController.updateActiveIndex(const UuidV4().generate(), resultIndexAfterReshow);
      controller.executeDefaultAction(const UuidV4().generate());
      expect(actionExecuted, isTrue, reason: 'Continue mode should keep retained result actions executable after re-show');
    });

    testWidgets('T2-18: Query box preserves pasted multi-line query text', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const query = 'Problem Statement\n输入的字符超过窗口长度时文字光标无法拖动查看\n\nProposed Solution\n希望可以随光标移动到指定字符';

      await enterQueryTextAndWait(tester, controller, query);

      // Regression coverage: maxLines=1 injects Flutter's single-line formatter and strips
      // pasted newlines before Wox sees the query, so this smoke must assert the accepted value.
      expect(controller.queryBoxTextFieldController.text, equals(query));
      expect(controller.currentQuery.value.queryText, equals(query));
      expect(controller.queryBoxLineCount.value, greaterThan(1));
    });

    testWidgets('T2-19: Query box expands for a visually wrapped long single-line query', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final baseInputHeight = controller.getQueryBoxInputHeight();
      final query = 'Problem Statement ${List.filled(8, '输入的字符超过窗口长度时文字光标需要继续保持可见').join()}';

      await enterQueryTextAndWait(tester, controller, query);
      await pumpUntil(tester, () => controller.queryBoxLineCount.value > 1, timeout: const Duration(seconds: 5));

      // Regression coverage: counting only explicit newlines kept this long one-line query at
      // one visible row, hiding wrapped text and making caret navigation hard to inspect.
      expect(controller.queryBoxTextFieldController.text, equals(query));
      expect(controller.queryBoxTextFieldController.text.contains('\n'), isFalse);
      expect(controller.queryBoxLineCount.value, greaterThan(1));
      expect(controller.getQueryBoxInputHeight(), greaterThan(baseInputHeight));
    });

    testWidgets('T2-20: List preview data keeps row icon title subtitle and tails', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const queryId = 'list-preview-smoke-query';
      final previewData = jsonEncode({
        'items': [
          {
            'icon': {'ImageType': WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, 'ImageData': 'P'},
            'title': 'photo.jpg',
            'subtitle': 'Compressing image',
            'tails': [
              {'Type': WoxListItemTailTypeEnum.WOX_LIST_ITEM_TAIL_TYPE_TEXT.code, 'Text': '42%', 'TextCategory': woxListItemTailTextCategoryWarning},
            ],
          },
        ],
      });

      // Feature coverage: plugins now use one generic list preview contract for
      // status-oriented rows. This smoke keeps the controller-facing payload
      // explicit so SDK and UI changes cannot drift back to file-only fields.
      // Bug fix: onReceivedQueryResults intentionally rejects stale query IDs.
      // The smoke injects controller-facing data directly, so it must first
      // bind the active query to this synthetic result batch instead of relying
      // on whatever query a previous smoke case left behind.
      final query = PlainQuery.text('list preview smoke');
      query.queryId = queryId;
      controller.currentQuery.value = query;
      final result = WoxQueryResult(
        queryId: queryId,
        id: 'list-preview-result',
        title: 'Compress 1 image',
        subTitle: 'Synthetic smoke result for list preview',
        icon: WoxImage.empty(),
        preview: WoxPreview(previewType: WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_LIST.code, previewData: previewData, scrollPosition: ''),
        score: 100,
        group: '',
        groupScore: 0,
        tails: const [],
        actions: const [],
        isGroup: false,
      );

      await controller.onReceivedQueryResults('list-preview-smoke', queryId, [result], isFinal: true);
      await tester.pump(const Duration(milliseconds: 100));

      final activeResult = controller.activeResultViewController.activeItem.data;
      expect(activeResult.preview.previewType, equals(WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_LIST.code));
      final listData = WoxPreviewListData.fromPreviewData(activeResult.preview.previewData);

      expect(listData.items, hasLength(1));
      expect(listData.items.first.icon?.imageType, equals(WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code));
      expect(listData.items.first.title, equals('photo.jpg'));
      expect(listData.items.first.subtitle, equals('Compressing image'));
      expect(listData.items.first.tails, hasLength(1));
      expect(listData.items.first.tails.first.text, equals('42%'));
      expect(listData.items.first.tails.first.textCategory, equals(woxListItemTailTextCategoryWarning));
    });
  });
}
