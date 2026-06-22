part of 'wox_demo.dart';

class WoxTrayQueriesDemo extends StatefulWidget {
  const WoxTrayQueriesDemo({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  State<WoxTrayQueriesDemo> createState() => _WoxTrayQueriesDemoState();
}

class _WoxTrayQueriesDemoState extends State<WoxTrayQueriesDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 5000))..repeat();
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

  double _cursorProgress() {
    if (_controller.value < 0.10) return 0;
    if (_controller.value < 0.38) {
      return _interval(0.10, 0.38, Curves.easeInOutCubic);
    }
    return 1;
  }

  double _windowProgress() {
    if (_controller.value < 0.48) return 0;
    if (_controller.value < 0.66) {
      return _interval(0.48, 0.66, Curves.easeOutCubic);
    }
    if (_controller.value < 0.92) return 1;
    return 1 - _interval(0.92, 1, Curves.easeInCubic);
  }

  bool _isTrayPressed() {
    return _controller.value >= 0.38 && _controller.value <= 0.50;
  }

  @override
  Widget build(BuildContext context) {
    final isMac = Platform.isMacOS;

    // Feature change: Tray Queries now show the click path from tray/menu-bar
    // icon to a query window near that system area. This separates the tray
    // mental model from keyboard-triggered query features.
    return AnimatedBuilder(
      key: const ValueKey('onboarding-tray-queries-demo'),
      animation: _controller,
      builder: (context, child) {
        final cursorProgress = _cursorProgress();
        final windowProgress = _windowProgress();

        return LayoutBuilder(
          builder: (context, constraints) {
            // Bug fix: the macOS tray affordance lives inside a 28 px simulated
            // menu bar, so it needs menu-bar-sized geometry. Reusing the 28 px
            // Windows taskbar button made the icon overflow the bar and look
            // vertically offset from Finder/search/time.
            final trayIconSize = isMac ? 18.0 : 28.0;
            const trayWindowGap = 4.0;
            final trayAnchor = isMac ? Offset(constraints.maxWidth - 88, 14) : Offset(constraints.maxWidth - 120, constraints.maxHeight - 23);
            final startCursor = Offset(60, constraints.maxHeight - 82);
            final cursorTargetOffset = isMac ? trayAnchor.translate(-6, -7) : trayAnchor.translate(-4, -8);
            final cursorOffset = Offset.lerp(startCursor, cursorTargetOffset, cursorProgress)!;
            final maxWindowWidth = (constraints.maxWidth - 96).clamp(260.0, double.infinity).toDouble();
            final windowWidth = 420.0.clamp(260.0, maxWindowWidth).toDouble();
            final windowHeight = (24 + (WoxThemeUtil.instance.getResultItemHeight() * 3)).clamp(150.0, 240.0).toDouble();
            final maxWindowLeft = (constraints.maxWidth - windowWidth - 48).clamp(48.0, double.infinity).toDouble();
            final windowLeft = (isMac ? trayAnchor.dx - windowWidth + (trayIconSize / 2) : trayAnchor.dx - windowWidth + trayIconSize).clamp(48.0, maxWindowLeft).toDouble();
            final trayIconTop = trayAnchor.dy - (trayIconSize / 2);
            final trayIconBottom = trayAnchor.dy + (trayIconSize / 2);
            const hintCardHeight = 66.0;
            const hintWindowGap = 12.0;
            const hintBottomInset = 36.0;
            final defaultHintTop = _demoDesktopHintTopInset();
            final hintSafeTop = defaultHintTop + hintCardHeight + 18;
            final macHintTop = math.max(trayIconBottom + trayWindowGap + windowHeight + hintWindowGap, constraints.maxHeight - hintCardHeight - hintBottomInset);
            final macMaxWindowTop = (macHintTop - windowHeight - hintWindowGap).clamp(trayIconBottom + trayWindowGap, double.infinity).toDouble();
            final windowsMaxWindowTop = (constraints.maxHeight - windowHeight - 52).clamp(hintSafeTop, double.infinity).toDouble();
            // Bug fix: macOS tray queries open from the top menu bar, so Wox must
            // appear directly below the status icon, while the explanatory hint
            // belongs near the bottom of the simulated desktop. The old shared
            // top-hint clamp matched Windows' bottom-taskbar flow and later
            // middle-stacked the macOS hint under Wox instead of pinning it to
            // the desktop edge.
            final windowTop =
                isMac
                    ? (trayIconBottom + trayWindowGap).clamp(trayIconBottom + trayWindowGap, macMaxWindowTop).toDouble()
                    : (trayIconTop - windowHeight - trayWindowGap).clamp(hintSafeTop, windowsMaxWindowTop).toDouble();
            final hintTop = isMac ? macHintTop : defaultHintTop;

            return ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Stack(
                children: [
                  Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: isMac, showDefaultIcons: false)),
                  Positioned(
                    left: 48,
                    right: 52,
                    top: hintTop,
                    child: WoxDemoHintCard(
                      accent: widget.accent,
                      icon: Icons.ads_click_rounded,
                      title: widget.tr('onboarding_tray_queries_title'),
                      from: 'tray icon',
                      to: 'weather',
                    ),
                  ),
                  Positioned(
                    left: trayAnchor.dx - (trayIconSize / 2),
                    top: trayAnchor.dy - (trayIconSize / 2),
                    child: _TrayQueryIcon(accent: widget.accent, pressed: _isTrayPressed(), size: trayIconSize, menuBarStyle: isMac),
                  ),
                  Positioned(left: cursorOffset.dx, top: cursorOffset.dy, child: _DemoCursor(accent: widget.accent)),
                  if (windowProgress > 0.01)
                    Positioned(
                      left: windowLeft,
                      top: windowTop,
                      width: windowWidth,
                      height: windowHeight,
                      child: Transform.translate(
                        offset: Offset(0, 18 * (1 - windowProgress)),
                        child: Transform.scale(
                          scale: 0.95 + (0.05 * windowProgress),
                          alignment: isMac ? Alignment.topRight : Alignment.bottomRight,
                          child: WoxDemoWindow(
                            accent: widget.accent,
                            query: 'weather',
                            opaqueBackground: true,
                            showQueryBox: false,
                            showToolbar: false,
                            results: [
                              WoxDemoResult(
                                title: 'Weather',
                                subtitle: 'Sunny, 24 C',
                                icon: const Icon(Icons.wb_sunny_outlined, color: Colors.white, size: 23),
                                selected: true,
                                tail: widget.tr('ui_tray_queries'),
                              ),
                              WoxDemoResult(
                                title: widget.tr('onboarding_tray_queries_title'),
                                subtitle: widget.tr('onboarding_tray_queries_body'),
                                icon: Icon(Icons.ads_click_rounded, color: widget.accent, size: 23),
                                tail: 'Tray',
                              ),
                              const WoxDemoResult(
                                title: 'Calendar',
                                subtitle: 'Next meeting in 25 minutes',
                                icon: Icon(Icons.calendar_month_outlined, color: Color(0xFF60A5FA), size: 23),
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
      },
    );
  }
}
