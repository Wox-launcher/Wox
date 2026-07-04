import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_chat_toolcall_duration.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/components/wox_platform_focus.dart';
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

class WoxAIChatView extends GetView<WoxAIChatController> {
  const WoxAIChatView({super.key});

  WoxTheme get woxTheme => WoxThemeUtil.instance.currentTheme.value;
  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  // Get translation from WoxSettingController
  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  Widget buildTopStatusBar() {
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);
    final subtitleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);

    return Obx(() {
      final title = controller.aiChatData.value.title.isEmpty ? tr('ui_ai_chat_new_chat') : controller.aiChatData.value.title;
      final isFullscreen = controller.launcherController.isPreviewFullscreen.value;
      final showExitChatMode = !controller.launcherController.isQueryBoxVisible.value;

      return SizedBox(
        height: _metrics.scaledSpacing(46),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            WoxTooltip(
              message: tr("ui_action_toggle_sidebar"),
              child: IconButton(
                onPressed: () => controller.launcherController.togglePreviewFullscreen(const UuidV4().generate()),
                icon: Icon(isFullscreen ? Icons.view_sidebar_outlined : Icons.splitscreen_outlined),
                iconSize: _metrics.scaledSpacing(22),
                color: subtitleColor,
                padding: EdgeInsets.zero,
                constraints: BoxConstraints.tightFor(width: _metrics.scaledSpacing(36), height: _metrics.scaledSpacing(36)),
                splashRadius: _metrics.scaledSpacing(18),
                visualDensity: VisualDensity.compact,
              ),
            ),
            SizedBox(width: _metrics.scaledSpacing(4)),
            Expanded(
              child: Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: fontColor, fontSize: _metrics.actionHeaderFontSize, fontWeight: FontWeight.w700, height: 1.05),
              ),
            ),
            if (showExitChatMode)
              WoxTooltip(
                message: "${tr("ui_back")} (Esc)",
                child: IconButton(
                  onPressed: () => controller.launcherController.exitChatInputMode(const UuidV4().generate()),
                  icon: const Icon(Icons.close_rounded),
                  iconSize: _metrics.scaledSpacing(20),
                  color: subtitleColor,
                  padding: EdgeInsets.zero,
                  constraints: BoxConstraints.tightFor(width: _metrics.scaledSpacing(36), height: _metrics.scaledSpacing(36)),
                  splashRadius: _metrics.scaledSpacing(18),
                  visualDensity: VisualDensity.compact,
                ),
              ),
          ],
        ),
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
        LayoutBuilder(
          builder: (context, constraints) {
            final showConversationSidebar = constraints.maxWidth >= _metrics.scaledSpacing(760);
            return Row(
              children: [
                if (showConversationSidebar) ...[_buildConversationSidebar(), Container(width: 1, color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(20))],
                Expanded(child: _buildChatConversationPane(context)),
              ],
            );
          },
        ),
        Obx(() => controller.isShowChatSelectPanel.value ? _buildChatSelectPanel(context) : const SizedBox.shrink()),
        Obx(() {
          final question = controller.pendingAIQuestion.value;
          return question == null ? const SizedBox.shrink() : _buildAIQuestionOverlay(question);
        }),
      ],
    );
  }

  Widget _buildChatConversationPane(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), _metrics.scaledSpacing(6), _metrics.scaledSpacing(10), _metrics.scaledSpacing(4)),
          child: buildTopStatusBar(),
        ),
        Expanded(
          child: SingleChildScrollView(
            controller: controller.aiChatScrollController,
            padding: EdgeInsets.only(top: _metrics.scaledSpacing(6), bottom: _metrics.scaledSpacing(8)),
            child: Obx(() => Column(children: controller.aiChatData.value.conversations.map((message) => _buildMessageItem(message, context)).toList())),
          ),
        ),
        _buildChatInputArea(),
      ],
    );
  }

  Widget _buildChatInputArea() {
    return WoxPlatformFocus(
      onKeyEvent: _handleChatInputKeyEvent,
      child: Padding(
        padding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), _metrics.scaledSpacing(6), _metrics.scaledSpacing(10), _metrics.scaledSpacing(8)),
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
                      contentPadding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(14), vertical: _metrics.scaledSpacing(8)),
                    ),
                    maxLines: null,
                    keyboardType: TextInputType.multiline,
                    cursorColor: safeFromCssColor(woxTheme.queryBoxCursorColor),
                    // AI chat lives in the launcher preview surface, so its controls
                    // follow density metrics while settings controls keep their own sizes.
                    style: TextStyle(fontSize: _metrics.resultSubtitleFontSize, color: safeFromCssColor(woxTheme.queryBoxFontColor)),
                  ),
                  Container(
                    height: _metrics.scaledSpacing(34),
                    padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(8)),
                    decoration: BoxDecoration(border: Border(top: BorderSide(color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25)))),
                    child: Row(
                      children: [
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
                        Obx(
                          () => Text(
                            controller.aiChatData.value.model.value.name.isEmpty ? tr("ui_ai_chat_select_model") : controller.aiChatData.value.model.value.name,
                            style: TextStyle(color: getThemeTextColor(), fontSize: _metrics.smallLabelFontSize),
                          ),
                        ),
                        const Spacer(),
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
                                Text(tr('ui_ai_chat_send'), style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: safeFromCssColor(woxTheme.actionItemActiveFontColor))),
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
    );
  }

  KeyEventResult _handleChatInputKeyEvent(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent) {
      switch (event.logicalKey) {
        case LogicalKeyboardKey.escape:
          controller.launcherController.exitChatInputMode(const UuidV4().generate());
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
  }

  Widget _buildConversationSidebar() {
    final sidebarColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);
    return SizedBox(
      width: _metrics.scaledSpacing(260),
      child: Obx(() {
        final groupedChats = _groupChats(controller.chats);
        return ListView(
          padding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), _metrics.scaledSpacing(12), _metrics.scaledSpacing(10), _metrics.scaledSpacing(12)),
          children: [
            _buildConversationSectionTitle(tr("ui_ai_chat_new_chat")),
            _buildNewChatTile(sidebarColor),
            if (groupedChats.today.isNotEmpty) ...[_buildConversationSectionTitle(tr("ui_ai_chat_history_today")), ...groupedChats.today.map(_buildConversationTile)],
            if (groupedChats.yesterday.isNotEmpty) ...[_buildConversationSectionTitle(tr("ui_ai_chat_history_yesterday")), ...groupedChats.yesterday.map(_buildConversationTile)],
            if (groupedChats.history.isNotEmpty) ...[_buildConversationSectionTitle(tr("ui_ai_chat_history_history")), ...groupedChats.history.map(_buildConversationTile)],
          ],
        );
      }),
    );
  }

  Widget _buildConversationSectionTitle(String title) {
    return Padding(
      padding: EdgeInsets.only(left: _metrics.scaledSpacing(4), top: _metrics.scaledSpacing(10), bottom: _metrics.scaledSpacing(6)),
      child: Text(title, style: TextStyle(color: safeFromCssColor(woxTheme.previewFontColor), fontSize: _metrics.resultSubtitleFontSize, fontWeight: FontWeight.w700)),
    );
  }

  Widget _buildNewChatTile(Color subtitleColor) {
    final isActiveDraft = controller.aiChatData.value.conversations.isEmpty && controller.chats.every((chat) => chat.id != controller.aiChatData.value.id);
    return _buildConversationTileShell(
      title: tr("ui_ai_chat_new_chat"),
      subtitle: tr("ui_ai_chat_create_new_chat"),
      active: isActiveDraft,
      onTap: controller.startNewChat,
      subtitleColor: subtitleColor,
    );
  }

  Widget _buildConversationTile(WoxAIChatData chat) {
    return _buildConversationTileShell(
      title: chat.title.isEmpty ? tr("ui_ai_chat_new_chat") : chat.title,
      subtitle: _getConversationSubtitle(chat),
      active: chat.id == controller.aiChatData.value.id,
      onTap: () => controller.selectChat(chat),
      subtitleColor: safeFromCssColor(woxTheme.resultItemSubTitleColor),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildConversationTileAction(tooltip: tr("ui_ai_chat_summarize_chat"), icon: Icons.short_text_rounded, onPressed: () => controller.summarizeChat(chat)),
          _buildConversationTileAction(tooltip: tr("ui_ai_chat_delete_chat"), icon: Icons.delete_outline_rounded, onPressed: () => controller.deleteChat(chat)),
        ],
      ),
    );
  }

  // Build a compact action button that fits inside the conversation sidebar row.
  Widget _buildConversationTileAction({required String tooltip, required IconData icon, required VoidCallback onPressed}) {
    return WoxTooltip(
      message: tooltip,
      child: IconButton(
        onPressed: onPressed,
        icon: Icon(icon, size: _metrics.scaledSpacing(15)),
        color: safeFromCssColor(woxTheme.resultItemSubTitleColor),
        padding: EdgeInsets.zero,
        constraints: BoxConstraints.tightFor(width: _metrics.scaledSpacing(26), height: _metrics.scaledSpacing(26)),
        splashRadius: _metrics.scaledSpacing(13),
        visualDensity: VisualDensity.compact,
      ),
    );
  }

  Widget _buildConversationTileShell({
    required String title,
    required String subtitle,
    required bool active,
    required VoidCallback onTap,
    required Color subtitleColor,
    Widget? trailing,
  }) {
    final backgroundColor = active ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor) : Colors.transparent;
    final titleColor = active ? safeFromCssColor(woxTheme.resultItemActiveTitleColor) : safeFromCssColor(woxTheme.resultItemTitleColor);
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(6),
      child: Container(
        height: _metrics.scaledSpacing(58),
        margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(4)),
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(8)),
        decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(6)),
        child: Row(
          children: [
            Icon(Icons.chat_bubble, size: _metrics.scaledSpacing(26), color: safeFromCssColor(woxTheme.resultItemActiveBackgroundColor)),
            SizedBox(width: _metrics.scaledSpacing(10)),
            Expanded(
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: titleColor, fontSize: _metrics.resultTitleFontSize, fontWeight: FontWeight.w700),
                  ),
                  SizedBox(height: _metrics.scaledSpacing(2)),
                  Text(subtitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: subtitleColor, fontSize: _metrics.smallLabelFontSize)),
                ],
              ),
            ),
            if (trailing != null) ...[SizedBox(width: _metrics.scaledSpacing(4)), trailing],
          ],
        ),
      ),
    );
  }

  ({List<WoxAIChatData> today, List<WoxAIChatData> yesterday, List<WoxAIChatData> history}) _groupChats(List<WoxAIChatData> chats) {
    final now = DateTime.now();
    final todayStart = DateTime(now.year, now.month, now.day);
    final yesterdayStart = todayStart.subtract(const Duration(days: 1));
    final today = <WoxAIChatData>[];
    final yesterday = <WoxAIChatData>[];
    final history = <WoxAIChatData>[];
    for (final chat in chats) {
      if (chat.conversations.isEmpty) {
        continue;
      }
      final updatedAt = DateTime.fromMillisecondsSinceEpoch(chat.updatedAt);
      if (!updatedAt.isBefore(todayStart)) {
        today.add(chat);
      } else if (!updatedAt.isBefore(yesterdayStart)) {
        yesterday.add(chat);
      } else {
        history.add(chat);
      }
    }
    return (today: today, yesterday: yesterday, history: history);
  }

  String _getConversationSubtitle(WoxAIChatData chat) {
    for (final conversation in chat.conversations) {
      if (conversation.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value && conversation.text.trim().isNotEmpty) {
        return conversation.text.trim();
      }
    }
    return tr("ui_ai_chat_continue_chat");
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
                        : (controller.currentChatSelectCategory.value == "models" ? tr("ui_ai_chat_select_model_title") : tr("ui_ai_chat_select_agent_title")),
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

  // Keeps ask_user prompts inside the chat preview instead of using launcher-level dialogs.
  Widget _buildAIQuestionOverlay(AIQuestion question) {
    final panelColor = safeFromCssColor(woxTheme.actionContainerBackgroundColor);
    final borderColor = safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(45);
    final titleColor = safeFromCssColor(woxTheme.actionContainerHeaderFontColor);
    final textColor = safeFromCssColor(woxTheme.queryBoxFontColor);
    final subTextColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);

    return Positioned.fill(
      child: Container(
        color: Colors.black.withValues(alpha: 0.28),
        alignment: Alignment.center,
        padding: EdgeInsets.all(_metrics.scaledSpacing(16)),
        child: WoxPlatformFocus(
          focusNode: controller.aiQuestionPanelFocusNode,
          onKeyEvent: (_, event) {
            if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
              controller.cancelPendingAIQuestion();
              return KeyEventResult.handled;
            }
            return KeyEventResult.ignored;
          },
          child: Material(
            elevation: 10,
            color: Colors.transparent,
            borderRadius: BorderRadius.circular(8),
            child: ConstrainedBox(
              constraints: BoxConstraints(maxWidth: _metrics.scaledSpacing(460)),
              child: Container(
                padding: EdgeInsets.all(_metrics.scaledSpacing(14)),
                decoration: BoxDecoration(color: panelColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: borderColor)),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(Icons.help_outline_rounded, size: _metrics.scaledSpacing(20), color: titleColor),
                        SizedBox(width: _metrics.scaledSpacing(8)),
                        Expanded(
                          child: Text(
                            tr("ui_ai_question_title"),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(color: titleColor, fontSize: _metrics.actionHeaderFontSize, fontWeight: FontWeight.w700),
                          ),
                        ),
                        WoxTooltip(
                          message: tr("ui_cancel"),
                          child: InkWell(
                            onTap: controller.cancelPendingAIQuestion,
                            borderRadius: BorderRadius.circular(4),
                            child: Padding(
                              padding: EdgeInsets.all(_metrics.scaledSpacing(4)),
                              child: Icon(Icons.close_rounded, size: _metrics.scaledSpacing(18), color: subTextColor),
                            ),
                          ),
                        ),
                      ],
                    ),
                    SizedBox(height: _metrics.scaledSpacing(10)),
                    Text(question.question, style: TextStyle(color: textColor, fontSize: _metrics.resultSubtitleFontSize, height: 1.35)),
                    SizedBox(height: _metrics.scaledSpacing(12)),
                    if (question.options.isEmpty) _buildAIQuestionFreeTextAnswer(textColor, subTextColor) else _buildAIQuestionOptions(question, textColor, subTextColor),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  // Free-text ask_user prompts need a focused editor and explicit submit action.
  Widget _buildAIQuestionFreeTextAnswer(Color textColor, Color subTextColor) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        TextField(
          controller: controller.aiQuestionAnswerController,
          focusNode: controller.aiQuestionAnswerFocusNode,
          minLines: 2,
          maxLines: 4,
          cursorColor: safeFromCssColor(woxTheme.queryBoxCursorColor),
          style: TextStyle(color: textColor, fontSize: _metrics.resultSubtitleFontSize),
          decoration: InputDecoration(
            hintText: tr("ui_ai_question_answer_hint"),
            hintStyle: TextStyle(color: subTextColor),
            filled: true,
            fillColor: safeFromCssColor(woxTheme.queryBoxBackgroundColor),
            enabledBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: subTextColor.withAlpha(60))),
            focusedBorder: OutlineInputBorder(borderRadius: BorderRadius.circular(6), borderSide: BorderSide(color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor))),
          ),
        ),
        SizedBox(height: _metrics.scaledSpacing(12)),
        Row(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            _buildAIQuestionButton(label: tr("ui_cancel"), onTap: controller.cancelPendingAIQuestion, primary: false),
            SizedBox(width: _metrics.scaledSpacing(8)),
            _buildAIQuestionButton(label: tr("ui_ai_question_submit"), onTap: controller.submitPendingAIQuestionAnswer, primary: true),
          ],
        ),
      ],
    );
  }

  // Structured ask_user prompts are selectable rows so the answer value is sent directly.
  Widget _buildAIQuestionOptions(AIQuestion question, Color textColor, Color subTextColor) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        ConstrainedBox(
          constraints: BoxConstraints(maxHeight: _metrics.actionItemBaseHeight * 6),
          child: ListView.separated(
            shrinkWrap: true,
            itemCount: question.options.length,
            separatorBuilder: (context, index) => SizedBox(height: _metrics.scaledSpacing(8)),
            itemBuilder: (_, index) => _buildAIQuestionOptionTile(question.options[index], textColor, subTextColor),
          ),
        ),
        SizedBox(height: _metrics.scaledSpacing(12)),
        Row(mainAxisAlignment: MainAxisAlignment.end, children: [_buildAIQuestionButton(label: tr("ui_cancel"), onTap: controller.cancelPendingAIQuestion, primary: false)]),
      ],
    );
  }

  Widget _buildAIQuestionOptionTile(AIQuestionOption option, Color textColor, Color subTextColor) {
    return InkWell(
      onTap: () => controller.answerPendingAIQuestion(option.value),
      borderRadius: BorderRadius.circular(6),
      child: Container(
        width: double.infinity,
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(10)),
        decoration: BoxDecoration(
          color: safeFromCssColor(woxTheme.queryBoxBackgroundColor),
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: subTextColor.withAlpha(45)),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    option.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: textColor, fontSize: _metrics.resultSubtitleFontSize, fontWeight: FontWeight.w700),
                  ),
                  if (option.subTitle.isNotEmpty) ...[
                    SizedBox(height: _metrics.scaledSpacing(4)),
                    Text(option.subTitle, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(color: subTextColor, fontSize: _metrics.smallLabelFontSize, height: 1.25)),
                  ],
                ],
              ),
            ),
            if (option.recommended) ...[
              SizedBox(width: _metrics.scaledSpacing(10)),
              Container(
                padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(6), vertical: _metrics.scaledSpacing(3)),
                decoration: BoxDecoration(color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor), borderRadius: BorderRadius.circular(4)),
                child: Text(tr("ui_ai_question_recommended"), style: TextStyle(color: safeFromCssColor(woxTheme.actionItemActiveFontColor), fontSize: _metrics.smallLabelFontSize)),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildAIQuestionButton({required String label, required VoidCallback onTap, required bool primary}) {
    final backgroundColor = primary ? safeFromCssColor(woxTheme.actionItemActiveBackgroundColor) : safeFromCssColor(woxTheme.queryBoxBackgroundColor);
    final textColor = primary ? safeFromCssColor(woxTheme.actionItemActiveFontColor) : safeFromCssColor(woxTheme.queryBoxFontColor);
    final borderColor = safeFromCssColor(woxTheme.resultItemSubTitleColor).withAlpha(70);

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(6),
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(7)),
        decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(6), border: Border.all(color: primary ? backgroundColor : borderColor)),
        child: Text(label, style: TextStyle(color: textColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w700)),
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

    if (isUser) {
      return _buildUserMessageItem(message, context);
    }

    return _buildAssistantMessageItem(message, isTool);
  }

  // Renders user messages without an avatar so the content column keeps more usable width.
  Widget _buildUserMessageItem(WoxAIChatConversation message, BuildContext context) {
    final fontColor = safeFromCssColor(woxTheme.resultItemActiveTitleColor);

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(3)),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final maxBubbleWidth = constraints.hasBoundedWidth ? constraints.maxWidth * 0.82 : _metrics.scaledSpacing(520);
          return Align(
            alignment: Alignment.centerRight,
            child: ConstrainedBox(
              constraints: BoxConstraints(maxWidth: maxBubbleWidth),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Container(
                    margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(3)),
                    padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(8)),
                    decoration: BoxDecoration(color: safeFromCssColor(woxTheme.resultItemActiveBackgroundColor), borderRadius: BorderRadius.circular(8)),
                    child: _buildMessageContent(message, fontColor, false),
                  ),
                  _buildMessageMetaRow(message, true),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  // Renders assistant and tool messages as a full-width reading column.
  Widget _buildAssistantMessageItem(WoxAIChatConversation message, bool isTool) {
    final fontColor = safeFromCssColor(woxTheme.resultItemTitleColor);

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(3)),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildAvatar(message),
          SizedBox(width: _metrics.scaledSpacing(6)),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: double.infinity,
                  margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(3)),
                  padding:
                      isTool
                          ? EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(10), vertical: _metrics.scaledSpacing(8))
                          : EdgeInsets.only(top: _metrics.scaledSpacing(1), right: _metrics.scaledSpacing(4)),
                  decoration: isTool ? BoxDecoration(color: safeFromCssColor(woxTheme.queryBoxBackgroundColor), borderRadius: BorderRadius.circular(6)) : null,
                  child: _buildMessageContent(message, fontColor, isTool),
                ),
                _buildMessageMetaRow(message, false),
              ],
            ),
          ),
        ],
      ),
    );
  }

  // Renders the shared text and image payload for chat messages.
  Widget _buildMessageContent(WoxAIChatConversation message, Color fontColor, bool isTool) {
    return Column(
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
                    .map((image) => ClipRRect(borderRadius: BorderRadius.circular(8), child: SizedBox(width: _metrics.scaledSpacing(200), child: WoxImageView(woxImage: image))))
                    .toList(),
          ),
        ],
      ],
    );
  }

  // Keeps timestamps and inline actions aligned with the message side.
  Widget _buildMessageMetaRow(WoxAIChatConversation message, bool isUser) {
    final metaColor = safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.resultItemSubTitleColor);

    return Padding(
      padding: EdgeInsets.only(left: _metrics.scaledSpacing(2), right: _metrics.scaledSpacing(2), bottom: _metrics.scaledSpacing(2)),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (!isUser) ...[
            Text(controller.formatTimestamp(message.timestamp), style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: metaColor)),
            SizedBox(width: _metrics.scaledSpacing(10)),
            Text("•", style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: metaColor)),
            SizedBox(width: _metrics.scaledSpacing(10)),
            _buildInlineActionButtons(message, false),
          ] else ...[
            _buildInlineActionButtons(message, true),
            SizedBox(width: _metrics.scaledSpacing(10)),
            Text("•", style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: metaColor)),
            SizedBox(width: _metrics.scaledSpacing(10)),
            Text(controller.formatTimestamp(message.timestamp), style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: metaColor)),
          ],
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
