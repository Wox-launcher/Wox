import 'package:desktop_drop/desktop_drop.dart';
import 'package:flutter/material.dart';
import 'package:from_css_color/from_css_color.dart';
import 'package:get/get.dart';
import 'package:wox/modules/launcher/views/wox_query_box_view.dart';
import 'package:wox/modules/launcher/views/wox_query_result_view.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';

class WoxLauncherView extends GetView<WoxLauncherController> {
  const WoxLauncherView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return Scaffold(
        backgroundColor: fromCssColor(controller.woxTheme.value.appBackgroundColor),
        body: Padding(
          padding: EdgeInsets.only(
            top: controller.woxTheme.value.appPaddingTop.toDouble(),
            right: controller.woxTheme.value.appPaddingRight.toDouble(),
            bottom: controller.woxTheme.value.appPaddingBottom.toDouble(),
            left: controller.woxTheme.value.appPaddingLeft.toDouble(),
          ),
          child: DropTarget(
            onDragDone: (DropDoneDetails details) {
              controller.handleDropFiles(details);
            },
            child: const Column(
              children: [WoxQueryBoxView(), WoxQueryResultView()],
            ),
          ),
        ),
      );
    });
  }
}
