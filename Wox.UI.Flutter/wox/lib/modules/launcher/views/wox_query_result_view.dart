import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_list_item_view.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

import '../wox_launcher_controller.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  Widget getActionListView() {
    return Scrollbar(
        controller: controller.resultActionListViewScrollController,
        child: Listener(
            onPointerSignal: (event) {
              if (event is PointerScrollEvent) {}
            },
            child: ListView.builder(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              controller: controller.resultActionListViewScrollController,
              itemCount: controller.filterResultActions.length,
              itemExtent: 40,
              itemBuilder: (context, index) {
                WoxResultAction woxResultAction = controller.getQueryResultActionByIndex(index);
                return WoxListItemView(
                    key: controller.getResultActionItemGlobalKeyByIndex(index),
                    woxTheme: controller.woxTheme.value,
                    icon: woxResultAction.icon,
                    title: woxResultAction.name,
                    subTitle: "".obs,
                    isActive: controller.isResultActionActiveByIndex(index),
                    listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code);
              },
            )));
  }

  Widget getActionPanelView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info("repaint: action panel view container");

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
                ),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 320, minHeight: 210),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.start,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text("Actions", style: TextStyle(color: fromCssColor(controller.woxTheme.value.actionContainerHeaderFontColor), fontSize: 16.0)),
                      const Divider(),
                      ConstrainedBox(
                        constraints: const BoxConstraints(minHeight: 120),
                        child: getActionListView(),
                      ),
                      getActionQueryBox()
                    ],
                  ),
                ),
              ),
            )
          : const SizedBox(),
    );
  }

  Widget getResultView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info("repaint: result view container");

    return Obx(
      () => controller.queryResults.isNotEmpty
          ? Expanded(
              child: Container(
                padding: EdgeInsets.only(
                  top: controller.woxTheme.value.resultContainerPaddingTop.toDouble(),
                  right: controller.woxTheme.value.resultContainerPaddingRight.toDouble(),
                  bottom: controller.woxTheme.value.resultContainerPaddingBottom.toDouble(),
                  left: controller.woxTheme.value.resultContainerPaddingLeft.toDouble(),
                ),
                child: Scrollbar(
                  controller: controller.resultListViewScrollController,
                  child: Listener(
                    onPointerSignal: (event) {
                      if (event is PointerScrollEvent) {
                        controller.changeResultScrollPosition(WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_MOUSE.code,
                            event.scrollDelta.dy > 0 ? WoxDirectionEnum.WOX_DIRECTION_DOWN.code : WoxDirectionEnum.WOX_DIRECTION_UP.code);
                      }
                    },
                    child: ListView.builder(
                      shrinkWrap: true,
                      physics: const NeverScrollableScrollPhysics(),
                      controller: controller.resultListViewScrollController,
                      itemCount: controller.queryResults.length,
                      itemExtent: WoxThemeUtil.instance.getResultListViewHeightByCount(1),
                      itemBuilder: (context, index) {
                        WoxQueryResult woxQueryResult = controller.getQueryResultByIndex(index);
                        return WoxListItemView(
                            key: controller.getResultItemGlobalKeyByIndex(index),
                            woxTheme: controller.woxTheme.value,
                            icon: woxQueryResult.icon,
                            title: woxQueryResult.title,
                            subTitle: woxQueryResult.subTitle,
                            isActive: controller.isQueryResultActiveByIndex(index),
                            listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code);
                      },
                    ),
                  ),
                ),
              ),
            )
          : const SizedBox(),
    );
  }

  Widget getPreviewView() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info("repaint: preview view container");

    return Obx(
      () => controller.isShowPreviewPanel.value
          ? Expanded(
              child: WoxPreviewView(
                woxPreview: controller.currentPreview.value,
                woxTheme: controller.woxTheme.value,
              ),
            )
          : const SizedBox(),
    );
  }

// Action Query Box
  Widget getActionQueryBox() {
    return RawKeyboardListener(
        focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
          if (event is RawKeyDownEvent) {
            if (event.logicalKey == LogicalKeyboardKey.escape) {
              controller.toggleActionPanel();
              return KeyEventResult.handled;
            }
            if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
              controller.changeResultActionScrollPosition(WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
              return KeyEventResult.handled;
            }
            if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
              controller.changeResultActionScrollPosition(WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_UP.code);
              return KeyEventResult.handled;
            }
            if (event.logicalKey == LogicalKeyboardKey.enter) {
              controller.executeResultAction();
              return KeyEventResult.handled;
            }
            if (event.isMetaPressed && event.logicalKey == LogicalKeyboardKey.keyJ) {
              controller.toggleActionPanel();
              return KeyEventResult.handled;
            }
          }

          return KeyEventResult.ignored;
        }),
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
              ),
              cursorColor: fromCssColor(controller.woxTheme.value.queryBoxCursorColor),
              autofocus: true,
              focusNode: controller.resultActionFocusNode,
              controller: controller.resultActionTextFieldController,
              onChanged: (value) {
                controller.onQueryActionChanged(value);
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
