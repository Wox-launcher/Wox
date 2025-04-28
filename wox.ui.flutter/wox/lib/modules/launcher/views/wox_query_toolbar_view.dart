import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxQueryToolbarView extends GetView<WoxLauncherController> {
  const WoxQueryToolbarView({super.key});

  bool get hasResultItems => controller.resultListViewController.items.isNotEmpty;

  Widget leftPart() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: toolbar view - left part");

    return Obx(() {
      final toolbarInfo = controller.toolbar.value;
      return SizedBox(
        width: 550,
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
                    style: TextStyle(color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
                  );
                  final textPainter = TextPainter(
                    text: textSpan,
                    maxLines: 1,
                    textDirection: TextDirection.ltr,
                  )..layout(maxWidth: constraints.maxWidth);

                  final isTextOverflow = textPainter.didExceedMaxLines;

                  return Row(
                    children: [
                      Expanded(
                        child: Text(
                          toolbarInfo.text ?? '',
                          style: TextStyle(color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
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
                              controller.toolbarCopyText.value = 'Copied';
                              Future.delayed(const Duration(seconds: 3), () {
                                controller.toolbarCopyText.value = 'Copy';
                              });
                            },
                            child: Padding(
                              padding: const EdgeInsets.only(left: 8.0),
                              child: Obx(() => Text(
                                    controller.toolbarCopyText.value,
                                    style: TextStyle(
                                      color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
                                      fontSize: 12,
                                      decoration: TextDecoration.underline,
                                    ),
                                  )),
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

  Widget rightPart() {
    if (LoggerSwitch.enablePaintLog) Logger.instance.debug(const UuidV4().generate(), "repaint: toolbar view  - right part");

    return Obx(() {
      final toolbarInfo = controller.toolbar.value;
      if (toolbarInfo.hotkey == null || toolbarInfo.hotkey!.isEmpty) {
        return const SizedBox();
      }

      var hotkey = WoxHotkey.parseHotkeyFromString(toolbarInfo.hotkey!);
      return Row(
        mainAxisAlignment: MainAxisAlignment.end,
        children: [
          Text(
            toolbarInfo.actionName ?? '',
            style: TextStyle(color: fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor)),
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(width: 8),
          WoxHotkeyView(
            hotkey: hotkey!,
            backgroundColor: hasResultItems
                ? fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarBackgroundColor)
                : fromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor).withValues(alpha: 0.1),
            borderColor: fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
            textColor: fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor),
          )
        ],
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
            color: hasResultItems ? fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarBackgroundColor) : Colors.transparent,
            border: Border(
              top: BorderSide(
                color: hasResultItems ? fromCssColor(WoxThemeUtil.instance.currentTheme.value.toolbarFontColor).withValues(alpha: 0.1) : Colors.transparent,
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
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                leftPart(),
                rightPart(),
              ],
            ),
          ),
        ),
      );
    });
  }
}
