import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_hotkey_view.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxQueryToolbarView extends GetView<WoxLauncherController> {
  const WoxQueryToolbarView({super.key});

  Widget leftTip() {
    return const SizedBox();
  }

  Widget rightTip() {
    var action = controller.getActiveAction();
    if (action == null) {
      return const SizedBox();
    }

    var hotkey = WoxHotkey.parseHotkeyFromString(action.hotkey) ?? WoxHotkey.parseHotkeyFromString("enter");
    return Row(
      children: [
        Text(action.name.value, style: TextStyle(color: fromCssColor(controller.woxTheme.value.toolbarFontColor))),
        const SizedBox(width: 8),
        WoxHotkeyView(
          hotkey: hotkey!,
          backgroundColor: fromCssColor(controller.woxTheme.value.toolbarBackgroundColor),
          borderColor: fromCssColor(controller.woxTheme.value.toolbarFontColor),
          textColor: fromCssColor(controller.woxTheme.value.toolbarFontColor),
        )
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return SizedBox(
        height: WoxThemeUtil.instance.getToolbarHeight(),
        child: Container(
          decoration: BoxDecoration(
            color: fromCssColor(controller.woxTheme.value.toolbarBackgroundColor),
            //add some shadow to the top of the toolbar
          ),
          child: Padding(
            padding: EdgeInsets.only(left: controller.woxTheme.value.toolbarPaddingLeft.toDouble(), right: controller.woxTheme.value.toolbarPaddingRight.toDouble()),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                leftTip(),
                rightTip(),
              ],
            ),
          ),
        ),
      );
    });
  }
}
