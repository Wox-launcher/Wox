import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/enums/wox_direction_enum.dart';
import 'package:wox/enums/wox_event_device_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';

import '../wox_launcher_controller.dart';

class WoxQueryBoxView extends GetView<WoxLauncherController> {
  const WoxQueryBoxView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
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
                        controller.changeScrollPosition(WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_DOWN.code);
                        break;
                      case LogicalKeyboardKey.arrowUp:
                        controller.changeScrollPosition(WoxEventDeviceTypeEnum.WOX_EVENT_DEVEICE_TYPE_KEYBOARD.code, WoxDirectionEnum.WOX_DIRECTION_UP.code);
                        break;
                      case LogicalKeyboardKey.enter:
                        controller.executeResultAction();
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
                      color: fromCssColor(controller.woxTheme.value.queryBoxFontColor),
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
    });
  }
}
