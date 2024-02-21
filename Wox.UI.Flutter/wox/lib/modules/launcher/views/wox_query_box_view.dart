import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/enums/wox_query_type_enum.dart';
import 'package:wox/enums/wox_selection_type_enum.dart';
import 'package:wox/utils/log.dart';

import '../wox_launcher_controller.dart';

class WoxQueryBoxView extends GetView<WoxLauncherController> {
  const WoxQueryBoxView({super.key});

  Widget getQueryTypeIcon() {
    if (controller.getCurrentQuery().queryType == WoxQueryTypeEnum.WOX_QUERY_TYPE_SELECTION.code) {
      if (controller.getCurrentQuery().querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_FILE.code) {
        return WoxImageView(
            woxImage: WoxImage(
                imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code,
                imageData:
                    '<svg t="1704957058350" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="4383" width="200" height="200"><path d="M127.921872 233.342828H852.118006c24.16765 0 43.960122 19.792472 43.960122 43.960122v522.104578c0 24.16765-19.792472 43.960122-43.960122 43.960122H172.090336c-24.16765 0-43.960122-19.792472-43.960122-43.960122L127.921872 233.342828z" fill="#FFB300" p-id="4384"></path><path d="M156.4647 180.63235h312.721058c15.625636 0 28.334486 13.125534 28.334486 29.376195V233.342828H127.921872v-23.334283c0-16.250661 12.917192-29.376195 28.542828-29.376195z" fill="#FFA000" p-id="4385"></path><path d="M361.889725 258.343845h348.347508v535.855138H312.512716V303.137335z" fill="#FFFFFF" p-id="4386"></path><path d="M170.631943 372.723499h282.719837l59.7941-47.918616H852.118006c23.542625 0 42.710071 19.792472 42.710071 43.960122v430.642523c0 24.16765-19.167447 43.960122-42.710071 43.960122H170.631943c-23.542625 0-42.710071-19.792472-42.710071-43.960122V416.683622c0-24.16765 19.375788-43.960122 42.710071-43.960123z" fill="#FFD54F" p-id="4387"></path><path d="M361.473042 303.76236l-48.960326-0.625025 48.960326-44.79349z" fill="#BDBDBD" p-id="4388"></path></svg>'));
      }
      if (controller.getCurrentQuery().querySelection.type == WoxSelectionTypeEnum.WOX_SELECTION_TYPE_TEXT.code) {
        return WoxImageView(
          woxImage: WoxImage(
              imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code,
              imageData:
                  '<svg t="1704958243895" class="icon" viewBox="0 0 1024 1024" version="1.1" xmlns="http://www.w3.org/2000/svg" p-id="5762" width="200" height="200"><path d="M925.48105 1024H98.092461a98.51895 98.51895 0 0 1-97.879217-98.945439V98.732195A98.732195 98.732195 0 0 1 98.092461 0.426489h827.388589a98.732195 98.732195 0 0 1 98.305706 98.305706v826.322366a98.51895 98.51895 0 0 1-98.305706 98.945439z m-829.094544-959.600167a33.052895 33.052895 0 0 0-32.199917 32.626406v829.734277a32.83965 32.83965 0 0 0 32.199917 33.26614h831.653477a32.83965 32.83965 0 0 0 31.773428-33.26614V97.026239a33.266139 33.266139 0 0 0-32.626406-32.626406z" fill="#0077F0" p-id="5763"></path><path d="M281.69596 230.943773h460.60808v73.569347h-187.655144v488.969596h-85.297792V304.51312h-187.655144z" fill="#0077F0" opacity=".5" p-id="5764"></path></svg>'),
        );
      }
    }

    return const SizedBox(width: 24, height: 24);
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.info(const UuidV4().generate(), "repaint: query box view");

    return Obx(() {
      return Stack(children: [
        Positioned(
            child: RawKeyboardListener(
                focusNode: FocusNode(onKey: (FocusNode node, RawKeyEvent event) {
                  if (event is RawKeyDownEvent) {
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
                        if (event.isMetaPressed || event.isAltPressed) {
                          controller.toggleActionPanel(const UuidV4().generate());
                          return KeyEventResult.handled;
                        }
                    }
                  }

                  return KeyEventResult.ignored;
                }),
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
                        isCollapsed: true,
                        contentPadding: const EdgeInsets.only(
                          left: 8,
                          right: 8,
                          top: 6,
                          bottom: 14,
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
              child: getQueryTypeIcon(),
            ),
          )),
        )
      ]);
    });
  }
}
