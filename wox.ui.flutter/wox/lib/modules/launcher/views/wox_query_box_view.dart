import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_drag_move_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxQueryBoxView extends GetView<WoxLauncherController> {
  const WoxQueryBoxView({super.key});

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: query box view");

    return Obx(() {
      return Stack(children: [
        Positioned(
            child: Focus(
                onKeyEvent: (FocusNode node, KeyEvent event) {
                  var isAnyModifierPressed = WoxHotkey.isAnyModifierPressed();
                  if (!isAnyModifierPressed) {
                    if (event is KeyDownEvent) {
                      switch (event.logicalKey) {
                        case LogicalKeyboardKey.escape:
                          controller.hideApp(const UuidV4().generate());
                          return KeyEventResult.handled;
                        case LogicalKeyboardKey.enter:
                          controller.executeToolbarAction(const UuidV4().generate());
                          return KeyEventResult.handled;
                        case LogicalKeyboardKey.arrowDown:
                          controller.handleQueryBoxArrowDown();
                          return KeyEventResult.handled;
                        case LogicalKeyboardKey.arrowUp:
                          controller.handleQueryBoxArrowUp();
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
                  }

                  var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
                  if (pressedHotkey == null) {
                    return KeyEventResult.ignored;
                  }

                  // list all actions
                  if (controller.isActionHotkey(pressedHotkey)) {
                    controller.toggleActionPanel(const UuidV4().generate());
                    return KeyEventResult.handled;
                  }

                  // check if the pressed hotkey is the action hotkey
                  var result = controller.getActiveResult();
                  var action = controller.getActionByHotkey(result, pressedHotkey);
                  if (action != null) {
                    controller.executeAction(const UuidV4().generate(), result, action);
                    return KeyEventResult.handled;
                  }

                  return KeyEventResult.ignored;
                },
                child: SizedBox(
                  height: 55.0,
                  child: Theme(
                    data: ThemeData(
                      textSelectionTheme: TextSelectionThemeData(
                        selectionColor: fromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxTextSelectionColor),
                      ),
                    ),
                    child: TextField(
                      style: TextStyle(
                        fontSize: 28.0,
                        color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxFontColor),
                      ),
                      decoration: InputDecoration(
                        contentPadding: const EdgeInsets.only(
                          left: 8,
                          right: 68,
                          top: 4,
                          bottom: 17,
                        ),
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(WoxThemeUtil.instance.currentTheme.value.queryBoxBorderRadius.toDouble()),
                          borderSide: BorderSide.none,
                        ),
                        filled: true,
                        fillColor: fromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxBackgroundColor),
                        hoverColor: Colors.transparent,
                      ),
                      cursorColor: fromCssColor(WoxThemeUtil.instance.currentTheme.value.queryBoxCursorColor),
                      focusNode: controller.queryBoxFocusNode,
                      controller: controller.queryBoxTextFieldController,
                      scrollController: controller.queryBoxScrollController,
                      enableIMEPersonalizedLearning: true,
                      inputFormatters: [
                        TextInputFormatter.withFunction((oldValue, newValue) {
                          var traceId = const UuidV4().generate();
                          Logger.instance.debug(traceId, "IME Formatter - old: ${oldValue.text}, new: ${newValue.text}, composing: ${newValue.composing}");

                          // Flutter's IME handling has inconsistencies across platforms, especially on Windows
                          // So we use input formatter to detect IME input completion instead of onChanged event
                          // Reference: https://github.com/flutter/flutter/issues/128565
                          //
                          // Issues:
                          // 1. isComposingRangeValid state is unstable on certain platforms
                          // 2. When IME input completes, the composing state changes occur in this order:
                          //    a. First, text content updates (e.g., from pinyin "wo'zhi'dao" to characters "我知道")
                          //    b. Then, the composing state is cleared (from valid to invalid)
                          //
                          // Solution:
                          // 1. Track composing range changes to more accurately detect when IME input completes
                          // 2. Use start and end positions to determine composing state instead of relying solely on isComposingRangeValid

                          // Check if both states are in IME editing mode
                          // composing.start >= 0 indicates an active IME composition region
                          bool wasComposing = oldValue.composing.start >= 0 && oldValue.composing.end >= 0;
                          bool isComposing = newValue.composing.start >= 0 && newValue.composing.end >= 0;

                          if (wasComposing && !isComposing) {
                            // Scenario 1: IME composition completed
                            // Transition from composing to non-composing state indicates user has finished word selection
                            // Example: The moment when "wo'zhi'dao" converts to "我知道"
                            Future.microtask(() {
                              Logger.instance.info(traceId, "IME: composition completed, start query: ${newValue.text}");
                              controller.onQueryBoxTextChanged(newValue.text);
                            });
                          } else if (!wasComposing && !isComposing && oldValue.text != newValue.text) {
                            // Scenario 2: Normal text input (non-IME)
                            // Text has changed but neither state is in IME composition
                            // Example: Direct input of English letters or numbers
                            Future.microtask(() {
                              Logger.instance.info(traceId, "IME: normal input, start query: ${newValue.text}");
                              controller.onQueryBoxTextChanged(newValue.text);
                            });
                          }

                          // Use Future.microtask to ensure query is triggered after text update is complete
                          // This prevents querying with incomplete state updates

                          return newValue;
                        }),
                      ],
                    ),
                  ),
                ))),
        Positioned(
          right: 10,
          height: 55,
          child: WoxDragMoveArea(
            onDragEnd: () {
              controller.focusQueryBox(selectAll: false);
            },
            child: Container(
              width: 55,
              height: 55,
              color: Colors.transparent,
              child: Padding(
                padding: const EdgeInsets.all(8.0),
                child: MouseRegion(
                  cursor: controller.queryIcon.value.action != null ? SystemMouseCursors.click : SystemMouseCursors.basic,
                  child: GestureDetector(
                    onTap: () {
                      controller.queryIcon.value.action?.call();
                      controller.focusQueryBox(selectAll: false);
                    },
                    child: WoxImageView(woxImage: controller.queryIcon.value.icon, width: 24, height: 24),
                  ),
                ),
              ),
            ),
          ),
        ),
      ]);
    });
  }
}
