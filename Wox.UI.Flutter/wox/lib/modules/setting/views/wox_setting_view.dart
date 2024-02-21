import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

class WoxSettingView extends GetView<WoxSettingController> {
  const WoxSettingView({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      home: Scaffold(
        appBar: AppBar(
          title: const Text('Wox Setting'),
        ),
        body: Column(
          children: [
            TextButton(
              onPressed: () async {},
              child: const Text('Close this window'),
            ),
          ],
        ),
      ),
    );
  }
}
