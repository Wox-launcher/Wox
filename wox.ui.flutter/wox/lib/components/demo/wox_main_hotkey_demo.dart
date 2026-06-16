part of 'wox_demo.dart';

class WoxMainHotkeyDemo extends StatefulWidget {
  const WoxMainHotkeyDemo({super.key, required this.accent, required this.hotkey, required this.tr});

  final Color accent;
  final String hotkey;
  final String Function(String key) tr;

  @override
  State<WoxMainHotkeyDemo> createState() => _WoxMainHotkeyDemoState();
}

class _WoxMainHotkeyDemoState extends State<WoxMainHotkeyDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 4200))..repeat();
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

  double _windowProgress() {
    if (_controller.value < 0.28) {
      return 0;
    }
    if (_controller.value < 0.46) {
      return _interval(0.28, 0.46, Curves.easeOutCubic);
    }
    if (_controller.value < 0.88) {
      return 1;
    }
    return 1 - _interval(0.88, 1, Curves.easeInCubic);
  }

  double _shortcutProgress() {
    if (_controller.value < 0.10) {
      return 0;
    }
    if (_controller.value < 0.22) {
      return _interval(0.10, 0.22, Curves.easeOutCubic);
    }
    if (_controller.value < 0.54) {
      return 1;
    }
    return 1 - _interval(0.54, 0.68, Curves.easeInCubic);
  }

  bool _isShortcutPressed() {
    return _controller.value >= 0.20 && _controller.value <= 0.34;
  }

  String _queryText() {
    // Bug fix: each character previously showed after a fixed 336ms gap (0.08 × 4200ms),
    // making 3-char typing feel sluggish. Now matched to the ~65ms/char reference
    // speed from the wpm-install-everything demo (22 chars over 1425ms).
    // 'app' (3 chars) × 65ms ≈ 195ms = 0.046 of 4200ms; window: 0.52–0.566.
    const target = 'app';
    if (_controller.value < 0.52) return '';
    final t = ((_controller.value - 0.52) / (0.566 - 0.52)).clamp(0.0, 1.0);
    return target.substring(0, (t * target.length).floor().clamp(0, target.length));
  }

  String _displayHotkey() {
    return _formatDemoHotkey(widget.hotkey, fallback: Platform.isMacOS ? 'option+space' : 'alt+space');
  }

  @override
  Widget build(BuildContext context) {
    final hotkey = _displayHotkey();
    final desktopIsMac = Platform.isMacOS;

    // Feature change: the main-hotkey preview now teaches the real launch
    // moment instead of showing an already-open Wox window. A scripted Flutter
    // scene keeps the demo theme-aware and platform-aware without shipping
    // separate recorded videos for macOS, Windows, and Linux.
    return AnimatedBuilder(
      key: const ValueKey('onboarding-main-hotkey-demo'),
      animation: _controller,
      builder: (context, child) {
        final shortcutProgress = _shortcutProgress();
        final windowProgress = _windowProgress();

        return ClipRRect(
          borderRadius: BorderRadius.circular(8),
          child: Stack(
            children: [
              Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: desktopIsMac)),
              Positioned.fill(
                child: Opacity(
                  opacity: shortcutProgress,
                  child: Transform.translate(
                    offset: Offset(0, 8 * (1 - shortcutProgress)),
                    child: _HotkeyPressOverlay(hotkey: hotkey, accent: widget.accent, pressed: _isShortcutPressed()),
                  ),
                ),
              ),
              // Feature refinement: Wox now enters by position and scale only.
              // Fading the whole launcher made the opened window look
              // translucent against the desktop, which weakened the demo.
              if (windowProgress > 0.01)
                Positioned.fill(
                  child: Transform.translate(
                    offset: Offset(0, 22 * (1 - windowProgress)),
                    child: Transform.scale(
                      scale: 0.95 + (0.05 * windowProgress),
                      child: Padding(
                        // Feature refinement: keep the opened Wox preview
                        // centered inside the now-taller demo area. The height
                        // fix belongs to the media slot, not to an artificial
                        // upward offset inside the desktop scene.
                        padding: const EdgeInsets.fromLTRB(34, 42, 34, 42),
                        child: WoxDemoWindow(
                          accent: widget.accent,
                          query: _queryText(),
                          opaqueBackground: true,
                          results: [
                            WoxDemoResult(
                              title: widget.tr('onboarding_main_hotkey_title'),
                              subtitle: widget.tr('onboarding_main_hotkey_tip'),
                              icon: const Icon(Icons.keyboard_alt_outlined, color: Colors.white, size: 23),
                              selected: true,
                              tail: hotkey,
                            ),
                            WoxDemoResult(
                              title: 'Applications',
                              subtitle: widget.tr('onboarding_media_app_result_subtitle'),
                              icon: Icon(Icons.apps_rounded, color: widget.accent, size: 23),
                              tail: 'Apps',
                            ),
                            WoxDemoResult(
                              title: 'Files',
                              subtitle: widget.tr('onboarding_media_file_result_subtitle'),
                              icon: const Icon(Icons.folder_outlined, color: Color(0xFFFACC15), size: 23),
                              tail: 'Files',
                            ),
                            const WoxDemoResult(
                              title: 'Plugins',
                              subtitle: r'C:\Users\qianl\dev\Wox.Plugin.Template.Nodejs',
                              icon: Icon(Icons.extension_outlined, color: Color(0xFF60A5FA), size: 23),
                              tail: '51 day ago',
                            ),
                          ],
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
  }
}
