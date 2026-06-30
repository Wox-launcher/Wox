import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:wox/components/wox_selectable_text.dart';

class WoxMacosQuickLookPreviewView extends StatelessWidget {
  final String filePath;

  const WoxMacosQuickLookPreviewView({super.key, required this.filePath});

  @override
  Widget build(BuildContext context) {
    if (!Platform.isMacOS) {
      return WoxSelectableText("Quick Look preview is currently only available on macOS.\nFile: $filePath");
    }

    return AppKitView(key: ValueKey(filePath), viewType: "wox/quick_look_preview", creationParams: {"filePath": filePath}, creationParamsCodec: const StandardMessageCodec());
  }
}
