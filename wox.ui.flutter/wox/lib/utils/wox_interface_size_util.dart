import 'package:get/get.dart';
import 'package:wox/utils/consts.dart';

// WoxInterfaceSizeMetrics centralises every density-scaled size, padding, and
// spacing value used across the launcher UI. Fields are grouped by module so
// that a single change here propagates consistently to all rendering sites.
// Keep numeric base values in sync with Go's density.go when they affect
// window-placement geometry (query box height, result row height, toolbar).
class WoxInterfaceSizeMetrics {
  final String density;
  final double scale;

  // ── Query box ─────────────────────────────────────────────────────────────
  // queryBoxBaseHeight is shared with the Go backend (ui/density.go) for window
  // height estimation; change both sides together to avoid misplacement.
  final double queryBoxBaseHeight;
  final double queryBoxFontSize;
  final double queryBoxIconSize;
  // Glance pill layout. queryBoxGlanceIconAndGapWidth covers the icon image and
  // the small gap between icon and label so only one field is needed.
  final double queryBoxGlanceFontSize;
  final double queryBoxGlanceHPadding;
  final double queryBoxGlanceTextSafetyWidth;
  final double queryBoxGlanceMinWidth;
  final double queryBoxGlanceMaxWidth;
  final double queryBoxGlanceIconAndGapWidth;
  final double queryBoxGlanceItemSpacing;
  // Width reserved for the right accessory area when no glance chip is visible.
  final double queryBoxRightAccessoryWidth;
  // Query refinement controls sit between the query box and results, so this
  // height participates in window geometry just like the query box itself.
  final double queryRefinementBarHeight;

  // ── Result item ───────────────────────────────────────────────────────────
  // resultItemBaseHeight is the content-only height; theme padding is added on
  // top. Keeping layout spacings here ensures icon, tail, and subtitle positions
  // shift together with font and icon sizes when density changes.
  final double resultItemBaseHeight;
  final double resultTitleFontSize;
  final double resultSubtitleFontSize;
  final double resultIconSize;
  final double resultItemIconPaddingLeft;
  final double resultItemIconPaddingRight;
  final double resultItemSubtitlePaddingTop;
  final double resultItemTailPaddingLeft;
  final double resultItemTailPaddingRight;
  final double resultItemTailItemPaddingLeft;
  final double resultItemQuickSelectPaddingLeft;
  final double resultItemQuickSelectPaddingRight;
  final double resultItemTextTailHPadding;
  final double resultItemTextTailVPadding;

  // ── Tail (shared by result and action rows) ────────────────────────────────
  final double tailHotkeyFontSize;
  final double tailImageSize;
  final double quickSelectSize;

  // ── List empty state ──────────────────────────────────────────────────────
  // Empty-match text is shared by result and action lists but should not borrow
  // resultTitleFontSize. Keeping a dedicated metric lets the action panel stay
  // visually quieter while density still scales the empty-state label together
  // with the rest of the launcher interface.
  final double listEmptyStateFontSize;

  // ── Action item ───────────────────────────────────────────────────────────
  // Action rows are shorter than result rows (40px vs 50px base), so their
  // icon and title sizes are scaled down independently to fit the smaller row.
  final double actionItemBaseHeight;
  final double actionHeaderFontSize;
  final double actionIconSize;
  final double actionTitleFontSize;

  // ── Action panel (floating overlay) ───────────────────────────────────────
  // Position offsets and size constraints for the floating action/form panels
  // that appear over the result list. Centralising them ensures density changes
  // affect overlay geometry consistently with the surrounding launcher UI.
  final double actionPanelOffsetRight;
  final double actionPanelOffsetBottom;
  final double actionPanelMaxWidth;
  final double actionFormMaxWidth;
  final double actionFormMaxHeight;

  // ── Toolbar ───────────────────────────────────────────────────────────────
  // Hotkey chip sizes (toolbarHotkeyKeySize / toolbarHotkeyKeySpacing) mirror
  // WoxHotkeyView so overflow decisions in _calculateActionWidth match rendering.
  final double toolbarHeight;
  final double toolbarFontSize;
  final double toolbarIconSize;
  final double toolbarIconSpacing;
  final double toolbarProgressSize;
  final double toolbarProgressStrokeWidth;
  final double toolbarActionSpacing;
  final double toolbarActionNameHotkeySpacing;
  final double toolbarRightReservedWidth;
  final double toolbarHotkeyKeySize;
  final double toolbarHotkeyKeySpacing;

  // ── Preview ───────────────────────────────────────────────────────────────
  // The quote treatment uses its own set of paddings separate from plain-text
  // padding so both can be adjusted independently without affecting each other.
  final double previewMarkdownPadding;
  final double previewTextFontSize;
  final double previewTextQuoteFontSize;
  final double previewTextQuoteHPadding;
  final double previewTextQuoteTopPadding;
  final double previewTextQuoteBottomPadding;
  final double previewTextQuoteGlyphSize;
  final double previewTextQuoteTextTopPadding;
  final double previewTextQuoteTextBottomPadding;
  final double previewTextQuoteGlyphOffset;
  final double previewTextPadding;

  // ── Grid ──────────────────────────────────────────────────────────────────
  // Group header and cell title measurements for the grid result surface.
  // Font sizes are independent from tailHotkeyFontSize even though the numeric
  // base matches today; they may diverge when grid layout requirements change.
  // Bug fix: grid headers now expose an explicit height so WoxGridController
  // does not rely on a stale hard-coded 32px value when calculating window
  // height and active-row scroll offsets.
  final double gridGroupHeaderHeight;
  final double gridGroupHeaderPaddingLeft;
  final double gridGroupHeaderPaddingTop;
  final double gridGroupHeaderPaddingBottom;
  final double gridTitleHeight;
  final double gridGroupHeaderFontSize;
  final double gridItemTitleFontSize;

  // ── General ───────────────────────────────────────────────────────────────
  // smallLabelFontSize is the shared base for any secondary/caption UI text
  // that is not a tail hotkey, a result subtitle, or a toolbar label. Using a
  // named field prevents UI code from borrowing tailHotkeyFontSize (which is
  // semantically tied to hotkey chips and list-item tail text) as a catch-all
  // 11 px value, keeping intent clear when the two sizes diverge in the future.
  final double smallLabelFontSize;

  const WoxInterfaceSizeMetrics({
    required this.density,
    required this.scale,
    // query box
    required this.queryBoxBaseHeight,
    required this.queryBoxFontSize,
    required this.queryBoxIconSize,
    required this.queryBoxGlanceFontSize,
    required this.queryBoxGlanceHPadding,
    required this.queryBoxGlanceTextSafetyWidth,
    required this.queryBoxGlanceMinWidth,
    required this.queryBoxGlanceMaxWidth,
    required this.queryBoxGlanceIconAndGapWidth,
    required this.queryBoxGlanceItemSpacing,
    required this.queryBoxRightAccessoryWidth,
    required this.queryRefinementBarHeight,
    // result item
    required this.resultItemBaseHeight,
    required this.resultTitleFontSize,
    required this.resultSubtitleFontSize,
    required this.resultIconSize,
    required this.resultItemIconPaddingLeft,
    required this.resultItemIconPaddingRight,
    required this.resultItemSubtitlePaddingTop,
    required this.resultItemTailPaddingLeft,
    required this.resultItemTailPaddingRight,
    required this.resultItemTailItemPaddingLeft,
    required this.resultItemQuickSelectPaddingLeft,
    required this.resultItemQuickSelectPaddingRight,
    required this.resultItemTextTailHPadding,
    required this.resultItemTextTailVPadding,
    // tail
    required this.tailHotkeyFontSize,
    required this.tailImageSize,
    required this.quickSelectSize,
    // list empty state
    required this.listEmptyStateFontSize,
    // action item
    required this.actionItemBaseHeight,
    required this.actionHeaderFontSize,
    required this.actionIconSize,
    required this.actionTitleFontSize,
    // action panel
    required this.actionPanelOffsetRight,
    required this.actionPanelOffsetBottom,
    required this.actionPanelMaxWidth,
    required this.actionFormMaxWidth,
    required this.actionFormMaxHeight,
    // toolbar
    required this.toolbarHeight,
    required this.toolbarFontSize,
    required this.toolbarIconSize,
    required this.toolbarIconSpacing,
    required this.toolbarProgressSize,
    required this.toolbarProgressStrokeWidth,
    required this.toolbarActionSpacing,
    required this.toolbarActionNameHotkeySpacing,
    required this.toolbarRightReservedWidth,
    required this.toolbarHotkeyKeySize,
    required this.toolbarHotkeyKeySpacing,
    // preview
    required this.previewMarkdownPadding,
    required this.previewTextFontSize,
    required this.previewTextQuoteFontSize,
    required this.previewTextQuoteHPadding,
    required this.previewTextQuoteTopPadding,
    required this.previewTextQuoteBottomPadding,
    required this.previewTextQuoteGlyphSize,
    required this.previewTextQuoteTextTopPadding,
    required this.previewTextQuoteTextBottomPadding,
    required this.previewTextQuoteGlyphOffset,
    required this.previewTextPadding,
    // grid
    required this.gridGroupHeaderHeight,
    required this.gridGroupHeaderPaddingLeft,
    required this.gridGroupHeaderPaddingTop,
    required this.gridGroupHeaderPaddingBottom,
    required this.gridTitleHeight,
    required this.gridGroupHeaderFontSize,
    required this.gridItemTitleFontSize,
    // general
    required this.smallLabelFontSize,
  });

  factory WoxInterfaceSizeMetrics.fromDensity(String value) {
    final density = WoxInterfaceSizeUtil.normalizeDensity(value);
    // Keep these scale values in sync with Go's scaledDensityHeight in
    // wox.core/ui/density.go. Flutter renders the launcher with these metrics,
    // while Go estimates window height before Flutter paints; if only one side
    // changes, compact/comfortable windows can be mispositioned or clipped.
    final scale = switch (density) {
      WoxInterfaceSizeUtil.compact => 0.9,
      WoxInterfaceSizeUtil.comfortable => 1.1,
      _ => 1.0,
    };

    double scaled(double base) => (base * scale).roundToDouble();

    return WoxInterfaceSizeMetrics(
      density: density,
      scale: scale,
      // query box
      queryBoxBaseHeight: scaled(QUERY_BOX_BASE_HEIGHT),
      queryBoxFontSize: scaled(28),
      queryBoxIconSize: scaled(30),
      queryBoxGlanceFontSize: scaled(15),
      queryBoxGlanceHPadding: scaled(16),
      queryBoxGlanceTextSafetyWidth: scaled(4),
      queryBoxGlanceMinWidth: scaled(44),
      queryBoxGlanceMaxWidth: scaled(192),
      queryBoxGlanceIconAndGapWidth: scaled(21),
      queryBoxGlanceItemSpacing: scaled(8),
      queryBoxRightAccessoryWidth: scaled(68),
      queryRefinementBarHeight: scaled(44),
      // result item
      resultItemBaseHeight: scaled(RESULT_ITEM_BASE_HEIGHT),
      resultTitleFontSize: scaled(15),
      resultSubtitleFontSize: scaled(12),
      resultIconSize: scaled(28),
      resultItemIconPaddingLeft: scaled(5),
      resultItemIconPaddingRight: scaled(10),
      resultItemSubtitlePaddingTop: scaled(2),
      resultItemTailPaddingLeft: scaled(10),
      resultItemTailPaddingRight: scaled(5),
      resultItemTailItemPaddingLeft: scaled(10),
      resultItemQuickSelectPaddingLeft: scaled(10),
      resultItemQuickSelectPaddingRight: scaled(5),
      resultItemTextTailHPadding: scaled(8),
      resultItemTextTailVPadding: scaled(3),
      // tail
      tailHotkeyFontSize: scaled(11),
      tailImageSize: scaled(20),
      quickSelectSize: scaled(20),
      // list empty state
      listEmptyStateFontSize: scaled(13),
      // action item
      actionItemBaseHeight: scaled(ACTION_ITEM_BASE_HEIGHT),
      actionHeaderFontSize: scaled(13),
      actionIconSize: scaled(22),
      actionTitleFontSize: scaled(13),
      // action panel
      actionPanelOffsetRight: scaled(10),
      actionPanelOffsetBottom: scaled(10),
      actionPanelMaxWidth: scaled(320),
      actionFormMaxWidth: scaled(360),
      actionFormMaxHeight: scaled(400),
      // toolbar
      toolbarHeight: scaled(TOOLBAR_HEIGHT),
      toolbarFontSize: scaled(12),
      toolbarIconSize: scaled(24),
      toolbarIconSpacing: scaled(8),
      toolbarProgressSize: scaled(14),
      toolbarProgressStrokeWidth: scaled(2),
      toolbarActionSpacing: scaled(16),
      toolbarActionNameHotkeySpacing: scaled(8),
      toolbarRightReservedWidth: scaled(200),
      toolbarHotkeyKeySize: scaled(28),
      toolbarHotkeyKeySpacing: scaled(4),
      // preview
      previewMarkdownPadding: scaled(20),
      previewTextFontSize: scaled(15),
      previewTextQuoteFontSize: scaled(17),
      previewTextQuoteHPadding: scaled(44),
      previewTextQuoteTopPadding: scaled(12),
      previewTextQuoteBottomPadding: scaled(4),
      previewTextQuoteGlyphSize: scaled(72),
      previewTextQuoteTextTopPadding: scaled(62),
      previewTextQuoteTextBottomPadding: scaled(62),
      previewTextQuoteGlyphOffset: scaled(22),
      previewTextPadding: scaled(24),
      // grid
      gridGroupHeaderHeight: scaled(32),
      gridGroupHeaderPaddingLeft: scaled(8),
      gridGroupHeaderPaddingTop: scaled(12),
      gridGroupHeaderPaddingBottom: scaled(4),
      gridTitleHeight: scaled(18),
      gridGroupHeaderFontSize: scaled(13),
      gridItemTitleFontSize: scaled(12),
      // general
      smallLabelFontSize: scaled(11),
    );
  }

  double scaledSpacing(double base) => (base * scale).roundToDouble();

  double get queryBoxLineHeight => queryBoxBaseHeight - QUERY_BOX_CONTENT_PADDING_TOP - QUERY_BOX_CONTENT_PADDING_BOTTOM;

  // Keep editable text layout inside the fixed query-box content band so font
  // metrics cannot drift away from the launcher height calculation.
  double get queryBoxTextHeightFactor => queryBoxLineHeight / queryBoxFontSize;
}

class WoxInterfaceSizeUtil {
  static const compact = 'compact';
  static const normal = 'normal';
  static const comfortable = 'comfortable';

  WoxInterfaceSizeUtil._privateConstructor();

  static final WoxInterfaceSizeUtil _instance = WoxInterfaceSizeUtil._privateConstructor();

  static WoxInterfaceSizeUtil get instance => _instance;

  final Rx<WoxInterfaceSizeMetrics> metrics = WoxInterfaceSizeMetrics.fromDensity(normal).obs;

  WoxInterfaceSizeMetrics get current => metrics.value;

  static String normalizeDensity(String value) {
    switch (value.trim().toLowerCase()) {
      case compact:
        return compact;
      case comfortable:
        return comfortable;
      default:
        return normal;
    }
  }

  void refreshFromDensity(String density) {
    final nextMetrics = WoxInterfaceSizeMetrics.fromDensity(density);
    if (nextMetrics.density == metrics.value.density) {
      return;
    }

    // Density changes affect launcher-only measurements. Keeping the metrics
    // in one observable avoids adding per-size fields to the backend DTO while
    // still letting the launcher rebuild and resize immediately after reload.
    metrics.value = nextMetrics;
  }
}
