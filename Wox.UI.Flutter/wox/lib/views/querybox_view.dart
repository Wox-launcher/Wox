import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:get/get.dart';
import 'package:wox/controller.dart';

class QueryBoxView extends GetView<WoxController> {
  const QueryBoxView({super.key});

  @override
  Widget build(BuildContext context) {
    return RawKeyboardListener(
      focusNode: FocusNode(),
      onKey: (event) {
        if (event.logicalKey == LogicalKeyboardKey.escape) {
          controller.hide();
        }
      },
      child: TextField(
        controller: controller.queryTextFieldController,
        onChanged: (value) {
          controller.onQueryChanged(value);
        },
      ),
    );
  }
}
