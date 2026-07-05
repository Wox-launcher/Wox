import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/utils/log.dart';

enum ChatCommandPaletteGroup { model, skill }

class ChatCommandPaletteItem {
  final String id;
  final ChatCommandPaletteGroup group;
  final String title;
  final String subTitle;
  final String searchText;
  final bool selected;
  final AIModel? model;
  final AISkill? skill;

  ChatCommandPaletteItem({
    required this.id,
    required this.group,
    required this.title,
    required this.subTitle,
    required this.searchText,
    required this.selected,
    this.model,
    this.skill,
  });
}

class _SlashToken {
  final int start;
  final int end;
  final String query;

  _SlashToken({required this.start, required this.end, required this.query});
}

class WoxAIChatController extends GetxController {
  final Rx<WoxAIChatData> aiChatData = WoxAIChatData.empty().obs;
  final RxList<WoxAIChatData> chats = <WoxAIChatData>[].obs;
  String _loadedPreviewPayload = "";
  int? _slashTokenStart;
  int? _slashTokenEnd;
  String _slashQuery = "";
  bool _suppressInputListener = false;

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  // Controllers and focus nodes
  final TextEditingController textController = TextEditingController();
  final TextEditingController aiQuestionAnswerController = TextEditingController();
  final WoxLauncherController launcherController = Get.find<WoxLauncherController>();
  final FocusNode aiChatFocusNode = FocusNode();
  final FocusNode aiQuestionPanelFocusNode = FocusNode();
  final FocusNode aiQuestionAnswerFocusNode = FocusNode();
  final ScrollController aiChatScrollController = ScrollController();
  final ScrollController commandPaletteScrollController = ScrollController();
  final RxList<AIModel> aiModels = <AIModel>[].obs;
  final RxList<AISkill> aiSkills = <AISkill>[].obs;
  // Skill refs are draft-only until the user sends the current message.
  final RxList<AISkillRef> draftSkillRefs = <AISkillRef>[].obs;
  final RxBool isLoadingModels = false.obs;
  final RxBool isLoadingSkills = false.obs;
  final Rxn<AIQuestion> pendingAIQuestion = Rxn<AIQuestion>();
  final Rxn<AIQuestionOption> selectedAIQuestionOption = Rxn<AIQuestionOption>();
  final RxBool isDebugInspectorVisible = false.obs;
  final RxBool isGenerating = false.obs;

  // State for slash command palette
  final RxBool isCommandPaletteVisible = false.obs;
  final RxList<ChatCommandPaletteItem> commandPaletteItems = <ChatCommandPaletteItem>[].obs;
  final RxInt commandPaletteSelectedIndex = 0.obs;
  final RxBool isConversationSidebarCollapsed = false.obs;
  double _commandPaletteItemHeight = 38;
  double _commandPaletteHeaderHeight = 28;
  double _commandPaletteVerticalPadding = 8;

  // Chat disclosure states stay local to the current preview/chat selection.
  final RxMap<String, bool> toolCallExpandedStates = <String, bool>{}.obs;
  final RxMap<String, bool> toolActivityExpandedStates = <String, bool>{}.obs;
  final RxMap<String, bool> reasoningExpandedStates = <String, bool>{}.obs;

  WoxAIChatController() {
    textController.addListener(_handleChatInputChanged);
  }

  // Toggle tool call expanded/collapsed state
  void toggleToolCallExpanded(String conversationId) {
    if (toolCallExpandedStates.containsKey(conversationId)) {
      toolCallExpandedStates[conversationId] = !toolCallExpandedStates[conversationId]!;
    } else {
      toolCallExpandedStates[conversationId] = true;
    }
  }

  // Get tool call expanded/collapsed state
  bool isToolCallExpanded(String conversationId) {
    return toolCallExpandedStates[conversationId] ?? false;
  }

  void toggleToolActivityExpanded(String activityId) {
    toolActivityExpandedStates[activityId] = !(toolActivityExpandedStates[activityId] ?? false);
  }

  bool isToolActivityExpanded(String activityId) {
    return toolActivityExpandedStates[activityId] ?? false;
  }

  void toggleReasoningExpanded(String conversationId) {
    reasoningExpandedStates[conversationId] = !(reasoningExpandedStates[conversationId] ?? true);
  }

  bool isReasoningExpanded(String conversationId) {
    return reasoningExpandedStates[conversationId] ?? true;
  }

  void _clearChatExpansionStates() {
    toolCallExpandedStates.clear();
    toolActivityExpandedStates.clear();
    reasoningExpandedStates.clear();
  }

  // Loads the preview bootstrap payload once per query result. Runtime chat
  // updates arrive through SendChatResponse and must not be overwritten by
  // repeated preview rebuilds using the original payload.
  void loadPreviewData(String payload, WoxAIChatPreviewData data) {
    if (_loadedPreviewPayload == payload && aiChatData.value.id.isNotEmpty) {
      return;
    }

    _loadedPreviewPayload = payload;
    chats.assignAll(data.chats.map((chat) => chat.clone()));
    _sortChats();
    aiChatData.value = data.activeChat.clone();
    _clearChatExpansionStates();
  }

  WoxAIChatData _createDraftChat() {
    final now = DateTime.now().millisecondsSinceEpoch;
    final currentModel = aiChatData.value.model.value;
    final model = currentModel.name.isEmpty ? AIModel.empty() : AIModel(name: currentModel.name, provider: currentModel.provider, providerAlias: currentModel.providerAlias);
    return WoxAIChatData(
      id: const UuidV4().generate(),
      title: "",
      conversations: RxList<WoxAIChatConversation>.from([]),
      compactionEntries: RxList<AIChatCompactionEntry>.from([]),
      model: model.obs,
      debugTrace: Rxn<AIChatDebugTrace>(),
      createdAt: now,
      updatedAt: now,
    );
  }

  void startNewChat() {
    aiChatData.value = _createDraftChat();
    draftSkillRefs.clear();
    _clearChatExpansionStates();
    isGenerating.value = false;
    hideCommandPalette();
    collapseConversationSidebar(const UuidV4().generate());
    if (aiChatData.value.model.value.name.isEmpty) {
      _setDefaultModel(const UuidV4().generate());
    }
    focusChatInput(const UuidV4().generate());
  }

  void selectChat(WoxAIChatData chat) {
    aiChatData.value = chat.clone();
    draftSkillRefs.clear();
    _clearChatExpansionStates();
    isGenerating.value = false;
    textController.clear();
    hideCommandPalette();
    SchedulerBinding.instance.addPostFrameCallback((_) {
      scrollToBottomOfAiChat();
    });
    focusChatInput(const UuidV4().generate());
  }

  // Hides the conversation sidebar after selecting a chat so the message pane
  // gets the full preview width and the input field can receive focus cleanly.
  void collapseConversationSidebar(String traceId) {
    if (isConversationSidebarCollapsed.value) {
      return;
    }
    isConversationSidebarCollapsed.value = true;
    Logger.instance.debug(traceId, "AI chat conversation sidebar collapsed after chat selection");
  }

  // Chat mode starts with the conversation list folded so the message surface gets the full preview width.
  void collapseConversationSidebarForChatMode(String traceId) {
    if (isConversationSidebarCollapsed.value) {
      return;
    }
    isConversationSidebarCollapsed.value = true;
    Logger.instance.debug(traceId, "AI chat conversation sidebar collapsed for chat mode");
  }

  void toggleConversationSidebar(String traceId) {
    isConversationSidebarCollapsed.value = !isConversationSidebarCollapsed.value;
    Logger.instance.debug(traceId, "AI chat conversation sidebar collapsed: ${isConversationSidebarCollapsed.value}");
  }

  // Delete the chat through the preview-owned chat channel and update the local sidebar.
  Future<void> deleteChat(WoxAIChatData chat) async {
    final traceId = const UuidV4().generate();
    try {
      await WoxApi.instance.deleteAIChat(traceId, chat.id);
      chats.removeWhere((item) => item.id == chat.id);
      if (aiChatData.value.id == chat.id) {
        startNewChat();
      }
    } catch (error, stackTrace) {
      Logger.instance.error(traceId, "AI: failed to delete chat: $error $stackTrace");
      launcherController.showToolbarMsg(traceId, ToolbarMsg(text: error.toString(), displaySeconds: 3));
    }
  }

  // Ask the backend to refresh the title; the final title arrives via SendChatResponse.
  Future<void> summarizeChat(WoxAIChatData chat) async {
    final traceId = const UuidV4().generate();
    try {
      await WoxApi.instance.summarizeAIChat(traceId, chat.id);
    } catch (error, stackTrace) {
      Logger.instance.error(traceId, "AI: failed to summarize chat: $error $stackTrace");
      launcherController.showToolbarMsg(traceId, ToolbarMsg(text: error.toString(), displaySeconds: 3));
    }
  }

  void _upsertChat(WoxAIChatData data) {
    if (data.conversations.isEmpty) {
      return;
    }

    final snapshot = data.clone();
    final index = chats.indexWhere((chat) => chat.id == snapshot.id);
    if (index >= 0) {
      chats[index] = snapshot;
    } else {
      chats.add(snapshot);
    }
    _sortChats();
  }

  void _sortChats() {
    chats.sort((a, b) => b.updatedAt.compareTo(a.updatedAt));
  }

  void reloadChatResources(String traceId, {String resourceName = "all"}) {
    Logger.instance.debug(traceId, "start reloading AI chat resources");
    if (resourceName == "models") {
      reloadAIModels(traceId);
    } else if (resourceName == "skills") {
      reloadAISkills(traceId);
    } else if (resourceName == "all") {
      reloadAIModels(traceId);
      reloadAISkills(traceId);
    }
  }

  // Load available AI models.
  void reloadAIModels(String traceId) {
    Logger.instance.debug(traceId, "start reloading ai models");

    isLoadingModels.value = true;
    WoxApi.instance
        .findAIModels(traceId)
        .then((models) {
          aiModels.assignAll(models);
          Logger.instance.debug(traceId, "reload ai models: ${aiModels.length}");
          updateCommandPaletteItems();
        })
        .catchError((error, stackTrace) {
          Logger.instance.error(traceId, 'Error fetching AI models: $error $stackTrace');
          aiModels.clear();
          updateCommandPaletteItems();
        })
        .whenComplete(() {
          isLoadingModels.value = false;
        });
  }

  void ensureModelsLoaded(String traceId) {
    if (aiModels.isNotEmpty || isLoadingModels.value) {
      return;
    }
    reloadAIModels(traceId);
  }

  // Load available AI skills.
  void reloadAISkills(String traceId) {
    Logger.instance.debug(traceId, "start reloading ai skills");

    isLoadingSkills.value = true;
    WoxApi.instance
        .findAISkills(traceId)
        .then((skills) {
          aiSkills.assignAll(skills);
          Logger.instance.debug(traceId, "reload ai skills: ${aiSkills.length}");
          updateCommandPaletteItems();
        })
        .catchError((error, stackTrace) {
          Logger.instance.error(traceId, 'Error fetching AI skills: $error $stackTrace');
          aiSkills.clear();
          updateCommandPaletteItems();
        })
        .whenComplete(() {
          isLoadingSkills.value = false;
        });
  }

  void ensureSkillsLoaded(String traceId) {
    if (aiSkills.isNotEmpty || isLoadingSkills.value) {
      return;
    }
    reloadAISkills(traceId);
  }

  // Rebuild the command palette from the current slash token and resource lists.
  void updateCommandPaletteItems() {
    if (!isCommandPaletteVisible.value) {
      return;
    }

    final query = _slashQuery.toLowerCase();
    final items = <ChatCommandPaletteItem>[];
    final selectedModel = aiChatData.value.model.value;

    final sortedModels =
        aiModels.toList()..sort((a, b) {
          final providerCompare = _modelProviderLabel(a).compareTo(_modelProviderLabel(b));
          if (providerCompare != 0) return providerCompare;
          return a.name.compareTo(b.name);
        });
    for (final model in sortedModels) {
      final item = ChatCommandPaletteItem(
        id: "model:${model.provider}:${model.providerAlias}:${model.name}",
        group: ChatCommandPaletteGroup.model,
        title: model.name,
        subTitle: _modelProviderLabel(model),
        searchText: "model 模型 ${model.name} ${model.provider} ${model.providerAlias}",
        selected: selectedModel.name == model.name && selectedModel.provider == model.provider && selectedModel.providerAlias == model.providerAlias,
        model: model,
      );
      if (_matchesPaletteQuery(item, query)) {
        items.add(item);
      }
    }

    final sortedSkills =
        aiSkills.where((skill) => skill.enabled).toList()..sort((a, b) {
          final sourceCompare = _skillSourceLabel(a).compareTo(_skillSourceLabel(b));
          if (sourceCompare != 0) return sourceCompare;
          return a.name.compareTo(b.name);
        });
    for (final skill in sortedSkills) {
      final item = ChatCommandPaletteItem(
        id: "skill:${skill.id}",
        group: ChatCommandPaletteGroup.skill,
        title: skill.name,
        subTitle: _skillSubtitle(skill),
        searchText: "skill 技能 ${skill.name} ${skill.description} ${skill.source} ${skill.sourceName}",
        selected: isDraftSkillSelected(skill),
        skill: skill,
      );
      if (_matchesPaletteQuery(item, query)) {
        items.add(item);
      }
    }

    commandPaletteItems.assignAll(items);
    if (items.isEmpty) {
      commandPaletteSelectedIndex.value = 0;
    } else if (commandPaletteSelectedIndex.value >= items.length) {
      commandPaletteSelectedIndex.value = items.length - 1;
    }
    ensureCommandPaletteSelectionVisible();
  }

  bool _matchesPaletteQuery(ChatCommandPaletteItem item, String query) {
    if (query.isEmpty) {
      return true;
    }
    return item.searchText.toLowerCase().contains(query) || item.title.toLowerCase().contains(query) || item.subTitle.toLowerCase().contains(query);
  }

  String _modelProviderLabel(AIModel model) {
    if (model.providerAlias.isEmpty) {
      return model.provider;
    }
    return "${model.provider} (${model.providerAlias})";
  }

  String _skillSourceLabel(AISkill skill) {
    return skill.sourceName.isEmpty ? skill.source : skill.sourceName;
  }

  String _skillSubtitle(AISkill skill) {
    final source = _skillSourceLabel(skill);
    if (skill.description.isEmpty) {
      return source;
    }
    if (source.isEmpty) {
      return skill.description;
    }
    return "$source · ${skill.description}";
  }

  void _handleChatInputChanged() {
    if (_suppressInputListener) {
      return;
    }

    final token = _findSlashToken();
    if (token == null) {
      hideCommandPalette();
      return;
    }

    final queryChanged = _slashQuery != token.query;
    _slashTokenStart = token.start;
    _slashTokenEnd = token.end;
    _slashQuery = token.query;
    isCommandPaletteVisible.value = true;
    if (queryChanged) {
      commandPaletteSelectedIndex.value = 0;
    }
    ensureModelsLoaded(const UuidV4().generate());
    ensureSkillsLoaded(const UuidV4().generate());
    updateCommandPaletteItems();
  }

  _SlashToken? _findSlashToken() {
    final text = textController.text;
    if (text.isEmpty) {
      return null;
    }

    final selection = textController.selection;
    final cursor = (selection.isValid ? selection.extentOffset : text.length).clamp(0, text.length).toInt();
    var start = cursor;
    while (start > 0 && !_isTokenBoundary(text.codeUnitAt(start - 1))) {
      start--;
    }
    if (start >= text.length || text[start] != "/") {
      return null;
    }

    var end = cursor;
    while (end < text.length && !_isTokenBoundary(text.codeUnitAt(end))) {
      end++;
    }

    return _SlashToken(start: start, end: end, query: text.substring(start + 1, end).trim());
  }

  bool _isTokenBoundary(int codeUnit) {
    return codeUnit == 32 || codeUnit == 9 || codeUnit == 10 || codeUnit == 13;
  }

  void hideCommandPalette() {
    isCommandPaletteVisible.value = false;
    commandPaletteItems.clear();
    commandPaletteSelectedIndex.value = 0;
    _slashTokenStart = null;
    _slashTokenEnd = null;
    _slashQuery = "";
  }

  bool handleCommandPaletteEscape() {
    if (!isCommandPaletteVisible.value) {
      return false;
    }
    hideCommandPalette();
    focusChatInput(const UuidV4().generate());
    return true;
  }

  bool executeSelectedCommandPaletteItem() {
    if (!isCommandPaletteVisible.value) {
      return false;
    }
    if (commandPaletteItems.isEmpty) {
      return true;
    }

    final index = commandPaletteSelectedIndex.value.clamp(0, commandPaletteItems.length - 1).toInt();
    executeCommandPaletteItem(commandPaletteItems[index]);
    return true;
  }

  void executeCommandPaletteItem(ChatCommandPaletteItem item) {
    if (item.model != null) {
      final model = item.model!;
      aiChatData.value.model.value = AIModel(name: model.name, provider: model.provider, providerAlias: model.providerAlias);
    }

    if (item.skill != null) {
      final skill = item.skill!;
      if (isDraftSkillSelected(skill)) {
        draftSkillRefs.removeWhere((ref) => ref.id == skill.id);
      } else {
        draftSkillRefs.add(_skillRefFromAISkill(skill));
      }
    }

    _removeSlashTokenFromInput();
    hideCommandPalette();
    focusChatInput(const UuidV4().generate());
  }

  void moveCommandPaletteSelection(int delta) {
    if (!isCommandPaletteVisible.value || commandPaletteItems.isEmpty) {
      return;
    }
    final nextIndex = (commandPaletteSelectedIndex.value + delta).clamp(0, commandPaletteItems.length - 1).toInt();
    commandPaletteSelectedIndex.value = nextIndex;
    ensureCommandPaletteSelectionVisible();
  }

  void updateCommandPaletteLayoutMetrics({required double itemHeight, required double headerHeight, required double verticalPadding}) {
    _commandPaletteItemHeight = itemHeight;
    _commandPaletteHeaderHeight = headerHeight;
    _commandPaletteVerticalPadding = verticalPadding;
  }

  void ensureCommandPaletteSelectionVisible() {
    if (!isCommandPaletteVisible.value || commandPaletteItems.isEmpty) {
      return;
    }

    SchedulerBinding.instance.addPostFrameCallback((_) {
      if (!commandPaletteScrollController.hasClients || commandPaletteItems.isEmpty) {
        return;
      }

      final index = commandPaletteSelectedIndex.value.clamp(0, commandPaletteItems.length - 1).toInt();
      final itemTop = _commandPaletteItemOffset(index);
      final itemBottom = itemTop + _commandPaletteItemHeight;
      final position = commandPaletteScrollController.position;
      final viewportTop = position.pixels;
      final viewportBottom = viewportTop + position.viewportDimension;
      final margin = 4.0;

      double? targetOffset;
      if (itemTop < viewportTop + margin) {
        targetOffset = itemTop - margin;
      } else if (itemBottom > viewportBottom - margin) {
        targetOffset = itemBottom - position.viewportDimension + margin;
      }

      if (targetOffset == null) {
        return;
      }

      final clampedOffset = targetOffset.clamp(position.minScrollExtent, position.maxScrollExtent);
      commandPaletteScrollController.animateTo(clampedOffset.toDouble(), duration: const Duration(milliseconds: 90), curve: Curves.easeOut);
    });
  }

  double _commandPaletteItemOffset(int targetIndex) {
    var offset = _commandPaletteVerticalPadding;
    ChatCommandPaletteGroup? currentGroup;
    for (var i = 0; i <= targetIndex; i++) {
      final item = commandPaletteItems[i];
      if (currentGroup != item.group) {
        currentGroup = item.group;
        offset += _commandPaletteHeaderHeight;
      }
      if (i == targetIndex) {
        return offset;
      }
      offset += _commandPaletteItemHeight;
    }
    return offset;
  }

  // Opens the same slash palette from the launcher action hotkey by inserting a slash token.
  void openCommandPaletteFromActionHotkey() {
    if (_findSlashToken() != null) {
      _handleChatInputChanged();
      return;
    }

    final text = textController.text;
    final selection = textController.selection;
    final cursor = (selection.isValid ? selection.extentOffset : text.length).clamp(0, text.length).toInt();
    final needsLeadingSpace = cursor > 0 && !_isTokenBoundary(text.codeUnitAt(cursor - 1));
    final insertedText = needsLeadingSpace ? " /" : "/";
    final newText = text.replaceRange(cursor, cursor, insertedText);
    final newCursor = cursor + insertedText.length;
    textController.value = TextEditingValue(text: newText, selection: TextSelection.collapsed(offset: newCursor));
  }

  void _removeSlashTokenFromInput() {
    final start = _slashTokenStart;
    final end = _slashTokenEnd;
    if (start == null || end == null) {
      return;
    }

    final text = textController.text;
    if (start < 0 || end > text.length || start > end) {
      return;
    }

    final before = text.substring(0, start);
    final after = text.substring(end);
    final needsSpace = before.isNotEmpty && after.isNotEmpty && !before.endsWith(" ") && !after.startsWith(" ");
    final replacement = needsSpace ? " " : "";
    final newText = before + replacement + after;
    final cursor = before.length + replacement.length;

    _suppressInputListener = true;
    textController.value = TextEditingValue(text: newText, selection: TextSelection.collapsed(offset: cursor));
    _suppressInputListener = false;
  }

  bool isDraftSkillSelected(AISkill skill) {
    return draftSkillRefs.any((ref) => ref.id == skill.id);
  }

  void removeDraftSkillRef(AISkillRef ref) {
    draftSkillRefs.removeWhere((item) => item.id == ref.id);
    updateCommandPaletteItems();
    focusChatInput(const UuidV4().generate());
  }

  AISkillRef _skillRefFromAISkill(AISkill skill) {
    final refPath = skill.manifestPath.isNotEmpty ? skill.manifestPath : skill.path;
    return AISkillRef(id: skill.id, name: skill.name, path: refPath, source: skill.source);
  }

  // Scroll to bottom of AI chat
  void scrollToBottomOfAiChat() {
    if (aiChatScrollController.hasClients) {
      aiChatScrollController.jumpTo(aiChatScrollController.position.maxScrollExtent);
    }
  }

  // Focus the chat input field after the chat preview is visible.
  void focusChatInput(String traceId) {
    Logger.instance.info(traceId, "focus to chat input");
    SchedulerBinding.instance.addPostFrameCallback((_) {
      aiChatFocusNode.requestFocus();
    });
  }

  Future<void> _setDefaultModel(String traceId) async {
    var defaultModel = await WoxApi.instance.findDefaultAIModel(traceId);
    aiChatData.value.model.value = defaultModel;
  }

  void sendMessage() {
    var text = textController.text.trim();
    // Check if AI model is selected
    if (aiChatData.value.model.value.name.isEmpty) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: tr("ui_ai_chat_select_model"), displaySeconds: 3));
      return;
    }
    // check if the text is empty
    if (text.isEmpty) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: tr("ui_ai_chat_enter_message"), displaySeconds: 3));
      return;
    }

    // append user message to chat data
    aiChatData.value.conversations.add(
      WoxAIChatConversation(
        id: const UuidV4().generate(),
        role: WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value,
        text: text,
        reasoning: '',
        images: [],
        skillRefs: List<AISkillRef>.from(draftSkillRefs),
        timestamp: DateTime.now().millisecondsSinceEpoch,
        toolCallInfo: ToolCallInfo.empty(),
      ),
    );
    aiChatData.value.updatedAt = DateTime.now().millisecondsSinceEpoch;
    _upsertChat(aiChatData.value);

    textController.clear();
    draftSkillRefs.clear();
    hideCommandPalette();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      scrollToBottomOfAiChat();
    });

    isGenerating.value = true;
    WoxApi.instance.sendChatRequest(const UuidV4().generate(), aiChatData.value);
  }

  // Stop the active streaming session for the current chat.
  void stopChat() {
    WoxApi.instance.stopChatRequest(const UuidV4().generate(), aiChatData.value.id);
  }

  String formatTimestamp(int timestamp) {
    final date = DateTime.fromMillisecondsSinceEpoch(timestamp);
    return '${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
  }

  void handleChatResponse(String traceId, WoxAIChatData data) {
    _upsertChat(data);

    // Update the chat data with the response
    if (data.id == aiChatData.value.id) {
      final shouldStickToBottom = !aiChatScrollController.hasClients || aiChatScrollController.position.maxScrollExtent - aiChatScrollController.position.pixels <= 64;

      aiChatData.value.title = data.title;
      aiChatData.value.conversations.assignAll(data.conversations);
      aiChatData.value.compactionEntries.assignAll(data.compactionEntries);
      aiChatData.value.debugTrace.value = data.debugTrace.value;
      aiChatData.value.updatedAt = data.updatedAt;

      // Sync streaming state from backend so the send/stop button toggles correctly.
      isGenerating.value = data.isStreaming;

      // Keep streaming pinned to the bottom while the user is already reading
      // the latest message, but preserve manual scrollback once they move away.
      if (shouldStickToBottom) {
        // Scroll to bottom after a short delay to ensure the new message is rendered
        SchedulerBinding.instance.addPostFrameCallback((_) {
          scrollToBottomOfAiChat();
        });
      }
    }
  }

  // Handles ask_user inside the chat preview surface; the launcher only routes the websocket event here.
  void handleAIQuestionRequest(String traceId, dynamic data) {
    String questionId = "";
    try {
      final rawQuestion = data is Map ? Map<String, dynamic>.from(data) : <String, dynamic>{};
      final question = AIQuestion.fromJson(rawQuestion);
      questionId = question.questionId;
      if (questionId.isEmpty) {
        Logger.instance.warn(traceId, "AIQuestion request missing questionId");
        return;
      }

      final currentQuestion = pendingAIQuestion.value;
      if (currentQuestion != null && currentQuestion.questionId != question.questionId) {
        unawaited(WoxApi.instance.answerAIQuestion(traceId, currentQuestion.questionId, "User cancelled"));
      }

      aiQuestionAnswerController.clear();
      selectedAIQuestionOption.value = null;
      pendingAIQuestion.value = question;
      SchedulerBinding.instance.addPostFrameCallback((_) {
        if (!isClosed && pendingAIQuestion.value?.questionId == question.questionId) {
          if (question.options.isEmpty) {
            aiQuestionAnswerFocusNode.requestFocus();
          } else {
            aiQuestionPanelFocusNode.requestFocus();
          }
        }
      });
    } catch (e, s) {
      Logger.instance.error(traceId, "AIQuestion request failed: $e $s");
      if (questionId.isNotEmpty) {
        unawaited(WoxApi.instance.answerAIQuestion(traceId, questionId, "Failed to show question UI: $e"));
      }
    }
  }

  void submitPendingAIQuestionAnswer() {
    final answer = aiQuestionAnswerController.text.trim();
    answerPendingAIQuestion(answer.isEmpty ? "User cancelled" : answer);
  }

  // Select an option from the ask_user panel (does not submit).
  void selectAIQuestionOption(AIQuestionOption option) {
    if (selectedAIQuestionOption.value?.value == option.value) {
      selectedAIQuestionOption.value = null;
    } else {
      selectedAIQuestionOption.value = option;
    }
    // Focus the text input when the free-text (last) option is selected.
    final question = pendingAIQuestion.value;
    if (question != null && question.options.isNotEmpty && selectedAIQuestionOption.value != null) {
      final isLastOption = question.options.last.value == selectedAIQuestionOption.value!.value;
      if (isLastOption) {
        SchedulerBinding.instance.addPostFrameCallback((_) {
          if (!isClosed) aiQuestionAnswerFocusNode.requestFocus();
        });
      }
    }
  }

  // Submit the selected option or typed free-text answer.
  void submitSelectedAIQuestionAnswer() {
    final question = pendingAIQuestion.value;
    if (question == null) return;

    final selected = selectedAIQuestionOption.value;
    if (selected != null) {
      // If the free-text (last) option is selected, use typed text if available.
      final isLastOption = question.options.isNotEmpty && question.options.last.value == selected.value;
      if (isLastOption) {
        final typed = aiQuestionAnswerController.text.trim();
        answerPendingAIQuestion(typed.isEmpty ? selected.value : typed);
      } else {
        answerPendingAIQuestion(selected.value);
      }
    } else {
      answerPendingAIQuestion("User cancelled");
    }
  }

  bool isAIQuestionFreeTextSelected() {
    final question = pendingAIQuestion.value;
    final selected = selectedAIQuestionOption.value;
    if (question == null || selected == null || question.options.isEmpty) return false;
    return question.options.last.value == selected.value;
  }

  void cancelPendingAIQuestion() {
    answerPendingAIQuestion("User cancelled");
  }

  // Resolves the current ask_user request through the chat HTTP channel.
  void answerPendingAIQuestion(String answer) {
    final question = pendingAIQuestion.value;
    if (question == null) {
      return;
    }

    pendingAIQuestion.value = null;
    selectedAIQuestionOption.value = null;
    aiQuestionAnswerController.clear();
    unawaited(WoxApi.instance.answerAIQuestion(const UuidV4().generate(), question.questionId, answer));
    focusChatInput(const UuidV4().generate());
  }

  // Copy message content to clipboard
  void copyMessageContent(WoxAIChatConversation message) {
    Clipboard.setData(ClipboardData(text: message.text));
    launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: tr("ui_ai_chat_message_copied"), displaySeconds: 2));
  }

  // Copy debug inspector section content to clipboard.
  void copyDebugSectionContent(String content) {
    Clipboard.setData(ClipboardData(text: content));
    launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: tr("ui_ai_chat_message_copied"), displaySeconds: 2));
  }

  // Regenerate AI response for a specific message or the last user message
  void regenerateAIResponse(String messageId) {
    int userMessageIndex = -1;

    // Find the AI message and its corresponding user message
    int aiMessageIndex = aiChatData.value.conversations.indexWhere((m) => m.id == messageId);
    if (aiMessageIndex == -1) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: tr("ui_ai_chat_message_not_found"), displaySeconds: 3));
      return;
    }

    // Find the user message that comes before this AI message
    for (int i = aiMessageIndex - 1; i >= 0; i--) {
      if (aiChatData.value.conversations[i].role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value) {
        userMessageIndex = i;
        break;
      }
    }

    if (userMessageIndex == -1) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: tr("ui_ai_chat_no_user_message_to_regenerate"), displaySeconds: 3));
      return;
    }

    // Remove all messages after the user message
    if (userMessageIndex < aiChatData.value.conversations.length - 1) {
      aiChatData.value.conversations.removeRange(userMessageIndex + 1, aiChatData.value.conversations.length);
    }
    aiChatData.value.compactionEntries.clear();
    aiChatData.value.debugTrace.value = null;

    isGenerating.value = true;
    WoxApi.instance.sendChatRequest(const UuidV4().generate(), aiChatData.value);
  }

  // Edit user message
  void editUserMessage(WoxAIChatConversation message) {
    // Set the text controller to the message content
    textController.text = message.text;
    draftSkillRefs.assignAll(message.skillRefs);

    // Find the index of the message
    int messageIndex = aiChatData.value.conversations.indexWhere((m) => m.id == message.id);
    if (messageIndex == -1) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: tr("ui_ai_chat_message_not_found"), displaySeconds: 2));
      return;
    }

    // Remove this message and all subsequent messages
    if (messageIndex < aiChatData.value.conversations.length - 1) {
      aiChatData.value.conversations.removeRange(messageIndex, aiChatData.value.conversations.length);
    } else {
      // If it's the last message, just remove it
      aiChatData.value.conversations.removeLast();
    }
    aiChatData.value.compactionEntries.clear();
    aiChatData.value.debugTrace.value = null;

    focusChatInput(const UuidV4().generate());
  }

  void toggleDebugInspector() {
    isDebugInspectorVisible.value = !isDebugInspectorVisible.value;
  }

  @override
  void onClose() {
    textController.removeListener(_handleChatInputChanged);
    textController.dispose();
    aiQuestionAnswerController.dispose();
    aiChatFocusNode.dispose();
    aiQuestionPanelFocusNode.dispose();
    aiQuestionAnswerFocusNode.dispose();
    aiChatScrollController.dispose();
    commandPaletteScrollController.dispose();
    super.onClose();
  }

  /// Drop reference lists so hidden window memory is released. Lists are lazily
  /// reloaded when the command palette is opened again.
  void clearReferenceDataCache() {
    aiModels.clear();
    aiSkills.clear();
  }
}
