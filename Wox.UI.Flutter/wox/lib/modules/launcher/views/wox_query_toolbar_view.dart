import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
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

    return Row(
      children: [
        Text(action.name.value, style: TextStyle(color: fromCssColor(controller.woxTheme.value.toolbarFontColor))),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return SizedBox(
        height: WoxThemeUtil.instance.getResultTipHeight(),
        child: Container(
          decoration: BoxDecoration(
            color: fromCssColor(controller.woxTheme.value.toolbarBackgroundColor),
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
