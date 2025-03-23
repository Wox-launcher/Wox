import 'dart:convert';

import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';

class WoxPreview {
  late WoxPreviewType previewType;
  late String previewData;
  late Map<String, String> previewProperties;
  late WoxPreviewScrollPosition scrollPosition;

  WoxPreview({required this.previewType, required this.previewData, required this.previewProperties, required this.scrollPosition});

  @override
  int get hashCode => previewType.hashCode ^ previewData.hashCode ^ previewProperties.hashCode;

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is WoxPreview && other.previewType == previewType && other.previewData == previewData && other.previewProperties == previewProperties;
  }

  WoxPreview.fromJson(Map<String, dynamic> json) {
    previewType = json['PreviewType'];
    previewData = json['PreviewData'];
    previewProperties = Map<String, String>.from(json['PreviewProperties'] ?? {});
    scrollPosition = json['ScrollPosition'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['PreviewType'] = previewType;
    data['PreviewData'] = previewData;
    data['PreviewProperties'] = previewProperties;
    data['ScrollPosition'] = scrollPosition;
    return data;
  }

  static WoxPreview empty() {
    return WoxPreview(previewType: "", previewData: "", previewProperties: {}, scrollPosition: "");
  }
}

class WoxPreviewChatData {
  late List<WoxPreviewChatConversation> messages;

  WoxPreviewChatData({required this.messages});

  static WoxPreviewChatData fromJson(Map<String, dynamic> json) {
    List<WoxPreviewChatConversation> messages = [];
    if (json['Messages'] != null) {
      messages = (json['Messages'] as List).map((e) => WoxPreviewChatConversation.fromJson(e as Map<String, dynamic>)).toList();
    }
    return WoxPreviewChatData(messages: messages);
  }

  Map<String, dynamic> toJson() {
    return {'Messages': messages.map((e) => e.toJson()).toList()};
  }
}

class WoxPreviewChatConversation {
  late String role; // user or ai
  late String text;
  late List<WoxImage> images;
  late int timestamp;

  WoxPreviewChatConversation({required this.role, required this.text, required this.images, required this.timestamp});

  static WoxPreviewChatConversation fromJson(Map<String, dynamic> json) {
    return WoxPreviewChatConversation(
      role: json['Role'],
      text: json['Text'],
      images: json['Images']?.map((e) => WoxImage.fromJson(e)).toList() ?? [],
      timestamp: json['Timestamp'],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'Role': role,
      'Text': text,
      'Images': images.map((e) => e.toJson()).toList(),
      'Timestamp': timestamp,
    };
  }
}
