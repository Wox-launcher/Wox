part of 'wox_demo.dart';

// Carries per-demo color overrides used by the theme-install step to preview
// the applied theme appearance without touching the global WoxThemeUtil state.
class _DemoThemeData {
  const _DemoThemeData({
    required this.background,
    required this.accent,
    required this.queryBarBackground,
    required this.queryBarText,
    required this.resultTitleColor,
    required this.resultSubtitleColor,
    required this.resultActiveBackground,
    required this.resultActiveTitleColor,
    required this.resultActiveSubtitleColor,
    required this.tailColor,
    required this.activeTailColor,
    required this.textColor,
  });

  final Color background;
  final Color accent;
  final Color queryBarBackground;
  final Color queryBarText;
  final Color resultTitleColor;
  final Color resultSubtitleColor;
  final Color resultActiveBackground;
  final Color resultActiveTitleColor;
  final Color resultActiveSubtitleColor;
  final Color tailColor;
  final Color activeTailColor;
  // Used for borders, toolbar labels, and other neutral chrome.
  final Color textColor;
}

// InheritedWidget that propagates _DemoThemeData through the demo widget tree.
// Descendant widgets (_MiniSearchBar, _MiniResultRow, _MiniFooter, etc.)
// read from this widget and substitute its colors instead of WoxThemeUtil,
// allowing the theme-install demo to show the "after" appearance without
// altering any global theme state.
class _InheritedDemoTheme extends InheritedWidget {
  const _InheritedDemoTheme({required this.data, required super.child});

  final _DemoThemeData data;

  static _DemoThemeData? of(BuildContext context) {
    return context.dependOnInheritedWidgetOfExactType<_InheritedDemoTheme>()?.data;
  }

  @override
  bool updateShouldNotify(_InheritedDemoTheme old) => old.data != data;
}

class WoxDemoResult {
  const WoxDemoResult({required this.title, required this.icon, this.subtitle, this.tail, this.tailColor, this.selected = false});

  final String title;
  final String? subtitle;
  final Widget icon;
  final bool selected;
  final String? tail;
  final Color? tailColor;
}

class WoxDemoWindow extends StatelessWidget {
  const WoxDemoWindow({
    super.key,
    required this.accent,
    required this.query,
    this.results = const [
      WoxDemoResult(title: 'Open Wox Settings', subtitle: r'C:\Users\qianl\AppData\Roaming\Wox', icon: WoxDemoLogoMark(), tail: '2 day ago', selected: true),
      WoxDemoResult(
        title: 'Open URL settings',
        subtitle: 'Configure URL open rules and browser targets',
        icon: Icon(Icons.link_rounded, color: Color(0xFF38BDF8), size: 24),
        tail: 'Settings',
      ),
      WoxDemoResult(title: 'Open WebView settings', subtitle: 'Inspect and tune embedded preview behavior', icon: Icon(Icons.language_rounded, color: Color(0xFF60A5FA), size: 24)),
      WoxDemoResult(
        title: 'Open Update settings',
        subtitle: 'Check update channel and release status',
        icon: Icon(Icons.sync_rounded, color: Color(0xFF3B82F6), size: 24),
        tail: 'Update',
      ),
    ],
    this.queryAccessory,
    this.footerHotkey,
    this.isFooterHotkeyPressed = false,
    this.actionPanel,
    this.actionPanelProgress = 0,
    this.opaqueBackground = false,
    this.showQueryBox = true,
    this.showToolbar = true,
  });

  final Color accent;
  final String query;
  final List<WoxDemoResult> results;
  final Widget? queryAccessory;
  final String? footerHotkey;
  final bool isFooterHotkeyPressed;
  final Widget? actionPanel;
  final double actionPanelProgress;
  final bool opaqueBackground;
  final bool showQueryBox;
  final bool showToolbar;

  // Keep demo windows aligned with the theme editor preview by using a similar
  // mica-like surface tint over a blurred backdrop when transparency is used.
  Color _micaSurfaceColor(Color appColor, {required bool forceOpaque}) {
    if (forceOpaque || appColor.a >= 0.96) {
      return appColor.withValues(alpha: 1);
    }

    final isDarkSurface = appColor.computeLuminance() < 0.5;
    final tint = isDarkSurface ? const Color(0xFF202020) : const Color(0xFFF2F2F2);
    final mixed = Color.lerp(appColor.withValues(alpha: 1), tint, 0.18) ?? appColor;
    final alpha = (0.64 + appColor.a * 0.18).clamp(0.64, 0.86).toDouble();
    return mixed.withValues(alpha: alpha);
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final metrics = WoxInterfaceSizeUtil.instance.current;
        // Bug fix: the previous 320px cap was not enough for 4-result demos with
        // a toolbar at normal/comfortable density (required up to ~337px). Raised
        // to 400 so all densities fit without clipping the last result row.
        final maxPreviewHeight = constraints.maxHeight.clamp(0.0, 400.0).toDouble();
        final previewWidth = constraints.maxWidth;
        // Feature refinement: the shared demo now defaults to the full Wox
        // launcher chrome. Most previews should show both the query box and the
        // toolbar; special entry points such as Tray Query opt out explicitly so
        // callers do not have to remember to add chrome for every normal demo.
        final hasFooter = showToolbar;
        final effectiveFooterHotkey = footerHotkey ?? _demoActionPanelHotkey();
        final footerHeight = hasFooter ? WoxThemeUtil.instance.getToolbarHeight() : 0.0;
        final actionPanelWidth = (previewWidth * 0.42).clamp(250.0, 320.0).toDouble();
        final queryTop = 12.0;
        final resultTop = showQueryBox ? queryTop + metrics.queryBoxBaseHeight + 10 : queryTop;
        final bottomPadding = hasFooter ? 0.0 : 12.0;
        final maxResultListHeight = (maxPreviewHeight - resultTop - footerHeight - bottomPadding).clamp(0.0, double.infinity).toDouble();
        final resultListHeight = (results.length * WoxThemeUtil.instance.getResultItemHeight()).clamp(0.0, maxResultListHeight).toDouble();
        // Bug fix: toolbar-enabled previews should shrink to their visible
        // results like the real launcher. The old full-height result list left
        // a dead band under short demos, so the footer is now placed directly
        // after the rendered rows unless the parent height forces clipping.
        final shouldShrinkToContent = hasFooter || !showQueryBox;
        final previewHeight = shouldShrinkToContent ? (resultTop + resultListHeight + footerHeight + bottomPadding).clamp(0.0, maxPreviewHeight).toDouble() : maxPreviewHeight;

        return Center(
          child: SizedBox(
            width: previewWidth,
            height: previewHeight,
            child: Builder(
              builder: (innerCtx) {
                // Theme override: the theme-install step wraps this window with
                // _InheritedDemoTheme to show the applied theme colors. We read
                // it here so the container background and accent gradient also
                // switch without needing an extra widget layer.
                final demoTheme = _InheritedDemoTheme.of(innerCtx);
                final effectiveAccent = demoTheme?.accent ?? accent;
                final baseBg = demoTheme?.background ?? getThemeBackgroundColor();
                final effectiveBg = _micaSurfaceColor(baseBg, forceOpaque: opaqueBackground);
                final effectiveBorderColor = (demoTheme?.textColor ?? getThemeTextColor()).withValues(alpha: 0.10);
                return ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: BackdropFilter(
                    filter: ui.ImageFilter.blur(sigmaX: 20, sigmaY: 20),
                    child: Stack(
                      children: [
                        Positioned.fill(
                          child: DecoratedBox(
                            decoration: BoxDecoration(color: effectiveBg, border: Border.all(color: effectiveBorderColor), borderRadius: BorderRadius.circular(8)),
                          ),
                        ),
                        Positioned.fill(
                          child: DecoratedBox(
                            decoration: BoxDecoration(
                              gradient: LinearGradient(
                                begin: Alignment.topLeft,
                                end: Alignment.bottomRight,
                                colors: [Colors.black.withValues(alpha: 0.02), effectiveAccent.withValues(alpha: 0.024), Colors.black.withValues(alpha: 0.14)],
                                stops: const [0.0, 0.38, 1.0],
                              ),
                            ),
                          ),
                        ),
                        if (showQueryBox) _MiniSearchBar(query: query, trailing: queryAccessory),
                        // Feature refinement: the mock launcher now follows the
                        // production query/result vertical rhythm instead of using
                        // compact onboarding-only row spacing. This keeps fonts,
                        // padding, and density comparable to the real Wox window.
                        Positioned(
                          left: 12,
                          right: 12,
                          top: resultTop,
                          bottom: shouldShrinkToContent ? null : bottomPadding,
                          height: shouldShrinkToContent ? resultListHeight : null,
                          child: _MiniResultList(results: results),
                        ),
                        if (hasFooter) _MiniFooter(accent: effectiveAccent, hotkey: effectiveFooterHotkey, isPressed: isFooterHotkeyPressed),
                        if (actionPanel != null)
                          Positioned(
                            right: 16,
                            bottom: footerHeight + 12,
                            width: actionPanelWidth,
                            child: Opacity(
                              opacity: actionPanelProgress,
                              child: Transform.translate(
                                offset: Offset(18 * (1 - actionPanelProgress), 10 * (1 - actionPanelProgress)),
                                child: Transform.scale(alignment: Alignment.bottomRight, scale: 0.96 + (0.04 * actionPanelProgress), child: actionPanel),
                              ),
                            ),
                          ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        );
      },
    );
  }
}

class _MiniSearchBar extends StatelessWidget {
  const _MiniSearchBar({required this.query, this.trailing});

  final String query;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    // Theme override: use demo-provided colors when a _InheritedDemoTheme
    // ancestor is present (e.g. during the theme-install applied state).
    final demoTheme = _InheritedDemoTheme.of(context);
    final barBackground = demoTheme?.queryBarBackground ?? woxTheme.queryBoxBackgroundColorParsed;
    final barText = demoTheme?.queryBarText ?? woxTheme.queryBoxFontColorParsed;

    return Positioned(
      left: 12,
      right: 12,
      top: 12,
      child: Container(
        height: metrics.queryBoxBaseHeight,
        // Demo fix: use symmetric horizontal padding and let the Row center text
        // vertically. The real launcher uses asymmetric QUERY_BOX_CONTENT_PADDING
        // values tuned for TextField cursor positioning, but those values clip
        // the top ascenders of large glyphs (e.g. 'g') when applied to a plain
        // Text widget in the demo.
        padding: const EdgeInsets.symmetric(horizontal: 8),
        decoration: BoxDecoration(color: barBackground, borderRadius: BorderRadius.circular(woxTheme.queryBoxBorderRadius.toDouble())),
        child: Row(
          children: [
            Expanded(child: Text(query, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: barText, fontSize: metrics.queryBoxFontSize))),
            // Feature refinement: the search row has no implicit Glance item.
            // Callers opt in only after the Glance step has introduced and
            // enabled it, which keeps earlier examples from leaking later
            // onboarding concepts.
            if (trailing != null) ...[const SizedBox(width: 10), trailing!],
          ],
        ),
      ),
    );
  }
}

class _MiniResultList extends StatelessWidget {
  const _MiniResultList({required this.results});

  final List<WoxDemoResult> results;

  @override
  Widget build(BuildContext context) {
    // Bug fix: the preview rows now use the real Wox result height, and the
    // Action Panel demo also reserves toolbar space. A fixed Column can exceed
    // the remaining preview height, while the production result view is a
    // clipped list. Using a non-scrollable ListView preserves real row sizing
    // and clips overflow instead of rendering Flutter's overflow warning.
    return ListView.builder(
      padding: EdgeInsets.zero,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: results.length,
      itemBuilder: (context, index) {
        final result = results[index];
        return _MiniResultRow(title: result.title, subtitle: result.subtitle, icon: result.icon, selected: result.selected, tail: result.tail, tailColor: result.tailColor);
      },
    );
  }
}

class _MiniResultRow extends StatelessWidget {
  const _MiniResultRow({required this.title, required this.icon, this.subtitle, this.selected = false, this.tail, this.tailColor});

  final String title;
  final String? subtitle;
  final Widget icon;
  final bool selected;
  final String? tail;
  final Color? tailColor;

  @override
  Widget build(BuildContext context) {
    // Bug fix: rows must keep launcher-like density even when a preview passes
    // fewer entries. The previous Expanded row made two-result demos stretch
    // into oversized blocks, so each mock result now has a stable row height.
    // Feature refinement: rows now also model Wox's subtitle and tail affordance
    // so the shared preview shows file paths, result descriptions, and status
    // chips instead of flattening every result into a single title line.
    // Feature refinement: result row metrics now come from the production
    // launcher sizing/theme utilities. The onboarding-specific padding and
    // bold text made the preview look unlike Wox, while reusing these values
    // keeps the example aligned with real query results across densities.
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    // Theme override: substitute demo colors when the window is wrapped with
    // _InheritedDemoTheme (e.g. while previewing the applied Ocean Dark theme).
    final demoTheme = _InheritedDemoTheme.of(context);
    final hasSubtitle = subtitle != null && subtitle!.isNotEmpty;
    final borderRadius = woxTheme.resultItemBorderRadius > 0 ? BorderRadius.circular(woxTheme.resultItemBorderRadius.toDouble()) : BorderRadius.zero;
    final maxBorderWidth =
        (woxTheme.resultItemActiveBorderLeftWidth > woxTheme.resultItemBorderLeftWidth ? woxTheme.resultItemActiveBorderLeftWidth : woxTheme.resultItemBorderLeftWidth).toDouble();
    final actualBorderWidth = selected ? woxTheme.resultItemActiveBorderLeftWidth.toDouble() : woxTheme.resultItemBorderLeftWidth.toDouble();
    final titleColor =
        demoTheme != null
            ? (selected ? demoTheme.resultActiveTitleColor : demoTheme.resultTitleColor)
            : (selected ? woxTheme.resultItemActiveTitleColorParsed : woxTheme.resultItemTitleColorParsed);
    final subtitleColor =
        demoTheme != null
            ? (selected ? demoTheme.resultActiveSubtitleColor : demoTheme.resultSubtitleColor)
            : (selected ? woxTheme.resultItemActiveSubTitleColorParsed : woxTheme.resultItemSubTitleColorParsed);
    final themeTailColor =
        demoTheme != null
            ? (selected ? demoTheme.activeTailColor : demoTheme.tailColor)
            : (selected ? woxTheme.resultItemActiveTailTextColorParsed : woxTheme.resultItemTailTextColorParsed);
    // Semantic tail override: permission demos need the same warning color as
    // the real onboarding cards. Callers that do not pass tailColor keep the
    // production-theme tail color, so existing demos remain unchanged.
    final effectiveTailColor = tailColor ?? themeTailColor;
    final activeBackground = demoTheme?.resultActiveBackground ?? woxTheme.resultItemActiveBackgroundColorParsed;

    Widget content = Container(
      decoration: BoxDecoration(color: selected ? activeBackground : Colors.transparent),
      padding: EdgeInsets.only(
        top: metrics.scaledSpacing(woxTheme.resultItemPaddingTop.toDouble()),
        right: metrics.scaledSpacing(woxTheme.resultItemPaddingRight.toDouble()),
        bottom: metrics.scaledSpacing(woxTheme.resultItemPaddingBottom.toDouble()),
        left: metrics.scaledSpacing(woxTheme.resultItemPaddingLeft.toDouble() + maxBorderWidth),
      ),
      child: Row(
        children: [
          Padding(
            padding: EdgeInsets.only(left: metrics.resultItemIconPaddingLeft, right: metrics.resultItemIconPaddingRight),
            child: SizedBox(width: metrics.resultIconSize, height: metrics.resultIconSize, child: FittedBox(fit: BoxFit.contain, child: icon)),
          ),
          Expanded(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: titleColor, fontSize: metrics.resultTitleFontSize)),
                if (hasSubtitle)
                  Padding(
                    padding: EdgeInsets.only(top: metrics.resultItemSubtitlePaddingTop),
                    child: Text(subtitle!, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: subtitleColor, fontSize: metrics.resultSubtitleFontSize)),
                  ),
              ],
            ),
          ),
          if (tail != null && tail!.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(left: metrics.resultItemTailPaddingLeft, right: metrics.resultItemTailPaddingRight),
              child: Padding(
                padding: EdgeInsets.only(left: metrics.resultItemTailItemPaddingLeft),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 132),
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      color: Colors.transparent,
                      border: Border.all(color: effectiveTailColor.withValues(alpha: selected ? 0.34 : 0.2)),
                      borderRadius: BorderRadius.circular(999),
                    ),
                    child: Padding(
                      padding: EdgeInsets.symmetric(horizontal: metrics.resultItemTextTailHPadding, vertical: metrics.resultItemTextTailVPadding),
                      child: Text(tail!, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: effectiveTailColor, fontSize: metrics.tailHotkeyFontSize)),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );

    if (borderRadius != BorderRadius.zero) {
      content = ClipRRect(borderRadius: borderRadius, child: content);
    }

    if (actualBorderWidth > 0) {
      content = Stack(
        children: [
          content,
          Positioned(
            left: 0,
            top: 0,
            bottom: 0,
            child: Container(
              width: actualBorderWidth,
              decoration: BoxDecoration(
                color: activeBackground,
                borderRadius: borderRadius != BorderRadius.zero ? BorderRadius.only(topLeft: borderRadius.topLeft, bottomLeft: borderRadius.bottomLeft) : BorderRadius.zero,
              ),
            ),
          ),
        ],
      );
    }

    return SizedBox(height: WoxThemeUtil.instance.getResultItemHeight(), child: content);
  }
}

class _MiniFooter extends StatelessWidget {
  const _MiniFooter({required this.accent, required this.hotkey, required this.isPressed});

  final Color accent;
  final String hotkey;
  final bool isPressed;

  @override
  Widget build(BuildContext context) {
    final keyLabels = hotkey.split('+');
    final metrics = WoxInterfaceSizeUtil.instance.current;
    // Organization cleanup: the toolbar belongs to the shared demo window, not
    // to the Action Panel overlay. Keeping it here makes the file layout match
    // the rendered hierarchy and avoids another action-panel-like file.
    final demoTheme = _InheritedDemoTheme.of(context);
    final textColor = demoTheme?.textColor ?? getThemeTextColor();

    return Positioned(
      left: 0,
      right: 0,
      bottom: 0,
      child: Container(
        height: metrics.toolbarHeight,
        padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(12)),
        decoration: BoxDecoration(color: textColor.withValues(alpha: 0.035), border: Border(top: BorderSide(color: textColor.withValues(alpha: 0.07)))),
        child: FittedBox(
          alignment: Alignment.centerRight,
          fit: BoxFit.scaleDown,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('Execute', style: TextStyle(color: textColor, fontSize: metrics.toolbarFontSize)),
              SizedBox(width: metrics.toolbarActionNameHotkeySpacing),
              _MiniShortcutKey(label: 'Enter', accent: accent, textColor: textColor, active: false),
              SizedBox(width: metrics.toolbarActionSpacing),
              Text('More Actions', style: TextStyle(color: isPressed ? accent : textColor, fontSize: metrics.toolbarFontSize)),
              SizedBox(width: metrics.toolbarActionNameHotkeySpacing),
              for (var index = 0; index < keyLabels.length; index++) ...[
                _MiniShortcutKey(label: keyLabels[index], accent: accent, textColor: textColor, active: isPressed),
                if (index < keyLabels.length - 1) SizedBox(width: metrics.toolbarHotkeyKeySpacing),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _MiniShortcutKey extends StatelessWidget {
  const _MiniShortcutKey({required this.label, required this.accent, required this.textColor, required this.active});

  final String label;
  final Color accent;
  final Color textColor;
  final bool active;

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;

    return AnimatedContainer(
      duration: const Duration(milliseconds: 140),
      height: metrics.scaledSpacing(22),
      constraints: BoxConstraints(minWidth: metrics.scaledSpacing(28)),
      padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(7)),
      decoration: BoxDecoration(
        color: active ? accent.withValues(alpha: 0.20) : Colors.transparent,
        border: Border.all(color: active ? accent : textColor.withValues(alpha: 0.66)),
        borderRadius: BorderRadius.circular(4),
      ),
      alignment: Alignment.center,
      child: Text(
        label,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: active ? accent : textColor, fontSize: metrics.tailHotkeyFontSize, fontWeight: FontWeight.w500),
      ),
    );
  }
}
