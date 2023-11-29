import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controller.dart';

import 'entity.dart';

class WoxView extends GetView<WoxController> {
  WoxView({super.key});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        RawKeyboardListener(
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
            }

            return KeyEventResult.ignored;
          }),
          child: TextField(
            autofocus: true,
            controller: controller.queryTextFieldController,
            onChanged: (value) {
              final query = ChangedQuery(queryId: const UuidV4().generate(), queryType: queryTypeInput, queryText: value, querySelection: Selection.empty());
              controller.onQueryChanged(query);
            },
          ),
        ),
        ConstrainedBox(
          constraints: const BoxConstraints(maxHeight: WoxController.maxHeight),
          child: Obx(() {
            return ListView.builder(
              shrinkWrap: true,
              physics: const ClampingScrollPhysics(),
              itemCount: controller.queryResults.length,
              itemBuilder: (context, index) {
                final result = controller.queryResults[index];
                return Container(
                  color: controller.activeResultIndex.value == index ? Colors.red : Colors.transparent,
                  child: Row(
                    children: [
                      Text("image"),
                      SizedBox(width: 8),
                      Expanded(
                        child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Text(
                            result.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          Text(
                            result.subTitle,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ]),
                      ),
                    ],
                  ),
                );
              },
            );
          }),
        ),
      ],
    );
  }
}
