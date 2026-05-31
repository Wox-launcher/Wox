part of 'wox_demo.dart';

class WoxSelectionHotkeyDemo extends StatefulWidget {
  const WoxSelectionHotkeyDemo({super.key, required this.accent, required this.hotkey, required this.tr});

  final Color accent;
  final String hotkey;
  final String Function(String key) tr;

  @override
  State<WoxSelectionHotkeyDemo> createState() => _WoxSelectionHotkeyDemoState();
}

class _WoxSelectionHotkeyDemoState extends State<WoxSelectionHotkeyDemo> with SingleTickerProviderStateMixin {
  static const String _selectedFileName = 'Quarterly plan.pdf';

  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 5200))..repeat();
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
    if (_controller.value < 0.08) {
      return 0;
    }
    if (_controller.value < 0.34) {
      return _interval(0.08, 0.34, Curves.easeInOutCubic);
    }
    return 1;
  }

  double _shortcutProgress() {
    if (_controller.value < 0.36) {
      return 0;
    }
    if (_controller.value < 0.46) {
      return _interval(0.36, 0.46, Curves.easeOutCubic);
    }
    if (_controller.value < 0.66) {
      return 1;
    }
    return 1 - _interval(0.66, 0.78, Curves.easeInCubic);
  }

  double _windowProgress() {
    if (_controller.value < 0.56) {
      return 0;
    }
    if (_controller.value < 0.74) {
      return _interval(0.56, 0.74, Curves.easeOutCubic);
    }
    if (_controller.value < 0.92) {
      return 1;
    }
    return 1 - _interval(0.92, 1, Curves.easeInCubic);
  }

  bool _isFileSelected() {
    return _controller.value >= 0.30 && _controller.value < 0.95;
  }

  bool _isShortcutPressed() {
    return _controller.value >= 0.46 && _controller.value <= 0.58;
  }

  String _displayHotkey() {
    return _formatDemoHotkey(widget.hotkey, fallback: Platform.isMacOS ? 'cmd+option+space' : 'ctrl+alt+space');
  }

  @override
  Widget build(BuildContext context) {
    final hotkey = _displayHotkey();
    final desktopIsMac = Platform.isMacOS;

    // Feature change: the selection-hotkey preview now demonstrates the real
    // workflow: choose something on the desktop, press the configured shortcut,
    // and open Wox with context-specific actions for that selection.
    return AnimatedBuilder(
      key: const ValueKey('onboarding-selection-hotkey-demo'),
      animation: _controller,
      builder: (context, child) {
        final cursorProgress = _cursorProgress();
        final shortcutProgress = _shortcutProgress();
        final windowProgress = _windowProgress();
        final fileSelected = _isFileSelected();

        return LayoutBuilder(
          builder: (context, constraints) {
            final startCursor = Offset(constraints.maxWidth - 96, constraints.maxHeight - 86);
            final targetCursor = Offset(186, 112);
            final cursorOffset = Offset.lerp(startCursor, targetCursor, cursorProgress)!;
            final cursorOpacity = 1 - _interval(0.70, 0.86, Curves.easeInCubic);

            return ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Stack(
                children: [
                  Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: desktopIsMac, showDefaultIcons: false)),
                  Positioned(left: 42, top: 54, child: WoxDemoDesktopFileIcon(label: 'Roadmap.md', icon: Icons.article_outlined, accent: const Color(0xFF60A5FA))),
                  Positioned(
                    left: 150,
                    top: 54,
                    child: WoxDemoDesktopFileIcon(label: _selectedFileName, icon: Icons.picture_as_pdf_outlined, accent: widget.accent, selected: fileSelected),
                  ),
                  Positioned(left: 258, top: 54, child: WoxDemoDesktopFileIcon(label: 'Screenshots', icon: Icons.folder_outlined, accent: const Color(0xFFFACC15))),
                  Positioned(left: 64, top: 150, child: WoxDemoDesktopFileIcon(label: 'Release notes.txt', icon: Icons.description_outlined, accent: const Color(0xFF34D399))),
                  if (cursorOpacity > 0.01)
                    Positioned(left: cursorOffset.dx, top: cursorOffset.dy, child: Opacity(opacity: cursorOpacity, child: _DemoCursor(accent: widget.accent))),
                  Positioned.fill(
                    child: Opacity(
                      opacity: shortcutProgress,
                      child: Transform.translate(
                        offset: Offset(0, 8 * (1 - shortcutProgress)),
                        child: _HotkeyPressOverlay(hotkey: hotkey, accent: widget.accent, pressed: _isShortcutPressed()),
                      ),
                    ),
                  ),
                  // Feature refinement: the selection launcher appears fully
                  // opaque, matching the main-hotkey demo and making the file
                  // action rows readable over the simulated desktop.
                  if (windowProgress > 0.01)
                    Positioned.fill(
                      child: Transform.translate(
                        offset: Offset(0, 20 * (1 - windowProgress)),
                        child: Transform.scale(
                          scale: 0.95 + (0.05 * windowProgress),
                          child: Center(
                            child: SizedBox(
                              width: (constraints.maxWidth - 104).clamp(560.0, 760.0).toDouble(),
                              height: (constraints.maxHeight * 0.72).clamp(340.0, 460.0).toDouble(),
                              child: _SelectionQueryPreviewWindow(accent: widget.accent, hotkey: hotkey, query: _selectedFileName, tr: widget.tr),
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

class _SelectionQueryPreviewWindow extends StatelessWidget {
  const _SelectionQueryPreviewWindow({required this.accent, required this.hotkey, required this.query, required this.tr});

  final Color accent;
  final String hotkey;
  final String query;
  final String Function(String key) tr;

  static const _selectedFilePath = r'C:\Users\qianl\Desktop\Quarterly plan.pdf';

  Color _micaSurfaceColor(Color appColor) {
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
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    final textColor = getThemeTextColor();
    final baseBg = getThemeBackgroundColor();
    final effectiveBg = _micaSurfaceColor(baseBg);
    final borderColor = textColor.withValues(alpha: 0.10);
    final queryTop = 12.0;
    final resultTop = queryTop + metrics.queryBoxBaseHeight + 10;
    final footerHeight = WoxThemeUtil.instance.getToolbarHeight();

    return ClipRRect(
      borderRadius: BorderRadius.circular(8),
      child: BackdropFilter(
        filter: ui.ImageFilter.blur(sigmaX: 20, sigmaY: 20),
        child: Stack(
          children: [
            Positioned.fill(child: DecoratedBox(decoration: BoxDecoration(color: effectiveBg, border: Border.all(color: borderColor), borderRadius: BorderRadius.circular(8)))),
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
            Positioned(
              left: 12,
              right: 12,
              top: queryTop,
              child: Container(
                height: metrics.queryBoxBaseHeight,
                padding: const EdgeInsets.symmetric(horizontal: 8),
                decoration: BoxDecoration(color: woxTheme.queryBoxBackgroundColorParsed, borderRadius: BorderRadius.circular(woxTheme.queryBoxBorderRadius.toDouble())),
                child: Row(children: [const Expanded(child: SizedBox.shrink()), Icon(Icons.folder_rounded, size: 20, color: const Color(0xFFFACC15).withValues(alpha: 0.92))]),
              ),
            ),
            Positioned(
              left: 12,
              right: 12,
              top: resultTop,
              bottom: footerHeight,
              child: Row(
                children: [
                  Expanded(
                    flex: 4,
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(8),
                      child: ListView(
                        padding: EdgeInsets.zero,
                        physics: const NeverScrollableScrollPhysics(),
                        children: [
                          _MiniResultRow(
                            title: tr('selection_preview'),
                            subtitle: query,
                            icon: const Icon(Icons.remove_red_eye_outlined, color: Colors.white, size: 23),
                            selected: true,
                            tail: hotkey,
                          ),
                          _MiniResultRow(
                            title: 'Open containing folder',
                            subtitle: 'Open containing folder',
                            icon: Icon(Icons.folder_open_outlined, color: accent, size: 23),
                            tail: 'Enter',
                          ),
                          const _MiniResultRow(title: 'Copy path', subtitle: _selectedFilePath, icon: Icon(Icons.copy_rounded, color: Color(0xFF38BDF8), size: 23), tail: 'Copy'),
                          const _MiniResultRow(
                            title: 'Translate text',
                            subtitle: 'Input tr hello, tr openai hello, tr claude hello',
                            icon: Icon(Icons.translate_rounded, color: Color(0xFF22D3EE), size: 23),
                          ),
                          const _MiniResultRow(
                            title: 'LocalSend device not found',
                            subtitle: 'Ensure target device is running LocalSend',
                            icon: Icon(Icons.radar_rounded, color: Color(0xFF2DD4BF), size: 23),
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                  const Expanded(flex: 6, child: _SelectionQueryPreviewPane()),
                ],
              ),
            ),
            _MiniFooter(accent: accent, hotkey: _demoActionPanelHotkey(), isPressed: false),
          ],
        ),
      ),
    );
  }
}

class _SelectionQueryPreviewPane extends StatelessWidget {
  const _SelectionQueryPreviewPane();

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final subTextColor = getThemeSubTextColor();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Expanded(
          child: Container(
            decoration: BoxDecoration(borderRadius: BorderRadius.circular(10), border: Border.all(color: textColor.withValues(alpha: 0.30))),
            child: Padding(
              padding: const EdgeInsets.all(14),
              child: Container(
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(6),
                  color: Colors.black.withValues(alpha: 0.20),
                  border: Border.all(color: textColor.withValues(alpha: 0.10)),
                ),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Container(height: 6, decoration: BoxDecoration(color: textColor.withValues(alpha: 0.09), borderRadius: BorderRadius.circular(999))),
                      const SizedBox(height: 6),
                      Container(
                        height: 6,
                        margin: const EdgeInsets.only(right: 68),
                        decoration: BoxDecoration(color: textColor.withValues(alpha: 0.09), borderRadius: BorderRadius.circular(999)),
                      ),
                      const SizedBox(height: 10),
                      Expanded(
                        child: Container(
                          decoration: BoxDecoration(
                            color: Colors.black.withValues(alpha: 0.16),
                            borderRadius: BorderRadius.circular(6),
                            border: Border.all(color: textColor.withValues(alpha: 0.08)),
                          ),
                          child: Center(child: Icon(Icons.image_outlined, size: 36, color: textColor.withValues(alpha: 0.42))),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
        const SizedBox(height: 10),
        Row(
          children: [
            Expanded(child: _SelectionMetaChip(value: '2026-05-10 21:22:15', textColor: textColor, borderColor: subTextColor.withValues(alpha: 0.48))),
            const SizedBox(width: 8),
            Expanded(child: _SelectionMetaChip(value: '2026-05-10 21:22:15', textColor: textColor, borderColor: subTextColor.withValues(alpha: 0.48))),
            const SizedBox(width: 8),
            Expanded(child: _SelectionMetaChip(value: '1016 KB', textColor: textColor, borderColor: subTextColor.withValues(alpha: 0.48))),
          ],
        ),
      ],
    );
  }
}

class _SelectionMetaChip extends StatelessWidget {
  const _SelectionMetaChip({required this.value, required this.textColor, required this.borderColor});

  final String value;
  final Color textColor;
  final Color borderColor;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(borderRadius: BorderRadius.circular(999), border: Border.all(color: borderColor)),
      child: Text(
        value,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        textAlign: TextAlign.center,
        style: TextStyle(color: textColor.withValues(alpha: 0.9), fontSize: 12, fontWeight: FontWeight.w600),
      ),
    );
  }
}
