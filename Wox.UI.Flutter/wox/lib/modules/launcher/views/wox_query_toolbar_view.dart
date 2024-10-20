import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxQueryToolbarView extends GetView<WoxLauncherController> {
  const WoxQueryToolbarView({super.key});

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
                    style: TextStyle(color: fromCssColor(controller.woxTheme.value.toolbarFontColor)),
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
                          style: TextStyle(color: fromCssColor(controller.woxTheme.value.toolbarFontColor)),
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
                              controller.toolbarCopyText.value = 'Copied'; // 更新状态为 "Copied"
                              Future.delayed(const Duration(seconds: 3), () {
                                controller.toolbarCopyText.value = 'Copy'; // 3秒后恢复为 "Copy"
                              });
                            },
                            child: Padding(
                              padding: const EdgeInsets.only(left: 8.0),
                              child: Obx(() => Text(
                                    controller.toolbarCopyText.value, // 使用状态变量
                                    style: TextStyle(
                                      color: fromCssColor(controller.woxTheme.value.toolbarFontColor),
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
            style: TextStyle(color: fromCssColor(controller.woxTheme.value.toolbarFontColor)),
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(width: 8),
          WoxHotkeyView(
            hotkey: hotkey!,
            backgroundColor: fromCssColor(controller.woxTheme.value.toolbarBackgroundColor),
            borderColor: fromCssColor(controller.woxTheme.value.toolbarFontColor),
            textColor: fromCssColor(controller.woxTheme.value.toolbarFontColor),
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
            color: fromCssColor(controller.woxTheme.value.toolbarBackgroundColor),
            border: Border(
              top: BorderSide(
                color: fromCssColor(controller.woxTheme.value.toolbarFontColor).withOpacity(0.1),
                width: 1,
              ),
            ),
          ),
          child: Padding(
            padding: EdgeInsets.only(left: controller.woxTheme.value.toolbarPaddingLeft.toDouble(), right: controller.woxTheme.value.toolbarPaddingRight.toDouble()),
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
