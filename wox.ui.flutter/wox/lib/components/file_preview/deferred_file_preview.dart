import 'dart:async';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

// Defers heavy native/WebView preview construction until the selection is stable
// or the user explicitly asks for it, keeping result navigation responsive.
class WoxDeferredFilePreview extends StatefulWidget {
  final String previewKey;
  final IconData icon;
  final String? fileIconPath;
  final Color accent;
  final String title;
  final String subtitle;
  final List<WoxFilePreviewProperty> properties;
  final String messageTitle;
  final String message;
  final String actionLabel;
  final ScrollController scrollController;
  final Duration? autoLoadDelay;
  final bool loadedPreviewHandlesScrolling;
  final WidgetBuilder previewBuilder;

  const WoxDeferredFilePreview({
    super.key,
    required this.previewKey,
    required this.icon,
    this.fileIconPath,
    required this.accent,
    required this.title,
    required this.subtitle,
    required this.properties,
    required this.messageTitle,
    required this.message,
    required this.actionLabel,
    required this.scrollController,
    required this.autoLoadDelay,
    this.loadedPreviewHandlesScrolling = true,
    required this.previewBuilder,
  });

  @override
  State<WoxDeferredFilePreview> createState() => _WoxDeferredFilePreviewState();
}

class _WoxDeferredFilePreviewState extends State<WoxDeferredFilePreview> {
  Timer? _autoLoadTimer;
  StreamSubscription<String>? _loadPreviewActionSubscription;
  WoxLauncherController? _launcherController;
  bool _isPreviewLoaded = false;

  @override
  void initState() {
    super.initState();
    _launcherController = Get.isRegistered<WoxLauncherController>() ? Get.find<WoxLauncherController>() : null;
    _loadPreviewActionSubscription = _launcherController?.manualFilePreviewLoadRequests.listen((_) {
      if (!mounted || _isPreviewLoaded || widget.autoLoadDelay != null) {
        return;
      }
      _loadPreview();
    });
    _syncManualLoadAvailability();
    _scheduleAutoLoad();
  }

  @override
  void didUpdateWidget(covariant WoxDeferredFilePreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.previewKey != widget.previewKey || oldWidget.autoLoadDelay != widget.autoLoadDelay) {
      _autoLoadTimer?.cancel();
      _syncManualLoadAvailability(forceAvailable: false, previewKey: oldWidget.previewKey);
      _isPreviewLoaded = false;
      _syncManualLoadAvailability();
      _scheduleAutoLoad();
    }
  }

  @override
  void dispose() {
    _autoLoadTimer?.cancel();
    _syncManualLoadAvailability(forceAvailable: false);
    unawaited(_loadPreviewActionSubscription?.cancel());
    super.dispose();
  }

  // Defers toolbar updates until after layout so preview construction does not
  // mutate launcher chrome while Flutter is still building this subtree.
  void _syncManualLoadAvailability({bool? forceAvailable, String? previewKey}) {
    final available = forceAvailable ?? (!_isPreviewLoaded && widget.autoLoadDelay == null);
    final launcherController = _launcherController;
    if (launcherController == null) {
      return;
    }
    final actionPreviewKey = previewKey ?? widget.previewKey;

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (available && !mounted) {
        return;
      }
      launcherController.updateManualFilePreviewLoadAvailability(const UuidV4().generate(), actionPreviewKey, available);
    });
  }

  void _scheduleAutoLoad() {
    final delay = widget.autoLoadDelay;
    if (delay == null) {
      return;
    }

    _autoLoadTimer = Timer(delay, () {
      if (!mounted) {
        return;
      }
      setState(() => _isPreviewLoaded = true);
      _syncManualLoadAvailability();
    });
  }

  void _loadPreview() {
    _autoLoadTimer?.cancel();
    if (_isPreviewLoaded) {
      return;
    }
    setState(() => _isPreviewLoaded = true);
    _syncManualLoadAvailability();
  }

  @override
  Widget build(BuildContext context) {
    if (_isPreviewLoaded) {
      final preview = widget.previewBuilder(context);
      return widget.loadedPreviewHandlesScrolling ? preview : _buildScrollable(preview);
    }

    if (widget.autoLoadDelay != null) {
      return const Center(child: WoxLoadingIndicator(size: 20));
    }

    return _buildManualPreview(context);
  }

  Widget _buildManualPreview(BuildContext context) {
    return _buildScrollable(
      WoxFileInfoPreview(
        icon: widget.icon,
        fileIconPath: widget.fileIconPath,
        accent: widget.accent,
        title: widget.title,
        subtitle: widget.subtitle,
        properties: widget.properties,
        sections: [
          WoxFilePreviewSection(
            title: widget.messageTitle,
            child: _ManualPreviewPrompt(
              message: widget.message,
              actionLabel: widget.actionLabel,
              hotkeyLabel: _launcherController?.filePreviewLoadHotkeyLabel ?? "",
              onPressed: _loadPreview,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildScrollable(Widget child) {
    return LayoutBuilder(
      builder:
          (context, constraints) => Scrollbar(
            thumbVisibility: true,
            controller: widget.scrollController,
            child: SingleChildScrollView(
              controller: widget.scrollController,
              child: ConstrainedBox(constraints: BoxConstraints(minWidth: constraints.maxWidth, maxWidth: constraints.maxWidth, minHeight: constraints.maxHeight), child: child),
            ),
          ),
    );
  }
}

class _ManualPreviewPrompt extends StatelessWidget {
  final String message;
  final String actionLabel;
  final String hotkeyLabel;
  final VoidCallback onPressed;

  const _ManualPreviewPrompt({required this.message, required this.actionLabel, required this.hotkeyLabel, required this.onPressed});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final padding = metrics.scaledSpacing(12);

    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < metrics.scaledSpacing(520);
        final promptMessage = Text(message, style: TextStyle(color: getThemeSubTextColor(), fontSize: metrics.resultSubtitleFontSize, height: 1.4));
        final action = _LoadPreviewButton(label: actionLabel, hotkeyLabel: hotkeyLabel, onPressed: onPressed, expanded: compact);

        return Padding(
          padding: EdgeInsets.all(padding),
          child:
              compact
                  ? Column(crossAxisAlignment: CrossAxisAlignment.start, children: [promptMessage, SizedBox(height: metrics.scaledSpacing(12)), action])
                  : Row(crossAxisAlignment: CrossAxisAlignment.center, children: [Expanded(child: promptMessage), SizedBox(width: metrics.scaledSpacing(14)), action]),
        );
      },
    );
  }
}

class _LoadPreviewButton extends StatelessWidget {
  final String label;
  final String hotkeyLabel;
  final VoidCallback onPressed;
  final bool expanded;

  const _LoadPreviewButton({required this.label, required this.hotkeyLabel, required this.onPressed, required this.expanded});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final activeColor = getThemeActiveBackgroundColor();
    final borderRadius = BorderRadius.circular(6);
    final hotkeyParts = hotkeyLabel.split("+").map((part) => part.trim()).where((part) => part.isNotEmpty).toList();

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onPressed,
        borderRadius: borderRadius,
        child: Container(
          constraints: BoxConstraints(minHeight: metrics.scaledSpacing(34)),
          padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(12), vertical: metrics.scaledSpacing(7)),
          decoration: BoxDecoration(color: activeColor.withValues(alpha: 0.08), borderRadius: borderRadius, border: Border.all(color: activeColor.withValues(alpha: 0.28))),
          child: Row(
            mainAxisSize: expanded ? MainAxisSize.max : MainAxisSize.min,
            children: [
              Flexible(
                fit: expanded ? FlexFit.tight : FlexFit.loose,
                child: Text(
                  label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: getThemeTextColor(), fontSize: metrics.resultSubtitleFontSize, fontWeight: FontWeight.w700),
                ),
              ),
              if (hotkeyParts.isNotEmpty) ...[
                SizedBox(width: metrics.scaledSpacing(10)),
                Wrap(spacing: metrics.scaledSpacing(4), runSpacing: metrics.scaledSpacing(4), children: hotkeyParts.map((part) => _HotkeyKeycap(label: part)).toList()),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _HotkeyKeycap extends StatelessWidget {
  final String label;

  const _HotkeyKeycap({required this.label});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeSubTextColor();

    return Container(
      constraints: BoxConstraints(minWidth: metrics.scaledSpacing(20)),
      padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(5), vertical: metrics.scaledSpacing(2)),
      decoration: BoxDecoration(
        color: getThemeCardBackgroundColor().withValues(alpha: 0.36),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(color: getThemeDividerColor().withValues(alpha: 0.55)),
      ),
      child: Text(
        label,
        textAlign: TextAlign.center,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: textColor, fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700, height: 1.1),
      ),
    );
  }
}
