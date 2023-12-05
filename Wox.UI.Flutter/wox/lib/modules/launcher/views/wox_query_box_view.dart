import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_query_type_enum.dart';

import '../wox_launcher_controller.dart';

class WoxQueryBoxView extends GetView<WoxLauncherController> {
  const WoxQueryBoxView({super.key});

  @override
  Widget build(BuildContext context) {
    return Stack(children: [
      Positioned(
          child: RawKeyboardListener(
              focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
                if (event is RawKeyDownEvent) {
                  switch (event.logicalKey) {
                    case LogicalKeyboardKey.escape:
                      controller.hideApp();
                      break;
                    case LogicalKeyboardKey.arrowDown:
                      controller.arrowDown();
                      break;
                    case LogicalKeyboardKey.arrowUp:
                      controller.arrowUp();
                      break;
                    case LogicalKeyboardKey.enter:
                      controller.handleResultItemAction();
                      break;
                    case LogicalKeyboardKey.keyJ:
                      {
                        if (event.isMetaPressed) {
                          controller.toggleActionPanel();
                          break;
                        }
                        return KeyEventResult.ignored;
                      }
                    default:
                      return KeyEventResult.ignored;
                  }
                  return KeyEventResult.handled;
                }
                return KeyEventResult.ignored;
              }),
              child: SizedBox(
                height: 55.0,
                child: TextField(
                  style: TextStyle(
                    fontSize: 24.0,
                    color: fromCssColor(controller.woxTheme.queryBoxFontColor),
                  ),
                  decoration: InputDecoration(
                    isCollapsed: true,
                    contentPadding: const EdgeInsets.only(
                      left: 8,
                      right: 8,
                      top: 20,
                      bottom: 20,
                    ),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(controller.woxTheme.queryBoxBorderRadius.toDouble()),
                      borderSide: BorderSide.none,
                    ),
                    filled: true,
                    fillColor: fromCssColor(controller.woxTheme.queryBoxBackgroundColor),
                  ),
                  cursorColor: Colors.white,
                  autofocus: true,
                  focusNode: controller.queryBoxFocusNode,
                  controller: controller.queryBoxTextFieldController,
                  onChanged: (value) {
                    WoxChangeQuery woxChangeQuery = WoxChangeQuery(
                        queryId: const UuidV4().generate(), queryType: WoxQueryTypeEnum.WOX_QUERY_TYPE_INPUT.code, queryText: value, querySelection: Selection.empty());
                    controller.onQueryChanged(woxChangeQuery);
                  },
                ),
              ))),
      Positioned(
        right: 0,
        height: 55,
        child: DragToMoveArea(child: Container(width: 100, height: 55, color: Colors.transparent)),
      )
    ]);
  }
}
