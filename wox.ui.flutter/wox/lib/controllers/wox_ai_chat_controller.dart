import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_list_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxAIChatController extends GetxController {
  final Rx<WoxAIChatData> aiChatData = WoxAIChatData.empty().obs;
  final RxList<WoxAIChatData> chats = <WoxAIChatData>[].obs;
  late final WoxListController<ChatSelectItem> chatSelectListController;
  String _loadedPreviewPayload = "";

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  String _makeProviderKey(String provider, String alias) {
    return alias.isEmpty ? provider : "${provider}_$alias";
  }

  (String provider, String alias) _parseProviderKey(String providerKey) {
    final idx = providerKey.indexOf("_");
    if (idx <= 0) return (providerKey, "");
    return (providerKey.substring(0, idx), providerKey.substring(idx + 1));
  }

  // Controllers and focus nodes
  final TextEditingController textController = TextEditingController();
  final TextEditingController aiQuestionAnswerController = TextEditingController();
  final WoxLauncherController launcherController = Get.find<WoxLauncherController>();
  final FocusNode aiChatFocusNode = FocusNode();
  final FocusNode aiQuestionPanelFocusNode = FocusNode();
  final FocusNode aiQuestionAnswerFocusNode = FocusNode();
  final ScrollController aiChatScrollController = ScrollController();
  final RxList<AIModel> aiModels = <AIModel>[].obs;
  final Rxn<AIQuestion> pendingAIQuestion = Rxn<AIQuestion>();

  // State for chat select panel
  final RxBool isShowChatSelectPanel = false.obs;
  final RxString currentChatSelectCategory = "".obs; // models, agents or empty

  // State for agents
  final RxList<AIAgent> availableAgents = <AIAgent>[].obs;
  final RxBool isLoadingAgents = false.obs;
  final RxBool isLoadingModels = false.obs;

  // Tool call expanded/collapsed states
  final RxMap<String, bool> toolCallExpandedStates = <String, bool>{}.obs;

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

  WoxAIChatController() {
    chatSelectListController = WoxListController<ChatSelectItem>(
      onItemExecuted: _onChatSelectItemExecuted,
      onFilterBoxEscPressed: (traceId) => hideChatSelectPanel(),
      itemHeightGetter: () => WoxThemeUtil.instance.getActionItemHeight(),
    );
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
    toolCallExpandedStates.clear();
  }

  WoxAIChatData _createDraftChat() {
    final now = DateTime.now().millisecondsSinceEpoch;
    final currentModel = aiChatData.value.model.value;
    final model = currentModel.name.isEmpty ? AIModel.empty() : AIModel(name: currentModel.name, provider: currentModel.provider, providerAlias: currentModel.providerAlias);
    return WoxAIChatData(id: const UuidV4().generate(), title: "", conversations: RxList<WoxAIChatConversation>.from([]), model: model.obs, createdAt: now, updatedAt: now);
  }

  void startNewChat() {
    aiChatData.value = _createDraftChat();
    toolCallExpandedStates.clear();
    if (aiChatData.value.model.value.name.isEmpty) {
      _setDefaultModel(const UuidV4().generate());
    }
    focusChatInput(const UuidV4().generate());
  }

  void selectChat(WoxAIChatData chat) {
    aiChatData.value = chat.clone();
    toolCallExpandedStates.clear();
    textController.clear();
    SchedulerBinding.instance.addPostFrameCallback((_) {
      scrollToBottomOfAiChat();
    });
    focusChatInput(const UuidV4().generate());
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
    } else if (resourceName == "agents") {
      fetchAvailableAgents(traceId);
    } else if (resourceName == "all") {
      reloadAIModels(traceId);
      fetchAvailableAgents(traceId);
    }
  }

  // Load available AI models
  void reloadAIModels(String traceId) {
    Logger.instance.debug(traceId, "start reloading ai models");

    isLoadingModels.value = true;
    WoxApi.instance
        .findAIModels(traceId)
        .then((models) {
          aiModels.assignAll(models);
          Logger.instance.debug(traceId, "reload ai models: ${aiModels.length}");
          if (isShowChatSelectPanel.value && currentChatSelectCategory.value == "models") {
            updateChatSelectItems();
          }
        })
        .catchError((error, stackTrace) {
          Logger.instance.error(traceId, 'Error fetching AI models: $error $stackTrace');
          aiModels.clear();
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

  void ensureAgentsLoaded(String traceId) {
    if (availableAgents.isNotEmpty || isLoadingAgents.value) {
      return;
    }
    fetchAvailableAgents(traceId);
  }

  void _onChatSelectItemExecuted(String traceId, WoxListItem<ChatSelectItem> item) {
    final chatSelectItem = item.data;
    if (chatSelectItem.onExecute != null) {
      chatSelectItem.onExecute!(traceId);
    }
  }

  // Update chat select items based on current category
  void updateChatSelectItems() {
    final List<WoxListItem<ChatSelectItem>> items = [];
    Logger.instance.debug(const UuidV4().generate(), "AI: Updating chat select items for category: ${currentChatSelectCategory.value}");

    if (currentChatSelectCategory.isEmpty) {
      items.add(
        WoxListItem<ChatSelectItem>(
          id: "agents",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
          title: tr("ui_ai_chat_select_agent"),
          subTitle: "",
          tails: [],
          isGroup: false,
          data: ChatSelectItem(
            id: "agents",
            name: tr("ui_ai_chat_select_agent"),
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
            isCategory: true,
            children: [],
            onExecute: (String traceId) {
              currentChatSelectCategory.value = "agents";
              chatSelectListController.clearFilter(traceId);
              updateChatSelectItems();
            },
          ),
        ),
      );

      items.add(
        WoxListItem<ChatSelectItem>(
          id: "models",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
          title: tr("ui_ai_chat_select_model"),
          subTitle: "",
          tails: [],
          isGroup: false,
          data: ChatSelectItem(
            id: "models",
            name: tr("ui_ai_chat_select_model"),
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
            isCategory: true,
            children: [],
            onExecute: (String traceId) {
              currentChatSelectCategory.value = "models";
              chatSelectListController.clearFilter(traceId);
              updateChatSelectItems();
            },
          ),
        ),
      );
    } else if (currentChatSelectCategory.value == "models") {
      // Show models grouped by provider
      // Group models by provider
      final modelsByProvider = <String, List<AIModel>>{};
      for (final model in aiModels) {
        final providerKey = _makeProviderKey(model.provider, model.providerAlias);
        modelsByProvider.putIfAbsent(providerKey, () => []).add(model);
      }

      // Sort providers
      final providers = modelsByProvider.keys.toList()..sort();

      // Add groups and models
      for (final providerKey in providers) {
        // Skip empty groups
        if (modelsByProvider[providerKey]!.isEmpty) continue;

        final providerInfo = _parseProviderKey(providerKey);
        final provider = providerInfo.$1;
        final alias = providerInfo.$2;
        final providerDisplayName = alias.isEmpty ? provider : "$provider ($alias)";

        // Add provider group header
        items.add(
          WoxListItem<ChatSelectItem>(
            id: "group_$providerKey",
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🏢"),
            title: providerDisplayName,
            subTitle: "",
            tails: [],
            isGroup: true,
            data: ChatSelectItem(
              id: "group_$providerKey",
              name: providerDisplayName,
              icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🏢"),
              isCategory: true,
              children: [],
              onExecute: null,
            ),
          ),
        );

        // Sort models within this provider
        final models = modelsByProvider[providerKey]!;
        models.sort((a, b) => a.name.compareTo(b.name));

        // Add models for this provider
        for (final model in models) {
          items.add(
            WoxListItem<ChatSelectItem>(
              id: "${providerKey}_${model.name}",
              icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
              title: model.name,
              subTitle: "",
              tails: [],
              isGroup: false,
              data: ChatSelectItem(
                id: "${providerKey}_${model.name}",
                name: model.name,
                icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
                isCategory: false,
                children: [],
                onExecute: (String traceId) {
                  aiChatData.value.model.value = AIModel(name: model.name, provider: model.provider, providerAlias: model.providerAlias);
                  hideChatSelectPanel();
                },
              ),
            ),
          );
        }
      }
    } else if (currentChatSelectCategory.value == "agents") {
      // Add "Cancel Selection" option at the top
      items.add(
        WoxListItem<ChatSelectItem>(
          id: "cancel_agent_selection",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "❌"),
          title: tr("ui_ai_chat_cancel_agent_selection"),
          subTitle: tr("ui_ai_chat_use_default_model_and_tools"),
          tails:
              (aiChatData.value.agentName == null || aiChatData.value.agentName!.isEmpty)
                  ? [WoxListItemTail.image(WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "✅"))]
                  : [],
          isGroup: false,
          data: ChatSelectItem(
            id: "cancel_agent_selection",
            name: tr("ui_ai_chat_cancel_agent_selection"),
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "❌"),
            isCategory: false,
            children: [],
            onExecute: (String traceId) {
              setCurrentAgent("");
              hideChatSelectPanel();
            },
          ),
        ),
      );

      // Display all available agents
      for (final agent in availableAgents) {
        final bool isSelected = aiChatData.value.agentName == agent.name;
        items.add(
          WoxListItem<ChatSelectItem>(
            id: agent.name,
            icon: agent.icon, // 使用agent自定义头像
            title: agent.name,
            subTitle: "${tr("ui_ai_chat_model")}: ${agent.model.name}",
            tails: isSelected ? [WoxListItemTail.image(WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "✅"))] : [],
            isGroup: false,
            data: ChatSelectItem(
              id: agent.name,
              name: agent.name,
              icon: agent.icon, // 使用agent自定义头像
              isCategory: false,
              children: [],
              onExecute: (String traceId) {
                setCurrentAgent(agent.name);
                hideChatSelectPanel();
              },
            ),
          ),
        );
      }
    }

    Logger.instance.debug(const UuidV4().generate(), "AI: Updating chat select list with ${items.length} items");
    chatSelectListController.updateItems(const UuidV4().generate(), items);

    SchedulerBinding.instance.addPostFrameCallback((_) {
      chatSelectListController.filterBoxFocusNode.requestFocus();
    });
  }

  // Show chat select panel
  void showChatSelectPanel() {
    Logger.instance.debug(const UuidV4().generate(), "AI: Showing chat select panel");
    isShowChatSelectPanel.value = true;
    currentChatSelectCategory.value = "";
    updateChatSelectItems();
  }

  // Show models panel directly
  void showModelsPanel() {
    ensureModelsLoaded(const UuidV4().generate());
    showChatSelectPanel();
    currentChatSelectCategory.value = "models";
    updateChatSelectItems();
  }

  // Show agents panel directly
  void showAgentsPanel() {
    ensureAgentsLoaded(const UuidV4().generate());
    showChatSelectPanel();
    currentChatSelectCategory.value = "agents";
    updateChatSelectItems();
  }

  // Hide chat select panel
  void hideChatSelectPanel() {
    isShowChatSelectPanel.value = false;
    chatSelectListController.clearFilter(const UuidV4().generate());
    focusChatInput(const UuidV4().generate());
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

  // Method to fetch available agents
  Future<void> fetchAvailableAgents(String traceId) async {
    Logger.instance.info(traceId, "start fetching AI agents");

    if (isLoadingAgents.value) return;
    isLoadingAgents.value = true;

    try {
      final agents = await WoxApi.instance.findAIAgents(traceId);
      availableAgents.assignAll(agents);
      Logger.instance.debug(traceId, "AI: loaded ${agents.length} agents");

      // Log each agent for debugging
      for (final agent in agents) {
        Logger.instance.debug(traceId, "AI: agent details - Name: ${agent.name}, Model: ${agent.model.name}");
      }

      // If currently displaying agent selection panel, update the list
      if (isShowChatSelectPanel.value && currentChatSelectCategory.value == "agents") {
        updateChatSelectItems();
      }
    } catch (e, s) {
      Logger.instance.error(traceId, 'AI: Error fetching AI agents: $e $s');
      availableAgents.clear();
    } finally {
      isLoadingAgents.value = false;
    }
  }

  // Method to set current agent
  void setCurrentAgent(String agentName) {
    Logger.instance.debug(const UuidV4().generate(), "AI: Setting current agent to: $agentName");
    aiChatData.value.agentName = agentName;

    // If an agent is selected, try to get agent details
    if (agentName.isNotEmpty) {
      bool agentFound = false;
      for (var agent in availableAgents) {
        if (agent.name == agentName) {
          Logger.instance.debug(const UuidV4().generate(), "AI: Found agent: ${agent.name}, setting model to ${agent.model.name}");
          aiChatData.value.model.value = agent.model;
          agentFound = true;
          break;
        }
      }
      if (!agentFound) {
        Logger.instance.error(const UuidV4().generate(), "AI: Agent with name $agentName not found in available agents");
      }
    } else {
      Logger.instance.debug(const UuidV4().generate(), "AI: No agent selected (empty agentName), setting default model");

      _setDefaultModel(const UuidV4().generate());
    }
  }

  Future<void> _setDefaultModel(String traceId) async {
    var defaultModel = await WoxApi.instance.findDefaultAIModel(traceId);
    aiChatData.value.model.value = defaultModel;
  }

  // Get the name of the current agent
  String getCurrentAgentName() {
    if (aiChatData.value.agentName == null || aiChatData.value.agentName!.isEmpty) {
      return "";
    }

    for (var agent in availableAgents) {
      if (agent.name == aiChatData.value.agentName) {
        return agent.name;
      }
    }

    return "";
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
        timestamp: DateTime.now().millisecondsSinceEpoch,
        toolCallInfo: ToolCallInfo.empty(),
      ),
    );
    aiChatData.value.updatedAt = DateTime.now().millisecondsSinceEpoch;
    _upsertChat(aiChatData.value);

    textController.clear();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      scrollToBottomOfAiChat();
    });

    WoxApi.instance.sendChatRequest(const UuidV4().generate(), aiChatData.value);
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
      aiChatData.value.updatedAt = data.updatedAt;

      if (data.agentName != null) {
        aiChatData.value.agentName = data.agentName;
      }

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
    aiQuestionAnswerController.clear();
    unawaited(WoxApi.instance.answerAIQuestion(const UuidV4().generate(), question.questionId, answer));
    focusChatInput(const UuidV4().generate());
  }

  // Copy message content to clipboard
  void copyMessageContent(WoxAIChatConversation message) {
    Clipboard.setData(ClipboardData(text: message.text));
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

    WoxApi.instance.sendChatRequest(const UuidV4().generate(), aiChatData.value);
  }

  // Edit user message
  void editUserMessage(WoxAIChatConversation message) {
    // Set the text controller to the message content
    textController.text = message.text;

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

    focusChatInput(const UuidV4().generate());
  }

  @override
  void onClose() {
    textController.dispose();
    aiQuestionAnswerController.dispose();
    chatSelectListController.dispose();
    aiChatFocusNode.dispose();
    aiQuestionPanelFocusNode.dispose();
    aiQuestionAnswerFocusNode.dispose();
    aiChatScrollController.dispose();
    super.onClose();
  }

  /// Drop reference lists so hidden window memory is released. Lists are lazily
  /// reloaded when the chat select panel is opened again.
  void clearReferenceDataCache() {
    aiModels.clear();
    availableAgents.clear();
  }
}
