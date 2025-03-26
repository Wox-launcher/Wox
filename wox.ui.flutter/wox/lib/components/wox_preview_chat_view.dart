import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';

class WoxPreviewChatView extends StatefulWidget {
  final WoxPreviewChatData chatData;
  final WoxTheme woxTheme;

  const WoxPreviewChatView({super.key, required this.chatData, required this.woxTheme});

  @override
  State<WoxPreviewChatView> createState() => _WoxPreviewChatViewState();
}

class _WoxPreviewChatViewState extends State<WoxPreviewChatView> {
  final ScrollController scrollController = ScrollController();
  final TextEditingController textController = TextEditingController();
  final TextEditingController searchController = TextEditingController();
  final controller = Get.find<WoxLauncherController>();
  List<AIModel> aiModels = [];
  AIModel? selectedModel;
  String searchQuery = '';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      scrollToBottom();
      loadAIModels();
    });
  }

  @override
  void didUpdateWidget(WoxPreviewChatView oldWidget) {
    super.didUpdateWidget(oldWidget);
    // When chat data is updated, scroll to bottom
    if (widget.chatData.conversations.length != oldWidget.chatData.conversations.length) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        scrollToBottom();
      });
    }
  }

  Future<void> loadAIModels() async {
    final models = await WoxApi.instance.findAIModels();
    setState(() {
      aiModels = models;

      // check conversation model
      if (widget.chatData.model.name.isEmpty) {
        selectedModel = models.first;
      } else {
        selectedModel = models.firstWhere((model) => model.name == widget.chatData.model.name && model.provider == widget.chatData.model.provider, orElse: () => models.first);
      }
    });
  }

  @override
  void dispose() {
    scrollController.dispose();
    textController.dispose();
    searchController.dispose();
    super.dispose();
  }

  // Group models by provider
  Map<String, List<AIModel>> getGroupedModels() {
    final Map<String, List<AIModel>> grouped = {};
    for (var model in aiModels) {
      if (searchQuery.isNotEmpty && !model.name.toLowerCase().contains(searchQuery.toLowerCase()) && !model.provider.toLowerCase().contains(searchQuery.toLowerCase())) {
        continue;
      }
      if (!grouped.containsKey(model.provider)) {
        grouped[model.provider] = [];
      }
      grouped[model.provider]!.add(model);
    }
    return grouped;
  }

  void showModelSelectionDialog() {
    showDialog(
      context: context,
      barrierColor: Colors.black87,
      builder: (BuildContext context) {
        return StatefulBuilder(
          builder: (context, setState) {
            return Dialog(
              backgroundColor: fromCssColor(widget.woxTheme.actionContainerBackgroundColor),
              elevation: 0,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
              child: Container(
                width: 400,
                constraints: const BoxConstraints(maxHeight: 500),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Header
                    Container(
                      padding: const EdgeInsets.all(16.0),
                      decoration: BoxDecoration(
                        color: fromCssColor(widget.woxTheme.actionContainerBackgroundColor),
                        borderRadius: const BorderRadius.only(
                          topLeft: Radius.circular(12),
                          topRight: Radius.circular(12),
                        ),
                      ),
                      child: Row(
                        children: [
                          Text(
                            '选择模型',
                            style: TextStyle(
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                              fontSize: 16,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const Spacer(),
                          IconButton(
                            icon: Icon(
                              Icons.close,
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                              size: 20,
                            ),
                            onPressed: () => Navigator.pop(context),
                            padding: EdgeInsets.zero,
                            constraints: const BoxConstraints(
                              minWidth: 32,
                              minHeight: 32,
                            ),
                          ),
                        ],
                      ),
                    ),
                    // Search box
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 16.0),
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 12.0),
                        decoration: BoxDecoration(
                          color: fromCssColor(widget.woxTheme.queryBoxBackgroundColor),
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Row(
                          children: [
                            Icon(
                              Icons.search,
                              size: 18,
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.5),
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: TextField(
                                controller: searchController,
                                style: TextStyle(
                                  color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                                  fontSize: 14,
                                ),
                                decoration: InputDecoration(
                                  hintText: '搜索模型...',
                                  hintStyle: TextStyle(
                                    color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.5),
                                  ),
                                  border: InputBorder.none,
                                  contentPadding: const EdgeInsets.symmetric(vertical: 12),
                                ),
                                onChanged: (value) {
                                  setState(() {
                                    searchQuery = value;
                                  });
                                },
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                    const SizedBox(height: 8),
                    // Model list
                    Flexible(
                      child: Container(
                        color: fromCssColor(widget.woxTheme.actionContainerBackgroundColor),
                        child: SingleChildScrollView(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: getGroupedModels().entries.expand((provider) {
                              final List<Widget> items = [];

                              // Provider header
                              items.add(
                                Padding(
                                  padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
                                  child: Text(
                                    provider.key,
                                    style: TextStyle(
                                      color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.7),
                                      fontSize: 12,
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                ),
                              );

                              // Models under this provider
                              items.addAll(
                                provider.value.map((model) => InkWell(
                                      onTap: () {
                                        // Update both dialog state and parent widget state
                                        setState(() {
                                          selectedModel = model;
                                        });
                                        this.setState(() {
                                          selectedModel = model;
                                        });
                                        // Update chat data model
                                        widget.chatData.model = WoxPreviewChatModel(
                                          name: model.name,
                                          provider: model.provider,
                                        );
                                        Navigator.pop(context);
                                      },
                                      child: Container(
                                        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                                        margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                                        decoration: BoxDecoration(
                                          color: selectedModel?.name == model.name ? fromCssColor(widget.woxTheme.actionItemActiveBackgroundColor) : Colors.transparent,
                                          borderRadius: BorderRadius.circular(6),
                                        ),
                                        child: Row(
                                          children: [
                                            Expanded(
                                              child: Text(
                                                model.name,
                                                style: TextStyle(
                                                  color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                                                  fontSize: 14,
                                                ),
                                              ),
                                            ),
                                            if (selectedModel?.name == model.name)
                                              Icon(
                                                Icons.check,
                                                size: 16,
                                                color: fromCssColor(widget.woxTheme.actionItemActiveFontColor),
                                              ),
                                          ],
                                        ),
                                      ),
                                    )),
                              );

                              return items;
                            }).toList(),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // AI Model Selection
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 8.0),
          child: InkWell(
            onTap: showModelSelectionDialog,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12.0, vertical: 8.0),
              decoration: BoxDecoration(
                color: fromCssColor(widget.woxTheme.queryBoxBackgroundColor),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.1),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.smart_toy_outlined,
                    size: 20,
                    color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      selectedModel?.name ?? '选择模型',
                      style: TextStyle(
                        color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                        fontSize: 14,
                      ),
                    ),
                  ),
                  Icon(
                    Icons.arrow_forward_ios,
                    size: 16,
                    color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                  ),
                ],
              ),
            ),
          ),
        ),
        // Messages list
        Expanded(
          child: ListView.builder(
            controller: scrollController,
            padding: const EdgeInsets.symmetric(vertical: 16.0),
            itemCount: widget.chatData.conversations.length,
            itemBuilder: (context, index) {
              final message = widget.chatData.conversations[index];
              return buildMessageItem(message);
            },
          ),
        ),
        // Input box
        Focus(
          onKeyEvent: (FocusNode node, KeyEvent event) {
            if (event is KeyDownEvent) {
              switch (event.logicalKey) {
                case LogicalKeyboardKey.escape:
                  controller.queryBoxFocusNode.requestFocus();
                  return KeyEventResult.handled;
                case LogicalKeyboardKey.enter:
                  sendMessage();
                  return KeyEventResult.handled;
              }
            }
            return KeyEventResult.ignored;
          },
          child: Container(
            padding: const EdgeInsets.all(12.0),
            child: Column(
              children: [
                Container(
                  decoration: BoxDecoration(
                    color: fromCssColor(widget.woxTheme.queryBoxBackgroundColor),
                    borderRadius: BorderRadius.circular(widget.woxTheme.queryBoxBorderRadius.toDouble()),
                  ),
                  child: Column(
                    children: [
                      TextField(
                        controller: textController,
                        focusNode: controller.aiChatFocusNode,
                        decoration: InputDecoration(
                          hintText: '在这里输入消息，按下 ← 发送',
                          hintStyle: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor)),
                          border: InputBorder.none,
                          contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                        ),
                        maxLines: null,
                        keyboardType: TextInputType.multiline,
                        style: TextStyle(
                          fontSize: 14,
                          color: fromCssColor(widget.woxTheme.queryBoxFontColor),
                        ),
                      ),
                      Container(
                        height: 36,
                        padding: const EdgeInsets.symmetric(horizontal: 8),
                        decoration: BoxDecoration(
                          border: Border(
                            top: BorderSide(
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.1),
                            ),
                          ),
                        ),
                        child: Row(
                          children: [
                            IconButton(
                              icon: const Icon(Icons.link, size: 18),
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                              onPressed: () {},
                              padding: EdgeInsets.zero,
                              constraints: const BoxConstraints(
                                minWidth: 32,
                                minHeight: 32,
                              ),
                            ),
                            IconButton(
                              icon: const Icon(Icons.keyboard_command_key, size: 18),
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                              onPressed: () {},
                              padding: EdgeInsets.zero,
                              constraints: const BoxConstraints(
                                minWidth: 32,
                                minHeight: 32,
                              ),
                            ),
                            IconButton(
                              icon: const Icon(Icons.eco_outlined, size: 18),
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                              onPressed: () {},
                              padding: EdgeInsets.zero,
                              constraints: const BoxConstraints(
                                minWidth: 32,
                                minHeight: 32,
                              ),
                            ),
                            const Spacer(),
                            Container(
                              padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                              decoration: BoxDecoration(
                                color: fromCssColor(widget.woxTheme.actionItemActiveBackgroundColor).withOpacity(0.1),
                                borderRadius: BorderRadius.circular(4),
                              ),
                              child: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Icon(
                                    Icons.keyboard_return,
                                    size: 14,
                                    color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                                  ),
                                  const SizedBox(width: 4),
                                  Text(
                                    '发送',
                                    style: TextStyle(
                                      fontSize: 12,
                                      color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  void sendMessage() {
    final text = textController.text.trim();
    // Check if AI model is selected
    if (selectedModel == null) {
      controller.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please select a model", displaySeconds: 3));
      return;
    }
    // check if the text is empty
    if (text.isEmpty) {
      controller.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please enter a message", displaySeconds: 3));
      return;
    }

    textController.clear();

    // append user message to chat data
    widget.chatData.conversations.add(WoxPreviewChatConversation(
      id: const UuidV4().generate(),
      role: WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value,
      text: text,
      images: [],
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
    widget.chatData.updatedAt = DateTime.now().millisecondsSinceEpoch;
    widget.chatData.model = WoxPreviewChatModel(
      name: selectedModel!.name,
      provider: selectedModel!.provider,
    );

    setState(() {});

    // Scroll to bottom after sending message
    WidgetsBinding.instance.addPostFrameCallback((_) {
      scrollToBottom();
    });

    controller.sendChatRequest(const UuidV4().generate(), widget.chatData);
  }

  void scrollToBottom() {
    if (scrollController.hasClients) {
      scrollController.animateTo(
        scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 300),
        curve: Curves.easeOut,
      );
    }
  }

  Widget buildMessageItem(WoxPreviewChatConversation message) {
    final isUser = message.role == 'user';
    final backgroundColor = isUser ? fromCssColor(widget.woxTheme.resultItemActiveBackgroundColor) : fromCssColor(widget.woxTheme.actionContainerBackgroundColor);
    final alignment = isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start;

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 4.0),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        children: [
          if (!isUser) buildAvatar(message),
          const SizedBox(width: 8),
          Flexible(
            child: Column(
              crossAxisAlignment: alignment,
              children: [
                Container(
                  constraints: BoxConstraints(
                    maxWidth: MediaQuery.of(context).size.width * 0.7,
                  ),
                  margin: const EdgeInsets.only(bottom: 4),
                  padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 10.0),
                  decoration: BoxDecoration(
                    color: backgroundColor,
                    borderRadius: BorderRadius.only(
                      topLeft: const Radius.circular(16),
                      topRight: const Radius.circular(16),
                      bottomLeft: Radius.circular(isUser ? 16 : 4),
                      bottomRight: Radius.circular(isUser ? 4 : 16),
                    ),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withOpacity(0.05),
                        blurRadius: 5,
                        offset: const Offset(0, 2),
                      ),
                    ],
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Text content
                      MarkdownBody(
                        data: message.text,
                        selectable: true,
                        styleSheet: MarkdownStyleSheet(
                          p: TextStyle(
                            color: fromCssColor(isUser ? widget.woxTheme.resultItemActiveTitleColor : widget.woxTheme.resultItemTitleColor),
                            fontSize: 14,
                          ),
                        ),
                      ),
                      // Images if any
                      if (message.images.isNotEmpty) ...[
                        const SizedBox(height: 8),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: message.images
                              .map((image) => ClipRRect(
                                    borderRadius: BorderRadius.circular(8),
                                    child: SizedBox(
                                      width: 200,
                                      child: WoxImageView(woxImage: image),
                                    ),
                                  ))
                              .toList(),
                        ),
                      ],
                    ],
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.only(left: 4, right: 4),
                  child: Text(
                    formatTimestamp(message.timestamp),
                    style: TextStyle(
                      fontSize: 11,
                      color: fromCssColor(widget.woxTheme.resultItemSubTitleColor),
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (isUser) buildAvatar(message),
        ],
      ),
    );
  }

  Widget buildAvatar(WoxPreviewChatConversation message) {
    final isUser = message.role == 'user';
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: fromCssColor(isUser ? widget.woxTheme.actionItemActiveBackgroundColor : widget.woxTheme.resultItemActiveBackgroundColor),
        shape: BoxShape.circle,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.1),
            blurRadius: 4,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Center(
        child: Text(
          isUser ? 'U' : 'A',
          style: TextStyle(
            color: fromCssColor(isUser ? widget.woxTheme.actionItemActiveFontColor : widget.woxTheme.resultItemActiveTitleColor),
            fontSize: 16,
            fontWeight: FontWeight.w500,
          ),
        ),
      ),
    );
  }

  String formatTimestamp(int timestamp) {
    final date = DateTime.fromMillisecondsSinceEpoch(timestamp);
    return '${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
  }
}
