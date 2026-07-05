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

class AISkillRef {
  late String id;
  late String name;
  late String path;
  late String source;

  AISkillRef({required this.id, required this.name, required this.path, required this.source});

  AISkillRef.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? json['id'] ?? "";
    name = json['Name'] ?? json['name'] ?? "";
    path = json['Path'] ?? json['path'] ?? "";
    source = json['Source'] ?? json['source'] ?? "";
  }

  Map<String, dynamic> toJson() {
    return {'Id': id, 'Name': name, 'Path': path, 'Source': source};
  }
}

class AIChatCompactionEntry {
  late String id;
  late String summary;
  late String firstCompactedConversationId;
  late String lastCompactedConversationId;
  late String firstKeptConversationId;
  late int estimatedTokensBefore;
  late int estimatedTokensAfter;
  late int conversationCount;
  late AIModel model;
  late int createdAt;

  AIChatCompactionEntry({
    required this.id,
    required this.summary,
    required this.firstCompactedConversationId,
    required this.lastCompactedConversationId,
    required this.firstKeptConversationId,
    required this.estimatedTokensBefore,
    required this.estimatedTokensAfter,
    required this.conversationCount,
    required this.model,
    required this.createdAt,
  });

  AIChatCompactionEntry.fromJson(Map<String, dynamic> json) {
    id = json['Id'] ?? "";
    summary = json['Summary'] ?? "";
    firstCompactedConversationId = json['FirstCompactedConversationId'] ?? "";
    lastCompactedConversationId = json['LastCompactedConversationId'] ?? "";
    firstKeptConversationId = json['FirstKeptConversationId'] ?? "";
    estimatedTokensBefore = json['EstimatedTokensBefore'] ?? 0;
    estimatedTokensAfter = json['EstimatedTokensAfter'] ?? 0;
    conversationCount = json['ConversationCount'] ?? 0;
    model = json['Model'] != null ? AIModel.fromJson(json['Model']) : AIModel.empty();
    createdAt = json['CreatedAt'] ?? 0;
  }

  Map<String, dynamic> toJson() {
    return {
      'Id': id,
      'Summary': summary,
      'FirstCompactedConversationId': firstCompactedConversationId,
      'LastCompactedConversationId': lastCompactedConversationId,
      'FirstKeptConversationId': firstKeptConversationId,
      'EstimatedTokensBefore': estimatedTokensBefore,
      'EstimatedTokensAfter': estimatedTokensAfter,
      'ConversationCount': conversationCount,
      'Model': model.toJson(),
      'CreatedAt': createdAt,
    };
  }
}

class AIChatDebugTrace {
  late List<AIChatDebugEvent> events;
  late int estimatedPersistedTokens;
  late int estimatedRuntimeTokens;

  AIChatDebugTrace({required this.events, required this.estimatedPersistedTokens, required this.estimatedRuntimeTokens});

  AIChatDebugTrace.fromJson(Map<String, dynamic> json) {
    events = _parseEvents(json['Events']);
    estimatedPersistedTokens = json['EstimatedPersistedTokens'] ?? 0;
    estimatedRuntimeTokens = json['EstimatedRuntimeTokens'] ?? 0;
  }

  Map<String, dynamic> toJson() {
    return {'Events': events.map((e) => e.toJson()).toList(), 'EstimatedPersistedTokens': estimatedPersistedTokens, 'EstimatedRuntimeTokens': estimatedRuntimeTokens};
  }

  static List<WoxAIChatConversation> _parseConversations(dynamic raw) {
    if (raw is! List) {
      return <WoxAIChatConversation>[];
    }
    return raw.whereType<Map>().map((item) => WoxAIChatConversation.fromJson(Map<String, dynamic>.from(item))).toList();
  }

  static List<AIChatDebugEvent> _parseEvents(dynamic raw) {
    if (raw is! List) {
      return <AIChatDebugEvent>[];
    }
    return raw.whereType<Map>().map((item) => AIChatDebugEvent.fromJson(Map<String, dynamic>.from(item))).toList();
  }
}

class AIChatDebugEvent {
  late int seq;
  late int timestamp;
  late String type;
  late String name;
  late int iteration;
  late String callId;
  late String parentCallId;
  late AIModel model;
  late String status;
  late String error;
  late List<WoxAIChatConversation> request;
  late List<WoxAIChatConversation> response;
  late List<AIChatDebugTool> visibleTools;
  ToolCallInfo? toolCallInfo;

  AIChatDebugEvent({
    required this.seq,
    required this.timestamp,
    required this.type,
    required this.name,
    required this.iteration,
    required this.callId,
    required this.parentCallId,
    required this.model,
    required this.status,
    required this.error,
    required this.request,
    required this.response,
    required this.visibleTools,
    required this.toolCallInfo,
  });

  AIChatDebugEvent.fromJson(Map<String, dynamic> json) {
    seq = json['Seq'] ?? 0;
    timestamp = json['Timestamp'] ?? 0;
    type = json['Type'] ?? "";
    name = json['Name'] ?? "";
    iteration = json['Iteration'] ?? 0;
    callId = json['CallId'] ?? "";
    parentCallId = json['ParentCallId'] ?? "";
    model = json['Model'] is Map ? AIModel.fromJson(Map<String, dynamic>.from(json['Model'])) : AIModel.empty();
    status = json['Status'] ?? "";
    error = json['Error'] ?? "";
    request = AIChatDebugTrace._parseConversations(json['Request']);
    response = AIChatDebugTrace._parseConversations(json['Response']);
    visibleTools = _parseVisibleTools(json['VisibleTools']);
    toolCallInfo = json['ToolCallInfo'] is Map ? ToolCallInfo.fromJson(Map<String, dynamic>.from(json['ToolCallInfo'])) : null;
  }

  Map<String, dynamic> toJson() {
    return {
      'Seq': seq,
      'Timestamp': timestamp,
      'Type': type,
      'Name': name,
      'Iteration': iteration,
      'CallId': callId,
      'ParentCallId': parentCallId,
      'Model': model.toJson(),
      'Status': status,
      'Error': error,
      'Request': request.map((e) => e.toJson()).toList(),
      'Response': response.map((e) => e.toJson()).toList(),
      'VisibleTools': visibleTools.map((e) => e.toJson()).toList(),
      'ToolCallInfo': toolCallInfo?.toJson(),
    };
  }

  static List<AIChatDebugTool> _parseVisibleTools(dynamic raw) {
    if (raw is! List) {
      return <AIChatDebugTool>[];
    }
    return raw.whereType<Map>().map((item) => AIChatDebugTool.fromJson(Map<String, dynamic>.from(item))).toList();
  }
}

class AIChatDebugTool {
  late String name;
  late String description;
  late String source;
  late String server;

  AIChatDebugTool({required this.name, required this.description, required this.source, required this.server});

  AIChatDebugTool.fromJson(Map<String, dynamic> json) {
    name = json['Name'] ?? "";
    description = json['Description'] ?? "";
    source = json['Source'] ?? "";
    server = json['Server'] ?? "";
  }

  Map<String, dynamic> toJson() {
    return {'Name': name, 'Description': description, 'Source': source, 'Server': server};
  }
}

// should be same as AIChatData in the ai chat plugin
class WoxAIChatData {
  late String id;
  late String title;
  late RxList<WoxAIChatConversation> conversations;
  late RxList<AIChatCompactionEntry> compactionEntries;
  late Rx<AIModel> model;
  late Rxn<AIChatDebugTrace> debugTrace;
  late int createdAt;
  late int updatedAt;
  late bool isStreaming;

  WoxAIChatData({
    required this.id,
    required this.title,
    required this.conversations,
    required this.compactionEntries,
    required this.model,
    required this.debugTrace,
    required this.createdAt,
    required this.updatedAt,
    this.isStreaming = false,
  });

  static WoxAIChatData fromJson(Map<String, dynamic> json) {
    List<WoxAIChatConversation> conversations = [];
    if (json['Conversations'] != null) {
      for (var e in json['Conversations']) {
        conversations.add(WoxAIChatConversation.fromJson(e));
      }
    }

    List<AIChatCompactionEntry> compactionEntries = [];
    if (json['CompactionEntries'] is List) {
      for (final e in json['CompactionEntries']) {
        if (e is Map) {
          compactionEntries.add(AIChatCompactionEntry.fromJson(Map<String, dynamic>.from(e)));
        }
      }
    }

    final parsedDebugTrace = Rxn<AIChatDebugTrace>();
    if (json['DebugTrace'] is Map) {
      parsedDebugTrace.value = AIChatDebugTrace.fromJson(Map<String, dynamic>.from(json['DebugTrace']));
    }

    return WoxAIChatData(
      id: json['Id'] ?? "",
      title: json['Title'] ?? "",
      conversations: RxList<WoxAIChatConversation>.from(conversations),
      compactionEntries: RxList<AIChatCompactionEntry>.from(compactionEntries),
      model: json['Model'] != null ? AIModel.fromJson(json['Model']).obs : AIModel(name: "", provider: "", providerAlias: "").obs,
      debugTrace: parsedDebugTrace,
      createdAt: json['CreatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
      updatedAt: json['UpdatedAt'] ?? DateTime.now().millisecondsSinceEpoch,
      isStreaming: json['IsStreaming'] ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> json = {
      'Id': id,
      'Title': title,
      'Conversations': conversations.map((e) => e.toJson()).toList(),
      'CompactionEntries': compactionEntries.map((e) => e.toJson()).toList(),
      'Model': model.toJson(),
      'CreatedAt': createdAt,
      'UpdatedAt': updatedAt,
    };

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
      compactionEntries: RxList<AIChatCompactionEntry>.from([]),
      model: AIModel(name: "", provider: "", providerAlias: "").obs,
      debugTrace: Rxn<AIChatDebugTrace>(),
      createdAt: 0,
      updatedAt: 0,
      isStreaming: false,
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
  late List<AISkillRef> skillRefs;
  late int timestamp;
  late ToolCallInfo toolCallInfo;

  WoxAIChatConversation({
    required this.id,
    required this.role,
    required this.text,
    required this.reasoning,
    required this.images,
    required this.skillRefs,
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

    List<AISkillRef> skillRefs = [];
    if (json['SkillRefs'] is List) {
      for (final e in json['SkillRefs']) {
        if (e is Map) {
          skillRefs.add(AISkillRef.fromJson(Map<String, dynamic>.from(e)));
        }
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
      skillRefs: skillRefs,
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
      'SkillRefs': skillRefs.map((e) => e.toJson()).toList(),
      'Timestamp': timestamp,
      'ToolCallInfo': toolCallInfo.toJson(),
    };

    return json;
  }
}
