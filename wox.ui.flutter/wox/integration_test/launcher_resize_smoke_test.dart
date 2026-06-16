import 'dart:io';

import 'package:integration_test/integration_test.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/utils/windows/window_manager.dart';

import 'smoke_test_helper.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  registerLauncherResizeSmokeTests();
}

void registerLauncherResizeSmokeTests() {
  group('T7: Resize Smoke Tests', () {
    testWidgets('T7-01: smaller result snapshots shrink the window immediately', (tester) async {
      if (!Platform.isWindows) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const traceId = 'resize-smoke-deferred-shrink';
      const queryId = 'resize-smoke-expanded-query';
      final query = PlainQuery(
        queryId: queryId,
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: 'resize smoke synthetic shrink',
        querySelection: Selection.empty(),
      );

      prepareSyntheticResizeQuery(controller, query);
      await controller.onReceivedQueryResults(traceId, queryId, buildSyntheticResults(queryId, 4), isFinal: true);
      await waitForWindowHeightToMatchController(tester, controller);
      final expandedHeight = (await windowManager.getSize()).height;

      await controller.onReceivedQueryResults(traceId, queryId, buildSyntheticResults(queryId, 1), isFinal: true);
      await waitForWindowHeightToMatchController(tester, controller, timeout: const Duration(milliseconds: 80), step: const Duration(milliseconds: 16));
      final shrunkHeight = (await windowManager.getSize()).height;
      expect(shrunkHeight, lessThan(expandedHeight - 2));
    });

    testWidgets('T7-02: non-final empty snapshots keep visible results stable', (tester) async {
      if (!Platform.isWindows) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const traceId = 'resize-smoke-non-final-empty';
      const queryId = 'resize-smoke-query';
      final query = PlainQuery(
        queryId: queryId,
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: 'resize smoke synthetic stable',
        querySelection: Selection.empty(),
      );

      prepareSyntheticResizeQuery(controller, query);
      await controller.onReceivedQueryResults(traceId, queryId, buildSyntheticResults(queryId, 4), isFinal: true);
      await waitForWindowHeightToMatchController(tester, controller);
      final stableHeight = (await windowManager.getSize()).height;

      await controller.onReceivedQueryResults(traceId, queryId, const [], isFinal: false);
      await tester.pump(const Duration(milliseconds: 150));
      final heightAfterNonFinalEmpty = (await windowManager.getSize()).height;

      expect(controller.activeResultViewController.items.length, greaterThanOrEqualTo(4));
      expect((heightAfterNonFinalEmpty - stableHeight).abs(), lessThanOrEqualTo(2));
    });

    testWidgets('T7-03: query changes drop stale results after grace without shrinking immediately', (tester) async {
      if (!Platform.isWindows) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const traceId = 'resize-smoke-stale-grace';
      const queryId = 'resize-smoke-old-query';
      final query = PlainQuery(queryId: queryId, queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: 'resize smoke stale source', querySelection: Selection.empty());

      prepareSyntheticResizeQuery(controller, query);
      await controller.onReceivedQueryResults(traceId, queryId, buildSyntheticResults(queryId, 4), isFinal: true);
      await waitForWindowHeightToMatchController(tester, controller);
      final oldHeight = (await windowManager.getSize()).height;

      const nextQueryId = 'resize-smoke-new-query';
      await controller.onQueryChanged(
        traceId,
        PlainQuery(queryId: nextQueryId, queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: 'settings x', querySelection: Selection.empty()),
        'resize smoke next query',
      );

      expect(controller.activeResultViewController.items.length, greaterThanOrEqualTo(4));

      await tester.pump(const Duration(milliseconds: 120));
      final heightAfterGrace = (await windowManager.getSize()).height;

      // Global queries can surface fallback rows for the new query during the
      // grace window. What must disappear here is the stale snapshot from the
      // previous query, not necessarily every visible row or every shrink. The
      // production bug fix keeps real final/current-query snapshots responsive
      // so the launcher can still shrink as soon as the backend has settled.
      expect(controller.activeResultViewController.items.where((item) => item.value.data.queryId == queryId), isEmpty);
      if (!controller.isCurrentQueryReturned) {
        expect((heightAfterGrace - oldHeight).abs(), lessThanOrEqualTo(2));
      }
    });

    testWidgets('T7-04: final empty snapshots shrink immediately', (tester) async {
      if (!Platform.isWindows) {
        return;
      }

      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const traceId = 'resize-smoke-final-empty';
      const queryId = 'resize-smoke-final-empty-query';
      final query = PlainQuery(
        queryId: queryId,
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: 'resize smoke synthetic empty',
        querySelection: Selection.empty(),
      );

      prepareSyntheticResizeQuery(controller, query);
      await controller.onReceivedQueryResults(traceId, queryId, buildSyntheticResults(queryId, 4), isFinal: true);
      await waitForWindowHeightToMatchController(tester, controller);
      final expandedHeight = (await windowManager.getSize()).height;

      await controller.onReceivedQueryResults(traceId, queryId, const [], isFinal: true);
      expect(controller.activeResultViewController.items, isEmpty);
      await waitForWindowHeightToMatchController(tester, controller, timeout: const Duration(milliseconds: 80), step: const Duration(milliseconds: 16));
      final compactHeight = (await windowManager.getSize()).height;
      expect(compactHeight, lessThan(expandedHeight - 2));
    });
  });
}

void prepareSyntheticResizeQuery(WoxLauncherController controller, PlainQuery query) {
  // Bug fix: resize smoke cases inject handcrafted snapshots and must not call
  // onQueryChanged, because that also starts a real backend query whose results
  // can race in and replace the synthetic rows before the height assertion.
  // This setup keeps the controller on the same current-query contract that
  // onReceivedQueryResults validates while leaving backend behavior out of the
  // direct resize checks.
  controller.cancelPendingResultTransitions();
  controller.currentQuery.value = query;
  controller.queryBoxTextFieldController.text = query.queryText;
  controller.isCurrentQueryReturned = false;
  controller.isLoading.value = false;
  controller.isGridLayout.value = false;
  controller.resultListViewController.clearItems();
  controller.resultGridViewController.clearItems();
  controller.actionListViewController.clearItems();
  controller.isShowPreviewPanel.value = false;
  controller.isShowActionPanel.value = false;
  controller.isShowFormActionPanel.value = false;
  controller.currentPreview.value = WoxPreview.empty();
  controller.syncPreviewFullscreenState();
}

List<WoxQueryResult> buildSyntheticResults(String queryId, int count) {
  return List.generate(count, (index) {
    return WoxQueryResult(
      queryId: queryId,
      id: '$queryId-$index',
      title: 'Synthetic Result $index',
      subTitle: 'Synthetic Subtitle $index',
      icon: WoxImage.empty(),
      preview: WoxPreview.empty(),
      score: 100 - index,
      group: '',
      groupScore: 0,
      tails: const [],
      actions: const [],
      isGroup: false,
    );
  });
}
