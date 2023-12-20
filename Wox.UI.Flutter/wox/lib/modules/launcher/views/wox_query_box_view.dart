import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/utils/log.dart';

import '../wox_launcher_controller.dart';

class WoxQueryBoxView extends GetView<WoxLauncherController> {
  const WoxQueryBoxView({super.key});

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info("repaint: query box view");

    return Obx(() {
      return Stack(children: [
        Positioned(
            child: RawKeyboardListener(
                focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
                  if (event is RawKeyDownEvent) {
                    switch (event.logicalKey) {
                      case LogicalKeyboardKey.escape:
                        controller.hideApp();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowDown:
                        controller.handleQueryBoxArrowDown();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowUp:
                        controller.handleQueryBoxArrowUp();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.enter:
                        controller.executeResultAction();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.tab:
                        controller.autoCompleteQuery();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.home:
                        controller.moveQueryBoxCursorToStart();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.end:
                        controller.moveQueryBoxCursorToEnd();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.keyJ:
                        if (event.isMetaPressed || event.isAltPressed) {
                          controller.toggleActionPanel();
                          return KeyEventResult.handled;
                        }
                    }
                  }

                  return KeyEventResult.ignored;
                }),
                child: SizedBox(
                  height: 55.0,
                  child: TextField(
                    style: TextStyle(
                      fontSize: 28.0,
                      color: fromCssColor(controller.woxTheme.value.queryBoxFontColor),
                    ),
                    decoration: InputDecoration(
                      isCollapsed: true,
                      contentPadding: const EdgeInsets.only(
                        left: 8,
                        right: 8,
                        top: 10,
                        bottom: 18,
                      ),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(controller.woxTheme.value.queryBoxBorderRadius.toDouble()),
                        borderSide: BorderSide.none,
                      ),
                      filled: true,
                      fillColor: fromCssColor(controller.woxTheme.value.queryBoxBackgroundColor),
                    ),
                    cursorColor: fromCssColor(controller.woxTheme.value.queryBoxCursorColor),
                    autofocus: true,
                    focusNode: controller.queryBoxFocusNode,
                    controller: controller.queryBoxTextFieldController,
                    scrollController: controller.queryBoxScrollController,
                    onChanged: (value) {
                      // isComposingRangeValid is not reliable on Windows, we need to use inside post frame callback to check the value
                      // see https://github.com/flutter/flutter/issues/128565#issuecomment-1772016743
                      WidgetsBinding.instance.addPostFrameCallback((_) {
                        // if the composing range is valid, which means the text is changed by IME and the query is not finished yet,
                        // we should not trigger the query until the composing is finished.
                        if (controller.queryBoxTextFieldController.value.isComposingRangeValid) {
                          return;
                        }

                        controller.canArrowUpHistory = false;
                        WoxChangeQuery woxChangeQuery = WoxChangeQuery(
                          queryId: const UuidV4().generate(),
                          queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code,
                          queryText: value,
                          querySelection: Selection.empty(),
                        );
                        controller.onQueryChanged(woxChangeQuery);
                      });
                    },
                  ),
                ))),
        Positioned(
          right: 0,
          height: 55,
          child: DragToMoveArea(child: Container(width: 100, height: 55, color: Colors.transparent)),
        )
      ]);
    });
  }
}
