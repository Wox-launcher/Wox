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
                icon: Icon(isConversationSidebarCollapsed ? Icons.menu : Icons.close),
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

    return LayoutBuilder(
      builder: (context, constraints) {
        final sidebarWidth = _metrics.scaledSpacing(260);

        final drawerWidth = sidebarWidth + 1;

        return Stack(
          children: [
            Positioned.fill(child: _buildChatConversationPane(context)),
            // Click-outside catcher: only visible while the drawer is open so
            // taps on the chat pane collapse the sidebar the same way zencode
            // dismisses its panel when the editor area is clicked.
            Obx(() {
              final showConversationSidebar = !controller.isConversationSidebarCollapsed.value;
              return showConversationSidebar
                  ? Positioned.fill(child: GestureDetector(behavior: HitTestBehavior.translucent, onTap: () => controller.toggleConversationSidebar(const UuidV4().generate())))
                  : const SizedBox.shrink();
            }),
            Obx(() {
              final showConversationSidebar = !controller.isConversationSidebarCollapsed.value;
              return AnimatedPositioned(
                duration: const Duration(milliseconds: 220),
                curve: Curves.easeOutCubic,
                top: 0,
                bottom: 0,
                left: showConversationSidebar ? 0.0 : -drawerWidth,
                width: drawerWidth,
                child: ColoredBox(
                  color: safeFromCssColor(woxTheme.actionContainerBackgroundColor),
                  child: Row(children: [_buildConversationSidebar(), Container(width: 1, color: safeFromCssColor(woxTheme.previewPropertyTitleColor).withAlpha(20))]),
                ),
              );
            }),
            Obx(() {
              final question = controller.pendingAIQuestion.value;
              return question == null ? const SizedBox.shrink() : _buildAIQuestionOverlay(question);
            }),
          ],
        );
      },
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
                  () =>
                      _buildModelSelectorChip(controller.aiChatData.value.model.value.name.isEmpty ? tr("ui_ai_chat_select_model") : controller.aiChatData.value.model.value.name),
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

  // Model chip below the chat input. Tapping it opens the command palette
  // filtered to models so the user can switch the current chat's model without
  // typing a slash. Hover shows a subtle highlight to signal it's clickable.
  Widget _buildModelSelectorChip(String text) {
    bool isHovered = false;
    return StatefulBuilder(
      builder: (context, setState) {
        return MouseRegion(
          onEnter: (_) => setState(() => isHovered = true),
          onExit: (_) => setState(() => isHovered = false),
          child: InkWell(
            onTap: () => controller.showModelPalette(),
            borderRadius: BorderRadius.circular(4),
            child: Container(
              padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(4), vertical: _metrics.scaledSpacing(2)),
              decoration: BoxDecoration(
                color: isHovered ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor).withAlpha(40) : Colors.transparent,
                borderRadius: BorderRadius.circular(4),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.model_training_rounded, size: _metrics.scaledSpacing(16), color: getThemeTextColor().withAlpha(180)),
                  SizedBox(width: _metrics.scaledSpacing(5)),
                  ConstrainedBox(
                    constraints: BoxConstraints(maxWidth: _metrics.scaledSpacing(220)),
                    child: Text(text, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: _metrics.resultSubtitleFontSize)),
                  ),
                  SizedBox(width: _metrics.scaledSpacing(4)),
                  Icon(Icons.keyboard_arrow_down, size: _metrics.scaledSpacing(14), color: getThemeTextColor().withAlpha(140)),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildCommandPalette(double maxHeight) {
    return Obx(() {
      if (!controller.isCommandPaletteVisible.value) {
        return const SizedBox.shrink();
      }

      final items = controller.commandPaletteItems.toList();
      // Read selectedIndex here (inside the Obx build) so the Obx tracks this
      // reactive dependency. Without this, the .value read only happens inside
      // StatefulBuilder.State.build — which runs *after* Obx.build returns — so
      // the Obx never subscribes to selectedIndex and won't rebuild when it
      // changes, leaving the keyboard highlight stuck.
      final selectedIndex = controller.commandPaletteSelectedIndex.value;
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
        children.add(_buildCommandPaletteItem(item, i, selectedIndex));
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

  Widget _buildCommandPaletteItem(ChatCommandPaletteItem item, int index, int selectedIndex) {
    final icon = item.group == ChatCommandPaletteGroup.model ? Icons.model_training_rounded : Icons.extension_rounded;
    final subTitleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);

    bool isHovered = false;
    return StatefulBuilder(
      builder: (context, setState) {
        return MouseRegion(
          onEnter: (_) => setState(() => isHovered = true),
          onExit: (_) => setState(() => isHovered = false),
          child: InkWell(
            onTap: () => controller.executeCommandPaletteItem(item),
            child: _buildCommandPaletteItemContent(item, index, icon, subTitleColor, isHovered, selectedIndex),
          ),
        );
      },
    );
  }

  Widget _buildCommandPaletteItemContent(ChatCommandPaletteItem item, int index, IconData icon, Color subTitleColor, bool isHovered, int selectedIndex) {
    final isActive = selectedIndex == index;
    final titleColor = isActive ? safeFromCssColor(woxTheme.resultItemActiveTitleColor) : safeFromCssColor(woxTheme.resultItemTitleColor);
    final backgroundColor =
        isActive
            ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor)
            : isHovered
            ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor).withAlpha(40)
            : Colors.transparent;

    return Container(
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
                    style: TextStyle(color: titleColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w600),
                  ),
                ),
                if (item.subTitle.isNotEmpty) ...[
                  SizedBox(width: _metrics.scaledSpacing(8)),
                  Expanded(child: Text(item.subTitle, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: subTitleColor, fontSize: _metrics.smallLabelFontSize))),
                ],
              ],
            ),
          ),
          if (item.selected) Icon(Icons.check_rounded, size: _metrics.scaledSpacing(18), color: titleColor),
        ],
      ),
    );
  }

  KeyEventResult _handleChatInputKeyEvent(FocusNode node, KeyEvent event) {
    if (event is KeyDownEvent) {
      switch (event.logicalKey) {
        case LogicalKeyboardKey.backspace:
          // Delete the entire {skill:xxx} pill when backspace is pressed right
          // after one, instead of deleting character by character.
          if (controller.textController.deleteAdjacentSkillTag()) {
            return KeyEventResult.handled;
          }
          return KeyEventResult.ignored;
        case LogicalKeyboardKey.delete:
          if (controller.textController.deleteAdjacentSkillTag(forward: true)) {
            return KeyEventResult.handled;
          }
          return KeyEventResult.ignored;
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

    // Handle key-repeat for arrow navigation so long-press quickly cycles
    // through command palette items.
    if (event is KeyRepeatEvent) {
      switch (event.logicalKey) {
        case LogicalKeyboardKey.backspace:
          if (controller.textController.deleteAdjacentSkillTag()) {
            return KeyEventResult.handled;
          }
          return KeyEventResult.ignored;
        case LogicalKeyboardKey.delete:
          if (controller.textController.deleteAdjacentSkillTag(forward: true)) {
            return KeyEventResult.handled;
          }
          return KeyEventResult.ignored;
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
    return _NewChatButton(onTap: controller.startNewChat);
  }

  Widget _buildConversationTile(WoxAIChatData chat) {
    return _buildConversationTileShell(
      title: chat.title.isEmpty ? tr("ui_ai_chat_new_chat") : chat.title,
      active: chat.id == controller.aiChatData.value.id,
      onTap: () => controller.selectChat(chat),
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

  Widget _buildConversationTileShell({required String title, required bool active, required VoidCallback onTap, Widget? trailing}) {
    return _ConversationTile(title: title, active: active, onTap: onTap, trailing: trailing);
  }

  ({List<WoxAIChatData> today, List<WoxAIChatData> yesterday, List<WoxAIChatData> history}) _groupChats(List<WoxAIChatData> chats) {
    final now = DateTime.now();
    final todayStart = DateTime(now.year, now.month, now.day);
    final yesterdayStart = todayStart.subtract(const Duration(days: 1));
    final today = <WoxAIChatData>[];
    final yesterday = <WoxAIChatData>[];
    final history = <WoxAIChatData>[];
    for (final chat in chats) {
      if (!chat.isSummary && chat.conversations.isEmpty) {
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

  // Build the flat list of render items by grouping the conversation into
  // user messages and assistant rounds. Each assistant round (the run of
  // assistant/tool messages between two user messages, or from the last user
  // message to the end of conversation) is wrapped in a _ChatRoundRenderItem
  // so the intermediate process can be collapsed once the round completes,
  // leaving only the final assistant reply + toolbar visible.
  List<_ChatRenderItem> _buildChatRenderItems(List<WoxAIChatConversation> conversations) {
    final items = <_ChatRenderItem>[];

    // Per-round accumulator: every conversation (assistant + tool) in order.
    var roundMessages = <WoxAIChatConversation>[];
    var roundStarted = false;

    _ChatRoundRenderItem? buildRoundItem({required bool isComplete}) {
      if (roundMessages.isEmpty) {
        return null;
      }

      // The final assistant reply is only the round's trailing assistant
      // message — i.e. the last message in the round must be an assistant
      // role. If the round ends with a tool call (still generating, tools
      // running after the last assistant chunk) there is no final reply yet
      // and every message renders as intermediate in original order. This
      // preserves chronological order: reasoning → tool calls → answer,
      // instead of promoting an earlier assistant chunk above later tools.
      final lastMsg = roundMessages.last;
      final finalAssistant = lastMsg.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_ASSISTANT.value ? lastMsg : null;
      final lastAssistantIndex = finalAssistant == null ? -1 : roundMessages.length - 1;

      // Rebuild intermediate render items from the non-final conversations,
      // batching consecutive tool calls into tool-activity items.
      final intermediateItems = <_ChatRenderItem>[];
      final pending = <WoxAIChatConversation>[];
      void flushPending() {
        if (pending.isEmpty) {
          return;
        }
        intermediateItems.add(_ChatToolActivityRenderItem(List<WoxAIChatConversation>.unmodifiable(pending)));
        pending.clear();
      }

      for (var i = 0; i < roundMessages.length; i++) {
        if (i == lastAssistantIndex) {
          continue;
        }
        final msg = roundMessages[i];
        if (msg.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_TOOL.value) {
          pending.add(msg);
          continue;
        }
        flushPending();
        intermediateItems.add(_ChatMessageRenderItem(msg));
      }
      flushPending();

      // The final reply's reasoning is part of the collapsible process, but
      // only once the round is complete: while generating the reasoning stays
      // inline with the streaming final reply. When complete it's appended as
      // a trailing intermediate item so it shows only when expanded, and the
      // final reply body renders without reasoning.
      if (isComplete && finalAssistant != null && finalAssistant.reasoning.trim().isNotEmpty) {
        intermediateItems.add(_ChatReasoningRenderItem(finalAssistant));
      }

      final firstAssistantTs = _firstAssistantTimestamp(roundMessages);
      final lastAssistantTs = finalAssistant?.timestamp;
      final roundId = _roundId(roundMessages, finalAssistant);

      return _ChatRoundRenderItem(
        id: roundId,
        intermediateItems: intermediateItems,
        finalAssistantMessage: finalAssistant,
        finalAssistantReasoning: finalAssistant?.reasoning ?? '',
        isComplete: isComplete,
        workedForStart: firstAssistantTs,
        workedForEnd: isComplete ? lastAssistantTs : null,
      );
    }

    void closeRound({required bool isComplete}) {
      if (!roundStarted) {
        return;
      }
      final round = buildRoundItem(isComplete: isComplete);
      if (round != null) {
        items.add(round);
      }
      roundMessages = <WoxAIChatConversation>[];
      roundStarted = false;
    }

    for (final conversation in conversations) {
      if (conversation.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_SYSTEM.value) {
        continue;
      }

      final isUser = conversation.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;
      if (isUser) {
        // A new user message closes the in-progress round (completed, since
        // the user only types after a round finishes) and emits it as a round
        // item before rendering the user message itself.
        closeRound(isComplete: true);
        items.add(_ChatMessageRenderItem(conversation));
        continue;
      }

      // assistant or tool: accumulate into the current round.
      roundStarted = true;
      roundMessages.add(conversation);
    }

    // The trailing round is only "complete" when generation has stopped; while
    // generating it stays open so intermediate process stays visible.
    closeRound(isComplete: !controller.isGenerating.value);

    return items;
  }

  int _firstAssistantTimestamp(List<WoxAIChatConversation> messages) {
    for (final msg in messages) {
      if (msg.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_ASSISTANT.value) {
        return msg.timestamp;
      }
    }
    return 0;
  }

  String _roundId(List<WoxAIChatConversation> messages, WoxAIChatConversation? finalAssistant) {
    final first = messages.isNotEmpty ? messages.first.id : 'empty';
    final last = finalAssistant?.id ?? (messages.isNotEmpty ? messages.last.id : 'empty');
    return 'round:$first:$last';
  }

  Widget _buildChatRenderItem(_ChatRenderItem item, BuildContext context) {
    if (item is _ChatRoundRenderItem) {
      return _buildRoundItem(item, context);
    }

    if (item is _ChatMessageRenderItem) {
      return _buildMessageItem(item.message, context);
    }

    if (item is _ChatToolActivityRenderItem) {
      return _buildToolActivityItem(item);
    }

    if (item is _ChatReasoningRenderItem) {
      return _buildAssistantReasoningBlock(item.message, item.message.reasoning);
    }

    return const SizedBox.shrink();
  }

  Widget _buildMessageItem(WoxAIChatConversation message, BuildContext context) {
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;
    final isAssistant = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_ASSISTANT.value;

    if (isUser) {
      return _buildUserMessageItem(message, context);
    }

    if (isAssistant) {
      // Standalone (non-round) assistant items hide the meta row: only the
      // final assistant inside a round owns the toolbar.
      return _buildAssistantMessageItem(message, showMetaRow: false);
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
                        Container(
                          margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(3)),
                          padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(8)),
                          decoration: BoxDecoration(color: safeFromCssColor(woxTheme.resultItemActiveBackgroundColor), borderRadius: BorderRadius.circular(8)),
                          child: _buildMessageContent(message, fontColor, isUser: true),
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
  // Only the last assistant reply in a round renders the meta row (timestamp
  // plus copy/regenerate actions); intermediate assistant replies hide it.
  // Renders assistant messages as a full-width reading column.
  // showReasoning controls whether the reasoning block is rendered; rounds
  // fold the final reply's reasoning into the collapsible intermediate area,
  // so the final reply only shows text + images when collapsed.
  Widget _buildAssistantMessageItem(WoxAIChatConversation message, {required bool showMetaRow, bool showReasoning = true}) {
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
                  child: _buildMessageContent(message, fontColor, showReasoning: showReasoning),
                ),
                if (showMetaRow) _buildHoverVisibleMessageMetaRow(message: message, isUser: false, visible: isHovered),
              ],
            ),
          ),
        );
      },
    );
  }

  // Renders one assistant round. Completed rounds collapse the intermediate
  // process (tool calls + earlier assistant messages + the final reply's
  // reasoning) by default and show a "Worked for {duration}" header above the
  // final reply. While generating, everything stays visible. The final
  // assistant reply always renders with its toolbar (timestamp +
  // copy/regenerate); when collapsed its reasoning is hidden inside the
  // collapsible area so only the answer text + toolbar remain visible.
  Widget _buildRoundItem(_ChatRoundRenderItem item, BuildContext context) {
    return Obx(() {
      final children = <Widget>[];

      if (item.canCollapse) {
        children.add(_buildRoundHeader(item));
      }

      if (item.canCollapse && controller.isRoundCollapsed(item.id)) {
        // Collapsed: intermediate process and the final reply's reasoning are
        // hidden inside the fold. The final reply body only shows text/images.
        children.add(_buildAssistantMessageItem(item.finalAssistantMessage!, showMetaRow: true, showReasoning: false));
      } else {
        // Open: either the round is still generating (canCollapse is false)
        // or the user expanded a completed round. In both cases intermediate
        // items render first. The final reply keeps its toolbar. When the
        // round can collapse, reasoning is rendered as the trailing
        // intermediate item and the reply body hides it; while generating the
        // reasoning has not been extracted yet, so the reply body shows it
        // inline.
        final showReasoningInline = !item.canCollapse;
        for (final sub in item.intermediateItems) {
          children.add(_buildChatRenderItem(sub, context));
        }
        if (item.finalAssistantMessage != null) {
          children.add(_buildAssistantMessageItem(item.finalAssistantMessage!, showMetaRow: true, showReasoning: showReasoningInline));
        }
      }

      return Column(crossAxisAlignment: CrossAxisAlignment.start, children: children);
    });
  }

  // Renders a standalone reasoning block (used when the final reply is
  // collapsed and its reasoning needs to live inside the collapsible area).
  Widget _buildAssistantReasoningBlock(WoxAIChatConversation message, String reasoning) {
    final fontColor = safeFromCssColor(woxTheme.resultItemTitleColor);
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(3)),
      child: Container(
        width: double.infinity,
        margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(3)),
        padding: EdgeInsets.only(top: _metrics.scaledSpacing(1), right: _metrics.scaledSpacing(4)),
        child: WoxSelectableText(reasoning.trim(), style: TextStyle(fontSize: _metrics.smallLabelFontSize, height: 1.4, color: fontColor.withAlpha(120))),
      ),
    );
  }

  // The collapsible round header: a clickable row showing the worked-for
  // duration (or "Working..." while in progress) with an expand/collapse
  // chevron. Mirrors the tool-activity header styling for consistency.
  Widget _buildRoundHeader(_ChatRoundRenderItem item) {
    final titleColor = safeFromCssColor(woxTheme.resultItemSubTitleColor);
    final collapsed = controller.isRoundCollapsed(item.id);

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(3)),
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () => controller.toggleRoundCollapsed(item.id),
        child: Padding(
          padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(2), vertical: _metrics.scaledSpacing(4)),
          child: Row(
            children: [
              Icon(collapsed ? Icons.keyboard_arrow_right : Icons.keyboard_arrow_down, size: _metrics.scaledSpacing(16), color: titleColor),
              SizedBox(width: _metrics.scaledSpacing(6)),
              WoxChatToolcallDuration(
                id: item.id,
                startTimestamp: item.workedForStart,
                endTimestamp: item.workedForEnd,
                showUnit: false,
                style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: titleColor, fontWeight: FontWeight.w600),
                builder: (context, durationMs) {
                  final label = item.workedForEnd == null ? tr('ui_ai_chat_round_working') : Strings.format(tr('ui_ai_chat_round_worked_duration'), [_formatDuration(durationMs)]);
                  return Text(label, style: TextStyle(fontSize: _metrics.smallLabelFontSize, color: titleColor, fontWeight: FontWeight.w600));
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  // Format a millisecond duration as a compact human-readable string. Uses
  // seconds for short durations and m/ss once it exceeds a minute.
  String _formatDuration(int durationMs) {
    if (durationMs < 0) {
      durationMs = 0;
    }
    final totalSeconds = (durationMs / 1000).round();
    if (totalSeconds < 60) {
      return '${totalSeconds}s';
    }
    final minutes = totalSeconds ~/ 60;
    final seconds = totalSeconds % 60;
    return seconds == 0 ? '${minutes}m' : '${minutes}m ${seconds}s';
  }

  // Renders the shared text and image payload for visible chat messages.
  // showReasoning folds the reasoning block out of the final round reply when
  // the round is collapsed (reasoning lives in the collapsible area instead).
  Widget _buildMessageContent(WoxAIChatConversation message, Color fontColor, {bool showReasoning = true, bool isUser = false}) {
    final hasReasoning = showReasoning && message.reasoning.trim().isNotEmpty;
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
        if (hasText) isUser ? _buildUserMessageText(message.text, fontColor) : WoxMarkdownView(data: message.text, fontColor: fontColor, fontSize: _metrics.resultSubtitleFontSize),
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

  // Render user message text with {skill:name} tags displayed as inline
  // highlighted chips. Falls back to WoxMarkdownView when no tags are present.
  Widget _buildUserMessageText(String text, Color fontColor) {
    final pattern = RegExp(r'\{skill:([^}]+)\}');
    final matches = pattern.allMatches(text);
    if (matches.isEmpty) {
      return WoxMarkdownView(data: text, fontColor: fontColor, fontSize: _metrics.resultSubtitleFontSize);
    }

    final spans = <InlineSpan>[];
    int lastEnd = 0;
    for (final match in matches) {
      if (match.start > lastEnd) {
        spans.add(TextSpan(text: text.substring(lastEnd, match.start)));
      }
      spans.add(WidgetSpan(alignment: PlaceholderAlignment.middle, child: _buildInlineSkillTag(match.group(1)!, fontColor)));
      lastEnd = match.end;
    }
    if (lastEnd < text.length) {
      spans.add(TextSpan(text: text.substring(lastEnd)));
    }

    return Text.rich(TextSpan(children: spans, style: TextStyle(color: fontColor, fontSize: _metrics.resultSubtitleFontSize, height: 1.5)));
  }

  Widget _buildInlineSkillTag(String name, Color fontColor) {
    final backgroundColor = safeFromCssColor(woxTheme.actionItemActiveBackgroundColor).withAlpha(40);
    final borderColor = safeFromCssColor(woxTheme.actionItemActiveBackgroundColor).withAlpha(80);
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 4),
      padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(5), vertical: _metrics.scaledSpacing(1)),
      decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(4), border: Border.all(color: borderColor)),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.extension_rounded, size: _metrics.scaledSpacing(12), color: fontColor.withAlpha(180)),
          SizedBox(width: _metrics.scaledSpacing(3)),
          Text(name, style: TextStyle(color: fontColor, fontSize: _metrics.smallLabelFontSize, fontWeight: FontWeight.w600)),
        ],
      ),
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

// A standalone reasoning block for the final assistant reply. Rounds fold the
// final reply's reasoning into the collapsible area, so it is rendered as a
// trailing intermediate item (visible only when expanded) instead of inline
// above the final answer.
class _ChatReasoningRenderItem extends _ChatRenderItem {
  final WoxAIChatConversation message;

  const _ChatReasoningRenderItem(this.message);
}

// Aggregates one assistant round: everything between a user message and the
// next user message (or end of conversation). The last assistant message is
// rendered as the round's "final answer" (with toolbar); everything before it
// is collapsible intermediate process. Completed rounds are collapsed by
// default and show a "Worked for {duration}" header.
class _ChatRoundRenderItem extends _ChatRenderItem {
  final String id;
  // Items in this round in original order, excluding the final assistant item.
  final List<_ChatRenderItem> intermediateItems;
  // The final assistant message in this round, shown outside the collapsed
  // region. Null when the round has no assistant reply yet (e.g. only tool
  // calls so far while generating).
  final WoxAIChatConversation? finalAssistantMessage;
  // The final assistant reply's reasoning, captured separately so it can be
  // rendered inside the collapsible area when the round is collapsed (the
  // final reply body only shows text + images when collapsed).
  final String finalAssistantReasoning;

  // True when the round is considered finished: a following user message
  // exists, or generation has stopped and the round is the trailing one.
  final bool isComplete;

  // Timestamp (ms) of the first assistant message in the round; 0 if none.
  final int workedForStart;
  // Timestamp (ms) of the final assistant message; null while generating.
  final int? workedForEnd;

  const _ChatRoundRenderItem({
    required this.id,
    required this.intermediateItems,
    required this.finalAssistantMessage,
    required this.finalAssistantReasoning,
    required this.isComplete,
    required this.workedForStart,
    required this.workedForEnd,
  });

  // A round only needs the collapse affordance when it has both intermediate
  // process and a final assistant reply. Single-message rounds or in-progress
  // rounds render as plain message lists.
  bool get canCollapse => isComplete && intermediateItems.isNotEmpty && finalAssistantMessage != null;
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

// A stateful new-chat button so it stays visually quiet by default and only
// highlights when the user hovers over it.
class _NewChatButton extends StatefulWidget {
  final VoidCallback onTap;

  const _NewChatButton({required this.onTap});

  @override
  State<_NewChatButton> createState() => _NewChatButtonState();
}

class _NewChatButtonState extends State<_NewChatButton> {
  bool isHovered = false;

  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  Widget build(BuildContext context) {
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    final backgroundColor = isHovered ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor) : Colors.transparent;
    return MouseRegion(
      onEnter: (_) => setState(() => isHovered = true),
      onExit: (_) => setState(() => isHovered = false),
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: widget.onTap,
        child: Container(
          width: double.infinity,
          padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(12), vertical: _metrics.scaledSpacing(10)),
          decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(6)),
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
      ),
    );
  }
}

// A stateful tile so hover changes survive rebuilds and are clearly visible.
class _ConversationTile extends StatefulWidget {
  final String title;
  final bool active;
  final VoidCallback onTap;
  final Widget? trailing;

  const _ConversationTile({required this.title, required this.active, required this.onTap, this.trailing});

  @override
  State<_ConversationTile> createState() => _ConversationTileState();
}

class _ConversationTileState extends State<_ConversationTile> {
  bool isHovered = false;

  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;

  @override
  Widget build(BuildContext context) {
    final woxTheme = WoxThemeUtil.instance.currentTheme.value;
    final backgroundColor = widget.active || isHovered ? safeFromCssColor(woxTheme.resultItemActiveBackgroundColor) : Colors.transparent;
    final titleColor = widget.active ? safeFromCssColor(woxTheme.resultItemActiveTitleColor) : safeFromCssColor(woxTheme.resultItemTitleColor);
    return MouseRegion(
      onEnter: (_) => setState(() => isHovered = true),
      onExit: (_) => setState(() => isHovered = false),
      child: GestureDetector(
        onTap: widget.onTap,
        child: Container(
          height: _metrics.scaledSpacing(42),
          margin: EdgeInsets.only(bottom: _metrics.scaledSpacing(4)),
          padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(8)),
          decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(6)),
          child: Row(
            children: [
              Icon(Icons.chat_bubble, size: _metrics.scaledSpacing(22), color: safeFromCssColor(woxTheme.resultItemActiveBackgroundColor)),
              SizedBox(width: _metrics.scaledSpacing(10)),
              Expanded(
                child: Text(
                  widget.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: titleColor, fontSize: _metrics.resultSubtitleFontSize, fontWeight: FontWeight.w600),
                ),
              ),
              if (widget.trailing != null) ...[SizedBox(width: _metrics.scaledSpacing(4)), widget.trailing!],
            ],
          ),
        ),
      ),
    );
  }
}
