part of 'wox_demo.dart';

class _ThemeStoreDemo extends StatefulWidget {
  const _ThemeStoreDemo({
    required this.demoKey,
    required this.accent,
    required this.icon,
    required this.title,
    required this.hintFrom,
    required this.hintTo,
    required this.queryStages,
    required this.installLabel,
    required this.installingLabel,
    required this.installedLabel,
    required this.primaryTitle,
    required this.primarySubtitle,
    required this.primaryIcon,
    required this.secondaryResults,
    required this.appliedTheme,
  });

  final ValueKey<String> demoKey;
  final Color accent;
  final IconData icon;
  final String title;
  final String hintFrom;
  final String hintTo;
  final List<String> queryStages;
  final String installLabel;
  final String installingLabel;
  final String installedLabel;
  final String primaryTitle;
  final String primarySubtitle;
  final Widget primaryIcon;
  final List<WoxDemoResult> secondaryResults;
  final _DemoThemeData appliedTheme;

  @override
  State<_ThemeStoreDemo> createState() => _ThemeStoreDemoState();
}

class _ThemeStoreDemoState extends State<_ThemeStoreDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    // Feature separation: theme store keeps its own install/apply timeline so
    // the plugin store animation can evolve independently without adding mode
    // flags to a shared install-flow widget.
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 5600))..repeat();
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
    // Bug fix retained from the earlier install demo: linear per-character
    // typing avoids uneven jumps between query stages and keeps the theme query
    // readable before the apply state starts.
    final target = widget.queryStages.last;
    if (target.isEmpty) return '';
    final t = _interval(0.08, 0.29, Curves.linear);
    return target.substring(0, (t * target.length).floor().clamp(0, target.length));
  }

  bool _isThemeApplied() => _controller.value >= 0.64 && _controller.value < 0.95;

  String _primaryTail() {
    if (_controller.value >= 0.53 && _controller.value < 0.63) {
      return widget.installingLabel;
    }
    if (_controller.value >= 0.63 && _controller.value < 0.95) {
      return widget.installedLabel;
    }
    return widget.installLabel;
  }

  Widget _buildDemoWindow() {
    final isApplied = _isThemeApplied();
    final effectiveAccent = isApplied ? widget.appliedTheme.accent : widget.accent;
    final window = WoxDemoWindow(
      accent: effectiveAccent,
      query: _queryText(),
      opaqueBackground: true,
      results: [
        WoxDemoResult(title: widget.primaryTitle, subtitle: widget.primarySubtitle, icon: widget.primaryIcon, selected: true, tail: _primaryTail()),
        ...widget.secondaryResults,
      ],
    );

    if (isApplied) {
      // Theme store feature: the demo applies the target theme through local
      // inherited colors only. This shows the visual result while avoiding any
      // mutation of global WoxThemeUtil state during onboarding.
      return _InheritedDemoTheme(data: widget.appliedTheme, child: window);
    }

    return window;
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      key: widget.demoKey,
      animation: _controller,
      builder: (context, child) {
        return ClipRRect(
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
                      WoxDemoHintCard(accent: widget.accent, icon: widget.icon, title: widget.title, from: widget.hintFrom, to: widget.hintTo),
                      const SizedBox(height: 12),
                      Expanded(
                        child: AnimatedSwitcher(duration: const Duration(milliseconds: 500), child: KeyedSubtree(key: ValueKey(_isThemeApplied()), child: _buildDemoWindow())),
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
