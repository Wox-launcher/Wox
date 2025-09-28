import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/modules/launcher/views/wox_query_box_view.dart';
import 'package:wox/modules/launcher/views/wox_query_result_view.dart';
import 'package:wox/modules/launcher/views/wox_query_toolbar_view.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/color_util.dart';

class WoxLauncherView extends GetView<WoxLauncherController> {
  const WoxLauncherView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return Scaffold(
        backgroundColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor),
        body: DropTarget(
          onDragDone: (DropDoneDetails details) {
            controller.handleDropFiles(details);
          },
          child: Column(
            children: [
              Expanded(
                child: Padding(
                  padding: EdgeInsets.only(
                    top: WoxThemeUtil.instance.currentTheme.value.appPaddingTop.toDouble(),
                    right: WoxThemeUtil.instance.currentTheme.value.appPaddingRight.toDouble(),
                    bottom: controller.isToolbarShowedWithoutResults ? 0 : WoxThemeUtil.instance.currentTheme.value.appPaddingBottom.toDouble(),
                    left: WoxThemeUtil.instance.currentTheme.value.appPaddingLeft.toDouble(),
                  ),
                  child: const Column(
                    children: [
                      WoxQueryBoxView(),
                      Expanded(child: WoxQueryResultView()),
                    ],
                  ),
                ),
              ),
              if (controller.isShowToolbar)
                const SizedBox(
                  height: 40,
                  child: WoxQueryToolbarView(),
                ),
            ],
          ),
        ),
      );
    });
  }
}
