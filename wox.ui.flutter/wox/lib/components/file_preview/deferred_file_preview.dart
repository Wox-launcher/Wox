import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_loading_indicator.dart';
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
  bool _isPreviewLoaded = false;

  @override
  void initState() {
    super.initState();
    _scheduleAutoLoad();
  }

  @override
  void didUpdateWidget(covariant WoxDeferredFilePreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.previewKey != widget.previewKey || oldWidget.autoLoadDelay != widget.autoLoadDelay) {
      _autoLoadTimer?.cancel();
      _isPreviewLoaded = false;
      _scheduleAutoLoad();
    }
  }

  @override
  void dispose() {
    _autoLoadTimer?.cancel();
    super.dispose();
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
    });
  }

  void _loadPreview() {
    _autoLoadTimer?.cancel();
    if (_isPreviewLoaded) {
      return;
    }
    setState(() => _isPreviewLoaded = true);
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
            child: Padding(
              padding: EdgeInsets.all(WoxInterfaceSizeUtil.instance.current.scaledSpacing(12)),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(widget.message, style: TextStyle(color: getThemeSubTextColor(), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize, height: 1.4)),
                  SizedBox(height: WoxInterfaceSizeUtil.instance.current.scaledSpacing(12)),
                  WoxButton.primary(
                    text: widget.actionLabel,
                    icon: Icon(Icons.visibility_rounded, size: WoxInterfaceSizeUtil.instance.current.toolbarIconSize),
                    padding: EdgeInsets.symmetric(
                      horizontal: WoxInterfaceSizeUtil.instance.current.scaledSpacing(14),
                      vertical: WoxInterfaceSizeUtil.instance.current.scaledSpacing(9),
                    ),
                    onPressed: _loadPreview,
                  ),
                ],
              ),
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
