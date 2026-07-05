import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter/gestures.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_chat_toolcall_duration.dart';
import 'package:wox/components/wox_image_view.dart';
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
  double get _commandPaletteItemHeight => _metrics.scaledSpacing(38);
  double get _commandPaletteHeaderHeight => _metrics.scaledSpacing(28);
  double get _commandPaletteVerticalPadding => _metrics.scaledSpacing(8);

  // Get translation from WoxSettingController
  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  Widget buildTopStatusBar() {
    final fontColor = safeFromCssColor(woxTheme.previewFontColor);
    final subtitleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);

    return Obx(() {
      final title = controller.aiChatData.value.title.isEmpty ? tr('ui_ai_chat_new_chat') : controller.aiChatData.value.title;
      final isConversationSidebarCollapsed = controller.isConversationSidebarCollapsed.value;
      final showExitChatMode = !controller.launcherController.isQueryBoxVisible.value;
      final hasDebugTrace = controller.aiChatData.value.debugTrace.value != null;

      return SizedBox(
        height: _metrics.scaledSpacing(46),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            WoxTooltip(
              message: "${tr("ui_action_toggle_sidebar")} (${controller.launcherController.previewFullscreenHotkeyLabel})",
              child: IconButton(
                onPressed: () => controller.toggleConversationSidebar(const UuidV4().generate()),
                icon: Icon(isConversationSidebarCollapsed ? Icons.view_sidebar_outlined : Icons.splitscreen_outlined),
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
            if (hasDebugTrace)
              WoxTooltip(
                message: "Debug trace",
                child: IconButton(
                  onPressed: controller.toggleDebugInspector,
                  icon: const Icon(Icons.bug_report_outlined),
                  iconSize: _metrics.scaledSpacing(19),
                  color: subtitleColor,
                  padding: EdgeInsets.zero,
                  constraints: BoxConstraints.tightFor(width: _metrics.scaledSpacing(36), height: _metrics.scaledSpacing(36)),
                  splashRadius: _metrics.scaledSpacing(18),
                  visualDensity: VisualDensity.compact,
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
            return Obx(() {
              final showConversationSidebar = constraints.maxWidth >= _metrics.scaledSpacing(760) && !controller.isConversationSidebarCollapsed.value;
              return Row(
                children: [
                  if (showConversationSidebar) ...[_buildConversationSidebar(), Container(width: 1, color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(20))],
                  Expanded(child: _buildChatConversationPane(context)),
                ],
              );
            });
          },
        ),
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
          child: Obx(() {
            final renderItems = _buildChatRenderItems(controller.aiChatData.value.conversations);
            if (renderItems.isEmpty) {
              return _buildEmptyChatPlaceholder();
            }

            return SingleChildScrollView(
              controller: controller.aiChatScrollController,
              physics: const ClampingScrollPhysics(),
              padding: EdgeInsets.only(top: _metrics.scaledSpacing(6), bottom: _metrics.scaledSpacing(8)),
              child: Column(children: renderItems.map((item) => _buildChatRenderItem(item, context)).toList()),
            );
          }),
        ),
        Obx(() {
          final trace = controller.aiChatData.value.debugTrace.value;
          if (!controller.isDebugInspectorVisible.value || trace == null) {
            return const SizedBox.shrink();
          }
          return _buildDebugInspector(context, trace);
        }),
        _buildChatInputArea(),
      ],
    );
  }

  Widget _buildEmptyChatPlaceholder() {
    final textColor = safeFromCssColor(woxTheme.resultItemTitleColor).withAlpha(150);

    return Center(
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(24)),
        child: Text(
          tr("ui_ai_chat_empty_prompt"),
          textAlign: TextAlign.center,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: TextStyle(color: textColor, fontSize: _metrics.scaledSpacing(28), fontWeight: FontWeight.w500, height: 1.2),
        ),
      ),
    );
  }

  Widget _buildDebugInspector(BuildContext context, AIChatDebugTrace trace) {
    final borderColor = safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(35);
    final backgroundColor = safeFromCssColor(woxTheme.actionContainerBackgroundColor);
    final titleColor = safeFromCssColor(woxTheme.actionContainerHeaderFontColor);
    final subTitleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);

    return Container(
      margin: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), _metrics.scaledSpacing(4), _metrics.scaledSpacing(10), _metrics.scaledSpacing(4)),
      constraints: BoxConstraints(maxHeight: _metrics.scaledSpacing(260)),
      decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: borderColor)),
      child: Column(
        children: [
          Padding(
            padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(10), vertical: _metrics.scaledSpacing(7)),
            child: Row(
              children: [
                Icon(Icons.bug_report_outlined, size: _metrics.scaledSpacing(16), color: titleColor),
                SizedBox(width: _metrics.scaledSpacing(6)),
                Expanded(
                  child: Text(
                    "Debug Trace Timeline",
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: titleColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w700),
                  ),
                ),
                Text(
                  "${trace.events.length} events, persisted ${trace.estimatedPersistedTokens} / runtime ${trace.estimatedRuntimeTokens} tokens",
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: subTitleColor, fontSize: _metrics.smallLabelFontSize),
                ),
              ],
            ),
          ),
          Expanded(child: SingleChildScrollView(child: Column(children: [_buildDebugSection("Timeline", trace.events.map((e) => e.toJson()).toList())]))),
        ],
      ),
    );
  }

  Widget _buildDebugSection(String title, Object payload) {
    final textColor = safeFromCssColor(woxTheme.resultItemTitleColor);
    final subTextColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);
    final jsonText = const JsonEncoder.withIndent('  ').convert(payload);

    return Theme(
      data: ThemeData(dividerColor: Colors.transparent),
      child: ExpansionTile(
        initiallyExpanded: title == "Timeline",
        tilePadding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(10)),
        childrenPadding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), 0, _metrics.scaledSpacing(10), _metrics.scaledSpacing(8)),
        collapsedIconColor: subTextColor,
        iconColor: subTextColor,
        title: Row(
          children: [
            Expanded(
              child: Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(color: textColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w700),
              ),
            ),
            WoxTooltip(
              message: "Copy section",
              child: IconButton(
                onPressed: () => controller.copyDebugSectionContent(jsonText),
                icon: const Icon(Icons.copy),
                iconSize: _metrics.scaledSpacing(14),
                color: subTextColor,
                padding: EdgeInsets.zero,
                constraints: BoxConstraints.tightFor(width: _metrics.scaledSpacing(28), height: _metrics.scaledSpacing(28)),
                splashRadius: _metrics.scaledSpacing(14),
                visualDensity: VisualDensity.compact,
              ),
            ),
          ],
        ),
        children: [
          Container(
            width: double.infinity,
            padding: EdgeInsets.all(_metrics.scaledSpacing(8)),
            decoration: BoxDecoration(color: Colors.black.withAlpha(20), borderRadius: BorderRadius.circular(6)),
            child: WoxSelectableText(jsonText, style: TextStyle(fontSize: _metrics.smallLabelFontSize, fontFamily: 'monospace', color: textColor, height: 1.25)),
          ),
        ],
      ),
    );
  }

  Widget _buildChatInputArea() {
    return WoxPlatformFocus(
      onKeyEvent: _handleChatInputKeyEvent,
      child: Padding(
        padding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), _metrics.scaledSpacing(6), _metrics.scaledSpacing(10), _metrics.scaledSpacing(8)),
        child: _ChatCommandPaletteOverlay(controller: controller, paletteBuilder: (context, maxHeight) => _buildCommandPalette(maxHeight), child: _buildChatInputBox()),
      ),
    );
  }

  Widget _buildChatInputBox() {
    return Container(
      decoration: BoxDecoration(
        color: safeFromCssColor(woxTheme.queryBoxBackgroundColor),
        borderRadius: BorderRadius.circular(woxTheme.queryBoxBorderRadius.toDouble()),
        border: Border.all(color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          _buildDraftSkillRefs(),
          TextField(
            controller: controller.textController,
            focusNode: controller.aiChatFocusNode,
            decoration: InputDecoration(
              hintText: tr('ui_ai_chat_input_hint'),
              hintStyle: TextStyle(color: safeFromCssColor(woxTheme.previewPropertyTitleColor)),
              border: InputBorder.none,
              contentPadding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(14), vertical: _metrics.scaledSpacing(8)),
            ),
            minLines: 1,
            maxLines: 4,
            keyboardType: TextInputType.multiline,
            cursorColor: safeFromCssColor(woxTheme.queryBoxCursorColor),
            // AI chat lives in the launcher preview surface, so its controls
            // follow density metrics while settings controls keep their own sizes.
            style: TextStyle(fontSize: _metrics.resultSubtitleFontSize, color: safeFromCssColor(woxTheme.queryBoxFontColor)),
          ),
          Container(
            height: _metrics.scaledSpacing(42),
            padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(8)),
            decoration: BoxDecoration(border: Border(top: BorderSide(color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(25)))),
            child: Row(
              children: [
                Obx(
                  () => _buildChatStatusChip(
                    Icons.model_training_rounded,
                    controller.aiChatData.value.model.value.name.isEmpty ? tr("ui_ai_chat_select_model") : controller.aiChatData.value.model.value.name,
                  ),
                ),
                SizedBox(width: _metrics.scaledSpacing(8)),
                const Spacer(),
                Obx(() {
                  final generating = controller.isGenerating.value;
                  return TextButton.icon(
                    onPressed: () => generating ? controller.stopChat() : controller.sendMessage(),
                    style: TextButton.styleFrom(
                      minimumSize: Size(_metrics.scaledSpacing(82), _metrics.scaledSpacing(34)),
                      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(14)),
                      tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      visualDensity: VisualDensity.compact,
                      backgroundColor: generating ? safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(30) : safeFromCssColor(woxTheme.actionItemActiveBackgroundColor),
                      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(6)),
                    ),
                    icon: Icon(
                      generating ? Icons.stop_rounded : Icons.keyboard_return,
                      size: _metrics.scaledSpacing(18),
                      color: generating ? safeFromCssColor(woxTheme.resultItemTitleColor) : safeFromCssColor(woxTheme.actionItemActiveFontColor),
                    ),
                    label: Text(
                      generating ? tr('ui_ai_chat_stop') : tr('ui_ai_chat_send'),
                      style: TextStyle(
                        fontSize: _metrics.resultSubtitleFontSize,
                        fontWeight: FontWeight.w700,
                        color: generating ? safeFromCssColor(woxTheme.resultItemTitleColor) : safeFromCssColor(woxTheme.actionItemActiveFontColor),
                      ),
                    ),
                  );
                }),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDraftSkillRefs() {
    return Obx(() {
      final refs = controller.draftSkillRefs.toList();
      if (refs.isEmpty) {
        return const SizedBox.shrink();
      }

      return Container(
        width: double.infinity,
        padding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), _metrics.scaledSpacing(8), _metrics.scaledSpacing(10), _metrics.scaledSpacing(2)),
        child: SingleChildScrollView(scrollDirection: Axis.horizontal, child: Row(children: refs.map((ref) => _buildSkillRefChip(ref, removable: true)).toList())),
      );
    });
  }

  Widget _buildSkillRefChip(AISkillRef ref, {required bool removable}) {
    final borderColor = safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(40);
    final backgroundColor = safeFromCssColor(woxTheme.actionContainerBackgroundColor).withAlpha(80);
    final textColor = safeFromCssColor(woxTheme.queryBoxFontColor);
    final subTextColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);

    return Container(
      margin: EdgeInsets.only(right: _metrics.scaledSpacing(6), bottom: _metrics.scaledSpacing(4)),
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(7), vertical: _metrics.scaledSpacing(4)),
      decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(5), border: Border.all(color: borderColor)),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.extension_rounded, size: _metrics.scaledSpacing(13), color: subTextColor),
          SizedBox(width: _metrics.scaledSpacing(5)),
          ConstrainedBox(
            constraints: BoxConstraints(maxWidth: _metrics.scaledSpacing(180)),
            child: Text(
              ref.name,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: textColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w600),
            ),
          ),
          if (removable) ...[
            SizedBox(width: _metrics.scaledSpacing(5)),
            InkWell(
              onTap: () => controller.removeDraftSkillRef(ref),
              borderRadius: BorderRadius.circular(8),
              child: Icon(Icons.close_rounded, size: _metrics.scaledSpacing(13), color: subTextColor),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildChatStatusChip(IconData icon, String text) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: _metrics.scaledSpacing(16), color: getThemeTextColor().withAlpha(180)),
        SizedBox(width: _metrics.scaledSpacing(5)),
        ConstrainedBox(
          constraints: BoxConstraints(maxWidth: _metrics.scaledSpacing(220)),
          child: Text(text, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: _metrics.smallLabelFontSize)),
        ),
      ],
    );
  }

  Widget _buildCommandPalette(double maxHeight) {
    return Obx(() {
      if (!controller.isCommandPaletteVisible.value) {
        return const SizedBox.shrink();
      }

      final items = controller.commandPaletteItems.toList();
      controller.updateCommandPaletteLayoutMetrics(
        itemHeight: _commandPaletteItemHeight,
        headerHeight: _commandPaletteHeaderHeight,
        verticalPadding: _commandPaletteVerticalPadding,
      );

      final children = <Widget>[];
      ChatCommandPaletteGroup? currentGroup;
      for (var i = 0; i < items.length; i++) {
        final item = items[i];
        if (currentGroup != item.group) {
          currentGroup = item.group;
          children.add(_buildCommandPaletteGroupHeader(item.group));
        }
        children.add(_buildCommandPaletteItem(item, i));
      }

      return Material(
        elevation: 10,
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(10),
        child: Container(
          width: double.infinity,
          decoration: BoxDecoration(
            color: safeFromCssColor(woxTheme.actionContainerBackgroundColor),
            borderRadius: BorderRadius.circular(10),
            border: Border.all(color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(35)),
          ),
          child: ConstrainedBox(
            constraints: BoxConstraints(maxHeight: maxHeight),
            child:
                items.isEmpty
                    ? Padding(
                      padding: EdgeInsets.all(_metrics.scaledSpacing(14)),
                      child: Text(tr("ui_no_data"), style: TextStyle(color: safeFromCssColor(woxTheme.resultItemSubTitleColor), fontSize: _metrics.resultSubtitleFontSize)),
                    )
                    : Scrollbar(
                      thumbVisibility: true,
                      controller: controller.commandPaletteScrollController,
                      child: Listener(
                        onPointerSignal: _handleCommandPalettePointerSignal,
                        onPointerPanZoomUpdate: _handleCommandPalettePointerPanZoomUpdate,
                        child: ListView(
                          controller: controller.commandPaletteScrollController,
                          padding: EdgeInsets.symmetric(vertical: _commandPaletteVerticalPadding),
                          primary: false,
                          shrinkWrap: true,
                          physics: const NeverScrollableScrollPhysics(),
                          children: children,
                        ),
                      ),
                    ),
          ),
        ),
      );
    });
  }

  void _scrollCommandPaletteByPointerDelta(double deltaY) {
    if (!controller.commandPaletteScrollController.hasClients) {
      return;
    }

    final position = controller.commandPaletteScrollController.position;
    final targetOffset = (position.pixels + deltaY).clamp(position.minScrollExtent, position.maxScrollExtent).toDouble();
    if ((targetOffset - position.pixels).abs() < 0.01) {
      return;
    }

    // The command palette is rendered in an overlay, so route pointer scrolling
    // directly to its controller instead of relying on ambient scroll handling.
    controller.commandPaletteScrollController.jumpTo(targetOffset);
  }

  void _handleCommandPalettePointerSignal(PointerSignalEvent event) {
    if (event is PointerScrollEvent) {
      _scrollCommandPaletteByPointerDelta(event.scrollDelta.dy);
    }
  }

  void _handleCommandPalettePointerPanZoomUpdate(PointerPanZoomUpdateEvent event) {
    _scrollCommandPaletteByPointerDelta(-event.panDelta.dy);
  }

  Widget _buildCommandPaletteGroupHeader(ChatCommandPaletteGroup group) {
    final label = group == ChatCommandPaletteGroup.model ? tr("ui_ai_chat_select_model_title") : tr("ui_ai_skills");
    return SizedBox(
      height: _commandPaletteHeaderHeight,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(14)),
        child: Align(
          alignment: Alignment.centerLeft,
          child: Text(label, style: TextStyle(color: safeFromCssColor(woxTheme.resultItemSubTitleColor), fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w700)),
        ),
      ),
    );
  }

  Widget _buildCommandPaletteItem(ChatCommandPaletteItem item, int index) {
    final isActive = controller.commandPaletteSelectedIndex.value == index;
    final icon = item.group == ChatCommandPaletteGroup.model ? Icons.model_training_rounded : Icons.extension_rounded;
    final titleColor = isActive ? safeFromCssColor(woxTheme.resultItemActiveTitleColor) : safeFromCssColor(woxTheme.resultItemTitleColor);
    final subTitleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);
    final backgroundColor = isActive ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor) : Colors.transparent;

    return InkWell(
      onTap: () => controller.executeCommandPaletteItem(item),
      child: Container(
        height: _commandPaletteItemHeight,
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(14)),
        color: backgroundColor,
        child: Row(
          children: [
            Icon(icon, size: _metrics.scaledSpacing(18), color: titleColor),
            SizedBox(width: _metrics.scaledSpacing(10)),
            Expanded(
              child: Row(
                children: [
                  Flexible(
                    child: Text(
                      item.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: titleColor, fontSize: _metrics.resultSubtitleFontSize, fontWeight: FontWeight.w600),
                    ),
                  ),
                  if (item.subTitle.isNotEmpty) ...[
                    SizedBox(width: _metrics.scaledSpacing(8)),
                    Expanded(
                      child: Text(item.subTitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: subTitleColor, fontSize: _metrics.smallLabelFontSize)),
                    ),
                  ],
                ],
              ),
            ),
            if (item.selected) Icon(Icons.check_rounded, size: _metrics.scaledSpacing(18), color: titleColor),
          ],
        ),
      ),
    );
  }

  KeyEventResult _handleChatInputKeyEvent(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent) {
      switch (event.logicalKey) {
        case LogicalKeyboardKey.escape:
          if (controller.handleCommandPaletteEscape()) {
            return KeyEventResult.handled;
          }
          controller.launcherController.exitChatInputMode(const UuidV4().generate());
          return KeyEventResult.handled;
        case LogicalKeyboardKey.enter:
          if (controller.executeSelectedCommandPaletteItem()) {
            return KeyEventResult.handled;
          }
          if (controller.isGenerating.value) {
            controller.stopChat();
          } else {
            controller.sendMessage();
          }
          return KeyEventResult.handled;
        case LogicalKeyboardKey.arrowDown:
          controller.moveCommandPaletteSelection(1);
          return controller.isCommandPaletteVisible.value ? KeyEventResult.handled : KeyEventResult.ignored;
        case LogicalKeyboardKey.arrowUp:
          controller.moveCommandPaletteSelection(-1);
          return controller.isCommandPaletteVisible.value ? KeyEventResult.handled : KeyEventResult.ignored;
      }
    }

    var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
    if (pressedHotkey == null) {
      return KeyEventResult.ignored;
    }

    if (controller.launcherController.isActionHotkey(pressedHotkey)) {
      controller.openCommandPaletteFromActionHotkey();
      return KeyEventResult.handled;
    }

    if (_isToggleConversationSidebarHotkey(pressedHotkey)) {
      controller.toggleConversationSidebar(const UuidV4().generate());
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

  bool _isToggleConversationSidebarHotkey(HotKey hotkey) {
    final parsed = WoxHotkey.parseHotkeyFromString(controller.launcherController.previewFullscreenHotkey);
    return parsed != null && parsed.isNormalHotkey && WoxHotkey.equals(parsed.normalHotkey, hotkey);
  }

  Widget _buildConversationSidebar() {
    final sidebarColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);
    return SizedBox(
      width: _metrics.scaledSpacing(260),
      child: Obx(() {
        final groupedChats = _groupChats(controller.chats);
        return ListView(
          physics: const ClampingScrollPhysics(),
          padding: EdgeInsets.fromLTRB(_metrics.scaledSpacing(10), _metrics.scaledSpacing(12), _metrics.scaledSpacing(10), _metrics.scaledSpacing(12)),
          children: [
            // New chat button at the top.
            _buildNewChatButton(sidebarColor),
            SizedBox(height: _metrics.scaledSpacing(8)),
            // Date-grouped chat history below.
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

  Widget _buildNewChatButton(Color subtitleColor) {
    final isActiveDraft = controller.aiChatData.value.conversations.isEmpty && controller.chats.every((chat) => chat.id != controller.aiChatData.value.id);
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: controller.startNewChat,
      child: Container(
        width: double.infinity,
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(10)),
        decoration: BoxDecoration(
          color: isActiveDraft ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor) : safeFromCssColor(woxTheme.actionItemActiveBackgroundColor).withAlpha(30),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(
          children: [
            Icon(Icons.add_rounded, size: _metrics.scaledSpacing(18), color: safeFromCssColor(woxTheme.actionItemActiveFontColor).withAlpha(200)),
            SizedBox(width: _metrics.scaledSpacing(8)),
            Text(
              tr("ui_ai_chat_new_chat"),
              style: TextStyle(color: safeFromCssColor(woxTheme.previewFontColor), fontSize: _metrics.resultSubtitleFontSize, fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildConversationTile(WoxAIChatData chat) {
    return _buildConversationTileShell(
      title: chat.title.isEmpty ? tr("ui_ai_chat_new_chat") : chat.title,
      subtitle: _getConversationSubtitle(chat),
      active: chat.id == controller.aiChatData.value.id,
      onTap: () => controller.selectChat(chat),
      subtitleColor: safeFromCssColor(woxTheme.resultItemSubTitleColor),
      trailing: _buildConversationTileAction(tooltip: tr("ui_ai_chat_delete_chat"), icon: Icons.delete_outline_rounded, onPressed: () => controller.deleteChat(chat)),
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
  // When [showButtons] is false, only the text field is rendered (used when
  // embedded inside the options panel which already has its own buttons).
  Widget _buildAIQuestionFreeTextAnswer(Color textColor, Color subTextColor, {bool showButtons = true}) {
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
        if (showButtons) ...[
          SizedBox(height: _metrics.scaledSpacing(12)),
          _buildAIQuestionActions(submitButton: _buildAIQuestionButton(label: tr("ui_ai_question_submit"), onTap: controller.submitPendingAIQuestionAnswer, primary: true)),
        ],
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
            itemBuilder: (_, index) => _buildAIQuestionOptionTile(question, question.options[index], textColor, subTextColor),
          ),
        ),
        // Show text input when the free-text (last) option is selected.
        Obx(
          () =>
              controller.isAIQuestionFreeTextSelected()
                  ? Padding(padding: EdgeInsets.only(top: _metrics.scaledSpacing(8)), child: _buildAIQuestionFreeTextAnswer(textColor, subTextColor, showButtons: false))
                  : const SizedBox.shrink(),
        ),
        SizedBox(height: _metrics.scaledSpacing(12)),
        _buildAIQuestionActions(
          submitButton: Obx(
            () => _buildAIQuestionButton(
              label: tr("ui_ai_question_submit"),
              onTap: controller.submitSelectedAIQuestionAnswer,
              primary: true,
              enabled: controller.selectedAIQuestionOption.value != null,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildAIQuestionOptionTile(AIQuestion question, AIQuestionOption option, Color textColor, Color subTextColor) {
    return Obx(() {
      final isSelected = controller.selectedAIQuestionOption.value?.value == option.value;
      return GestureDetector(
        onTap: () => controller.selectAIQuestionOption(option),
        child: Container(
          width: double.infinity,
          padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(10)),
          decoration: BoxDecoration(
            color: isSelected ? safeFromCssColor(woxTheme.actionItemActiveBackgroundColor).withAlpha(50) : safeFromCssColor(woxTheme.queryBoxBackgroundColor),
            borderRadius: BorderRadius.circular(6),
            border: Border.all(color: isSelected ? safeFromCssColor(woxTheme.actionItemActiveBackgroundColor) : subTextColor.withAlpha(45), width: isSelected ? 1.5 : 1),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                isSelected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                size: _metrics.scaledSpacing(16),
                color: isSelected ? safeFromCssColor(woxTheme.actionItemActiveBackgroundColor) : subTextColor.withAlpha(80),
              ),
              SizedBox(width: _metrics.scaledSpacing(8)),
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
                      Text(
                        option.subTitle,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(color: subTextColor, fontSize: _metrics.smallLabelFontSize, height: 1.25),
                      ),
                    ],
                  ],
                ),
              ),
              if (option.recommended) ...[
                SizedBox(width: _metrics.scaledSpacing(10)),
                Container(
                  padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(6), vertical: _metrics.scaledSpacing(3)),
                  decoration: BoxDecoration(color: safeFromCssColor(woxTheme.actionItemActiveBackgroundColor), borderRadius: BorderRadius.circular(4)),
                  child: Text(
                    tr("ui_ai_question_recommended"),
                    style: TextStyle(color: safeFromCssColor(woxTheme.actionItemActiveFontColor), fontSize: _metrics.smallLabelFontSize),
                  ),
                ),
              ],
            ],
          ),
        ),
      );
    });
  }

  Widget _buildAIQuestionActions({required Widget submitButton}) {
    return Row(
      children: [
        const Spacer(),
        _buildAIQuestionButton(label: tr("ui_cancel"), onTap: controller.cancelPendingAIQuestion, primary: false),
        SizedBox(width: _metrics.scaledSpacing(8)),
        submitButton,
      ],
    );
  }

  Widget _buildAIQuestionButton({required String label, required VoidCallback onTap, required bool primary, bool enabled = true}) {
    final backgroundColor = primary ? safeFromCssColor(woxTheme.actionItemActiveBackgroundColor) : safeFromCssColor(woxTheme.queryBoxBackgroundColor);
    final textColor = primary ? safeFromCssColor(woxTheme.actionItemActiveFontColor) : safeFromCssColor(woxTheme.queryBoxFontColor);
    final borderColor = safeFromCssColor(woxTheme.resultItemSubTitleColor).withAlpha(70);
    final effectiveBackgroundColor = enabled ? backgroundColor : backgroundColor.withAlpha(50);
    final effectiveTextColor = enabled ? textColor : textColor.withAlpha(80);

    return GestureDetector(
      onTap: enabled ? onTap : null,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(7)),
        decoration: BoxDecoration(
          color: effectiveBackgroundColor,
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: primary ? effectiveBackgroundColor : borderColor),
        ),
        child: Text(label, style: TextStyle(color: effectiveTextColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w700)),
      ),
    );
  }

  List<_ChatRenderItem> _buildChatRenderItems(List<WoxAIChatConversation> conversations) {
    final items = <_ChatRenderItem>[];
    final pendingTools = <WoxAIChatConversation>[];

    void flushToolActivity() {
      if (pendingTools.isEmpty) {
        return;
      }
      items.add(_ChatToolActivityRenderItem(List<WoxAIChatConversation>.unmodifiable(pendingTools)));
      pendingTools.clear();
    }

    for (final conversation in conversations) {
      if (conversation.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_SYSTEM.value) {
        continue;
      }

      if (_isToolConversation(conversation)) {
        pendingTools.add(conversation);
        continue;
      }

      flushToolActivity();
      items.add(_ChatMessageRenderItem(conversation));
    }

    flushToolActivity();
    return items;
  }

  Widget _buildChatRenderItem(_ChatRenderItem item, BuildContext context) {
    if (item is _ChatMessageRenderItem) {
      return _buildMessageItem(item.message, context);
    }

    if (item is _ChatToolActivityRenderItem) {
      return _buildToolActivityItem(item);
    }

    return const SizedBox.shrink();
  }

  bool _isToolConversation(WoxAIChatConversation conversation) {
    return conversation.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_TOOL.value;
  }

  Widget _buildMessageItem(WoxAIChatConversation message, BuildContext context) {
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;
    final isAssistant = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_ASSISTANT.value;

    if (isUser) {
      return _buildUserMessageItem(message, context);
    }

    if (isAssistant) {
      return _buildAssistantMessageItem(message);
    }

    return const SizedBox.shrink();
  }

  // Renders user messages without an avatar so the content column keeps more usable width.
  Widget _buildUserMessageItem(WoxAIChatConversation message, BuildContext context) {
    final fontColor = safeFromCssColor(woxTheme.resultItemActiveTitleColor);
    var isHovered = false;

    return StatefulBuilder(
      builder: (context, setHoverState) {
        return MouseRegion(
          onEnter: (_) => setHoverState(() => isHovered = true),
          onExit: (_) => setHoverState(() => isHovered = false),
          child: Padding(
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
                        if (message.skillRefs.isNotEmpty) ...[
                          Align(
                            alignment: Alignment.centerRight,
                            child: Wrap(alignment: WrapAlignment.end, children: message.skillRefs.map((ref) => _buildSkillRefChip(ref, removable: false)).toList()),
                          ),
                          SizedBox(height: _metrics.scaledSpacing(3)),
                        ],
                        Container(
                          margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(3)),
                          padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(8)),
                          decoration: BoxDecoration(color: safeFromCssColor(woxTheme.resultItemActiveBackgroundColor), borderRadius: BorderRadius.circular(8)),
                          child: _buildMessageContent(message, fontColor),
                        ),
                        _buildHoverVisibleMessageMetaRow(message: message, isUser: true, visible: isHovered),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        );
      },
    );
  }

  // Renders assistant messages as a full-width reading column.
  Widget _buildAssistantMessageItem(WoxAIChatConversation message) {
    final fontColor = safeFromCssColor(woxTheme.resultItemTitleColor);
    var isHovered = false;

    return StatefulBuilder(
      builder: (context, setHoverState) {
        return MouseRegion(
          onEnter: (_) => setHoverState(() => isHovered = true),
          onExit: (_) => setHoverState(() => isHovered = false),
          child: Padding(
            padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(3)),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  width: double.infinity,
                  margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(3)),
                  padding: EdgeInsets.only(top: _metrics.scaledSpacing(1), right: _metrics.scaledSpacing(4)),
                  child: _buildMessageContent(message, fontColor),
                ),
                _buildHoverVisibleMessageMetaRow(message: message, isUser: false, visible: isHovered),
              ],
            ),
          ),
        );
      },
    );
  }

  // Renders the shared text and image payload for visible chat messages.
  Widget _buildMessageContent(WoxAIChatConversation message, Color fontColor) {
    final hasReasoning = message.reasoning.trim().isNotEmpty;
    final hasText = message.text.trim().isNotEmpty;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Reasoning is displayed as a visually de-emphasized block above the
        // main answer: smaller font, lighter color, italic style.
        if (hasReasoning)
          Padding(
            padding: EdgeInsets.only(bottom: hasText ? _metrics.scaledSpacing(6) : 0),
            child: WoxSelectableText(message.reasoning.trim(), style: TextStyle(fontSize: _metrics.smallLabelFontSize, height: 1.4, color: fontColor.withAlpha(120))),
          ),
        if (hasText) WoxMarkdownView(data: message.text, fontColor: fontColor, fontSize: _metrics.resultSubtitleFontSize),
        if (message.images.isNotEmpty) ...[
          if (hasText) SizedBox(height: _metrics.scaledSpacing(8)),
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

  // Keeps the metadata row reserved while hiding actions until hover.
  Widget _buildHoverVisibleMessageMetaRow({required WoxAIChatConversation message, required bool isUser, required bool visible}) {
    return IgnorePointer(
      ignoring: !visible,
      child: ExcludeSemantics(
        excluding: !visible,
        child: AnimatedOpacity(opacity: visible ? 1 : 0, duration: const Duration(milliseconds: 80), curve: Curves.easeOut, child: _buildMessageMetaRow(message, isUser)),
      ),
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
            Text(controller.formatTimestamp(message.timestamp), style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: metaColor)),
            SizedBox(width: _metrics.scaledSpacing(10)),
            Text("•", style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: metaColor)),
            SizedBox(width: _metrics.scaledSpacing(10)),
            _buildInlineActionButtons(message, true),
          ],
        ],
      ),
    );
  }

  Widget _buildToolActivityItem(_ChatToolActivityRenderItem item) {
    if (item.tools.isEmpty) {
      return const SizedBox.shrink();
    }

    final titleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(3)),
      child: Obx(() {
        final expanded = controller.isToolActivityExpanded(item.id);
        final status = _toolActivityStatus(item.tools);

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: () => controller.toggleToolActivityExpanded(item.id),
              child: Padding(
                padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(2), vertical: _metrics.scaledSpacing(6)),
                child: Row(
                  children: [
                    Icon(_toolActivityLeadingIcon(item.tools), size: _metrics.scaledSpacing(16), color: status == ToolCallStatus.failed ? Colors.red : titleColor),
                    SizedBox(width: _metrics.scaledSpacing(8)),
                    Flexible(
                      child: Text(
                        _buildToolActivitySummary(item.tools, status),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: titleColor, fontWeight: FontWeight.w600),
                      ),
                    ),
                    if (status == ToolCallStatus.failed) ...[SizedBox(width: _metrics.scaledSpacing(8)), _buildToolStatusIndicator(status, _toolActivityStatusLabel(status))],
                    SizedBox(width: _metrics.scaledSpacing(6)),
                    Icon(expanded ? Icons.keyboard_arrow_down : Icons.keyboard_arrow_right, size: _metrics.scaledSpacing(16), color: titleColor),
                  ],
                ),
              ),
            ),
            if (expanded)
              Padding(
                padding: EdgeInsets.only(left: _metrics.scaledSpacing(24), top: _metrics.scaledSpacing(2), bottom: _metrics.scaledSpacing(4)),
                child: Column(children: [for (final tool in item.tools) Padding(padding: EdgeInsets.only(bottom: _metrics.scaledSpacing(6)), child: _buildToolCallBadge(tool))]),
              ),
          ],
        );
      }),
    );
  }

  Widget _buildToolCallBadge(WoxAIChatConversation message) {
    final subtitleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);
    final startTimestamp = _toolCallStartTimestamp(message);
    final endTimestamp = _toolCallEndTimestamp(message.toolCallInfo, startTimestamp);
    final toolName = message.toolCallInfo.name.isEmpty ? tr("ui_ai_chat_tools") : message.toolCallInfo.name;

    return Obx(() {
      final expanded = controller.isToolCallExpanded(message.id);
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () => controller.toggleToolCallExpanded(message.id),
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(2), vertical: _metrics.scaledSpacing(6)),
              child: Row(
                children: [
                  Icon(Icons.build_outlined, size: _metrics.scaledSpacing(16), color: subtitleColor),
                  SizedBox(width: _metrics.scaledSpacing(8)),
                  Flexible(
                    child: Text(
                      toolName,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: subtitleColor, fontWeight: FontWeight.w600),
                    ),
                  ),
                  SizedBox(width: _metrics.scaledSpacing(8)),
                  WoxChatToolcallDuration(
                    id: message.id,
                    startTimestamp: startTimestamp,
                    endTimestamp: endTimestamp,
                    style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: subtitleColor),
                  ),
                  SizedBox(width: _metrics.scaledSpacing(8)),
                  _buildStatusIndicator(message.toolCallInfo),
                  SizedBox(width: _metrics.scaledSpacing(6)),
                  Icon(expanded ? Icons.keyboard_arrow_down : Icons.keyboard_arrow_right, size: _metrics.scaledSpacing(16), color: subtitleColor),
                ],
              ),
            ),
          ),
          if (expanded)
            Padding(
              padding: EdgeInsets.only(left: _metrics.scaledSpacing(24), top: _metrics.scaledSpacing(2), bottom: _metrics.scaledSpacing(4)),
              child: _buildToolCallDetails(message.toolCallInfo),
            ),
        ],
      );
    });
  }

  Widget _buildStatusIndicator(ToolCallInfo info) {
    return _buildToolStatusIndicator(info.status, _toolStatusTooltip(info.status, info.response));
  }

  Widget _buildToolStatusIndicator(ToolCallStatus status, String tooltip) {
    // Tool-call status hints are hover-only metadata, so use WoxTooltip to keep
    // chat details consistent with launcher and settings tooltip overlays.
    return WoxTooltip(message: tooltip, child: Icon(_toolStatusIcon(status), size: _metrics.scaledSpacing(14), color: _toolStatusColor(status)));
  }

  IconData _toolStatusIcon(ToolCallStatus status) {
    switch (status) {
      case ToolCallStatus.streaming:
        return Icons.play_arrow;
      case ToolCallStatus.pending:
        return Icons.hourglass_empty;
      case ToolCallStatus.running:
        return Icons.refresh;
      case ToolCallStatus.succeeded:
        return Icons.check_circle;
      case ToolCallStatus.failed:
        return Icons.error;
    }
  }

  Color _toolStatusColor(ToolCallStatus status) {
    switch (status) {
      case ToolCallStatus.streaming:
      case ToolCallStatus.running:
        return Colors.blue;
      case ToolCallStatus.pending:
        return Colors.grey;
      case ToolCallStatus.succeeded:
        return Colors.green;
      case ToolCallStatus.failed:
        return Colors.red;
    }
  }

  String _toolStatusTooltip(ToolCallStatus status, String response) {
    switch (status) {
      case ToolCallStatus.streaming:
        return tr('ui_ai_chat_tool_status_streaming');
      case ToolCallStatus.pending:
        return tr('ui_ai_chat_tool_status_pending');
      case ToolCallStatus.running:
        return tr('ui_ai_chat_tool_status_running');
      case ToolCallStatus.succeeded:
        return tr('ui_ai_chat_tool_status_succeeded');
      case ToolCallStatus.failed:
        return Strings.format(tr('ui_ai_chat_tool_status_failed'), [response.isEmpty ? tr('ui_ai_chat_tool_activity_status_failed') : response]);
    }
  }

  ToolCallStatus _toolActivityStatus(List<WoxAIChatConversation> tools) {
    var hasStreaming = false;
    var hasPending = false;
    var hasRunning = false;

    for (final tool in tools) {
      switch (tool.toolCallInfo.status) {
        case ToolCallStatus.failed:
          return ToolCallStatus.failed;
        case ToolCallStatus.running:
          hasRunning = true;
          break;
        case ToolCallStatus.streaming:
          hasStreaming = true;
          break;
        case ToolCallStatus.pending:
          hasPending = true;
          break;
        case ToolCallStatus.succeeded:
          break;
      }
    }

    if (hasRunning) {
      return ToolCallStatus.running;
    }
    if (hasStreaming) {
      return ToolCallStatus.streaming;
    }
    if (hasPending) {
      return ToolCallStatus.pending;
    }
    return ToolCallStatus.succeeded;
  }

  String _buildToolActivitySummary(List<WoxAIChatConversation> tools, ToolCallStatus status) {
    final seenActions = <String>{};
    final actions = <String>[];
    for (final tool in tools) {
      final action = _toolActionLabel(tool.toolCallInfo.name);
      if (seenActions.add(action)) {
        actions.add(action);
      }
    }

    final separator = tr('ui_ai_chat_tool_activity_action_separator');
    final actionText = actions.isEmpty ? tr('ui_ai_chat_tools') : actions.join(separator);
    final countText = tools.length == 1 ? tr('ui_ai_chat_tool_activity_count_one') : Strings.format(tr('ui_ai_chat_tool_activity_count_many'), [tools.length.toString()]);
    return [_toolActivityStatusLabel(status), actionText, countText].where((part) => part.isNotEmpty).join(' · ');
  }

  String _toolActionLabel(String name) {
    switch (name) {
      case 'web_search':
        return tr('ui_ai_chat_tool_action_web_search');
      case 'web_fetch':
        return tr('ui_ai_chat_tool_action_web_fetch');
      case 'read_skill':
        return tr('ui_ai_chat_tool_action_read_skill');
      case 'load_tools':
        return tr('ui_ai_chat_tool_action_load_tools');
      default:
        return name.isEmpty ? tr('ui_ai_chat_tools') : name;
    }
  }

  String _toolActivityStatusLabel(ToolCallStatus status) {
    switch (status) {
      case ToolCallStatus.streaming:
        return tr('ui_ai_chat_tool_activity_status_streaming');
      case ToolCallStatus.pending:
        return tr('ui_ai_chat_tool_activity_status_pending');
      case ToolCallStatus.running:
        return tr('ui_ai_chat_tool_activity_status_running');
      case ToolCallStatus.succeeded:
        return tr('ui_ai_chat_tool_activity_status_succeeded');
      case ToolCallStatus.failed:
        return tr('ui_ai_chat_tool_activity_status_failed');
    }
  }

  IconData _toolActivityLeadingIcon(List<WoxAIChatConversation> tools) {
    if (tools.any((tool) => tool.toolCallInfo.name == 'web_search')) {
      return Icons.search;
    }
    if (tools.any((tool) => tool.toolCallInfo.name == 'web_fetch')) {
      return Icons.article_outlined;
    }
    if (tools.any((tool) => tool.toolCallInfo.name == 'load_tools')) {
      return Icons.extension_outlined;
    }
    return Icons.terminal_rounded;
  }

  int _toolCallStartTimestamp(WoxAIChatConversation message) {
    if (message.toolCallInfo.startTimestamp > 0) {
      return message.toolCallInfo.startTimestamp;
    }
    if (message.timestamp > 0) {
      return message.timestamp;
    }
    return DateTime.now().millisecondsSinceEpoch;
  }

  int? _toolCallEndTimestamp(ToolCallInfo info, int startTimestamp) {
    if (_isActiveToolStatus(info.status)) {
      return null;
    }
    final endTimestamp = info.endTimestamp > 0 ? info.endTimestamp : startTimestamp;
    return endTimestamp < startTimestamp ? startTimestamp : endTimestamp;
  }

  bool _isActiveToolStatus(ToolCallStatus status) {
    return status == ToolCallStatus.streaming || status == ToolCallStatus.pending || status == ToolCallStatus.running;
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
}

abstract class _ChatRenderItem {
  const _ChatRenderItem();
}

class _ChatMessageRenderItem extends _ChatRenderItem {
  final WoxAIChatConversation message;

  const _ChatMessageRenderItem(this.message);
}

class _ChatToolActivityRenderItem extends _ChatRenderItem {
  final List<WoxAIChatConversation> tools;

  const _ChatToolActivityRenderItem(this.tools);

  String get id {
    if (tools.isEmpty) {
      return 'tool-activity:empty';
    }
    return 'tool-activity:${tools.first.id}';
  }
}

class _ChatCommandPaletteOverlay extends StatefulWidget {
  final WoxAIChatController controller;
  final Widget child;
  final Widget Function(BuildContext context, double maxHeight) paletteBuilder;

  const _ChatCommandPaletteOverlay({required this.controller, required this.child, required this.paletteBuilder});

  @override
  State<_ChatCommandPaletteOverlay> createState() => _ChatCommandPaletteOverlayState();
}

class _ChatCommandPaletteOverlayState extends State<_ChatCommandPaletteOverlay> {
  final LayerLink _layerLink = LayerLink();
  final GlobalKey _targetKey = GlobalKey();
  Worker? _visibilityWorker;
  OverlayEntry? _overlayEntry;

  @override
  void initState() {
    super.initState();
    _visibilityWorker = ever<bool>(widget.controller.isCommandPaletteVisible, (_) => _syncOverlay());
    WidgetsBinding.instance.addPostFrameCallback((_) => _syncOverlay());
  }

  @override
  void didUpdateWidget(covariant _ChatCommandPaletteOverlay oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.controller == widget.controller) {
      return;
    }

    _visibilityWorker?.dispose();
    _visibilityWorker = ever<bool>(widget.controller.isCommandPaletteVisible, (_) => _syncOverlay());
    _removeOverlay();
    WidgetsBinding.instance.addPostFrameCallback((_) => _syncOverlay());
  }

  @override
  void dispose() {
    _visibilityWorker?.dispose();
    _removeOverlay();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_overlayEntry != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) => _overlayEntry?.markNeedsBuild());
    }

    return CompositedTransformTarget(key: _targetKey, link: _layerLink, child: widget.child);
  }

  void _syncOverlay() {
    if (!mounted) {
      return;
    }

    if (widget.controller.isCommandPaletteVisible.value) {
      _showOverlay();
      return;
    }

    _removeOverlay();
  }

  void _showOverlay() {
    if (_overlayEntry != null) {
      _overlayEntry!.markNeedsBuild();
      return;
    }

    _overlayEntry = OverlayEntry(builder: _buildOverlay);
    Overlay.of(context).insert(_overlayEntry!);
  }

  void _removeOverlay() {
    _overlayEntry?.remove();
    _overlayEntry = null;
  }

  Widget _buildOverlay(BuildContext overlayContext) {
    final renderBox = _targetKey.currentContext?.findRenderObject() as RenderBox?;
    final targetSize = renderBox?.size ?? Size.zero;
    final targetTop = renderBox?.localToGlobal(Offset.zero).dy ?? 0;
    final maxHeight = (targetTop - 12).clamp(96.0, 310.0).toDouble();

    return Positioned.fill(
      child: Stack(
        children: [
          GestureDetector(behavior: HitTestBehavior.translucent, onTap: widget.controller.hideCommandPalette),
          Positioned(
            width: targetSize.width,
            child: CompositedTransformFollower(
              link: _layerLink,
              showWhenUnlinked: false,
              targetAnchor: Alignment.topLeft,
              followerAnchor: Alignment.bottomLeft,
              offset: const Offset(0, -8),
              child: Material(color: Colors.transparent, child: widget.paletteBuilder(overlayContext, maxHeight)),
            ),
          ),
        ],
      ),
    );
  }
}
