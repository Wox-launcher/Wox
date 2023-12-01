import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:window_manager/window_manager.dart';
import '../wox_launcher_controller.dart';

class WoxQueryBoxView extends GetView<WoxLauncherController> {
  const WoxQueryBoxView({super.key});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
            child: RawKeyboardListener(
                focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
                  if (event is RawKeyDownEvent) {
                    switch (event.logicalKey) {
                      case LogicalKeyboardKey.escape:
                        controller.hide();
                        break;
                      case LogicalKeyboardKey.arrowDown:
                        controller.arrowDown();
                        break;
                      case LogicalKeyboardKey.arrowUp:
                        controller.arrowUp();
                        break;
                      case LogicalKeyboardKey.enter:
                        controller.selectResult();
                        break;
                      case LogicalKeyboardKey.keyJ:
                        {
                          if (event.isMetaPressed) {
                            controller.toggleActionPanel();
                            break;
                          }
                        }
                      default:
                        return KeyEventResult.ignored;
                    }
                    return KeyEventResult.handled;
                  }
                  return KeyEventResult.ignored;
                }),
                child: TextField(
                  autofocus: true,
                  focusNode: controller.queryBoxFocusNode,
                  controller: controller.queryBoxTextFieldController,
                ))),
        DragToMoveArea(child: Container(width: 100, height: 48, color: Colors.transparent)),
      ],
    );
  }
}
