import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxAIChatView extends GetView<WoxAIChatController> {
  const WoxAIChatView({super.key});

  WoxTheme get woxTheme => WoxThemeUtil.instance.currentTheme.value;

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: chat view");

    return Stack(
      children: [
        Column(
          children: [
            // AI Model Selection
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 8.0),
              child: InkWell(
                onTap: () {
                  controller.showModelsPanel();
                },
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12.0, vertical: 8.0),
                  decoration: BoxDecoration(
                    color: fromCssColor(woxTheme.queryBoxBackgroundColor),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: fromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25),
                    ),
                  ),
                  child: Row(
                    children: [
                      Icon(
                        Icons.smart_toy_outlined,
                        size: 20,
                        color: fromCssColor(woxTheme.previewPropertyTitleColor),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Obx(() => Text(
                              controller.aiChatData.value.model.value.name.isEmpty ? "请选择模型" : controller.aiChatData.value.model.value.name,
                              style: TextStyle(
                                color: fromCssColor(woxTheme.previewPropertyTitleColor),
                                fontSize: 14,
                              ),
                            )),
                      ),
                      Icon(
                        Icons.arrow_forward_ios,
                        size: 16,
                        color: fromCssColor(woxTheme.previewPropertyTitleColor),
                      ),
                    ],
                  ),
                ),
              ),
            ),
            // Messages list
            Expanded(
              child: SingleChildScrollView(
                controller: controller.aiChatScrollController,
                padding: const EdgeInsets.symmetric(vertical: 16.0),
                child: Obx(() => Column(
                      children: controller.aiChatData.value.conversations.map((message) => _buildMessageItem(message)).toList(),
                    )),
              ),
            ),
            // Input box and controls area
            Focus(
              onKeyEvent: (FocusNode node, KeyEvent event) {
                if (event is KeyDownEvent) {
                  switch (event.logicalKey) {
                    case LogicalKeyboardKey.escape:
                      controller.launcherController.focusQueryBox();
                      return KeyEventResult.handled;
                    case LogicalKeyboardKey.enter:
                      controller.sendMessage();
                      return KeyEventResult.handled;
                  }
                }

                var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
                if (pressedHotkey == null) {
                  return KeyEventResult.ignored;
                }

                // Show chat select panel on Cmd+J
                if (controller.launcherController.isActionHotkey(pressedHotkey)) {
                  controller.showChatSelectPanel();
                  return KeyEventResult.handled;
                }

                return KeyEventResult.ignored;
              },
              // Wrap the input area content in a Column to place the expandable section above
              child: Padding(
                padding: const EdgeInsets.all(12.0),
                child: Column(
                  // New outer Column
                  mainAxisSize: MainAxisSize.min, // Important for Column height
                  children: [
                    const SizedBox.shrink(),
                    Container(
                      decoration: BoxDecoration(
                        color: fromCssColor(woxTheme.queryBoxBackgroundColor),
                        borderRadius: BorderRadius.circular(woxTheme.queryBoxBorderRadius.toDouble()),
                        border: Border.all(
                          color: fromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25),
                        ),
                      ),
                      child: Column(
                        children: [
                          TextField(
                            controller: controller.textController,
                            focusNode: controller.aiChatFocusNode,
                            decoration: InputDecoration(
                              hintText: '在这里输入消息，按下 ← 发送',
                              hintStyle: TextStyle(color: fromCssColor(woxTheme.previewPropertyTitleColor)),
                              border: InputBorder.none,
                              contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                            ),
                            maxLines: null,
                            keyboardType: TextInputType.multiline,
                            cursorColor: fromCssColor(woxTheme.queryBoxCursorColor),
                            style: TextStyle(
                              fontSize: 14,
                              color: fromCssColor(woxTheme.queryBoxFontColor),
                            ),
                          ),
                          // Input Box Toolbar (Send button, Tool icon)
                          Container(
                            height: 36,
                            padding: const EdgeInsets.symmetric(horizontal: 8),
                            decoration: BoxDecoration(
                              border: Border(
                                top: BorderSide(
                                  color: fromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25),
                                ),
                              ),
                            ),
                            child: Row(
                              children: [
                                // Tool configuration button - opens chat select panel
                                Obx(() => IconButton(
                                      tooltip: 'Configure Tool Usage',
                                      icon: Icon(Icons.build,
                                          size: 18,
                                          color: controller.selectedTools.isNotEmpty
                                              ? fromCssColor(woxTheme.actionItemActiveFontColor)
                                              : fromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(128)),
                                      color: fromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      onPressed: () {
                                        controller.showToolsPanel();
                                      },
                                      padding: EdgeInsets.zero,
                                      constraints: const BoxConstraints(
                                        minWidth: 32,
                                        minHeight: 32,
                                      ),
                                    )),
                                const Spacer(),
                                // Send button container (unchanged)
                                InkWell(
                                  onTap: () => controller.sendMessage(),
                                  child: Container(
                                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                                    decoration: BoxDecoration(
                                      color: fromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      borderRadius: BorderRadius.circular(4),
                                    ),
                                    child: Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Icon(
                                          Icons.keyboard_return,
                                          size: 14,
                                          color: fromCssColor(woxTheme.actionItemActiveFontColor),
                                        ),
                                        const SizedBox(width: 4),
                                        Text(
                                          '发送',
                                          style: TextStyle(
                                            fontSize: 12,
                                            color: fromCssColor(woxTheme.actionItemActiveFontColor),
                                          ),
                                        ),
                                      ],
                                    ),
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
        ),
        Obx(() => controller.isShowChatSelectPanel.value ? _buildChatSelectPanel(context) : const SizedBox.shrink()),
      ],
    );
  }

  Widget _buildChatSelectPanel(BuildContext context) {
    return Positioned(
      right: 10,
      bottom: 10,
      child: Material(
        elevation: 8,
        borderRadius: BorderRadius.circular(woxTheme.actionQueryBoxBorderRadius.toDouble()),
        child: Container(
          padding: EdgeInsets.only(
            top: woxTheme.actionContainerPaddingTop.toDouble(),
            bottom: woxTheme.actionContainerPaddingBottom.toDouble(),
            left: woxTheme.actionContainerPaddingLeft.toDouble(),
            right: woxTheme.actionContainerPaddingRight.toDouble(),
          ),
          decoration: BoxDecoration(
            color: fromCssColor(woxTheme.actionContainerBackgroundColor),
            borderRadius: BorderRadius.circular(woxTheme.actionQueryBoxBorderRadius.toDouble()),
          ),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 320),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              mainAxisAlignment: MainAxisAlignment.start,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Obx(() => Text(
                      controller.currentChatSelectCategory.isEmpty ? "Chat Options" : (controller.currentChatSelectCategory.value == "models" ? "Select Model" : "Configure Tools"),
                      style: TextStyle(color: fromCssColor(woxTheme.actionContainerHeaderFontColor), fontSize: 16.0),
                    )),
                const Divider(),
                WoxListView<ChatSelectItem>(
                  controller: controller.chatSelectListController,
                  listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_CHAT.code,
                  showFilter: true,
                  maxHeight: 350,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildMessageItem(WoxAIChatConversation message) {
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;
    final isTool = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_TOOL.value;

    Color backgroundColor;
    if (isUser) {
      backgroundColor = fromCssColor(woxTheme.actionItemActiveBackgroundColor);
    } else if (isTool) {
      backgroundColor = fromCssColor(woxTheme.actionContainerBackgroundColor);
    } else {
      backgroundColor = fromCssColor(woxTheme.actionContainerBackgroundColor);
    }
    final alignment = isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start;

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 4.0),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        children: [
          if (!isUser) _buildAvatar(message),
          const SizedBox(width: 8),
          Flexible(
            child: Column(
              crossAxisAlignment: alignment,
              children: [
                Container(
                  margin: const EdgeInsets.only(bottom: 4),
                  padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 10.0),
                  decoration: BoxDecoration(
                    color: backgroundColor,
                    borderRadius: BorderRadius.circular(8),
                    boxShadow: [
                      BoxShadow(
                        color: Colors.black.withAlpha(13),
                        blurRadius: 5,
                        offset: const Offset(0, 2),
                      ),
                    ],
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (isTool && message.toolCallInfo.id.isNotEmpty) _buildToolCallBadge(message),
                      if (!isTool)
                        MarkdownBody(
                          data: message.text,
                          selectable: true,
                          styleSheet: MarkdownStyleSheet(
                            a: TextStyle(
                              color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemActiveTitleColor),
                              fontSize: 14,
                              decoration: TextDecoration.underline,
                              decorationColor: fromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemActiveTitleColor),
                            ),
                            p: TextStyle(
                              color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemActiveTitleColor),
                              fontSize: 14,
                            ),
                          ),
                        ),
                      if (message.images.isNotEmpty) ...[
                        const SizedBox(height: 8),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: message.images
                              .map((image) => ClipRRect(
                                    borderRadius: BorderRadius.circular(8),
                                    child: SizedBox(
                                      width: 200, // Consider making this adaptive
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
                    controller.formatTimestamp(message.timestamp),
                    style: TextStyle(
                      fontSize: 11,
                      color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (isUser) _buildAvatar(message),
        ],
      ),
    );
  }

  Widget _buildToolCallBadge(WoxAIChatConversation message) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          onTap: () {
            controller.toggleToolCallExpanded(message.id);
          },
          child: Container(
            width: double.infinity,
            decoration: BoxDecoration(
              color: fromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(25),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(
                color: fromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(75),
                width: 1.0,
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.max,
              children: [
                Icon(
                  Icons.build,
                  size: 14,
                  color: fromCssColor(woxTheme.resultItemActiveTitleColor),
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    message.toolCallInfo.name,
                    style: TextStyle(
                      fontSize: 12,
                      color: fromCssColor(woxTheme.resultItemActiveTitleColor),
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ),
                const SizedBox(width: 6),
                Text(
                  '${message.toolCallInfo.duration}ms',
                  style: TextStyle(
                    fontSize: 12,
                    color: fromCssColor(woxTheme.resultItemActiveTitleColor),
                  ),
                ),
                const SizedBox(width: 6),
                _buildStatusIndicator(message.toolCallInfo),
                const SizedBox(width: 6),
                Obx(() => Icon(
                      controller.isToolCallExpanded(message.id) ? Icons.keyboard_arrow_up : Icons.keyboard_arrow_down,
                      size: 14,
                      color: fromCssColor(woxTheme.resultItemActiveTitleColor),
                    )),
              ],
            ),
          ),
        ),
        Obx(
          () => controller.isToolCallExpanded(message.id) ? _buildToolCallDetails(message.toolCallInfo) : const SizedBox.shrink(),
        ),
      ],
    );
  }

  Widget _buildStatusIndicator(ToolCallInfo info) {
    IconData icon;
    Color color;
    String tooltip;

    switch (info.status) {
      case ToolCallStatus.streaming:
        icon = Icons.play_arrow;
        color = Colors.blue;
        tooltip = '正在调用';
        break;
      case ToolCallStatus.pending:
        icon = Icons.hourglass_empty;
        color = Colors.grey;
        tooltip = '等待执行';
        break;
      case ToolCallStatus.running:
        icon = Icons.refresh;
        color = Colors.blue;
        tooltip = '正在执行';
        break;
      case ToolCallStatus.succeeded:
        icon = Icons.check_circle;
        color = Colors.green;
        tooltip = '执行成功';
        break;
      case ToolCallStatus.failed:
        icon = Icons.error;
        color = Colors.red;
        tooltip = '执行失败: ${info.response}';
        break;
    }

    return Tooltip(
      message: tooltip,
      child: Icon(
        icon,
        size: 14,
        color: color,
      ),
    );
  }

  Widget _buildToolCallDetails(ToolCallInfo info) {
    return Container(
      width: double.infinity,
      margin: const EdgeInsets.only(top: 8.0),
      padding: const EdgeInsets.all(8.0),
      decoration: BoxDecoration(
        color: fromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(15),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: fromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(40),
          width: 1.0,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildDetailItem('Id', info.id),
          _buildDetailItem('名称', info.name),
          _buildDetailItem('参数', info.status == ToolCallStatus.streaming ? info.delta : info.arguments.toString()),
          _buildDetailItem('耗时', '${info.duration}ms'),
          if (info.response.isNotEmpty) _buildDetailItem('响应', info.response),
        ],
      ),
    );
  }

  Widget _buildDetailItem(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.bold,
              color: fromCssColor(woxTheme.resultItemSubTitleColor),
            ),
          ),
          const SizedBox(height: 4),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(6.0),
            decoration: BoxDecoration(
              color: Colors.black.withAlpha(20),
              border: Border.all(
                color: Colors.black.withAlpha(10),
                width: 1.0,
              ),
            ),
            child: SelectableText(
              value,
              style: TextStyle(
                fontSize: 12,
                fontFamily: 'monospace',
                color: fromCssColor(woxTheme.resultItemTitleColor),
              ),
            ),
          )
        ],
      ),
    );
  }

  Widget _buildAvatar(WoxAIChatConversation message) {
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;

    // 根据角色使用不同的背景色
    Color avatarColor = isUser ? fromCssColor(woxTheme.actionItemActiveBackgroundColor) : fromCssColor(woxTheme.actionContainerBackgroundColor);

    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: avatarColor,
        shape: BoxShape.circle,
        boxShadow: [
          BoxShadow(
            color: Colors.black.withAlpha(25),
            blurRadius: 4,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Center(
        child: isUser
            ? Icon(
                Icons.person,
                size: 20,
                color: fromCssColor(woxTheme.actionItemActiveFontColor),
              )
            : Icon(
                Icons.smart_toy_outlined,
                size: 20,
                color: fromCssColor(woxTheme.resultItemActiveTitleColor),
              ),
      ),
    );
  }
}
