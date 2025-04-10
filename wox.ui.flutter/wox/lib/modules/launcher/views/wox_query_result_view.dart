import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_list_item.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

import '../wox_launcher_controller.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  RxList<WoxQueryResultTail> getHotkeyTails(WoxResultAction action) {
    var tails = <WoxQueryResultTail>[];
    if (action.hotkey != "") {
      var hotkey = WoxHotkey.parseHotkeyFromString(action.hotkey);
      if (hotkey != null) {
        tails.add(WoxQueryResultTail.hotkey(hotkey));
      }
    }
    return tails.obs;
  }

  Widget getActionListView() {
    return Obx(() {
      if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: action list view container");

      return Scrollbar(
          controller: controller.actionScrollerController,
          child: Listener(
              onPointerSignal: (event) {
                if (event is PointerScrollEvent) {}
              },
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxHeight: 350),
                child: ListView.builder(
                  shrinkWrap: true,
                  controller: controller.actionScrollerController,
                  itemCount: controller.actions.length,
                  itemExtent: 40,
                  itemBuilder: (context, index) {
                    WoxResultAction woxResultAction = controller.geActionByIndex(index);
                    return WoxListItemView(
                      listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code,
                      key: ValueKey(woxResultAction.id),
                      item: WoxListItem(
                        id: woxResultAction.id,
                        woxTheme: controller.woxTheme.value,
                        icon: woxResultAction.icon.value,
                        title: woxResultAction.name.value,
                        tails: getHotkeyTails(woxResultAction),
                        subTitle: "",
                        isGroup: false,
                      ),
                      isActive: controller.isActionActiveByIndex(index),
                      isHovered: controller.isResultHoveredByIndex(index),
                    );
                  },
                ),
              )));
    });
  }

  Widget getActionPanelView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: action panel view container");

    return Obx(
      () => controller.isShowActionPanel.value
          ? Positioned(
              right: 10,
              bottom: 10,
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
                  boxShadow: [
                    BoxShadow(
                      color: Colors.black.withOpacity(0.1),
                      spreadRadius: 2,
                      blurRadius: 8,
                      offset: const Offset(0, 3),
                    ),
                  ],
                ),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 320),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.start,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(controller.actionsTitle.value, style: TextStyle(color: fromCssColor(controller.woxTheme.value.actionContainerHeaderFontColor), fontSize: 16.0)),
                      const Divider(),
                      getActionListView(),
                      getActionQueryBox()
                    ],
                  ),
                ),
              ),
            )
          : const SizedBox(),
    );
  }

  Widget getResultContainer() {
    return Container(
      padding: EdgeInsets.only(
        top: controller.woxTheme.value.resultContainerPaddingTop.toDouble(),
        right: controller.woxTheme.value.resultContainerPaddingRight.toDouble(),
        bottom: controller.woxTheme.value.resultContainerPaddingBottom.toDouble(),
        left: controller.woxTheme.value.resultContainerPaddingLeft.toDouble(),
      ),
      child: WoxListView(
        controller: controller.resultListViewController,
        listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code,
        items: controller.results
            .map((result) => WoxListItem(
                  id: result.id,
                  woxTheme: controller.woxTheme.value,
                  icon: result.icon.value,
                  title: result.title.value,
                  tails: result.tails,
                  subTitle: result.subTitle.value,
                  isGroup: result.isGroup,
                ))
            .toList(),
        onItemDoubleTap: (item) {
          controller.onEnter(const UuidV4().generate());
        },
        onItemActive: (item) {
          controller.onResultItemActivated(const UuidV4().generate(), item);
        },
      ),
    );
  }

  Widget getResultView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: result view container");

    return Obx(
      () => controller.results.isNotEmpty
          ? controller.isShowPreviewPanel.value
              ? Flexible(
                  flex: (controller.resultPreviewRatio.value * 100).toInt(),
                  child: getResultContainer(),
                )
              : Expanded(
                  child: getResultContainer(),
                )
          : const SizedBox(),
    );
  }

  Widget getPreviewView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: preview view container");

    return Obx(
      () => controller.isShowPreviewPanel.value
          ? Flexible(
              flex: (100 - controller.resultPreviewRatio.value * 100).toInt(),
              child: controller.currentPreview.value.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code
                  ? FutureBuilder(
                      future: controller.currentPreview.value.unWrap(),
                      builder: (context, snapshot) {
                        if (snapshot.hasData) {
                          return WoxPreviewView(
                            woxPreview: snapshot.data!,
                            woxTheme: controller.woxTheme.value,
                          );
                        } else if (snapshot.hasError) {
                          return Text("${snapshot.error}");
                        } else {
                          return const SizedBox();
                        }
                      },
                    )
                  : WoxPreviewView(
                      woxPreview: controller.currentPreview.value,
                      woxTheme: controller.woxTheme.value,
                    ),
            )
          : const SizedBox(),
    );
  }

// Action Query Box
  Widget getActionQueryBox() {
    return Focus(
        onKeyEvent: (FocusNode node, KeyEvent event) {
          var isAnyModifierPressed = WoxHotkey.isAnyModifierPressed();
          if (!isAnyModifierPressed) {
            if (event is KeyDownEvent) {
              switch (event.logicalKey) {
                case LogicalKeyboardKey.escape:
                  controller.actionEscFunction(const UuidV4().generate());
                  return KeyEventResult.handled;
                case LogicalKeyboardKey.arrowDown:
                  controller.changeActionScrollPosition(
                      const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
                  return KeyEventResult.handled;
                case LogicalKeyboardKey.arrowUp:
                  controller.changeActionScrollPosition(
                      const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_UP.code);
                  return KeyEventResult.handled;
                case LogicalKeyboardKey.enter:
                  controller.onEnter(const UuidV4().generate());
                  return KeyEventResult.handled;
              }
            }

            if (event is KeyRepeatEvent) {
              switch (event.logicalKey) {
                case LogicalKeyboardKey.arrowDown:
                  controller.changeActionScrollPosition(
                      const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
                  return KeyEventResult.handled;
                case LogicalKeyboardKey.arrowUp:
                  controller.changeActionScrollPosition(
                      const UuidV4().generate(), WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_UP.code);
                  return KeyEventResult.handled;
              }
            }
          }

          var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
          if (pressedHotkey == null) {
            return KeyEventResult.ignored;
          }

          // list all actions
          if (controller.isActionHotkey(pressedHotkey)) {
            controller.toggleActionPanel(const UuidV4().generate());
            return KeyEventResult.handled;
          }

          // check if the pressed hotkey is the action hotkey
          var result = controller.getActiveResult();
          var action = controller.getActionByHotkey(result, pressedHotkey);
          if (action != null) {
            controller.executeAction(const UuidV4().generate(), result, action);
            return KeyEventResult.handled;
          }

          return KeyEventResult.ignored;
        },
        child: Padding(
          padding: const EdgeInsets.only(top: 6.0),
          child: SizedBox(
            height: 40.0,
            child: TextField(
              style: TextStyle(
                fontSize: 14.0,
                color: fromCssColor(controller.woxTheme.value.actionQueryBoxFontColor),
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
                  borderRadius: BorderRadius.circular(controller.woxTheme.value.actionQueryBoxBorderRadius.toDouble()),
                  borderSide: BorderSide.none,
                ),
                filled: true,
                fillColor: fromCssColor(controller.woxTheme.value.actionQueryBoxBackgroundColor),
                hoverColor: Colors.transparent,
              ),
              cursorColor: fromCssColor(controller.woxTheme.value.queryBoxCursorColor),
              autofocus: true,
              focusNode: controller.actionFocusNode,
              controller: controller.actionTextFieldController,
              onChanged: (value) {
                controller.onActionQueryBoxTextChanged(const UuidV4().generate(), value);
              },
            ),
          ),
        ));
  }

  @override
  Widget build(BuildContext context) {
    return ConstrainedBox(
      constraints: BoxConstraints(maxHeight: WoxThemeUtil.instance.getResultContainerMaxHeight()),
      child: Obx(() => Stack(
            fit: controller.isShowActionPanel.value || controller.isShowPreviewPanel.value ? StackFit.expand : StackFit.loose,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  getResultView(),
                  getPreviewView(),
                ],
              ),
              getActionPanelView()
            ],
          )),
    );
  }
}
