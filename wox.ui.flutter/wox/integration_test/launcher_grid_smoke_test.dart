import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:wox/components/wox_grid_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';

import 'smoke_test_helper.dart';

const String _onePixelPngBase64 = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  registerLauncherGridSmokeTests();
}

void registerLauncherGridSmokeTests() {
  group('T8: Grid View Smoke Tests', () {
    testWidgets('T8-01: emoji grid honors explicit item padding', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);

      await queryAndWaitForResults(tester, controller, 'emoji smile');
      await pumpUntil(tester, () => find.byType(WoxGridView).evaluate().isNotEmpty, timeout: const Duration(seconds: 10));

      final params = controller.gridLayoutParams.value;
      expect(controller.isGridLayout.value, isTrue);
      // Smoke coverage follows the emoji plugin's current grid contract; the
      // important regression guard below is that explicit ItemPadding remains
      // authoritative for content sizing.
      expect(params.columns, equals(10));
      expect(params.itemPadding, equals(12));
      expect(params.itemMargin, equals(6));
      expect(params.aspectRatio, equals(1.0));

      final expectedContentSize = _expectedGridContentSize(tester, params);
      final emojiImages = _visibleImagesOfType(tester, WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code);

      // Bug coverage: plugin-specified ItemPadding must remain authoritative.
      // The grid outline is paint-only, so this assertion checks the content
      // box produced from the explicit emoji padding instead of an implicit
      // border or fallback value.
      expect(emojiImages, isNotEmpty);
      expect(emojiImages.any((image) => _isClose(image.width, expectedContentSize.width) && _isClose(image.height, expectedContentSize.height)), isTrue);
    });

    testWidgets('T8-02: media grid respects aspect ratio and keeps image size stable while selection moves', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const traceId = 'grid-smoke-aspect-ratio';
      const queryId = 'grid-smoke-aspect-ratio-query';
      final query = PlainQuery(queryId: queryId, queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: 'emoji smoke', querySelection: Selection.empty());
      final params = GridLayoutParams(columns: 4, showTitle: false, itemPadding: 0, itemMargin: 6, aspectRatio: 16 / 9, commands: const []);

      controller.currentQuery.value = query;
      controller.backendQueryContext = QueryContext(isGlobalQuery: false, pluginId: 'grid-smoke');
      controller.backendQueryContextQueryId = query.queryId;
      controller.applyQueryLayoutForQuery(traceId, query, QueryLayout(icon: WoxImage.empty(), resultPreviewWidthRatio: 0.5, isGridLayout: true, gridLayoutParams: params));
      await controller.onReceivedQueryResults(traceId, queryId, _buildSyntheticMediaResults(queryId, 4), isFinal: true);
      await pumpUntil(tester, () => find.byType(WoxGridView).evaluate().isNotEmpty, timeout: const Duration(seconds: 10));
      await tester.pump(const Duration(milliseconds: 300));

      final expectedContentSize = _expectedGridContentSize(tester, params);
      final initialMediaImages = _visibleImagesOfType(tester, WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code);

      // Feature coverage: wallpaper/media grids declare a width/height ratio,
      // and Wox owns the row-height math instead of leaving thumbnails in
      // square icon cells.
      expect(initialMediaImages, hasLength(greaterThanOrEqualTo(4)));
      for (final image in initialMediaImages.take(4)) {
        expect(image.width, closeTo(expectedContentSize.width, 0.5));
        expect(image.height, closeTo(expectedContentSize.height, 0.5));
      }

      final widthsBeforeSelectionMove = initialMediaImages.take(4).map((image) => image.width).toList();
      final heightsBeforeSelectionMove = initialMediaImages.take(4).map((image) => image.height).toList();

      controller.resultGridViewController.updateActiveIndexByDirection(traceId, WoxDirectionEnum.WOX_DIRECTION_RIGHT.code);
      await tester.pump(const Duration(milliseconds: 200));

      final imagesAfterSelectionMove = _visibleImagesOfType(tester, WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code).take(4).toList();

      // Bug coverage: the active outline is paint-only. Moving selection should
      // not change image layout or reintroduce the previous border-driven scale
      // jump.
      expect(imagesAfterSelectionMove.map((image) => image.width).toList(), equals(widthsBeforeSelectionMove));
      expect(imagesAfterSelectionMove.map((image) => image.height).toList(), equals(heightsBeforeSelectionMove));

      controller.applyQueryLayoutForQuery(
        traceId,
        query,
        QueryLayout(icon: WoxImage.empty(), resultPreviewWidthRatio: 0.0, isGridLayout: false, gridLayoutParams: GridLayoutParams.empty()),
      );

      // Regression coverage: QueryResponse layout must preserve an explicit
      // zero preview ratio. Zero means "preview takes the whole result area",
      // so treating it as an unset fallback would keep the old 0.5 split view.
      expect(controller.preferredResultPreviewRatio, equals(0.0));

      final pluginIcon = WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: _onePixelPngBase64);
      controller.applyQueryLayoutForQuery(traceId, query, QueryLayout(icon: pluginIcon, resultPreviewWidthRatio: 0.5, isGridLayout: true, gridLayoutParams: params));
      controller.prepareQueryLayoutOnQueryChanged(
        traceId,
        PlainQuery(
          queryId: 'grid-smoke-next-plugin-query',
          queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
          queryText: 'emoji smoke next',
          querySelection: Selection.empty(),
        ),
      );

      // Regression coverage: plugin-shaped queries keep the current plugin icon
      // until QueryResponse carries the next layout, avoiding a clear-then-set
      // flicker that the old async metadata request path did not have.
      expect(controller.queryIcon.value.icon.imageData, equals(pluginIcon.imageData));

      final newerPluginQuery = PlainQuery(
        queryId: 'grid-smoke-newer-plugin-query',
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: 'emoji smoke newer',
        querySelection: Selection.empty(),
      );
      controller.currentQuery.value = newerPluginQuery;
      expect(controller.applyQueryContextForQueryId(traceId, 'grid-smoke-old-global-query', QueryContext(isGlobalQuery: true, pluginId: '')), isFalse);

      // Bug coverage: QueryContext responses are asynchronous just like result
      // batches. A stale backend classification must not overwrite the current
      // query's accessory state.
      expect(controller.queryIcon.value.icon.imageData, equals(pluginIcon.imageData));

      final backendGlobalQuery = PlainQuery(
        queryId: 'grid-smoke-backend-global-query',
        queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
        queryText: 'ui ilwidlksiek',
        querySelection: Selection.empty(),
      );
      controller.currentQuery.value = backendGlobalQuery;
      expect(controller.applyQueryContextForQueryId(traceId, backendGlobalQuery.queryId, QueryContext(isGlobalQuery: true, pluginId: '')), isTrue);

      // Regression coverage: backend QueryContext owns the final global/plugin
      // classification. A global query that contains spaces must clear stale
      // plugin chrome so Glance can occupy the query-box accessory again.
      expect(controller.isGlobalInputQuery(backendGlobalQuery), isTrue);
      expect(controller.queryIcon.value.icon.imageData, isEmpty);

      controller.currentQuery.value = query;
      controller.backendQueryContext = QueryContext(isGlobalQuery: false, pluginId: 'grid-smoke');
      controller.backendQueryContextQueryId = query.queryId;
      controller.applyQueryLayoutForQuery(traceId, query, QueryLayout(icon: pluginIcon, resultPreviewWidthRatio: 0.5, isGridLayout: true, gridLayoutParams: params));
      controller.prepareQueryLayoutOnQueryChanged(
        traceId,
        PlainQuery(queryId: 'grid-smoke-global-query', queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: 'grid', querySelection: Selection.empty()),
      );
      expect(controller.queryIcon.value.icon.imageData, isEmpty);
    });
  });
}

List<WoxQueryResult> _buildSyntheticMediaResults(String queryId, int count) {
  return List.generate(count, (index) {
    return WoxQueryResult(
      queryId: queryId,
      id: '$queryId-$index',
      title: 'Grid Smoke Media $index',
      subTitle: 'Synthetic media result for grid smoke',
      icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: _onePixelPngBase64),
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

Size _expectedGridContentSize(WidgetTester tester, GridLayoutParams params) {
  final gridSize = tester.getSize(find.byType(WoxGridView));
  final cellWidth = params.columns > 0 ? (gridSize.width / params.columns).floorToDouble() : 48.0;
  final contentWidth = cellWidth - (params.itemPadding + params.itemMargin) * 2;
  return Size(contentWidth, contentWidth / params.aspectRatio);
}

List<WoxImageView> _visibleImagesOfType(WidgetTester tester, String imageType) {
  return tester.widgetList<WoxImageView>(find.byType(WoxImageView)).where((image) => image.woxImage.imageType == imageType && image.width != null && image.height != null).toList();
}

bool _isClose(double? actual, double expected) {
  if (actual == null) {
    return false;
  }
  return (actual - expected).abs() <= 0.5;
}
