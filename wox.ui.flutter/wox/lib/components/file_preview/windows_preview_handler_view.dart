import 'dart:async';
import 'dart:io';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';

typedef WoxPreviewHandlerFallbackBuilder = Widget Function(String? error);

/// Hosts the Windows shell preview handler in a native child window that follows
/// this Flutter widget's bounds.
class WoxWindowsPreviewHandlerView extends StatefulWidget {
  final String filePath;
  final WoxPreviewHandlerFallbackBuilder fallbackBuilder;

  const WoxWindowsPreviewHandlerView({super.key, required this.filePath, required this.fallbackBuilder});

  @override
  State<WoxWindowsPreviewHandlerView> createState() => _WoxWindowsPreviewHandlerViewState();
}

class _WoxWindowsPreviewHandlerViewState extends State<WoxWindowsPreviewHandlerView> with WidgetsBindingObserver {
  static const MethodChannel _channel = MethodChannel("com.wox.preview_handler");
  static bool _escapeHandlerRegistered = false;

  final GlobalKey _viewKey = GlobalKey();
  Future<int?>? _instanceFuture;
  int? _instanceId;
  String? _error;
  Rect? _lastPhysicalBounds;
  bool _boundsReportScheduled = false;

  @override
  void initState() {
    super.initState();
    _ensureEscapeHandlerRegistered();
    WidgetsBinding.instance.addObserver(this);
    _instanceFuture = _createInstance();
  }

  @override
  void didUpdateWidget(covariant WoxWindowsPreviewHandlerView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.filePath != widget.filePath) {
      unawaited(_disposeInstance());
      _error = null;
      _lastPhysicalBounds = null;
      _instanceFuture = _createInstance();
    } else {
      _scheduleBoundsReport();
    }
  }

  @override
  void didChangeMetrics() {
    _lastPhysicalBounds = null;
    _scheduleBoundsReport();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    unawaited(_disposeInstance());
    super.dispose();
  }

  /// Creates the native preview handler after the first layout pass has supplied
  /// an approximate host rectangle.
  Future<int?> _createInstance() async {
    if (!Platform.isWindows) {
      _error = "not_windows";
      return null;
    }

    try {
      final bounds = await _waitForInitialPhysicalBounds();
      if (!mounted) {
        return null;
      }
      if (bounds == null) {
        _error = "layout_unavailable";
        return null;
      }
      final result = await _channel.invokeMapMethod<String, dynamic>("create", {
        "filePath": widget.filePath,
        "x": bounds.left,
        "y": bounds.top,
        "width": bounds.width,
        "height": bounds.height,
      });
      final id = result?["id"];
      if (id is int) {
        _instanceId = id;
      } else if (id is num) {
        _instanceId = id.toInt();
      }
      _scheduleBoundsReport();
      return _instanceId;
    } on PlatformException catch (error) {
      _error = error.message?.trim().isNotEmpty == true ? error.message : error.code;
      return null;
    } catch (error) {
      _error = error.toString();
      return null;
    }
  }

  /// Waits for Flutter to publish a real preview rectangle so native handlers
  /// are never shown at the view origin and then moved into place.
  Future<Rect?> _waitForInitialPhysicalBounds() async {
    for (var attempt = 0; attempt < 8; attempt++) {
      await WidgetsBinding.instance.endOfFrame;
      if (!mounted) {
        return null;
      }

      final bounds = _currentPhysicalBounds();
      if (bounds != null && bounds.width > 1 && bounds.height > 1) {
        return bounds;
      }
    }

    return _currentPhysicalBounds();
  }

  /// Releases the native host window before Flutter removes or replaces this
  /// preview widget.
  Future<void> _disposeInstance() async {
    final id = _instanceId;
    _instanceId = null;
    if (id == null) {
      return;
    }

    try {
      await _channel.invokeMethod("dispose", {"id": id});
    } catch (_) {
      // Native preview handlers can be torn down during process/window shutdown.
    }
  }

  /// Converts this widget's logical Flutter bounds into physical pixels relative
  /// to the Flutter view HWND used by the native host.
  Rect? _currentPhysicalBounds() {
    final context = _viewKey.currentContext;
    final box = context?.findRenderObject() as RenderBox?;
    if (context == null || box == null || !box.hasSize) {
      return null;
    }

    final topLeft = box.localToGlobal(Offset.zero);
    final devicePixelRatio = View.of(context).devicePixelRatio;
    final width = math.max(1.0, box.size.width * devicePixelRatio);
    final height = math.max(1.0, box.size.height * devicePixelRatio);
    return Rect.fromLTWH(topLeft.dx * devicePixelRatio, topLeft.dy * devicePixelRatio, width, height);
  }

  /// Coalesces repeated layout/build notifications into one native bounds update.
  void _scheduleBoundsReport() {
    if (_boundsReportScheduled) {
      return;
    }

    _boundsReportScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _boundsReportScheduled = false;
      unawaited(_reportBounds());
    });
  }

  /// Keeps the native preview child window aligned with the Flutter preview area.
  Future<void> _reportBounds() async {
    final id = _instanceId;
    if (id == null || !mounted) {
      return;
    }

    final bounds = _currentPhysicalBounds();
    if (bounds == null || bounds == _lastPhysicalBounds) {
      return;
    }

    _lastPhysicalBounds = bounds;
    try {
      await _channel.invokeMethod("setBounds", {"id": id, "x": bounds.left, "y": bounds.top, "width": bounds.width, "height": bounds.height});
    } catch (error) {
      if (mounted) {
        setState(() => _error = error.toString());
      }
    }
  }

  static void _ensureEscapeHandlerRegistered() {
    if (_escapeHandlerRegistered) {
      return;
    }

    // Native Office preview windows can own keyboard focus, so Escape arrives
    // through this method channel instead of Flutter's key event pipeline.
    _escapeHandlerRegistered = true;
    _channel.setMethodCallHandler((call) async {
      if (call.method != "onEscapePressed") {
        return null;
      }
      if (!Get.isRegistered<WoxLauncherController>()) {
        return null;
      }

      await Get.find<WoxLauncherController>().hideApp(const UuidV4().generate());
      return null;
    });
  }

  @override
  Widget build(BuildContext context) {
    _scheduleBoundsReport();

    return SizedBox.expand(
      key: _viewKey,
      child: FutureBuilder<int?>(
        future: _instanceFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const Center(child: WoxLoadingIndicator(size: 20));
          }
          if (snapshot.data == null) {
            return widget.fallbackBuilder(_error);
          }

          return const SizedBox.expand();
        },
      ),
    );
  }
}
