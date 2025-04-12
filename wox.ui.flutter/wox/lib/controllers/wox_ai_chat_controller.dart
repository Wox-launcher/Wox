import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_preview.dart';

class WoxAIChatController extends GetxController {
  final WoxAIChatData aiChatData;

  final TextEditingController textController = TextEditingController();
  final TextEditingController chatSelectFilterController = TextEditingController();
  final FocusNode chatSelectFilterFocusNode = FocusNode();
  final WoxLauncherController controller = Get.find<WoxLauncherController>();
  final ScrollController _chatSelectScrollController = ScrollController();

  WoxAIChatController({required this.aiChatData});
}
