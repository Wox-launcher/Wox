part of 'wox_demo.dart';

// Animated demo for the welcome onboarding step.
//
// Phase timeline (total 9500ms, looping):
//   0.00–0.20  Concept card visible — static (1900ms); users read the anatomy labels.
//   0.20–0.40  Card slides up + fades; three colored chip ghosts fly toward the
//              Wox query bar.
//   0.28–0.45  Wox window rises in.
//   0.42–0.55  Query bar assembles: '' → 'wpm' → 'wpm install' → 'wpm install everything'.
//              Each token keeps the color it had on the anatomy card.
//   0.57–0.65  Three plugin-store results appear together.
//   0.65–0.70  Brief pause — results fully visible.
//   0.70–0.83  Footer Alt+J hotkey highlights (key-press visual).
//   0.72–0.79  Action panel slides in from bottom-right (665ms rise).
//   0.79–0.95  Action panel holds fully visible (1520ms ≈ 1.5s).
//   0.95–0.99  Action panel fades out.
//   0.99–1.00  Loop pause.
//
// The action panel phase is merged from the removed standalone actionPanel step
// so the full search-to-action story plays out in one continuous animation.
class WoxQueryConceptDemo extends StatefulWidget {
  const WoxQueryConceptDemo({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  State<WoxQueryConceptDemo> createState() => _WoxQueryConceptDemoState();
}

class _WoxQueryConceptDemoState extends State<WoxQueryConceptDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  // Fixed token colors: the anatomy card chips and the Wox query bar spans use
  // identical tints so users visually connect each label to its query segment.
  static const _commandColor = Color(0xFFFACC15);
  static const _searchTermColor = Color(0xFF4ADE80);

  @override
  void initState() {
    super.initState();
    // Duration extended to 9500ms: static card is now 20% (1900ms) instead of
    // the previous 30%, and the action panel phase includes a 1.5s hold window
    // that was not present when this was a separate onboarding step.
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 9500))..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  double _interval(double start, double end, Curve curve) {
    final t = ((_controller.value - start) / (end - start)).clamp(0.0, 1.0);
    return curve.transform(t.toDouble());
  }

  // ── Concept card ─────────────────────────────────────────────────────────
  double get _cardOpacity {
    // Static for the first 20% of the loop; then fades out over the next 20%.
    if (_controller.value < 0.20) return 1.0;
    if (_controller.value < 0.40) return 1.0 - _interval(0.20, 0.40, Curves.easeInCubic);
    return 0.0;
  }

  double get _cardDY {
    if (_controller.value < 0.20) return 0.0;
    return -30.0 * _interval(0.20, 0.40, Curves.easeInCubic);
  }

  // ── Flying chip overlay ───────────────────────────────────────────────────
  // Chips are only rendered during the crossfade window (0.20–0.42).
  bool get _showFlyingChips => _controller.value >= 0.20 && _controller.value < 0.42;

  double get _flyProgress {
    if (_controller.value < 0.20) return 0.0;
    if (_controller.value < 0.42) return _interval(0.20, 0.42, Curves.easeInCubic);
    return 1.0;
  }

  // Chips fade in at lift-off and fade out near the query bar so they vanish
  // just before the colored text spans appear in the bar.
  double _chipAlpha(double fly) {
    if (fly < 0.08) return fly / 0.08;
    if (fly > 0.70) return 1.0 - ((fly - 0.70) / 0.30);
    return 1.0;
  }

  // ── Wox window ───────────────────────────────────────────────────────────
  double get _woxOpacity {
    if (_controller.value < 0.28) return 0.0;
    if (_controller.value < 0.45) return _interval(0.28, 0.45, Curves.easeOutCubic);
    return 1.0;
  }

  double get _woxDY {
    if (_controller.value < 0.28) return 22.0;
    if (_controller.value < 0.45) return 22.0 * (1.0 - _interval(0.28, 0.45, Curves.easeOutCubic));
    return 0.0;
  }

  // The full query typed character-by-character. Each segment keeps its
  // anatomy-card color inside _ConceptDemoWindow._buildSpans.
  static const _fullQuery = 'wpm install everything';

  String get _typedQuery {
    // Bug fix: the previous stage approach showed whole words ('wpm', 'install',
    // 'everything') jumping in at once. Typing one character at a time at a
    // uniform ~65ms/char keeps the animation legible and natural.
    if (_controller.value < 0.42) return '';
    final t = ((_controller.value - 0.42) / (0.57 - 0.42)).clamp(0.0, 1.0);
    return _fullQuery.substring(0, (t * _fullQuery.length).floor().clamp(0, _fullQuery.length));
  }

  // ── Results ───────────────────────────────────────────────────────────────
  // All three results appear together immediately after the query is complete.
  double get _resultsOpacity {
    if (_controller.value < 0.57) return 0.0;
    if (_controller.value < 0.65) return _interval(0.57, 0.65, Curves.easeOutCubic);
    return 1.0;
  }

  // ── Action panel ──────────────────────────────────────────────────────────
  // The footer hotkey label highlights when the user would press Alt+J, then
  // the action panel slides in. This teaches the full query→action flow that
  // was previously shown on a separate onboarding step.
  //
  // Footer is pressed from 0.70–0.83: during the key-hold approach (0.70–0.72)
  // and the panel rise (0.72–0.79), then released once the panel is fully up.
  bool get _isFooterHotkeyPressed => _controller.value >= 0.70 && _controller.value < 0.83;

  // Panel rise 0.72–0.79 (665ms), hold 0.79–0.95 (1520ms ≈ 1.5s), fade 0.95–0.99.
  double get _actionPanelProgress {
    if (_controller.value < 0.72) return 0.0;
    if (_controller.value < 0.79) return _interval(0.72, 0.79, Curves.easeOutCubic);
    if (_controller.value < 0.95) return 1.0;
    return 1.0 - _interval(0.95, 0.99, Curves.easeInCubic);
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      key: const ValueKey('onboarding-query-concept-demo'),
      animation: _controller,
      builder: (context, child) {
        final showChips = _showFlyingChips;
        final fly = _flyProgress;

        return LayoutBuilder(
          builder: (context, constraints) {
            final w = constraints.maxWidth;
            final h = constraints.maxHeight;

            // ── Chip source positions ───────────────────────────────────────
            // The concept card is centered in the area padded 40px per side.
            // Its inner padding is 20px. Chip widths are estimated from the
            // token text at fontSize 15 w700 on a typical sans-serif font.
            // Vertical position targets the chip text row, which sits roughly
            // at 42% of the demo height (card is vertically centered, chip row
            // is above the label area).
            const estWpmW = 53.0;
            const estInstallW = 84.0;
            const estEverythingW = 110.0;
            const gap = 10.0;
            const estRowW = estWpmW + gap + estInstallW + gap + estEverythingW; // ≈257
            const cardInnerPad = 20.0;
            // Chip row left: card centered in (w-80) with 40px outer offset.
            final chipRowLeft = (w - estRowW - 2 * cardInnerPad) / 2 + cardInnerPad;
            final wpmSrcX = chipRowLeft + estWpmW / 2;
            final instSrcX = chipRowLeft + estWpmW + gap + estInstallW / 2;
            final evSrcX = chipRowLeft + estWpmW + gap + estInstallW + gap + estEverythingW / 2;
            final srcY = h * 0.42;

            // ── Chip destination: query bar text area ───────────────────────
            // Wox window: left-padding 48, top-padding 40 + current woxDY.
            // Query bar: inset left 12 + bar left-padding 8 → text starts at x=68.
            // Query bar center-Y: 40 + woxDY + 12 (bar top inset) + 18 (half ~36px bar height).
            const queryTextX = 48.0 + 12.0 + 8.0; // 68
            final queryBarCenterY = 40.0 + _woxDY + 12.0 + 18.0;
            // All three chips converge toward the query bar start; the slight
            // x-offsets spread them so they don't collapse to a single point.
            const dstBaseX = queryTextX + 16.0;
            final dstY = queryBarCenterY;

            double lerp(double a, double b, double t) => a + (b - a) * t;

            Widget flyChip(double srcX, double srcY2, double dstX, Color color, String text) {
              final cx = lerp(srcX, dstX, fly);
              final cy = lerp(srcY2, dstY, fly);
              final scale = lerp(1.0, 0.40, fly);
              final alpha = _chipAlpha(fly);
              return Positioned(
                left: cx,
                top: cy,
                child: FractionalTranslation(
                  translation: const Offset(-0.5, -0.5),
                  child: IgnorePointer(
                    child: Opacity(
                      opacity: alpha,
                      child: Transform.scale(
                        scale: scale,
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
                          decoration: BoxDecoration(color: color.withValues(alpha: 0.18), borderRadius: BorderRadius.circular(6)),
                          child: Text(text, style: TextStyle(color: color, fontSize: 15, fontWeight: FontWeight.w700)),
                        ),
                      ),
                    ),
                  ),
                ),
              );
            }

            return ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Stack(
                clipBehavior: Clip.hardEdge,
                children: [
                  Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: Platform.isMacOS, showDefaultIcons: false)),

                  // Concept card – slides up and fades out during phase 2.
                  if (_cardOpacity > 0.01)
                    Positioned.fill(
                      child: Center(
                        child: Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 40, vertical: 24),
                          child: Opacity(
                            opacity: _cardOpacity,
                            child: Transform.translate(
                              offset: Offset(0, _cardDY),
                              child: _QueryConceptCard(accent: widget.accent, commandColor: _commandColor, searchTermColor: _searchTermColor, tr: widget.tr),
                            ),
                          ),
                        ),
                      ),
                    ),

                  // Flying chip ghosts – rendered only during crossfade window.
                  if (showChips) ...[
                    flyChip(wpmSrcX, srcY, dstBaseX, widget.accent, 'wpm'),
                    flyChip(instSrcX, srcY, dstBaseX + 44.0, _commandColor, 'install'),
                    flyChip(evSrcX, srcY, dstBaseX + 104.0, _searchTermColor, 'everything'),
                  ],

                  // Wox window – rises in and shows the assembled query + results.
                  if (_woxOpacity > 0.01)
                    Positioned.fill(
                      child: Padding(
                        // Keep a larger gap above the simulated taskbar so the
                        // welcome window does not visually sit on top of it.
                        padding: const EdgeInsets.fromLTRB(48, 34, 52, 56),
                        child: Opacity(
                          opacity: _woxOpacity,
                          child: Transform.translate(
                            offset: Offset(0, _woxDY),
                            child: _ConceptDemoWindow(
                              accent: widget.accent,
                              queryText: _typedQuery,
                              resultsOpacity: _resultsOpacity,
                              actionPanelProgress: _actionPanelProgress,
                              isFooterHotkeyPressed: _isFooterHotkeyPressed,
                              triggerKeywordColor: widget.accent,
                              commandColor: _commandColor,
                              searchTermColor: _searchTermColor,
                              tr: widget.tr,
                            ),
                          ),
                        ),
                      ),
                    ),
                ],
              ),
            );
          },
        );
      },
    );
  }
}

// The card itself: a rounded panel with a title, three annotated tokens, and
// connector lines linking each token to its semantic label below.
class _QueryConceptCard extends StatelessWidget {
  const _QueryConceptCard({required this.accent, required this.commandColor, required this.searchTermColor, required this.tr});

  final Color accent;
  final Color commandColor;
  final Color searchTermColor;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.88),
        border: Border.all(color: getThemeTextColor().withValues(alpha: 0.09)),
        borderRadius: BorderRadius.circular(10),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.14), blurRadius: 24, offset: const Offset(0, 10))],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(tr('onboarding_query_concept_title'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 11, letterSpacing: 0.6, fontWeight: FontWeight.w600)),
          const SizedBox(height: 16),
          // Each token is placed in its own column so the chip and label are
          // always center-aligned with each other regardless of text length.
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              _ConceptToken(token: 'wpm', label: tr('onboarding_query_concept_trigger_keyword'), subLabel: tr('onboarding_query_concept_optional'), color: accent),
              const SizedBox(width: 10),
              _ConceptToken(token: 'install', label: tr('onboarding_query_concept_command'), subLabel: tr('onboarding_query_concept_optional'), color: commandColor),
              const SizedBox(width: 10),
              _ConceptToken(token: 'everything', label: tr('onboarding_query_concept_search_term'), subLabel: tr('onboarding_query_concept_optional'), color: searchTermColor),
            ],
          ),
        ],
      ),
    );
  }
}

// A single token chip with a short vertical connector and the semantic label
// below it. IntrinsicWidth ensures the chip and label share the same center
// regardless of which one is wider.
class _ConceptToken extends StatelessWidget {
  const _ConceptToken({required this.token, required this.label, required this.color, this.subLabel});

  final String token;
  final String label;
  final String? subLabel;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return IntrinsicWidth(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          // Token chip: tinted background keeps the color mark readable on
          // both light and dark themes without relying on full opacity.
          Container(
            width: double.infinity,
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
            decoration: BoxDecoration(color: color.withValues(alpha: 0.14), borderRadius: BorderRadius.circular(6)),
            child: Text(token, textAlign: TextAlign.center, style: TextStyle(color: color, fontSize: 15, fontWeight: FontWeight.w700)),
          ),
          const SizedBox(height: 6),
          // Short vertical line connecting the chip to the label below.
          Container(width: 1, height: 12, color: color.withValues(alpha: 0.35)),
          const SizedBox(height: 6),
          Text(label, textAlign: TextAlign.center, style: TextStyle(color: color, fontSize: 11, fontWeight: FontWeight.w600)),
          if (subLabel != null) ...[const SizedBox(height: 2), Text(subLabel!, textAlign: TextAlign.center, style: TextStyle(color: color.withValues(alpha: 0.65), fontSize: 10))],
        ],
      ),
    );
  }
}

// A stripped-down Wox window rendered only during the query-concept demo.
// Uses a RichText query bar so each token segment keeps its anatomy card
// color: accent for trigger keyword, amber for command, green for search term.
//
// Layout: a Stack that fills its parent, matching the structure of
// WoxDemoWindow. _MiniFooter and _MiniSearchBar are both Positioned widgets
// that must live inside a Stack — placing them in a Column caused the toolbar
// to float in the center of the window instead of anchoring to the bottom.
class _ConceptDemoWindow extends StatelessWidget {
  const _ConceptDemoWindow({
    required this.accent,
    required this.queryText,
    required this.resultsOpacity,
    required this.actionPanelProgress,
    required this.isFooterHotkeyPressed,
    required this.triggerKeywordColor,
    required this.commandColor,
    required this.searchTermColor,
    required this.tr,
  });

  final Color accent;
  // The currently typed portion of 'wpm install everything'; each character
  // is colored by its segment (trigger/command/search-term) inside _buildSpans.
  final String queryText;
  // All three results share one opacity value — they appear simultaneously
  // once the query is fully assembled, so no stagger is needed here.
  final double resultsOpacity;
  // Progress driving the action panel slide-in (0=hidden, 1=fully visible).
  final double actionPanelProgress;
  // Whether the footer Alt+J key should render in its "pressed" highlight state.
  final bool isFooterHotkeyPressed;
  final Color triggerKeywordColor;
  final Color commandColor;
  final Color searchTermColor;
  final String Function(String key) tr;

  List<TextSpan> _buildSpans(double fontSize) {
    const full = 'wpm install everything';
    final n = queryText.length.clamp(0, full.length);
    if (n == 0) return [];
    // Segment boundaries: 'wpm'(0-3) | ' install'(3-11) | ' everything'(11-22).
    // Each character is colored with the right anatomy tint as it appears.
    const seg1 = 3; // end of 'wpm'
    const seg2 = 11; // end of ' install'
    return [
      TextSpan(text: full.substring(0, n.clamp(0, seg1)), style: TextStyle(color: triggerKeywordColor, fontWeight: FontWeight.w700)),
      if (n > seg1) TextSpan(text: full.substring(seg1, n.clamp(seg1, seg2)), style: TextStyle(color: commandColor, fontWeight: FontWeight.w700)),
      if (n > seg2) TextSpan(text: full.substring(seg2, n), style: TextStyle(color: searchTermColor)),
    ];
  }

  Color _conceptMicaSurfaceColor(Color appColor) {
    if (appColor.a >= 0.96) {
      return appColor;
    }

    final isDarkSurface = appColor.computeLuminance() < 0.5;
    final tint = isDarkSurface ? const Color(0xFF202020) : const Color(0xFFF2F2F2);
    final mixed = Color.lerp(appColor.withValues(alpha: 1), tint, 0.18) ?? appColor;
    final alpha = (0.64 + appColor.a * 0.18).clamp(0.64, 0.86).toDouble();
    return mixed.withValues(alpha: alpha);
  }

  @override
  Widget build(BuildContext context) {
    // LayoutBuilder at the root gives us the mini window's actual width so we
    // can compute panelWidth with the same formula used by WoxDemoWindow.
    // A LayoutBuilder placed inside the Stack instead would be treated as an
    // expanded non-Positioned child, causing Positioned widgets inside it to
    // lose their Stack-relative positioning.
    return LayoutBuilder(
      builder: (context, constraints) {
        final metrics = WoxInterfaceSizeUtil.instance.current;
        final woxTheme = WoxThemeUtil.instance.currentTheme.value;
        // Vertical layout mirrors WoxDemoWindow: query bar at top, results below,
        // toolbar at the absolute bottom via _MiniFooter's own Positioned.
        final queryTop = 12.0;
        final resultTop = queryTop + metrics.queryBoxBaseHeight + 10.0;
        final footerHeight = WoxThemeUtil.instance.getToolbarHeight();
        // Panel width matches WoxDemoWindow's formula so the overlay looks
        // identical to the real launcher's action-panel overlay.
        final panelWidth = (constraints.maxWidth * 0.42).clamp(250.0, 320.0);

        final effectiveBg = _conceptMicaSurfaceColor(getThemeBackgroundColor());
        final effectiveBorderColor = getThemeTextColor().withValues(alpha: 0.10);

        return ClipRRect(
          borderRadius: BorderRadius.circular(8),
          child: BackdropFilter(
            filter: ui.ImageFilter.blur(sigmaX: 20, sigmaY: 20),
            // Stack fills the parent so every Positioned child (query bar, footer)
            // uses the same coordinate space as the real WoxDemoWindow.
            child: Stack(
              fit: StackFit.expand,
              children: [
                Positioned.fill(
                  child: DecoratedBox(decoration: BoxDecoration(color: effectiveBg, border: Border.all(color: effectiveBorderColor), borderRadius: BorderRadius.circular(8))),
                ),
                // Accent tint stays subtle so the glass blur remains legible.
                Positioned.fill(
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                        colors: [Colors.black.withValues(alpha: 0.02), accent.withValues(alpha: 0.024), Colors.black.withValues(alpha: 0.14)],
                        stops: const [0.0, 0.38, 1.0],
                      ),
                    ),
                  ),
                ),
                // Query bar – RichText keeps each token in its anatomy color.
                Positioned(
                  left: 12,
                  right: 12,
                  top: queryTop,
                  height: metrics.queryBoxBaseHeight,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8),
                    decoration: BoxDecoration(color: woxTheme.queryBoxBackgroundColorParsed, borderRadius: BorderRadius.circular(woxTheme.queryBoxBorderRadius.toDouble())),
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: RichText(
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        text: TextSpan(style: TextStyle(fontSize: metrics.queryBoxFontSize), children: _buildSpans(metrics.queryBoxFontSize)),
                      ),
                    ),
                  ),
                ),
                // Three plugin-store results, all fading in together.
                if (resultsOpacity > 0.01)
                  Positioned(
                    left: 12,
                    right: 12,
                    top: resultTop,
                    bottom: footerHeight,
                    child: Opacity(
                      opacity: resultsOpacity,
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          _MiniResultRow(
                            title: 'Everything',
                            subtitle: tr('onboarding_query_concept_result1_subtitle'),
                            icon: Icon(Icons.search_rounded, color: accent, size: 23),
                            selected: true,
                          ),
                          _MiniResultRow(
                            title: 'Everything (portable)',
                            subtitle: tr('onboarding_query_concept_result2_subtitle'),
                            icon: Icon(Icons.search_outlined, color: accent.withValues(alpha: 0.65), size: 23),
                          ),
                          _MiniResultRow(
                            title: 'Everything-cli',
                            subtitle: tr('onboarding_query_concept_result3_subtitle'),
                            icon: const Icon(Icons.terminal_rounded, color: Color(0xFF94A3B8), size: 23),
                          ),
                        ],
                      ),
                    ),
                  ),
                // Toolbar – _MiniFooter is a Positioned widget that anchors itself
                // to bottom:0, left:0, right:0 inside the Stack.
                _MiniFooter(accent: accent, hotkey: _demoActionPanelHotkey(), isPressed: isFooterHotkeyPressed),
                // Action panel – mirrors WoxDemoWindow's own panel overlay:
                // Positioned to the bottom-right corner above the toolbar,
                // with the same slide+scale entrance animation.
                if (actionPanelProgress > 0.01)
                  Positioned(
                    right: 16,
                    bottom: footerHeight + 12,
                    width: panelWidth,
                    child: Opacity(
                      opacity: actionPanelProgress,
                      child: Transform.translate(
                        offset: Offset(18 * (1 - actionPanelProgress), 10 * (1 - actionPanelProgress)),
                        child: Transform.scale(alignment: Alignment.bottomRight, scale: 0.96 + (0.04 * actionPanelProgress), child: WoxDemoActionPanel(accent: accent, tr: tr)),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        );
      },
    );
  }
}
