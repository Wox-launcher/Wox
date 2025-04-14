import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_list_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_result_tail_type_enum.dart';
import 'package:wox/utils/log.dart';

class WoxAIChatController extends GetxController {
  final Rx<WoxAIChatData> aiChatData = WoxAIChatData.empty().obs;
  late final WoxListController<ChatSelectItem> chatSelectListController;

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

  // 工具调用展开/折叠状态
  final RxMap<String, bool> toolCallExpandedStates = <String, bool>{}.obs;

  // 切换工具调用展开/折叠状态
  void toggleToolCallExpanded(String conversationId) {
    if (toolCallExpandedStates.containsKey(conversationId)) {
      toolCallExpandedStates[conversationId] = !toolCallExpandedStates[conversationId]!;
    } else {
      toolCallExpandedStates[conversationId] = true;
    }
  }

  // 获取工具调用展开/折叠状态
  bool isToolCallExpanded(String conversationId) {
    return toolCallExpandedStates[conversationId] ?? false;
  }

  WoxAIChatController() {
    chatSelectListController = WoxListController<ChatSelectItem>(
      onItemExecuted: _onChatSelectItemExecuted,
      onFilterBoxEscPressed: (traceId) => hideChatSelectPanel(),
    );

    reloadAIModels();
    fetchAvailableTools();
  }

  // Load available AI models
  void reloadAIModels() {
    WoxApi.instance.findAIModels().then((models) {
      aiModels.assignAll(models);
      Logger.instance.debug(const UuidV4().generate(), "reload ai models: ${aiModels.length}");
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

    if (currentChatSelectCategory.isEmpty) {
      // Show main categories
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
    }

    chatSelectListController.updateItems(const UuidV4().generate(), items);
  }

  // Show chat select panel
  void showChatSelectPanel() {
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

  // Hide chat select panel
  void hideChatSelectPanel() {
    isShowChatSelectPanel.value = false;
    chatSelectListController.clearFilter(const UuidV4().generate());
    aiChatFocusNode.requestFocus();
  }

  // Scroll to bottom of AI chat
  void scrollToBottomOfAiChat() {
    if (aiChatScrollController.hasClients) {
      aiChatScrollController.animateTo(
        aiChatScrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
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
  Future<void> fetchAvailableTools() async {
    if (isLoadingTools.value) return;
    isLoadingTools.value = true;

    try {
      final tools = await WoxApi.instance.findAIMCPServerToolsAll();
      availableTools.assignAll(tools);

      // Default select all tools
      selectedTools.assignAll(tools.map((tool) => tool.name).toSet());
    } catch (e, s) {
      Logger.instance.error(const UuidV4().generate(), 'Error fetching AI tools: $e $s');
      availableTools.clear();
      selectedTools.clear();
    } finally {
      isLoadingTools.value = false;
    }
  }

  void sendMessage() {
    final text = textController.text.trim();
    // Check if AI model is selected
    if (aiChatData.value.model.value.name.isEmpty) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please select a model", displaySeconds: 3));
      return;
    }
    // check if the text is empty
    if (text.isEmpty) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please enter a message", displaySeconds: 3));
      return;
    }

    // append user message to chat data
    aiChatData.value.conversations.add(WoxPreviewChatConversation(
      id: const UuidV4().generate(),
      role: WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value,
      text: text,
      images: [],
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
    aiChatData.value.updatedAt = DateTime.now().millisecondsSinceEpoch;

    textController.clear();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      scrollToBottomOfAiChat();
    });

    aiChatData.value.selectedTools = selectedTools.toList();

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

      // Scroll to bottom after a short delay to ensure the new message is rendered
      SchedulerBinding.instance.addPostFrameCallback((_) {
        scrollToBottomOfAiChat();
      });
    }
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
