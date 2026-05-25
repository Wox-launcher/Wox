import 'dart:async';
import 'dart:io';
import 'dart:ui' as ui;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_preview_webview_data.dart';
import 'package:wox/utils/webview/wox_webview_util.dart';
import 'package:wox/utils/webview/wox_webview_session.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class WoxWebViewPreview extends StatefulWidget {
  final String previewData;
  final WoxLauncherController launcherController;

  const WoxWebViewPreview({super.key, required this.previewData, required this.launcherController});

  @override
  State<WoxWebViewPreview> createState() => _WoxWebViewPreviewState();
}

class _WoxWebViewPreviewState extends State<WoxWebViewPreview> {
  static const double _toolbarBottomSpacing = 60;
  static const double _toolbarHeight = 36;
  // These are normal-density base values; preview toolbar controls scale from
  // them so the floating shape stays proportional to the active interface size.
  static const double _toolbarWidth = 240;
  static const double _toolbarTriggerWidth = 288;
  static const double _toolbarTriggerHeight = 72;
  static const Duration _toolbarAnimationDuration = Duration(milliseconds: 180);
  static const Duration _toolbarAutoHideDelay = Duration(milliseconds: 1200);

  Future<WoxWebViewSession?>? _windowsSessionFuture;
  WoxWebViewSession? _session;
  StreamSubscription<WoxWebViewSessionAction>? _sessionActionSubscription;
  StreamSubscription<void>? _unhandledEscapeSubscription;
  String? _windowsErrorMessage;
  Timer? _toolbarHideTimer;
  bool _isToolbarVisible = true;
  String? _focusedHiddenQueryBoxPreviewData;
  int _hiddenQueryBoxWebViewFocusToken = 0;

  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;
  WoxLauncherController get launcherController => widget.launcherController;

  double _scaled(double value) => _metrics.scaledSpacing(value);

  WoxPreviewWebviewData get webviewData {
    return WoxPreviewWebviewData.fromPreviewData(widget.previewData);
  }

  @override
  void initState() {
    super.initState();
    _refreshWindowsSession();
    _subscribeUnhandledEscape();
    _showToolbarTemporarily();
  }

  @override
  void didUpdateWidget(covariant WoxWebViewPreview oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.previewData != widget.previewData) {
      _focusedHiddenQueryBoxPreviewData = null;
      unawaited(_replaceWindowsSession());
      _showToolbarTemporarily();
    }
  }

  @override
  void dispose() {
    _toolbarHideTimer?.cancel();
    _unhandledEscapeSubscription?.cancel();
    unawaited(_releaseCurrentSession());
    super.dispose();
  }

  void _subscribeUnhandledEscape() {
    _unhandledEscapeSubscription?.cancel();
    _unhandledEscapeSubscription = WoxWebViewUtil.unhandledEscape.listen((_) {
      _handleFallbackEscape();
    });
  }

  void _refreshWindowsSession() {
    if (!Platform.isWindows) {
      return;
    }

    _windowsErrorMessage = null;
    _windowsSessionFuture = WoxWebViewUtil.acquireSession(webviewData)
        .then((session) {
          _session = session;
          WoxWebViewUtil.setActiveSession(session);
          _subscribeSessionActions(session);
          _focusWebViewIfQueryBoxHidden(session: session);
          return session;
        })
        .catchError((error) {
          _windowsErrorMessage = error.toString();
          return null;
        });
  }

  Future<void> _releaseCurrentSession() async {
    await _sessionActionSubscription?.cancel();
    _sessionActionSubscription = null;

    final session = _session;
    if (session == null) {
      return;
    }

    WoxWebViewUtil.clearActiveSession(session);
    _session = null;
    await WoxWebViewUtil.releaseSession(session);
  }

  Future<void> _replaceWindowsSession() async {
    await _releaseCurrentSession();
    if (!mounted) {
      return;
    }

    _refreshWindowsSession();
  }

  void _subscribeSessionActions(WoxWebViewSession? session) {
    _sessionActionSubscription?.cancel();
    _sessionActionSubscription = null;

    if (session == null) {
      return;
    }

    _sessionActionSubscription = session.actions.listen((action) {
      switch (action) {
        case WoxWebViewSessionAction.toggleActionPanel:
          launcherController.toggleActionPanel(const UuidV4().generate());
          break;
        case WoxWebViewSessionAction.fallbackEscape:
          _handleFallbackEscape();
          break;
      }
    });
  }

  void _handleFallbackEscape() {
    final traceId = const UuidV4().generate();
    if (launcherController.isQueryBoxVisible.value) {
      launcherController.focusQueryBox();
      return;
    }

    launcherController.hideApp(traceId);
  }

  void _focusWebViewIfQueryBoxHidden({WoxWebViewSession? session}) {
    if (launcherController.isQueryBoxVisible.value || _focusedHiddenQueryBoxPreviewData == widget.previewData) {
      return;
    }

    _focusedHiddenQueryBoxPreviewData = widget.previewData;
    final focusToken = ++_hiddenQueryBoxWebViewFocusToken;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || launcherController.isQueryBoxVisible.value) {
        return;
      }
      if (focusToken != _hiddenQueryBoxWebViewFocusToken) {
        return;
      }
      if (session != null && !identical(_session, session)) {
        return;
      }

      unawaited(_focusActiveWebView(focusToken: focusToken, session: session));
    });
  }

  Future<void> _focusActiveWebView({required int focusToken, WoxWebViewSession? session}) async {
    const retryDelays = [Duration.zero, Duration(milliseconds: 50), Duration(milliseconds: 100), Duration(milliseconds: 200), Duration(milliseconds: 400)];

    for (final delay in retryDelays) {
      if (delay > Duration.zero) {
        await Future.delayed(delay);
      }
      if (!mounted || launcherController.isQueryBoxVisible.value || focusToken != _hiddenQueryBoxWebViewFocusToken) {
        return;
      }
      if (session != null && !identical(_session, session)) {
        return;
      }

      final focused = await WoxWebViewUtil.focusActiveSession();
      if (focused) {
        return;
      }
    }

    if (mounted && !launcherController.isQueryBoxVisible.value && focusToken == _hiddenQueryBoxWebViewFocusToken) {
      _focusedHiddenQueryBoxPreviewData = null;
    }
  }

  Widget _buildWindowsPreview(WoxPreviewWebviewData preview) {
    final future = _windowsSessionFuture;
    if (future == null) {
      return WoxSelectableText("WebView preview is not initialized on Windows.\nURL: ${preview.url}");
    }

    return FutureBuilder<WoxWebViewSession?>(
      future: future,
      builder: (context, snapshot) {
        if (snapshot.connectionState != ConnectionState.done) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }

        final session = snapshot.data;
        if (session == null) {
          final message = _windowsErrorMessage ?? "WebView2 Runtime is not available on this system.";
          return WoxSelectableText("$message\nURL: ${preview.url}");
        }

        return _buildPreviewWithToolbar(child: session.buildWidget(), navigationState: session.navigationState);
      },
    );
  }

  Widget _buildPreviewWithToolbar({required Widget child, ValueListenable<WoxWebViewNavigationState>? navigationState}) {
    return Stack(
      children: [
        Positioned.fill(child: child),
        _buildToolbarTrigger(),
        if (navigationState == null)
          _buildToolbar()
        else
          ValueListenableBuilder<WoxWebViewNavigationState>(
            valueListenable: navigationState,
            builder: (context, state, _) {
              return _buildToolbar(navigationState: state);
            },
          ),
      ],
    );
  }

  Widget _buildToolbarTrigger() {
    return Positioned(
      left: 0,
      right: 0,
      bottom: _scaled(_toolbarBottomSpacing - 18),
      child: Align(
        alignment: Alignment.bottomCenter,
        child: MouseRegion(
          onEnter: (_) => _showToolbarTemporarily(),
          onExit: (_) => _scheduleToolbarHide(),
          child: SizedBox(width: _scaled(_toolbarTriggerWidth), height: _scaled(_toolbarTriggerHeight)),
        ),
      ),
    );
  }

  Widget _buildToolbar({WoxWebViewNavigationState? navigationState}) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final iconColor = isDark ? Colors.black.withValues(alpha: 0.82) : Colors.black.withValues(alpha: 0.72);
    final backgroundColor = Colors.white.withValues(alpha: isDark ? 0.58 : 0.72);
    final borderColor = Colors.white.withValues(alpha: isDark ? 0.28 : 0.46);
    final shadowColor = Colors.black.withValues(alpha: isDark ? 0.22 : 0.12);

    return Positioned(
      bottom: _scaled(_toolbarBottomSpacing),
      left: 0,
      right: 0,
      child: IgnorePointer(
        ignoring: !_isToolbarVisible,
        child: Align(
          alignment: Alignment.bottomCenter,
          child: MouseRegion(
            onEnter: (_) => _showToolbar(keepVisible: true),
            onExit: (_) => _scheduleToolbarHide(),
            child: AnimatedOpacity(
              duration: _toolbarAnimationDuration,
              curve: Curves.easeOutCubic,
              opacity: _isToolbarVisible ? 1 : 0,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(_scaled(_toolbarHeight) / 2),
                child: BackdropFilter(
                  filter: ui.ImageFilter.blur(sigmaX: 16, sigmaY: 16),
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      color: backgroundColor,
                      borderRadius: BorderRadius.circular(_scaled(_toolbarHeight) / 2),
                      border: Border.all(color: borderColor),
                      boxShadow: [BoxShadow(color: shadowColor, blurRadius: 20, offset: const Offset(0, 8))],
                    ),
                    child: Padding(
                      padding: EdgeInsets.symmetric(horizontal: _scaled(6), vertical: _scaled(2)),
                      child: SizedBox(
                        width: _scaled(_toolbarWidth),
                        child: Row(
                          children: [
                            Expanded(child: _buildToolbarDragHandle()),
                            _buildToolbarButton(
                              icon: Icons.arrow_back_rounded,
                              tooltip: launcherController.tr("ui_action_webview_go_back"),
                              iconColor: iconColor,
                              enabled: navigationState?.canGoBack,
                              onPressed: _goBack,
                            ),
                            _buildToolbarButton(
                              icon: Icons.refresh_rounded,
                              tooltip: launcherController.tr("ui_action_webview_refresh"),
                              iconColor: iconColor,
                              onPressed: _refresh,
                            ),
                            _buildToolbarButton(
                              icon: Icons.arrow_forward_rounded,
                              tooltip: launcherController.tr("ui_action_webview_go_forward"),
                              iconColor: iconColor,
                              enabled: navigationState?.canGoForward,
                              onPressed: _goForward,
                            ),
                            _buildToolbarButton(
                              icon: Icons.open_in_browser_rounded,
                              tooltip: launcherController.tr("ui_action_webview_open_in_browser"),
                              iconColor: iconColor,
                              onPressed: _openInBrowser,
                            ),
                            _buildToolbarButton(
                              icon: Icons.visibility_off_rounded,
                              tooltip: launcherController.tr("ui_action_webview_hide_wox"),
                              iconColor: iconColor,
                              onPressed: _hideWox,
                            ),
                            Expanded(child: _buildToolbarDragHandle()),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildToolbarButton({required IconData icon, required String tooltip, required Color iconColor, required VoidCallback onPressed, bool? enabled}) {
    final isEnabled = enabled ?? true;

    // The floating webview toolbar used IconButton.tooltip before, which made
    // its hover help use Material's overlay while the rest of Wox used WoxTooltip.
    // Wrapping the button keeps the same click target and centralizes tooltip UI.
    return WoxTooltip(
      message: tooltip,
      child: IconButton(
        onPressed: isEnabled ? onPressed : null,
        icon: Icon(icon),
        iconSize: _scaled(20),
        color: iconColor,
        disabledColor: iconColor.withValues(alpha: 0.28),
        padding: EdgeInsets.only(left: _scaled(6), right: _scaled(6)),
        constraints: BoxConstraints.tightFor(width: _scaled(32), height: _scaled(32)),
        splashRadius: _scaled(16),
        visualDensity: VisualDensity.compact,
      ),
    );
  }

  Widget _buildToolbarDragHandle() {
    return MouseRegion(
      cursor: SystemMouseCursors.move,
      child: GestureDetector(
        behavior: HitTestBehavior.translucent,
        onPanStart: (_) {
          launcherController.windowDriver.startDragging();
        },
        child: SizedBox(height: _scaled(32)),
      ),
    );
  }

  void _showToolbarTemporarily() {
    _showToolbar();
  }

  void _showToolbar({bool keepVisible = false}) {
    _toolbarHideTimer?.cancel();

    if (!_isToolbarVisible && mounted) {
      setState(() {
        _isToolbarVisible = true;
      });
    }

    if (!keepVisible) {
      _scheduleToolbarHide();
    }
  }

  void _scheduleToolbarHide({Duration delay = _toolbarAutoHideDelay}) {
    _toolbarHideTimer?.cancel();
    _toolbarHideTimer = Timer(delay, () {
      if (!mounted || !_isToolbarVisible) {
        return;
      }

      setState(() {
        _isToolbarVisible = false;
      });
    });
  }

  void _refresh() {
    unawaited(WoxWebViewUtil.refresh());
  }

  void _goBack() {
    unawaited(WoxWebViewUtil.goBack());
  }

  void _goForward() {
    unawaited(WoxWebViewUtil.goForward());
  }

  void _openInBrowser() {
    unawaited(_openCurrentUrlInBrowser());
  }

  void _hideWox() {
    unawaited(launcherController.hideApp(const UuidV4().generate()));
  }

  Future<void> _openCurrentUrlInBrowser() async {
    // WebView navigation can move away from the original preview URL. Prefer the platform-reported current
    // URL and fall back to the preview data so cached/native views still have a usable browser escape hatch.
    final currentUrl = await WoxWebViewUtil.getCurrentUrl();
    final uri = _resolveExternalBrowserUri(currentUrl) ?? _resolveExternalBrowserUri(webviewData.url);
    if (uri == null) {
      return;
    }

    await launchUrl(uri, mode: LaunchMode.externalApplication);
  }

  Uri? _resolveExternalBrowserUri(String? url) {
    final uri = Uri.tryParse(url?.trim() ?? "");
    if (uri == null || uri.host.isEmpty || (uri.scheme != "http" && uri.scheme != "https")) {
      return null;
    }

    return uri;
  }

  @override
  Widget build(BuildContext context) {
    final preview = webviewData;

    if (Platform.isWindows) {
      return _buildWindowsPreview(preview);
    }

    if (Platform.isMacOS) {
      _focusWebViewIfQueryBoxHidden();
      return _buildPreviewWithToolbar(
        child: AppKitView(key: ValueKey(widget.previewData), viewType: "wox/webview_preview", creationParams: preview.toJson(), creationParamsCodec: const StandardMessageCodec()),
      );
    }

    return WoxSelectableText("WebView preview is currently only available on macOS and Windows.\nURL: ${preview.url}");
  }
}
