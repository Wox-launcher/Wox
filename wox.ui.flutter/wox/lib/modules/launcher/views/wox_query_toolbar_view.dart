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

  Widget leftPart() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: toolbar view - left part");

    return Obx(() {
      final toolbarInfo = controller.toolbar.value;
      return Flexible(
        child: Row(
          children: [
            if (toolbarInfo.icon != null)
              Padding(
                padding: const EdgeInsets.only(right: 8),
                child: WoxImageView(woxImage: toolbarInfo.icon!, width: 24, height: 24),
              ),
            Expanded(
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
                    children: [
                      const SizedBox(width: 0),
                      Expanded(
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
                              // i18n: store key, render via tr
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

  Widget rightPart() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: toolbar view  - right part");

    return Obx(() {
      final toolbarInfo = controller.toolbar.value;

      // Show all actions with hotkeys
      if (toolbarInfo.actions == null || toolbarInfo.actions!.isEmpty) {
        return const SizedBox();
      }

      List<Widget> actionWidgets = [];

      for (var actionInfo in toolbarInfo.actions!) {
        var hotkey = WoxHotkey.parseHotkeyFromString(actionInfo.hotkey);
        if (hotkey != null) {
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
      }

      return Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: actionWidgets,
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
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                leftPart(),
                const SizedBox(width: 16),
                Expanded(child: rightPart()),
              ],
            ),
          ),
        ),
      );
    });
  }
}
