import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_chat_toolcall_duration.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/strings.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxAIChatView extends GetView<WoxAIChatController> {
  const WoxAIChatView({super.key});

  WoxTheme get woxTheme => WoxThemeUtil.instance.currentTheme.value;

  // Get translation from WoxSettingController
  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: chat view");

    return Stack(
      children: [
        Column(
          children: [
            // AI Model & Agent Info Display
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 8.0),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 12.0, vertical: 8.0),
                decoration: BoxDecoration(
                  color: safeFromCssColor(woxTheme.queryBoxBackgroundColor),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25),
                  ),
                ),
                child: Obx(() => Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        // Sidebar toggle button
                        Obx(() => IconButton(
                              tooltip: controller.isLeftPanelCollapsed.value ? tr('ui_ai_chat_show_sidebar') : tr('ui_ai_chat_hide_sidebar'),
                              icon: Icon(
                                controller.isLeftPanelCollapsed.value ? Icons.last_page : Icons.first_page, // Or any other suitable icon
                                size: 20,
                                color: safeFromCssColor(woxTheme.previewPropertyTitleColor),
                              ),
                              padding: EdgeInsets.zero,
                              constraints: const BoxConstraints(
                                minWidth: 32,
                                minHeight: 32,
                              ),
                              onPressed: () {
                                controller.toggleLeftPanel();
                              },
                            )),

                        Expanded(
                            child: Center(
                          child: Text(
                            controller.aiChatData.value.title.isEmpty ? "New Chat" : controller.aiChatData.value.title,
                            style: TextStyle(
                              color: safeFromCssColor(woxTheme.previewPropertyTitleColor),
                              fontSize: 14,
                              fontWeight: FontWeight.bold,
                            ),
                            overflow: TextOverflow.ellipsis,
                          ),
                        )),

                        // 占位
                        const SizedBox(width: 32),
                      ],
                    )),
              ),
            ),
            // Messages list
            Expanded(
              child: SingleChildScrollView(
                controller: controller.aiChatScrollController,
                padding: const EdgeInsets.symmetric(vertical: 16.0),
                child: Obx(() => Column(
                      children: controller.aiChatData.value.conversations.map((message) => _buildMessageItem(message, context)).toList(),
                    )),
              ),
            ),
            // Input box and controls area
            WoxPlatformFocus(
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
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const SizedBox.shrink(),
                    Container(
                      decoration: BoxDecoration(
                        color: safeFromCssColor(woxTheme.queryBoxBackgroundColor),
                        borderRadius: BorderRadius.circular(woxTheme.queryBoxBorderRadius.toDouble()),
                        border: Border.all(
                          color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25),
                        ),
                      ),
                      child: Column(
                        children: [
                          TextField(
                            controller: controller.textController,
                            focusNode: controller.aiChatFocusNode,
                            decoration: InputDecoration(
                              hintText: tr('ui_ai_chat_input_hint'),
                              hintStyle: TextStyle(color: safeFromCssColor(woxTheme.previewPropertyTitleColor)),
                              border: InputBorder.none,
                              contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                            ),
                            maxLines: null,
                            keyboardType: TextInputType.multiline,
                            cursorColor: safeFromCssColor(woxTheme.queryBoxCursorColor),
                            style: TextStyle(
                              fontSize: 14,
                              color: safeFromCssColor(woxTheme.queryBoxFontColor),
                            ),
                          ),
                          // Input Box Toolbar (Send button, Tool icon)
                          Container(
                            height: 36,
                            padding: const EdgeInsets.symmetric(horizontal: 8),
                            decoration: BoxDecoration(
                              border: Border(
                                top: BorderSide(
                                  color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25),
                                ),
                              ),
                            ),
                            child: Row(
                              children: [
                                // Tool configuration button - opens chat select panel
                                Obx(() => IconButton(
                                      tooltip: tr('ui_ai_chat_configure_tools'),
                                      icon: Icon(Icons.build, size: 18, color: controller.selectedTools.isNotEmpty ? getThemeTextColor() : getThemeTextColor().withAlpha(128)),
                                      color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      onPressed: () {
                                        controller.showToolsPanel();
                                      },
                                      padding: EdgeInsets.zero,
                                      constraints: const BoxConstraints(
                                        minWidth: 32,
                                        minHeight: 32,
                                      ),
                                    )),
                                // Agent selection button
                                Obx(() => IconButton(
                                      tooltip: tr('ui_ai_chat_select_agent'),
                                      icon: Icon(Icons.smart_toy,
                                          size: 18,
                                          color: (controller.aiChatData.value.agentName != null && controller.aiChatData.value.agentName!.isNotEmpty)
                                              ? getThemeTextColor()
                                              : getThemeTextColor().withAlpha(128)),
                                      color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      onPressed: () {
                                        controller.showAgentsPanel();
                                      },
                                      padding: EdgeInsets.zero,
                                      constraints: const BoxConstraints(
                                        minWidth: 32,
                                        minHeight: 32,
                                      ),
                                    )),
                                // Model selection button
                                Obx(() => IconButton(
                                      tooltip: tr('ui_ai_chat_select_model_title'),
                                      icon: Icon(
                                        Icons.model_training,
                                        size: 18,
                                        color: controller.aiChatData.value.model.value.name.isNotEmpty ? getThemeTextColor() : getThemeTextColor().withAlpha(128),
                                      ),
                                      color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      onPressed: () {
                                        controller.showModelsPanel();
                                      },
                                      padding: EdgeInsets.zero,
                                      constraints: const BoxConstraints(
                                        minWidth: 32,
                                        minHeight: 32,
                                      ),
                                    )),
                                // Model Name Display
                                Obx(() => Text(
                                      controller.aiChatData.value.model.value.name.isEmpty ? tr("ui_ai_chat_select_model") : controller.aiChatData.value.model.value.name,
                                      style: TextStyle(
                                        color: getThemeTextColor(),
                                        fontSize: 12,
                                      ),
                                    )),

                                const Spacer(),
                                // Send button container (unchanged)
                                InkWell(
                                  onTap: () => controller.sendMessage(),
                                  child: Container(
                                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                                    decoration: BoxDecoration(
                                      color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      borderRadius: BorderRadius.circular(4),
                                    ),
                                    child: Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Icon(
                                          Icons.keyboard_return,
                                          size: 14,
                                          color: safeFromCssColor(woxTheme.actionItemActiveFontColor),
                                        ),
                                        const SizedBox(width: 4),
                                        Text(
                                          tr('ui_ai_chat_send'),
                                          style: TextStyle(
                                            fontSize: 12,
                                            color: safeFromCssColor(woxTheme.actionItemActiveFontColor),
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
            color: safeFromCssColor(woxTheme.actionContainerBackgroundColor),
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
                      controller.currentChatSelectCategory.isEmpty
                          ? tr("ui_ai_chat_options")
                          : (controller.currentChatSelectCategory.value == "models"
                              ? tr("ui_ai_chat_select_model_title")
                              : (controller.currentChatSelectCategory.value == "tools" ? tr("ui_ai_chat_configure_tools_title") : tr("ui_ai_chat_select_agent_title"))),
                      style: TextStyle(color: safeFromCssColor(woxTheme.actionContainerHeaderFontColor), fontSize: 16.0),
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

  Widget _buildMessageItem(WoxAIChatConversation message, BuildContext context) {
    final isSystem = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_SYSTEM.value;
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;
    final isTool = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_TOOL.value;

    if (isSystem) {
      return const SizedBox.shrink();
    }

    Color backgroundColor;
    Color fontColor;
    if (isUser) {
      backgroundColor = safeFromCssColor(woxTheme.resultItemActiveBackgroundColor);
      fontColor = safeFromCssColor(woxTheme.resultItemActiveTitleColor);
    } else {
      backgroundColor = safeFromCssColor(woxTheme.queryBoxBackgroundColor);
      fontColor = safeFromCssColor(woxTheme.resultItemTitleColor);
    }

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
              crossAxisAlignment: isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
              children: [
                Container(
                  margin: const EdgeInsets.only(bottom: 4),
                  padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 10.0),
                  decoration: BoxDecoration(
                    color: backgroundColor,
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (isTool && message.toolCallInfo.id.isNotEmpty) _buildToolCallBadge(message),
                      if (!isTool)
                        WoxMarkdownView(
                          data: _formatMessageWithReasoning(message),
                          fontColor: fontColor,
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
                  child: Row(
                    mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
                    children: [
                      if (!isUser) ...[
                        Text(
                          controller.formatTimestamp(message.timestamp),
                          style: TextStyle(
                            fontSize: 11,
                            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Text(
                          "•",
                          style: TextStyle(
                            fontSize: 11,
                            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
                          ),
                        ),
                        const SizedBox(width: 12),
                        _buildInlineActionButtons(message, false),
                      ] else ...[
                        _buildInlineActionButtons(message, true),
                        const SizedBox(width: 12),
                        Text(
                          "•",
                          style: TextStyle(
                            fontSize: 11,
                            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
                          ),
                        ),
                        const SizedBox(width: 12),
                        Text(
                          controller.formatTimestamp(message.timestamp),
                          style: TextStyle(
                            fontSize: 11,
                            color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
                          ),
                        ),
                      ],
                    ],
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
        GestureDetector(
          onTap: () {
            controller.toggleToolCallExpanded(message.id);
          },
          child: SizedBox(
            width: double.infinity,
            child: Row(
              mainAxisSize: MainAxisSize.max,
              children: [
                Icon(
                  Icons.build,
                  size: 14,
                  color: safeFromCssColor(woxTheme.queryBoxFontColor),
                ),
                const SizedBox(width: 6),
                Expanded(
                  child: Text(
                    message.toolCallInfo.name,
                    style: TextStyle(
                      fontSize: 12,
                      color: safeFromCssColor(woxTheme.queryBoxFontColor),
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ),
                const SizedBox(width: 6),
                WoxChatToolcallDuration(
                  id: message.id,
                  startTimestamp: message.toolCallInfo.startTimestamp,
                  endTimestamp: (message.toolCallInfo.status == ToolCallStatus.streaming ||
                          message.toolCallInfo.status == ToolCallStatus.pending ||
                          message.toolCallInfo.status == ToolCallStatus.running)
                      ? null
                      : message.toolCallInfo.endTimestamp,
                  style: TextStyle(
                    fontSize: 12,
                    color: safeFromCssColor(woxTheme.queryBoxFontColor),
                  ),
                ),
                const SizedBox(width: 6),
                _buildStatusIndicator(message.toolCallInfo),
                const SizedBox(width: 6),
                Obx(() => Icon(
                      controller.isToolCallExpanded(message.id) ? Icons.keyboard_arrow_up : Icons.keyboard_arrow_down,
                      size: 14,
                      color: safeFromCssColor(woxTheme.queryBoxFontColor),
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
        tooltip = tr('ui_ai_chat_tool_status_streaming');
        break;
      case ToolCallStatus.pending:
        icon = Icons.hourglass_empty;
        color = Colors.grey;
        tooltip = tr('ui_ai_chat_tool_status_pending');
        break;
      case ToolCallStatus.running:
        icon = Icons.refresh;
        color = Colors.blue;
        tooltip = tr('ui_ai_chat_tool_status_running');
        break;
      case ToolCallStatus.succeeded:
        icon = Icons.check_circle;
        color = Colors.green;
        tooltip = tr('ui_ai_chat_tool_status_succeeded');
        break;
      case ToolCallStatus.failed:
        icon = Icons.error;
        color = Colors.red;
        tooltip = Strings.format(tr('ui_ai_chat_tool_status_failed'), [info.response]);
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
        color: safeFromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(15),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: safeFromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(40),
          width: 1.0,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildDetailItem(tr('ui_ai_chat_tool_detail_id'), info.id),
          _buildDetailItem(tr('ui_ai_chat_tool_detail_name'), info.name),
          _buildDetailItem(tr('ui_ai_chat_tool_detail_params'), info.status == ToolCallStatus.streaming ? info.delta : info.arguments.toString()),
          if (info.response.isNotEmpty) _buildDetailItem(tr('ui_ai_chat_tool_detail_response'), info.response),
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
              color: safeFromCssColor(woxTheme.resultItemSubTitleColor),
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
                color: safeFromCssColor(woxTheme.resultItemTitleColor),
              ),
            ),
          )
        ],
      ),
    );
  }

  Widget _buildInlineActionButtons(WoxAIChatConversation message, bool isUser) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Copy button
        Tooltip(
          message: tr('ui_ai_chat_copy_message'),
          child: InkWell(
            onTap: () => controller.copyMessageContent(message),
            child: Icon(
              Icons.copy,
              size: 14,
              color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
            ),
          ),
        ),
        const SizedBox(width: 8),
        // Refresh button (only for AI messages) or Edit button (only for user messages)
        if (!isUser)
          Tooltip(
            message: tr('ui_ai_chat_regenerate_response'),
            child: InkWell(
              onTap: () => controller.regenerateAIResponse(message.id),
              child: Icon(
                Icons.refresh,
                size: 14,
                color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
              ),
            ),
          ),
        if (isUser)
          Tooltip(
            message: tr('ui_ai_chat_edit_message'),
            child: InkWell(
              onTap: () => controller.editUserMessage(message),
              child: Icon(
                Icons.edit,
                size: 14,
                color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor),
              ),
            ),
          ),
      ],
    );
  }

  Widget _buildAvatar(WoxAIChatConversation message) {
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;

    if (isUser) {
      return Container(
        width: 36,
        height: 36,
        decoration: BoxDecoration(
          color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
          shape: BoxShape.circle,
        ),
        child: Center(
          child: Icon(
            Icons.person,
            size: 20,
            color: safeFromCssColor(woxTheme.actionItemActiveFontColor),
          ),
        ),
      );
    }

    if (controller.aiChatData.value.agentName != null && controller.aiChatData.value.agentName!.isNotEmpty) {
      final currentAgent = controller.availableAgents.firstWhere(
        (agent) => agent.name == controller.aiChatData.value.agentName,
        orElse: () => AIAgent.empty(),
      );

      if (currentAgent.name.isNotEmpty && currentAgent.icon.imageData.isNotEmpty) {
        return ClipRRect(
          borderRadius: BorderRadius.circular(18),
          child: SizedBox(
            width: 36,
            height: 36,
            child: WoxImageView(
              woxImage: currentAgent.icon,
              width: 36,
              height: 36,
            ),
          ),
        );
      }
    }

    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: safeFromCssColor(woxTheme.queryBoxBackgroundColor),
        shape: BoxShape.circle,
      ),
      child: Center(
        child: Icon(
          Icons.smart_toy_outlined,
          size: 20,
          color: safeFromCssColor(woxTheme.queryBoxFontColor),
        ),
      ),
    );
  }

  String _formatMessageWithReasoning(WoxAIChatConversation message) {
    final content = message.text;
    final reasoning = message.reasoning;

    if (reasoning.isEmpty) {
      return content;
    }

    // Format reasoning as markdown blockquote (each line prefixed with "> ")
    final reasoningLines = reasoning.split('\n');
    final formattedReasoning = reasoningLines.map((line) => '> $line').join('\n');

    return '$formattedReasoning\n\n$content';
  }
}
