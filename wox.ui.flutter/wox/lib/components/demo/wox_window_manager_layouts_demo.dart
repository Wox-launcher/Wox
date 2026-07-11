part of 'wox_demo.dart';

class WoxWindowManagerLayoutsDemo extends StatefulWidget {
  const WoxWindowManagerLayoutsDemo({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  State<WoxWindowManagerLayoutsDemo> createState() => _WoxWindowManagerLayoutsDemoState();
}

class _WoxWindowManagerLayoutsDemoState extends State<WoxWindowManagerLayoutsDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  // Single sequential loop: search -> confirm -> hide -> expand -> hold -> fade.
  static const Duration _loopDuration = Duration(milliseconds: 6500);

  // Query typed character-by-character during the search phase.
  static const String _query = 'code';

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: _loopDuration)..repeat();
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

  // Phase 1 (0.10 -> 0.19): query text grows from empty to 'code' (faster typing).
  String _queryText() {
    if (_controller.value < 0.10) return '';
    final t = ((_controller.value - 0.10) / (0.19 - 0.10)).clamp(0.0, 1.0);
    return _query.substring(0, (t * _query.length).floor().clamp(0, _query.length));
  }

  // Phase 3 (0.36 -> 0.47): Wox scales down and fades out after Enter.
  double _woxHideProgress() {
    if (_controller.value < 0.36) return 0;
    if (_controller.value > 0.47) return 1;
    return _interval(0.36, 0.47, Curves.easeInCubic);
  }

  // Phase 4 (0.47 -> 0.69): three tiles expand from center to target rects.
  double _layoutProgress() {
    if (_controller.value < 0.47) return 0;
    if (_controller.value > 0.69) return 1;
    return _interval(0.47, 0.69, Curves.easeOutCubic);
  }

  // Phase 6 (0.89 -> 1.00): layout fades out before the loop restarts.
  double _layoutFadeOut() {
    if (_controller.value < 0.89) return 0;
    return _interval(0.89, 1.00, Curves.easeInCubic);
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      key: const ValueKey('window-manager-layouts-demo'),
      animation: _controller,
      builder: (context, child) {
        final hideProgress = _woxHideProgress();
        final layoutProgress = _layoutProgress();
        final layoutOpacity = (1 - _layoutFadeOut()) * layoutProgress;

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
                      WoxDemoHintCard(
                        accent: widget.accent,
                        icon: Icons.view_quilt_outlined,
                        title: widget.tr('plugin_window_manager_layouts_demo_title'),
                        from: _query,
                        to: widget.tr('plugin_window_manager_layouts_demo_three_pane'),
                      ),
                      const SizedBox(height: 12),
                      Expanded(
                        child: Stack(
                          children: [
                            // Phase 1-3: Wox search window, then hides on Enter.
                            Opacity(
                              opacity: 1 - hideProgress,
                              child: Transform.scale(
                                scale: 1 - hideProgress,
                                alignment: Alignment.center,
                                child: WoxDemoWindow(
                                  accent: widget.accent,
                                  query: _queryText(),
                                  opaqueBackground: true,
                                  showToolbar: false,
                                  results: [
                                    WoxDemoResult(
                                      title: 'Code',
                                      subtitle: widget.tr('plugin_window_manager_group_action_apply'),
                                      icon: Icon(Icons.view_quilt_outlined, color: widget.accent, size: 23),
                                      selected: true,
                                      tail: widget.tr('plugin_window_manager_setting_groups'),
                                    ),
                                    const WoxDemoResult(title: 'Browser', subtitle: 'Right display', icon: Icon(Icons.language_rounded, color: Color(0xFF34D399), size: 23)),
                                    const WoxDemoResult(title: 'Terminal', subtitle: 'Bottom-right', icon: Icon(Icons.terminal_rounded, color: Color(0xFFFACC15), size: 23)),
                                  ],
                                ),
                              ),
                            ),
                            // Phase 4-6: three-pane layout fills the area.
                            if (layoutOpacity > 0) Opacity(opacity: layoutOpacity, child: _WindowManagerLayout(progress: layoutProgress)),
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

// Three window tiles that expand from a center rect to fill the available area.
// Replaces the old _WindowManagerLayoutPreview + _DemoMonitor pair.
class _WindowManagerLayout extends StatelessWidget {
  const _WindowManagerLayout({required this.progress});

  final double progress;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        final height = constraints.maxHeight;

        return Stack(
          children: [
            _DemoWindowTile(
              title: 'Code',
              icon: Icons.code_rounded,
              color: const Color(0xFF60A5FA),
              rect: const Rect.fromLTWH(0.02, 0.06, 0.46, 0.88),
              progress: progress,
              areaWidth: width,
              areaHeight: height,
            ),
            _DemoWindowTile(
              title: 'Browser',
              icon: Icons.language_rounded,
              color: const Color(0xFF34D399),
              rect: const Rect.fromLTWH(0.52, 0.06, 0.46, 0.42),
              progress: progress,
              areaWidth: width,
              areaHeight: height,
            ),
            _DemoWindowTile(
              title: 'Terminal',
              icon: Icons.terminal_rounded,
              color: const Color(0xFFFACC15),
              rect: const Rect.fromLTWH(0.52, 0.52, 0.46, 0.42),
              progress: progress,
              areaWidth: width,
              areaHeight: height,
            ),
          ],
        );
      },
    );
  }
}

class _DemoWindowTile extends StatelessWidget {
  const _DemoWindowTile({
    required this.title,
    required this.icon,
    required this.color,
    required this.rect,
    required this.progress,
    required this.areaWidth,
    required this.areaHeight,
  });

  final String title;
  final IconData icon;
  final Color color;
  final Rect rect;
  final double progress;
  final double areaWidth;
  final double areaHeight;

  @override
  Widget build(BuildContext context) {
    final target = Rect.fromLTWH(rect.left * areaWidth, rect.top * areaHeight, rect.width * areaWidth, rect.height * areaHeight);
    // Tiles start collapsed at the center of the area and expand outward.
    final start = Rect.fromLTWH(areaWidth * 0.30, areaHeight * 0.40, areaWidth * 0.40, areaHeight * 0.24);
    final current = Rect.lerp(start, target, progress)!;

    return Positioned(
      left: current.left,
      top: current.top,
      width: current.width,
      height: current.height,
      child: Opacity(
        opacity: 0.40 + (0.60 * progress),
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: Color.lerp(getThemeBackgroundColor(), color, 0.16)!.withValues(alpha: 0.92),
            border: Border.all(color: color.withValues(alpha: 0.62)),
            borderRadius: BorderRadius.circular(6),
          ),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
            child: Row(
              children: [
                Icon(icon, color: color, size: 16),
                const SizedBox(width: 6),
                Expanded(child: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 11, fontWeight: FontWeight.w700))),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
