import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:path/path.dart' as path;
import 'package:wox/components/file_preview/file_info_preview.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/utils/colors.dart';

class ShortcutFilePreviewRenderer implements WoxFilePreviewRenderer {
  @override
  bool supports(String fileExtension) {
    return fileExtension == "lnk";
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_shortcut_not_found", {"path": context.filePath})));
    }

    final shortcut = _WindowsShortcutInfo.tryRead(file);
    final typeLabel = context.tr("ui_file_preview_type_windows_shortcut");
    final targetName = _displayShortcutTarget(shortcut);
    final properties = [
      ...buildWoxFilePreviewCommonProperties(file, typeLabel: typeLabel, tr: context.tr),
      if (targetName.isNotEmpty) WoxFilePreviewProperty(label: context.tr("ui_file_preview_shortcut_target"), value: targetName),
    ];

    return WoxFilePreviewResult(
      content: WoxFileInfoPreview(
        icon: Icons.shortcut_rounded,
        fileIconPath: _shortcutIconSource(file, shortcut),
        accent: const Color(0xFFA78BFA),
        title: path.basenameWithoutExtension(file.path),
        subtitle: targetName.isNotEmpty ? context.tr("ui_file_preview_shortcut_opens", {"name": targetName}) : context.tr("ui_file_preview_shortcut_target_unavailable"),
        properties: properties,
        sections: [
          if (shortcut?.description.isNotEmpty == true)
            WoxFilePreviewSection(
              title: context.tr("ui_file_preview_shortcut_description"),
              child: Padding(padding: const EdgeInsets.all(12), child: Text(shortcut!.description, style: TextStyle(color: getThemeTextColor(), fontSize: 12, height: 1.35))),
            ),
        ],
      ),
    );
  }
}

// Prefer the target's icon because .lnk files often expose only a generic shortcut glyph.
String _shortcutIconSource(File file, _WindowsShortcutInfo? shortcut) {
  final targetPath = shortcut?.targetPath.trim() ?? "";
  if (targetPath.isEmpty) {
    return file.path;
  }

  try {
    if (FileSystemEntity.typeSync(targetPath) != FileSystemEntityType.notFound) {
      return targetPath;
    }
  } catch (_) {
    return file.path;
  }

  return file.path;
}

// Shows a friendly app/file name instead of the raw shortcut target path.
String _displayShortcutTarget(_WindowsShortcutInfo? shortcut) {
  final targetPath = shortcut?.targetPath.trim() ?? "";
  if (targetPath.isEmpty) {
    return "";
  }

  final targetFileName = path.basename(targetPath);
  if (path.extension(targetFileName).toLowerCase() == ".exe") {
    return path.basenameWithoutExtension(targetFileName);
  }
  return targetFileName;
}

class _WindowsShortcutInfo {
  final String targetPath;
  final String relativePath;
  final String workingDirectory;
  final String arguments;
  final String iconLocation;
  final String description;

  const _WindowsShortcutInfo({
    required this.targetPath,
    required this.relativePath,
    required this.workingDirectory,
    required this.arguments,
    required this.iconLocation,
    required this.description,
  });

  // Parses the Shell Link header, LinkInfo, and string-data fields without
  // resolving the shortcut through COM, so preview remains read-only.
  static _WindowsShortcutInfo? tryRead(File file) {
    try {
      final bytes = file.readAsBytesSync();
      final reader = _ShortcutReader(bytes);
      if (reader.length < 76 || reader.u32(0) != 0x4C || reader.u32(4) != 0x00021401) {
        return null;
      }

      final linkFlags = reader.u32(20);
      var offset = 76;
      if ((linkFlags & _ShortcutFlags.hasLinkTargetIdList) != 0) {
        final idListSize = reader.u16(offset);
        offset += 2 + idListSize;
      }

      var targetPath = "";
      if ((linkFlags & _ShortcutFlags.hasLinkInfo) != 0 && offset + 4 <= reader.length) {
        final linkInfoSize = reader.u32(offset);
        if (linkInfoSize > 0 && offset + linkInfoSize <= reader.length) {
          targetPath = reader.readLinkInfoTarget(offset, linkInfoSize);
          offset += linkInfoSize;
        }
      }

      final unicode = (linkFlags & _ShortcutFlags.isUnicode) != 0;
      var description = "";
      var relativePath = "";
      var workingDirectory = "";
      var arguments = "";
      var iconLocation = "";

      if ((linkFlags & _ShortcutFlags.hasName) != 0) {
        final result = reader.readStringData(offset, unicode: unicode);
        description = result.value;
        offset = result.nextOffset;
      }
      if ((linkFlags & _ShortcutFlags.hasRelativePath) != 0) {
        final result = reader.readStringData(offset, unicode: unicode);
        relativePath = result.value;
        offset = result.nextOffset;
      }
      if ((linkFlags & _ShortcutFlags.hasWorkingDir) != 0) {
        final result = reader.readStringData(offset, unicode: unicode);
        workingDirectory = result.value;
        offset = result.nextOffset;
      }
      if ((linkFlags & _ShortcutFlags.hasArguments) != 0) {
        final result = reader.readStringData(offset, unicode: unicode);
        arguments = result.value;
        offset = result.nextOffset;
      }
      if ((linkFlags & _ShortcutFlags.hasIconLocation) != 0) {
        final result = reader.readStringData(offset, unicode: unicode);
        iconLocation = result.value;
      }

      return _WindowsShortcutInfo(
        targetPath: targetPath.isNotEmpty ? targetPath : relativePath,
        relativePath: relativePath,
        workingDirectory: workingDirectory,
        arguments: arguments,
        iconLocation: iconLocation,
        description: description,
      );
    } catch (_) {
      return null;
    }
  }
}

class _ShortcutFlags {
  static const hasLinkTargetIdList = 0x00000001;
  static const hasLinkInfo = 0x00000002;
  static const hasName = 0x00000004;
  static const hasRelativePath = 0x00000008;
  static const hasWorkingDir = 0x00000010;
  static const hasArguments = 0x00000020;
  static const hasIconLocation = 0x00000040;
  static const isUnicode = 0x00000080;
}

class _ShortcutReader {
  final Uint8List bytes;
  late final ByteData data = ByteData.sublistView(bytes);

  _ShortcutReader(this.bytes);

  int get length => bytes.length;

  int u16(int offset) {
    if (offset < 0 || offset + 2 > length) {
      return 0;
    }
    return data.getUint16(offset, Endian.little);
  }

  int u32(int offset) {
    if (offset < 0 || offset + 4 > length) {
      return 0;
    }
    return data.getUint32(offset, Endian.little);
  }

  String readLinkInfoTarget(int start, int size) {
    final end = start + size;
    final headerSize = u32(start + 4);
    final localBasePathOffset = u32(start + 16);
    final commonPathSuffixOffset = u32(start + 24);
    var localBasePath = "";
    var commonPathSuffix = "";

    if (headerSize >= 0x24) {
      final localBasePathOffsetUnicode = u32(start + 28);
      final commonPathSuffixOffsetUnicode = u32(start + 32);
      if (localBasePathOffsetUnicode > 0) {
        localBasePath = readUtf16NullTerminated(start + localBasePathOffsetUnicode, end);
      }
      if (commonPathSuffixOffsetUnicode > 0) {
        commonPathSuffix = readUtf16NullTerminated(start + commonPathSuffixOffsetUnicode, end);
      }
    }

    if (localBasePath.isEmpty && localBasePathOffset > 0) {
      localBasePath = readAnsiNullTerminated(start + localBasePathOffset, end);
    }
    if (commonPathSuffix.isEmpty && commonPathSuffixOffset > 0) {
      commonPathSuffix = readAnsiNullTerminated(start + commonPathSuffixOffset, end);
    }

    return joinShortcutPath(localBasePath, commonPathSuffix);
  }

  _StringDataResult readStringData(int offset, {required bool unicode}) {
    final charCount = u16(offset);
    var cursor = offset + 2;
    if (charCount == 0 || cursor >= length) {
      return _StringDataResult(value: "", nextOffset: cursor);
    }

    if (unicode) {
      final bytesLength = charCount * 2;
      final value = readUtf16(cursor, cursor + bytesLength);
      return _StringDataResult(value: value, nextOffset: cursor + bytesLength);
    }

    final end = (cursor + charCount).clamp(cursor, length);
    final value = latin1.decode(bytes.sublist(cursor, end), allowInvalid: true).trim();
    return _StringDataResult(value: value, nextOffset: end);
  }

  String readAnsiNullTerminated(int offset, int end) {
    if (offset <= 0 || offset >= length) {
      return "";
    }

    final safeEnd = end.clamp(offset, length);
    var cursor = offset;
    while (cursor < safeEnd && bytes[cursor] != 0) {
      cursor++;
    }
    return latin1.decode(bytes.sublist(offset, cursor), allowInvalid: true).trim();
  }

  String readUtf16NullTerminated(int offset, int end) {
    if (offset <= 0 || offset >= length) {
      return "";
    }

    final safeEnd = end.clamp(offset, length);
    var cursor = offset;
    while (cursor + 1 < safeEnd && u16(cursor) != 0) {
      cursor += 2;
    }
    return readUtf16(offset, cursor);
  }

  String readUtf16(int start, int end) {
    if (start <= 0 || start >= length) {
      return "";
    }

    final safeEnd = end.clamp(start, length);
    final codeUnits = <int>[];
    for (var offset = start; offset + 1 < safeEnd; offset += 2) {
      final value = u16(offset);
      if (value == 0) {
        break;
      }
      codeUnits.add(value);
    }
    return String.fromCharCodes(codeUnits).trim();
  }

  String joinShortcutPath(String localBasePath, String commonPathSuffix) {
    if (localBasePath.isEmpty) {
      return commonPathSuffix;
    }
    if (commonPathSuffix.isEmpty || localBasePath.toLowerCase().endsWith(commonPathSuffix.toLowerCase())) {
      return localBasePath;
    }
    if (localBasePath.endsWith("\\") || localBasePath.endsWith("/") || commonPathSuffix.startsWith("\\") || commonPathSuffix.startsWith("/")) {
      return "$localBasePath$commonPathSuffix";
    }
    return "$localBasePath\\$commonPathSuffix";
  }
}

class _StringDataResult {
  final String value;
  final int nextOffset;

  const _StringDataResult({required this.value, required this.nextOffset});
}
