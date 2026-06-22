import 'dart:convert';
import 'dart:io';

import 'package:wox/utils/env.dart';

/// Builds an escaped loopback media source URL for WebView preview HTML.
String buildFilePreviewMediaSource(File file) {
  final encodedPath = base64UrlEncode(utf8.encode(file.path));
  return const HtmlEscape(HtmlEscapeMode.attribute).convert("http://127.0.0.1:${Env.serverPort}/preview/file/media?path=$encodedPath");
}
