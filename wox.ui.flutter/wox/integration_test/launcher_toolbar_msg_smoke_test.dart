import 'package:flutter_test/flutter_test.dart';
import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_toolbar.dart';

import 'smoke_test_helper.dart';

void registerLauncherToolbarMsgSmokeTests() {
  group('T5: Toolbar Msg Smoke Tests', () {
    testWidgets('T5-01: Toolbar msg stays visible without results and notify cannot override it', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final traceId = const UuidV4().generate();
      final msg = ToolbarMsg(id: 'indexing', title: 'Indexing files', icon: null, progress: 40, indeterminate: false, actions: const []);

      await controller.showToolbarMsg(traceId, msg);
      await tester.pump();

      expect(controller.activeResultViewController.items, isEmpty);
      expect(controller.isShowToolbar, isTrue);
      expect(controller.isToolbarShowedWithoutResults, isTrue);
      expect(controller.resolvedToolbarText, equals('Indexing files'));
      expect(controller.resolvedToolbarProgress, equals(40));

      controller.showToolbarMsg(traceId, ToolbarMsg(text: 'notify should not win'));
      await tester.pump();

      expect(controller.resolvedToolbarText, equals('Indexing files'));
      expect(controller.toolbar.value.text == 'notify should not win', isFalse);
    });

    testWidgets('T5-02: Toolbar msg actions override conflicting result hotkeys and restore after clear', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final result = await queryAndWaitForActiveResult(tester, controller, '1+1');
      final copyAction = expectResultActionByName(result, 'copy');

      expect(copyAction.hotkey, isNotEmpty);

      final traceId = const UuidV4().generate();
      final msg = ToolbarMsg(
        id: 'calculator-status',
        title: 'Calculating',
        icon: null,
        progress: null,
        indeterminate: true,
        actions: [ToolbarMsgActionInfo(id: 'retry', name: 'Retry', icon: null, hotkey: copyAction.hotkey, isDefault: false, preventHideAfterAction: true, contextData: const {})],
      );

      await controller.showToolbarMsg(traceId, msg);
      await tester.pump();

      final msgWinner = controller.getActionByToolbarHotkey(result, copyAction.hotkey);
      expect(msgWinner, isNotNull);
      expect(msgWinner!.name, equals('Retry'));

      final unifiedActionsWithMsg = controller.buildUnifiedActions(traceId, result);
      final restoredCopyWhileMsgVisible = unifiedActionsWithMsg.firstWhere((action) => action.name == copyAction.name);
      expect(restoredCopyWhileMsgVisible.hotkey, isEmpty);

      await controller.clearToolbarMsg(traceId, 'calculator-status');
      await tester.pump();

      final restoredWinner = controller.getActionByToolbarHotkey(result, copyAction.hotkey);
      expect(restoredWinner, isNotNull);
      expect(restoredWinner!.name, equals(copyAction.name));
      expect(controller.buildUnifiedActions(traceId, result).any((action) => action.name == 'Retry'), isFalse);
    });

    testWidgets('T5-03: Later toolbar msg updates replace the visible one', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      final traceId = const UuidV4().generate();

      await controller.showToolbarMsg(traceId, const ToolbarMsg(id: 'download-a', title: 'Downloading A', progress: 10));
      await tester.pump();
      expect(controller.resolvedToolbarText, equals('Downloading A'));
      expect(controller.resolvedToolbarProgress, equals(10));

      await controller.showToolbarMsg(traceId, const ToolbarMsg(id: 'download-b', title: 'Downloading B', progress: 80));
      await tester.pump();
      expect(controller.resolvedToolbarText, equals('Downloading B'));
      expect(controller.resolvedToolbarProgress, equals(80));

      await controller.clearToolbarMsg(traceId, 'download-b');
      await tester.pump();
      expect(controller.hasVisibleToolbarMsg, isFalse);
    });

    testWidgets('T5-04: Bug aware mode forces persistent toolbar indicator', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      controller.updateDiagnosticStatus(const UuidV4().generate(), true);
      await tester.pump();

      // New feature: bug aware mode is launcher-owned UI state, so it must keep
      // the toolbar visible without pretending to be a plugin ShowToolbarMsg.
      expect(controller.activeResultViewController.items, isEmpty);
      expect(controller.isShowToolbar, isTrue);
      expect(controller.isToolbarShowedWithoutResults, isTrue);
      expect(controller.hasBugAwareToolbarIndicator, isTrue);

      await controller.activateBugReportQuery(const UuidV4().generate());
      await tester.pump();
      expect(controller.currentQuery.value.queryText, equals('bugreport '));

      controller.updateDiagnosticStatus(const UuidV4().generate(), false);
      await tester.pump();
      expect(controller.hasBugAwareToolbarIndicator, isFalse);
    });
  });
}
