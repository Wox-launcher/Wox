part of 'wox_demo.dart';

class WoxDemoPopover extends StatefulWidget {
  const WoxDemoPopover({
    super.key,
    required this.child,
    required this.demo,
    required this.popoverKey,
    this.width = 560,
    this.height = 400,
    this.margin = 12,
    this.showDelay = const Duration(milliseconds: 120),
    this.hideDelay = const Duration(milliseconds: 160),
  });

  final Widget child;
  final Widget demo;
  final Key popoverKey;
  final double width;
  final double height;
  final double margin;
  final Duration showDelay;
  final Duration hideDelay;

  @override
  State<WoxDemoPopover> createState() => _WoxDemoPopoverState();
}

class _WoxDemoPopoverState extends State<WoxDemoPopover> {
  final LayerLink _layerLink = LayerLink();
  OverlayEntry? _entry;
  Timer? _showTimer;
  Timer? _hideTimer;
  bool _hoveringTarget = false;
  bool _hoveringPopover = false;

  @override
  void dispose() {
    _showTimer?.cancel();
    _hideTimer?.cancel();
    _removeEntry();
    super.dispose();
  }

  void _scheduleShow() {
    _hoveringTarget = true;
    _hideTimer?.cancel();
    _showTimer?.cancel();
    _showTimer = Timer(widget.showDelay, () => unawaited(_showEntry()));
  }

  void _scheduleHide({required bool fromPopover}) {
    if (fromPopover) {
      _hoveringPopover = false;
    } else {
      _hoveringTarget = false;
    }
    _showTimer?.cancel();
    _hideTimer?.cancel();
    _hideTimer = Timer(widget.hideDelay, () {
      if (!_hoveringTarget && !_hoveringPopover) {
        _removeEntry();
      }
    });
  }

  Future<void> _showEntry() async {
    if (!mounted || _entry != null || !_hoveringTarget) {
      return;
    }

    // The popover is inserted only after the shared wallpaper is decoded; otherwise
    // the desktop demo visibly paints its dark fallback before the image arrives.
    if (!WoxSystemWallpaperUtil.instance.isCachedSystemWallpaperImageReady) {
      await WoxSystemWallpaperUtil.instance.preloadSystemWallpaperImageProvider(context);
      if (!mounted || _entry != null || !_hoveringTarget) {
        return;
      }
    }

    final overlay = Overlay.of(context, rootOverlay: true);
    final targetBox = context.findRenderObject() as RenderBox?;
    final overlayBox = overlay.context.findRenderObject() as RenderBox?;
    if (targetBox == null || overlayBox == null || !targetBox.hasSize) {
      return;
    }

    final targetTopLeft = targetBox.localToGlobal(Offset.zero, ancestor: overlayBox);
    final targetSize = targetBox.size;
    final overlaySize = overlayBox.size;
    final availableWidth = (overlaySize.width - (widget.margin * 2)).clamp(160.0, double.infinity).toDouble();
    final availableHeight = (overlaySize.height - (widget.margin * 2)).clamp(140.0, double.infinity).toDouble();
    final popoverWidth = widget.width.clamp(160.0, availableWidth).toDouble();
    final popoverHeight = widget.height.clamp(140.0, availableHeight).toDouble();
    final showAbove = targetTopLeft.dy + targetSize.height + widget.margin + popoverHeight > overlaySize.height;
    final preferredLeft = targetTopLeft.dx + targetSize.width - popoverWidth;
    final maxLeft = (overlaySize.width - popoverWidth - widget.margin).clamp(widget.margin, double.infinity).toDouble();
    final maxTop = (overlaySize.height - popoverHeight - widget.margin).clamp(widget.margin, double.infinity).toDouble();
    final preferredTop = showAbove ? targetTopLeft.dy - popoverHeight - widget.margin : targetTopLeft.dy + targetSize.height + widget.margin;
    // Feature guard: settings panes can be narrow or scrolled near an edge, so the preview is clamped after choosing above/below placement instead of relying on a fixed follower offset.
    final clampedLeft = preferredLeft.clamp(widget.margin, maxLeft).toDouble();
    final clampedTop = preferredTop.clamp(widget.margin, maxTop).toDouble();
    // Bug fix: demo popovers must be opaque because settings content remains
    // active underneath the overlay. The previous transparent Material let
    // table text bleed through semi-transparent demo scenes, so the host now
    // provides a hard backdrop before clipping the animated preview.
    final popoverBackground = getThemeBackgroundColor().withValues(alpha: 1);

    _entry = OverlayEntry(
      builder: (context) {
        // Feature change: demo popovers host animated widgets, so they use a dedicated overlay instead of the text-only WoxTooltip. This keeps hover on either the trigger or the preview alive and disposes animation controllers as soon as the overlay closes.
        return Positioned(
          left: clampedLeft,
          top: clampedTop,
          width: popoverWidth,
          height: popoverHeight,
          child: MouseRegion(
            onEnter: (_) {
              _hoveringPopover = true;
              _hideTimer?.cancel();
            },
            onExit: (_) => _scheduleHide(fromPopover: true),
            child: Material(
              key: widget.popoverKey,
              color: popoverBackground,
              borderRadius: BorderRadius.circular(8),
              clipBehavior: Clip.antiAlias,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: popoverBackground,
                  border: Border.all(color: getThemeTextColor().withValues(alpha: 0.12)),
                  borderRadius: BorderRadius.circular(8),
                  boxShadow: [BoxShadow(color: Colors.black.withValues(alpha: 0.28), blurRadius: 32, offset: const Offset(0, 18))],
                ),
                child: ClipRRect(borderRadius: BorderRadius.circular(8), child: ColoredBox(color: popoverBackground, child: widget.demo)),
              ),
            ),
          ),
        );
      },
    );
    overlay.insert(_entry!);
  }

  void _removeEntry() {
    _entry?.remove();
    _entry = null;
    _hoveringPopover = false;
  }

  @override
  Widget build(BuildContext context) {
    return CompositedTransformTarget(link: _layerLink, child: MouseRegion(onEnter: (_) => _scheduleShow(), onExit: (_) => _scheduleHide(fromPopover: false), child: widget.child));
  }
}
