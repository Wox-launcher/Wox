import 'dart:math' as math;

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
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/enums/wox_ai_conversation_role_enum.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/log.dart';
// 不再需要fluent_ui

// Class to represent an item in the chat select panel
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

class WoxPreviewChatView extends StatefulWidget {
  final WoxPreviewChatData aiChatData;
  final WoxTheme woxTheme;

  const WoxPreviewChatView({super.key, required this.aiChatData, required this.woxTheme});

  @override
  State<WoxPreviewChatView> createState() => _WoxPreviewChatViewState();
}

class _WoxPreviewChatViewState extends State<WoxPreviewChatView> {
  final TextEditingController textController = TextEditingController();
  final TextEditingController chatSelectFilterController = TextEditingController();
  final FocusNode chatSelectFilterFocusNode = FocusNode();
  final WoxLauncherController controller = Get.find<WoxLauncherController>();
  final ScrollController _chatSelectScrollController = ScrollController();

  // State for chat select panel
  bool _isShowChatSelectPanel = false;
  int _activeChatSelectIndex = 0;
  List<ChatSelectItem> _chatSelectItems = [];
  List<ChatSelectItem> _filteredChatSelectItems = [];
  String _currentChatSelectCategory = "";

  // State for tool usage
  Set<String> _selectedTools = {};
  List<AIMCPTool> _availableTools = [];
  bool _isLoadingTools = false;

  @override
  void initState() {
    super.initState();

    SchedulerBinding.instance.addPostFrameCallback((_) {
      controller.scrollToBottomOfAiChat();
    });

    // Initialize chat select items
    _initChatSelectItems();

    // Fetch tools if a model is selected initially
    if (widget.aiChatData.model.name.isNotEmpty) {
      // Don't await here, let it load in background
      _fetchAvailableTools();
    }

    // Add listener to hide chat select panel when filter textbox loses focus
    chatSelectFilterFocusNode.addListener(() {
      if (!chatSelectFilterFocusNode.hasFocus && _isShowChatSelectPanel) {
        // Use a small delay to allow for clicks on items to register first
        Future.delayed(const Duration(milliseconds: 100), () {
          if (!chatSelectFilterFocusNode.hasFocus && _isShowChatSelectPanel) {
            _hideChatSelectPanel();
          }
        });
      }
    });
  }

  // Initialize chat select items
  void _initChatSelectItems() {
    // First level categories
    _chatSelectItems = [
      ChatSelectItem(
          id: "models",
          name: "Model Selection",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
          isCategory: true,
          children: [],
          onExecute: (String traceId) {
            setState(() {
              _currentChatSelectCategory = "models";
              _activeChatSelectIndex = 0;
              // Clear filter content
              chatSelectFilterController.text = "";
              _updateFilteredChatSelectItems();

              // Ensure focus is on the filter textbox
              SchedulerBinding.instance.addPostFrameCallback((_) {
                chatSelectFilterFocusNode.requestFocus();
              });
            });
          }),
      ChatSelectItem(
          id: "tools",
          name: "Tool Configuration",
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🔧"),
          isCategory: true,
          children: [],
          onExecute: (String traceId) {
            setState(() {
              _currentChatSelectCategory = "tools";
              _activeChatSelectIndex = 0;
              // 清空过滤器内容
              chatSelectFilterController.text = "";
              _updateFilteredChatSelectItems();

              // 确保焦点在过滤器文本框上
              SchedulerBinding.instance.addPostFrameCallback((_) {
                chatSelectFilterFocusNode.requestFocus();
              });
            });
          }),
    ];

    _filteredChatSelectItems = List.from(_chatSelectItems);
  }

  // Update filtered chat select items based on current category and filter text
  void _updateFilteredChatSelectItems() {
    final filterText = chatSelectFilterController.text.toLowerCase();

    if (_currentChatSelectCategory.isEmpty) {
      // Show main categories
      _filteredChatSelectItems = _chatSelectItems.where((item) => item.name.toLowerCase().contains(filterText)).toList();
    } else if (_currentChatSelectCategory == "models") {
      // Show models grouped by provider
      _filteredChatSelectItems = [];

      // Group models by provider
      final modelsByProvider = <String, List<AIModel>>{};

      // Filter and group models
      for (final model in controller.aiModels) {
        if (filterText.isEmpty || model.name.toLowerCase().contains(filterText) || model.provider.toLowerCase().contains(filterText)) {
          modelsByProvider.putIfAbsent(model.provider, () => []).add(model);
        }
      }

      // Sort providers
      final providers = modelsByProvider.keys.toList()..sort();

      // Add groups and models
      for (final provider in providers) {
        // Skip empty groups
        if (modelsByProvider[provider]!.isEmpty) continue;

        // Add provider group header
        _filteredChatSelectItems.add(ChatSelectItem(
          id: "group_$provider",
          name: provider,
          icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🏢"),
          isCategory: true,
          children: [],
        ));

        // Sort models within this provider
        final models = modelsByProvider[provider]!;
        models.sort((a, b) => a.name.compareTo(b.name));

        // Add models for this provider
        for (final model in models) {
          _filteredChatSelectItems.add(ChatSelectItem(
              id: "${model.provider}_${model.name}",
              name: model.name,
              icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🤖"),
              isCategory: false,
              children: [],
              onExecute: (String traceId) {
                widget.aiChatData.model = WoxPreviewChatModel(name: model.name, provider: model.provider);
                _hideChatSelectPanel();
              }));
        }
      }
    } else if (_currentChatSelectCategory == "tools") {
      // Show tools
      _filteredChatSelectItems = _availableTools
          .map((tool) => ChatSelectItem(
              id: tool.name,
              name: tool.name,
              icon: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: "🔧"),
              isCategory: false,
              children: [],
              onExecute: (String traceId) {
                setState(() {
                  if (_selectedTools.contains(tool.name)) {
                    _selectedTools.remove(tool.name);
                  } else {
                    _selectedTools.add(tool.name);
                  }
                  // 不关闭面板，让用户可以继续选择其他工具
                });
              }))
          .where((item) => item.name.toLowerCase().contains(filterText))
          .toList();
    }

    // Ensure active index is within bounds
    if (_filteredChatSelectItems.isNotEmpty && _activeChatSelectIndex >= _filteredChatSelectItems.length) {
      _activeChatSelectIndex = _filteredChatSelectItems.length - 1;
    }
  }

  // Show chat select panel
  void _showChatSelectPanel() {
    setState(() {
      _isShowChatSelectPanel = true;
      _currentChatSelectCategory = "";
      _activeChatSelectIndex = 0;
      chatSelectFilterController.text = "";
      _updateFilteredChatSelectItems();
    });

    // 确保在下一帧渲染后设置焦点
    SchedulerBinding.instance.addPostFrameCallback((_) {
      // 先清除任何现有焦点
      FocusScope.of(context).unfocus();
      // 然后请求过滤文本框的焦点
      chatSelectFilterFocusNode.requestFocus();

      // 打印日志以确认焦点请求
      Logger.instance.debug(const UuidV4().generate(), "Requesting focus for chat select filter");
    });
  }

  // Hide chat select panel
  void _hideChatSelectPanel() {
    setState(() {
      _isShowChatSelectPanel = false;
      chatSelectFilterController.text = "";
    });
    controller.aiChatFocusNode.requestFocus();
  }

  // Scroll to active item in the list
  void _scrollToActiveItem() {
    if (_filteredChatSelectItems.isEmpty) return;

    // Use a post frame callback to ensure the list has been built
    SchedulerBinding.instance.addPostFrameCallback((_) {
      // Fixed item height - we use this for calculation
      const itemHeight = 40.0;

      // Calculate position to scroll to
      final offset = _activeChatSelectIndex * itemHeight;

      // Get visible area
      const viewportHeight = 350.0; // Same as maxHeight constraint
      final currentOffset = _chatSelectScrollController.offset;
      final visibleStart = currentOffset;
      final visibleEnd = currentOffset + viewportHeight;

      // Only scroll if the item is not fully visible
      if (offset < visibleStart || (offset + itemHeight) > visibleEnd) {
        // Jump to position that centers the item
        _chatSelectScrollController.jumpTo(math.max(0, offset - (viewportHeight / 2) + (itemHeight / 2)));
      }
    });
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
      // 使用findAIMCPServerToolsAll获取所有工具
      final tools = await WoxApi.instance.findAIMCPServerToolsAll({});
      if (mounted) {
        setState(() {
          _availableTools = tools;
          // Default select all tools
          _selectedTools = tools.map((tool) => tool.name).toSet();
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

  // Handle chat select panel keyboard navigation
  void _handleChatSelectKeyboard(KeyEvent event) {
    // 只处理特定的键盘事件，让其他键盘输入正常工作
    if (event is KeyDownEvent) {
      switch (event.logicalKey) {
        case LogicalKeyboardKey.escape:
          if (_currentChatSelectCategory.isNotEmpty) {
            // Go back to main categories
            setState(() {
              _currentChatSelectCategory = "";
              _activeChatSelectIndex = 0;
              chatSelectFilterController.text = "";
              _updateFilteredChatSelectItems();

              // 返回主类别时，确保焦点在过滤器文本框上
              SchedulerBinding.instance.addPostFrameCallback((_) {
                chatSelectFilterFocusNode.requestFocus();
              });
            });
          } else {
            // Close panel
            _hideChatSelectPanel();
          }
          return; // 返回KeyEventResult.handled
        case LogicalKeyboardKey.arrowDown:
          setState(() {
            if (_filteredChatSelectItems.isNotEmpty) {
              _activeChatSelectIndex = (_activeChatSelectIndex + 1) % _filteredChatSelectItems.length;
              _scrollToActiveItem();
            }
          });
          return; // 返回KeyEventResult.handled
        case LogicalKeyboardKey.arrowUp:
          setState(() {
            if (_filteredChatSelectItems.isNotEmpty) {
              _activeChatSelectIndex = (_activeChatSelectIndex - 1 + _filteredChatSelectItems.length) % _filteredChatSelectItems.length;
              _scrollToActiveItem();
            }
          });
          return; // 返回KeyEventResult.handled
        case LogicalKeyboardKey.enter:
          if (_filteredChatSelectItems.isNotEmpty) {
            final selectedItem = _filteredChatSelectItems[_activeChatSelectIndex];
            if (selectedItem.onExecute != null) {
              selectedItem.onExecute!(const UuidV4().generate());
            }
          }
          return; // 返回KeyEventResult.handled
        default:
          // 对于其他键，让它们正常处理（如文本输入）
          return; // 返回KeyEventResult.ignored
      }
    }
  }

  // Build chat select panel
  Widget _buildChatSelectPanel() {
    return Positioned(
      right: 10,
      bottom: 10,
      child: Material(
        elevation: 8,
        borderRadius: BorderRadius.circular(controller.woxTheme.value.actionQueryBoxBorderRadius.toDouble()),
        child: Container(
          padding: EdgeInsets.only(
            top: controller.woxTheme.value.actionContainerPaddingTop.toDouble(),
            bottom: controller.woxTheme.value.actionContainerPaddingBottom.toDouble(),
            left: controller.woxTheme.value.actionContainerPaddingLeft.toDouble(),
            right: controller.woxTheme.value.actionContainerPaddingRight.toDouble(),
          ),
          decoration: BoxDecoration(
            color: fromCssColor(controller.woxTheme.value.actionContainerBackgroundColor),
            borderRadius: BorderRadius.circular(controller.woxTheme.value.actionQueryBoxBorderRadius.toDouble()),
          ),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 320),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              mainAxisAlignment: MainAxisAlignment.start,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(_currentChatSelectCategory.isEmpty ? "Chat Options" : (_currentChatSelectCategory == "models" ? "Select Model" : "Configure Tools"),
                    style: TextStyle(color: fromCssColor(controller.woxTheme.value.actionContainerHeaderFontColor), fontSize: 16.0)),
                const Divider(),
                // List of items
                ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 350),
                  child: _filteredChatSelectItems.isEmpty
                      ? Center(
                          child: Padding(
                            padding: const EdgeInsets.all(16.0),
                            child: Text(
                              "No items found",
                              style: TextStyle(color: fromCssColor(controller.woxTheme.value.actionItemFontColor)),
                            ),
                          ),
                        )
                      : ListView.builder(
                          shrinkWrap: true,
                          controller: _chatSelectScrollController,
                          itemCount: _filteredChatSelectItems.length,
                          // Variable height for items
                          itemBuilder: (context, index) {
                            final item = _filteredChatSelectItems[index];
                            final isActive = index == _activeChatSelectIndex;

                            // For tools category, show checkbox
                            Widget? trailing;
                            if (_currentChatSelectCategory == "tools") {
                              trailing = Checkbox(
                                value: _selectedTools.contains(item.id),
                                onChanged: (value) {
                                  setState(() {
                                    if (value == true) {
                                      _selectedTools.add(item.id);
                                    } else {
                                      _selectedTools.remove(item.id);
                                    }
                                  });
                                },
                              );
                            } else if (item.isCategory) {
                              trailing = const Icon(Icons.arrow_forward_ios, size: 16);
                            }

                            // Different styling for group headers vs regular items
                            if (item.isCategory && _currentChatSelectCategory == "models") {
                              // Group header style
                              return Container(
                                decoration: BoxDecoration(
                                  color: Colors.transparent,
                                  border: Border(
                                    bottom: BorderSide(
                                      color: fromCssColor(controller.woxTheme.value.actionItemFontColor).withOpacity(0.1),
                                      width: 1,
                                    ),
                                  ),
                                ),
                                padding: const EdgeInsets.only(top: 8, left: 8, right: 8),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        WoxImageView(
                                          woxImage: item.icon,
                                          width: 16,
                                          height: 16,
                                        ),
                                        const SizedBox(width: 8),
                                        Text(
                                          item.name.toUpperCase(),
                                          style: TextStyle(
                                            color: fromCssColor(controller.woxTheme.value.actionItemFontColor),
                                            fontSize: 12,
                                            fontWeight: FontWeight.bold,
                                          ),
                                        ),
                                      ],
                                    ),
                                    const SizedBox(height: 4),
                                  ],
                                ),
                              );
                            } else {
                              // Regular item style
                              return Container(
                                decoration: BoxDecoration(
                                  color: isActive ? fromCssColor(controller.woxTheme.value.actionItemActiveBackgroundColor) : Colors.transparent,
                                  borderRadius: BorderRadius.circular(4),
                                ),
                                child: ListTile(
                                  dense: true,
                                  contentPadding: const EdgeInsets.symmetric(horizontal: 8),
                                  leading: WoxImageView(
                                    woxImage: item.icon,
                                    width: 24,
                                    height: 24,
                                  ),
                                  title: Text(
                                    item.name,
                                    style: TextStyle(
                                      color: fromCssColor(isActive ? controller.woxTheme.value.actionItemActiveFontColor : controller.woxTheme.value.actionItemFontColor),
                                    ),
                                  ),
                                  trailing: trailing,
                                  onTap: () {
                                    setState(() {
                                      _activeChatSelectIndex = index;
                                    });
                                    if (item.onExecute != null) {
                                      item.onExecute!(const UuidV4().generate());
                                    }
                                    if (item.isCategory) {
                                      SchedulerBinding.instance.addPostFrameCallback((_) {
                                        chatSelectFilterFocusNode.requestFocus();
                                      });
                                    }
                                  },
                                ),
                              );
                            }
                          },
                        ),
                ),
                // Filter box
                Container(
                  margin: const EdgeInsets.only(top: 8),
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  decoration: BoxDecoration(
                    color: fromCssColor(controller.woxTheme.value.queryBoxBackgroundColor),
                    borderRadius: BorderRadius.circular(controller.woxTheme.value.queryBoxBorderRadius.toDouble()),
                  ),
                  child: Focus(
                    onKeyEvent: (FocusNode node, KeyEvent event) {
                      // Only handle navigation keys, let TextField handle other keys
                      if (event is KeyDownEvent) {
                        switch (event.logicalKey) {
                          case LogicalKeyboardKey.escape:
                          case LogicalKeyboardKey.arrowDown:
                          case LogicalKeyboardKey.arrowUp:
                          case LogicalKeyboardKey.enter:
                            _handleChatSelectKeyboard(event);
                            return KeyEventResult.handled;
                          default:
                            return KeyEventResult.ignored;
                        }
                      }
                      return KeyEventResult.ignored;
                    },
                    child: TextField(
                      controller: chatSelectFilterController,
                      focusNode: chatSelectFilterFocusNode,
                      decoration: InputDecoration(
                        hintText: 'Filter...',
                        hintStyle: TextStyle(color: fromCssColor(controller.woxTheme.value.queryBoxFontColor).withOpacity(0.5)),
                        border: InputBorder.none,
                        contentPadding: const EdgeInsets.symmetric(vertical: 8),
                      ),
                      style: TextStyle(
                        color: fromCssColor(controller.woxTheme.value.queryBoxFontColor),
                      ),
                      onChanged: (value) {
                        setState(() {
                          _updateFilteredChatSelectItems();
                        });
                      },
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

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
                  _showChatSelectPanel();
                  setState(() {
                    _currentChatSelectCategory = "models";
                    _activeChatSelectIndex = 0;
                    chatSelectFilterController.text = "";
                    _updateFilteredChatSelectItems();

                    SchedulerBinding.instance.addPostFrameCallback((_) {
                      chatSelectFilterFocusNode.requestFocus();
                    });
                  });
                },
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

                // Show chat select panel on Cmd+J
                if (controller.isActionHotkey(pressedHotkey)) {
                  _showChatSelectPanel();
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
                    // 不再需要可展开的工具部分，使用chat select panel代替
                    const SizedBox.shrink(),
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
                                // Tool configuration button - opens chat select panel
                                IconButton(
                                  tooltip: 'Configure Tool Usage',
                                  icon: Icon(Icons.build,
                                      size: 18,
                                      color: _selectedTools.isNotEmpty
                                          ? Theme.of(context).colorScheme.primary
                                          : fromCssColor(widget.woxTheme.previewPropertyTitleColor).withOpacity(0.5)),
                                  color: fromCssColor(widget.woxTheme.previewPropertyTitleColor),
                                  onPressed: () {
                                    _showChatSelectPanel();
                                    setState(() {
                                      _currentChatSelectCategory = "tools";
                                      _activeChatSelectIndex = 0;
                                      chatSelectFilterController.text = "";
                                      _updateFilteredChatSelectItems();

                                      SchedulerBinding.instance.addPostFrameCallback((_) {
                                        chatSelectFilterFocusNode.requestFocus();
                                      });
                                    });
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
        ),
        if (_isShowChatSelectPanel) _buildChatSelectPanel(),
      ],
    );
  }

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

    widget.aiChatData.selectedTools = _selectedTools.toList();
    controller.sendChatRequest(
      const UuidV4().generate(),
      widget.aiChatData,
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
