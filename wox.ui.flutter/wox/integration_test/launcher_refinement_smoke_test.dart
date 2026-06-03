import 'package:flutter_test/flutter_test.dart';
import 'package:wox/components/refinement/wox_query_refinement_bar_view.dart';
import 'package:wox/components/refinement/wox_query_refinement_multi_select_view.dart';
import 'package:wox/components/refinement/wox_query_refinement_single_select_view.dart';
import 'package:wox/components/refinement/wox_query_refinement_sort_view.dart';
import 'package:wox/components/refinement/wox_query_refinement_toggle_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:integration_test/integration_test.dart';
import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/utils/wox_platform_hotkey_util.dart';

import 'smoke_test_helper.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  registerLauncherRefinementSmokeTests();
}

void registerLauncherRefinementSmokeTests() {
  group('T10: Query Refinement Smoke Tests', () {
    testWidgets('T10-01: QueryResponse refinements render controls and update next query values', (tester) async {
      final controller = await launchAndShowLauncher(tester, windowSize: smokeLargeWindowSize);
      const traceId = 'query-refinement-smoke';
      final query = PlainQuery(queryId: 'query-refinement-smoke-query', queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: 'cb ', querySelection: Selection.empty());
      final refinements = _buildSyntheticRefinements();

      controller.currentQuery.value = query;
      controller.queryBoxTextFieldController.text = query.queryText;
      controller.applyQueryRefinementsForQuery(traceId, query, refinements);
      await tester.pump(const Duration(milliseconds: 300));

      // Feature coverage: plugins can advertise refinements without forcing the
      // launcher to spend vertical space until the user explicitly expands them.
      expect(controller.shouldShowQueryRefinementAffordance, isTrue);
      expect(controller.isQueryRefinementBarExpanded.value, isFalse);
      expect(controller.getQueryRefinementBarHeight(), equals(0));
      expect(find.byType(WoxQueryRefinementBarView), findsNothing);
      expect(find.text(controller.tr('ui_query_refinement_filters')), findsOneWidget);

      final toggleHotkey = WoxHotkey.parseHotkeyFromString(controller.queryRefinementToggleHotkey)!.normalHotkey!;
      expect(controller.executeQueryRefinementToggleHotkey(const UuidV4().generate(), toggleHotkey), isTrue);
      await tester.pump(const Duration(milliseconds: 300));

      // Expanded coverage: every public refinement control type has a concrete
      // launcher widget, and the bar reserves height only while expanded.
      expect(find.byType(WoxQueryRefinementBarView), findsOneWidget);
      expect(find.byType(WoxQueryRefinementSingleSelectView), findsOneWidget);
      expect(find.byType(WoxQueryRefinementMultiSelectView), findsOneWidget);
      expect(find.byType(WoxQueryRefinementToggleView), findsOneWidget);
      expect(find.byType(WoxQueryRefinementSortView), findsOneWidget);
      expect(find.text('All'), findsOneWidget);
      expect(find.text('Text (2)'), findsOneWidget);
      expect(find.text('Image (1)'), findsOneWidget);
      expect(controller.getQueryRefinementBarHeight(), greaterThan(0));
      expect(controller.calculateWindowHeight(), greaterThanOrEqualTo(controller.getQueryBoxTotalHeight() + controller.getQueryRefinementBarHeight()));

      controller.toggleQueryRefinementBar(const UuidV4().generate());
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.byType(WoxQueryRefinementBarView), findsNothing);

      final refinementHotkey = WoxHotkey.parseHotkeyFromString(WoxPlatformHotkeyUtil.primaryHotkey('t'))!.normalHotkey!;
      expect(controller.executeQueryRefinementHotkey(const UuidV4().generate(), refinementHotkey), isTrue);
      expect(controller.currentQuery.value.queryRefinements['type'], equals('text'));
      expect(controller.getQueryRefinementAffordanceLabel(), equals('Text'));

      final previousQueryId = controller.currentQuery.value.queryId;
      controller.updateQueryRefinementSelection(const UuidV4().generate(), refinements.first, const ['text']);

      // Bug coverage: selected refinement values must travel on the normal
      // query-change path, otherwise plugins would render a control that never
      // affects the next Query.Refinements payload. Assert before pumping so a
      // real backend clipboard response cannot replace this synthetic payload.
      expect(controller.currentQuery.value.queryId, isNot(previousQueryId));
      expect(controller.currentQuery.value.queryText, equals('cb '));
      expect(controller.currentQuery.value.queryRefinements['type'], equals('text'));
    });
  });
}

List<WoxQueryRefinement> _buildSyntheticRefinements() {
  final emptyIcon = WoxImage.empty();
  return [
    WoxQueryRefinement(
      id: 'type',
      title: 'Type',
      type: 'singleSelect',
      defaultValue: const ['all'],
      hotkey: WoxPlatformHotkeyUtil.primaryHotkey('t'),
      persist: false,
      options: [
        WoxQueryRefinementOption(value: 'all', title: 'All', icon: emptyIcon, keywords: const [], count: null),
        WoxQueryRefinementOption(value: 'text', title: 'Text', icon: emptyIcon, keywords: const [], count: 2),
        WoxQueryRefinementOption(value: 'image', title: 'Image', icon: emptyIcon, keywords: const [], count: 1),
      ],
    ),
    WoxQueryRefinement(
      id: 'source',
      title: 'Source',
      type: 'multiSelect',
      defaultValue: const [],
      hotkey: '',
      persist: false,
      options: [
        WoxQueryRefinementOption(value: 'app', title: 'App', icon: emptyIcon, keywords: const [], count: null),
        WoxQueryRefinementOption(value: 'browser', title: 'Browser', icon: emptyIcon, keywords: const [], count: null),
      ],
    ),
    WoxQueryRefinement(
      id: 'favorite',
      title: 'Pinned',
      type: 'toggle',
      defaultValue: const [],
      hotkey: '',
      persist: false,
      options: [WoxQueryRefinementOption(value: 'yes', title: 'Only pinned', icon: emptyIcon, keywords: const [], count: null)],
    ),
    WoxQueryRefinement(
      id: 'sort',
      title: 'Sort',
      type: 'sort',
      defaultValue: const ['recent'],
      hotkey: '',
      persist: false,
      options: [
        WoxQueryRefinementOption(value: 'recent', title: 'Recent', icon: emptyIcon, keywords: const [], count: null),
        WoxQueryRefinementOption(value: 'oldest', title: 'Oldest', icon: emptyIcon, keywords: const [], count: null),
      ],
    ),
  ];
}
