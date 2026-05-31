import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_toolbar.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/wox_text_measure_util.dart';

class WoxQueryToolbarView extends GetView<WoxLauncherController> {
  const WoxQueryToolbarView({super.key});

  bool get hasResultItems => controller.resultListViewController.items.isNotEmpty;

  bool get hasLeftMessage {
    final text = controller.resolvedToolbarText;
    return text != null && text.isNotEmpty;
  }

  Widget leftPart(double maxLeftWidth) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: toolbar view - left part");

    return Obx(() {
      final text = controller.resolvedToolbarText;
      final hasToolbarProgress = controller.hasVisibleToolbarMsg && (controller.resolvedToolbarProgress != null || controller.resolvedToolbarIndeterminate);
      final metrics = WoxInterfaceSizeUtil.instance.current;

      // If no message, return empty widget
      if (text == null || text.isEmpty) {
        return const SizedBox.shrink();
      }

      // Cap the left section width while allowing it to shrink to content size.
      return ConstrainedBox(
        constraints: BoxConstraints(maxWidth: maxLeftWidth),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (controller.resolvedToolbarIcon != null)
              Padding(
                padding: EdgeInsets.only(right: metrics.toolbarIconSpacing),
                child: WoxImageView(woxImage: controller.resolvedToolbarIcon!, width: metrics.toolbarIconSize, height: metrics.toolbarIconSize),
              ),
            // Text area flexes inside the capped max width and will ellipsize when needed
            Flexible(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final textStyle = TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor), fontSize: metrics.toolbarFontSize);
                  final isTextOverflow = WoxTextMeasureUtil.isTextOverflow(context: context, text: text, style: textStyle, maxWidth: constraints.maxWidth);

                  return Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Flexible(child: Text(text, style: textStyle, overflow: TextOverflow.ellipsis, maxLines: 1)),
                      if (hasToolbarProgress)
                        Padding(
                          padding: EdgeInsets.only(left: metrics.toolbarIconSpacing),
                          child: SizedBox(
                            width: metrics.toolbarProgressSize,
                            height: metrics.toolbarProgressSize,
                            child: CircularProgressIndicator(
                              strokeWidth: metrics.toolbarProgressStrokeWidth,
                              value: controller.resolvedToolbarIndeterminate ? null : (controller.resolvedToolbarProgress ?? 0).clamp(0, 100) / 100,
                              valueColor: AlwaysStoppedAnimation<Color>(safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
                              backgroundColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor).withValues(alpha: 0.2),
                            ),
                          ),
                        ),
                      if (isTextOverflow && !controller.hasVisibleToolbarMsg)
                        MouseRegion(
                          cursor: SystemMouseCursors.click,
                          child: GestureDetector(
                            onTap: () {
                              Clipboard.setData(ClipboardData(text: text));
                              controller.toolbarCopyText.value = 'toolbar_copied';
                              Future.delayed(const Duration(seconds: 3), () {
                                controller.toolbarCopyText.value = 'toolbar_copy';
                              });
                            },
                            child: Padding(
                              padding: EdgeInsets.only(left: metrics.toolbarIconSpacing),
                              child: Obx(() {
                                final settingController = Get.find<WoxSettingController>();
                                return Text(
                                  settingController.tr(controller.toolbarCopyText.value),
                                  style: TextStyle(
                                    color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
                                    fontSize: metrics.toolbarFontSize,
                                    decoration: TextDecoration.underline,
                                  ),
                                );
                              }),
                            ),
                          ),
                        ),
                    ],
                  );
                },
              ),
            ),
          ],
        ),
      );
    });
  }

  /// Calculate the precise width of a single action (name + hotkey + spacing)
  double _calculateActionWidth(BuildContext context, String actionName, HotkeyX hotkey) {
    final nameWidth = WoxTextMeasureUtil.measureTextWidth(
      context: context,
      text: actionName,
      style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor), fontSize: WoxInterfaceSizeUtil.instance.current.toolbarFontSize),
    );

    // Feature fix: modifier labels now vary by platform, so the toolbar's
    // fit calculation must use the same dynamic chip width as WoxHotkeyView
    // instead of the old glyph-only constant width.
    final hotkeyWidth = WoxHotkeyView.measureHotkeyWidth(context, hotkey);
    final metrics = WoxInterfaceSizeUtil.instance.current;

    return nameWidth + metrics.toolbarActionNameHotkeySpacing + hotkeyWidth + metrics.toolbarActionSpacing;
  }

  Widget rightPart() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: toolbar view  - right part");

    return Obx(() {
      final toolbarInfo = controller.toolbar.value;

      // Show all actions with hotkeys
      if (toolbarInfo.actions == null || toolbarInfo.actions!.isEmpty) {
        return const SizedBox();
      }

      return LayoutBuilder(
        builder: (context, constraints) {
          final availableWidth = constraints.maxWidth;

          // Parse all actions and calculate their widths
          final actionData = <Map<String, dynamic>>[];
          for (var actionInfo in toolbarInfo.actions!) {
            var hotkey = WoxHotkey.parseHotkeyFromString(actionInfo.hotkey);
            if (hotkey != null) {
              final calculatedWidth = _calculateActionWidth(context, actionInfo.name, hotkey);
              actionData.add({'info': actionInfo, 'hotkey': hotkey, 'width': calculatedWidth});
            }
          }

          if (actionData.isEmpty) {
            return const SizedBox();
          }

          // Determine how many actions to show from right to left
          // Start from the rightmost action and work backwards
          final actionsToShow = <Map<String, dynamic>>[];
          double totalWidth = 0;

          // Iterate from right to left (reverse order)
          for (int i = actionData.length - 1; i >= 0; i--) {
            final action = actionData[i];
            final actionWidth = action['width'] as double;

            // Check if adding this action would exceed available width
            if (totalWidth + actionWidth <= availableWidth) {
              actionsToShow.insert(0, action); // Insert at beginning to maintain order
              totalWidth += actionWidth;
            } else {
              // No more space, stop adding actions
              break;
            }
          }

          // When there's a left message, ensure at least one action is shown (the rightmost one)
          if (hasLeftMessage && actionsToShow.isEmpty && actionData.isNotEmpty) {
            actionsToShow.add(actionData.last);
          }

          // Build widgets for the actions to show
          List<Widget> actionWidgets = [];
          for (var actionData in actionsToShow) {
            final actionInfo = actionData['info'] as ToolbarActionInfo;
            final hotkey = actionData['hotkey'] as HotkeyX;

            if (actionWidgets.isNotEmpty) {
              actionWidgets.add(SizedBox(width: WoxInterfaceSizeUtil.instance.current.toolbarActionSpacing));
            }

            actionWidgets.add(_buildClickableToolbarAction(actionInfo, hotkey));
          }

          return Align(alignment: Alignment.centerRight, child: Row(mainAxisSize: MainAxisSize.min, children: actionWidgets));
        },
      );
    });
  }

  Widget _buildClickableToolbarAction(ToolbarActionInfo actionInfo, HotkeyX hotkey) {
    return MouseRegion(
      cursor: SystemMouseCursors.click,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: () {
          controller.handleToolbarActionTap(const UuidV4().generate(), actionInfo);
        },
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              actionInfo.name,
              style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor), fontSize: WoxInterfaceSizeUtil.instance.current.toolbarFontSize),
              overflow: TextOverflow.ellipsis,
              maxLines: 1,
            ),
            SizedBox(width: WoxInterfaceSizeUtil.instance.current.toolbarActionNameHotkeySpacing),
            WoxHotkeyView(
              hotkey: hotkey,
              backgroundColor:
                  hasResultItems
                      ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarBackgroundColor)
                      : safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor).withValues(alpha: 0.1),
              borderColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
              textColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
            ),
          ],
        ),
      ),
    );
  }

  Widget bugAwareIndicator() {
    return Obx(() {
      if (!controller.hasBugAwareToolbarIndicator) {
        return const SizedBox.shrink();
      }

      final metrics = WoxInterfaceSizeUtil.instance.current;
      final settingController = Get.find<WoxSettingController>();
      const iconColor = Color(0xFFE5484D);

      // Feature: bug aware mode needs a launcher-owned entry point that is
      // always visible while monitoring is enabled. It cannot reuse
      // ShowToolbarMsg because plugin messages are transient and may be
      // cleared by unrelated plugin actions.
      return Tooltip(
        message: settingController.tr("ui_bug_aware_enabled_tooltip"),
        child: MouseRegion(
          cursor: SystemMouseCursors.click,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () {
              controller.activateBugReportQuery(const UuidV4().generate());
            },
            child: SizedBox(
              width: metrics.toolbarIconSize,
              height: metrics.toolbarIconSize,
              child: Icon(Icons.bug_report_outlined, size: metrics.toolbarIconSize, color: iconColor),
            ),
          ),
        ),
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: query toolbar view - container");

    return Obx(() {
      final metrics = WoxInterfaceSizeUtil.instance.metrics.value;
      final baseToolbarColor = hasResultItems ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarBackgroundColor) : Colors.transparent;
      // Feature update: bug aware mode is now indicated only by the fixed red
      // icon. Keeping the toolbar background unchanged avoids making normal
      // result actions look like warnings while diagnostics are enabled.
      final toolbarColor = baseToolbarColor;
      final toolbarBorderColor = hasResultItems ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor).withValues(alpha: 0.1) : Colors.transparent;
      return SizedBox(
        height: WoxThemeUtil.instance.getToolbarHeight(),
        child: Container(
          decoration: BoxDecoration(color: toolbarColor, border: Border(top: BorderSide(color: toolbarBorderColor, width: 1))),
          child: Padding(
            padding: EdgeInsets.only(
              left: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.toolbarPaddingLeft.toDouble()),
              right: WoxInterfaceSizeUtil.instance.current.scaledSpacing(WoxThemeUtil.instance.currentTheme.value.toolbarPaddingRight.toDouble()),
            ),
            child: LayoutBuilder(
              builder: (context, constraints) {
                // Limit left message to a max fraction so right side always has room
                // Toolbar text, icons, and hotkey chips use density metrics now,
                // so reserve the right-action area with the same scale instead
                // of the old normal-only 200px estimate.
                final bugAwareIndicatorWidth = controller.hasBugAwareToolbarIndicator ? metrics.toolbarIconSize + metrics.toolbarIconSpacing : 0.0;
                final double leftMaxWidth = (constraints.maxWidth - metrics.toolbarRightReservedWidth - bugAwareIndicatorWidth).clamp(0.0, constraints.maxWidth).toDouble();
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    bugAwareIndicator(),
                    if (controller.hasBugAwareToolbarIndicator) SizedBox(width: metrics.toolbarIconSpacing),
                    // Left part takes only the space it needs up to leftMaxWidth
                    leftPart(leftMaxWidth),
                    if (hasLeftMessage) SizedBox(width: metrics.toolbarActionSpacing),
                    // Right part fills remaining space and aligns content to the right
                    Expanded(child: rightPart()),
                  ],
                );
              },
            ),
          ),
        ),
      );
    });
  }
}
