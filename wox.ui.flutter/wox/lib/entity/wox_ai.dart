import 'package:get/get.dart';
import 'package:wox/entity/wox_image.dart';

import '../enums/wox_ai_conversation_role_enum.dart';

class AIModel {
  late String name;
  late String provider;

  AIModel({required this.name, required this.provider});

  AIModel.fromJson(Map<String, dynamic> json) {
    name = json['Name'];
    provider = json['Provider'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['Provider'] = provider;
    return data;
  }
}

class AIMCPTool {
  late String name;
  late String description;

  AIMCPTool({required this.name, required this.description});

  AIMCPTool.fromJson(Map<String, dynamic> json) {
    name = json['Name'] ?? "";
    description = json['Description'] ?? "";
  }
}

class ChatSelectItem {
  final String id;
  final String name;
  final WoxImage icon;
  final bool isCategory;
  final List<ChatSelectItem> children;
  Function(String traceId)? onExecute;

  ChatSelectItem({
    required this.id,
    required this.name,
    required this.icon,
    required this.isCategory,
    required this.children,
    this.onExecute,
  });
}

// should be same as AIChatData in the ai chat plugin
class WoxAIChatData {
  late String id;
  late String title;
  late RxList<WoxAIChatConversation> conversations;
  late Rx<AIModel> model;
  late int createdAt;
  late int updatedAt;
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
    List<WoxAIChatConversation> conversations = [];
    if (json['Conversations'] != null) {
      for (var e in json['Conversations']) {
        conversations.add(WoxAIChatConversation.fromJson(e));
      }
    }

    return WoxAIChatData(
      id: json['Id'] ?? "",
      title: json['Title'] ?? "",
      conversations: RxList<WoxAIChatConversation>.from(conversations),
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
      conversations: RxList<WoxAIChatConversation>.from([]),
      model: AIModel(name: "", provider: "").obs,
      createdAt: 0,
      updatedAt: 0,
      selectedTools: null,
    );
  }
}

enum ToolCallStatus {
  streaming("streaming"),
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

class ToolCallInfo {
  late String id;
  late String name;
  late Map<String, dynamic> arguments;
  late ToolCallStatus status;

  late String delta; // when toolcall is streaming, we will put the delta content here
  late String response;

  late int startTimestamp;
  late int endTimestamp;

  bool isExpanded = false;

  int get duration => (status == ToolCallStatus.streaming || status == ToolCallStatus.pending || status == ToolCallStatus.running)
      ? DateTime.now().millisecondsSinceEpoch - startTimestamp
      : endTimestamp - startTimestamp;

  ToolCallInfo({
    required this.id,
    required this.name,
    required this.arguments,
    required this.response,
    required this.status,
    required this.delta,
    required this.startTimestamp,
    required this.endTimestamp,
  });

  ToolCallInfo.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? "";
    name = json['Name'] ?? "";
    arguments = json['Arguments'] ?? {};
    delta = json['Delta'] ?? "";
    response = json['Response'] ?? "";
    status = ToolCallStatus.fromString(json['Status']);
    startTimestamp = json['StartTimestamp'] ?? 0;
    endTimestamp = json['EndTimestamp'] ?? 0;
  }

  Map<String, dynamic> toJson() {
    return {
      'Id': id,
      'Name': name,
      'Arguments': arguments,
      'Delta': delta,
      'Response': response,
      'Status': status.value,
      'StartTimestamp': startTimestamp,
      'EndTimestamp': endTimestamp,
    };
  }

  static ToolCallInfo empty() {
    return ToolCallInfo(
      id: "",
      name: "",
      arguments: {},
      response: "",
      delta: "",
      status: ToolCallStatus.pending,
      startTimestamp: 0,
      endTimestamp: 0,
    );
  }
}

class WoxAIChatConversation {
  late String id;
  late WoxAIChatConversationRole role;
  late String text;
  late List<WoxImage> images;
  late int timestamp;
  late ToolCallInfo toolCallInfo;

  WoxAIChatConversation({
    required this.id,
    required this.role,
    required this.text,
    required this.images,
    required this.timestamp,
    required this.toolCallInfo,
  });

  static WoxAIChatConversation fromJson(Map<String, dynamic> json) {
    List<WoxImage> images = [];
    if (json['Images'] != null) {
      for (var e in json['Images']) {
        images.add(WoxImage.fromJson(e));
      }
    }

    ToolCallInfo toolCallInfo = ToolCallInfo.empty();
    if (json['ToolCallInfo'] != null) {
      toolCallInfo = ToolCallInfo.fromJson(json['ToolCallInfo']);
    }

    return WoxAIChatConversation(
      id: json['Id'],
      role: json['Role'],
      text: json['Text'],
      images: images,
      timestamp: json['Timestamp'],
      toolCallInfo: toolCallInfo,
    );
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> json = {
      'Id': id,
      'Role': WoxAIChatConversationRoleEnum.getValue(role),
      'Text': text,
      'Images': images.map((e) => e.toJson()).toList(),
      'Timestamp': timestamp,
      'ToolCallInfo': toolCallInfo.toJson(),
    };

    return json;
  }
}
