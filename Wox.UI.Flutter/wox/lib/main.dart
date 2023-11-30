import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_acrylic/flutter_acrylic.dart';
import 'package:get/get.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/controller.dart';
import 'package:wox/view.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();
  await Window.initialize();

  await Window.setEffect(
    effect: WindowEffect.popover,
    dark: false,
  );

  Get.put(WoxController());

  WindowOptions windowOptions = const WindowOptions(
    size: Size(800, 300),
    center: true,
    backgroundColor: Colors.transparent,
    skipTaskbar: true,
    alwaysOnTop: true,
    titleBarStyle: TitleBarStyle.hidden,
    windowButtonVisibility: false,
  );
  if (Platform.isMacOS) {
    await windowManager.setVisibleOnAllWorkspaces(true, visibleOnFullScreen: true);
  }
  await windowManager.setAsFrameless();
  await windowManager.setResizable(false);
  await windowManager.waitUntilReadyToShow(windowOptions, () async {
    await windowManager.show();
    await windowManager.focus();
  });

  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(
      home: WoxApp(),
    );
  }
}

class WoxApp extends StatefulWidget {
  const WoxApp({super.key});

  @override
  State<WoxApp> createState() => _WoxAppState();
}

class _WoxAppState extends State<WoxApp> {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: WoxView(),
    );
  }
}
