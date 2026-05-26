import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/utils/log.dart';

class WoxSystemWallpaperUtil {
  WoxSystemWallpaperUtil._privateConstructor();

  static final WoxSystemWallpaperUtil _instance = WoxSystemWallpaperUtil._privateConstructor();

  static WoxSystemWallpaperUtil get instance => _instance;

  String _cachedWallpaperPath = '';
  bool _hasCachedWallpaperPath = false;
  Future<String?>? _loadingWallpaperPath;

  // Resolve and cache the active desktop wallpaper path so theme editor previews can reuse it without rerunning platform commands.
  Future<String?> loadSystemWallpaperPath({String? traceId, bool forceRefresh = false}) async {
    if (!forceRefresh && _hasCachedWallpaperPath) {
      return _cachedWallpaperPath.isEmpty ? null : _cachedWallpaperPath;
    }

    final runningLoad = _loadingWallpaperPath;
    if (runningLoad != null) {
      return runningLoad;
    }

    final effectiveTraceId = traceId ?? const UuidV4().generate();
    final future = _resolveSystemWallpaperPath(effectiveTraceId);
    _loadingWallpaperPath = future;

    try {
      final wallpaperPath = await future;
      _cachedWallpaperPath = wallpaperPath ?? '';
      _hasCachedWallpaperPath = true;
      return wallpaperPath;
    } finally {
      if (identical(_loadingWallpaperPath, future)) {
        _loadingWallpaperPath = null;
      }
    }
  }

  // Precache the wallpaper image once settings opens so the theme editor backdrop is ready when the editor is selected.
  Future<void> precacheSystemWallpaperPath(BuildContext context, String wallpaperPath, {String? traceId}) async {
    if (wallpaperPath.isEmpty) {
      return;
    }

    try {
      await precacheImage(FileImage(File(wallpaperPath)), context);
    } catch (e) {
      Logger.instance.error(traceId ?? const UuidV4().generate(), 'Failed to precache system wallpaper: $e');
    }
  }

  // Pick the platform-specific wallpaper resolver and keep failures non-fatal for settings startup.
  Future<String?> _resolveSystemWallpaperPath(String traceId) async {
    try {
      if (Platform.isWindows) {
        return await _getWindowsWallpaperPath();
      }
      if (Platform.isMacOS) {
        return await _getMacOSWallpaperPath();
      }
      if (Platform.isLinux) {
        return await _getLinuxWallpaperPath();
      }
      return null;
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to load system wallpaper: $e');
      return null;
    }
  }

  // Windows can expose the active wallpaper through the registry, a transcoded cache, or a cached theme image.
  Future<String?> _getWindowsWallpaperPath() async {
    final result = await Process.run('reg', ['query', r'HKCU\Control Panel\Desktop', '/v', 'WallPaper']).timeout(const Duration(seconds: 2));
    final match = RegExp(r'WallPaper\s+REG_SZ\s+(.+)', caseSensitive: false).firstMatch(result.stdout.toString());
    final registryPath = match?.group(1)?.trim();
    if (registryPath != null && registryPath.isNotEmpty && await File(registryPath).exists()) {
      return registryPath;
    }

    final appData = Platform.environment['APPDATA'];
    if (appData == null || appData.isEmpty) {
      return null;
    }

    final transcodedWallpaper = '$appData\\Microsoft\\Windows\\Themes\\TranscodedWallpaper';
    if (await File(transcodedWallpaper).exists()) {
      return transcodedWallpaper;
    }

    final cachedDirectory = Directory('$appData\\Microsoft\\Windows\\Themes\\CachedFiles');
    if (!await cachedDirectory.exists()) {
      return null;
    }
    await for (final entity in cachedDirectory.list()) {
      if (entity is File) {
        return entity.path;
      }
    }
    return null;
  }

  // macOS keeps the current desktop picture behind System Events.
  Future<String?> _getMacOSWallpaperPath() async {
    final result = await Process.run('osascript', [
      '-e',
      'tell application "System Events" to get POSIX path of (picture of desktop 1 as alias)',
    ]).timeout(const Duration(seconds: 2));
    final path = result.stdout.toString().trim();
    if (path.isNotEmpty && await File(path).exists()) {
      return path;
    }
    return null;
  }

  // GNOME exposes wallpaper URIs through gsettings; other Linux desktops fall back to no preview backdrop.
  Future<String?> _getLinuxWallpaperPath() async {
    for (final key in ['picture-uri-dark', 'picture-uri']) {
      final result = await Process.run('gsettings', ['get', 'org.gnome.desktop.background', key]).timeout(const Duration(seconds: 2));
      final path = _wallpaperPathFromUri(result.stdout.toString().trim());
      if (path != null && await File(path).exists()) {
        return path;
      }
    }
    return null;
  }

  // gsettings may return a quoted file URI; normalize it to a local path Flutter can load.
  String? _wallpaperPathFromUri(String rawValue) {
    final value = rawValue.replaceAll("'", '').replaceAll('"', '').trim();
    if (value.isEmpty) {
      return null;
    }
    if (!value.startsWith('file://')) {
      return value;
    }
    return Uri.parse(value).toFilePath();
  }
}
