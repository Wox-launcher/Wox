import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview_media.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxMediaPreviewView extends StatefulWidget {
  final WoxPreviewMedia data;
  final WoxTheme woxTheme;
  final String previousTooltip;
  final String toggleTooltip;
  final String nextTooltip;
  final VoidCallback? onPrevious;
  final VoidCallback? onToggle;
  final VoidCallback? onNext;

  const WoxMediaPreviewView({
    super.key,
    required this.data,
    required this.woxTheme,
    required this.previousTooltip,
    required this.toggleTooltip,
    required this.nextTooltip,
    this.onPrevious,
    this.onToggle,
    this.onNext,
  });

  @override
  State<WoxMediaPreviewView> createState() => _WoxMediaPreviewViewState();
}

class _WoxMediaPreviewViewState extends State<WoxMediaPreviewView> with TickerProviderStateMixin {
  static const _accent = Color(0xFFFF6B35);
  late final AnimationController _recordController;
  late final AnimationController _tonearmController;
  late WoxPreviewMedia _displayedRecord;
  int _trackAnimationGeneration = 0;

  @override
  void initState() {
    super.initState();
    _recordController = AnimationController(vsync: this, duration: const Duration(seconds: 12));
    _tonearmController = AnimationController(vsync: this, duration: const Duration(milliseconds: 480), value: widget.data.isPlaying ? 1 : 0);
    _displayedRecord = widget.data;
    _syncPlaybackAnimation();
  }

  @override
  void didUpdateWidget(covariant WoxMediaPreviewView oldWidget) {
    super.didUpdateWidget(oldWidget);
    final playbackChanged = oldWidget.data.isPlaying != widget.data.isPlaying;
    final trackChanged = oldWidget.data.trackIdentity != widget.data.trackIdentity;
    if (playbackChanged) {
      _syncPlaybackAnimation();
    }
    if (trackChanged) {
      unawaited(_animateTrackChange(widget.data));
    } else if (playbackChanged) {
      _tonearmController.animateTo(widget.data.isPlaying ? 1 : 0, curve: Curves.easeInOutCubic);
    }
  }

  // Lift the tonearm before swapping records, then lower it again only if playback continues.
  Future<void> _animateTrackChange(WoxPreviewMedia nextRecord) async {
    final generation = ++_trackAnimationGeneration;
    await _tonearmController.animateTo(0, duration: const Duration(milliseconds: 240), curve: Curves.easeOutCubic);
    if (!mounted || generation != _trackAnimationGeneration) {
      return;
    }

    setState(() => _displayedRecord = nextRecord);
    await Future<void>.delayed(const Duration(milliseconds: 440));
    if (!mounted || generation != _trackAnimationGeneration || !widget.data.isPlaying) {
      return;
    }
    await _tonearmController.animateTo(1, duration: const Duration(milliseconds: 360), curve: Curves.easeInOutCubic);
  }

  // Keep the current turn value when paused so playback resumes from the same visual position.
  void _syncPlaybackAnimation() {
    if (widget.data.isPlaying) {
      _recordController.repeat();
    } else {
      _recordController.stop();
    }
  }

  @override
  void dispose() {
    _recordController.dispose();
    _tonearmController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final fontColor = safeFromCssColor(widget.woxTheme.previewFontColor);
    final secondaryColor = safeFromCssColor(widget.woxTheme.previewPropertyContentColor, defaultColor: fontColor).withValues(alpha: 0.72);
    final borderColor = safeFromCssColor(widget.woxTheme.previewSplitLineColor).withValues(alpha: 0.38);

    return Padding(
      padding: EdgeInsets.fromLTRB(metrics.scaledSpacing(18), metrics.scaledSpacing(16), metrics.scaledSpacing(16), metrics.scaledSpacing(14)),
      child: DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(18),
          border: Border.all(color: borderColor),
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [_accent.withValues(alpha: 0.105), fontColor.withValues(alpha: 0.025), fontColor.withValues(alpha: 0.045)],
            stops: const [0, 0.52, 1],
          ),
          boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.08), blurRadius: 24, offset: const Offset(0, 10))],
        ),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(18),
          child: LayoutBuilder(
            builder: (context, constraints) {
              final wideLayout = constraints.maxWidth >= metrics.scaledSpacing(690) && constraints.maxHeight >= metrics.scaledSpacing(330);
              final horizontalPadding = metrics.scaledSpacing(wideLayout ? 34 : 22);
              final verticalPadding = metrics.scaledSpacing(wideLayout ? 28 : 20);

              if (wideLayout) {
                return Padding(
                  padding: EdgeInsets.symmetric(horizontal: horizontalPadding, vertical: verticalPadding),
                  child: Row(
                    children: [
                      Expanded(flex: 11, child: _buildRecordStage(constraints.maxHeight - verticalPadding * 2, fontColor)),
                      SizedBox(width: metrics.scaledSpacing(38)),
                      Expanded(flex: 10, child: _buildTrackDetails(fontColor, secondaryColor, compact: false)),
                    ],
                  ),
                );
              }

              return Padding(
                padding: EdgeInsets.symmetric(horizontal: horizontalPadding, vertical: verticalPadding),
                child: Column(
                  children: [
                    Expanded(flex: 6, child: _buildRecordStage(constraints.maxHeight * 0.48, fontColor)),
                    SizedBox(height: metrics.scaledSpacing(14)),
                    Expanded(flex: 5, child: _buildTrackDetails(fontColor, secondaryColor, compact: true)),
                  ],
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  // The record stage keeps the artwork, vinyl grooves, and tonearm as separate layers.
  Widget _buildRecordStage(double availableHeight, Color fontColor) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return LayoutBuilder(
      builder: (context, constraints) {
        final maxSize = math.min(constraints.maxWidth, math.min(constraints.maxHeight, availableHeight));
        final recordSize = math.min(maxSize, metrics.scaledSpacing(330));

        return Center(
          child: SizedBox(
            width: recordSize,
            height: recordSize,
            child: Stack(
              clipBehavior: Clip.none,
              children: [
                Positioned.fill(
                  child: AnimatedSwitcher(
                    duration: const Duration(milliseconds: 420),
                    switchInCurve: Curves.easeOutCubic,
                    switchOutCurve: Curves.easeInCubic,
                    layoutBuilder: (currentChild, previousChildren) => Stack(alignment: Alignment.center, children: [...previousChildren, if (currentChild != null) currentChild]),
                    transitionBuilder: (child, animation) {
                      final scale = Tween<double>(begin: 0.72, end: 1).animate(animation);
                      final turn = Tween<double>(begin: -0.035, end: 0).animate(animation);
                      return FadeTransition(opacity: animation, child: ScaleTransition(scale: scale, child: RotationTransition(turns: turn, child: child)));
                    },
                    child: RotationTransition(
                      key: ValueKey(_displayedRecord.trackIdentity),
                      turns: _recordController,
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          gradient: const RadialGradient(colors: [Color(0xFF343438), Color(0xFF111114), Color(0xFF050506)], stops: [0, 0.54, 1]),
                          boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.34), blurRadius: 28, spreadRadius: 2, offset: const Offset(0, 12))],
                        ),
                        child: CustomPaint(
                          painter: _VinylGroovePainter(highlightColor: fontColor.withValues(alpha: 0.16)),
                          child: Center(child: _buildArtwork(_displayedRecord, recordSize * 0.43)),
                        ),
                      ),
                    ),
                  ),
                ),
                Positioned.fill(
                  child: IgnorePointer(
                    child: AnimatedBuilder(
                      animation: _tonearmController,
                      builder: (context, child) => CustomPaint(painter: _TonearmPainter(playbackProgress: _tonearmController.value)),
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

  Widget _buildArtwork(WoxPreviewMedia record, double size) {
    final artwork = WoxImage.parse(record.artwork);
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(shape: BoxShape.circle, color: _accent.withValues(alpha: 0.18), border: Border.all(color: Colors.white.withValues(alpha: 0.18), width: 1)),
      child: ClipOval(
        child: artwork == null ? Icon(Icons.music_note_rounded, color: _accent, size: size * 0.42) : WoxImageView(woxImage: artwork, width: size, height: size, fit: BoxFit.cover),
      ),
    );
  }

  // Track details scale down as one block on unusually short launcher windows.
  Widget _buildTrackDetails(Color fontColor, Color secondaryColor, {required bool compact}) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final progress = widget.data.duration <= 0 ? 0.0 : (widget.data.position / widget.data.duration).clamp(0.0, 1.0);
    final source = widget.data.appName.trim();

    return LayoutBuilder(
      builder: (context, constraints) {
        final titleSize = metrics.scaledSpacing(compact ? 22 : 30);
        final content = Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: compact ? CrossAxisAlignment.center : CrossAxisAlignment.start,
          children: [
            _buildPlaybackStatus(fontColor, source),
            SizedBox(height: metrics.scaledSpacing(compact ? 10 : 22)),
            Text(
              widget.data.title,
              maxLines: compact ? 1 : 3,
              overflow: TextOverflow.ellipsis,
              textAlign: compact ? TextAlign.center : TextAlign.left,
              style: TextStyle(color: fontColor, fontSize: titleSize, height: 1.14, fontWeight: FontWeight.w700, letterSpacing: -0.35),
            ),
            if (widget.data.artist.trim().isNotEmpty) ...[
              SizedBox(height: metrics.scaledSpacing(9)),
              Text(
                widget.data.artist,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                textAlign: compact ? TextAlign.center : TextAlign.left,
                style: TextStyle(color: fontColor.withValues(alpha: 0.78), fontSize: metrics.scaledSpacing(compact ? 14 : 17), fontWeight: FontWeight.w500),
              ),
            ],
            if (widget.data.album.trim().isNotEmpty && !compact) ...[
              SizedBox(height: metrics.scaledSpacing(5)),
              Text(widget.data.album, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: secondaryColor, fontSize: metrics.scaledSpacing(13))),
            ],
            SizedBox(height: metrics.scaledSpacing(compact ? 14 : 30)),
            ClipRRect(
              borderRadius: BorderRadius.circular(999),
              child: LinearProgressIndicator(
                minHeight: metrics.scaledSpacing(5),
                value: progress,
                backgroundColor: fontColor.withValues(alpha: 0.12),
                valueColor: const AlwaysStoppedAnimation(_accent),
              ),
            ),
            SizedBox(height: metrics.scaledSpacing(8)),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  _formatDuration(widget.data.position),
                  style: TextStyle(color: secondaryColor, fontSize: metrics.scaledSpacing(12), fontFeatures: const [FontFeature.tabularFigures()]),
                ),
                Text(
                  _formatDuration(widget.data.duration),
                  style: TextStyle(color: secondaryColor, fontSize: metrics.scaledSpacing(12), fontFeatures: const [FontFeature.tabularFigures()]),
                ),
              ],
            ),
            SizedBox(height: metrics.scaledSpacing(compact ? 14 : 22)),
            _buildPlaybackControls(fontColor),
          ],
        );
        return Align(
          alignment: compact ? Alignment.center : Alignment.centerLeft,
          child: FittedBox(fit: BoxFit.scaleDown, alignment: compact ? Alignment.center : Alignment.centerLeft, child: SizedBox(width: constraints.maxWidth, child: content)),
        );
      },
    );
  }

  Widget _buildPlaybackControls(Color fontColor) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return Align(
      alignment: Alignment.center,
      child: Container(
        width: metrics.scaledSpacing(176),
        height: metrics.scaledSpacing(48),
        padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(8)),
        decoration: BoxDecoration(
          color: fontColor.withValues(alpha: 0.035),
          borderRadius: BorderRadius.circular(999),
          border: Border.all(color: fontColor.withValues(alpha: 0.10)),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            _MediaControlButton(tooltip: widget.previousTooltip, icon: Icons.skip_previous_rounded, color: fontColor.withValues(alpha: 0.72), onPressed: widget.onPrevious),
            _MediaControlButton(
              tooltip: widget.toggleTooltip,
              icon: widget.data.isPlaying ? Icons.pause_rounded : Icons.play_arrow_rounded,
              color: _accent,
              emphasized: true,
              onPressed: widget.onToggle,
            ),
            _MediaControlButton(tooltip: widget.nextTooltip, icon: Icons.skip_next_rounded, color: fontColor.withValues(alpha: 0.72), onPressed: widget.onNext),
          ],
        ),
      ),
    );
  }

  Widget _buildPlaybackStatus(Color fontColor, String source) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    return Container(
      height: metrics.scaledSpacing(28),
      padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(10)),
      decoration: BoxDecoration(color: fontColor.withValues(alpha: 0.055), borderRadius: BorderRadius.circular(999), border: Border.all(color: fontColor.withValues(alpha: 0.09))),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(widget.data.isPlaying ? Icons.graphic_eq_rounded : Icons.pause_rounded, size: metrics.scaledSpacing(15), color: _accent),
          if (source.isNotEmpty) ...[
            SizedBox(width: metrics.scaledSpacing(7)),
            ConstrainedBox(
              constraints: BoxConstraints(maxWidth: metrics.scaledSpacing(180)),
              child: Text(
                source,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: fontColor.withValues(alpha: 0.74), fontSize: metrics.scaledSpacing(11), fontWeight: FontWeight.w600),
              ),
            ),
          ],
        ],
      ),
    );
  }

  String _formatDuration(int seconds) {
    final safeSeconds = math.max(0, seconds);
    final hours = safeSeconds ~/ 3600;
    final minutes = (safeSeconds % 3600) ~/ 60;
    final remainingSeconds = safeSeconds % 60;
    if (hours > 0) {
      return "$hours:${minutes.toString().padLeft(2, "0")}:${remainingSeconds.toString().padLeft(2, "0")}";
    }
    return "$minutes:${remainingSeconds.toString().padLeft(2, "0")}";
  }
}

class _MediaControlButton extends StatefulWidget {
  final String tooltip;
  final IconData icon;
  final Color color;
  final bool emphasized;
  final VoidCallback? onPressed;

  const _MediaControlButton({required this.tooltip, required this.icon, required this.color, required this.onPressed, this.emphasized = false});

  @override
  State<_MediaControlButton> createState() => _MediaControlButtonState();
}

class _MediaControlButtonState extends State<_MediaControlButton> {
  bool _isHovered = false;
  bool _isPressed = false;

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final enabled = widget.onPressed != null;
    final buttonSize = metrics.scaledSpacing(widget.emphasized ? 36 : 34);
    final baseAlpha = widget.emphasized ? 0.13 : 0.0;
    final hoverAlpha = widget.emphasized ? 0.22 : 0.08;
    final backgroundColor = widget.color.withValues(alpha: _isHovered ? hoverAlpha : baseAlpha);

    return WoxTooltip(
      message: widget.tooltip,
      child: Semantics(
        button: true,
        enabled: enabled,
        label: widget.tooltip,
        child: MouseRegion(
          cursor: enabled ? SystemMouseCursors.click : SystemMouseCursors.basic,
          onEnter: enabled ? (_) => setState(() => _isHovered = true) : null,
          onExit:
              enabled
                  ? (_) => setState(() {
                    _isHovered = false;
                    _isPressed = false;
                  })
                  : null,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: widget.onPressed,
            onTapDown: enabled ? (_) => setState(() => _isPressed = true) : null,
            onTapUp: enabled ? (_) => setState(() => _isPressed = false) : null,
            onTapCancel: enabled ? () => setState(() => _isPressed = false) : null,
            child: AnimatedScale(
              duration: const Duration(milliseconds: 100),
              scale: _isPressed ? 0.92 : 1,
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 140),
                width: buttonSize,
                height: buttonSize,
                decoration: BoxDecoration(shape: BoxShape.circle, color: backgroundColor),
                child: Icon(widget.icon, size: metrics.scaledSpacing(widget.emphasized ? 20 : 18), color: enabled ? widget.color : widget.color.withValues(alpha: 0.32)),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _VinylGroovePainter extends CustomPainter {
  final Color highlightColor;

  const _VinylGroovePainter({required this.highlightColor});

  @override
  void paint(Canvas canvas, Size size) {
    final center = size.center(Offset.zero);
    final radius = size.shortestSide / 2;
    final groovePaint = Paint()..style = PaintingStyle.stroke;
    for (var i = 0; i < 9; i++) {
      groovePaint
        ..color = Colors.white.withValues(alpha: i.isEven ? 0.055 : 0.025)
        ..strokeWidth = 0.8;
      canvas.drawCircle(center, radius * (0.5 + i * 0.052), groovePaint);
    }

    final shinePaint =
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeCap = StrokeCap.round
          ..strokeWidth = radius * 0.055
          ..color = highlightColor;
    canvas.drawArc(Rect.fromCircle(center: center, radius: radius * 0.82), -2.55, 0.72, false, shinePaint);
  }

  @override
  bool shouldRepaint(covariant _VinylGroovePainter oldDelegate) => oldDelegate.highlightColor != highlightColor;
}

class _TonearmPainter extends CustomPainter {
  final double playbackProgress;

  const _TonearmPainter({required this.playbackProgress});

  @override
  void paint(Canvas canvas, Size size) {
    final scale = size.shortestSide;
    final pivot = Offset(size.width * 0.80, size.height * 0.135);
    final parkedNeedle = Offset(size.width * 0.84, size.height * 0.33);
    final playingNeedle = Offset(size.width * 0.69, size.height * 0.34);
    final needle = Offset.lerp(parkedNeedle, playingNeedle, playbackProgress)!;
    final armVector = needle - pivot;
    if (armVector.distance == 0) {
      return;
    }

    final direction = armVector / armVector.distance;
    final perpendicular = Offset(-direction.dy, direction.dx);
    final headshellStart = needle - direction * scale * 0.044;
    final armLength = (headshellStart - pivot).distance;
    final control = pivot + direction * armLength * 0.52 - perpendicular * scale * 0.006;
    final armPath =
        Path()
          ..moveTo(pivot.dx, pivot.dy)
          ..quadraticBezierTo(control.dx, control.dy, headshellStart.dx, headshellStart.dy);
    final shadowOffset = Offset(scale * 0.004, scale * 0.007);

    canvas.drawPath(
      armPath.shift(shadowOffset),
      Paint()
        ..color = Colors.black.withValues(alpha: 0.44)
        ..style = PaintingStyle.stroke
        ..strokeCap = StrokeCap.round
        ..strokeWidth = scale * 0.017,
    );

    canvas.drawPath(
      armPath,
      Paint()
        ..shader = const LinearGradient(colors: [Color(0xFF8F8A84), Color(0xFFE3DED7), Color(0xFFAAA39B)]).createShader(Rect.fromPoints(pivot, headshellStart))
        ..style = PaintingStyle.stroke
        ..strokeCap = StrokeCap.round
        ..strokeWidth = scale * 0.0105,
    );
    canvas.drawPath(
      armPath,
      Paint()
        ..color = Colors.white.withValues(alpha: 0.38)
        ..style = PaintingStyle.stroke
        ..strokeCap = StrokeCap.round
        ..strokeWidth = scale * 0.0024,
    );

    final shellWidth = scale * 0.0115;
    final headshellEnd = needle - direction * scale * 0.007;
    final headshell =
        Path()
          ..moveTo((headshellStart + perpendicular * shellWidth).dx, (headshellStart + perpendicular * shellWidth).dy)
          ..lineTo((headshellEnd + perpendicular * shellWidth * 0.58).dx, (headshellEnd + perpendicular * shellWidth * 0.58).dy)
          ..lineTo((headshellEnd - perpendicular * shellWidth * 0.58).dx, (headshellEnd - perpendicular * shellWidth * 0.58).dy)
          ..lineTo((headshellStart - perpendicular * shellWidth).dx, (headshellStart - perpendicular * shellWidth).dy)
          ..close();
    canvas.drawPath(headshell.shift(shadowOffset * 0.55), Paint()..color = Colors.black.withValues(alpha: 0.42));
    canvas.drawPath(headshell, Paint()..shader = const LinearGradient(colors: [Color(0xFF3C3B3A), Color(0xFF76716C)]).createShader(Rect.fromPoints(headshellStart, headshellEnd)));

    final cartridgeCenter = headshellEnd - direction * scale * 0.004;
    canvas.drawCircle(cartridgeCenter, scale * 0.0065, Paint()..color = const Color(0xFFB84A36));
    canvas.drawLine(
      cartridgeCenter,
      needle,
      Paint()
        ..color = const Color(0xFFB8B1A8)
        ..strokeCap = StrokeCap.round
        ..strokeWidth = scale * 0.0022,
    );
    canvas.drawCircle(needle, scale * 0.0036, Paint()..color = const Color(0xFFEE7048));

    canvas.drawCircle(pivot + shadowOffset, scale * 0.038, Paint()..color = Colors.black.withValues(alpha: 0.42));
    canvas.drawCircle(
      pivot,
      scale * 0.035,
      Paint()..shader = const RadialGradient(colors: [Color(0xFF85807A), Color(0xFF4B4845)]).createShader(Rect.fromCircle(center: pivot, radius: scale * 0.035)),
    );
    canvas.drawCircle(pivot, scale * 0.019, Paint()..color = const Color(0xFFC8BDAE));
    canvas.drawCircle(pivot - Offset(scale * 0.004, scale * 0.005), scale * 0.007, Paint()..color = Colors.white.withValues(alpha: 0.22));
  }

  @override
  bool shouldRepaint(covariant _TonearmPainter oldDelegate) => oldDelegate.playbackProgress != playbackProgress;
}
