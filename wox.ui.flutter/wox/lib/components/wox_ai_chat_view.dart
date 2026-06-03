import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_chat_toolcall_duration.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/components/wox_preview_top_status_bar.dart';
import 'package:wox/components/wox_selectable_text.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/strings.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxAIChatView extends StatelessWidget {
  const WoxAIChatView({super.key, required this.controller});

  final WoxAIChatController controller;

  WoxTheme get woxTheme => WoxThemeUtil.instance.currentTheme.value;
  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  // Get translation from WoxSettingController
  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  Widget buildTopStatusBar() {
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);

    return Obx(() {
      final title = controller.aiChatData.value.title.isEmpty ? tr('ui_ai_chat_new_chat') : controller.aiChatData.value.title;
      final isFullscreen = controller.launcherController.isPreviewFullscreen.value;

      return WoxPreviewTopStatusBar(
        woxTheme: woxTheme,
        title: Text(
          title,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(color: fontColor, fontSize: _metrics.actionHeaderFontSize, fontWeight: FontWeight.w600, height: 1.1),
        ),
        actions: [
          WoxPreviewTopStatusBarAction(
            tooltip: controller.launcherController.previewFullscreenHotkeyLabel,
            onPressed: () {
              controller.launcherController.togglePreviewFullscreen(const UuidV4().generate());
            },
            icon: Icon(isFullscreen ? Icons.fullscreen_exit : Icons.fullscreen),
          ),
        ],
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) {
      Logger.instance.debug(const UuidV4().generate(), "repaint: chat view");
    }

    return Stack(
      children: [
        Column(
          children: [
            Padding(padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(16), vertical: _metrics.scaledSpacing(8)), child: buildTopStatusBar()),
            // Messages list
            Expanded(
              child: SingleChildScrollView(
                controller: controller.aiChatScrollController,
                padding: EdgeInsets.symmetric(vertical: _metrics.scaledSpacing(16)),
                child: Obx(() => Column(children: controller.aiChatData.value.conversations.map((message) => _buildMessageItem(message, context)).toList())),
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

                // Show chat select panel on the primary-modifier action hotkey.
                if (controller.launcherController.isActionHotkey(pressedHotkey)) {
                  controller.showChatSelectPanel();
                  return KeyEventResult.handled;
                }

                if (controller.launcherController.executeLocalActionByHotkey(
                  const UuidV4().generate(),
                  pressedHotkey,
                  allowedActionIds: {WoxLauncherController.localActionTogglePreviewFullscreenId},
                )) {
                  return KeyEventResult.handled;
                }

                return KeyEventResult.ignored;
              },
              // Wrap the input area content in a Column to place the expandable section above
              child: Padding(
                padding: EdgeInsets.all(_metrics.scaledSpacing(12)),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const SizedBox.shrink(),
                    Container(
                      decoration: BoxDecoration(
                        color: safeFromCssColor(woxTheme.queryBoxBackgroundColor),
                        borderRadius: BorderRadius.circular(woxTheme.queryBoxBorderRadius.toDouble()),
                        border: Border.all(color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25)),
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
                              contentPadding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(16), vertical: _metrics.scaledSpacing(10)),
                            ),
                            maxLines: null,
                            keyboardType: TextInputType.multiline,
                            cursorColor: safeFromCssColor(woxTheme.queryBoxCursorColor),
                            // AI chat lives in the launcher preview surface, so
                            // its controls follow density metrics while the
                            // settings/plugin-setting controls keep their own sizes.
                            style: TextStyle(fontSize: _metrics.resultSubtitleFontSize, color: safeFromCssColor(woxTheme.queryBoxFontColor)),
                          ),
                          // Input Box Toolbar (Send button, Tool icon)
                          Container(
                            height: _metrics.scaledSpacing(36),
                            padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(8)),
                            decoration: BoxDecoration(border: Border(top: BorderSide(color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25)))),
                            child: Row(
                              children: [
                                // Tool configuration button - opens chat select panel
                                Obx(
                                  () => WoxTooltip(
                                    // IconButton.tooltip would create a Material tooltip, so
                                    // chat toolbar icons use the shared WoxTooltip wrapper.
                                    message: tr('ui_ai_chat_configure_tools'),
                                    child: IconButton(
                                      icon: Icon(
                                        Icons.build,
                                        size: _metrics.scaledSpacing(18),
                                        color: controller.selectedTools.isNotEmpty ? getThemeTextColor() : getThemeTextColor().withAlpha(128),
                                      ),
                                      color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      onPressed: () {
                                        controller.showToolsPanel();
                                      },
                                      padding: EdgeInsets.zero,
                                      constraints: BoxConstraints(minWidth: _metrics.scaledSpacing(32), minHeight: _metrics.scaledSpacing(32)),
                                    ),
                                  ),
                                ),
                                // Agent selection button
                                Obx(
                                  () => WoxTooltip(
                                    message: tr('ui_ai_chat_select_agent'),
                                    child: IconButton(
                                      icon: Icon(
                                        Icons.smart_toy,
                                        size: _metrics.scaledSpacing(18),
                                        color:
                                            (controller.aiChatData.value.agentName != null && controller.aiChatData.value.agentName!.isNotEmpty)
                                                ? getThemeTextColor()
                                                : getThemeTextColor().withAlpha(128),
                                      ),
                                      color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      onPressed: () {
                                        controller.showAgentsPanel();
                                      },
                                      padding: EdgeInsets.zero,
                                      constraints: BoxConstraints(minWidth: _metrics.scaledSpacing(32), minHeight: _metrics.scaledSpacing(32)),
                                    ),
                                  ),
                                ),
                                // Model selection button
                                Obx(
                                  () => WoxTooltip(
                                    message: tr('ui_ai_chat_select_model_title'),
                                    child: IconButton(
                                      icon: Icon(
                                        Icons.model_training,
                                        size: _metrics.scaledSpacing(18),
                                        color: controller.aiChatData.value.model.value.name.isNotEmpty ? getThemeTextColor() : getThemeTextColor().withAlpha(128),
                                      ),
                                      color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                                      onPressed: () {
                                        controller.showModelsPanel();
                                      },
                                      padding: EdgeInsets.zero,
                                      constraints: BoxConstraints(minWidth: _metrics.scaledSpacing(32), minHeight: _metrics.scaledSpacing(32)),
                                    ),
                                  ),
                                ),
                                // Model Name Display
                                Obx(
                                  () => Text(
                                    controller.aiChatData.value.model.value.name.isEmpty ? tr("ui_ai_chat_select_model") : controller.aiChatData.value.model.value.name,
                                    style: TextStyle(color: getThemeTextColor(), fontSize: _metrics.smallLabelFontSize),
                                  ),
                                ),

                                const Spacer(),
                                // Send button container (unchanged)
                                InkWell(
                                  onTap: () => controller.sendMessage(),
                                  child: Container(
                                    padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(8), vertical: _metrics.scaledSpacing(4)),
                                    decoration: BoxDecoration(color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor), borderRadius: BorderRadius.circular(4)),
                                    child: Row(
                                      mainAxisSize: MainAxisSize.min,
                                      children: [
                                        Icon(Icons.keyboard_return, size: _metrics.scaledSpacing(14), color: safeFromCssColor(woxTheme.actionItemActiveFontColor)),
                                        SizedBox(width: _metrics.scaledSpacing(4)),
                                        Text(
                                          tr('ui_ai_chat_send'),
                                          style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(woxTheme.actionItemActiveFontColor)),
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
      right: _metrics.scaledSpacing(10),
      bottom: _metrics.scaledSpacing(10),
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
            constraints: BoxConstraints(maxWidth: _metrics.scaledSpacing(320)),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              mainAxisAlignment: MainAxisAlignment.start,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Obx(
                  () => Text(
                    controller.currentChatSelectCategory.isEmpty
                        ? tr("ui_ai_chat_options")
                        : (controller.currentChatSelectCategory.value == "models"
                            ? tr("ui_ai_chat_select_model_title")
                            : (controller.currentChatSelectCategory.value == "tools" ? tr("ui_ai_chat_configure_tools_title") : tr("ui_ai_chat_select_agent_title"))),
                    style: TextStyle(color: safeFromCssColor(woxTheme.actionContainerHeaderFontColor), fontSize: _metrics.actionHeaderFontSize),
                  ),
                ),
                const Divider(),
                WoxListView<ChatSelectItem>(
                  controller: controller.chatSelectListController,
                  listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_CHAT.code,
                  showFilter: true,
                  // Chat selection uses the launcher action-list surface, so
                  // visible capacity follows density instead of the old fixed
                  // 350px panel height.
                  maxHeight: _metrics.actionItemBaseHeight * 8.75,
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
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(16), vertical: _metrics.scaledSpacing(4)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        children: [
          if (!isUser) _buildAvatar(message),
          SizedBox(width: _metrics.scaledSpacing(8)),
          Flexible(
            child: Column(
              crossAxisAlignment: isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
              children: [
                Container(
                  margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(4)),
                  padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(16), vertical: _metrics.scaledSpacing(10)),
                  decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(8)),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (isTool && message.toolCallInfo.id.isNotEmpty) _buildToolCallBadge(message),
                      if (!isTool) WoxMarkdownView(data: _formatMessageWithReasoning(message), fontColor: fontColor, fontSize: _metrics.resultSubtitleFontSize),
                      if (message.images.isNotEmpty) ...[
                        SizedBox(height: _metrics.scaledSpacing(8)),
                        Wrap(
                          spacing: _metrics.scaledSpacing(8),
                          runSpacing: _metrics.scaledSpacing(8),
                          children:
                              message.images
                                  .map(
                                    (image) => ClipRRect(
                                      borderRadius: BorderRadius.circular(8),
                                      child: SizedBox(width: _metrics.scaledSpacing(200), child: WoxImageView(woxImage: image)),
                                    ),
                                  )
                                  .toList(),
                        ),
                      ],
                    ],
                  ),
                ),
                Padding(
                  padding: EdgeInsets.only(left: _metrics.scaledSpacing(4), right: _metrics.scaledSpacing(4)),
                  child: Row(
                    mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
                    children: [
                      if (!isUser) ...[
                        Text(
                          controller.formatTimestamp(message.timestamp),
                          style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
                        ),
                        SizedBox(width: _metrics.scaledSpacing(12)),
                        Text(
                          "•",
                          style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
                        ),
                        SizedBox(width: _metrics.scaledSpacing(12)),
                        _buildInlineActionButtons(message, false),
                      ] else ...[
                        _buildInlineActionButtons(message, true),
                        SizedBox(width: _metrics.scaledSpacing(12)),
                        Text(
                          "•",
                          style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
                        ),
                        SizedBox(width: _metrics.scaledSpacing(12)),
                        Text(
                          controller.formatTimestamp(message.timestamp),
                          style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),
          SizedBox(width: _metrics.scaledSpacing(8)),
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
                Icon(Icons.build, size: _metrics.scaledSpacing(14), color: safeFromCssColor(woxTheme.queryBoxFontColor)),
                SizedBox(width: _metrics.scaledSpacing(6)),
                Expanded(
                  child: Text(
                    message.toolCallInfo.name,
                    style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(woxTheme.queryBoxFontColor), fontWeight: FontWeight.w500),
                  ),
                ),
                SizedBox(width: _metrics.scaledSpacing(6)),
                WoxChatToolcallDuration(
                  id: message.id,
                  startTimestamp: message.toolCallInfo.startTimestamp,
                  endTimestamp:
                      (message.toolCallInfo.status == ToolCallStatus.streaming ||
                              message.toolCallInfo.status == ToolCallStatus.pending ||
                              message.toolCallInfo.status == ToolCallStatus.running)
                          ? null
                          : message.toolCallInfo.endTimestamp,
                  style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(woxTheme.queryBoxFontColor)),
                ),
                SizedBox(width: _metrics.scaledSpacing(6)),
                _buildStatusIndicator(message.toolCallInfo),
                SizedBox(width: _metrics.scaledSpacing(6)),
                Obx(
                  () => Icon(
                    controller.isToolCallExpanded(message.id) ? Icons.keyboard_arrow_up : Icons.keyboard_arrow_down,
                    size: _metrics.scaledSpacing(14),
                    color: safeFromCssColor(woxTheme.queryBoxFontColor),
                  ),
                ),
              ],
            ),
          ),
        ),
        Obx(() => controller.isToolCallExpanded(message.id) ? _buildToolCallDetails(message.toolCallInfo) : const SizedBox.shrink()),
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

    // Tool-call status hints are hover-only metadata, so use WoxTooltip to keep
    // chat details consistent with launcher and settings tooltip overlays.
    return WoxTooltip(message: tooltip, child: Icon(icon, size: _metrics.scaledSpacing(14), color: color));
  }

  Widget _buildToolCallDetails(ToolCallInfo info) {
    return Container(
      width: double.infinity,
      margin: EdgeInsets.only(top: _metrics.scaledSpacing(8)),
      padding: EdgeInsets.all(_metrics.scaledSpacing(8)),
      decoration: BoxDecoration(
        color: safeFromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(15),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: safeFromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(40), width: 1.0),
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
      padding: EdgeInsets.only(bottom: _metrics.scaledSpacing(8)),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: TextStyle(fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.bold, color: safeFromCssColor(woxTheme.resultItemSubTitleColor))),
          SizedBox(height: _metrics.scaledSpacing(4)),
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(_metrics.scaledSpacing(6)),
            decoration: BoxDecoration(color: Colors.black.withAlpha(20), border: Border.all(color: Colors.black.withAlpha(10), width: 1.0)),
            child: WoxSelectableText(
              value,
              style: TextStyle(fontSize: _metrics.smallLabelFontSize, fontFamily: 'monospace', color: safeFromCssColor(woxTheme.resultItemTitleColor)),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildInlineActionButtons(WoxAIChatConversation message, bool isUser) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Copy button
        WoxTooltip(
          message: tr('ui_ai_chat_copy_message'),
          child: InkWell(
            onTap: () => controller.copyMessageContent(message),
            child: Icon(Icons.copy, size: _metrics.scaledSpacing(14), color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
          ),
        ),
        SizedBox(width: _metrics.scaledSpacing(8)),
        // Refresh button (only for AI messages) or Edit button (only for user messages)
        if (!isUser)
          WoxTooltip(
            message: tr('ui_ai_chat_regenerate_response'),
            child: InkWell(
              onTap: () => controller.regenerateAIResponse(message.id),
              child: Icon(Icons.refresh, size: _metrics.scaledSpacing(14), color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
            ),
          ),
        if (isUser)
          WoxTooltip(
            message: tr('ui_ai_chat_edit_message'),
            child: InkWell(
              onTap: () => controller.editUserMessage(message),
              child: Icon(Icons.edit, size: _metrics.scaledSpacing(14), color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor)),
            ),
          ),
      ],
    );
  }

  Widget _buildAvatar(WoxAIChatConversation message) {
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;

    if (isUser) {
      return Container(
        width: _metrics.scaledSpacing(36),
        height: _metrics.scaledSpacing(36),
        decoration: BoxDecoration(color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor), shape: BoxShape.circle),
        child: Center(child: Icon(Icons.person, size: _metrics.scaledSpacing(20), color: safeFromCssColor(woxTheme.actionItemActiveFontColor))),
      );
    }

    if (controller.aiChatData.value.agentName != null && controller.aiChatData.value.agentName!.isNotEmpty) {
      final currentAgent = controller.availableAgents.firstWhere((agent) => agent.name == controller.aiChatData.value.agentName, orElse: () => AIAgent.empty());

      if (currentAgent.name.isNotEmpty && currentAgent.icon.imageData.isNotEmpty) {
        final avatarSize = _metrics.scaledSpacing(36);
        return ClipRRect(
          borderRadius: BorderRadius.circular(avatarSize / 2),
          child: SizedBox(width: avatarSize, height: avatarSize, child: WoxImageView(woxImage: currentAgent.icon, width: avatarSize, height: avatarSize)),
        );
      }
    }

    return Container(
      width: _metrics.scaledSpacing(36),
      height: _metrics.scaledSpacing(36),
      decoration: BoxDecoration(color: safeFromCssColor(woxTheme.queryBoxBackgroundColor), shape: BoxShape.circle),
      child: Center(child: Icon(Icons.smart_toy_outlined, size: _metrics.scaledSpacing(20), color: safeFromCssColor(woxTheme.queryBoxFontColor))),
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
