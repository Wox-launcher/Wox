import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/color_util.dart';

class WoxQueryToolbarView extends GetView<WoxLauncherController> {
  const WoxQueryToolbarView({super.key});

  bool get hasResultItems => controller.resultListViewController.items.isNotEmpty;

  bool get hasLeftMessage {
    final toolbarInfo = controller.toolbar.value;
    return toolbarInfo.text != null && toolbarInfo.text!.isNotEmpty;
  }

  Widget leftPart(double maxLeftWidth) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: toolbar view - left part");

    return Obx(() {
      final toolbarInfo = controller.toolbar.value;

      // If no message, return empty widget
      if (toolbarInfo.text == null || toolbarInfo.text!.isEmpty) {
        return const SizedBox.shrink();
      }

      // Cap the left section width while allowing it to shrink to content size.
      return ConstrainedBox(
        constraints: BoxConstraints(maxWidth: maxLeftWidth),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (toolbarInfo.icon != null)
              Padding(
                padding: const EdgeInsets.only(right: 8),
                child: WoxImageView(woxImage: toolbarInfo.icon!, width: 24, height: 24),
              ),
            // Text area flexes inside the capped max width and will ellipsize when needed
            Flexible(
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final textSpan = TextSpan(
                    text: toolbarInfo.text ?? '',
                    style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
                  );
                  final textPainter = TextPainter(
                    text: textSpan,
                    maxLines: 1,
                    textDirection: TextDirection.ltr,
                  )..layout(maxWidth: constraints.maxWidth);

                  final isTextOverflow = textPainter.didExceedMaxLines;

                  return Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Flexible(
                        child: Text(
                          toolbarInfo.text ?? '',
                          style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
                          overflow: TextOverflow.ellipsis,
                          maxLines: 1,
                        ),
                      ),
                      if (isTextOverflow)
                        MouseRegion(
                          cursor: SystemMouseCursors.click,
                          child: GestureDetector(
                            onTap: () {
                              Clipboard.setData(ClipboardData(text: toolbarInfo.text ?? ''));
                              controller.toolbarCopyText.value = 'toolbar_copied';
                              Future.delayed(const Duration(seconds: 3), () {
                                controller.toolbarCopyText.value = 'toolbar_copy';
                              });
                            },
                            child: Padding(
                              padding: const EdgeInsets.only(left: 8.0),
                              child: Obx(() {
                                final settingController = Get.find<WoxSettingController>();
                                return Text(
                                  settingController.tr(controller.toolbarCopyText.value),
                                  style: TextStyle(
                                    color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
                                    fontSize: 12,
                                    decoration: TextDecoration.underline,
                                  ),
                                );
                              }),
                            ),
                          ),
                        ),
                      if (isTextOverflow) ...[
                        const SizedBox(width: 8),
                        Theme(
                          data: Theme.of(context).copyWith(
                            popupMenuTheme: PopupMenuThemeData(
                              color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarBackgroundColor),
                              textStyle: TextStyle(
                                color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
                                fontSize: 12,
                              ),
                            ),
                          ),
                          child: PopupMenuButton<String>(
                            padding: EdgeInsets.zero,
                            tooltip: '',
                            onSelected: (value) async {
                              final text = toolbarInfo.text ?? '';
                              await WoxApi.instance.toolbarSnooze(text, value);
                              // Hide current toolbar message immediately
                              controller.toolbar.value = controller.toolbar.value.emptyLeftSide();
                            },
                            itemBuilder: (context) {
                              final settingController = Get.find<WoxSettingController>();
                              return [
                                PopupMenuItem(value: '3d', child: Text(settingController.tr('toolbar_snooze_3d'))),
                                PopupMenuItem(value: '7d', child: Text(settingController.tr('toolbar_snooze_7d'))),
                                PopupMenuItem(value: '1m', child: Text(settingController.tr('toolbar_snooze_1m'))),
                                PopupMenuItem(value: 'forever', child: Text(settingController.tr('toolbar_snooze_forever'))),
                              ];
                            },
                            child: Builder(
                              builder: (context) {
                                final settingController = Get.find<WoxSettingController>();
                                return Text(
                                  settingController.tr('toolbar_snooze'),
                                  style: TextStyle(
                                    color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
                                    fontSize: 12,
                                    decoration: TextDecoration.underline,
                                  ),
                                );
                              },
                            ),
                          ),
                        ),
                      ],
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
  double _calculateActionWidth(String actionName, HotkeyX hotkey) {
    // Use TextPainter to precisely measure text width (works for all languages)
    final textSpan = TextSpan(
      text: actionName,
      style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
    );
    final textPainter = TextPainter(
      text: textSpan,
      maxLines: 1,
      textDirection: TextDirection.ltr,
    )..layout();

    final nameWidth = textPainter.width;

    // Calculate hotkey width
    double hotkeyWidth = 0;
    if (hotkey.isNormalHotkey) {
      // Each key is 28px wide, spacing between keys is 4px
      final keyCount = (hotkey.normalHotkey!.modifiers?.length ?? 0) + 1;
      hotkeyWidth = keyCount * 28.0 + (keyCount - 1) * 4.0;
    } else if (hotkey.isDoubleHotkey) {
      // Two keys, each 28px wide, 4px spacing
      hotkeyWidth = 28.0 * 2 + 4.0;
    } else if (hotkey.isSingleHotkey) {
      // Single key, 28px wide
      hotkeyWidth = 28.0;
    }

    // Total: name + 8px spacing + hotkey + 16px spacing between actions
    return nameWidth + 8.0 + hotkeyWidth + 16.0;
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
              final calculatedWidth = _calculateActionWidth(actionInfo.name, hotkey);
              actionData.add({
                'info': actionInfo,
                'hotkey': hotkey,
                'width': calculatedWidth,
              });
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
            final actionInfo = actionData['info'];
            final hotkey = actionData['hotkey'] as HotkeyX;

            if (actionWidgets.isNotEmpty) {
              actionWidgets.add(const SizedBox(width: 16));
            }

            actionWidgets.add(
              Text(
                actionInfo.name,
                style: TextStyle(color: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
                overflow: TextOverflow.ellipsis,
                maxLines: 1,
              ),
            );
            actionWidgets.add(const SizedBox(width: 8));
            actionWidgets.add(
              WoxHotkeyView(
                hotkey: hotkey,
                backgroundColor: hasResultItems
                    ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarBackgroundColor)
                    : safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor).withValues(alpha: 0.1),
                borderColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
                textColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
              ),
            );
          }

          return Align(
            alignment: Alignment.centerRight,
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: actionWidgets,
            ),
          );
        },
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: query toolbar view - container");

    return Obx(() {
      return SizedBox(
        height: WoxThemeUtil.instance.getToolbarHeight(),
        child: Container(
          decoration: BoxDecoration(
            color: hasResultItems ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarBackgroundColor) : Colors.transparent,
            border: Border(
              top: BorderSide(
                color: hasResultItems ? safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor).withValues(alpha: 0.1) : Colors.transparent,
                width: 1,
              ),
            ),
          ),
          child: Padding(
            padding: EdgeInsets.only(
              left: WoxThemeUtil.instance.currentTheme.value.toolbarPaddingLeft.toDouble(),
              right: WoxThemeUtil.instance.currentTheme.value.toolbarPaddingRight.toDouble(),
            ),
            child: LayoutBuilder(
              builder: (context, constraints) {
                // Limit left message to a max fraction so right side always has room
                final double leftMaxWidth = constraints.maxWidth - 200 /* width of more actions hotkey */;
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    // Left part takes only the space it needs up to leftMaxWidth
                    leftPart(leftMaxWidth),
                    if (hasLeftMessage) const SizedBox(width: 16),
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
