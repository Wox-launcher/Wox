import 'package:flutter/material.dart';
import 'package:get/get.dart';

import '../wox_launcher_controller.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return Container(constraints: BoxConstraints(maxHeight: controller.getMaxHeight()));
    });
  }
}
