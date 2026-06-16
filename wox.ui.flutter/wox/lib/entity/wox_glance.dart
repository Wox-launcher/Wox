import 'package:wox/entity/wox_image.dart';

class GlanceRef {
  late String pluginId;
  late String glanceId;

  GlanceRef({required this.pluginId, required this.glanceId});

  GlanceRef.empty() {
    pluginId = '';
    glanceId = '';
  }

  GlanceRef.fromJson(Map<String, dynamic>? json) {
    pluginId = json?['PluginId'] ?? '';
    glanceId = json?['GlanceId'] ?? '';
  }

  bool get isEmpty => pluginId.isEmpty || glanceId.isEmpty;

  Map<String, dynamic> toJson() => {'PluginId': pluginId, 'GlanceId': glanceId};

  String get key => '$pluginId\x00$glanceId';
}

class MetadataGlance {
  late String id;
  late String name;
  late String description;
  late String icon;
  late int refreshIntervalMs;

  MetadataGlance.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? '';
    name = json['Name'] ?? '';
    description = json['Description'] ?? '';
    icon = json['Icon'] ?? '';
    refreshIntervalMs = json['RefreshIntervalMs'] ?? 0;
  }
}

class GlanceItem {
  late String pluginId;
  late String id;
  late String text;
  late WoxImage icon;
  late String tooltip;
  late GlanceAction? action;

  GlanceItem.fromJson(Map<String, dynamic> json) {
    pluginId = json['PluginId'] ?? '';
    id = json['Id'] ?? '';
    text = json['Text'] ?? '';
    icon = json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : WoxImage.empty();
    tooltip = json['Tooltip'] ?? '';
    action = json['Action'] == null ? null : GlanceAction.fromJson(Map<String, dynamic>.from(json['Action']));
  }

  bool get isEmpty => pluginId.isEmpty || id.isEmpty || text.isEmpty;
}

class GlanceAction {
  late String id;
  late String name;
  late WoxImage icon;
  late bool preventHideAfterAction;
  late Map<String, String> contextData;

  GlanceAction.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? '';
    name = json['Name'] ?? '';
    icon = json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : WoxImage.empty();
    preventHideAfterAction = json['PreventHideAfterAction'] ?? false;
    contextData = Map<String, String>.from(json['ContextData'] ?? {});
  }
}
