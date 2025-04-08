import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/log.dart';
import 'package:fluent_ui/fluent_ui.dart' as fluent;

class WoxPreviewChatView extends StatefulWidget {
  final WoxPreviewChatData aiChatData;
  final WoxTheme woxTheme;

  const WoxPreviewChatView({super.key, required this.aiChatData, required this.woxTheme});

  @override
  State<WoxPreviewChatView> createState() => _WoxPreviewChatViewState();
}

class _WoxPreviewChatViewState extends State<WoxPreviewChatView> {
  final TextEditingController textController = TextEditingController();
  final WoxLauncherController controller = Get.find<WoxLauncherController>();

  // State for tool usage
  bool _isToolUseEnabled = true;
  Set<String> _selectedTools = {};
  List<AIMCPTool> _availableTools = [];
  bool _isLoadingTools = false;
  bool _isToolSectionExpanded = false; // State for expandable section

  @override
  void initState() {
    super.initState();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      controller.scrollToBottomOfAiChat();
    });

    // Fetch tools if a model is selected initially
    if (widget.aiChatData.model.name.isNotEmpty) {
      // Don't await here, let it load in background
      _fetchAvailableTools();
    }
  }

  // Method to fetch available tools based on the current model
  Future<void> _fetchAvailableTools() async {
    // Prevent concurrent fetches
    if (_isLoadingTools) return;

    if (widget.aiChatData.model.name.isEmpty) {
      if (mounted) {
        setState(() {
          _availableTools = [];
          _selectedTools = {};
          _isLoadingTools = false;
        });
      }
      return;
    }

    if (mounted) {
      setState(() {
        _isLoadingTools = true;
      });
    }

    try {
      final apiParams = <String, dynamic>{}; // TODO: Fix tool fetching parameters

      final tools = await WoxApi.instance.findAIMCPServerTools(apiParams);
      if (mounted) {
        setState(() {
          _availableTools = tools;
          // Default select all tools
          _selectedTools = tools.map((tool) => tool.name).toSet(); // Assuming tool has 'name'
        });
      }
    } catch (e, s) {
      Logger.instance.error(const UuidV4().generate(), 'Error fetching AI tools: $e $s');
      if (mounted) {
        setState(() {
          _availableTools = [];
          _selectedTools = {};
        });
      }
    } finally {
      if (mounted) {
        setState(() {
          _isLoadingTools = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: chat view");

    return Column(
      children: [
        // AI Model Selection
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 8.0),
          child: InkWell(
            onTap: () => controller.showActionPanelForModelSelection(const UuidV4().generate(), widget.aiChatData),
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
                      widget.aiChatData.model.name.isEmpty ? "请选择模型" : widget.aiChatData.model.name,
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
          child: SingleChildScrollView(
            controller: controller.aiChatScrollController,
            padding: const EdgeInsets.symmetric(vertical: 16.0),
            child: Column(
              children: widget.aiChatData.conversations.map((message) => buildMessageItem(message)).toList(),
            ),
          ),
        ),
        // Input box and controls area
        Focus(
          onFocusChange: (bool hasFocus) {
            final traceId = const UuidV4().generate();
            if (!hasFocus) {
              controller.updateToolbarByActiveAction(traceId);
            } else {
              controller.updateToolbarByChat(traceId);
            }
          },
          onKeyEvent: (FocusNode node, KeyEvent event) {
            if (event is KeyDownEvent) {
              switch (event.logicalKey) {
                case LogicalKeyboardKey.escape:
                  controller.focusQueryBox();
                  return KeyEventResult.handled;
                case LogicalKeyboardKey.enter:
                  sendMessage();
                  return KeyEventResult.handled;
              }
            }

            var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
            if (pressedHotkey == null) {
              return KeyEventResult.ignored;
            }

            // list all models
            if (controller.isActionHotkey(pressedHotkey)) {
              controller.showActionPanelForModelSelection(const UuidV4().generate(), widget.aiChatData);
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
                // --- Expandable Tool Section ---
                _isToolSectionExpanded
                    ? Column(
                        children: [
                          buildToolSelectionContent(),
                          const SizedBox(height: 12),
                        ],
                      )
                    : const SizedBox.shrink(),
                // --- Input Box Container ---
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
                      // Input Box Toolbar (Send button, Tool icon)
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
                            // tool use IconButton
                            IconButton(
                              tooltip: 'Configure Tool Usage',
                              // Change icon based on expanded state? Optional.
                              icon: Icon(Icons.build,
                                  size: 18,
                                  color: _isToolUseEnabled
                                      ? Theme.of(context).colorScheme.primary // Use theme accent if enabled
                                      : fromCssColor(widget.woxTheme.previewPropertyTitleColor)),
                              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                              // Toggle the expansion state on press
                              onPressed: () {
                                setState(() {
                                  _isToolSectionExpanded = !_isToolSectionExpanded;
                                });
                                // Fetch tools only when expanding and if needed
                                if (_isToolSectionExpanded && widget.aiChatData.model.name.isNotEmpty && _availableTools.isEmpty && !_isLoadingTools) {
                                  _fetchAvailableTools();
                                }
                              },
                              padding: EdgeInsets.zero,
                              constraints: const BoxConstraints(
                                minWidth: 32,
                                minHeight: 32,
                              ),
                            ),
                            const Spacer(),
                            // Send button container (unchanged)
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

  Widget buildToolSelectionContent() {
    // Use a container to provide background and shape
    return Container(
      margin: const EdgeInsets.only(bottom: 0), // Remove bottom margin if input box is directly below
      padding: const EdgeInsets.symmetric(horizontal: 12.0, vertical: 8.0),
      decoration: BoxDecoration(
        color: fromCssColor(widget.woxTheme.queryBoxBackgroundColor), // Match input box background
        borderRadius: BorderRadius.circular(widget.woxTheme.queryBoxBorderRadius.toDouble()),
        border: Border(
          top: BorderSide(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.1)),
          left: BorderSide(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.1)),
          right: BorderSide(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.1)),
          bottom: BorderSide(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.1)),
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          // Enable/Disable Toggle
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 4.0),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  'Enable Tool Usage',
                  style: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor)),
                ),
                Switch(
                  value: _isToolUseEnabled,
                  onChanged: (bool value) {
                    // Use setState directly here as this content is built within the main state
                    setState(() {
                      _isToolUseEnabled = value;
                    });
                  },
                ),
              ],
            ),
          ),
          // Conditional Divider and Tool List
          if (_isToolUseEnabled) ...[
            Divider(
              color: fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.1),
            ),
            // Status messages or Tool List
            if (widget.aiChatData.model.name.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 16.0),
                child: Center(child: Text('Please select a model first.', style: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor)))),
              )
            else if (_isLoadingTools)
              const Padding(padding: EdgeInsets.symmetric(vertical: 16.0), child: Center(child: CircularProgressIndicator())) // Smaller progress ring
            else if (_availableTools.isEmpty)
              Padding(
                  padding: const EdgeInsets.symmetric(vertical: 16.0),
                  child: Center(child: Text('No tools available for this model.', style: TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor)))))
            else
              // Constrain height and make scrollable if list is long
              ConstrainedBox(
                constraints: const BoxConstraints(maxHeight: 150), // Limit max height
                child: ListView(
                  padding: const EdgeInsets.only(top: 4.0, bottom: 4.0), // Add some padding
                  shrinkWrap: true,
                  children: _availableTools.map((tool) {
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 1.0),
                      child: Row(
                        children: [
                          fluent.Checkbox(
                            checked: _selectedTools.contains(tool.name),
                            onChanged: (bool? selected) {
                              setState(() {
                                // Directly use setState
                                if (selected == true) {
                                  _selectedTools.add(tool.name);
                                } else {
                                  _selectedTools.remove(tool.name);
                                }
                              });
                            },
                          ),
                          const SizedBox(width: 8),
                          fluent.Expanded(
                            child: fluent.Text(
                              tool.name,
                              style: fluent.TextStyle(color: fromCssColor(widget.woxTheme.previewPropertyTitleColor)),
                              overflow: fluent.TextOverflow.ellipsis,
                            ),
                          ),
                        ],
                      ),
                    );
                  }).toList(),
                ),
              ),
          ],
        ],
      ),
    );
  }
  // --- End Expandable Section Logic ---

  void sendMessage() {
    final text = textController.text.trim();
    // Check if AI model is selected
    if (widget.aiChatData.model.name.isEmpty) {
      controller.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please select a model", displaySeconds: 3));
      return;
    }
    // check if the text is empty
    if (text.isEmpty) {
      controller.showToolbarMsg(const UuidV4().generate(), ToolbarMsg(text: "Please enter a message", displaySeconds: 3));
      return;
    }

    // append user message to chat data
    widget.aiChatData.conversations.add(WoxPreviewChatConversation(
      id: const UuidV4().generate(),
      role: WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value,
      text: text,
      images: [], // TODO: Support images if needed
      timestamp: DateTime.now().millisecondsSinceEpoch,
    ));
    widget.aiChatData.updatedAt = DateTime.now().millisecondsSinceEpoch;

    textController.clear();

    // Collapse the tool section after sending? Optional.
    // if (_isToolSectionExpanded) {
    //   setState(() {
    //     _isToolSectionExpanded = false;
    //   });
    // } else {
    setState(() {}); // Refresh to show the new message
    // }

    SchedulerBinding.instance.addPostFrameCallback((_) {
      controller.scrollToBottomOfAiChat();
    });

    // Pass tool usage info to the controller method
    // IMPORTANT: Assumes sendChatRequest signature is updated like:
    // sendChatRequest(String traceId, WoxPreviewChatData aiChatData, {bool isToolUseEnabled, Set<String> selectedTools})
    controller.sendChatRequest(
      const UuidV4().generate(),
      widget.aiChatData,
      // TODO: Update WoxLauncherController.sendChatRequest to accept tool usage parameters
      // isToolUseEnabled: _isToolUseEnabled,
      // selectedTools: _selectedTools,
    );
  }

  Widget buildMessageItem(WoxPreviewChatConversation message) {
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;
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
                            color: fromCssColor(isUser ? controller.woxTheme.value.resultItemActiveTitleColor : controller.woxTheme.value.resultItemTitleColor),
                            fontSize: 14,
                          ),
                          // TODO: Add styling for code blocks, lists etc based on theme
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
                    formatTimestamp(message.timestamp),
                    style: TextStyle(
                      fontSize: 11,
                      color: fromCssColor(controller.woxTheme.value.resultItemSubTitleColor),
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
    final isUser = message.role == WoxAIChatConversationRoleEnum.WOX_AIChat_CONVERSATION_ROLE_USER.value;
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
          isUser ? 'U' : 'A', // Consider using user/model icons later
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
