import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
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
  final controller = Get.find<WoxLauncherController>();

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (scrollController.hasClients) {
        scrollController.animateTo(
          scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }

  @override
  void dispose() {
    scrollController.dispose();
    textController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
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
          onFocusChange: (bool hasFocus) {
            // when focused, set the preview ratio to 0.3 for better user experience
            if (!hasFocus) {
              controller.resultPreviewRatio.value = 0.5;
            } else {
              controller.resultPreviewRatio.value = 0.3;
            }
          },
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
    if (text.isEmpty) return;

    textController.clear();

    // append user message to chat data
    widget.chatData.conversations.add(WoxPreviewChatConversation(role: 'user', text: text, images: [], timestamp: DateTime.now().millisecondsSinceEpoch));
    widget.chatData.updatedAt = DateTime.now().millisecondsSinceEpoch;

    setState(() {});

    // send message to ai
    // controller.sendMessageToAi(widget.chatData);
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
