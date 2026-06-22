import 'dart:convert';

import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_list_item.dart';

class WoxPreviewListData {
  final List<WoxPreviewListItem> items;

  const WoxPreviewListData({required this.items});

  factory WoxPreviewListData.fromJson(Map<String, dynamic> json) {
    final rawItems = json["items"];

    // The list preview intentionally consumes explicit rows instead of file
    // paths. This keeps progress/status plugins from encoding UI semantics in
    // filenames or markdown while still sharing result-tail rendering.
    return WoxPreviewListData(items: rawItems is List ? rawItems.whereType<Map<String, dynamic>>().map(WoxPreviewListItem.fromJson).toList() : const []);
  }

  factory WoxPreviewListData.fromPreviewData(String previewData) {
    final decoded = jsonDecode(previewData);

    return WoxPreviewListData.fromJson(decoded is Map<String, dynamic> ? decoded : const {});
  }

  Map<String, dynamic> toJson() {
    return {"items": items.map((item) => item.toJson()).toList()};
  }
}

class WoxPreviewListItem {
  final WoxImage? icon;
  final String title;
  final String subtitle;
  final List<WoxListItemTail> tails;

  const WoxPreviewListItem({required this.icon, required this.title, required this.subtitle, required this.tails});

  factory WoxPreviewListItem.fromJson(Map<String, dynamic> json) {
    final rawIcon = json["icon"];
    final rawTails = json["tails"];

    // Preview list tails reuse the normal result-tail entity so status chips
    // and semantic colors remain identical across the result and preview panes.
    return WoxPreviewListItem(
      icon: rawIcon is Map<String, dynamic> ? WoxImage.fromJson(rawIcon) : null,
      title: json["title"]?.toString() ?? "",
      subtitle: json["subtitle"]?.toString() ?? "",
      tails: rawTails is List ? rawTails.whereType<Map<String, dynamic>>().map(WoxListItemTail.fromJson).toList() : const [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      if (icon != null) "icon": icon!.toJson(),
      "title": title,
      if (subtitle.isNotEmpty) "subtitle": subtitle,
      if (tails.isNotEmpty) "tails": tails.map((tail) => tail.toJson()).toList(),
    };
  }
}
