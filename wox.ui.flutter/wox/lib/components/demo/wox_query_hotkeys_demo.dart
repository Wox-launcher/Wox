part of 'wox_demo.dart';

// The shared settings-page preview keeps the original showcase, while the
// Query Hotkey preset pills request a single focused demo for each mode.
enum WoxQueryHotkeysDemoMode { showcase, normal, webPanel, silent, custom }

// Two-example showcase for Query Hotkeys.
//
// Phase timeline (total 9200 ms, looping):
//
//   Example 1 – "Ctrl+Shift+G → github repo" (0.00–0.47):
//     Demonstrates a basic hotkey that opens Wox with a preset query.
//     0.00–0.09  Static hint card; hotkey badge fades in.
//     0.09–0.15  Hotkey overlay rises in.
//     0.15–0.25  Hotkey badge held (key visually pressed at 0.15–0.21).
//     0.25–0.31  Hotkey overlay fades out.
//     0.20–0.29  Wox window rises in.
//     0.29–0.47  Wox window held (results fully visible).
//
//   Crossfade (0.43–0.55):
//     Example 1 fades out while example 2 fades in.
//
//   Example 2 – "Ctrl+Shift+I → webview instagram" (0.55–0.94):
//     Demonstrates hiding the query box and toolbar so the entire Wox
//     window becomes a borderless embedded webpage (webview plugin).
//     0.55–0.63  Hotkey overlay rises in.
//     0.63–0.72  Hotkey badge held (key visually pressed at 0.63–0.70).
//     0.72–0.79  Hotkey overlay fades out.
//     0.68–0.80  Instagram webview window rises in.
//     0.80–0.94  Webview window held fully visible.
//
//   Pause (0.94–1.00): brief gap before the loop restarts.
class WoxQueryHotkeysDemo extends StatefulWidget {
  const WoxQueryHotkeysDemo({super.key, required this.accent, required this.tr, this.mode = WoxQueryHotkeysDemoMode.showcase});

  final Color accent;
  final String Function(String key) tr;
  final WoxQueryHotkeysDemoMode mode;

  @override
  State<WoxQueryHotkeysDemo> createState() => _WoxQueryHotkeysDemoState();
}

class _WoxQueryHotkeysDemoState extends State<WoxQueryHotkeysDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: _durationForMode(widget.mode))..repeat();
  }

  @override
  void didUpdateWidget(covariant WoxQueryHotkeysDemo oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.mode == widget.mode) {
      return;
    }

    _controller
      ..duration = _durationForMode(widget.mode)
      ..reset()
      ..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  double _interval(double start, double end, Curve curve) {
    final value = ((_controller.value - start) / (end - start)).clamp(0.0, 1.0).toDouble();
    return curve.transform(value);
  }

  Duration _durationForMode(WoxQueryHotkeysDemoMode mode) {
    switch (mode) {
      case WoxQueryHotkeysDemoMode.showcase:
        // Extended from 4600 ms to 9200 ms to fit both examples without
        // compressing their individual pacing.
        return const Duration(milliseconds: 9200);
      case WoxQueryHotkeysDemoMode.normal:
      case WoxQueryHotkeysDemoMode.webPanel:
      case WoxQueryHotkeysDemoMode.silent:
      case WoxQueryHotkeysDemoMode.custom:
        return const Duration(milliseconds: 4600);
    }
  }

  // Single-mode previews reuse the same pacing so each preset popover stays
  // quick to scan instead of looping through the full showcase sequence.
  double _singleShortcutProgress() {
    if (_controller.value < 0.10) return 0;
    if (_controller.value < 0.16) return _interval(0.10, 0.16, Curves.easeOutCubic);
    if (_controller.value < 0.26) return 1;
    if (_controller.value < 0.32) return 1 - _interval(0.26, 0.32, Curves.easeInCubic);
    return 0;
  }

  double _singleContentProgress({double start = 0.22, double end = 0.34}) {
    if (_controller.value < start) return 0;
    if (_controller.value < end) return _interval(start, end, Curves.easeOutCubic);
    return 1;
  }

  bool _isSingleShortcutPressed() => _controller.value >= 0.16 && _controller.value <= 0.22;

  double get _singleSceneOpacity {
    if (_controller.value < 0.04) return _interval(0.00, 0.04, Curves.easeOutCubic);
    if (_controller.value < 0.94) return 1.0;
    return 1.0 - _interval(0.94, 1.00, Curves.easeInCubic);
  }

  // ── Example 1 animations ──────────────────────────────────────────────────

  double _shortcutProgress1() {
    if (_controller.value < 0.09) return 0;
    if (_controller.value < 0.15) return _interval(0.09, 0.15, Curves.easeOutCubic);
    if (_controller.value < 0.25) return 1;
    if (_controller.value < 0.31) return 1 - _interval(0.25, 0.31, Curves.easeInCubic);
    return 0;
  }

  double _windowProgress1() {
    if (_controller.value < 0.20) return 0;
    if (_controller.value < 0.29) return _interval(0.20, 0.29, Curves.easeOutCubic);
    return 1;
  }

  bool _isShortcutPressed1() => _controller.value >= 0.15 && _controller.value <= 0.21;

  // ── Crossfade between examples ────────────────────────────────────────────

  // Example 1 starts fading at 0.43 (while still holding) and is gone by 0.52.
  double get _example1Opacity {
    if (_controller.value < 0.43) return 1.0;
    if (_controller.value < 0.52) return 1.0 - _interval(0.43, 0.52, Curves.easeInCubic);
    return 0.0;
  }

  // Example 2 fades in from 0.50 to 0.60, holds through 0.94, then fades out.
  double get _example2Opacity {
    if (_controller.value < 0.50) return 0.0;
    if (_controller.value < 0.60) return _interval(0.50, 0.60, Curves.easeOutCubic);
    if (_controller.value < 0.94) return 1.0;
    return 1.0 - _interval(0.94, 1.00, Curves.easeInCubic);
  }

  // ── Example 2 animations ──────────────────────────────────────────────────

  double _shortcutProgress2() {
    if (_controller.value < 0.55) return 0;
    if (_controller.value < 0.63) return _interval(0.55, 0.63, Curves.easeOutCubic);
    if (_controller.value < 0.72) return 1;
    if (_controller.value < 0.79) return 1 - _interval(0.72, 0.79, Curves.easeInCubic);
    return 0;
  }

  double _windowProgress2() {
    if (_controller.value < 0.68) return 0;
    if (_controller.value < 0.80) return _interval(0.68, 0.80, Curves.easeOutCubic);
    return 1.0;
  }

  bool _isShortcutPressed2() => _controller.value >= 0.63 && _controller.value <= 0.70;

  Widget _buildSingleModeDemo({required String hotkey, required String query, required Widget Function(String hotkeyLabel, double contentProgress) buildContent}) {
    final hotkeyLabel = _formatDemoHotkey('', fallback: hotkey);

    return AnimatedBuilder(
      key: ValueKey('onboarding-query-hotkeys-demo-${widget.mode.name}'),
      animation: _controller,
      builder: (context, child) {
        final shortcutProgress = _singleShortcutProgress();
        final contentProgress = _singleContentProgress();

        return Opacity(
          opacity: _singleSceneOpacity,
          child: ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: Stack(
              children: [
                Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: Platform.isMacOS, showDefaultIcons: false)),
                Positioned.fill(
                  child: Padding(
                    padding: _demoDesktopHintContentPadding(),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        WoxDemoHintCard(accent: widget.accent, icon: Icons.keyboard_command_key, title: widget.tr('onboarding_query_hotkeys_title'), from: hotkeyLabel, to: query),
                        const SizedBox(height: 12),
                        Expanded(
                          child: Stack(
                            children: [
                              Positioned.fill(
                                child: Opacity(
                                  opacity: shortcutProgress,
                                  child: Transform.translate(
                                    offset: Offset(0, 8 * (1 - shortcutProgress)),
                                    child: _HotkeyPressOverlay(hotkey: hotkeyLabel, accent: widget.accent, pressed: _isSingleShortcutPressed()),
                                  ),
                                ),
                              ),
                              if (contentProgress > 0.01) buildContent(hotkeyLabel, contentProgress),
                            ],
                          ),
                        ),
                      ],
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

  Widget _buildNormalDemo() {
    return _buildSingleModeDemo(
      hotkey: Platform.isMacOS ? 'cmd+shift+g' : 'ctrl+shift+g',
      query: 'github repo',
      buildContent: (hotkeyLabel, contentProgress) {
        return Positioned.fill(
          child: Transform.translate(
            offset: Offset(0, 20 * (1 - contentProgress)),
            child: Transform.scale(
              scale: 0.95 + (0.05 * contentProgress),
              child: WoxDemoWindow(
                accent: widget.accent,
                query: 'github repo',
                opaqueBackground: true,
                footerHotkey: _demoActionPanelHotkey(),
                results: [
                  WoxDemoResult(
                    title: 'Wox repository',
                    subtitle: 'Open Wox-launcher/Wox on GitHub',
                    icon: const Icon(Icons.code_rounded, color: Colors.white, size: 23),
                    selected: true,
                    tail: hotkeyLabel,
                  ),
                  WoxDemoResult(
                    title: widget.tr('onboarding_query_hotkeys_title'),
                    subtitle: widget.tr('onboarding_query_hotkeys_body'),
                    icon: Icon(Icons.bolt_outlined, color: widget.accent, size: 23),
                    tail: widget.tr('ui_query_hotkeys'),
                  ),
                  const WoxDemoResult(title: 'Issues', subtitle: 'github repo issues', icon: Icon(Icons.bug_report_outlined, color: Color(0xFFFACC15), size: 23), tail: 'GitHub'),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildWebPanelDemo() {
    return _buildSingleModeDemo(
      hotkey: Platform.isMacOS ? 'cmd+shift+i' : 'ctrl+shift+i',
      query: 'webview instagram',
      buildContent: (hotkeyLabel, contentProgress) {
        return Positioned(
          top: 0,
          bottom: 0,
          left: 0,
          right: 0,
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 340),
              child: Transform.translate(
                offset: Offset(0, 20 * (1 - contentProgress)),
                child: Transform.scale(scale: 0.95 + (0.05 * contentProgress), child: const _InstagramWebviewWindow()),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildSilentDemo() {
    return _buildSingleModeDemo(
      hotkey: Platform.isMacOS ? 'cmd+shift+s' : 'ctrl+shift+s',
      query: 'copy github repo',
      buildContent: (hotkeyLabel, contentProgress) {
        return Positioned(
          left: 0,
          right: 0,
          bottom: Platform.isMacOS ? 36 : 54,
          child: Center(
            child: Transform.translate(
              offset: Offset(0, 18 * (1 - contentProgress)),
              child: Transform.scale(scale: 0.96 + (0.04 * contentProgress), child: _SilentExecutionToast(accent: widget.accent, hotkey: hotkeyLabel)),
            ),
          ),
        );
      },
    );
  }

  Widget _buildCustomDemo() {
    return _buildSingleModeDemo(
      hotkey: Platform.isMacOS ? 'cmd+shift+d' : 'ctrl+shift+d',
      query: 'daily dashboard',
      buildContent: (hotkeyLabel, contentProgress) {
        return Stack(
          children: [
            Positioned(
              left: 18,
              bottom: Platform.isMacOS ? 30 : 48,
              child: Opacity(
                opacity: contentProgress,
                child: Transform.translate(offset: Offset(-10 * (1 - contentProgress), 10 * (1 - contentProgress)), child: _CustomModeSummaryBadge(accent: widget.accent)),
              ),
            ),
            Positioned(
              top: 6,
              right: 16,
              width: 320,
              child: Transform.translate(
                offset: Offset(18 * (1 - contentProgress), 20 * (1 - contentProgress)),
                child: Transform.scale(
                  alignment: Alignment.topRight,
                  scale: 0.95 + (0.05 * contentProgress),
                  child: WoxDemoWindow(
                    accent: widget.accent,
                    query: 'daily dashboard',
                    opaqueBackground: true,
                    showQueryBox: false,
                    footerHotkey: hotkeyLabel,
                    results: const [
                      WoxDemoResult(
                        title: 'Today',
                        subtitle: 'Agenda, tasks, and focus notes',
                        icon: Icon(Icons.today_rounded, color: Color(0xFF60A5FA), size: 23),
                        selected: true,
                        tail: 'Pinned',
                      ),
                      WoxDemoResult(
                        title: 'Standup.md',
                        subtitle: 'Open the daily standup note',
                        icon: Icon(Icons.description_outlined, color: Color(0xFFFACC15), size: 23),
                        tail: 'Notes',
                      ),
                      WoxDemoResult(title: 'Calendar', subtitle: 'Next event at 10:30', icon: Icon(Icons.event_outlined, color: Color(0xFF34D399), size: 23), tail: 'Work'),
                    ],
                  ),
                ),
              ),
            ),
          ],
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    switch (widget.mode) {
      case WoxQueryHotkeysDemoMode.normal:
        return _buildNormalDemo();
      case WoxQueryHotkeysDemoMode.webPanel:
        return _buildWebPanelDemo();
      case WoxQueryHotkeysDemoMode.silent:
        return _buildSilentDemo();
      case WoxQueryHotkeysDemoMode.custom:
        return _buildCustomDemo();
      case WoxQueryHotkeysDemoMode.showcase:
        break;
    }

    final hotkey1 = _formatDemoHotkey('', fallback: Platform.isMacOS ? 'cmd+shift+g' : 'ctrl+shift+g');
    // Example 2 uses a fixed illustrative hotkey; it is not tied to any user
    // configuration because its purpose is to show the hide-chrome capability
    // rather than the exact shortcut value.
    const hotkey2 = 'Ctrl+Shift+I';

    return AnimatedBuilder(
      key: const ValueKey('onboarding-query-hotkeys-demo'),
      animation: _controller,
      builder: (context, child) {
        final sp1 = _shortcutProgress1();
        final wp1 = _windowProgress1();
        final sp2 = _shortcutProgress2();
        final wp2 = _windowProgress2();
        final ex1 = _example1Opacity;
        final ex2 = _example2Opacity;

        return ClipRRect(
          borderRadius: BorderRadius.circular(8),
          child: Stack(
            children: [
              Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: Platform.isMacOS, showDefaultIcons: false)),

              // ── Example 1: Ctrl+Shift+G opens a normal Wox query ───────────
              if (ex1 > 0.01)
                Positioned.fill(
                  child: Opacity(
                    opacity: ex1,
                    child: Padding(
                      padding: _demoDesktopHintContentPadding(),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          WoxDemoHintCard(
                            accent: widget.accent,
                            icon: Icons.keyboard_command_key,
                            title: widget.tr('onboarding_query_hotkeys_title'),
                            from: hotkey1,
                            to: 'github repo',
                          ),
                          const SizedBox(height: 12),
                          Expanded(
                            child: Stack(
                              children: [
                                Positioned.fill(
                                  child: Opacity(
                                    opacity: sp1,
                                    child: Transform.translate(
                                      offset: Offset(0, 8 * (1 - sp1)),
                                      child: _HotkeyPressOverlay(hotkey: hotkey1, accent: widget.accent, pressed: _isShortcutPressed1()),
                                    ),
                                  ),
                                ),
                                if (wp1 > 0.01)
                                  Positioned.fill(
                                    child: Transform.translate(
                                      offset: Offset(0, 20 * (1 - wp1)),
                                      child: Transform.scale(
                                        scale: 0.95 + (0.05 * wp1),
                                        child: WoxDemoWindow(
                                          accent: widget.accent,
                                          query: 'github repo',
                                          opaqueBackground: true,
                                          footerHotkey: _demoActionPanelHotkey(),
                                          results: [
                                            WoxDemoResult(
                                              title: 'Wox repository',
                                              subtitle: 'Open Wox-launcher/Wox on GitHub',
                                              icon: const Icon(Icons.code_rounded, color: Colors.white, size: 23),
                                              selected: true,
                                              tail: hotkey1,
                                            ),
                                            WoxDemoResult(
                                              title: widget.tr('onboarding_query_hotkeys_title'),
                                              subtitle: widget.tr('onboarding_query_hotkeys_body'),
                                              icon: Icon(Icons.bolt_outlined, color: widget.accent, size: 23),
                                              tail: widget.tr('ui_query_hotkeys'),
                                            ),
                                            const WoxDemoResult(
                                              title: 'Issues',
                                              subtitle: 'github repo issues',
                                              icon: Icon(Icons.bug_report_outlined, color: Color(0xFFFACC15), size: 23),
                                              tail: 'GitHub',
                                            ),
                                          ],
                                        ),
                                      ),
                                    ),
                                  ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),

              // ── Example 2: Ctrl+Shift+X opens a borderless webview ─────────
              // Demonstrates that hideQueryBox + hideToolbar lets the entire
              // Wox window become a frameless embedded webpage, ideal for
              // quick browsing via the webview plugin.
              if (ex2 > 0.01)
                Positioned.fill(
                  child: Opacity(
                    opacity: ex2,
                    child: Padding(
                      padding: _demoDesktopHintContentPadding(),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          WoxDemoHintCard(
                            accent: widget.accent,
                            icon: Icons.keyboard_command_key,
                            title: widget.tr('onboarding_query_hotkeys_title'),
                            from: hotkey2,
                            to: 'webview instagram',
                          ),
                          const SizedBox(height: 12),
                          Expanded(
                            child: Stack(
                              children: [
                                // Hotkey press overlay – same visual as example 1 but with hotkey2.
                                Positioned.fill(
                                  child: Opacity(
                                    opacity: sp2,
                                    child: Transform.translate(
                                      offset: Offset(0, 8 * (1 - sp2)),
                                      child: _HotkeyPressOverlay(hotkey: hotkey2, accent: widget.accent, pressed: _isShortcutPressed2()),
                                    ),
                                  ),
                                ),
                                // Instagram webview window – rises in after the
                                // hotkey press. It is narrower than the full
                                // available width (capped at 340 px) to convey
                                // that Query Hotkeys support a custom window
                                // size — the user would set a narrow width so
                                // the frameless page sits in one corner of the
                                // screen without covering everything.
                                if (wp2 > 0.01)
                                  Positioned(
                                    top: 0,
                                    bottom: 0,
                                    // Center a narrow window inside the stack,
                                    // mirroring how a real narrow-hotkey window
                                    // would appear floating on the desktop.
                                    left: 0,
                                    right: 0,
                                    child: Center(
                                      child: ConstrainedBox(
                                        constraints: const BoxConstraints(maxWidth: 340),
                                        child: Transform.translate(
                                          offset: Offset(0, 20 * (1 - wp2)),
                                          child: Transform.scale(scale: 0.95 + (0.05 * wp2), child: const _InstagramWebviewWindow()),
                                        ),
                                      ),
                                    ),
                                  ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
            ],
          ),
        );
      },
    );
  }
}

// Silent execution runs the query without showing the launcher, so the preview
// uses a compact completion toast instead of another Wox window.
class _SilentExecutionToast extends StatelessWidget {
  const _SilentExecutionToast({required this.accent, required this.hotkey});

  final Color accent;
  final String hotkey;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 312,
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.96),
        border: Border.all(color: accent.withValues(alpha: 0.28)),
        borderRadius: BorderRadius.circular(12),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.18), blurRadius: 24, offset: const Offset(0, 14))],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 34,
                height: 34,
                decoration: BoxDecoration(color: accent.withValues(alpha: 0.18), borderRadius: BorderRadius.circular(10)),
                child: Icon(Icons.copy_all_rounded, color: accent, size: 18),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('Copied GitHub repo URL', style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: FontWeight.w700)),
                    const SizedBox(height: 2),
                    Text('Triggered by $hotkey without opening Wox', style: TextStyle(color: getThemeSubTextColor(), fontSize: 10.5, height: 1.25)),
                  ],
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(color: accent.withValues(alpha: 0.12), borderRadius: BorderRadius.circular(999)),
                child: Text('Silent', style: TextStyle(color: accent, fontSize: 10, fontWeight: FontWeight.w700)),
              ),
            ],
          ),
          const SizedBox(height: 10),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
            decoration: BoxDecoration(color: getThemeTextColor().withValues(alpha: 0.05), borderRadius: BorderRadius.circular(10)),
            child: Row(
              children: [
                Icon(Icons.check_circle_rounded, color: accent, size: 16),
                const SizedBox(width: 8),
                Expanded(
                  child: Text('No launcher window appeared; the action completed in the background.', style: TextStyle(color: getThemeSubTextColor(), fontSize: 10.5, height: 1.3)),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// Custom mode is about combining layout knobs, so the preview highlights a
// narrow top-right launcher with the query box hidden but toolbar still shown.
class _CustomModeSummaryBadge extends StatelessWidget {
  const _CustomModeSummaryBadge({required this.accent});

  final Color accent;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.94),
        border: Border.all(color: accent.withValues(alpha: 0.24)),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Text('Mixed layout example', style: TextStyle(color: getThemeTextColor(), fontSize: 11.5, fontWeight: FontWeight.w700)),
          const SizedBox(height: 4),
          Text('Top-right  •  320 px  •  query hidden', style: TextStyle(color: getThemeSubTextColor(), fontSize: 10.5, height: 1.25)),
          const SizedBox(height: 2),
          Text('Toolbar still visible for action hints', style: TextStyle(color: getThemeSubTextColor(), fontSize: 10.5, height: 1.25)),
        ],
      ),
    );
  }
}

// A mocked Instagram post rendered inside a Wox-window chrome that has no
// query box and no toolbar.  The sole purpose of this widget is to convey
// the "borderless embedded webpage" concept that hideQueryBox + hideToolbar
// enables when combined with the webview plugin.  All content is fictional.
class _InstagramWebviewWindow extends StatelessWidget {
  const _InstagramWebviewWindow();

  // Instagram-like color palette (light theme to contrast with the dark Wox UI).
  static const _bg = Color(0xFFFFFFFF);
  static const _textColor = Color(0xFF000000);
  static const _subText = Color(0xFF737373);
  static const _divider = Color(0xFFDBDBDB);
  static const _igBlue = Color(0xFF0095F6);

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(color: _bg, border: Border.all(color: getThemeTextColor().withValues(alpha: 0.10)), borderRadius: BorderRadius.circular(8)),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Column(
          children: [
            // ── Instagram top bar ────────────────────────────────────────────
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
              decoration: const BoxDecoration(color: _bg, border: Border(bottom: BorderSide(color: _divider, width: 0.5))),
              child: Row(
                children: [
                  Expanded(
                    child: Row(
                      children: const [
                        Text('Instagram', style: TextStyle(color: _textColor, fontSize: 18, fontWeight: FontWeight.w700, fontStyle: FontStyle.italic)),
                        SizedBox(width: 3),
                        Icon(Icons.keyboard_arrow_down_rounded, color: _textColor, size: 18),
                      ],
                    ),
                  ),
                  const Icon(Icons.add_box_outlined, color: _textColor, size: 22),
                  const SizedBox(width: 16),
                  Stack(
                    clipBehavior: Clip.none,
                    children: [
                      const Icon(Icons.favorite_border, color: _textColor, size: 22),
                      // Notification dot
                      Positioned(top: -1, right: -1, child: Container(width: 7, height: 7, decoration: const BoxDecoration(color: Color(0xFFE1306C), shape: BoxShape.circle))),
                    ],
                  ),
                ],
              ),
            ),

            // ── Post image placeholder ───────────────────────────────────────
            // A gradient stand-in for the photo of a girl holding a camera in
            // front of colorful koi-nobori (kite) streamers.
            Expanded(
              child: Stack(
                fit: StackFit.expand,
                children: [
                  // Sky-to-ground gradient evokes a sunny outdoor scene.
                  Container(
                    decoration: const BoxDecoration(
                      gradient: LinearGradient(colors: [Color(0xFF87CEEB), Color(0xFFB0D8C8), Color(0xFFF5EFD8)], begin: Alignment.topCenter, end: Alignment.bottomCenter),
                    ),
                  ),
                  // A few decorative streamer rectangles to hint at the koi flags.
                  Positioned(left: 18, top: 12, child: _KoiFlag(color: const Color(0xFFE96B6B), angle: -0.15)),
                  Positioned(left: 38, top: 6, child: _KoiFlag(color: const Color(0xFF5BC8E8), angle: 0.08)),
                  Positioned(left: 60, top: 18, child: _KoiFlag(color: const Color(0xFFF5C842), angle: -0.05)),
                  Positioned(left: 80, top: 8, child: _KoiFlag(color: const Color(0xFF82D48A), angle: 0.12)),
                  Positioned(left: 98, top: 16, child: _KoiFlag(color: const Color(0xFFE96B6B), angle: -0.08)),
                  Positioned(right: 18, top: 10, child: _KoiFlag(color: const Color(0xFF5BC8E8), angle: 0.15)),
                  Positioned(right: 40, top: 4, child: _KoiFlag(color: const Color(0xFFF5C842), angle: -0.10)),
                  Positioned(right: 62, top: 20, child: _KoiFlag(color: const Color(0xFF82D48A), angle: 0.06)),
                  // Silhouette of a person (avatar placeholder).
                  Positioned(
                    bottom: 10,
                    left: 0,
                    right: 0,
                    child: Center(
                      child: Container(
                        width: 40,
                        height: 40,
                        decoration: BoxDecoration(color: const Color(0xFFCCCCCC), shape: BoxShape.circle, border: Border.all(color: Colors.white, width: 2)),
                        child: const Icon(Icons.person, color: Color(0xFF888888), size: 24),
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // ── Post actions and caption ─────────────────────────────────────
            Container(
              color: _bg,
              padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  // Action icon row: like, comment, share, bookmark.
                  Row(
                    children: [
                      const Icon(Icons.favorite_border, color: _textColor, size: 20),
                      const SizedBox(width: 12),
                      const Icon(Icons.chat_bubble_outline, color: _textColor, size: 20),
                      const SizedBox(width: 12),
                      Transform.rotate(angle: -0.45, child: const Icon(Icons.send_outlined, color: _textColor, size: 20)),
                      const Spacer(),
                      const Icon(Icons.bookmark_border, color: _textColor, size: 20),
                    ],
                  ),
                  const SizedBox(height: 4),
                  const Text('eu_imozou和其他用户赞了', style: TextStyle(color: _textColor, fontSize: 10, fontWeight: FontWeight.w600)),
                  const SizedBox(height: 2),
                  RichText(
                    text: const TextSpan(
                      children: [
                        TextSpan(text: 'camel8326', style: TextStyle(color: _textColor, fontSize: 10, fontWeight: FontWeight.w700)),
                        TextSpan(text: '  こどもの日…', style: TextStyle(color: _textColor, fontSize: 10)),
                        TextSpan(text: '  更多', style: TextStyle(color: _subText, fontSize: 10)),
                      ],
                    ),
                  ),
                  const SizedBox(height: 2),
                  const Text('5天前', style: TextStyle(color: _subText, fontSize: 9)),

                  // "Use this app" call-to-action banner that appears in mobile
                  // web Instagram, reinforcing the "real webpage" feel.
                  Container(
                    margin: const EdgeInsets.only(top: 6),
                    padding: const EdgeInsets.symmetric(horizontal: 0, vertical: 4),
                    decoration: const BoxDecoration(border: Border(top: BorderSide(color: _divider, width: 0.5))),
                    child: Row(
                      children: const [
                        Expanded(child: Text('使用这款应用', style: TextStyle(color: _igBlue, fontSize: 10, fontWeight: FontWeight.w600))),
                        Icon(Icons.close, color: _subText, size: 14),
                      ],
                    ),
                  ),
                ],
              ),
            ),

            // ── Bottom navigation bar ────────────────────────────────────────
            Container(
              decoration: const BoxDecoration(color: _bg, border: Border(top: BorderSide(color: _divider, width: 0.5))),
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: [
                  const Icon(Icons.home_filled, color: _textColor, size: 22),
                  const Icon(Icons.search_rounded, color: _textColor, size: 22),
                  const Icon(Icons.play_circle_outline, color: _textColor, size: 22),
                  Transform.rotate(angle: -0.45, child: const Icon(Icons.send_outlined, color: _textColor, size: 22)),
                  CircleAvatar(radius: 10, backgroundImage: null, backgroundColor: Colors.grey.shade300),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// A small tilted rectangle that resembles a koi-nobori (kite streamer) in
// the Instagram post image placeholder.
class _KoiFlag extends StatelessWidget {
  const _KoiFlag({required this.color, required this.angle});

  final Color color;
  final double angle;

  @override
  Widget build(BuildContext context) {
    return Transform.rotate(
      angle: angle,
      child: Container(width: 8, height: 28, decoration: BoxDecoration(color: color.withValues(alpha: 0.80), borderRadius: BorderRadius.circular(2))),
    );
  }
}
