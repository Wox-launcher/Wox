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
    return Scaffold(
      backgroundColor: fromCssColor(controller.woxTheme.appBackgroundColor),
      body: Padding(
          padding: EdgeInsets.only(
            top: controller.woxTheme.appPaddingTop.toDouble(),
            right: controller.woxTheme.appPaddingRight.toDouble(),
            bottom: controller.woxTheme.appPaddingBottom.toDouble(),
            left: controller.woxTheme.appPaddingLeft.toDouble(),
          ),
          child: const Column(
            children: [WoxQueryBoxView(), WoxQueryResultView()],
          )),
    );
  }
}
