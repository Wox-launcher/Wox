import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

class FontFilePreviewRenderer implements WoxFilePreviewRenderer {
  static const fontExtensions = {"ttf", "otf", "woff", "woff2"};

  @override
  bool supports(String fileExtension) {
    return fontExtensions.contains(fileExtension);
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_font_not_found", {"path": context.filePath})));
    }

    return WoxFilePreviewResult(content: _FontFilePreview(file: file, tr: context.tr));
  }
}

class _FontFilePreview extends StatefulWidget {
  final File file;
  final WoxFilePreviewTranslationFormatter tr;

  const _FontFilePreview({required this.file, required this.tr});

  @override
  State<_FontFilePreview> createState() => _FontFilePreviewState();
}

class _FontFilePreviewState extends State<_FontFilePreview> {
  late final Future<_FontPreviewData> _previewFuture;

  @override
  void initState() {
    super.initState();
    _previewFuture = _loadFontPreview(widget.file);
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<_FontPreviewData>(
      future: _previewFuture,
      builder: (context, snapshot) {
        if (snapshot.connectionState == ConnectionState.waiting) {
          return const Center(child: WoxLoadingIndicator(size: 20));
        }

        final data = snapshot.data ?? _FontPreviewData.empty();
        final typeLabel = widget.tr("ui_file_preview_type_font");
        final properties = [
          ...buildWoxFilePreviewCommonProperties(widget.file, typeLabel: typeLabel, tr: widget.tr),
          if (data.familyName.isNotEmpty) WoxFilePreviewProperty(label: widget.tr("ui_file_preview_font_family"), value: data.familyName),
          if (data.subfamilyName.isNotEmpty) WoxFilePreviewProperty(label: widget.tr("ui_file_preview_font_style"), value: data.subfamilyName),
          if (data.fullName.isNotEmpty) WoxFilePreviewProperty(label: widget.tr("ui_file_preview_font_full_name"), value: data.fullName),
          if (data.version.isNotEmpty) WoxFilePreviewProperty(label: widget.tr("ui_file_preview_font_version"), value: data.version),
        ];

        return WoxFileInfoPreview(
          icon: Icons.font_download_rounded,
          fileIconPath: widget.file.path,
          accent: const Color(0xFF8B5CF6),
          title: data.fullName.isNotEmpty ? data.fullName : path.basename(widget.file.path),
          subtitle: data.familyName.isNotEmpty ? data.familyName : typeLabel,
          properties: properties,
          sections: [
            WoxFilePreviewSection(title: widget.tr("ui_file_preview_font_sample"), child: _FontSamplePreview(loadedFamilyName: data.loadedFamilyName)),
            if (data.loadError.isNotEmpty)
              WoxFilePreviewSection(
                title: widget.tr("ui_file_preview_font_metadata_unavailable"),
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Text(data.loadError, style: TextStyle(color: getThemeTextColor(), fontSize: WoxInterfaceSizeUtil.instance.current.resultSubtitleFontSize, height: 1.35)),
                ),
              ),
          ],
        );
      },
    );
  }
}

class _FontSamplePreview extends StatelessWidget {
  final String loadedFamilyName;

  const _FontSamplePreview({required this.loadedFamilyName});

  @override
  Widget build(BuildContext context) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = getThemeTextColor();
    final style = TextStyle(color: textColor, fontFamily: loadedFamilyName.isEmpty ? null : loadedFamilyName, height: 1.18);

    return Padding(
      padding: EdgeInsets.all(metrics.scaledSpacing(14)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text("AaBbCc 1234567890", style: style.copyWith(fontSize: metrics.scaledSpacing(30), fontWeight: FontWeight.w700)),
          SizedBox(height: metrics.scaledSpacing(10)),
          Text("The quick brown fox jumps over the lazy dog.", style: style.copyWith(fontSize: metrics.previewTextFontSize)),
        ],
      ),
    );
  }
}

class _FontPreviewData {
  final String familyName;
  final String subfamilyName;
  final String fullName;
  final String version;
  final String loadedFamilyName;
  final String loadError;

  const _FontPreviewData({
    required this.familyName,
    required this.subfamilyName,
    required this.fullName,
    required this.version,
    required this.loadedFamilyName,
    required this.loadError,
  });

  factory _FontPreviewData.empty() {
    return const _FontPreviewData(familyName: "", subfamilyName: "", fullName: "", version: "", loadedFamilyName: "", loadError: "");
  }

  _FontPreviewData copyWith({String? loadedFamilyName, String? loadError}) {
    return _FontPreviewData(
      familyName: familyName,
      subfamilyName: subfamilyName,
      fullName: fullName,
      version: version,
      loadedFamilyName: loadedFamilyName ?? this.loadedFamilyName,
      loadError: loadError ?? this.loadError,
    );
  }
}

Future<_FontPreviewData> _loadFontPreview(File file) async {
  final bytes = await file.readAsBytes();
  final names = _readFontNames(bytes);
  final familyName = "wox_preview_${file.path.hashCode}_${file.lastModifiedSync().millisecondsSinceEpoch}";

  try {
    final loader = FontLoader(familyName)..addFont(Future.value(ByteData.sublistView(bytes)));
    await loader.load();
    return names.copyWith(loadedFamilyName: familyName);
  } catch (e) {
    return names.copyWith(loadError: e.toString());
  }
}

_FontPreviewData _readFontNames(Uint8List bytes) {
  try {
    final nameTable = _readNameTableBytes(bytes);
    if (nameTable == null) {
      return _FontPreviewData.empty();
    }
    final names = _parseNameTable(nameTable);
    return _FontPreviewData(familyName: names[1] ?? "", subfamilyName: names[2] ?? "", fullName: names[4] ?? "", version: names[5] ?? "", loadedFamilyName: "", loadError: "");
  } catch (_) {
    return _FontPreviewData.empty();
  }
}

Uint8List? _readNameTableBytes(Uint8List bytes) {
  if (bytes.length < 12) {
    return null;
  }

  final data = ByteData.sublistView(bytes);
  final signature = _asciiTag(bytes, 0);
  if (signature == "wOFF") {
    return _readWoffTable(bytes, "name");
  }

  final numTables = data.getUint16(4, Endian.big);
  for (var i = 0; i < numTables; i++) {
    final recordOffset = 12 + i * 16;
    if (recordOffset + 16 > bytes.length) {
      break;
    }
    if (_asciiTag(bytes, recordOffset) != "name") {
      continue;
    }

    final offset = data.getUint32(recordOffset + 8, Endian.big);
    final length = data.getUint32(recordOffset + 12, Endian.big);
    if (offset + length <= bytes.length) {
      return Uint8List.sublistView(bytes, offset, offset + length);
    }
  }
  return null;
}

Uint8List? _readWoffTable(Uint8List bytes, String tableTag) {
  if (bytes.length < 44) {
    return null;
  }

  final data = ByteData.sublistView(bytes);
  final numTables = data.getUint16(12, Endian.big);
  for (var i = 0; i < numTables; i++) {
    final recordOffset = 44 + i * 20;
    if (recordOffset + 20 > bytes.length) {
      break;
    }
    if (_asciiTag(bytes, recordOffset) != tableTag) {
      continue;
    }

    final offset = data.getUint32(recordOffset + 4, Endian.big);
    final compLength = data.getUint32(recordOffset + 8, Endian.big);
    final origLength = data.getUint32(recordOffset + 12, Endian.big);
    if (offset + compLength > bytes.length) {
      return null;
    }

    final tableBytes = Uint8List.sublistView(bytes, offset, offset + compLength);
    if (compLength == origLength) {
      return tableBytes;
    }
    return Uint8List.fromList(ZLibCodec().decode(tableBytes));
  }
  return null;
}

Map<int, String> _parseNameTable(Uint8List tableBytes) {
  final data = ByteData.sublistView(tableBytes);
  if (tableBytes.length < 6) {
    return {};
  }

  final count = data.getUint16(2, Endian.big);
  final stringOffset = data.getUint16(4, Endian.big);
  final names = <int, String>{};

  for (var i = 0; i < count; i++) {
    final recordOffset = 6 + i * 12;
    if (recordOffset + 12 > tableBytes.length) {
      break;
    }

    final platformId = data.getUint16(recordOffset, Endian.big);
    final nameId = data.getUint16(recordOffset + 6, Endian.big);
    if (!{1, 2, 4, 5}.contains(nameId) || names.containsKey(nameId)) {
      continue;
    }

    final length = data.getUint16(recordOffset + 8, Endian.big);
    final offset = data.getUint16(recordOffset + 10, Endian.big);
    final start = stringOffset + offset;
    final end = start + length;
    if (start < 0 || end > tableBytes.length) {
      continue;
    }

    final valueBytes = Uint8List.sublistView(tableBytes, start, end);
    final value = platformId == 0 || platformId == 3 ? _decodeUtf16Be(valueBytes) : latin1.decode(valueBytes, allowInvalid: true).trim();
    if (value.isNotEmpty) {
      names[nameId] = value;
    }
  }
  return names;
}

String _decodeUtf16Be(Uint8List bytes) {
  final data = ByteData.sublistView(bytes);
  final codeUnits = <int>[];
  for (var offset = 0; offset + 1 < bytes.length; offset += 2) {
    codeUnits.add(data.getUint16(offset, Endian.big));
  }
  return String.fromCharCodes(codeUnits).trim();
}

String _asciiTag(Uint8List bytes, int offset) {
  if (offset < 0 || offset + 4 > bytes.length) {
    return "";
  }
  return String.fromCharCodes(bytes.sublist(offset, offset + 4));
}
