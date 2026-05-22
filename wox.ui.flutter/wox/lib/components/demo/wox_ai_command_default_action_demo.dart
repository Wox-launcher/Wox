part of 'wox_demo.dart';

enum WoxAICommandDefaultActionDemoMode { run, runAndShow, runAndPaste }

/// Animated preview for the AI command default action options.
class WoxAICommandDefaultActionDemo extends StatefulWidget {
  const WoxAICommandDefaultActionDemo({super.key, required this.mode, required this.accent, required this.tr});

  final WoxAICommandDefaultActionDemoMode mode;
  final Color accent;
  final String Function(String key) tr;

  @override
  State<WoxAICommandDefaultActionDemo> createState() => _WoxAICommandDefaultActionDemoState();
}

class _WoxAICommandDefaultActionDemoState extends State<WoxAICommandDefaultActionDemo> with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: const Duration(milliseconds: 6200))..repeat();
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

  double _shortcutProgress() {
    if (_controller.value < 0.42) return 0;
    if (_controller.value < 0.52) return _interval(0.42, 0.52, Curves.easeOutCubic);
    if (_controller.value < 0.63) return 1;
    if (_controller.value < 0.72) return 1 - _interval(0.63, 0.72, Curves.easeInCubic);
    return 0;
  }

  double _resultProgress() {
    if (_controller.value < 0.62) return 0;
    if (_controller.value < 0.77) return _interval(0.62, 0.77, Curves.easeOutCubic);
    if (_controller.value < 0.92) return 1;
    return 1 - _interval(0.92, 1.0, Curves.easeInCubic);
  }

  double _typingProgress() {
    if (_controller.value < 0.76) return 0;
    if (_controller.value < 0.88) return _interval(0.76, 0.88, Curves.easeOutCubic);
    return 1;
  }

  double _cursorMoveProgress() {
    if (_controller.value < 0.08) return 0;
    if (_controller.value < 0.22) return _interval(0.08, 0.22, Curves.easeInOutCubic);
    return 1;
  }

  double _selectionProgress() {
    if (_controller.value < 0.22) return 0;
    if (_controller.value < 0.38) return _interval(0.22, 0.38, Curves.easeInOutCubic);
    if (_controller.value < 0.96) return 1;
    return 1 - _interval(0.96, 1.0, Curves.easeInCubic);
  }

  double _cursorOpacity() {
    if (_controller.value < 0.84) return 1;
    return 1 - _interval(0.84, 0.96, Curves.easeInCubic);
  }

  bool _isShortcutPressed() => _controller.value >= 0.52 && _controller.value <= 0.62;

  bool _isReplaced() => widget.mode == WoxAICommandDefaultActionDemoMode.runAndPaste && _controller.value >= 0.62 && _controller.value < 0.96;

  String _hotkey() => _formatDemoHotkey('', fallback: Platform.isMacOS ? 'cmd+shift+t' : 'ctrl+shift+t');

  String _text(String key) => widget.tr(key);

  String _actionLabel() {
    switch (widget.mode) {
      case WoxAICommandDefaultActionDemoMode.run:
        return _text('plugin_ai_command_default_action_run');
      case WoxAICommandDefaultActionDemoMode.runAndShow:
        return _text('plugin_ai_command_default_action_run_and_show');
      case WoxAICommandDefaultActionDemoMode.runAndPaste:
        return _text('plugin_ai_command_default_action_run_and_paste');
    }
  }

  String _actionDescription() {
    switch (widget.mode) {
      case WoxAICommandDefaultActionDemoMode.run:
        return _text('plugin_ai_command_default_action_run_tooltip');
      case WoxAICommandDefaultActionDemoMode.runAndShow:
        return _text('plugin_ai_command_default_action_run_and_show_tooltip');
      case WoxAICommandDefaultActionDemoMode.runAndPaste:
        return _text('plugin_ai_command_default_action_run_and_paste_tooltip');
    }
  }

  String _revealed(String text, double progress) {
    final end = (text.length * progress).round().clamp(0, text.length).toInt();
    return text.substring(0, end);
  }

  String _sourceText() {
    switch (widget.mode) {
      case WoxAICommandDefaultActionDemoMode.run:
        return _text('plugin_ai_command_default_action_demo_run_source');
      case WoxAICommandDefaultActionDemoMode.runAndShow:
        return _text('plugin_ai_command_default_action_demo_show_source');
      case WoxAICommandDefaultActionDemoMode.runAndPaste:
        return _text('plugin_ai_command_default_action_demo_paste_source');
    }
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      key: ValueKey('ai-command-default-action-demo-${widget.mode}'),
      animation: _controller,
      builder: (context, child) {
        final shortcutProgress = _shortcutProgress();
        final resultProgress = _resultProgress();
        final typingProgress = _typingProgress();
        final selectionProgress = _isReplaced() ? 0.0 : _selectionProgress();
        final sourceText = _sourceText();
        final displayedText = _isReplaced() ? _text('plugin_ai_command_default_action_demo_translation_answer') : sourceText;

        return LayoutBuilder(
          builder: (context, constraints) {
            final statusTop = Platform.isMacOS ? 42.0 : 20.0;
            final sceneTop = Platform.isMacOS ? 104.0 : 82.0;
            final documentLeft = 28.0;
            final documentWidth = (constraints.maxWidth * 0.48).clamp(220.0, 290.0).toDouble();
            final cursorStart = Offset(constraints.maxWidth - 76, constraints.maxHeight - 82);
            final cursorSelectionStart = Offset(documentLeft + 52, sceneTop + 112);
            final cursorSelectionEnd = Offset(documentLeft + documentWidth - 70, sceneTop + 140);
            final cursorMoveOffset = Offset.lerp(cursorStart, cursorSelectionStart, _cursorMoveProgress())!;
            final cursorOffset = Offset.lerp(cursorMoveOffset, cursorSelectionEnd, selectionProgress)!;

            return ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Stack(
                children: [
                  Positioned.fill(child: WoxDemoDesktopBackground(accent: widget.accent, isMac: Platform.isMacOS, showDefaultIcons: false)),
                  Positioned(left: 24, right: 24, top: statusTop, child: _AICommandDemoStatusBar(accent: widget.accent, title: _actionLabel(), description: _actionDescription())),
                  Positioned(
                    left: documentLeft,
                    top: sceneTop,
                    width: documentWidth,
                    bottom: 42,
                    child: _AICommandDemoDocument(
                      accent: widget.accent,
                      title: _text('plugin_ai_command_preview_selected_text'),
                      text: displayedText,
                      selectionProgress: selectionProgress,
                      replaced: _isReplaced(),
                    ),
                  ),
                  Positioned.fill(
                    child: Opacity(
                      opacity: shortcutProgress,
                      child: Transform.translate(
                        offset: Offset(0, 8 * (1 - shortcutProgress)),
                        child: _HotkeyPressOverlay(hotkey: _hotkey(), accent: widget.accent, pressed: _isShortcutPressed()),
                      ),
                    ),
                  ),
                  if (widget.mode == WoxAICommandDefaultActionDemoMode.run && resultProgress > 0.01)
                    Positioned(
                      right: 28,
                      top: sceneTop + 14,
                      bottom: 42,
                      width: (constraints.maxWidth * 0.47).clamp(230.0, 300.0).toDouble(),
                      child: _AnimatedDemoSurface(progress: resultProgress, child: _AICommandRunPreview(accent: widget.accent, tr: widget.tr, answerProgress: typingProgress)),
                    ),
                  if (widget.mode == WoxAICommandDefaultActionDemoMode.runAndShow && resultProgress > 0.01)
                    Positioned(
                      right: 32,
                      top: sceneTop + 42,
                      width: (constraints.maxWidth * 0.44).clamp(220.0, 280.0).toDouble(),
                      child: _AnimatedDemoSurface(
                        progress: resultProgress,
                        child: _AICommandFloatingAnswer(accent: widget.accent, text: _revealed(_text('plugin_ai_command_default_action_demo_overlay_answer'), typingProgress)),
                      ),
                    ),
                  if (widget.mode == WoxAICommandDefaultActionDemoMode.runAndPaste && resultProgress > 0.01)
                    Positioned(
                      right: 40,
                      top: sceneTop + 50,
                      child: Opacity(
                        opacity: resultProgress,
                        child: Transform.scale(scale: 0.9 + (0.1 * resultProgress), child: Icon(Icons.auto_fix_high_rounded, color: widget.accent, size: 44)),
                      ),
                    ),
                  Positioned(left: cursorOffset.dx, top: cursorOffset.dy, child: Opacity(opacity: _cursorOpacity(), child: _DemoCursor(accent: widget.accent))),
                ],
              ),
            );
          },
        );
      },
    );
  }
}

class _AICommandDemoStatusBar extends StatelessWidget {
  const _AICommandDemoStatusBar({required this.accent, required this.title, required this.description});

  final Color accent;
  final String title;
  final String description;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.92),
        border: Border.all(color: textColor.withValues(alpha: 0.10)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.12), blurRadius: 22, offset: const Offset(0, 10))],
      ),
      child: Row(
        children: [
          Icon(Icons.play_circle_outline_rounded, color: accent, size: 20),
          const SizedBox(width: 10),
          Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 13, fontWeight: FontWeight.w800)),
          const SizedBox(width: 12),
          Expanded(
            child: Text(description, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeSubTextColor(), fontSize: 11, fontWeight: FontWeight.w600)),
          ),
        ],
      ),
    );
  }
}

class _AnimatedDemoSurface extends StatelessWidget {
  const _AnimatedDemoSurface({required this.progress, required this.child});

  final double progress;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Opacity(opacity: progress, child: Transform.translate(offset: Offset(0, 18 * (1 - progress)), child: Transform.scale(scale: 0.96 + (0.04 * progress), child: child)));
  }
}

class _AICommandDemoDocument extends StatelessWidget {
  const _AICommandDemoDocument({required this.accent, required this.title, required this.text, required this.selectionProgress, required this.replaced});

  final Color accent;
  final String title;
  final String text;
  final double selectionProgress;
  final bool replaced;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final surfaceColor = getThemeBackgroundColor().withValues(alpha: 0.88);
    final selected = selectionProgress > 0.01;

    return DecoratedBox(
      decoration: BoxDecoration(
        color: surfaceColor,
        border: Border.all(color: textColor.withValues(alpha: 0.11)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.18), blurRadius: 22, offset: const Offset(0, 12))],
      ),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(replaced ? Icons.check_circle_rounded : Icons.subject_rounded, color: replaced ? accent : textColor.withValues(alpha: 0.72), size: 18),
                const SizedBox(width: 8),
                Expanded(child: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: textColor, fontSize: 12, fontWeight: FontWeight.w800))),
              ],
            ),
            const SizedBox(height: 12),
            Expanded(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final textStyle = TextStyle(color: textColor.withValues(alpha: 0.92), fontSize: 12, height: 1.42, fontWeight: FontWeight.w600);

                  return DecoratedBox(
                    decoration: BoxDecoration(
                      color: textColor.withValues(alpha: 0.045),
                      border: Border.all(color: selected ? accent.withValues(alpha: 0.55) : textColor.withValues(alpha: 0.10)),
                      borderRadius: BorderRadius.circular(7),
                    ),
                    child: ClipRRect(
                      borderRadius: BorderRadius.circular(7),
                      child: Stack(
                        children: [
                          Positioned.fill(
                            child: CustomPaint(painter: _AICommandTextSelectionPainter(text: text, textStyle: textStyle, accent: accent, progress: selectionProgress)),
                          ),
                          Padding(padding: const EdgeInsets.all(12), child: Text(text, maxLines: 8, overflow: TextOverflow.ellipsis, style: textStyle)),
                        ],
                      ),
                    ),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _AICommandRunPreview extends StatelessWidget {
  const _AICommandRunPreview({required this.accent, required this.tr, required this.answerProgress});

  final Color accent;
  final String Function(String key) tr;
  final double answerProgress;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final answer = tr('plugin_ai_command_default_action_demo_summary_answer');
    final visibleAnswer = answer.substring(0, (answer.length * answerProgress).round().clamp(0, answer.length).toInt());

    return DecoratedBox(
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.96),
        border: Border.all(color: textColor.withValues(alpha: 0.12)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.22), blurRadius: 26, offset: const Offset(0, 14))],
      ),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Container(
              height: 34,
              padding: const EdgeInsets.symmetric(horizontal: 10),
              decoration: BoxDecoration(color: textColor.withValues(alpha: 0.06), borderRadius: BorderRadius.circular(6)),
              alignment: Alignment.centerLeft,
              child: Text(
                'ai summarize {wox:selected_text}',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: textColor, fontSize: 11, fontWeight: FontWeight.w700),
              ),
            ),
            const SizedBox(height: 10),
            Expanded(
              child: Row(
                children: [
                  SizedBox(
                    width: 88,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        _AICommandDemoResultChip(accent: accent, title: tr('plugin_ai_command_default_action_run'), selected: true),
                        const SizedBox(height: 8),
                        _AICommandDemoResultChip(accent: accent, title: tr('plugin_ai_command_copy'), selected: false),
                      ],
                    ),
                  ),
                  const SizedBox(width: 10),
                  Expanded(
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: textColor.withValues(alpha: 0.045),
                        borderRadius: BorderRadius.circular(7),
                        border: Border.all(color: textColor.withValues(alpha: 0.09)),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.all(10),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(tr('plugin_ai_command_preview_answer'), style: TextStyle(color: accent, fontSize: 11, fontWeight: FontWeight.w800)),
                            const SizedBox(height: 8),
                            Text(
                              visibleAnswer,
                              maxLines: 7,
                              overflow: TextOverflow.ellipsis,
                              style: TextStyle(color: textColor.withValues(alpha: 0.88), fontSize: 11, height: 1.38),
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
    );
  }
}

class _AICommandTextSelectionPainter extends CustomPainter {
  const _AICommandTextSelectionPainter({required this.text, required this.textStyle, required this.accent, required this.progress});

  final String text;
  final TextStyle textStyle;
  final Color accent;
  final double progress;

  @override
  void paint(Canvas canvas, Size size) {
    final selectionProgress = progress.clamp(0.0, 1.0);
    if (selectionProgress <= 0) {
      return;
    }

    const textPadding = 12.0;
    final painter = TextPainter(text: TextSpan(text: text, style: textStyle), maxLines: 8, ellipsis: '...', textDirection: TextDirection.ltr)
      ..layout(maxWidth: (size.width - (textPadding * 2)).clamp(0.0, double.infinity));

    final selectedOffset = (text.length * selectionProgress).round().clamp(0, text.length).toInt();
    if (selectedOffset <= 0) {
      return;
    }

    final selection = TextSelection(baseOffset: 0, extentOffset: selectedOffset);
    final boxes = painter.getBoxesForSelection(selection);
    final paint = Paint()..color = accent.withValues(alpha: 0.22);

    for (final box in boxes) {
      final rect = Rect.fromLTRB(box.left + textPadding - 2, box.top + textPadding + 1, box.right + textPadding + 2, box.bottom + textPadding - 1);
      canvas.drawRRect(RRect.fromRectAndRadius(rect, const Radius.circular(3)), paint);
    }
  }

  @override
  bool shouldRepaint(covariant _AICommandTextSelectionPainter oldDelegate) {
    return oldDelegate.text != text || oldDelegate.textStyle != textStyle || oldDelegate.accent != accent || oldDelegate.progress != progress;
  }
}

class _AICommandDemoResultChip extends StatelessWidget {
  const _AICommandDemoResultChip({required this.accent, required this.title, required this.selected});

  final Color accent;
  final String title;
  final bool selected;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();

    return Container(
      height: 42,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      decoration: BoxDecoration(
        color: selected ? accent.withValues(alpha: 0.18) : textColor.withValues(alpha: 0.045),
        border: Border.all(color: selected ? accent.withValues(alpha: 0.42) : textColor.withValues(alpha: 0.08)),
        borderRadius: BorderRadius.circular(6),
      ),
      alignment: Alignment.centerLeft,
      child: Text(
        title,
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: selected ? accent : textColor.withValues(alpha: 0.72), fontSize: 10, fontWeight: FontWeight.w800),
      ),
    );
  }
}

class _AICommandFloatingAnswer extends StatelessWidget {
  const _AICommandFloatingAnswer({required this.accent, required this.text});

  final Color accent;
  final String text;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();

    return DecoratedBox(
      decoration: BoxDecoration(
        color: getThemeBackgroundColor().withValues(alpha: 0.97),
        border: Border.all(color: accent.withValues(alpha: 0.32)),
        borderRadius: BorderRadius.circular(8),
        boxShadow: [
          BoxShadow(color: Colors.black.withValues(alpha: 0.28), blurRadius: 28, offset: const Offset(0, 16)),
          BoxShadow(color: accent.withValues(alpha: 0.10), blurRadius: 28),
        ],
      ),
      child: Padding(
        padding: const EdgeInsets.all(13),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 9,
                  height: 9,
                  decoration: BoxDecoration(color: accent, shape: BoxShape.circle, boxShadow: [BoxShadow(color: accent.withValues(alpha: 0.45), blurRadius: 12)]),
                ),
                const SizedBox(width: 8),
                Text('Wox AI', style: TextStyle(color: textColor, fontSize: 12, fontWeight: FontWeight.w800)),
              ],
            ),
            const SizedBox(height: 9),
            Text(
              text,
              maxLines: 7,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: textColor.withValues(alpha: 0.90), fontSize: 11, height: 1.42, fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    );
  }
}
