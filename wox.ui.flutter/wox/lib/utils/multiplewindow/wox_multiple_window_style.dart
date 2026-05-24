// ignore_for_file: invalid_use_of_internal_member, implementation_imports

import 'dart:ffi' as ffi;
import 'dart:io';

import 'package:ffi/ffi.dart';
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
const int _offscreenCoordinate = -32000;

const int _dwmwaUseImmersiveDarkMode = 20;
const int _dwmwaWindowCornerPreference = 33;
const int _dwmwaSystemBackdropType = 38;
const int _dwmcpRound = 2;
const int _dwmsbtNone = 1;
const int _dwmsbtTabbedWindow = 4;

const int _wcaAccentPolicy = 19;
const int _accentEnableAcrylicBlurBehind = 4;
const int _accentEnableHostBackdrop = 5;
const int _woxAcrylicGradientColor = 0x2A202020;
const int _woxHostBackdropGradientColor = 0x70202020;

final class _AccentPolicy extends ffi.Struct {
  @ffi.Int32()
  external int accentState;

  @ffi.Uint32()
  external int accentFlags;

  @ffi.Uint32()
  external int gradientColor;

  @ffi.Uint32()
  external int animationId;
}

final class _WindowCompositionAttribData extends ffi.Struct {
  @ffi.Uint32()
  external int attrib;

  external ffi.Pointer<ffi.Void> data;

  @ffi.UintPtr()
  external int sizeOfData;
}

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
typedef _GetMonitorInfoNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_MonitorInfo>);
typedef _GetMonitorInfoDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_MonitorInfo>);
typedef _GetWindowRectNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowRect>);
typedef _GetWindowRectDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowRect>);
typedef _ReleaseCaptureNative = ffi.Int32 Function();
typedef _ReleaseCaptureDart = int Function();
typedef _SendMessageNative = ffi.IntPtr Function(ffi.Pointer<ffi.Void>, ffi.Uint32, ffi.UintPtr, ffi.IntPtr);
typedef _SendMessageDart = int Function(ffi.Pointer<ffi.Void>, int, int, int);
typedef _SetWindowCompositionAttributeNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowCompositionAttribData>);
typedef _SetWindowCompositionAttributeDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_WindowCompositionAttribData>);
typedef _DwmSetWindowAttributeNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Uint32, ffi.Pointer<ffi.Void>, ffi.Uint32);
typedef _DwmSetWindowAttributeDart = int Function(ffi.Pointer<ffi.Void>, int, ffi.Pointer<ffi.Void>, int);
typedef _DwmExtendFrameIntoClientAreaNative = ffi.Int32 Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_Margins>);
typedef _DwmExtendFrameIntoClientAreaDart = int Function(ffi.Pointer<ffi.Void>, ffi.Pointer<_Margins>);

class WoxMultipleWindowStyle {
  static final ffi.DynamicLibrary? _user32 = Platform.isWindows ? ffi.DynamicLibrary.open("user32.dll") : null;
  static final ffi.DynamicLibrary? _dwmapi = Platform.isWindows ? ffi.DynamicLibrary.open("dwmapi.dll") : null;

  static final _GetWindowLongPtrDart? _getWindowLongPtr = _user32?.lookupFunction<_GetWindowLongPtrNative, _GetWindowLongPtrDart>("GetWindowLongPtrW");
  static final _SetWindowLongPtrDart? _setWindowLongPtr = _user32?.lookupFunction<_SetWindowLongPtrNative, _SetWindowLongPtrDart>("SetWindowLongPtrW");
  static final _SetWindowPosDart? _setWindowPos = _user32?.lookupFunction<_SetWindowPosNative, _SetWindowPosDart>("SetWindowPos");
  static final _GetCursorPosDart? _getCursorPos = _user32?.lookupFunction<_GetCursorPosNative, _GetCursorPosDart>("GetCursorPos");
  static final _MonitorFromRectDart? _monitorFromRect = _user32?.lookupFunction<_MonitorFromRectNative, _MonitorFromRectDart>("MonitorFromRect");
  static final _GetMonitorInfoDart? _getMonitorInfo = _user32?.lookupFunction<_GetMonitorInfoNative, _GetMonitorInfoDart>("GetMonitorInfoW");
  static final _GetWindowRectDart? _getWindowRect = _user32?.lookupFunction<_GetWindowRectNative, _GetWindowRectDart>("GetWindowRect");
  static final _ReleaseCaptureDart? _releaseCapture = _user32?.lookupFunction<_ReleaseCaptureNative, _ReleaseCaptureDart>("ReleaseCapture");
  static final _SendMessageDart? _sendMessage = _user32?.lookupFunction<_SendMessageNative, _SendMessageDart>("SendMessageW");
  static final _SetWindowCompositionAttributeDart? _setWindowCompositionAttribute = _user32
      ?.lookupFunction<_SetWindowCompositionAttributeNative, _SetWindowCompositionAttributeDart>("SetWindowCompositionAttribute");
  static final _DwmSetWindowAttributeDart? _dwmSetWindowAttribute = _dwmapi?.lookupFunction<_DwmSetWindowAttributeNative, _DwmSetWindowAttributeDart>("DwmSetWindowAttribute");
  static final _DwmExtendFrameIntoClientAreaDart? _dwmExtendFrameIntoClientArea = _dwmapi?.lookupFunction<_DwmExtendFrameIntoClientAreaNative, _DwmExtendFrameIntoClientAreaDart>(
    "DwmExtendFrameIntoClientArea",
  );

  /// Applies native Windows chrome policy for a Flutter windowing controller.
  static void apply(Object controller, {required bool mica}) {
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
    _enableDarkMode(hwnd);
    _enableRoundedCorners(hwnd);
    if (mica) {
      _enableBackdrop(hwnd);
    }
    _refreshWindowFrame(hwnd);
  }

  /// Centers a newly created window on the display currently containing the mouse cursor.
  static void centerOnCursorDisplay(Object controller) {
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
      final windowWidth = windowRect.ref.right - windowRect.ref.left;
      final windowHeight = windowRect.ref.bottom - windowRect.ref.top;
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

  /// Moves a new window outside visible work areas while Flutter paints its first frames.
  static void moveOffscreen(Object controller) {
    if (!Platform.isWindows) {
      return;
    }

    final hwnd = _windowHandleOf(controller);
    if (hwnd == null || hwnd.address == 0) {
      return;
    }

    _setWindowPos?.call(hwnd, ffi.nullptr, _offscreenCoordinate, _offscreenCoordinate, 0, 0, _swpNoSize | _swpNoZOrder | _swpNoActivate | _swpFrameChanged);
  }

  /// Starts native dragging for a custom Flutter-drawn title area.
  static void startDragging(Object controller) {
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

  static ffi.Pointer<ffi.Void>? _windowHandleOf(Object controller) {
    if (controller is win32_windowing.WindowControllerWin32) {
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

  static void _enableDarkMode(ffi.Pointer<ffi.Void> hwnd) {
    final value = calloc<ffi.Int32>();
    try {
      value.value = 1;
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

  static void _enableBackdrop(ffi.Pointer<ffi.Void> hwnd) {
    if (_tryEnableAccent(hwnd, _accentEnableAcrylicBlurBehind, _woxAcrylicGradientColor, 2) ||
        _tryEnableAccent(hwnd, _accentEnableHostBackdrop, _woxHostBackdropGradientColor, 0)) {
      _extendFrame(hwnd, 0);
      _setDwmBackdrop(hwnd, _dwmsbtNone);
      return;
    }

    _extendFrame(hwnd, -1);
    _setDwmBackdrop(hwnd, _dwmsbtTabbedWindow);
  }

  static bool _tryEnableAccent(ffi.Pointer<ffi.Void> hwnd, int accentState, int gradientColor, int accentFlags) {
    final setWindowCompositionAttribute = _setWindowCompositionAttribute;
    if (setWindowCompositionAttribute == null) {
      return false;
    }

    final policy = calloc<_AccentPolicy>();
    final data = calloc<_WindowCompositionAttribData>();
    try {
      policy.ref
        ..accentState = accentState
        ..accentFlags = accentFlags
        ..gradientColor = gradientColor
        ..animationId = 0;

      data.ref
        ..attrib = _wcaAccentPolicy
        ..data = policy.cast<ffi.Void>()
        ..sizeOfData = ffi.sizeOf<_AccentPolicy>();

      return setWindowCompositionAttribute(hwnd, data) != 0;
    } finally {
      calloc.free(data);
      calloc.free(policy);
    }
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
}
