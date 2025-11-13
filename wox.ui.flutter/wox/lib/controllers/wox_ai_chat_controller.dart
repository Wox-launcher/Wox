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

class WoxAIChatController extends GetxController {
  final Rx<WoxAIChatData> aiChatData = WoxAIChatData.empty().obs;
  late final WoxListController<ChatSelectItem> chatSelectListController;

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  // Controllers and focus nodes
  final TextEditingController textController = TextEditingController();
  final WoxLauncherController launcherController = Get.find<WoxLauncherController>();
  final FocusNode aiChatFocusNode = FocusNode();
  final ScrollController aiChatScrollController = ScrollController();
  final RxList<AIModel> aiModels = <AIModel>[].obs;

  // State for chat select panel
  final RxBool isShowChatSelectPanel = false.obs;
  final RxString currentChatSelectCategory = "".obs; // models, tools or empty

  // State for tool usage
  final RxSet<String> selectedTools = <String>{}.obs;
  final RxList<AIMCPTool> availableTools = <AIMCPTool>[].obs;
  final RxBool isLoadingTools = false.obs;

  // State for agents
  final RxList<AIAgent> availableAgents = <AIAgent>[].obs;
  final RxBool isLoadingAgents = false.obs;

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
    );

    reloadChatResources(const UuidV4().generate());
  }

  void reloadChatResources(String traceId, {String resourceName = "all"}) {
    Logger.instance.debug(traceId, "start reloading AI chat resources");
    if (resourceName == "models") {
      reloadAIModels(traceId);
    } else if (resourceName == "tools") {
      fetchAvailableTools(traceId);
    } else if (resourceName == "agents") {
      fetchAvailableAgents(traceId);
    } else if (resourceName == "all") {
      reloadAIModels(traceId);
      fetchAvailableTools(traceId);
      fetchAvailableAgents(traceId);
    }
  }

  // Load available AI models
  void reloadAIModels(String traceId) {
    Logger.instance.debug(traceId, "start reloading ai models");

    WoxApi.instance.findAIModels().then((models) {
      aiModels.assignAll(models);
      Logger.instance.debug(traceId, "reload ai models: ${aiModels.length}");
    });
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
      items.add(WoxListItem<ChatSelectItem>(
        id: "agents",
        icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
        title: "Agent Selection",
        subTitle: "",
        tails: [],
        isGroup: false,
        data: ChatSelectItem(
            id: "agents",
            name: "Agent Selection",
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
            isCategory: true,
            children: [],
            onExecute: (String traceId) {
              currentChatSelectCategory.value = "agents";
              chatSelectListController.clearFilter(traceId);
              updateChatSelectItems();
            }),
      ));

      items.add(WoxListItem<ChatSelectItem>(
        id: "models",
        icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
        title: "Model Selection",
        subTitle: "",
        tails: [],
        isGroup: false,
        data: ChatSelectItem(
            id: "models",
            name: "Model Selection",
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
            isCategory: true,
            children: [],
            onExecute: (String traceId) {
              currentChatSelectCategory.value = "models";
              chatSelectListController.clearFilter(traceId);
              updateChatSelectItems();
            }),
      ));

      items.add(WoxListItem<ChatSelectItem>(
        id: "tools",
        icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🔧"),
        title: "Tool Configuration",
        subTitle: "",
        tails: [],
        isGroup: false,
        data: ChatSelectItem(
            id: "tools",
            name: "Tool Configuration",
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🔧"),
            isCategory: true,
            children: [],
            onExecute: (String traceId) {
              currentChatSelectCategory.value = "tools";
              chatSelectListController.clearFilter(traceId);
              updateChatSelectItems();
            }),
      ));
    } else if (currentChatSelectCategory.value == "models") {
      // Show models grouped by provider
      // Group models by provider
      final modelsByProvider = <String, List<AIModel>>{};
      for (final model in aiModels) {
        modelsByProvider.putIfAbsent(model.provider, () => []).add(model);
      }

      // Sort providers
      final providers = modelsByProvider.keys.toList()..sort();

      // Add groups and models
      for (final provider in providers) {
        // Skip empty groups
        if (modelsByProvider[provider]!.isEmpty) continue;

        // Add provider group header
        items.add(WoxListItem<ChatSelectItem>(
          id: "group_$provider",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🏢"),
          title: provider,
          subTitle: "",
          tails: [],
          isGroup: true,
          data: ChatSelectItem(
            id: "group_$provider",
            name: provider,
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🏢"),
            isCategory: true,
            children: [],
            onExecute: null,
          ),
        ));

        // Sort models within this provider
        final models = modelsByProvider[provider]!;
        models.sort((a, b) => a.name.compareTo(b.name));

        // Add models for this provider
        for (final model in models) {
          items.add(WoxListItem<ChatSelectItem>(
            id: "${model.provider}_${model.name}",
            icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
            title: model.name,
            subTitle: "",
            tails: [],
            isGroup: false,
            data: ChatSelectItem(
                id: "${model.provider}_${model.name}",
                name: model.name,
                icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
                isCategory: false,
                children: [],
                onExecute: (String traceId) {
                  aiChatData.value.model.value = AIModel(name: model.name, provider: model.provider);
                  hideChatSelectPanel();
                }),
          ));
        }
      }
    } else if (currentChatSelectCategory.value == "tools") {
      // Show tools
      for (final tool in availableTools) {
        // Check if this tool is selected to determine if we should show the checkmark
        final bool isSelected = selectedTools.contains(tool.name);

        items.add(WoxListItem<ChatSelectItem>(
          id: tool.name,
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🔧"),
          title: tool.name,
          subTitle: "",
          tails: isSelected ? [WoxListItemTail.image(WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "✅"))] : [],
          isGroup: false,
          data: ChatSelectItem(
              id: tool.name,
              name: tool.name,
              icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🔧"),
              isCategory: false,
              children: [],
              onExecute: (String traceId) {
                if (selectedTools.contains(tool.name)) {
                  selectedTools.remove(tool.name);
                } else {
                  selectedTools.add(tool.name);
                }
                // Update the items to reflect the change in selection status
                updateChatSelectItems();
              }),
        ));
      }
    } else if (currentChatSelectCategory.value == "agents") {
      // Add "Cancel Selection" option at the top
      items.add(WoxListItem<ChatSelectItem>(
        id: "cancel_agent_selection",
        icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "❌"),
        title: tr("ui_ai_chat_cancel_agent_selection"),
        subTitle: tr("ui_ai_chat_use_default_model_and_tools"),
        tails: (aiChatData.value.agentName == null || aiChatData.value.agentName!.isEmpty)
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
            }),
      ));

      // Display all available agents
      for (final agent in availableAgents) {
        final bool isSelected = aiChatData.value.agentName == agent.name;
        items.add(WoxListItem<ChatSelectItem>(
          id: agent.name,
          icon: agent.icon, // 使用agent自定义头像
          title: agent.name,
          subTitle: "Model: ${agent.model.name}",
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
              }),
        ));
      }
    }

    Logger.instance.debug(const UuidV4().generate(), "AI: Updating chat select list with ${items.length} items");
    chatSelectListController.updateItems(const UuidV4().generate(), items);
  }

  // Show chat select panel
  void showChatSelectPanel() {
    Logger.instance.debug(const UuidV4().generate(), "AI: Showing chat select panel");
    isShowChatSelectPanel.value = true;
    currentChatSelectCategory.value = "";
    updateChatSelectItems();
    SchedulerBinding.instance.addPostFrameCallback((_) {
      chatSelectListController.filterBoxFocusNode.requestFocus();
    });
  }

  // Show models panel directly
  void showModelsPanel() {
    showChatSelectPanel();
    currentChatSelectCategory.value = "models";
    updateChatSelectItems();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      chatSelectListController.filterBoxFocusNode.requestFocus();
    });
  }

  // Show tools panel directly
  void showToolsPanel() {
    showChatSelectPanel();
    currentChatSelectCategory.value = "tools";
    updateChatSelectItems();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      chatSelectListController.filterBoxFocusNode.requestFocus();
    });
  }

  // Show agents panel directly
  void showAgentsPanel() {
    showChatSelectPanel();
    currentChatSelectCategory.value = "agents";
    updateChatSelectItems();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      chatSelectListController.filterBoxFocusNode.requestFocus();
    });
  }

  // Hide chat select panel
  void hideChatSelectPanel() {
    isShowChatSelectPanel.value = false;
    chatSelectListController.clearFilter(const UuidV4().generate());
    aiChatFocusNode.requestFocus();
  }

  // Scroll to bottom of AI chat
  void scrollToBottomOfAiChat() {
    if (aiChatScrollController.hasClients) {
      aiChatScrollController.jumpTo(
        aiChatScrollController.position.maxScrollExtent,
      );
    }
  }

  // Focus to chat input
  void focusToChatInput(String traceId) {
    Logger.instance.info(traceId, "focus to chat input");
    SchedulerBinding.instance.addPostFrameCallback((_) {
      aiChatFocusNode.requestFocus();
    });
  }

  // Method to fetch available tools based on the current model
  Future<void> fetchAvailableTools(String traceId) async {
    Logger.instance.info(traceId, "start fetching AI tools");

    if (isLoadingTools.value) return;
    isLoadingTools.value = true;

    try {
      final tools = await WoxApi.instance.findAIMCPServerToolsAll();
      availableTools.assignAll(tools);
      // Default select all tools
      selectedTools.assignAll(tools.map((tool) => tool.name).toSet());

      Logger.instance.debug(const UuidV4().generate(), "AI: loaded ${tools.length} tools");
    } catch (e, s) {
      Logger.instance.error(const UuidV4().generate(), 'Error fetching AI tools: $e $s');
      availableTools.clear();
      selectedTools.clear();
    } finally {
      isLoadingTools.value = false;
    }
  }

  // Method to fetch available agents
  Future<void> fetchAvailableAgents(String traceId) async {
    Logger.instance.info(traceId, "start fetching AI agents");

    if (isLoadingAgents.value) return;
    isLoadingAgents.value = true;

    try {
      final agents = await WoxApi.instance.findAIAgents();
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
      Logger.instance.error(const UuidV4().generate(), 'AI: Error fetching AI agents: $e $s');
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
          Logger.instance.debug(const UuidV4().generate(), "AI: Found agent: ${agent.name}, setting model to ${agent.model.name} and tools to ${agent.tools.length} tools");
          aiChatData.value.model.value = agent.model;
          aiChatData.value.tools = agent.tools;
          agentFound = true;
          break;
        }
      }
      if (!agentFound) {
        Logger.instance.error(const UuidV4().generate(), "AI: Agent with name $agentName not found in available agents");
      }
    } else {
      Logger.instance.debug(const UuidV4().generate(), "AI: No agent selected (empty agentName), setting default model and all tools");

      _setDefaultModel();

      // Select all available tools
      selectedTools.clear();
      selectedTools.addAll(availableTools.map((tool) => tool.name).toSet());
      aiChatData.value.tools = selectedTools.toList();
    }
  }

  Future<void> _setDefaultModel() async {
    var defaultModel = await WoxApi.instance.findDefaultAIModel();
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
    aiChatData.value.conversations.add(WoxAIChatConversation(
      id: const UuidV4().generate(),
      role: WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value,
      text: text,
      images: [],
      timestamp: DateTime.now().millisecondsSinceEpoch,
      toolCallInfo: ToolCallInfo.empty(),
    ));
    aiChatData.value.updatedAt = DateTime.now().millisecondsSinceEpoch;

    textController.clear();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      scrollToBottomOfAiChat();
    });

    aiChatData.value.tools = selectedTools.toList();

    WoxApi.instance.sendChatRequest(aiChatData.value);
  }

  String formatTimestamp(int timestamp) {
    final date = DateTime.fromMillisecondsSinceEpoch(timestamp);
    return '${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
  }

  void handleChatResponse(String traceId, WoxAIChatData data) {
    // Update the chat data with the response
    if (data.id == aiChatData.value.id) {
      aiChatData.value.title = data.title;
      aiChatData.value.conversations.assignAll(data.conversations);
      aiChatData.value.updatedAt = data.updatedAt;

      if (data.agentName != null) {
        aiChatData.value.agentName = data.agentName;
      }

      // if the scrollbar is already at the bottom, scroll to bottom, otherwise, do nothing
      if (aiChatScrollController.hasClients && aiChatScrollController.position.pixels == aiChatScrollController.position.maxScrollExtent) {
        // Scroll to bottom after a short delay to ensure the new message is rendered
        SchedulerBinding.instance.addPostFrameCallback((_) {
          scrollToBottomOfAiChat();
        });
      }
    }
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

    // Send the chat request to regenerate the response
    aiChatData.value.tools = selectedTools.toList();
    WoxApi.instance.sendChatRequest(aiChatData.value);
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

    // Focus on the text input
    SchedulerBinding.instance.addPostFrameCallback((_) {
      aiChatFocusNode.requestFocus();
    });
  }

  @override
  void onClose() {
    textController.dispose();
    chatSelectListController.dispose();
    aiChatFocusNode.dispose();
    aiChatScrollController.dispose();
    super.onClose();
  }
}
