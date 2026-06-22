import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/utils/colors.dart';

class RdpFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "rdp";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_rdp_not_found", {"path": context.filePath})));
    }

    try {
      final profile = _RdpConnectionProfile.parse(file);
      final serverAddress = profile.serverAddress;
      final displayMode = profile.displayMode(context.tr);
      final authenticationLevel = profile.authenticationLevel(context.tr);
      final credentialSecurity = profile.credentialSecurity(context.tr);
      final clipboardRedirection = profile.clipboardRedirection(context.tr);
      final driveRedirection = profile.driveRedirection(context.tr);
      final audioMode = profile.audioMode(context.tr);
      final remoteApplicationSummary = profile.remoteApplicationSummary;
      final properties = [
        ...buildWoxFilePreviewCommonProperties(file, typeLabel: context.tr("ui_file_preview_type_rdp_connection"), tr: context.tr),
        if (serverAddress.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_server"), value: serverAddress),
        if (profile.gatewayHost.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_gateway"), value: profile.gatewayHost),
        if (profile.username.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_username"), value: profile.username),
        if (profile.domain.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_domain"), value: profile.domain),
        if (profile.desktopSize.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_resolution"), value: profile.desktopSize),
        if (displayMode.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_display_mode"), value: displayMode),
        if (profile.colorDepth.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_color_depth"), value: profile.colorDepth),
        if (authenticationLevel.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_authentication"), value: authenticationLevel),
        if (credentialSecurity.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_credential_security"), value: credentialSecurity),
        if (clipboardRedirection.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_clipboard"), value: clipboardRedirection),
        if (driveRedirection.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_drives"), value: driveRedirection),
        if (audioMode.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_rdp_audio"), value: audioMode),
      ];

      return WoxFilePreviewResult(
        content: WoxFileInfoPreview(
          icon: Icons.desktop_windows_rounded,
          fileIconPath: file.path,
          accent: const Color(0xFF38BDF8),
          title: path.basenameWithoutExtension(file.path),
          subtitle: serverAddress.isNotEmpty ? context.tr("ui_file_preview_rdp_connects_to", {"server": serverAddress}) : context.tr("ui_file_preview_rdp_server_unavailable"),
          properties: properties,
          sections: [
            if (remoteApplicationSummary.isNotEmpty)
              WoxFilePreviewSection(
                title: context.tr("ui_file_preview_rdp_remote_app"),
                child: Padding(padding: const EdgeInsets.all(12), child: Text(remoteApplicationSummary, style: TextStyle(color: getThemeTextColor(), fontSize: 12, height: 1.35))),
              ),
          ],
        ),
      );
    } catch (e) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_error", {"error": e.toString()})));
    }
  }
}

class _RdpConnectionProfile {
  final Map<String, String> values;

  const _RdpConnectionProfile(this.values);

  // Parses the plain-text RDP key:type:value format without opening or resolving the connection.
  static _RdpConnectionProfile parse(File file) {
    final settings = <String, String>{};
    for (final line in const LineSplitter().convert(_readRdpProfileText(file))) {
      final entry = _parseRdpLine(line);
      if (entry != null) {
        settings[entry.key] = entry.value;
      }
    }
    return _RdpConnectionProfile(settings);
  }

  String get serverAddress {
    final fullAddress = _value("full address");
    return fullAddress.isNotEmpty ? fullAddress : _value("alternate full address");
  }

  String get gatewayHost => _value("gatewayhostname");

  String get username => _value("username");

  String get domain => _value("domain");

  String get desktopSize {
    final width = _value("desktopwidth");
    final height = _value("desktopheight");
    if (width.isEmpty || height.isEmpty) {
      return "";
    }
    return "${width}x$height";
  }

  String get colorDepth {
    final bpp = _value("session bpp");
    return bpp.isEmpty ? "" : "$bpp-bit";
  }

  String get remoteApplicationSummary {
    final values = [_value("remoteapplicationname"), _value("remoteapplicationprogram"), _value("remoteapplicationcmdline")].where((value) => value.isNotEmpty).toList();
    return values.join("\n");
  }

  String displayMode(WoxFilePreviewTranslationFormatter tr) {
    switch (_value("screen mode id")) {
      case "1":
        return tr("ui_file_preview_rdp_display_windowed");
      case "2":
        return tr("ui_file_preview_rdp_display_fullscreen");
      default:
        return "";
    }
  }

  String authenticationLevel(WoxFilePreviewTranslationFormatter tr) {
    switch (_value("authentication level")) {
      case "0":
        return tr("ui_file_preview_rdp_authentication_always_connect");
      case "1":
        return tr("ui_file_preview_rdp_authentication_warn");
      case "2":
        return tr("ui_file_preview_rdp_authentication_required");
      default:
        return "";
    }
  }

  String credentialSecurity(WoxFilePreviewTranslationFormatter tr) {
    return _enabledLabel(_value("enablecredsspsupport"), tr);
  }

  String clipboardRedirection(WoxFilePreviewTranslationFormatter tr) {
    return _enabledLabel(_value("redirectclipboard"), tr);
  }

  String driveRedirection(WoxFilePreviewTranslationFormatter tr) {
    final value = _value("drivestoredirect");
    if (value.isEmpty) {
      return "";
    }
    if (value == "*") {
      return tr("ui_file_preview_rdp_drives_all");
    }
    return value;
  }

  String audioMode(WoxFilePreviewTranslationFormatter tr) {
    switch (_value("audiomode")) {
      case "0":
        return tr("ui_file_preview_rdp_audio_local");
      case "1":
        return tr("ui_file_preview_rdp_audio_remote");
      case "2":
        return tr("ui_file_preview_rdp_audio_disabled");
      default:
        return "";
    }
  }

  String _value(String key) {
    return values[key]?.trim() ?? "";
  }
}

class _RdpLineEntry {
  final String key;
  final String value;

  const _RdpLineEntry({required this.key, required this.value});
}

String _enabledLabel(String value, WoxFilePreviewTranslationFormatter tr) {
  switch (value) {
    case "0":
      return tr("ui_file_preview_rdp_disabled");
    case "1":
      return tr("ui_file_preview_rdp_enabled");
    default:
      return "";
  }
}

_RdpLineEntry? _parseRdpLine(String line) {
  final trimmed = line.trim();
  if (trimmed.isEmpty) {
    return null;
  }

  final keyEnd = trimmed.indexOf(":");
  if (keyEnd <= 0 || keyEnd + 2 >= trimmed.length) {
    return null;
  }

  final typeEnd = trimmed.indexOf(":", keyEnd + 1);
  if (typeEnd <= keyEnd + 1 || typeEnd + 1 > trimmed.length) {
    return null;
  }

  return _RdpLineEntry(key: trimmed.substring(0, keyEnd).trim().toLowerCase(), value: trimmed.substring(typeEnd + 1).trim());
}

// RDP files commonly come from mstsc as UTF-16LE, but exported files can also be UTF-8 text.
String _readRdpProfileText(File file) {
  final bytes = file.readAsBytesSync();
  if (bytes.isEmpty) {
    return "";
  }
  if (_hasUtf16LittleEndianBom(bytes)) {
    return _decodeUtf16(bytes.sublist(2), Endian.little);
  }
  if (_hasUtf16BigEndianBom(bytes)) {
    return _decodeUtf16(bytes.sublist(2), Endian.big);
  }
  if (_looksLikeUtf16LittleEndian(bytes)) {
    return _decodeUtf16(bytes, Endian.little);
  }
  if (_looksLikeUtf16BigEndian(bytes)) {
    return _decodeUtf16(bytes, Endian.big);
  }

  try {
    return utf8.decode(bytes);
  } catch (_) {
    return latin1.decode(bytes, allowInvalid: true);
  }
}

bool _hasUtf16LittleEndianBom(Uint8List bytes) {
  return bytes.length >= 2 && bytes[0] == 0xFF && bytes[1] == 0xFE;
}

bool _hasUtf16BigEndianBom(Uint8List bytes) {
  return bytes.length >= 2 && bytes[0] == 0xFE && bytes[1] == 0xFF;
}

bool _looksLikeUtf16LittleEndian(Uint8List bytes) {
  final sampleLength = bytes.length.clamp(0, 256).toInt();
  var zeroOddBytes = 0;
  var pairs = 0;
  for (var i = 0; i + 1 < sampleLength; i += 2) {
    if (bytes[i + 1] == 0 && bytes[i] != 0) {
      zeroOddBytes++;
    }
    pairs++;
  }
  return pairs > 0 && zeroOddBytes / pairs > 0.45;
}

bool _looksLikeUtf16BigEndian(Uint8List bytes) {
  final sampleLength = bytes.length.clamp(0, 256).toInt();
  var zeroEvenBytes = 0;
  var pairs = 0;
  for (var i = 0; i + 1 < sampleLength; i += 2) {
    if (bytes[i] == 0 && bytes[i + 1] != 0) {
      zeroEvenBytes++;
    }
    pairs++;
  }
  return pairs > 0 && zeroEvenBytes / pairs > 0.45;
}

String _decodeUtf16(Uint8List bytes, Endian endian) {
  final data = ByteData.sublistView(bytes);
  final codeUnits = <int>[];
  for (var offset = 0; offset + 1 < bytes.length; offset += 2) {
    final codeUnit = data.getUint16(offset, endian);
    if (codeUnit != 0xFEFF) {
      codeUnits.add(codeUnit);
    }
  }
  return String.fromCharCodes(codeUnits);
}
