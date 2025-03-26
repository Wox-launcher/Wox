import 'package:uuid/uuid.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
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

// should be same as AIChatData in the ai chat plugin
class WoxPreviewChatData {
  late String id;
  late String title;
  late List<WoxPreviewChatConversation> conversations;
  late WoxPreviewChatModel model;
  late int createdAt;
  late int updatedAt;

  WoxPreviewChatData({required this.id, required this.title, required this.conversations, required this.model, required this.createdAt, required this.updatedAt});

  static WoxPreviewChatData fromJson(Map<String, dynamic> json) {
    List<WoxPreviewChatConversation> conversations = [];
    if (json['Conversations'] != null) {
      for (var e in json['Conversations']) {
        conversations.add(WoxPreviewChatConversation.fromJson(e));
      }
    }

    return WoxPreviewChatData(
      id: json['Id'] ?? const Uuid().v4(),
      title: json['Title'] ?? "",
      conversations: conversations,
      model: json['Model'] != null ? WoxPreviewChatModel.fromJson(json['Model']) : WoxPreviewChatModel(name: "", provider: ""),
      createdAt: json['CreatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
      updatedAt: json['UpdatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'Id': id,
      'Title': title,
      'Conversations': conversations.map((e) => e.toJson()).toList(),
      'Model': model.toJson(),
      'CreatedAt': createdAt,
      'UpdatedAt': updatedAt,
    };
  }
}

class WoxPreviewChatModel {
  late String name;
  late String provider;

  WoxPreviewChatModel({required this.name, required this.provider});

  static WoxPreviewChatModel fromJson(Map<String, dynamic> json) {
    return WoxPreviewChatModel(name: json['Name'], provider: json['Provider']);
  }

  Map<String, dynamic> toJson() {
    return {'Name': name, 'Provider': provider};
  }
}

class WoxPreviewChatConversation {
  late String id;
  late WoxAIChatConversationRole role;
  late String text;
  late List<WoxImage> images;
  late int timestamp;

  WoxPreviewChatConversation({required this.id, required this.role, required this.text, required this.images, required this.timestamp});

  static WoxPreviewChatConversation fromJson(Map<String, dynamic> json) {
    List<WoxImage> images = [];
    if (json['Images'] != null) {
      for (var e in json['Images']) {
        images.add(WoxImage.fromJson(e));
      }
    }

    return WoxPreviewChatConversation(
      id: json['Id'],
      role: json['Role'],
      text: json['Text'],
      images: images,
      timestamp: json['Timestamp'],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'Id': id,
      'Role': WoxAIChatConversationRoleEnum.getValue(role),
      'Text': text,
      'Images': images.map((e) => e.toJson()).toList(),
      'Timestamp': timestamp,
    };
  }
}
