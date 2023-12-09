import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_list_view.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_list_view_type_enum.dart';
import 'package:wox/utils/wox_theme_util.dart';

import '../wox_launcher_controller.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  // Result List View
  Widget getResultListView() {
    return WoxListView(
      woxTheme: controller.woxTheme.value,
      scrollController: controller.resultListViewScrollController,
      itemCount: controller.queryResults.length,
      itemExtent: WoxThemeUtil.instance.getResultListViewHeightByCount(1),
      listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_RESULT.code,
      listViewItemParamsGetter: (index) {
        WoxQueryResult woxQueryResult = controller.getQueryResultByIndex(index);
        return WoxListViewItemParams.fromJson({
          "Icon": woxQueryResult.icon,
          "Title": woxQueryResult.title,
          "SubTitle": woxQueryResult.subTitle,
        });
      },
      listViewItemGlobalKeyGetter: (index) {
        return controller.getResultItemGlobalKeyByIndex(index);
      },
      listViewItemIsActiveGetter: (index) {
        return controller.isQueryResultActiveByIndex(index);
      },
      onMouseWheelScroll: (event) {
        controller.changeResultScrollPosition(
            WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_MOUSE.code, event.scrollDelta.dy > 0 ? WoxDirectionEnum.WOX_DIRECTION_DOWN.code : WoxDirectionEnum.WOX_DIRECTION_UP.code);
      },
    );
  }

  Widget getResultActionListView() {
    return WoxListView(
      woxTheme: controller.woxTheme.value,
      scrollController: controller.resultActionListViewScrollController,
      itemCount: controller.filterResultActions.length,
      itemExtent: 40,
      listViewType: WoxListViewTypeEnum.WOX_LIST_VIEW_TYPE_ACTION.code,
      listViewItemParamsGetter: (index) {
        WoxResultAction woxResultAction = controller.getQueryResultActionByIndex(index);
        return WoxListViewItemParams.fromJson({
          "Icon": woxResultAction.icon,
          "Title": woxResultAction.name,
        });
      },
      listViewItemGlobalKeyGetter: (index) {
        return controller.getResultActionItemGlobalKeyByIndex(index);
      },
      listViewItemIsActiveGetter: (index) {
        return controller.isResultActionActiveByIndex(index);
      },
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
    return Obx(() {
      return ConstrainedBox(
        constraints: BoxConstraints(maxHeight: WoxThemeUtil.instance.getResultContainerMaxHeight()),
        child: Stack(fit: (controller.isShowActionPanel.value || controller.isShowPreviewPanel.value) ? StackFit.expand : StackFit.loose, children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (controller.queryResults.isNotEmpty)
                Expanded(
                  child: Container(
                      padding: EdgeInsets.only(
                        top: controller.woxTheme.value.resultContainerPaddingTop.toDouble(),
                        right: controller.woxTheme.value.resultContainerPaddingRight.toDouble(),
                        bottom: controller.woxTheme.value.resultContainerPaddingBottom.toDouble(),
                        left: controller.woxTheme.value.resultContainerPaddingLeft.toDouble(),
                      ),
                      child: getResultListView()),
                ),
              if (controller.isShowPreviewPanel.value)
                Expanded(
                    child: WoxPreviewView(
                  woxPreview: controller.currentPreview.value,
                  woxTheme: controller.woxTheme.value,
                )),
            ],
          ),
          if (controller.isShowActionPanel.value)
            Positioned(
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
                  constraints: const BoxConstraints(maxWidth: 320),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.start,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text("Actions", style: TextStyle(color: fromCssColor(controller.woxTheme.value.actionContainerHeaderFontColor), fontSize: 16.0)),
                      const Divider(),
                      getResultActionListView(),
                      getActionQueryBox(),
                    ],
                  ),
                ),
              ),
            ),
        ]),
      );
    });
  }
}
