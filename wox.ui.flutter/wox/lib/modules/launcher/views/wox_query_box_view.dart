import 'dart:io';
import 'dart:math' as math;

import 'package:extended_text_field/extended_text_field.dart';
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_drag_move_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_glance.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_text_measure_util.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxQueryBoxView extends StatelessWidget {
  const WoxQueryBoxView({super.key, required this.controller});

  final WoxLauncherController controller;

  bool _isSubmitKey(LogicalKeyboardKey key) {
    return key == LogicalKeyboardKey.enter || key == LogicalKeyboardKey.numpadEnter;
  }

  // Helper method to convert LogicalKeyboardKey to number for quick select
  int? getNumberFromKey(LogicalKeyboardKey key) {
    switch (key) {
      case LogicalKeyboardKey.digit1:
        return 1;
      case LogicalKeyboardKey.digit2:
        return 2;
      case LogicalKeyboardKey.digit3:
        return 3;
      case LogicalKeyboardKey.digit4:
        return 4;
      case LogicalKeyboardKey.digit5:
        return 5;
      case LogicalKeyboardKey.digit6:
        return 6;
      case LogicalKeyboardKey.digit7:
        return 7;
      case LogicalKeyboardKey.digit8:
        return 8;
      case LogicalKeyboardKey.digit9:
        return 9;
      default:
        return null;
    }
  }

  // Check if only the quick select modifier key is pressed (no other keys)
  bool isQuickSelectModifierKeyOnly(KeyEvent event) {
    if (Platform.isMacOS) {
      // On macOS, check if only Cmd key is pressed
      return event.logicalKey == LogicalKeyboardKey.meta || event.logicalKey == LogicalKeyboardKey.metaLeft || event.logicalKey == LogicalKeyboardKey.metaRight;
    } else {
      // On Windows/Linux, check if only Alt key is pressed
      return event.logicalKey == LogicalKeyboardKey.alt || event.logicalKey == LogicalKeyboardKey.altLeft || event.logicalKey == LogicalKeyboardKey.altRight;
    }
  }

  int getQueryBoxWordBoundaryOffset(String text, int offset, {required bool forward}) {
    if (text.isEmpty) {
      return 0;
    }

    final painter = TextPainter(text: TextSpan(text: text), textDirection: TextDirection.ltr)..layout();
    try {
      final wordBoundary = painter.wordBoundaries.moveByWordBoundary;
      if (forward) {
        return wordBoundary.getTrailingTextBoundaryAt(offset.clamp(0, text.length)) ?? text.length;
      }

      return wordBoundary.getLeadingTextBoundaryAt((offset - 1).clamp(0, text.length)) ?? 0;
    } finally {
      painter.dispose();
    }
  }

  TextEditingValue buildQueryBoxReplacementValue(TextEditingValue value, int baseOffset, int extentOffset) {
    final text = value.text;
    final start = baseOffset.clamp(0, text.length);
    final end = extentOffset.clamp(0, text.length);
    final rangeStart = start < end ? start : end;
    final rangeEnd = start < end ? end : start;
    if (rangeStart == rangeEnd) {
      return value;
    }

    final newText = text.replaceRange(rangeStart, rangeEnd, '');
    return TextEditingValue(text: newText, selection: TextSelection.collapsed(offset: rangeStart), composing: TextRange.empty);
  }

  TextEditingValue buildQueryBoxWordDeletionValue(TextEditingValue value, {required bool forward}) {
    final text = value.text;
    final selection = value.selection;
    if (!selection.isValid) {
      return value;
    }

    if (!selection.isCollapsed) {
      return buildQueryBoxReplacementValue(value, selection.start, selection.end);
    }

    final offset = selection.baseOffset.clamp(0, text.length);
    final boundaryOffset = getQueryBoxWordBoundaryOffset(text, offset, forward: forward);
    return buildQueryBoxReplacementValue(value, offset, boundaryOffset);
  }

  TextEditingValue normalizeMacOptionDeleteInFormatter(TextEditingValue oldValue, TextEditingValue newValue) {
    final isDeletionFromTextInput = oldValue.text.length > newValue.text.length;
    if (!Platform.isMacOS || !isDeletionFromTextInput || !oldValue.selection.isValid || !oldValue.selection.isCollapsed) {
      return newValue;
    }

    final oldComposing = oldValue.composing;
    final newComposing = newValue.composing;
    final isComposing = (oldComposing.start >= 0 && oldComposing.end >= 0) || (newComposing.start >= 0 && newComposing.end >= 0);
    if (isComposing) {
      return newValue;
    }

    if (!HardwareKeyboard.instance.isAltPressed) {
      return newValue;
    }

    final oldOffset = oldValue.selection.baseOffset.clamp(0, oldValue.text.length);
    final newOffset = newValue.selection.baseOffset.clamp(0, newValue.text.length);
    final forward = newOffset >= oldOffset;
    // Bug fix: macOS can deliver Option+Backspace to the formatter as a plain one-character
    // deletion instead of a KeyEvent or selector intent. Rewriting the formatter value here keeps
    // the native word-deletion contract at the only layer that still observes the committed edit.
    return buildQueryBoxWordDeletionValue(oldValue, forward: forward);
  }

  double _getQueryBoxRightAccessoryWidth(BuildContext context, dynamic currentTheme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final refinementWidth = _getRefinementAccessoryWidth(context, currentTheme);
    final accessoryGap = refinementWidth > 0 ? metrics.scaledSpacing(8) : 0.0;
    if (!controller.shouldShowGlance) {
      final iconSlotWidth = controller.queryIcon.value.icon.imageData.isEmpty ? 0.0 : metrics.queryBoxRightAccessoryWidth;
      return math.max(metrics.queryBoxRightAccessoryWidth, refinementWidth + accessoryGap + iconSlotWidth);
    }

    final visibleItems = controller.glanceItems.take(1).toList();
    final baseTextColor = safeFromCssColor(currentTheme.queryBoxFontColor);
    final textColor = baseTextColor.withValues(alpha: 0.8);
    final textStyle = TextStyle(color: textColor, fontSize: metrics.queryBoxGlanceFontSize);
    final itemWidth = visibleItems.fold<double>(0, (sum, item) => sum + _getGlanceItemWidth(context, item, textStyle));
    return refinementWidth + accessoryGap + metrics.queryBoxGlanceHPadding + itemWidth + math.max(visibleItems.length - 1, 0) * metrics.queryBoxGlanceItemSpacing;
  }

  double _getRefinementAccessoryWidth(BuildContext context, dynamic currentTheme) {
    if (!controller.shouldShowQueryRefinementAffordance) {
      return 0.0;
    }

    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textStyle = TextStyle(color: safeFromCssColor(currentTheme.queryBoxFontColor), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700);
    final labelWidth = WoxTextMeasureUtil.measureTextWidth(context: context, text: controller.getQueryRefinementAffordanceLabel(), style: textStyle).ceilToDouble();
    return (metrics.scaledSpacing(28) + labelWidth + metrics.scaledSpacing(14)).clamp(metrics.scaledSpacing(72), metrics.scaledSpacing(150)).toDouble();
  }

  double _getGlanceItemWidth(BuildContext context, GlanceItem item, TextStyle textStyle) {
    // Bug fix: Glance width must be measured with the same BuildContext that
    // renders the Text widget. Windows font metrics differ enough that the old
    // controller-side TextPainter estimate could make valid text hit ellipsis.
    final textWidth = WoxTextMeasureUtil.measureTextWidth(context: context, text: item.text, style: textStyle).ceilToDouble();
    final hasIcon = controller.shouldShowGlanceIcon(item);
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final iconWidth = hasIcon ? metrics.queryBoxGlanceIconAndGapWidth : 0.0;
    // Keep the minimum width independent from icon visibility. The measured
    // content already includes icon space, and a larger icon-only minimum makes
    // short values such as Windows "AC" look padded instead of compact.
    return (textWidth + iconWidth + metrics.queryBoxGlanceHPadding + metrics.queryBoxGlanceTextSafetyWidth)
        .clamp(metrics.queryBoxGlanceMinWidth, metrics.queryBoxGlanceMaxWidth)
        .toDouble();
  }

  // Build the TextField widget
  Widget _buildTextField(dynamic currentTheme, double rightAccessoryWidth) {
    // Query box text and accessory sizes follow density while existing
    // decoration padding stays fixed, preserving the normal layout and keeping
    // multi-line height calculation aligned with controller metrics.
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textHeightFactor = metrics.queryBoxTextHeightFactor;
    final queryBoxTextStyle = TextStyle(fontSize: metrics.queryBoxFontSize, height: textHeightFactor, color: safeFromCssColor(currentTheme.queryBoxFontColor));
    final queryBoxStrutStyle = StrutStyle(fontSize: metrics.queryBoxFontSize, height: textHeightFactor, leading: 0, forceStrutHeight: true);

    return MediaQuery.withNoTextScaling(
      child: ExtendedTextField(
        key: controller.queryBoxTextFieldKey,
        style: queryBoxTextStyle,
        strutStyle: queryBoxStrutStyle,
        textAlignVertical: TextAlignVertical.center,
        decoration: InputDecoration(
          contentPadding: EdgeInsets.only(left: 8, right: rightAccessoryWidth, top: QUERY_BOX_CONTENT_PADDING_TOP, bottom: QUERY_BOX_CONTENT_PADDING_BOTTOM),
          border: OutlineInputBorder(borderRadius: BorderRadius.circular(currentTheme.queryBoxBorderRadius.toDouble()), borderSide: BorderSide.none),
          filled: true,
          fillColor: safeFromCssColor(currentTheme.queryBoxBackgroundColor),
          hoverColor: Colors.transparent,
        ),
        cursorColor: safeFromCssColor(currentTheme.queryBoxCursorColor),
        focusNode: controller.queryBoxFocusNode,
        controller: controller.queryBoxTextFieldController,
        scrollController: controller.queryBoxScrollController,
        keyboardType: TextInputType.multiline,
        textInputAction: TextInputAction.newline,
        minLines: 1,
        maxLines: QUERY_BOX_MAX_LINES,
        enableIMEPersonalizedLearning: true,
        inputFormatters: [
          TextInputFormatter.withFunction((oldValue, newValue) {
            var traceId = const UuidV4().generate();
            final formattedValue = normalizeMacOptionDeleteInFormatter(oldValue, newValue);
            Logger.instance.debug(traceId, "IME Formatter - old: ${oldValue.text}, new: ${formattedValue.text}, composing: ${formattedValue.composing}");

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
            bool isComposing = formattedValue.composing.start >= 0 && formattedValue.composing.end >= 0;

            if (wasComposing && !isComposing) {
              // Scenario 1: IME composition completed
              // Transition from composing to non-composing state indicates user has finished word selection
              // Example: The moment when "wo'zhi'dao" converts to "我知道"
              Future.microtask(() {
                Logger.instance.debug(traceId, "IME: composition completed, start query: ${formattedValue.text}");
                controller.onQueryBoxTextChanged(formattedValue.text);
              });
            } else if (!wasComposing && !isComposing && oldValue.text != formattedValue.text) {
              // Scenario 2: Normal text input (non-IME)
              // Text has changed but neither state is in IME composition
              // Example: Direct input of English letters or numbers
              Future.microtask(() {
                Logger.instance.debug(traceId, "IME: normal input, start query: ${formattedValue.text}");
                controller.onQueryBoxTextChanged(formattedValue.text);
              });
            }

            // Use Future.microtask to ensure query is triggered after text update is complete
            // This prevents querying with incomplete state updates

            return formattedValue;
          }),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) {
      Logger.instance.debug(const UuidV4().generate(), "repaint: query box view");
    }

    return Obx(() {
      final currentTheme = WoxThemeUtil.instance.currentTheme.value;
      final queryBoxHeight = controller.getQueryBoxInputHeight();
      controller.updateQueryBoxSelectedTextStyle();

      return Stack(
        children: [
          Positioned(
            child: WoxPlatformFocus(
              onKeyEvent: (FocusNode node, KeyEvent event) {
                var traceId = const UuidV4().generate();

                // Handle number keys in quick select mode first (higher priority)
                if (controller.isQuickSelectMode.value && event is KeyDownEvent) {
                  var numberKey = getNumberFromKey(event.logicalKey);
                  if (numberKey != null) {
                    if (controller.handleQuickSelectNumberKey(traceId, numberKey)) {
                      return KeyEventResult.handled;
                    }
                  }
                }

                // Handle quick select modifier key press/release
                if ((event is KeyDownEvent || event is KeyRepeatEvent) && isQuickSelectModifierKeyOnly(event)) {
                  controller.startQuickSelectTimer(traceId);
                } else {
                  controller.stopQuickSelectTimer(traceId);
                }

                if (Platform.isLinux && event is KeyUpEvent && _isSubmitKey(event.logicalKey)) {
                  controller.resetLinuxQueryBoxSubmitKeyHandling();
                  return KeyEventResult.ignored;
                }

                var isAnyModifierPressed = WoxHotkey.isAnyModifierPressed();
                if (!isAnyModifierPressed) {
                  if (event is KeyDownEvent) {
                    switch (event.logicalKey) {
                      case LogicalKeyboardKey.escape:
                        controller.hideApp(const UuidV4().generate());
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.enter:
                      case LogicalKeyboardKey.numpadEnter:
                        var composing = controller.queryBoxTextFieldController.value.composing;
                        var isComposing = composing.start >= 0 && composing.end >= 0;
                        if (isComposing) {
                          return KeyEventResult.ignored;
                        }

                        if (Platform.isLinux) {
                          // Record the normal KeyDown path so a later Linux KeyRepeatEvent from the same
                          // physical press is swallowed instead of inserting newline or re-executing.
                          controller.markLinuxQueryBoxSubmitKeyHandled();
                        }

                        controller.executeDefaultAction(const UuidV4().generate());
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowDown:
                        controller.handleQueryBoxArrowDown();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowUp:
                        controller.handleQueryBoxArrowUp();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowLeft:
                        if (controller.isInGridMode()) {
                          controller.handleQueryBoxArrowLeft();
                          return KeyEventResult.handled;
                        }
                        break;
                      case LogicalKeyboardKey.arrowRight:
                        if (controller.isInGridMode()) {
                          controller.handleQueryBoxArrowRight();
                          return KeyEventResult.handled;
                        }
                        break;
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
                      case LogicalKeyboardKey.enter:
                      case LogicalKeyboardKey.numpadEnter:
                        if (!Platform.isLinux) {
                          break;
                        }

                        // #4410 Linux can generate KeyRepeatEvents without a preceding KeyDownEvent, so also check the composing state to avoid submitting incomplete IME input.
                        // This is a workaround for the underlying issue of unreliable KeyDown/KeyUp events on Linux, which may require a more robust solution in the future.
                        var composing = controller.queryBoxTextFieldController.value.composing;
                        var isComposing = composing.start >= 0 && composing.end >= 0;
                        if (isComposing) {
                          return KeyEventResult.ignored;
                        }

                        if (controller.shouldExecuteLinuxQueryBoxSubmitFromRepeat()) {
                          controller.executeDefaultAction(const UuidV4().generate());
                        }
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowDown:
                        controller.handleQueryBoxArrowDown();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowUp:
                        controller.handleQueryBoxArrowUp();
                        return KeyEventResult.handled;
                      case LogicalKeyboardKey.arrowLeft:
                        if (controller.isInGridMode()) {
                          controller.handleQueryBoxArrowLeft();
                          return KeyEventResult.handled;
                        }
                        break;
                      case LogicalKeyboardKey.arrowRight:
                        if (controller.isInGridMode()) {
                          controller.handleQueryBoxArrowRight();
                          return KeyEventResult.handled;
                        }
                        break;
                    }
                  }
                }

                // Emacs-style navigation: Ctrl+N / Ctrl+P move through results
                // just like Arrow Down / Arrow Up, so launcher users can keep
                // their hands on the home row.
                if ((event is KeyDownEvent || event is KeyRepeatEvent) &&
                    HardwareKeyboard.instance.isControlPressed &&
                    !HardwareKeyboard.instance.isShiftPressed &&
                    !HardwareKeyboard.instance.isAltPressed &&
                    !HardwareKeyboard.instance.isMetaPressed) {
                  switch (event.logicalKey) {
                    case LogicalKeyboardKey.keyN:
                      controller.handleQueryBoxArrowDown();
                      return KeyEventResult.handled;
                    case LogicalKeyboardKey.keyP:
                      controller.handleQueryBoxArrowUp();
                      return KeyEventResult.handled;
                  }
                }

                var pressedHotkey = WoxHotkey.parseNormalHotkeyFromEvent(event);
                if (pressedHotkey == null) {
                  return KeyEventResult.ignored;
                }

                if (controller.executeLocalActionByHotkey(traceId, pressedHotkey)) {
                  return KeyEventResult.handled;
                }

                if (controller.executeQueryRefinementToggleHotkey(traceId, pressedHotkey)) {
                  return KeyEventResult.handled;
                }

                // list all actions
                if (controller.isActionHotkey(pressedHotkey)) {
                  controller.toggleActionPanel(const UuidV4().generate());
                  return KeyEventResult.handled;
                }

                // Feature addition: query-level refinement hotkeys sit after
                // launcher-local shortcuts and before result actions, so filters
                // stay keyboard-accessible without stealing reserved commands.
                if (controller.executeQueryRefinementHotkey(traceId, pressedHotkey)) {
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
                height: queryBoxHeight,
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final rightAccessoryWidth = _getQueryBoxRightAccessoryWidth(context, currentTheme);
                    // The query box height now follows visual wrapping, so update the controller with the
                    // same text width used by the input decoration. This keeps pasted multi-line text intact
                    // while giving long single-line queries enough vertical room for caret navigation.
                    SchedulerBinding.instance.addPostFrameCallback((_) {
                      controller.updateQueryBoxTextWrapWidth(const UuidV4().generate(), (constraints.maxWidth - 8 - rightAccessoryWidth).clamp(0, double.infinity));
                    });

                    return Theme(
                      data: ThemeData(textSelectionTheme: TextSelectionThemeData(selectionColor: safeFromCssColor(currentTheme.queryBoxTextSelectionBackgroundColor))),
                      child: _buildTextField(currentTheme, rightAccessoryWidth),
                    );
                  },
                ),
              ),
            ),
          ),
          Positioned(
            right: 6,
            height: queryBoxHeight,
            child: WoxDragMoveArea(
              onDragStart: controller.windowDriver.startDragging,
              onDragEnd: () {
                controller.focusQueryBox();
              },
              child: SizedBox(width: _getQueryBoxRightAccessoryWidth(context, currentTheme), height: queryBoxHeight, child: Center(child: _buildRightAccessory(currentTheme))),
            ),
          ),
        ],
      );
    });
  }

  Widget _buildRightAccessory(dynamic currentTheme) {
    if (controller.isLoading.value) {
      final metrics = WoxInterfaceSizeUtil.instance.current;
      return Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          SizedBox(
            // Bug fix: loading previously centered itself in the full accessory
            // width. Query refinements can make that width much wider than the
            // plugin icon slot, so anchor the spinner inside the same right slot
            // used by the plugin icon to avoid a leftward jump while waiting.
            width: metrics.queryBoxRightAccessoryWidth,
            child: Center(child: WoxLoadingIndicator(size: metrics.scaledSpacing(20), color: safeFromCssColor(currentTheme.queryBoxCursorColor))),
          ),
        ],
      );
    }
    final accessoryChildren = <Widget>[];
    if (controller.shouldShowQueryRefinementAffordance) {
      accessoryChildren.add(_buildRefinementAccessory(currentTheme));
    }

    if (controller.shouldShowGlance) {
      final items = controller.glanceItems.take(1).toList();
      for (final item in items) {
        if (accessoryChildren.isNotEmpty) {
          accessoryChildren.add(SizedBox(width: WoxInterfaceSizeUtil.instance.current.scaledSpacing(12)));
        }
        accessoryChildren.add(_buildGlanceItem(currentTheme, item));
      }
      return Row(mainAxisAlignment: MainAxisAlignment.end, children: accessoryChildren);
    }

    if (controller.queryIcon.value.icon.imageData.isNotEmpty) {
      if (accessoryChildren.isNotEmpty) {
        accessoryChildren.add(SizedBox(width: WoxInterfaceSizeUtil.instance.current.scaledSpacing(12)));
      }
      final metrics = WoxInterfaceSizeUtil.instance.current;
      accessoryChildren.add(
        Padding(
          // Bug fix: preserve the original right-side breathing room without
          // centering the icon in a wide slot. Centering restored the margin but
          // created a large left gap between Filters and the plugin icon.
          padding: EdgeInsets.only(right: (metrics.queryBoxRightAccessoryWidth - metrics.queryBoxIconSize).clamp(0, double.infinity) / 2),
          child: MouseRegion(
            cursor: controller.queryIcon.value.action != null ? SystemMouseCursors.click : SystemMouseCursors.basic,
            child: GestureDetector(
              onTap: () {
                controller.queryIcon.value.action?.call();
                controller.focusQueryBox();
              },
              child: WoxImageView(woxImage: controller.queryIcon.value.icon, width: metrics.queryBoxIconSize, height: metrics.queryBoxIconSize),
            ),
          ),
        ),
      );
    }

    return Row(mainAxisAlignment: MainAxisAlignment.end, children: accessoryChildren);
  }

  Widget _buildRefinementAccessory(dynamic currentTheme) {
    final metrics = WoxInterfaceSizeUtil.instance.current;
    final textColor = safeFromCssColor(currentTheme.queryBoxFontColor);
    final activeColor = safeFromCssColor(currentTheme.queryBoxCursorColor);
    final isExpanded = controller.isQueryRefinementBarExpanded.value;
    final isActive = controller.hasActiveQueryRefinements;
    final label = controller.getQueryRefinementAffordanceLabel();
    final tint = isExpanded || isActive ? activeColor : textColor;
    // Visual refinement: keep the collapsed affordance tooltip because the
    // visible label can become the active value ("Image", "Text", etc.).
    // The tooltip preserves the stable Filters command name and shortcut
    // without adding extra text to the query box chrome.
    final tooltip = "${controller.tr("ui_query_refinement_filters")} ${controller.queryRefinementToggleHotkeyLabel}";

    return WoxTooltip(
      message: tooltip,
      child: MouseRegion(
        cursor: SystemMouseCursors.click,
        child: GestureDetector(
          onTap: () {
            controller.toggleQueryRefinementBar(const UuidV4().generate());
            controller.focusQueryBox();
          },
          child: Container(
            height: metrics.scaledSpacing(26),
            constraints: BoxConstraints(maxWidth: metrics.scaledSpacing(150)),
            padding: EdgeInsets.only(left: metrics.scaledSpacing(8), right: metrics.scaledSpacing(9)),
            decoration: BoxDecoration(
              // Feature addition: a collapsed filter affordance keeps the
              // launcher default path clean while still making plugin-provided
              // refinements discoverable next to the query itself.
              color: tint.withValues(alpha: isExpanded || isActive ? 0.15 : 0.075),
              borderRadius: BorderRadius.circular(7),
              border: Border.all(color: tint.withValues(alpha: isExpanded || isActive ? 0.32 : 0.13)),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.filter_list_rounded, size: metrics.scaledSpacing(15), color: tint.withValues(alpha: 0.92)),
                SizedBox(width: metrics.scaledSpacing(5)),
                Flexible(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(color: textColor.withValues(alpha: isExpanded || isActive ? 0.94 : 0.72), fontSize: metrics.smallLabelFontSize, fontWeight: FontWeight.w700),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildGlanceItem(dynamic currentTheme, GlanceItem item) {
    final baseTextColor = safeFromCssColor(currentTheme.queryBoxFontColor);
    // Glance now has no status field in v1; keeping one quiet opacity preserves
    // the auxiliary feel without exposing unused state semantics in the API.
    const textAlpha = 0.8;
    final textColor = baseTextColor.withValues(alpha: textAlpha);
    var isHovered = false;

    // Glance is auxiliary status, so the default state is fully transparent and
    // visually merges with the query box; hover is only a light affordance.
    // Glance items are the launcher chrome shown in the screenshot. Using
    // WoxTooltip here keeps plugin/system glance hints visually aligned with
    // other launcher overlays instead of falling back to Material Tooltip.
    return WoxTooltip(
      message: item.tooltip.isNotEmpty ? item.tooltip : item.text,
      child: StatefulBuilder(
        builder: (context, setHovered) {
          final metrics = WoxInterfaceSizeUtil.instance.current;
          final textStyle = TextStyle(color: textColor, fontSize: metrics.scaledSpacing(15));
          final itemWidth = _getGlanceItemWidth(context, item, textStyle);
          final iconVisible = controller.shouldShowGlanceIcon(item);

          return MouseRegion(
            cursor: item.action == null ? SystemMouseCursors.basic : SystemMouseCursors.click,
            onEnter: (_) => setHovered(() => isHovered = true),
            onExit: (_) => setHovered(() => isHovered = false),
            child: GestureDetector(
              onTap: item.action == null ? null : () => controller.executeGlanceDefaultAction(const UuidV4().generate(), item),
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 120),
                width: itemWidth,
                height: metrics.scaledSpacing(30),
                padding: EdgeInsets.symmetric(horizontal: metrics.scaledSpacing(8)),
                decoration: BoxDecoration(
                  color: isHovered ? baseTextColor.withValues(alpha: 0.10) : Colors.transparent,
                  borderRadius: BorderRadius.circular(5),
                  border: Border.all(color: isHovered ? baseTextColor.withValues(alpha: 0.08) : Colors.transparent),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (iconVisible) ...[
                      Opacity(
                        opacity: textAlpha * 0.9,
                        child: WoxImageView(woxImage: item.icon, width: metrics.scaledSpacing(16), height: metrics.scaledSpacing(16), svgColor: textColor),
                      ),
                      SizedBox(width: metrics.scaledSpacing(5)),
                    ],
                    Flexible(child: Text(item.text, overflow: TextOverflow.ellipsis, maxLines: 1, style: textStyle)),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}
