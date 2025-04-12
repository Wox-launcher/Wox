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
import 'package:wox/utils/log.dart';

class ChatSelectItem {
  final String id;
  final String name;
  final WoxImage icon;
  final bool isCategory;
  final List<ChatSelectItem> children;
  final Function(String traceId)? onExecute;

  ChatSelectItem({
    required this.id,
    required this.name,
    required this.icon,
    required this.isCategory,
    required this.children,
    this.onExecute,
  });
}

class WoxAIChatController extends GetxController {
  final WoxAIChatData aiChatData = WoxAIChatData.empty();
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

  WoxAIChatController() {
    // Initialize chat select list controller
    chatSelectListController = WoxListController<ChatSelectItem>(
      onItemExecuted: _onChatSelectItemExecuted,
      onFilterBoxEscPressed: (traceId) => hideChatSelectPanel(),
    );

    // Load AI models
    reloadAIModels();

    // Fetch tools if a model is selected initially
    if (aiChatData.model.name.isNotEmpty) {
      // Don't await here, let it load in background
      fetchAvailableTools();
    }
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
                  aiChatData.model = WoxPreviewChatModel(name: model.name, provider: model.provider);
                  hideChatSelectPanel();
                }),
          ));
        }
      }
    } else if (currentChatSelectCategory.value == "tools") {
      // Show tools
      for (final tool in availableTools) {
        items.add(WoxListItem<ChatSelectItem>(
          id: tool.name,
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🔧"),
          title: tool.name,
          subTitle: "",
          tails: [],
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

  // Send chat request to backend
  Future<void> sendChatRequest(String traceId, WoxAIChatData data) async {
    WoxApi.instance.sendChatRequest(data);
  }

  // Method to fetch available tools based on the current model
  Future<void> fetchAvailableTools() async {
    // Prevent concurrent fetches
    if (isLoadingTools.value) return;

    if (aiChatData.model.name.isEmpty) {
      availableTools.clear();
      selectedTools.clear();
      isLoadingTools.value = false;
      return;
    }

    isLoadingTools.value = true;

    try {
      // 使用findAIMCPServerToolsAll获取所有工具
      final tools = await WoxApi.instance.findAIMCPServerToolsAll({});
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

  // Handle chat select panel keyboard navigation
  KeyEventResult handleChatSelectKeyboard(KeyEvent event) {
    // 只处理特定的键盘事件，让其他键盘输入正常工作
    if (event is KeyDownEvent) {
      switch (event.logicalKey) {
        case LogicalKeyboardKey.escape:
          if (currentChatSelectCategory.isNotEmpty) {
            // Go back to main categories
            currentChatSelectCategory.value = "";
            updateChatSelectItems();

            // 返回主类别时，确保焦点在过滤器文本框上
            SchedulerBinding.instance.addPostFrameCallback((_) {
              chatSelectListController.filterBoxFocusNode.requestFocus();
            });
          } else {
            // Close panel
            hideChatSelectPanel();
          }
          return KeyEventResult.handled;
        default:
          // 对于其他键，让 WoxListController 处理
          return KeyEventResult.ignored;
      }
    }
    return KeyEventResult.ignored;
  }

  // Send message
  void sendMessage() {
    final text = textController.text.trim();
    // Check if AI model is selected
    if (aiChatData.model.name.isEmpty) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please select a model", displaySeconds: 3));
      return;
    }
    // check if the text is empty
    if (text.isEmpty) {
      launcherController.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please enter a message", displaySeconds: 3));
      return;
    }

    // append user message to chat data
    aiChatData.conversations.add(WoxPreviewChatConversation(
      id: const UuidV4().generate(),
      role: WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value,
      text: text,
      images: [], // TODO: Support images if needed
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
    aiChatData.updatedAt = DateTime.now().millisecondsSinceEpoch;

    textController.clear();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      scrollToBottomOfAiChat();
    });

    aiChatData.selectedTools = selectedTools.toList();
    sendChatRequest(
      const UuidV4().generate(),
      aiChatData,
    );

    // Update toolbar
    updateToolbarByChat(const UuidV4().generate());
  }

  String formatTimestamp(int timestamp) {
    final date = DateTime.fromMillisecondsSinceEpoch(timestamp);
    return '${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
  }

  // Handle chat response from backend
  void handleChatResponse(String traceId, WoxAIChatData data) {
    // Update the chat data with the response
    if (data.id == aiChatData.id) {
      aiChatData.conversations.assignAll(data.conversations);
      aiChatData.updatedAt = data.updatedAt;

      // Scroll to bottom after a short delay to ensure the new message is rendered
      SchedulerBinding.instance.addPostFrameCallback((_) {
        scrollToBottomOfAiChat();
      });
    }
  }

  // Update the toolbar to chat view
  void updateToolbarByChat(String traceId) {
    Logger.instance.debug(traceId, "update toolbar to chat");
    launcherController.toolbar.value = ToolbarInfo(
      hotkey: "cmd+j",
      actionName: "Select models",
      action: () {
        showModelsPanel();
      },
    );
  }

  @override
  void onClose() {
    // Dispose controllers and focus nodes
    textController.dispose();
    chatSelectListController.dispose();
    aiChatFocusNode.dispose();
    aiChatScrollController.dispose();
    super.onClose();
  }
}
