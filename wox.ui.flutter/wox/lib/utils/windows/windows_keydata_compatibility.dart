import 'dart:async';
import 'dart:io';
import 'dart:ui';

import 'package:uuid/v4.dart';
import 'package:wox/utils/log.dart';

enum _ClipboardHistoryPasteState { idle, controlDown, vDown, vUp }

/// Applies narrow Windows key data fixes before Flutter's keyboard manager sees them.
/// This addresses this issue:
/// https://github.com/Wox-launcher/Wox/issues/4467
/// https://github.com/flutter/flutter/issues/143997
class WindowsKeyDataCompatibility {
  static const int _maxInstallAttempts = 20;
  static const int _clipboardHistoryPhysicalKey = 0x1600000000;
  static const int _emptyPhysicalKey = 0;
  static const int _emptyLogicalKey = 0;
  static const int _controlLeftPhysicalKey = 0x700e0;
  static const int _controlLeftLogicalKey = 0x200000100;
  static const int _keyVPhysicalKey = 0x70019;
  static const int _keyVLogicalKey = 0x76;

  static bool _installed = false;
  static _ClipboardHistoryPasteState _pasteState = _ClipboardHistoryPasteState.idle;

  /// Installs the Windows 11 clipboard-history paste workaround once.
  static void install({int attempt = 0}) {
    if (!Platform.isWindows || _installed) {
      return;
    }

    final callback = PlatformDispatcher.instance.onKeyData;
    if (callback == null) {
      if (attempt >= _maxInstallAttempts) {
        Logger.instance.warn(const UuidV4().generate(), "WindowsKeyDataCompatibility skipped: onKeyData callback is still null");
        return;
      }

      // Flutter installs onKeyData asynchronously after syncing keyboard state.
      Timer(const Duration(milliseconds: 16), () => install(attempt: attempt + 1));
      return;
    }

    _installed = true;
    Logger.instance.info(const UuidV4().generate(), "WindowsKeyDataCompatibility installed");
    PlatformDispatcher.instance.onKeyData = (data) {
      final fixedData = _fixClipboardHistoryPasteKeyData(data);
      if (fixedData == null) {
        _logKeyData("drop", data);
        return true;
      }

      if (!identical(fixedData, data)) {
        _logKeyData("rewrite", data, fixedData: fixedData);
      } else if (_shouldLogCandidate(data)) {
        _logKeyData("pass", data);
      }

      return callback(fixedData);
    };
  }

  /// Normalizes Windows clipboard-history KeyData into a regular Ctrl+V sequence.
  static KeyData? _fixClipboardHistoryPasteKeyData(KeyData data) {
    // Windows clipboard history emits reordered synthetic keys; keep Ctrl down until the V pair has been delivered.
    if (_pasteState == _ClipboardHistoryPasteState.idle && _isClipboardHistoryControl(data, KeyEventType.down, synthesized: false)) {
      _pasteState = _ClipboardHistoryPasteState.controlDown;
      return _copyAs(data, KeyEventType.down, _controlLeftPhysicalKey, _controlLeftLogicalKey);
    }

    if (_pasteState == _ClipboardHistoryPasteState.controlDown &&
        data.physical == _emptyPhysicalKey &&
        data.logical == _emptyLogicalKey &&
        data.type == KeyEventType.down &&
        !data.synthesized) {
      return null;
    }

    if (_pasteState == _ClipboardHistoryPasteState.controlDown && _isClipboardHistoryControl(data, KeyEventType.up, synthesized: true)) {
      return null;
    }

    if (_pasteState == _ClipboardHistoryPasteState.controlDown && _isClipboardHistoryControl(data, KeyEventType.up, synthesized: false)) {
      _pasteState = _ClipboardHistoryPasteState.vDown;
      return _copyAs(data, KeyEventType.down, _keyVPhysicalKey, _keyVLogicalKey);
    }

    if (_pasteState == _ClipboardHistoryPasteState.controlDown && _isClipboardHistoryV(data, KeyEventType.down, synthesized: false)) {
      _pasteState = _ClipboardHistoryPasteState.vDown;
      return _copyAs(data, KeyEventType.down, _keyVPhysicalKey, _keyVLogicalKey);
    }

    if (_pasteState == _ClipboardHistoryPasteState.vDown && _isClipboardHistoryControl(data, KeyEventType.down, synthesized: true)) {
      _pasteState = _ClipboardHistoryPasteState.vUp;
      return _copyAs(data, KeyEventType.up, _keyVPhysicalKey, _keyVLogicalKey);
    }

    if (_pasteState == _ClipboardHistoryPasteState.vDown && _isClipboardHistoryV(data, KeyEventType.up, synthesized: false)) {
      _pasteState = _ClipboardHistoryPasteState.vUp;
      return _copyAs(data, KeyEventType.up, _keyVPhysicalKey, _keyVLogicalKey);
    }

    if (_pasteState == _ClipboardHistoryPasteState.vUp && _isClipboardHistoryControl(data, KeyEventType.down, synthesized: true)) {
      return null;
    }

    if (_pasteState == _ClipboardHistoryPasteState.vUp && _isClipboardHistoryControl(data, KeyEventType.up, synthesized: true)) {
      _pasteState = _ClipboardHistoryPasteState.idle;
      return _copyAs(data, KeyEventType.up, _controlLeftPhysicalKey, _controlLeftLogicalKey);
    }

    _pasteState = _ClipboardHistoryPasteState.idle;
    return data;
  }

  static bool _isClipboardHistoryControl(KeyData data, KeyEventType type, {required bool synthesized}) {
    return data.physical == _clipboardHistoryPhysicalKey && data.logical == _controlLeftLogicalKey && data.type == type && data.synthesized == synthesized;
  }

  static bool _isClipboardHistoryV(KeyData data, KeyEventType type, {required bool synthesized}) {
    return data.physical == _clipboardHistoryPhysicalKey && data.logical == _keyVLogicalKey && data.type == type && data.synthesized == synthesized;
  }

  static KeyData _copyAs(KeyData data, KeyEventType type, int physical, int logical) {
    return KeyData(timeStamp: data.timeStamp, type: type, physical: physical, logical: logical, character: null, synthesized: false, deviceType: data.deviceType);
  }

  static bool _shouldLogCandidate(KeyData data) {
    return _pasteState != _ClipboardHistoryPasteState.idle ||
        data.physical == _clipboardHistoryPhysicalKey ||
        data.physical == _emptyPhysicalKey ||
        data.logical == _controlLeftLogicalKey ||
        data.logical == _keyVLogicalKey;
  }

  static void _logKeyData(String stage, KeyData data, {KeyData? fixedData}) {
    final before = "type=${data.type.name}, physical=0x${data.physical.toRadixString(16)}, logical=0x${data.logical.toRadixString(16)}, synthesized=${data.synthesized}";
    final after =
        fixedData == null
            ? ""
            : ", fixedType=${fixedData.type.name}, fixedPhysical=0x${fixedData.physical.toRadixString(16)}, fixedLogical=0x${fixedData.logical.toRadixString(16)}, fixedSynthesized=${fixedData.synthesized}";
    Logger.instance.debug(const UuidV4().generate(), "WindowsKeyDataCompatibility $stage: $before$after");
  }
}
