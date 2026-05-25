// ignore_for_file: invalid_use_of_internal_member, implementation_imports

// This wrapper intentionally uses Flutter's experimental internal windowing
// API (`flutter/src/widgets/_window.dart`) so Wox can host multiple launcher
// surfaces in one Flutter process. Recheck this file when upgrading Flutter;
// it was first verified on Flutter 3.45.0-1.0.pre-196, master revision
// 2731746a84.

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/src/widgets/_window.dart' as flutter_windowing;
import 'package:get/get.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window_style.dart';
import 'package:wox/utils/wox_theme_util.dart';

const double _woxManagedWindowCornerRadius = 12;

class WoxMultipleWindowHost extends StatefulWidget {
  const WoxMultipleWindowHost({super.key, required this.theme, required this.child});

  final ThemeData theme;
  final Widget child;

  @override
  State<WoxMultipleWindowHost> createState() => _WoxMultipleWindowHostState();
}

class _WoxMultipleWindowHostState extends State<WoxMultipleWindowHost> {
  flutter_windowing.WindowRegistry? _currentRegistry;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _currentRegistry = flutter_windowing.WindowRegistry.maybeOf(context);
    WoxMultipleWindow._attach(_currentRegistry, widget.theme);
  }

  @override
  void didUpdateWidget(WoxMultipleWindowHost oldWidget) {
    super.didUpdateWidget(oldWidget);
    _currentRegistry = flutter_windowing.WindowRegistry.maybeOf(context);
    WoxMultipleWindow._attach(_currentRegistry, widget.theme);
  }

  @override
  void dispose() {
    WoxMultipleWindow._detach(_currentRegistry);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    _currentRegistry = flutter_windowing.WindowRegistry.maybeOf(context);
    WoxMultipleWindow._attach(_currentRegistry, widget.theme);
    return widget.child;
  }
}

abstract class WoxMultipleWindowHandle {
  String get id;

  int? get nativeHandle;

  Future<Size> getSize();

  Future<Offset> getPosition();

  Future<void> setBounds(Offset position, Size size);

  Future<void> setSize(Size size);

  Future<void> setAlwaysOnTop(bool value);

  void startDragging();

  Future<void> show();

  Future<void> focus();

  Future<void> hide();

  Future<void> close();
}

class WoxMultipleWindow {
  static flutter_windowing.WindowRegistry? _registry;
  static ThemeData? _theme;
  static final Map<String, _WoxWindowRecord> _windows = {};

  static bool isOpen(String id) => _windows.containsKey(id);

  static bool canPop(String id) => _windows[id]?.navigatorKey.currentState?.canPop() ?? false;

  static void popUntilRoot(String id) {
    _windows[id]?.navigatorKey.currentState?.popUntil((route) => route.isFirst);
  }

  static void startDragging(String id) {
    final record = _windows[id];
    if (record == null) {
      return;
    }

    WoxMultipleWindowStyle.startDragging(record.controller);
  }

  static void _minimizeWindow(String id) {
    final record = _windows[id];
    if (record == null) {
      return;
    }

    record.controller.setMinimized(true);
  }

  static void _toggleMaximizeWindow(String id) {
    final record = _windows[id];
    if (record == null) {
      return;
    }

    record.controller.setMaximized(!record.controller.isMaximized);
  }

  /// Creates or focuses a top-level Flutter window managed through Flutter windowing.
  ///
  /// When [showTitleBar] is true, the titlebar is Flutter-drawn. Desktop
  /// platforms with native window handles remove their native frame through
  /// [WoxMultipleWindowStyle] so the wrapper owns all visible chrome.
  static Future<WoxMultipleWindowHandle> createWindow({
    required String id,
    required String title,
    required Size preferredSize,
    BoxConstraints? preferredConstraints,
    required WidgetBuilder builder,
    bool showTitleBar = true,
    bool mica = true,
    bool focusIfExists = true,
    bool resizable = false,
    bool minimizable = true,
    bool closeOnRequest = true,
    bool centerOnCreate = true,
    bool roundedCorners = true,
    VoidCallback? onDestroyed,
  }) async {
    final existing = _windows[id];
    if (existing != null) {
      if (focusIfExists) {
        await existing.handle.focus();
      }
      return existing.handle;
    }

    final registry = _registry;
    if (registry == null) {
      throw UnsupportedError("Flutter windowing registry is not available.");
    }

    late final flutter_windowing.WindowEntry entry;
    final navigatorKey = GlobalKey<NavigatorState>(debugLabel: "window-navigator-$id");
    final delegate = _WoxMultipleWindowDelegate(id: id, closeOnRequest: closeOnRequest);
    final effectiveConstraints = resizable ? preferredConstraints : BoxConstraints.tight(preferredSize);
    final controller = flutter_windowing.RegularWindowController(preferredSize: preferredSize, preferredConstraints: effectiveConstraints, title: title, delegate: delegate);
    final handle = _WoxMultipleWindowHandleImpl(id: id, controller: controller);
    // Windows can keep the HWND hidden until Flutter is ready. macOS shows the
    // NSWindow during controller creation, so moving it offscreen only creates
    // extra visible jumps before the final placement.
    final prepareOffscreen = Platform.isWindows;
    final preparationReady = Completer<void>();
    final record = _WoxWindowRecord(
      id: id,
      controller: controller,
      handle: handle,
      navigatorKey: navigatorKey,
      unregister: () => registry.unregister(entry),
      onDestroyed: onDestroyed,
    );

    entry = flutter_windowing.WindowEntry(
      controller: controller,
      builder: (windowContext) {
        return _WoxMultipleWindowRoot(
          id: id,
          title: title,
          controller: controller,
          navigatorKey: navigatorKey,
          theme: _theme ?? ThemeData(useMaterial3: true),
          showTitleBar: showTitleBar,
          resizable: resizable,
          minimizable: minimizable,
          mica: mica,
          roundedCorners: roundedCorners,
          onPrepared: () {
            if (!preparationReady.isCompleted) {
              preparationReady.complete();
            }
          },
          builder: builder,
        );
      },
    );

    record.registered = true;
    _windows[id] = record;
    if (prepareOffscreen) {
      await WoxMultipleWindowStyle.moveOffscreen(controller);
    }
    await WoxMultipleWindowStyle.apply(controller, mica: mica, darkMode: isThemeDark(), roundedCorners: roundedCorners, minimizable: minimizable, resizable: resizable);
    if (Platform.isMacOS && centerOnCreate) {
      await WoxMultipleWindowStyle.centerOnCursorDisplay(controller, preferredSize: preferredSize);
    }
    registry.register(entry);
    controller.activate();
    if (prepareOffscreen) {
      await preparationReady.future.timeout(const Duration(milliseconds: 500), onTimeout: () {});
    }
    if (!Platform.isMacOS && centerOnCreate) {
      await WoxMultipleWindowStyle.centerOnCursorDisplay(controller, preferredSize: preferredSize);
    }
    return handle;
  }

  static Future<void> closeWindow(String id) async {
    _destroyWindow(id);
  }

  static void _attach(flutter_windowing.WindowRegistry? registry, ThemeData theme) {
    _registry = registry;
    _theme = theme;
  }

  static void _detach(flutter_windowing.WindowRegistry? registry) {
    if (_registry == registry) {
      _registry = null;
    }
  }

  static void _destroyWindow(String id) {
    final record = _windows.remove(id);
    if (record == null) {
      return;
    }

    record.unregisterIfNeeded();
    record.controller.destroy();
    record.notifyDestroyed();
  }

  static void _handleExternalDestroy(String id) {
    final record = _windows.remove(id);
    if (record == null) {
      return;
    }

    record.unregisterIfNeeded();
    record.notifyDestroyed();
  }
}

class _WoxMultipleWindowHandleImpl implements WoxMultipleWindowHandle {
  _WoxMultipleWindowHandleImpl({required this.id, required this.controller});

  @override
  final String id;
  final flutter_windowing.RegularWindowController controller;
  Offset _position = Offset.zero;

  @override
  int? get nativeHandle => WoxMultipleWindowStyle.nativeHandleOf(controller);

  @override
  Future<Size> getSize() async => controller.contentSize;

  @override
  Future<Offset> getPosition() async {
    final nativePosition = await WoxMultipleWindowStyle.positionOf(controller);
    return nativePosition ?? _position;
  }

  @override
  Future<void> setBounds(Offset position, Size size) async {
    _position = position;
    controller.setConstraints(BoxConstraints.tight(size));
    controller.setSize(size);
    await WoxMultipleWindowStyle.setBounds(controller, position, size);
  }

  @override
  Future<void> setSize(Size size) async {
    controller.setConstraints(BoxConstraints.tight(size));
    controller.setSize(size);
    await WoxMultipleWindowStyle.setSize(controller, size);
  }

  @override
  Future<void> setAlwaysOnTop(bool value) async {
    await WoxMultipleWindowStyle.setAlwaysOnTop(controller, value);
  }

  @override
  void startDragging() {
    WoxMultipleWindowStyle.startDragging(controller);
  }

  @override
  Future<void> show() async {
    await WoxMultipleWindowStyle.show(controller);
    controller.activate();
  }

  @override
  Future<void> focus() async {
    await WoxMultipleWindowStyle.focus(controller);
    controller.activate();
  }

  @override
  Future<void> hide() async {
    await WoxMultipleWindowStyle.hide(controller);
  }

  @override
  Future<void> close() async {
    WoxMultipleWindow._destroyWindow(id);
  }
}

class _WoxWindowRecord {
  _WoxWindowRecord({required this.id, required this.controller, required this.handle, required this.navigatorKey, required this.unregister, required this.onDestroyed});

  final String id;
  final flutter_windowing.RegularWindowController controller;
  final WoxMultipleWindowHandle handle;
  final GlobalKey<NavigatorState> navigatorKey;
  final VoidCallback unregister;
  final VoidCallback? onDestroyed;
  bool registered = false;
  bool destroyedNotified = false;

  void unregisterIfNeeded() {
    if (!registered) {
      return;
    }
    registered = false;
    unregister();
  }

  void notifyDestroyed() {
    if (destroyedNotified) {
      return;
    }
    destroyedNotified = true;
    onDestroyed?.call();
  }
}

class _WoxMultipleWindowDelegate with flutter_windowing.RegularWindowControllerDelegate {
  _WoxMultipleWindowDelegate({required this.id, required this.closeOnRequest});

  final String id;
  final bool closeOnRequest;

  @override
  void onWindowCloseRequested(flutter_windowing.RegularWindowController controller) {
    if (!closeOnRequest) {
      controller.activate();
      return;
    }

    WoxMultipleWindow._destroyWindow(id);
  }

  @override
  void onWindowDestroyed() {
    WoxMultipleWindow._handleExternalDestroy(id);
  }
}

class _WoxMultipleWindowRoot extends StatefulWidget {
  const _WoxMultipleWindowRoot({
    required this.id,
    required this.title,
    required this.controller,
    required this.navigatorKey,
    required this.theme,
    required this.showTitleBar,
    required this.resizable,
    required this.minimizable,
    required this.mica,
    required this.roundedCorners,
    required this.onPrepared,
    required this.builder,
  });

  final String id;
  final String title;
  final flutter_windowing.RegularWindowController controller;
  final GlobalKey<NavigatorState> navigatorKey;
  final ThemeData theme;
  final bool showTitleBar;
  final bool resizable;
  final bool minimizable;
  final bool mica;
  final bool roundedCorners;
  final VoidCallback onPrepared;
  final WidgetBuilder builder;

  @override
  State<_WoxMultipleWindowRoot> createState() => _WoxMultipleWindowRootState();
}

class _WoxMultipleWindowRootState extends State<_WoxMultipleWindowRoot> {
  Worker? _themeWorker;

  @override
  void initState() {
    super.initState();
    _themeWorker = ever(WoxThemeUtil.instance.currentTheme, (_) {
      _applyWindowStyle();
      if (mounted) {
        setState(() {});
      }
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      // The native window can report a stale zero/initial size until Flutter
      // presents its first frames, so defer style application and preparation
      // long enough for RegularWindowController.contentSize/GetWindowRect to
      // reflect the painted surface.
      Timer(const Duration(milliseconds: 120), () {
        if (!mounted) {
          return;
        }
        _applyWindowStyle();
        setState(() {});
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) {
            return;
          }
          widget.onPrepared();
        });
      });
    });
  }

  @override
  void didUpdateWidget(_WoxMultipleWindowRoot oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.mica != widget.mica || oldWidget.roundedCorners != widget.roundedCorners) {
      _applyWindowStyle();
    }
  }

  @override
  void dispose() {
    _themeWorker?.dispose();
    super.dispose();
  }

  void _applyWindowStyle() {
    // Multiple windows have their own native handles, so theme changes must
    // reapply the same appearance policy that the main query window receives.
    unawaited(
      WoxMultipleWindowStyle.apply(
        widget.controller,
        mica: widget.mica,
        darkMode: isThemeDark(),
        roundedCorners: widget.roundedCorners,
        minimizable: widget.minimizable,
        resizable: widget.resizable,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final transparentTheme = widget.theme.copyWith(scaffoldBackgroundColor: Colors.transparent);
    return MaterialApp(
      navigatorKey: widget.navigatorKey,
      debugShowCheckedModeBanner: false,
      color: Colors.transparent,
      theme: transparentTheme,
      home: Builder(
        builder: (context) {
          return _WoxMultipleWindowFrame(
            windowId: widget.id,
            title: widget.title,
            showTitleBar: widget.showTitleBar,
            resizable: widget.resizable,
            minimizable: widget.minimizable,
            roundedCorners: widget.roundedCorners,
            child: widget.builder(context),
          );
        },
      ),
    );
  }
}

class _WoxMultipleWindowFrame extends StatelessWidget {
  const _WoxMultipleWindowFrame({
    required this.windowId,
    required this.title,
    required this.showTitleBar,
    required this.resizable,
    required this.minimizable,
    required this.roundedCorners,
    required this.child,
  });

  final String windowId;
  final String title;
  final bool showTitleBar;
  final bool resizable;
  final bool minimizable;
  final bool roundedCorners;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final Widget frame;
    if (!showTitleBar) {
      frame = child;
    } else {
      frame = Material(
        type: MaterialType.transparency,
        child: Column(children: [_WoxSimulatedTitleBar(windowId: windowId, title: title, resizable: resizable, minimizable: minimizable), Expanded(child: child)]),
      );
    }

    if (Platform.isMacOS && roundedCorners) {
      return ClipRRect(borderRadius: BorderRadius.circular(_woxManagedWindowCornerRadius), child: frame);
    }
    return frame;
  }
}

class _WoxSimulatedTitleBar extends StatelessWidget {
  const _WoxSimulatedTitleBar({required this.windowId, required this.title, required this.resizable, required this.minimizable});

  static const double height = 40;

  final String windowId;
  final String title;
  final bool resizable;
  final bool minimizable;

  @override
  Widget build(BuildContext context) {
    final backgroundColor = getThemeBackgroundColor();
    final borderColor = getThemeDividerColor().withValues(alpha: isThemeDark() ? 0.42 : 0.30);
    final textColor = getThemeTextColor();

    return SizedBox(
      height: height,
      child: DecoratedBox(
        decoration: BoxDecoration(color: backgroundColor, border: Border(bottom: BorderSide(color: borderColor, width: 1))),
        child: Platform.isMacOS ? _buildMacTitleBar(textColor) : _buildDefaultTitleBar(textColor),
      ),
    );
  }

  Widget _buildDefaultTitleBar(Color textColor) {
    return Row(
      children: [
        Expanded(
          child: WoxMultipleWindowDragMoveArea(
            windowId: windowId,
            child: Padding(
              padding: const EdgeInsets.only(left: 12, right: 8),
              child: Row(
                children: [
                  const _WoxTitleBarAppMark(),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: textColor, fontSize: 13, fontWeight: FontWeight.w600, decoration: TextDecoration.none),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
        if (minimizable) _WoxTitleBarButton(icon: Icons.remove_rounded, onPressed: () => WoxMultipleWindow._minimizeWindow(windowId)),
        _WoxTitleBarButton(icon: Icons.close_rounded, isCloseButton: true, onPressed: () => unawaited(WoxMultipleWindow.closeWindow(windowId))),
      ],
    );
  }

  Widget _buildMacTitleBar(Color textColor) {
    return Row(
      children: [
        _WoxMacTrafficLightGroup(windowId: windowId, minimizable: minimizable, resizable: resizable),
        Expanded(
          child: WoxMultipleWindowDragMoveArea(
            windowId: windowId,
            child: Center(
              child: Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: textColor, fontSize: 13, fontWeight: FontWeight.w600, decoration: TextDecoration.none),
              ),
            ),
          ),
        ),
        const SizedBox(width: _WoxMacTrafficLightGroup.width),
      ],
    );
  }
}

class _WoxTitleBarAppMark extends StatelessWidget {
  const _WoxTitleBarAppMark();

  static final MemoryImage _iconImage = MemoryImage(base64Decode(WOX_ICON.split(";base64,").last));

  @override
  Widget build(BuildContext context) {
    return ClipRRect(borderRadius: BorderRadius.circular(4), child: Image(image: _iconImage, width: 20, height: 20, fit: BoxFit.cover));
  }
}

class _WoxTitleBarButton extends StatefulWidget {
  const _WoxTitleBarButton({required this.icon, required this.onPressed, this.isCloseButton = false});

  final IconData icon;
  final VoidCallback onPressed;
  final bool isCloseButton;

  @override
  State<_WoxTitleBarButton> createState() => _WoxTitleBarButtonState();
}

class _WoxTitleBarButtonState extends State<_WoxTitleBarButton> {
  bool _hovered = false;

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    final backgroundColor = _hovered ? (widget.isCloseButton && Platform.isWindows ? const Color(0xFFE81123) : textColor.withValues(alpha: 0.10)) : Colors.transparent;
    final iconColor = _hovered && widget.isCloseButton && Platform.isWindows ? Colors.white : textColor.withValues(alpha: 0.90);

    return MouseRegion(
      onEnter: (_) => setState(() => _hovered = true),
      onExit: (_) => setState(() => _hovered = false),
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: widget.onPressed,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 120),
          width: 46,
          height: _WoxSimulatedTitleBar.height,
          alignment: Alignment.center,
          color: backgroundColor,
          child: Icon(widget.icon, size: 18, color: iconColor),
        ),
      ),
    );
  }
}

class _WoxMacTrafficLightGroup extends StatefulWidget {
  const _WoxMacTrafficLightGroup({required this.windowId, required this.minimizable, required this.resizable});

  static const double _leftPadding = 18;
  static const double _rightPadding = 8;
  static const double _buttonSize = 20;
  static const double _buttonGap = 3;
  static const double width = _leftPadding + _rightPadding + (_buttonSize * 3) + (_buttonGap * 2);

  final String windowId;
  final bool minimizable;
  final bool resizable;

  @override
  State<_WoxMacTrafficLightGroup> createState() => _WoxMacTrafficLightGroupState();
}

class _WoxMacTrafficLightGroupState extends State<_WoxMacTrafficLightGroup> {
  bool _hovered = false;

  @override
  Widget build(BuildContext context) {
    return MouseRegion(
      hitTestBehavior: HitTestBehavior.opaque,
      onEnter: (_) => setState(() => _hovered = true),
      onExit: (_) => setState(() => _hovered = false),
      child: SizedBox(
        width: _WoxMacTrafficLightGroup.width,
        height: _WoxSimulatedTitleBar.height,
        child: Padding(
          padding: const EdgeInsets.only(left: _WoxMacTrafficLightGroup._leftPadding, right: _WoxMacTrafficLightGroup._rightPadding),
          child: Row(
            children: [
              _WoxMacTrafficLight(kind: _WoxMacTrafficLightKind.close, hovered: _hovered, onPressed: () => unawaited(WoxMultipleWindow.closeWindow(widget.windowId))),
              const SizedBox(width: _WoxMacTrafficLightGroup._buttonGap),
              _WoxMacTrafficLight(
                kind: _WoxMacTrafficLightKind.minimize,
                hovered: _hovered,
                onPressed: widget.minimizable ? () => WoxMultipleWindow._minimizeWindow(widget.windowId) : null,
              ),
              const SizedBox(width: _WoxMacTrafficLightGroup._buttonGap),
              _WoxMacTrafficLight(
                kind: _WoxMacTrafficLightKind.zoom,
                hovered: _hovered,
                onPressed: widget.resizable ? () => WoxMultipleWindow._toggleMaximizeWindow(widget.windowId) : null,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

enum _WoxMacTrafficLightKind { close, minimize, zoom }

class _WoxMacTrafficLight extends StatelessWidget {
  const _WoxMacTrafficLight({required this.kind, required this.hovered, required this.onPressed});

  final _WoxMacTrafficLightKind kind;
  final bool hovered;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    final enabled = onPressed != null;
    final color = switch (kind) {
      _WoxMacTrafficLightKind.close => const Color(0xFFFF5F57),
      _WoxMacTrafficLightKind.minimize => enabled ? const Color(0xFFFFBD2E) : const Color(0xFF6E6E73),
      _WoxMacTrafficLightKind.zoom => enabled ? const Color(0xFF28C840) : const Color(0xFF6E6E73),
    };

    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onPressed,
      child: SizedBox(
        width: _WoxMacTrafficLightGroup._buttonSize,
        height: _WoxMacTrafficLightGroup._buttonSize,
        child: Center(
          child: SizedBox(
            width: 14,
            height: 14,
            child: DecoratedBox(
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              child: hovered && enabled ? CustomPaint(painter: _WoxMacTrafficLightGlyphPainter(kind)) : const SizedBox.expand(),
            ),
          ),
        ),
      ),
    );
  }
}

class _WoxMacTrafficLightGlyphPainter extends CustomPainter {
  const _WoxMacTrafficLightGlyphPainter(this.kind);

  final _WoxMacTrafficLightKind kind;

  @override
  void paint(Canvas canvas, Size size) {
    final paint =
        Paint()
          ..color = Colors.black.withValues(alpha: 0.58)
          ..strokeCap = StrokeCap.round
          ..strokeWidth = 1.45
          ..style = PaintingStyle.stroke;
    final center = Offset(size.width / 2, size.height / 2);
    const half = 2.6;

    switch (kind) {
      case _WoxMacTrafficLightKind.close:
        canvas.drawLine(center.translate(-half, -half), center.translate(half, half), paint);
        canvas.drawLine(center.translate(half, -half), center.translate(-half, half), paint);
      case _WoxMacTrafficLightKind.minimize:
        canvas.drawLine(center.translate(-3.0, 0), center.translate(3.0, 0), paint);
      case _WoxMacTrafficLightKind.zoom:
        canvas.drawLine(center.translate(-3.0, 0), center.translate(3.0, 0), paint);
        canvas.drawLine(center.translate(0, -3.0), center.translate(0, 3.0), paint);
    }
  }

  @override
  bool shouldRepaint(_WoxMacTrafficLightGlyphPainter oldDelegate) => oldDelegate.kind != kind;
}

class WoxMultipleWindowDragMoveArea extends StatelessWidget {
  const WoxMultipleWindowDragMoveArea({super.key, required this.windowId, required this.child});

  final String windowId;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(behavior: HitTestBehavior.translucent, onPanStart: (_) => WoxMultipleWindow.startDragging(windowId), child: child);
  }
}
