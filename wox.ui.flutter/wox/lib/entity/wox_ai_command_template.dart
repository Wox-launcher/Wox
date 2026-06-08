class AICommandThinkingModeValue {
  static const providerDefault = "provider_default";
  static const thinking = "thinking";
  static const nonThinking = "non_thinking";
}

class AICommandTemplateQueryHotkey {
  late String hotkey;
  late bool hideQueryBox;
  late bool hideToolbar;
  late bool isSilentExecution;
  late String width;
  late String maxResultCount;
  late String position;

  AICommandTemplateQueryHotkey.empty() {
    hotkey = "";
    hideQueryBox = false;
    hideToolbar = false;
    isSilentExecution = false;
    width = "";
    maxResultCount = "";
    position = "system_default";
  }

  AICommandTemplateQueryHotkey.fromJson(Map<String, dynamic>? json) {
    hotkey = json?["Hotkey"] ?? "";
    hideQueryBox = json?["HideQueryBox"] ?? false;
    hideToolbar = json?["HideToolbar"] ?? false;
    isSilentExecution = json?["IsSilentExecution"] ?? false;
    width = json?["Width"] == null || json?["Width"] == 0 ? "" : json!["Width"].toString();
    maxResultCount = json?["MaxResultCount"] == null || json?["MaxResultCount"] == 0 ? "" : json!["MaxResultCount"].toString();
    position = json?["Position"] ?? "system_default";
  }

  bool get hasQuery => hotkey.trim().isNotEmpty;
}

class AICommandTemplate {
  late String id;
  late String category;
  late String name;
  late String description;
  late String author;
  late String command;
  late String prompt;
  late String thinkingMode;
  late String defaultAction;
  late bool vision;
  late AICommandTemplateQueryHotkey recommendedQueryHotkey;

  AICommandTemplate.fromJson(Map<String, dynamic> json) {
    id = json["Id"] ?? "";
    category = json["Category"] ?? "";
    name = json["Name"] ?? "";
    description = json["Description"] ?? "";
    author = json["Author"] ?? "";
    command = json["Command"] ?? "";
    prompt = json["Prompt"] ?? "";
    thinkingMode = json["ThinkingMode"] ?? "provider_default";
    defaultAction = json["DefaultAction"] ?? "run";
    vision = json["Vision"] ?? false;
    recommendedQueryHotkey = AICommandTemplateQueryHotkey.fromJson(json["RecommendedQueryHotkey"]);
  }
}
