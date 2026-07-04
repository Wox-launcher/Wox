import 'package:get/get.dart';
import 'package:wox/entity/wox_image.dart';

import '../enums/wox_ai_conversation_role_enum.dart';

class AIModel {
  late String name;
  late String provider;
  late String providerAlias;

  AIModel({required this.name, required this.provider, required this.providerAlias});

  AIModel.fromJson(Map<String, dynamic> json) {
    name = json['Name'];
    provider = json['Provider'];
    providerAlias = json['ProviderAlias'] ?? "";
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['Provider'] = provider;
    data['ProviderAlias'] = providerAlias;
    return data;
  }

  static AIModel empty() {
    return AIModel(name: "", provider: "", providerAlias: "");
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

class AIQuestionOption {
  late String value;
  late String title;
  late String subTitle;
  late bool recommended;
  late Map<String, String> extra;

  AIQuestionOption({required this.value, required this.title, required this.subTitle, required this.recommended, required this.extra});

  AIQuestionOption.fromJson(Map<String, dynamic> json) {
    value = json['Value'] ?? json['value'] ?? "";
    title = json['Title'] ?? json['title'] ?? value;
    subTitle = json['SubTitle'] ?? json['subTitle'] ?? json['Subtitle'] ?? json['subtitle'] ?? "";
    recommended = json['Recommended'] ?? json['recommended'] ?? false;
    final rawExtra = json['Extra'] ?? json['extra'];
    extra = rawExtra is Map ? rawExtra.map((key, value) => MapEntry(key.toString(), value.toString())) : <String, String>{};
    if (value.isEmpty) {
      value = title;
    }
    if (title.isEmpty) {
      title = value;
    }
  }
}

class AIQuestion {
  late String questionId;
  late String question;
  late List<AIQuestionOption> options;

  AIQuestion({required this.questionId, required this.question, required this.options});

  AIQuestion.fromJson(Map<String, dynamic> json) {
    questionId = json['QuestionId'] ?? json['questionId'] ?? "";
    question = json['Question'] ?? json['question'] ?? "";
    final rawOptions = json['Options'] ?? json['options'];
    options =
        rawOptions is List
            ? rawOptions
                .map((option) {
                  if (option is String) {
                    return AIQuestionOption(value: option, title: option, subTitle: "", recommended: false, extra: <String, String>{});
                  }
                  if (option is Map) {
                    return AIQuestionOption.fromJson(Map<String, dynamic>.from(option));
                  }
                  return null;
                })
                .whereType<AIQuestionOption>()
                .where((option) => option.title.isNotEmpty)
                .toList()
            : <AIQuestionOption>[];
  }
}

class AIAgent {
  late String name;
  late String prompt;
  late AIModel model;
  late WoxImage icon;

  AIAgent({required this.name, required this.prompt, required this.model, WoxImage? icon}) : icon = icon ?? WoxImage(imageType: "emoji", imageData: "🤖");

  AIAgent.fromJson(Map<String, dynamic> json) {
    name = json['Name'] ?? "";
    prompt = json['Prompt'] ?? "";
    model = json['Model'] != null ? AIModel.fromJson(json['Model']) : AIModel(name: "", provider: "", providerAlias: "");
    icon = json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : WoxImage(imageType: "emoji", imageData: "🤖");
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Name'] = name;
    data['Prompt'] = prompt;
    data['Model'] = model.toJson();
    data['Icon'] = icon.toJson();
    return data;
  }

  static AIAgent empty() {
    return AIAgent(name: "", prompt: "", model: AIModel(name: "", provider: "", providerAlias: ""), icon: WoxImage(imageType: "emoji", imageData: "🤖"));
  }
}

class ChatSelectItem {
  final String id;
  final String name;
  final WoxImage icon;
  final bool isCategory;
  final List<ChatSelectItem> children;
  Function(String traceId)? onExecute;

  ChatSelectItem({required this.id, required this.name, required this.icon, required this.isCategory, required this.children, this.onExecute});
}

// should be same as AIChatData in the ai chat plugin
class WoxAIChatData {
  late String id;
  late String title;
  late RxList<WoxAIChatConversation> conversations;
  late Rx<AIModel> model;
  late int createdAt;
  late int updatedAt;
  String? agentName;

  WoxAIChatData({required this.id, required this.title, required this.conversations, required this.model, required this.createdAt, required this.updatedAt, this.agentName});

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
      model: json['Model'] != null ? AIModel.fromJson(json['Model']).obs : AIModel(name: "", provider: "", providerAlias: "").obs,
      createdAt: json['CreatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
      updatedAt: json['UpdatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
      agentName: json['AgentName'],
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

    // Add agent name if available
    if (agentName != null && agentName!.isNotEmpty) {
      json['AgentName'] = agentName;
    }

    return json;
  }

  WoxAIChatData clone() {
    return WoxAIChatData.fromJson(toJson());
  }

  static WoxAIChatData empty() {
    return WoxAIChatData(
      id: "",
      title: "",
      conversations: RxList<WoxAIChatConversation>.from([]),
      model: AIModel(name: "", provider: "", providerAlias: "").obs,
      createdAt: 0,
      updatedAt: 0,
      agentName: null,
    );
  }
}

class WoxAIChatPreviewData {
  late WoxAIChatData activeChat;
  late List<WoxAIChatData> chats;

  WoxAIChatPreviewData({required this.activeChat, required this.chats});

  WoxAIChatPreviewData.fromJson(Map<String, dynamic> json) {
    activeChat = json['ActiveChat'] != null ? WoxAIChatData.fromJson(json['ActiveChat']) : WoxAIChatData.empty();
    final rawChats = json['Chats'];
    chats = rawChats is List ? rawChats.whereType<Map<String, dynamic>>().map(WoxAIChatData.fromJson).toList() : <WoxAIChatData>[];
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
    return ToolCallStatus.values.firstWhere((e) => e.value == value, orElse: () => ToolCallStatus.pending);
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

  int get duration =>
      (status == ToolCallStatus.streaming || status == ToolCallStatus.pending || status == ToolCallStatus.running)
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
    return ToolCallInfo(id: "", name: "", arguments: {}, response: "", delta: "", status: ToolCallStatus.pending, startTimestamp: 0, endTimestamp: 0);
  }
}

class WoxAIChatConversation {
  late String id;
  late WoxAIChatConversationRole role;
  late String text;
  late String reasoning; // Reasoning content from models that support reasoning (e.g., DeepSeek, OpenAI o1, qwen3)
  late List<WoxImage> images;
  late int timestamp;
  late ToolCallInfo toolCallInfo;

  WoxAIChatConversation({
    required this.id,
    required this.role,
    required this.text,
    required this.reasoning,
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
      reasoning: json['Reasoning'] ?? '',
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
      'Reasoning': reasoning,
      'Images': images.map((e) => e.toJson()).toList(),
      'Timestamp': timestamp,
      'ToolCallInfo': toolCallInfo.toJson(),
    };

    return json;
  }
}
