import 'package:get/get.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/wox_http_util.dart';

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

  // unwrap the remote preview
  Future<WoxPreview> unWrap() async {
    if (previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code) {
      return await WoxHttpUtil.instance.getData<WoxPreview>(previewData);
    }

    return this;
  }
}

// should be same as AIChatData in the ai chat plugin
class WoxAIChatData {
  late String id;
  late String title;
  late RxList<WoxPreviewChatConversation> conversations;
  late Rx<AIModel> model;
  late int createdAt;
  late int updatedAt;

  // Selected tools list, not persisted
  List<String>? selectedTools;

  WoxAIChatData({
    required this.id,
    required this.title,
    required this.conversations,
    required this.model,
    required this.createdAt,
    required this.updatedAt,
    this.selectedTools,
  });

  static WoxAIChatData fromJson(Map<String, dynamic> json) {
    List<WoxPreviewChatConversation> conversations = [];
    if (json['Conversations'] != null) {
      for (var e in json['Conversations']) {
        conversations.add(WoxPreviewChatConversation.fromJson(e));
      }
    }

    return WoxAIChatData(
      id: json['Id'] ?? "",
      title: json['Title'] ?? "",
      conversations: RxList<WoxPreviewChatConversation>.from(conversations),
      model: json['Model'] != null ? AIModel.fromJson(json['Model']).obs : AIModel(name: "", provider: "").obs,
      createdAt: json['CreatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
      updatedAt: json['UpdatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
    );
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> json = {
      'Id': id,
      'Title': title,
      'Conversations': conversations.map((e) => e.toJson()).toList(),
      'Model': model.toJson(),
      'CreatedAt': createdAt,
      'UpdatedAt': updatedAt,
    };

    // Add selected tools to JSON if available
    if (selectedTools != null && selectedTools!.isNotEmpty) {
      json['SelectedTools'] = selectedTools;
    }

    return json;
  }

  static WoxAIChatData empty() {
    return WoxAIChatData(
      id: "",
      title: "",
      conversations: RxList<WoxPreviewChatConversation>.from([]),
      model: AIModel(name: "", provider: "").obs,
      createdAt: 0,
      updatedAt: 0,
      selectedTools: null,
    );
  }
}

// 工具调用状态
enum ToolCallStatus {
  pending("pending"),
  running("running"),
  succeeded("succeeded"),
  failed("failed");

  final String value;
  const ToolCallStatus(this.value);

  static ToolCallStatus fromString(String? value) {
    if (value == null) return ToolCallStatus.pending;
    return ToolCallStatus.values.firstWhere(
      (e) => e.value == value,
      orElse: () => ToolCallStatus.pending,
    );
  }
}

// 工具调用信息
class ToolCallInfo {
  late String name;
  late String arguments;
  late String response;
  late int duration;
  late ToolCallStatus status;
  late int startTime;
  late int endTime;

  // 是否展开显示详情
  bool isExpanded = false;

  ToolCallInfo({
    required this.name,
    required this.arguments,
    required this.response,
    required this.duration,
    required this.status,
    required this.startTime,
    required this.endTime,
  });

  ToolCallInfo.fromJson(Map<String, dynamic> json) {
    name = json['name'] ?? "";
    arguments = json['arguments'] ?? "";
    response = json['response'] ?? "";
    duration = json['duration'] ?? 0;
    status = ToolCallStatus.fromString(json['status']);
    startTime = json['startTime'] ?? 0;
    endTime = json['endTime'] ?? 0;
  }

  Map<String, dynamic> toJson() {
    return {
      'name': name,
      'arguments': arguments,
      'response': response,
      'duration': duration,
      'status': status.value,
      'startTime': startTime,
      'endTime': endTime,
    };
  }
}

class WoxPreviewChatConversation {
  late String id;
  late WoxAIChatConversationRole role;
  late String text;
  late List<WoxImage> images;
  late int timestamp;
  String? toolCallId; // 添加工具调用ID字段
  ToolCallInfo? toolCallInfo; // 添加工具调用信息

  WoxPreviewChatConversation({
    required this.id,
    required this.role,
    required this.text,
    required this.images,
    required this.timestamp,
    this.toolCallId,
    this.toolCallInfo,
  });

  static WoxPreviewChatConversation fromJson(Map<String, dynamic> json) {
    List<WoxImage> images = [];
    if (json['Images'] != null) {
      for (var e in json['Images']) {
        images.add(WoxImage.fromJson(e));
      }
    }

    // 解析工具调用信息
    ToolCallInfo? toolCallInfo;
    if (json['ToolCallInfo'] != null) {
      toolCallInfo = ToolCallInfo.fromJson(json['ToolCallInfo']);
    }

    return WoxPreviewChatConversation(
      id: json['Id'],
      role: json['Role'],
      text: json['Text'],
      images: images,
      timestamp: json['Timestamp'],
      toolCallId: json['ToolCallID'], // 添加工具调用ID
      toolCallInfo: toolCallInfo, // 添加工具调用信息
    );
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> json = {
      'Id': id,
      'Role': WoxAIChatConversationRoleEnum.getValue(role),
      'Text': text,
      'Images': images.map((e) => e.toJson()).toList(),
      'Timestamp': timestamp,
    };

    // 如果有工具调用ID，添加到JSON中
    if (toolCallId != null) {
      json['ToolCallID'] = toolCallId;
    }

    // 如果有工具调用信息，添加到JSON中
    if (toolCallInfo != null) {
      json['ToolCallInfo'] = toolCallInfo!.toJson();
    }

    return json;
  }
}
