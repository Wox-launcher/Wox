// ignore_for_file: invalid_use_of_internal_member, implementation_imports

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/src/widgets/_window.dart' as flutter_windowing;
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window_style.dart';

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

  Future<void> focus();

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

  /// Creates or focuses a top-level Flutter window managed through Flutter windowing.
  ///
  /// When [showTitleBar] is true, the titlebar is Flutter-drawn; the native
  /// frame is still removed so theme and backdrop behavior stay consistent.
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
          minimizable: minimizable,
          mica: mica,
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
      WoxMultipleWindowStyle.moveOffscreen(controller);
    }
    registry.register(entry);
    controller.activate();
    if (prepareOffscreen) {
      await preparationReady.future.timeout(const Duration(milliseconds: 500), onTimeout: () {});
    }
    WoxMultipleWindowStyle.centerOnCursorDisplay(controller);
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

  @override
  Future<void> focus() async {
    controller.activate();
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
    required this.minimizable,
    required this.mica,
    required this.onPrepared,
    required this.builder,
  });

  final String id;
  final String title;
  final flutter_windowing.RegularWindowController controller;
  final GlobalKey<NavigatorState> navigatorKey;
  final ThemeData theme;
  final bool showTitleBar;
  final bool minimizable;
  final bool mica;
  final VoidCallback onPrepared;
  final WidgetBuilder builder;

  @override
  State<_WoxMultipleWindowRoot> createState() => _WoxMultipleWindowRootState();
}

class _WoxMultipleWindowRootState extends State<_WoxMultipleWindowRoot> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      Timer(const Duration(milliseconds: 120), () {
        if (!mounted) {
          return;
        }
        WoxMultipleWindowStyle.apply(widget.controller, mica: widget.mica);
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
            minimizable: widget.minimizable,
            child: widget.builder(context),
          );
        },
      ),
    );
  }
}

class _WoxMultipleWindowFrame extends StatelessWidget {
  const _WoxMultipleWindowFrame({required this.windowId, required this.title, required this.showTitleBar, required this.minimizable, required this.child});

  final String windowId;
  final String title;
  final bool showTitleBar;
  final bool minimizable;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    if (!showTitleBar) {
      return child;
    }

    return Material(
      type: MaterialType.transparency,
      child: Column(children: [_WoxSimulatedTitleBar(windowId: windowId, title: title, minimizable: minimizable), Expanded(child: child)]),
    );
  }
}

class _WoxSimulatedTitleBar extends StatelessWidget {
  const _WoxSimulatedTitleBar({required this.windowId, required this.title, required this.minimizable});

  static const double height = 40;

  final String windowId;
  final String title;
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
        Padding(
          padding: const EdgeInsets.only(left: 12),
          child: Row(
            children: [
              _WoxMacTrafficLight(color: const Color(0xFFFF5F57), onPressed: () => unawaited(WoxMultipleWindow.closeWindow(windowId))),
              if (minimizable) ...[const SizedBox(width: 8), _WoxMacTrafficLight(color: const Color(0xFFFFBD2E), onPressed: () => WoxMultipleWindow._minimizeWindow(windowId))],
            ],
          ),
        ),
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
        const SizedBox(width: 68),
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

class _WoxMacTrafficLight extends StatelessWidget {
  const _WoxMacTrafficLight({required this.color, required this.onPressed});

  final Color color;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onPressed,
      child: Padding(
        padding: const EdgeInsets.all(6),
        child: DecoratedBox(decoration: BoxDecoration(color: color, shape: BoxShape.circle), child: const SizedBox(width: 12, height: 12)),
      ),
    );
  }
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
