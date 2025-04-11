import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:fuzzywuzzy/fuzzywuzzy.dart';
import 'package:get/get.dart';
import 'package:lpinyin/lpinyin.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxListView extends StatefulWidget {
  final List<WoxListItem> items;
  final WoxTheme woxTheme;
  final WoxListViewType listViewType;
  final bool showFilter;
  final WoxListViewController? controller;
  final double maxHeight;
  // events
  final Function(WoxListItem item)? onItemExecuted; // double tap on item or enter on filter box
  final Function(WoxListItem item)? onItemActive;
  final Function()? onFilterEscPressed;

  const WoxListView({
    super.key,
    required this.items,
    required this.listViewType,
    required this.woxTheme,
    this.showFilter = true,
    this.controller,
    required this.maxHeight,
    this.onItemExecuted,
    this.onItemActive,
    this.onFilterEscPressed,
  });

  @override
  State<WoxListView> createState() => _WoxListViewState();
}

class _WoxListViewState extends State<WoxListView> {
  final ScrollController scrollerController = ScrollController();

  /// This flag is used to control whether the result item is selected by mouse hover.
  /// This is used to prevent the result item from being selected when the mouse is just hovering over the item in the result list.
  var isMouseMoved = false;
  var hoveredResultIndex = -1; // -1 means no item is hovered
  var activeIndex = 0;
  final resultGlobalKeys = <GlobalKey>[]; // the global keys for each result item, used to calculate the position of the result item

  // filter box
  var filterBoxController = TextEditingController();
  var filterBoxFocusNode = FocusNode();

  @override
  void initState() {
    super.initState();

    if (widget.controller != null) {
      widget.controller!._attach(this);
    }

    // Initialize global keys for each item
    for (int i = 0; i < widget.items.length; i++) {
      resultGlobalKeys.add(GlobalKey());
    }
  }

  @override
  void didUpdateWidget(WoxListView oldWidget) {
    super.didUpdateWidget(oldWidget);

    // Handle controller changes
    if (widget.controller != oldWidget.controller) {
      if (oldWidget.controller != null) {
        oldWidget.controller!._detach();
      }
      if (widget.controller != null) {
        widget.controller!._attach(this);
      }
    }

    // Update global keys if items changed
    if (widget.items.length != resultGlobalKeys.length) {
      resultGlobalKeys.clear();
      for (int i = 0; i < widget.items.length; i++) {
        resultGlobalKeys.add(GlobalKey());
      }
    }
  }

  @override
  void dispose() {
    if (widget.controller != null) {
      widget.controller!._detach();
    }
    scrollerController.dispose();
    super.dispose();
  }

  void update(String traceId, WoxListItem item) {
    if (widget.items.isEmpty) {
      return;
    }

    final index = widget.items.indexWhere((element) => element.id == item.id);
    if (index != -1) {
      setState(() {
        widget.items[index] = item;
      });
    }
  }

  void updateActiveIndex(String traceId, WoxDirection woxDirection) {
    if (widget.items.isEmpty) {
      return;
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      // select next none group result
      activeIndex++;
      if (activeIndex == widget.items.length) {
        activeIndex = 0;
      }
      while (widget.items[activeIndex].isGroup) {
        activeIndex++;
        if (activeIndex == widget.items.length) {
          activeIndex = 0;
          break;
        }
      }
    }
    if (woxDirection == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      // select previous none group result
      activeIndex--;
      if (activeIndex == -1) {
        activeIndex = widget.items.length - 1;
      }
      while (widget.items[activeIndex].isGroup) {
        activeIndex--;
        if (activeIndex == -1) {
          activeIndex = widget.items.length - 1;
          break;
        }
      }
    }

    widget.onItemActive?.call(widget.items[activeIndex]);
    setState(() {});
  }

  bool isItemAtBottom(int index) {
    RenderBox? renderBox = resultGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) return false;

    if (renderBox.localToGlobal(Offset.zero).dy.ceil() >= WoxThemeUtil.instance.getQueryBoxHeight() + WoxThemeUtil.instance.getMaxResultListViewHeight()) {
      return true;
    }
    return false;
  }

  bool isItemAtTop(int index) {
    RenderBox? renderBox = resultGlobalKeys[index].currentContext?.findRenderObject() as RenderBox?;
    if (renderBox == null) return false;

    return renderBox.localToGlobal(Offset.zero).dy.ceil() <= WoxThemeUtil.instance.getQueryBoxHeight();
  }

  void changeScrollPosition(String traceId, WoxEventDeviceType deviceType, WoxDirection direction) {
    final prevResultIndex = activeIndex;
    updateActiveIndex(traceId, direction);
    if (widget.items.length < WoxThemeUtil.instance.getMaxResultCount()) {
      return;
    }

    if (direction == WoxDirectionEnum.WOX_DIRECTION_DOWN.code) {
      if (activeIndex < prevResultIndex) {
        scrollerController.jumpTo(0);
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code ? isItemAtBottom(activeIndex - 1) : !isItemAtBottom(widget.items.length - 1);
        if (shouldJump) {
          scrollerController.jumpTo(scrollerController.offset.ceil() + WoxThemeUtil.instance.getResultItemHeight() * (activeIndex - prevResultIndex).abs());
        }
      }
    }
    if (direction == WoxDirectionEnum.WOX_DIRECTION_UP.code) {
      if (activeIndex > prevResultIndex) {
        scrollerController.jumpTo(WoxThemeUtil.instance.getMaxResultListViewHeight());
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code ? isItemAtTop(activeIndex + 1) : !isItemAtTop(0);
        if (shouldJump) {
          scrollerController.jumpTo(scrollerController.offset.ceil() - WoxThemeUtil.instance.getResultItemHeight() * (activeIndex - prevResultIndex).abs());
        }
      }
    }
  }

  List<WoxListItem> getFilteredItems() {
    if (filterBoxController.text.isEmpty || !widget.showFilter) {
      return widget.items;
    }

    return widget.items.where((element) {
      // fuzzy search
      String queryText = element.title;
      if (WoxSettingUtil.instance.currentSetting.usePinYin) {
        queryText = transferChineseToPinYin(queryText).toLowerCase();
      } else {
        queryText = queryText.toLowerCase();
      }

      var score = weightedRatio(queryText, filterBoxController.text.toLowerCase());
      Logger.instance.debug(const UuidV4().generate(), "calculate fuzzy match score, queryText: $queryText, filterText: $filterBoxController.text, score: $score");
      return score > 50;
    }).toList();
  }

  String transferChineseToPinYin(String str) {
    RegExp regExp = RegExp(r'[\u4e00-\u9fa5]');
    if (regExp.hasMatch(str)) {
      return PinyinHelper.getPinyin(str, separator: "", format: PinyinFormat.WITHOUT_TONE);
    }
    return str;
  }

  @override
  Widget build(BuildContext context) {
    return ConstrainedBox(
      constraints: BoxConstraints(maxHeight: widget.maxHeight, maxWidth: 200),
      child: Column(
        children: [
          // list view
          Scrollbar(
            controller: scrollerController,
            thumbVisibility: true,
            child: Listener(
              onPointerSignal: (event) {
                if (event is PointerScrollEvent) {
                  changeScrollPosition(
                    const UuidV4().generate(),
                    WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_MOUSE.code,
                    event.scrollDelta.dy > 0 ? WoxDirectionEnum.WOX_DIRECTION_DOWN.code : WoxDirectionEnum.WOX_DIRECTION_UP.code,
                  );
                }
              },
              child: ListView.builder(
                shrinkWrap: true,
                controller: scrollerController,
                physics: const NeverScrollableScrollPhysics(),
                itemCount: getFilteredItems().length,
                itemExtent: WoxThemeUtil.instance.getResultListViewHeightByCount(1),
                itemBuilder: (context, index) {
                  WoxListItem item = getFilteredItems()[index];
                  return MouseRegion(
                    key: resultGlobalKeys[index],
                    onEnter: (_) {
                      if (isMouseMoved && !item.isGroup) {
                        Logger.instance.info(const UuidV4().generate(), "MOUSE: onenter, is mouse moved: $isMouseMoved, is group: ${item.isGroup}");
                        setState(() {
                          hoveredResultIndex = index;
                        });
                      }
                    },
                    onHover: (_) {
                      if (!isMouseMoved && !item.isGroup) {
                        Logger.instance.info(const UuidV4().generate(), "MOUSE: onHover, is mouse moved: $isMouseMoved, is group: ${item.isGroup}");
                        isMouseMoved = true;
                        setState(() {
                          hoveredResultIndex = index;
                        });
                      }
                    },
                    onExit: (_) {
                      if (!item.isGroup && hoveredResultIndex == index) {
                        setState(() {
                          hoveredResultIndex = -1;
                        });
                      }
                    },
                    child: GestureDetector(
                      onTap: () {
                        if (!item.isGroup) {
                          setState(() {
                            activeIndex = index;
                          });
                          widget.onItemActive?.call(item);
                        }
                      },
                      onDoubleTap: () {
                        if (!item.isGroup) {
                          widget.onItemExecuted?.call(item);
                        }
                      },
                      child: WoxListItemView(
                        item: WoxListItem(
                          id: item.id,
                          icon: item.icon,
                          title: item.title,
                          tails: item.tails,
                          subTitle: item.subTitle,
                          isGroup: item.isGroup,
                        ),
                        woxTheme: widget.woxTheme,
                        isActive: activeIndex == index,
                        isHovered: hoveredResultIndex == index,
                        listViewType: widget.listViewType,
                      ),
                    ),
                  );
                },
              ),
            ),
          ),

          // optional filterbox
          if (widget.showFilter)
            Focus(
                onKeyEvent: (FocusNode node, KeyEvent event) {
                  var isAnyModifierPressed = WoxHotkey.isAnyModifierPressed();
                  if (!isAnyModifierPressed) {
                    if (event is KeyDownEvent) {
                      switch (event.logicalKey) {
                        case LogicalKeyboardKey.escape:
                          widget.onFilterEscPressed?.call();
                          return KeyEventResult.handled;
                        case LogicalKeyboardKey.arrowDown:
                          changeScrollPosition(const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
                          return KeyEventResult.handled;
                        case LogicalKeyboardKey.arrowUp:
                          changeScrollPosition(const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_UP.code);
                          return KeyEventResult.handled;
                        case LogicalKeyboardKey.enter:
                          widget.onItemExecuted?.call(widget.items[activeIndex]);
                          return KeyEventResult.handled;
                      }
                    }

                    if (event is KeyRepeatEvent) {
                      switch (event.logicalKey) {
                        case LogicalKeyboardKey.arrowDown:
                          changeScrollPosition(const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
                          return KeyEventResult.handled;
                        case LogicalKeyboardKey.arrowUp:
                          changeScrollPosition(const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_UP.code);
                          return KeyEventResult.handled;
                      }
                    }
                  }

                  var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
                  if (pressedHotkey == null) {
                    return KeyEventResult.ignored;
                  }

                  // check if the pressed hotkey is matched with any item
                  WoxListItem? itemMatchedHotkey = widget.items.firstWhereOrNull((element) {
                    if (element.hotkey == null || element.hotkey!.isEmpty) {
                      return false;
                    }

                    var elementHotkey = WoxHotkey.parseHotkeyFromString(element.hotkey!);
                    if (elementHotkey != null && WoxHotkey.equals(elementHotkey.normalHotkey, pressedHotkey)) {
                      return true;
                    }

                    return false;
                  });

                  if (itemMatchedHotkey == null) {
                    return KeyEventResult.ignored;
                  } else {
                    widget.onItemExecuted?.call(itemMatchedHotkey);
                    return KeyEventResult.handled;
                  }
                },
                child: Padding(
                  padding: const EdgeInsets.only(top: 6.0),
                  child: SizedBox(
                    height: 40.0,
                    child: TextField(
                      style: TextStyle(
                        fontSize: 14.0,
                        color: fromCssColor(widget.woxTheme.actionQueryBoxFontColor),
                      ),
                      decoration: InputDecoration(
                        isCollapsed: true,
                        contentPadding: const EdgeInsets.only(
                          left: 8,
                          right: 8,
                          top: 20,
                          bottom: 18,
                        ),
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(widget.woxTheme.actionQueryBoxBorderRadius.toDouble()),
                          borderSide: BorderSide.none,
                        ),
                        filled: true,
                        fillColor: fromCssColor(widget.woxTheme.actionQueryBoxBackgroundColor),
                        hoverColor: Colors.transparent,
                      ),
                      cursorColor: fromCssColor(widget.woxTheme.queryBoxCursorColor),
                      focusNode: filterBoxFocusNode,
                      controller: filterBoxController,
                      onChanged: (value) {
                        //refresh
                        setState(() {
                          activeIndex = 0;
                        });
                      },
                    ),
                  ),
                ))
        ],
      ),
    );
  }
}

class WoxListViewController extends ChangeNotifier {
  final Map<String, _WoxListViewState> _states = {};
  String? _activeStateId;

  void _attach(_WoxListViewState state) {
    String viewId = state.widget.key?.toString() ?? const UuidV4().generate();
    _states[viewId] = state;
    _activeStateId = viewId;
  }

  void _detach() {
    if (_activeStateId != null) {
      _states.remove(_activeStateId);
      _activeStateId = _states.isNotEmpty ? _states.keys.first : null;
    }
  }

  // 获取当前活跃的State
  _WoxListViewState? get _state => _activeStateId != null ? _states[_activeStateId] : null;

  void changeScrollPosition(String traceId, WoxEventDeviceType deviceType, WoxDirection direction) {
    if (_state != null) {
      _state!.changeScrollPosition(traceId, deviceType, direction);
      notifyListeners();
    }
  }

  void update(String traceId, WoxListItem item) {
    if (_state != null) {
      _state!.update(traceId, item);
      notifyListeners();
    }
  }

  bool isItemActive(String itemId) {
    if (_state != null) {
      final itemIndex = _state!.widget.items.indexWhere((element) => element.id == itemId);
      if (itemIndex != -1) {
        return _state!.activeIndex == itemIndex;
      }
    }

    return false;
  }

  void requestFocus() {
    if (_state != null) {
      _state!.filterBoxFocusNode.requestFocus();
    }
  }
}
