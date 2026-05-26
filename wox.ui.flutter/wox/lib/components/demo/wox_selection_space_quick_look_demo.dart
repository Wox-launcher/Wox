part of 'wox_demo.dart';

// Animated demo for the Selection plugin's Space Quick Look setting.
class WoxSelectionSpaceQuickLookDemo extends StatefulWidget {
  const WoxSelectionSpaceQuickLookDemo({super.key, required this.accent, required this.tr});

  final Color accent;
  final String Function(String key) tr;

  @override
  State<WoxSelectionSpaceQuickLookDemo> createState() => _WoxSelectionSpaceQuickLookDemoState();
}

class _WoxSelectionSpaceQuickLookDemoState extends State<WoxSelectionSpaceQuickLookDemo> with SingleTickerProviderStateMixin {
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
    if (_controller.value < 0.30) {
      return _interval(0.08, 0.30, Curves.easeInOutCubic);
    }
    return 1;
  }

  double _shortcutProgress() {
    if (_controller.value < 0.34) {
      return 0;
    }
    if (_controller.value < 0.44) {
      return _interval(0.34, 0.44, Curves.easeOutCubic);
    }
    if (_controller.value < 0.58) {
      return 1;
    }
    return 1 - _interval(0.58, 0.70, Curves.easeInCubic);
  }

  double _previewProgress() {
    if (_controller.value < 0.50) {
      return 0;
    }
    if (_controller.value < 0.68) {
      return _interval(0.50, 0.68, Curves.easeOutCubic);
    }
    if (_controller.value < 0.92) {
      return 1;
    }
    return 1 - _interval(0.92, 1, Curves.easeInCubic);
  }

  bool _isFileSelected() {
    return _controller.value >= 0.27 && _controller.value < 0.96;
  }

  bool _isShortcutPressed() {
    return _controller.value >= 0.44 && _controller.value <= 0.54;
  }

  @override
  Widget build(BuildContext context) {
    final desktopIsMac = Platform.isMacOS;

    return AnimatedBuilder(
      key: const ValueKey('settings-selection-space-quick-look-demo'),
      animation: _controller,
      builder: (context, child) {
        final cursorProgress = _cursorProgress();
        final shortcutProgress = _shortcutProgress();
        final previewProgress = _previewProgress();
        final fileSelected = _isFileSelected();

        return LayoutBuilder(
          builder: (context, constraints) {
            final startCursor = Offset(constraints.maxWidth - 94, constraints.maxHeight - 86);
            final targetCursor = Offset(188, 116);
            final cursorOffset = Offset.lerp(startCursor, targetCursor, cursorProgress)!;
            final cursorOpacity = 1 - _interval(0.66, 0.82, Curves.easeInCubic);

            return ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Stack(
                children: [
                  Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: desktopIsMac, showDefaultIcons: false)),
                  Padding(
                    padding: _demoDesktopHintContentPadding(),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        WoxDemoHintCard(
                          accent: widget.accent,
                          icon: Icons.space_bar_rounded,
                          title: widget.tr('plugin_selection_setting_enable_space_quick_look'),
                          from: 'Space',
                          to: widget.tr('selection_preview'),
                        ),
                        const Expanded(child: SizedBox.shrink()),
                      ],
                    ),
                  ),
                  Positioned(left: 42, top: 92, child: WoxDemoDesktopFileIcon(label: 'Roadmap.md', icon: Icons.article_outlined, accent: const Color(0xFF60A5FA))),
                  Positioned(
                    left: 150,
                    top: 92,
                    child: WoxDemoDesktopFileIcon(label: _selectedFileName, icon: Icons.picture_as_pdf_outlined, accent: widget.accent, selected: fileSelected),
                  ),
                  Positioned(left: 258, top: 92, child: WoxDemoDesktopFileIcon(label: 'Screenshots', icon: Icons.folder_outlined, accent: const Color(0xFFFACC15))),
                  if (cursorOpacity > 0.01)
                    Positioned(left: cursorOffset.dx, top: cursorOffset.dy, child: Opacity(opacity: cursorOpacity, child: _DemoCursor(accent: widget.accent))),
                  Positioned.fill(
                    child: Opacity(
                      opacity: shortcutProgress,
                      child: Transform.translate(
                        offset: Offset(0, 8 * (1 - shortcutProgress)),
                        child: _HotkeyPressOverlay(hotkey: 'Space', accent: widget.accent, pressed: _isShortcutPressed()),
                      ),
                    ),
                  ),
                  if (previewProgress > 0.01)
                    Positioned.fill(
                      child: Transform.translate(
                        offset: Offset(0, 18 * (1 - previewProgress)),
                        child: Transform.scale(
                          scale: 0.96 + (0.04 * previewProgress),
                          child: Padding(
                            padding: const EdgeInsets.fromLTRB(92, 162, 92, 42),
                            child: _SelectionQuickLookPreviewWindow(accent: widget.accent, fileName: _selectedFileName, tr: widget.tr),
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

class _SelectionQuickLookPreviewWindow extends StatelessWidget {
  const _SelectionQuickLookPreviewWindow({required this.accent, required this.fileName, required this.tr});

  final Color accent;
  final String fileName;
  final String Function(String key) tr;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final background = getThemeBackgroundColor();
    final mutedText = getThemeSubTextColor();

    return Container(
      decoration: BoxDecoration(
        color: background.withValues(alpha: 0.98),
        border: Border.all(color: textColor.withValues(alpha: 0.12)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.20), blurRadius: 32, offset: const Offset(0, 18))],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Stack(
          children: [
            Positioned.fill(
              child: DecoratedBox(
                decoration: BoxDecoration(gradient: RadialGradient(center: const Alignment(0.7, -0.7), radius: 1.1, colors: [accent.withValues(alpha: 0.13), Colors.transparent])),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(18, 16, 18, 16),
              child: Row(
                children: [
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        color: Colors.white.withValues(alpha: 0.94),
                        borderRadius: BorderRadius.circular(6),
                        border: Border.all(color: Colors.black.withValues(alpha: 0.08)),
                      ),
                      padding: const EdgeInsets.fromLTRB(16, 18, 16, 14),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          Row(
                            children: [
                              Container(
                                width: 34,
                                height: 34,
                                decoration: BoxDecoration(color: accent.withValues(alpha: 0.16), borderRadius: BorderRadius.circular(7)),
                                child: Icon(Icons.picture_as_pdf_outlined, color: accent, size: 21),
                              ),
                              const SizedBox(width: 10),
                              Expanded(
                                child: Text(
                                  fileName,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: const TextStyle(color: Color(0xFF1F2937), fontSize: 13, fontWeight: FontWeight.w800),
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: 16),
                          Container(height: 8, decoration: BoxDecoration(color: const Color(0xFFE5E7EB), borderRadius: BorderRadius.circular(999))),
                          const SizedBox(height: 8),
                          Container(
                            height: 8,
                            margin: const EdgeInsets.only(right: 24),
                            decoration: BoxDecoration(color: const Color(0xFFE5E7EB), borderRadius: BorderRadius.circular(999)),
                          ),
                          const SizedBox(height: 8),
                          Container(
                            height: 8,
                            margin: const EdgeInsets.only(right: 52),
                            decoration: BoxDecoration(color: const Color(0xFFE5E7EB), borderRadius: BorderRadius.circular(999)),
                          ),
                          const Spacer(),
                          Row(
                            children: [
                              Expanded(child: Container(height: 46, decoration: BoxDecoration(color: accent.withValues(alpha: 0.14), borderRadius: BorderRadius.circular(5)))),
                              const SizedBox(width: 8),
                              Expanded(child: Container(height: 46, decoration: BoxDecoration(color: const Color(0xFFCBD5E1), borderRadius: BorderRadius.circular(5)))),
                            ],
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(width: 16),
                  SizedBox(
                    width: 132,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Icon(Icons.visibility_outlined, color: accent, size: 20),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Text(
                                tr('selection_preview'),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: TextStyle(color: textColor, fontSize: 14, fontWeight: FontWeight.w800),
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 14),
                        _QuickLookProperty(label: tr('selection_created_at'), value: '09:41', textColor: textColor, mutedText: mutedText),
                        const SizedBox(height: 10),
                        _QuickLookProperty(label: tr('selection_modified_at'), value: 'Today', textColor: textColor, mutedText: mutedText),
                        const SizedBox(height: 10),
                        _QuickLookProperty(label: tr('selection_size'), value: '1.8 MB', textColor: textColor, mutedText: mutedText),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _QuickLookProperty extends StatelessWidget {
  const _QuickLookProperty({required this.label, required this.value, required this.textColor, required this.mutedText});

  final String label;
  final String value;
  final Color textColor;
  final Color mutedText;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: mutedText, fontSize: 10.5, fontWeight: FontWeight.w600)),
        const SizedBox(height: 3),
        Text(value, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 12, fontWeight: FontWeight.w700)),
      ],
    );
  }
}
