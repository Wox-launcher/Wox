import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxListView extends StatefulWidget {
  final List<WoxListItem> items;
  final WoxListViewType listViewType;
  final WoxListViewController? controller;

  final Function(WoxListItem item)? onItemDoubleTap;
  final Function(WoxListItem item)? onItemActive;

  const WoxListView({
    super.key,
    required this.items,
    required this.listViewType,
    this.controller,
    this.onItemDoubleTap,
    this.onItemActive,
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

    if (renderBox.localToGlobal(Offset.zero).dy.ceil() >=
        WoxThemeUtil.instance.getQueryBoxHeight() + WoxThemeUtil.instance.getResultListViewHeightByCount(MAX_LIST_VIEW_ITEM_COUNT - 1)) {
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
    if (widget.items.length < MAX_LIST_VIEW_ITEM_COUNT) {
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
        scrollerController.jumpTo(WoxThemeUtil.instance.getResultListViewHeightByCount(widget.items.length - MAX_LIST_VIEW_ITEM_COUNT));
      } else {
        bool shouldJump = deviceType == WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code ? isItemAtTop(activeIndex + 1) : !isItemAtTop(0);
        if (shouldJump) {
          scrollerController.jumpTo(scrollerController.offset.ceil() - WoxThemeUtil.instance.getResultItemHeight() * (activeIndex - prevResultIndex).abs());
        }
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scrollbar(
      controller: scrollerController,
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
          physics: const NeverScrollableScrollPhysics(),
          controller: scrollerController,
          itemCount: widget.items.length,
          itemExtent: WoxThemeUtil.instance.getResultListViewHeightByCount(1),
          itemBuilder: (context, index) {
            WoxListItem item = widget.items[index];
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
                    widget.onItemDoubleTap?.call(item);
                  }
                },
                child: WoxListItemView(
                  item: WoxListItem(
                    id: item.id,
                    woxTheme: item.woxTheme,
                    icon: item.icon,
                    title: item.title,
                    tails: item.tails,
                    subTitle: item.subTitle,
                    isGroup: item.isGroup,
                  ),
                  isActive: activeIndex == index,
                  isHovered: hoveredResultIndex == index,
                  listViewType: widget.listViewType,
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class WoxListViewController extends ChangeNotifier {
  _WoxListViewState? _state;

  void _attach(_WoxListViewState state) {
    _state = state;
  }

  void _detach() {
    _state = null;
  }

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
}
