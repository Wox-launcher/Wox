import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/components/wox_preview_view.dart';
import 'package:wox/controller.dart';

import 'components/wox_result_view.dart';
import 'entity.dart';

class WoxView extends GetView<WoxController> {
  const WoxView({super.key});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Row(
          children: [
            Expanded(
              child: RawKeyboardListener(
                focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
                  if (event is RawKeyDownEvent) {
                    if (event.logicalKey == LogicalKeyboardKey.escape) {
                      controller.hide();
                      return KeyEventResult.handled;
                    }
                    if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
                      controller.arrowDown();
                      return KeyEventResult.handled;
                    }
                    if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
                      controller.arrowUp();
                      return KeyEventResult.handled;
                    }
                    if (event.logicalKey == LogicalKeyboardKey.enter) {
                      controller.selectResult();
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
                  autofocus: true,
                  focusNode: controller.queryFocusNode,
                  controller: controller.queryTextFieldController,
                  onChanged: (value) {
                    final query = ChangedQuery(queryId: const UuidV4().generate(), queryType: queryTypeInput, queryText: value, querySelection: Selection.empty());
                    controller.onQueryChanged(query);
                  },
                ),
              ),
            ),
            DragToMoveArea(child: Container(width: 100, height: 48, color: Colors.red)),
          ],
        ),
        Obx(() {
          return ConstrainedBox(
            constraints: const BoxConstraints(maxHeight: WoxController.maxHeight),
            child: Stack(
              fit: (controller.isShowActionPanel.value || controller.isShowPreviewPanel.value) ? StackFit.expand : StackFit.loose,
              children: [
                Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Results
                    Expanded(
                      child: ListView.builder(
                        shrinkWrap: true,
                        physics: const AlwaysScrollableScrollPhysics(),
                        itemCount: controller.queryResults.length,
                        itemExtent: 40,
                        itemBuilder: (context, index) {
                          return WoxResultView(result: controller.queryResults[index], isActive: index == controller.activeResultIndex.value);
                        },
                      ),
                    ),

                    // Preview
                    if (controller.isShowPreviewPanel.value) Expanded(child: WoxPreviewView(woxPreview: controller.currentPreview.value)),
                  ],
                ),

                // Action Panel
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
                            for (var action in controller.actionResults)
                              Row(
                                children: [
                                  Text(action.name),
                                ],
                              ),
                            RawKeyboardListener(
                              focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
                                if (event is RawKeyDownEvent) {
                                  if (event.logicalKey == LogicalKeyboardKey.escape) {
                                    controller.toggleActionPanel();
                                    return KeyEventResult.handled;
                                  }
                                  if (event.logicalKey == LogicalKeyboardKey.arrowDown) {
                                    controller.arrowDownAction();
                                    return KeyEventResult.handled;
                                  }
                                  if (event.logicalKey == LogicalKeyboardKey.arrowUp) {
                                    controller.arrowUpAction();
                                    return KeyEventResult.handled;
                                  }
                                  if (event.logicalKey == LogicalKeyboardKey.enter) {
                                    controller.selectAction();
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
                                focusNode: controller.actionFocusNode,
                                controller: controller.actionTextFieldController,
                                onChanged: (value) {
                                  controller.onActionQueryChanged(value);
                                },
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          );
        }),
      ],
    );
  }
}
