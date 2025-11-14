import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:hotkey_manager/hotkey_manager.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/controllers/wox_list_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxListView<T> extends StatelessWidget {
  final WoxListController<T> controller;
  final WoxListViewType listViewType;
  final bool showFilter;
  final double maxHeight;
  final VoidCallback? onItemTapped;
  final bool Function(String traceId, HotKey hotkey)? onFilteHotkeyPressed;

  const WoxListView({
    super.key,
    required this.controller,
    required this.listViewType,
    this.showFilter = true,
    required this.maxHeight,
    this.onItemTapped,
    this.onFilteHotkeyPressed,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        ConstrainedBox(
          constraints: BoxConstraints(
            maxHeight: maxHeight,
          ),
          child: Scrollbar(
            thumbVisibility: true,
            controller: controller.scrollController,
            child: Listener(
              onPointerSignal: (event) {
                if (event is PointerScrollEvent) {
                  controller.updateActiveIndexByDirection(
                    const UuidV4().generate(),
                    event.scrollDelta.dy > 0 ? WoxDirectionEnum.WOX_DIRECTION_DOWN.code : WoxDirectionEnum.WOX_DIRECTION_UP.code,
                  );
                }
              },
              child: Obx(
                () => controller.items.isEmpty && controller.filterBoxController.text.isNotEmpty
                    ? SizedBox(
                        height: WoxThemeUtil.instance.getResultListViewHeightByCount(1),
                        child: Center(
                          child: Text(
                            Get.find<WoxSettingController>().tr('ui_no_matches'),
                            style: TextStyle(
                              fontSize: 14.0,
                              color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxFontColor).withOpacity(0.5),
                            ),
                          ),
                        ),
                      )
                    : AnimatedSwitcher(
                        duration: Duration.zero,
                        child: ListView.builder(
                          key: ValueKey(controller.items.length),
                          shrinkWrap: true,
                          controller: controller.scrollController,
                          physics: const NeverScrollableScrollPhysics(),
                          itemCount: controller.items.length,
                          itemExtent: WoxThemeUtil.instance.getResultListViewHeightByCount(1),
                          itemBuilder: (context, index) {
                            var item = controller.items[index];
                            return MouseRegion(
                              onEnter: (_) {
                                if (controller.isMouseMoved && !item.value.isGroup) {
                                  controller.updateHoveredIndex(index);
                                }
                              },
                              onHover: (_) {
                                if (!controller.isMouseMoved && !item.value.isGroup) {
                                  controller.isMouseMoved = true;
                                  controller.updateHoveredIndex(index);
                                }
                              },
                              onExit: (_) {
                                if (!item.value.isGroup && controller.hoveredIndex.value == index) {
                                  controller.clearHoveredResult();
                                }
                              },
                              child: _WoxListItemGestureWrapper<T>(
                                controller: controller,
                                index: index,
                                item: item,
                                onItemTapped: () {
                                  onItemTapped?.call();
                                },
                                child: Obx(
                                  () => WoxListItemView(
                                    key: ValueKey(item.value.id),
                                    item: item.value,
                                    woxTheme: WoxThemeUtil.instance.currentTheme.value,
                                    isActive: controller.activeIndex.value == index,
                                    isHovered: controller.hoveredIndex.value == index,
                                    listViewType: listViewType,
                                  ),
                                ),
                              ),
                            );
                          },
                        ),
                      ),
              ),
            ),
          ),
        ),
        if (showFilter)
          WoxPlatformFocus(
            onKeyEvent: (FocusNode node, KeyEvent event) {
              var traceId = const UuidV4().generate();
              var isAnyModifierPressed = WoxHotkey.isAnyModifierPressed();
              if (!isAnyModifierPressed) {
                if (event is KeyDownEvent) {
                  switch (event.logicalKey) {
                    case LogicalKeyboardKey.escape:
                      controller.onFilterBoxEscPressed?.call(traceId);
                      return KeyEventResult.handled;
                    case LogicalKeyboardKey.arrowDown:
                      controller.updateActiveIndexByDirection(traceId, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
                      return KeyEventResult.handled;
                    case LogicalKeyboardKey.arrowUp:
                      controller.updateActiveIndexByDirection(traceId, WoxDirectionEnum.WOX_DIRECTION_UP.code);
                      return KeyEventResult.handled;
                    case LogicalKeyboardKey.enter:
                      if (controller.items.isNotEmpty && controller.activeIndex.value < controller.items.length) {
                        controller.onItemExecuted?.call(traceId, controller.items[controller.activeIndex.value].value);
                      }
                      return KeyEventResult.handled;
                  }
                }

                if (event is KeyRepeatEvent) {
                  switch (event.logicalKey) {
                    case LogicalKeyboardKey.arrowDown:
                      controller.updateActiveIndexByDirection(
                        traceId,
                        WoxDirectionEnum.WOX_DIRECTION_DOWN.code,
                      );
                      return KeyEventResult.handled;
                    case LogicalKeyboardKey.arrowUp:
                      controller.updateActiveIndexByDirection(
                        traceId,
                        WoxDirectionEnum.WOX_DIRECTION_UP.code,
                      );
                      return KeyEventResult.handled;
                  }
                }
              }

              var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
              if (pressedHotkey == null) {
                return KeyEventResult.ignored;
              }

              // Let caller handle hotkey first
              if (onFilteHotkeyPressed != null && onFilteHotkeyPressed!(traceId, pressedHotkey)) {
                return KeyEventResult.handled;
              }

              Rx<WoxListItem<T>>? itemMatchedHotkey = controller.items.firstWhereOrNull((element) {
                if (element.value.hotkey == null || element.value.hotkey!.isEmpty) {
                  return false;
                }

                var elementHotkey = WoxHotkey.parseHotkeyFromString(element.value.hotkey!);
                if (elementHotkey != null && WoxHotkey.equals(elementHotkey.normalHotkey, pressedHotkey)) {
                  return true;
                }

                return false;
              });

              if (itemMatchedHotkey == null) {
                return KeyEventResult.ignored;
              } else {
                controller.onItemExecuted?.call(traceId, itemMatchedHotkey.value);
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
                    color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionQueryBoxFontColor),
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
                      borderRadius: BorderRadius.circular(WoxThemeUtil.instance.currentTheme.value.actionQueryBoxBorderRadius.toDouble()),
                      borderSide: BorderSide.none,
                    ),
                    filled: true,
                    fillColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.actionQueryBoxBackgroundColor),
                    hoverColor: Colors.transparent,
                  ),
                  cursorColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxCursorColor),
                  focusNode: controller.filterBoxFocusNode,
                  controller: controller.filterBoxController,
                  onChanged: (value) {
                    controller.filterItems(const UuidV4().generate(), value);
                  },
                ),
              ),
            ),
          ),
      ],
    );
  }
}

class _WoxListItemGestureWrapper<T> extends StatefulWidget {
  final WoxListController<T> controller;
  final int index;
  final Rx<WoxListItem<T>> item;
  final Widget child;
  final VoidCallback? onItemTapped;

  const _WoxListItemGestureWrapper({
    required this.controller,
    required this.index,
    required this.item,
    required this.child,
    this.onItemTapped,
  });

  @override
  State<_WoxListItemGestureWrapper<T>> createState() => _WoxListItemGestureWrapperState<T>();
}

class _WoxListItemGestureWrapperState<T> extends State<_WoxListItemGestureWrapper<T>> {
  DateTime? _lastTapTime;
  static const _doubleClickThreshold = Duration(milliseconds: 200);

  void _handleTapDown() {
    if (widget.item.value.isGroup) {
      return;
    }

    final traceId = const UuidV4().generate();
    final now = DateTime.now();

    // Double click
    if (_lastTapTime != null && now.difference(_lastTapTime!) <= _doubleClickThreshold) {
      widget.controller.onItemExecuted?.call(traceId, widget.item.value);
      widget.onItemTapped?.call();
      _lastTapTime = null;
      return;
    }

    // Single click
    widget.controller.updateActiveIndex(traceId, widget.index);
    widget.controller.onItemActive?.call(traceId, widget.item.value);

    widget.onItemTapped?.call();

    _lastTapTime = now;
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTapDown: (_) => _handleTapDown(),
      child: widget.child,
    );
  }
}
