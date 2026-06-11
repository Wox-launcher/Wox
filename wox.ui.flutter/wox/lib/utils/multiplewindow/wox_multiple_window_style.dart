// ignore_for_file: invalid_use_of_internal_member, implementation_imports

// Native helpers for Flutter's experimental internal windowing API. This file
// imports platform-specific `_window_*` internals to reach the HWND/NSWindow
// handles used for frame removal, placement, topmost state, and custom
// dragging. Recheck it when upgrading Flutter; it was first verified on Flutter
// 3.45.0-1.0.pre-196, master revision 2731746a84.

import 'dart:async';
import 'dart:ffi' as ffi;
import 'dart:io';

import 'package:ffi/ffi.dart';
import 'package:flutter/services.dart';
import 'package:flutter/src/widgets/_window_macos.dart' as macos_windowing;
import 'package:flutter/src/widgets/_window_win32.dart' as win32_windowing;

const int _gwlStyle = -16;

const int _wsPopup = 0x80000000;
const int _wsCaption = 0x00C00000;
const int _wsThickFrame = 0x00040000;
const int _wsMinimizeBox = 0x00020000;
const int _wsMaximizeBox = 0x00010000;
const int _wsSysMenu = 0x00080000;
const int _wsBorder = 0x00800000;
const int _wsDlgFrame = 0x00400000;

const int _swpNoMove = 0x0002;
const int _swpNoSize = 0x0001;
const int _swpNoZOrder = 0x0004;
const int _swpNoActivate = 0x0010;
const int _swpFrameChanged = 0x0020;

const int _wmNcLButtonDown = 0x00A1;
const int _htCaption = 2;
const int _monitorDefaultToNearest = 2;
const int _defaultDpi = 96;
const int _mdtEffectiveDpi = 0;
const int _offscreenCoordinate = -32000;
const int _swHide = 0;
const int _swShow = 5;
const int _hwndTopmost = -1;
const int _hwndNoTopmost = -2;

const int _dwmwaUseImmersiveDarkMode = 20;
const int _dwmwaWindowCornerPreference = 33;
const int _dwmwaSystemBackdropType = 38;
const int _dwmcpDoNotRound = 1;
const int _dwmcpRound = 2;
const int _dwmsbtNone = 0;
const int _dwmsbtTabbedWindow = 3;

final class _Margins extends ffi.Struct {
  @ffi.Int32()
  external int cxLeftWidth;

  @ffi.Int32()
  external int cxRightWidth;

  @ffi.Int32()
  external int cyTopHeight;

  @ffi.Int32()
  external int cyBottomHeight;
}

final class _WindowPoint extends ffi.Struct {
  @ffi.Int32()
  external int x;

  @ffi.Int32()
  external int y;
}

final class _WindowRect extends ffi.Struct {
  @ffi.Int32()
  external int left;

  @ffi.Int32()
  external int top;

  @ffi.Int32()
  external int right;

  @ffi.Int32()
  external int bottom;
}

final class _MonitorInfo extends ffi.Struct {
  @ffi.Uint32()
  external int cbSize;

  external _WindowRect rcMonitor;

  external _WindowRect rcWork;

  @ffi.Uint32()
  external int dwFlags;
}

// Carries the target logical point and monitor match result through the Win32
// EnumDisplayMonitors callback.
final class _MonitorFindData extends ffi.Struct {
  @ffi.Int32()
  external int targetX;

  @ffi.Int32()
  external int targetY;

  external ffi.Pointer<ffi.Void> foundMonitor;

  @ffi.Uint32()
  external int foundDpi;
}

typedef _GetWindowLongPtrNative = ffi.IntPtr Function(ffi.Pointer<ffi.Void>, ffi.Int32);
typedef _GetWindowLongPtrDart = int Function(ffi.Pointer<ffi.Void>, int);
typedef _SetWindowLongPtrNative = ffi.IntPtr Function(ffi.Pointer<ffi.Void>, ffi.Int32, ffi.IntPtr);
typedef _SetWindowLongPtrDart = int Function(ffi.Pointer<ffi.Void>, int, int);
typedef _SetWindowPosNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<ffi.Void>, ffi.Int32, ffi.Int32, ffi.Int32, ffi.Int32, ffi.Uint32);
typedef _SetWindowPosDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<ffi.Void>, int, int, int, int, int);
typedef _GetCursorPosNative = ffi.Int32 Function(ffi.Pointer<_WindowPoint>);
typedef _GetCursorPosDart = int Function(ffi.Pointer<_WindowPoint>);
typedef _MonitorFromRectNative = ffi.Pointer<ffi.Void> Function(ffi.Pointer<_WindowRect>, ffi.Uint32);
typedef _MonitorFromRectDart = ffi.Pointer<ffi.Void> Function(ffi.Pointer<_WindowRect>, int);
typedef _MonitorEnumProcNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowRect>, ffi.IntPtr);
typedef _EnumDisplayMonitorsNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowRect>, ffi.Pointer<ffi.NativeFunction<_MonitorEnumProcNative>>, ffi.IntPtr);
typedef _EnumDisplayMonitorsDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowRect>, ffi.Pointer<ffi.NativeFunction<_MonitorEnumProcNative>>, int);
typedef _GetMonitorInfoNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_MonitorInfo>);
typedef _GetMonitorInfoDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_MonitorInfo>);
typedef _GetWindowRectNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowRect>);
typedef _GetWindowRectDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowRect>);
typedef _GetDpiForMonitorNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Int32, ffi.Pointer<ffi.Uint32>, ffi.Pointer<ffi.Uint32>);
typedef _GetDpiForMonitorDart = int Function(ffi.Pointer<ffi.Void>, int, ffi.Pointer<ffi.Uint32>, ffi.Pointer<ffi.Uint32>);
typedef _ReleaseCaptureNative = ffi.Int32 Function();
typedef _ReleaseCaptureDart = int Function();
typedef _SendMessageNative = ffi.IntPtr Function(ffi.Pointer<ffi.Void>, ffi.Uint32, ffi.UintPtr, ffi.IntPtr);
typedef _SendMessageDart = int Function(ffi.Pointer<ffi.Void>, int, int, int);
typedef _ShowWindowNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Int32);
typedef _ShowWindowDart = int Function(ffi.Pointer<ffi.Void>, int);
typedef _DwmSetWindowAttributeNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Uint32, ffi.Pointer<ffi.Void>, ffi.Uint32);
typedef _DwmSetWindowAttributeDart = int Function(ffi.Pointer<ffi.Void>, int, ffi.Pointer<ffi.Void>, int);
typedef _DwmExtendFrameIntoClientAreaNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_Margins>);
typedef _DwmExtendFrameIntoClientAreaDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_Margins>);

class WoxMultipleWindowStyle {
  static const MethodChannel _macosChannel = MethodChannel("com.wox.macos_window_manager");

  static final ffi.DynamicLibrary? _user32 = Platform.isWindows ? ffi.DynamicLibrary.open("user32.dll") : null;
  static final ffi.DynamicLibrary? _shcore = Platform.isWindows ? ffi.DynamicLibrary.open("shcore.dll") : null;
  static final ffi.DynamicLibrary? _dwmapi = Platform.isWindows ? ffi.DynamicLibrary.open("dwmapi.dll") : null;

  static final _GetWindowLongPtrDart? _getWindowLongPtr = _user32?.lookupFunction<_GetWindowLongPtrNative, _GetWindowLongPtrDart>("GetWindowLongPtrW");
  static final _SetWindowLongPtrDart? _setWindowLongPtr = _user32?.lookupFunction<_SetWindowLongPtrNative, _SetWindowLongPtrDart>("SetWindowLongPtrW");
  static final _SetWindowPosDart? _setWindowPos = _user32?.lookupFunction<_SetWindowPosNative, _SetWindowPosDart>("SetWindowPos");
  static final _GetCursorPosDart? _getCursorPos = _user32?.lookupFunction<_GetCursorPosNative, _GetCursorPosDart>("GetCursorPos");
  static final _MonitorFromRectDart? _monitorFromRect = _user32?.lookupFunction<_MonitorFromRectNative, _MonitorFromRectDart>("MonitorFromRect");
  static final _EnumDisplayMonitorsDart? _enumDisplayMonitors = _user32?.lookupFunction<_EnumDisplayMonitorsNative, _EnumDisplayMonitorsDart>("EnumDisplayMonitors");
  static final _GetMonitorInfoDart? _getMonitorInfo = _user32?.lookupFunction<_GetMonitorInfoNative, _GetMonitorInfoDart>("GetMonitorInfoW");
  static final _GetWindowRectDart? _getWindowRect = _user32?.lookupFunction<_GetWindowRectNative, _GetWindowRectDart>("GetWindowRect");
  static final _GetDpiForMonitorDart? _getDpiForMonitor = _shcore?.lookupFunction<_GetDpiForMonitorNative, _GetDpiForMonitorDart>("GetDpiForMonitor");
  static final _ReleaseCaptureDart? _releaseCapture = _user32?.lookupFunction<_ReleaseCaptureNative, _ReleaseCaptureDart>("ReleaseCapture");
  static final _SendMessageDart? _sendMessage = _user32?.lookupFunction<_SendMessageNative, _SendMessageDart>("SendMessageW");
  static final _ShowWindowDart? _showWindow = _user32?.lookupFunction<_ShowWindowNative, _ShowWindowDart>("ShowWindow");
  static final _DwmSetWindowAttributeDart? _dwmSetWindowAttribute = _dwmapi?.lookupFunction<_DwmSetWindowAttributeNative, _DwmSetWindowAttributeDart>("DwmSetWindowAttribute");
  static final _DwmExtendFrameIntoClientAreaDart? _dwmExtendFrameIntoClientArea = _dwmapi?.lookupFunction<_DwmExtendFrameIntoClientAreaNative, _DwmExtendFrameIntoClientAreaDart>(
    "DwmExtendFrameIntoClientArea",
  );

  /// Applies native chrome policy for a Flutter windowing controller.
  static Future<void> apply(Object controller, {required bool mica, required bool darkMode, bool roundedCorners = true, bool minimizable = true, bool resizable = false}) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "applyManagedWindowStyle", {
        "mica": mica,
        "darkMode": darkMode,
        "roundedCorners": roundedCorners,
        "minimizable": minimizable,
        "resizable": resizable,
      });
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return;
    }

    // The wrapper draws any requested titlebar in Flutter. Native chrome is
    // always removed so all platforms share the same visual contract.
    _removeNativeFrame(hwnd);
    _setDarkMode(hwnd, darkMode);
    if (roundedCorners) {
      _enableRoundedCorners(hwnd);
    } else {
      _disableRoundedCorners(hwnd);
    }
    if (mica) {
      _enableBackdrop(hwnd);
    } else {
      _disableBackdrop(hwnd);
    }
    _refreshWindowFrame(hwnd);
  }

  /// Centers a newly created window on the display currently containing the mouse cursor.
  static Future<void> centerOnCursorDisplay(Object controller, {Size? preferredSize}) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "center", {
        if (preferredSize != null) ...{"width": preferredSize.width, "height": preferredSize.height},
      });
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    final setWindowPos = _setWindowPos;
    final getCursorPos = _getCursorPos;
    final monitorFromRect = _monitorFromRect;
    final getMonitorInfo = _getMonitorInfo;
    final getWindowRect = _getWindowRect;
    if (hwnd == null || hwnd.address == 0 || setWindowPos == null || getCursorPos == null || monitorFromRect == null || getMonitorInfo == null || getWindowRect == null) {
      return;
    }

    final cursor = calloc<_WindowPoint>();
    final cursorRect = calloc<_WindowRect>();
    final monitorInfo = calloc<_MonitorInfo>();
    final windowRect = calloc<_WindowRect>();
    try {
      if (getCursorPos(cursor) == 0) {
        return;
      }

      cursorRect.ref
        ..left = cursor.ref.x
        ..top = cursor.ref.y
        ..right = cursor.ref.x + 1
        ..bottom = cursor.ref.y + 1;

      final monitor = monitorFromRect(cursorRect, _monitorDefaultToNearest);
      if (monitor.address == 0) {
        return;
      }

      monitorInfo.ref.cbSize = ffi.sizeOf<_MonitorInfo>();
      if (getMonitorInfo(monitor, monitorInfo) == 0 || getWindowRect(hwnd, windowRect) == 0) {
        return;
      }

      final workArea = monitorInfo.ref.rcWork;
      final targetDpiScale = _dpiScaleForMonitor(monitor);
      final hasPreferredSize = preferredSize != null && targetDpiScale > 0;
      final windowWidth = hasPreferredSize ? (preferredSize.width * targetDpiScale).round() : windowRect.ref.right - windowRect.ref.left;
      final windowHeight = hasPreferredSize ? (preferredSize.height * targetDpiScale).round() : windowRect.ref.bottom - windowRect.ref.top;
      if (windowWidth <= 0 || windowHeight <= 0) {
        return;
      }

      final x = workArea.left + ((workArea.right - workArea.left - windowWidth) / 2).round();
      final y = workArea.top + ((workArea.bottom - workArea.top - windowHeight) / 2).round();
      setWindowPos(hwnd, ffi.nullptr, x, y, 0, 0, _swpNoSize | _swpNoZOrder | _swpFrameChanged);
    } finally {
      calloc.free(windowRect);
      calloc.free(monitorInfo);
      calloc.free(cursorRect);
      calloc.free(cursor);
    }
  }

  /// Caps a preferred window size to the cursor display work area before the window is created.
  static Size constrainSizeToCursorDisplayWorkArea(Size preferredSize, {double maxWorkAreaFraction = 1}) {
    if (!Platform.isWindows) {
      return preferredSize;
    }

    final availableWorkAreaSize = _cursorDisplayLogicalWorkAreaSize();
    if (availableWorkAreaSize == null) {
      return preferredSize;
    }

    final safeMaxWorkAreaFraction = maxWorkAreaFraction <= 0 ? 1.0 : maxWorkAreaFraction.clamp(0.0, 1.0).toDouble();
    final maxWidth = availableWorkAreaSize.width * safeMaxWorkAreaFraction;
    final maxHeight = availableWorkAreaSize.height * safeMaxWorkAreaFraction;
    final constrainedWidth = preferredSize.width > maxWidth ? maxWidth.floorToDouble() : preferredSize.width;
    final constrainedHeight = preferredSize.height > maxHeight ? maxHeight.floorToDouble() : preferredSize.height;
    if (constrainedWidth <= 0 || constrainedHeight <= 0) {
      return preferredSize;
    }
    return Size(constrainedWidth, constrainedHeight);
  }

  /// Moves and resizes a managed window where the current platform exposes native positioning.
  static Future<void> setBounds(Object controller, Offset position, Size size) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "setBounds", {"x": position.dx, "y": position.dy, "width": size.width, "height": size.height});
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    final setWindowPos = _setWindowPos;
    if (hwnd == null || hwnd.address == 0 || setWindowPos == null) {
      return;
    }

    // Core sends the same logical coordinates used by the primary window.
    // Convert with the target monitor DPI before calling Win32 so secondary
    // windows follow the primary placement contract on high-DPI displays.
    final dpiScale = _dpiScaleForLogicalPosition(position);
    setWindowPos(
      hwnd,
      ffi.nullptr,
      (position.dx * dpiScale).toInt(),
      (position.dy * dpiScale).toInt(),
      (size.width * dpiScale).toInt(),
      (size.height * dpiScale).toInt(),
      _swpNoZOrder | _swpFrameChanged,
    );
  }

  /// Resizes a managed window where controller-only resize does not update the native frame.
  static Future<void> setSize(Object controller, Size size) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "setSize", {"width": size.width, "height": size.height});
    }
  }

  /// Returns the logical top-left position for platforms where Wox currently needs it.
  static Future<Offset?> positionOf(Object controller) async {
    if (Platform.isMacOS) {
      final result = await _invokeMacOSWindowMethod<Map<dynamic, dynamic>>(controller, "getPosition");
      if (result == null) {
        return null;
      }
      final x = result["x"];
      final y = result["y"];
      if (x is num && y is num) {
        return Offset(x.toDouble(), y.toDouble());
      }
      return null;
    }

    if (!Platform.isWindows) {
      return null;
    }

    final hwnd = _windowHandleOf(controller);
    final getWindowRect = _getWindowRect;
    if (hwnd == null || hwnd.address == 0 || getWindowRect == null) {
      return null;
    }

    final windowRect = calloc<_WindowRect>();
    try {
      if (getWindowRect(hwnd, windowRect) == 0) {
        return null;
      }
      final dpiScale = _dpiScaleForNativeRect(windowRect);
      return Offset(windowRect.ref.left / dpiScale, windowRect.ref.top / dpiScale);
    } finally {
      calloc.free(windowRect);
    }
  }

  /// Applies topmost state for managed windows where native handles are exposed.
  static Future<void> setAlwaysOnTop(Object controller, bool value) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "setAlwaysOnTop", {"value": value});
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return;
    }

    final insertAfter = ffi.Pointer<ffi.Void>.fromAddress(value ? _hwndTopmost : _hwndNoTopmost);
    _setWindowPos?.call(hwnd, insertAfter, 0, 0, 0, 0, _swpNoMove | _swpNoSize | _swpFrameChanged);
  }

  /// Moves a new window outside visible work areas while Flutter paints its first frames.
  static Future<void> moveOffscreen(Object controller) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "setPosition", {"x": _offscreenCoordinate.toDouble(), "y": _offscreenCoordinate.toDouble()});
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return;
    }

    _setWindowPos?.call(hwnd, ffi.nullptr, _offscreenCoordinate, _offscreenCoordinate, 0, 0, _swpNoSize | _swpNoZOrder | _swpNoActivate | _swpFrameChanged);
  }

  /// Hides a managed window without destroying its Flutter view.
  static Future<void> hide(Object controller) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "hide");
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return;
    }

    _showWindow?.call(hwnd, _swHide);
  }

  /// Shows a managed window that was hidden without destroying its Flutter view.
  static Future<void> show(Object controller) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "show");
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return;
    }

    _showWindow?.call(hwnd, _swShow);
  }

  /// Focuses a managed window where Wox needs native focus semantics.
  static Future<void> focus(Object controller) async {
    if (Platform.isMacOS) {
      await _invokeMacOSWindowMethod<void>(controller, "focus");
      return;
    }
  }

  /// Starts native dragging for a custom Flutter-drawn title area.
  static void startDragging(Object controller) {
    if (Platform.isMacOS) {
      unawaited(_invokeMacOSWindowMethod<void>(controller, "startDragging"));
      return;
    }

    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return;
    }

    _releaseCapture?.call();
    _sendMessage?.call(hwnd, _wmNcLButtonDown, _htCaption, 0);
  }

  /// Returns the native window handle for bridge calls that must target this window.
  static int? nativeHandleOf(Object controller) {
    if (Platform.isMacOS) {
      return _macOSWindowHandleOf(controller);
    }

    if (!Platform.isWindows) {
      return null;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return null;
    }
    return hwnd.address;
  }

  static Future<T?> _invokeMacOSWindowMethod<T>(Object controller, String method, [Map<String, dynamic>? arguments]) async {
    final windowHandle = _macOSWindowHandleOf(controller);
    if (windowHandle == null) {
      return null;
    }

    final payload = <String, dynamic>{...?arguments, "windowHandle": windowHandle};
    try {
      return await _macosChannel.invokeMethod<T>(method, payload);
    } on MissingPluginException {
      return null;
    } on PlatformException {
      return null;
    }
  }

  static int? _macOSWindowHandleOf(Object controller) {
    if (controller is macos_windowing.RegularWindowControllerMacOS) {
      final handle = controller.windowHandle;
      if (handle.address != 0) {
        return handle.address;
      }
    }
    return null;
  }

  static ffi.Pointer<ffi.Void>? _windowHandleOf(Object controller) {
    if (controller is win32_windowing.RegularWindowControllerWin32) {
      return controller.windowHandle;
    }
    return null;
  }

  static void _removeNativeFrame(ffi.Pointer<ffi.Void> hwnd) {
    final getWindowLongPtr = _getWindowLongPtr;
    final setWindowLongPtr = _setWindowLongPtr;
    if (getWindowLongPtr == null || setWindowLongPtr == null) {
      return;
    }

    final style = getWindowLongPtr(hwnd, _gwlStyle);
    final updatedStyle = (style | _wsPopup) & ~(_wsCaption | _wsThickFrame | _wsMinimizeBox | _wsMaximizeBox | _wsSysMenu | _wsBorder | _wsDlgFrame);
    setWindowLongPtr(hwnd, _gwlStyle, updatedStyle);
  }

  static void _setDarkMode(ffi.Pointer<ffi.Void> hwnd, bool enabled) {
    final value = calloc<ffi.Int32>();
    try {
      value.value = enabled ? 1 : 0;
      _dwmSetWindowAttribute?.call(hwnd, _dwmwaUseImmersiveDarkMode, value.cast<ffi.Void>(), ffi.sizeOf<ffi.Int32>());
    } finally {
      calloc.free(value);
    }
  }

  static void _enableRoundedCorners(ffi.Pointer<ffi.Void> hwnd) {
    final value = calloc<ffi.Int32>();
    try {
      value.value = _dwmcpRound;
      _dwmSetWindowAttribute?.call(hwnd, _dwmwaWindowCornerPreference, value.cast<ffi.Void>(), ffi.sizeOf<ffi.Int32>());
    } finally {
      calloc.free(value);
    }
  }

  static void _disableRoundedCorners(ffi.Pointer<ffi.Void> hwnd) {
    final value = calloc<ffi.Int32>();
    try {
      value.value = _dwmcpDoNotRound;
      _dwmSetWindowAttribute?.call(hwnd, _dwmwaWindowCornerPreference, value.cast<ffi.Void>(), ffi.sizeOf<ffi.Int32>());
    } finally {
      calloc.free(value);
    }
  }

  static void _enableBackdrop(ffi.Pointer<ffi.Void> hwnd) {
    // Match the main query window's Windows 11 DWM path. Acrylic accent tints
    // produce a different blend under translucent Wox theme backgrounds.
    _extendFrame(hwnd, -1);
    _setDwmBackdrop(hwnd, _dwmsbtTabbedWindow);
  }

  static void _disableBackdrop(ffi.Pointer<ffi.Void> hwnd) {
    _extendFrame(hwnd, 0);
    _setDwmBackdrop(hwnd, _dwmsbtNone);
  }

  static void _extendFrame(ffi.Pointer<ffi.Void> hwnd, int margin) {
    final margins = calloc<_Margins>();
    try {
      margins.ref
        ..cxLeftWidth = margin
        ..cxRightWidth = margin
        ..cyTopHeight = margin
        ..cyBottomHeight = margin;
      _dwmExtendFrameIntoClientArea?.call(hwnd, margins);
    } finally {
      calloc.free(margins);
    }
  }

  static void _setDwmBackdrop(ffi.Pointer<ffi.Void> hwnd, int backdropType) {
    final backdrop = calloc<ffi.Int32>();
    try {
      backdrop.value = backdropType;
      _dwmSetWindowAttribute?.call(hwnd, _dwmwaSystemBackdropType, backdrop.cast<ffi.Void>(), ffi.sizeOf<ffi.Int32>());
    } finally {
      calloc.free(backdrop);
    }
  }

  static void _refreshWindowFrame(ffi.Pointer<ffi.Void> hwnd) {
    _setWindowPos?.call(hwnd, ffi.nullptr, 0, 0, 0, 0, _swpNoMove | _swpNoSize | _swpNoZOrder | _swpFrameChanged);
  }

  static double _dpiScaleForLogicalPosition(Offset position) {
    final dpi = _dpiForLogicalPosition(position);
    return dpi <= 0 ? 1 : dpi / _defaultDpi;
  }

  // Matches the primary window's native setBounds behavior: locate the monitor
  // by logical bounds first, then use that monitor's DPI for physical Win32
  // coordinates.
  static int _dpiForLogicalPosition(Offset position) {
    final enumDisplayMonitors = _enumDisplayMonitors;
    if (enumDisplayMonitors != null) {
      final data = calloc<_MonitorFindData>();
      try {
        data.ref
          ..targetX = position.dx.toInt()
          ..targetY = position.dy.toInt()
          ..foundMonitor = ffi.nullptr
          ..foundDpi = _defaultDpi;

        final callback = ffi.Pointer.fromFunction<_MonitorEnumProcNative>(_logicalMonitorEnumCallback, 1);
        enumDisplayMonitors(ffi.nullptr, ffi.nullptr, callback, data.address);
        if (data.ref.foundMonitor.address != 0) {
          return data.ref.foundDpi;
        }
      } finally {
        calloc.free(data);
      }
    }

    return _dpiForMonitor(_fallbackMonitor());
  }

  // Called synchronously by EnumDisplayMonitors while resolving the target
  // logical coordinate to a monitor.
  static int _logicalMonitorEnumCallback(ffi.Pointer<ffi.Void> monitor, ffi.Pointer<ffi.Void> hdc, ffi.Pointer<_WindowRect> rect, int lParam) {
    final getMonitorInfo = _getMonitorInfo;
    if (monitor.address == 0 || lParam == 0 || getMonitorInfo == null) {
      return 1;
    }

    final data = ffi.Pointer<_MonitorFindData>.fromAddress(lParam);
    final monitorInfo = calloc<_MonitorInfo>();
    try {
      monitorInfo.ref.cbSize = ffi.sizeOf<_MonitorInfo>();
      if (getMonitorInfo(monitor, monitorInfo) == 0) {
        return 1;
      }

      final dpi = _dpiForMonitor(monitor);
      final scale = dpi / _defaultDpi;
      if (scale <= 0) {
        return 1;
      }

      final monitorRect = monitorInfo.ref.rcMonitor;
      final logicalLeft = (monitorRect.left / scale).toInt();
      final logicalTop = (monitorRect.top / scale).toInt();
      final logicalRight = (monitorRect.right / scale).toInt();
      final logicalBottom = (monitorRect.bottom / scale).toInt();
      final targetX = data.ref.targetX;
      final targetY = data.ref.targetY;

      if (targetX >= logicalLeft && targetX < logicalRight && targetY >= logicalTop && targetY < logicalBottom) {
        data.ref
          ..foundMonitor = monitor
          ..foundDpi = dpi;
        return 0;
      }
    } finally {
      calloc.free(monitorInfo);
    }

    return 1;
  }

  // Converts a native physical rect back into the logical coordinate system
  // expected by launcher layout code.
  static double _dpiScaleForNativeRect(ffi.Pointer<_WindowRect> rect) {
    final monitorFromRect = _monitorFromRect;
    if (monitorFromRect == null) {
      return 1;
    }

    final monitor = monitorFromRect(rect, _monitorDefaultToNearest);
    return _dpiScaleForMonitor(monitor);
  }

  // rcWork is reported in physical pixels. Convert it with the cursor
  // monitor's DPI so the requested logical size maps to the visible target
  // monitor size instead of the primary monitor scale on mixed-DPI desktops.
  static Size? _cursorDisplayLogicalWorkAreaSize() {
    final getCursorPos = _getCursorPos;
    final monitorFromRect = _monitorFromRect;
    final getMonitorInfo = _getMonitorInfo;
    if (getCursorPos == null || monitorFromRect == null || getMonitorInfo == null) {
      return null;
    }

    final cursor = calloc<_WindowPoint>();
    final cursorRect = calloc<_WindowRect>();
    final monitorInfo = calloc<_MonitorInfo>();
    try {
      if (getCursorPos(cursor) == 0) {
        return null;
      }

      cursorRect.ref
        ..left = cursor.ref.x
        ..top = cursor.ref.y
        ..right = cursor.ref.x + 1
        ..bottom = cursor.ref.y + 1;

      final monitor = monitorFromRect(cursorRect, _monitorDefaultToNearest);
      if (monitor.address == 0) {
        return null;
      }

      monitorInfo.ref.cbSize = ffi.sizeOf<_MonitorInfo>();
      if (getMonitorInfo(monitor, monitorInfo) == 0) {
        return null;
      }

      final targetDpiScale = _dpiScaleForMonitor(monitor);
      if (targetDpiScale <= 0) {
        return null;
      }

      // rcWork is the monitor work area after Windows subtracts taskbars and app bars.
      final availableWorkArea = monitorInfo.ref.rcWork;
      final availableWorkAreaWidth = (availableWorkArea.right - availableWorkArea.left) / targetDpiScale;
      final availableWorkAreaHeight = (availableWorkArea.bottom - availableWorkArea.top) / targetDpiScale;
      if (availableWorkAreaWidth <= 0 || availableWorkAreaHeight <= 0) {
        return null;
      }
      return Size(availableWorkAreaWidth, availableWorkAreaHeight);
    } finally {
      calloc.free(monitorInfo);
      calloc.free(cursorRect);
      calloc.free(cursor);
    }
  }

  static double _dpiScaleForMonitor(ffi.Pointer<ffi.Void> monitor) {
    final dpi = _dpiForMonitor(monitor);
    return dpi <= 0 ? 1 : dpi / _defaultDpi;
  }

  // Reads effective monitor DPI with a safe 96 DPI fallback when the OS bridge
  // is unavailable.
  static int _dpiForMonitor(ffi.Pointer<ffi.Void> monitor) {
    final getDpiForMonitor = _getDpiForMonitor;
    if (monitor.address == 0 || getDpiForMonitor == null) {
      return _defaultDpi;
    }

    final dpiX = calloc<ffi.Uint32>();
    final dpiY = calloc<ffi.Uint32>();
    try {
      final result = getDpiForMonitor(monitor, _mdtEffectiveDpi, dpiX, dpiY);
      if (result != 0 || dpiX.value <= 0) {
        return _defaultDpi;
      }
      return dpiX.value;
    } finally {
      calloc.free(dpiY);
      calloc.free(dpiX);
    }
  }

  // Uses the virtual desktop origin as the same practical primary-monitor
  // fallback used by the existing Windows placement path.
  static ffi.Pointer<ffi.Void> _fallbackMonitor() {
    final monitorFromRect = _monitorFromRect;
    if (monitorFromRect == null) {
      return ffi.nullptr;
    }

    final originRect = calloc<_WindowRect>();
    try {
      originRect.ref
        ..left = 0
        ..top = 0
        ..right = 1
        ..bottom = 1;
      return monitorFromRect(originRect, _monitorDefaultToNearest);
    } finally {
      calloc.free(originRect);
    }
  }
}
