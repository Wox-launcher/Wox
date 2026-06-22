import 'dart:convert';

import 'package:wox/entity/wox_image.dart';

class TriggerKeywordConflictPreviewPlugin {
  late String pluginId;
  late String pluginName;
  late WoxImage icon;
  late List<String> triggerKeywords;

  TriggerKeywordConflictPreviewPlugin.fromJson(Map<String, dynamic> json) {
    pluginId = json['PluginId']?.toString() ?? "";
    pluginName = json['PluginName']?.toString() ?? "";
    // Older core builds did not send preview icons. Keep a safe empty image so
    // the redesigned preview can fall back without breaking stale preview data.
    icon = json['Icon'] is Map<String, dynamic> ? WoxImage.fromJson(json['Icon']) : WoxImage.empty();
    triggerKeywords = (json['TriggerKeywords'] as List<dynamic>? ?? []).map((item) => item.toString()).toList();
  }
}

class TriggerKeywordConflictPreviewData {
  late String keyword;
  late String title;
  late String message;
  late List<TriggerKeywordConflictPreviewPlugin> plugins;

  TriggerKeywordConflictPreviewData.fromJson(Map<String, dynamic> json) {
    keyword = json['Keyword']?.toString() ?? "";
    title = json['Title']?.toString() ?? "";
    message = json['Message']?.toString() ?? "";
    plugins = (json['Plugins'] as List<dynamic>? ?? []).map((item) => TriggerKeywordConflictPreviewPlugin.fromJson(Map<String, dynamic>.from(item))).toList();
  }

  factory TriggerKeywordConflictPreviewData.fromPreviewData(String previewData) {
    return TriggerKeywordConflictPreviewData.fromJson(Map<String, dynamic>.from(jsonDecode(previewData)));
  }
}
