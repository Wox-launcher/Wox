import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/components/wox_result_item_view.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/utils/wox_theme_util.dart';

import '../wox_launcher_controller.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

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
                      child: Scrollbar(
                          controller: controller.scrollController,
                          child: Listener(
                              onPointerSignal: (event) {
                                if (event is PointerScrollEvent) {
                                  controller.changeScrollPosition(WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_MOUSE.code,
                                      event.scrollDelta.dy > 0 ? WoxDirectionEnum.WOX_DIRECTION_DOWN.code : WoxDirectionEnum.WOX_DIRECTION_UP.code);
                                }
                              },
                              child: ListView.builder(
                                shrinkWrap: true,
                                physics: const NeverScrollableScrollPhysics(),
                                controller: controller.scrollController,
                                itemCount: controller.queryResults.length,
                                itemExtent: WoxThemeUtil.instance.getResultListViewHeightByCount(1),
                                itemBuilder: (context, index) {
                                  WoxQueryResult queryResult = controller.getQueryResultByIndex(index);
                                  return WoxResultItemView(
                                      key: controller.getResultItemGlobalKeyByIndex(index),
                                      woxTheme: controller.woxTheme.value,
                                      icon: queryResult.icon,
                                      title: queryResult.title,
                                      subTitle: queryResult.subTitle,
                                      isActive: index == controller.activeResultIndex.value);
                                },
                              )))),
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
                color: Colors.red,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxHeight: 200, maxWidth: 200),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.start,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Text("Actions"),
                      const Divider(),
                      Row(
                        children: [for (var action in controller.resultActions) Text(action.name)],
                      ),
                      RawKeyboardListener(
                        focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
                          if (event is RawKeyDownEvent) {
                            if (event.logicalKey == LogicalKeyboardKey.escape) {
                              controller.toggleActionPanel();
                              return KeyEventResult.handled;
                            }
                            if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
                              return KeyEventResult.handled;
                            }
                            if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
                              return KeyEventResult.handled;
                            }
                            if (event.logicalKey == LogicalKeyboardKey.enter) {
                              return KeyEventResult.handled;
                            }
                            if (event.isMetaPressed && event.logicalKey == LogicalKeyboardKey.keyJ) {
                              controller.toggleActionPanel();
                              return KeyEventResult.handled;
                            }
                          }
                          return KeyEventResult.ignored;
                        }),
                        child: TextField(
                          focusNode: controller.resultActionFocusNode,
                          controller: controller.resultActionTextFieldController,
                          onChanged: (value) {
                            // controller.onActionQueryChanged(value);
                          },
                        ),
                      ),
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
