part of 'wox_demo.dart';

class WoxQueryShortcutsDemo extends StatefulWidget {
  const WoxQueryShortcutsDemo({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  State<WoxQueryShortcutsDemo> createState() => _WoxQueryShortcutsDemoState();
}

class _WoxQueryShortcutsDemoState extends State<WoxQueryShortcutsDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 4400))..repeat();
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

  String _queryText() {
    // Bug fix: the previous hard-coded stage ranges had unequal character
    // intervals (528ms, 792ms, 440ms, 440ms) and a 3-char jump from 'gh r'
    // directly to 'gh repo'. Typing character-by-character at a uniform
    // ~264ms/char keeps each keystroke visible and feels natural.
    const target = 'gh repo';
    if (_controller.value < 0.18) return '';
    final t = ((_controller.value - 0.18) / (0.62 - 0.18)).clamp(0.0, 1.0);
    return target.substring(0, (t * target.length).floor().clamp(0, target.length));
  }

  bool _isExpanded() {
    return _controller.value >= 0.68 && _controller.value < 0.94;
  }

  @override
  Widget build(BuildContext context) {
    // Feature change: Query Shortcuts are now shown as a typing workflow. The
    // animation makes the alias expansion visible, which the old combined
    // advanced-query page could only describe in text.
    return AnimatedBuilder(
      key: const ValueKey('onboarding-query-shortcuts-demo'),
      animation: _controller,
      builder: (context, child) {
        final expandedProgress = _isExpanded() ? _interval(0.68, 0.80, Curves.easeOutCubic) : 0.0;

        return ClipRRect(
          borderRadius: BorderRadius.circular(8),
          child: Stack(
            children: [
              Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: Platform.isMacOS, showDefaultIcons: false)),
              Positioned.fill(
                child: Padding(
                  // Feature fix: hint content stays in a horizontal strip above
                  // Wox. This preserves the top/bottom demo rhythm while keeping
                  // the alias mapping visible instead of letting the launcher
                  // overlap the teaching content.
                  padding: _demoDesktopHintContentPadding(),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      WoxDemoHintCard(
                        accent: widget.accent,
                        icon: Icons.short_text_outlined,
                        title: widget.tr('onboarding_query_shortcuts_title'),
                        from: 'gh repo',
                        to: 'github repo',
                        progress: 0.35 + (0.65 * expandedProgress),
                      ),
                      const SizedBox(height: 12),
                      Expanded(
                        child: WoxDemoWindow(
                          accent: widget.accent,
                          query: _queryText(),
                          opaqueBackground: true,
                          footerHotkey: _demoActionPanelHotkey(),
                          results: [
                            WoxDemoResult(
                              // Feature fix: Wox keeps the visible query as
                              // "gh repo"; only the internal query sent to
                              // providers expands to "github repo".
                              title: 'Open repository',
                              subtitle: _isExpanded() ? 'github repo' : widget.tr('onboarding_query_shortcuts_body'),
                              icon: const Icon(Icons.short_text_outlined, color: Colors.white, size: 23),
                              selected: true,
                              tail: _isExpanded() ? 'gh' : widget.tr('ui_query_shortcuts'),
                            ),
                            WoxDemoResult(
                              title: 'Repository search',
                              subtitle: 'github repo',
                              icon: Icon(Icons.open_in_new_rounded, color: widget.accent, size: 23),
                              tail: 'Enter',
                            ),
                            const WoxDemoResult(title: 'Search issues', subtitle: 'github issues', icon: Icon(Icons.search_rounded, color: Color(0xFF60A5FA), size: 23)),
                          ],
                        ),
                      ),
                    ],
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
