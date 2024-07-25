import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/log.dart';

class WoxQueryBoxView extends GetView<WoxLauncherController> {
  const WoxQueryBoxView({super.key});

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: query box view");

    return Obx(() {
      return Stack(children: [
        Positioned(
            child: Focus(
                autofocus: true,
                onKeyEvent: (FocusNode node, KeyEvent event) {
                  if (event is KeyDownEvent) {
                    switch (event.logicalKey) {
                      case LogicalKeyboardKey.escape:
                        controller.hideApp(const UuidV4().generate());
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowDown:
                        controller.handleQueryBoxArrowDown();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowUp:
                        controller.handleQueryBoxArrowUp();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.enter:
                        controller.executeResultAction(const UuidV4().generate());
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.tab:
                        controller.autoCompleteQuery(const UuidV4().generate());
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.home:
                        controller.moveQueryBoxCursorToStart();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.end:
                        controller.moveQueryBoxCursorToEnd();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.keyJ:
                        if (HardwareKeyboard.instance.isMetaPressed || HardwareKeyboard.instance.isAltPressed) {
                          controller.toggleActionPanel(const UuidV4().generate());
                          return KeyEventResult.handled;
                        }
                    }
                  }

                  if (event is KeyRepeatEvent) {
                    switch (event.logicalKey) {
                      case LogicalKeyboardKey.arrowDown:
                        controller.handleQueryBoxArrowDown();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowUp:
                        controller.handleQueryBoxArrowUp();
                        return KeyEventResult.handled;
                    }
                  }

                  return KeyEventResult.ignored;
                },
                child: SizedBox(
                  height: 55.0,
                  child: Theme(
                    data: ThemeData(
                      textSelectionTheme: TextSelectionThemeData(
                        selectionColor: fromCssColor(controller.woxTheme.value.queryBoxTextSelectionColor),
                      ),
                    ),
                    child: TextField(
                      style: TextStyle(
                        fontSize: 28.0,
                        color: fromCssColor(controller.woxTheme.value.queryBoxFontColor),
                      ),
                      decoration: InputDecoration(
                        contentPadding: const EdgeInsets.only(
                          left: 8,
                          right: 68,
                          top: 4,
                          bottom: 17,
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

                          controller.onQueryBoxTextChanged(value);
                        });
                      },
                    ),
                  ),
                ))),
        Positioned(
          right: 10,
          height: 55,
          child: DragToMoveArea(
              child: Container(
            width: 55,
            height: 55,
            color: Colors.transparent,
            child: Padding(
              padding: const EdgeInsets.all(8.0),
              child: WoxImageView(woxImage: controller.queryIcon.value, width: 24, height: 24),
            ),
          )),
        ),
      ]);
    });
  }
}
