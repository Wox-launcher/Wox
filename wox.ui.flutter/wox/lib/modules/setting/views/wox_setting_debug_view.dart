import 'dart:async';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/components/wox_switch.dart';
import 'package:wox/modules/setting/views/wox_setting_base.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window_ids.dart';

class WoxSettingDebugView extends WoxSettingBaseView {
  const WoxSettingDebugView({super.key});

  /// Opens a Markdown dialog through Wox's stable multiple-window wrapper.
  Future<void> _showFlutterWindowingDemo() async {
    await WoxMultipleWindow.createWindow(
      id: WoxMultipleWindowIds.debugDemo,
      preferredSize: const Size(720, 420),
      preferredConstraints: const BoxConstraints(minWidth: 360, minHeight: 240),
      title: controller.tr("ui_debug_flutter_windowing_demo_title"),
      showTitleBar: false,
      mica: true,
      builder:
          (_) => _FlutterWindowingDemoWindow(
            title: controller.tr("ui_debug_flutter_windowing_demo_title"),
            markdown: controller.tr("ui_debug_flutter_windowing_demo_markdown"),
            onClose: () => unawaited(WoxMultipleWindow.closeWindow(WoxMultipleWindowIds.debugDemo)),
          ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return form(
        title: controller.tr("ui_debug"),
        description: controller.tr("ui_debug_description"),
        children: [
          formField(
            settingKey: "ShowScoreTail",
            label: controller.tr("ui_debug_show_score_tail"),
            tips: controller.tr("ui_debug_show_score_tail_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showScoreTail,
              onChanged: (bool value) {
                // New debug setting: score tails are useful when tuning ranking,
                // but keeping the switch here avoids editing backend call sites
                // whenever a developer needs to inspect scores.
                controller.updateConfig("ShowScoreTail", value.toString());
              },
            ),
          ),
          formField(
            settingKey: "ShowPerformanceTail",
            label: controller.tr("ui_debug_show_performance_tail"),
            tips: controller.tr("ui_debug_show_performance_tail_tips"),
            child: WoxSwitch(
              value: controller.woxSetting.value.showPerformanceTail,
              onChanged: (bool value) {
                // New debug setting: query timing tails were previously forced
                // in dev; the persisted toggle keeps performance inspection
                // available while letting developers turn off noisy tags.
                controller.updateConfig("ShowPerformanceTail", value.toString());
              },
            ),
          ),
          formField(
            settingKey: "FlutterWindowingDemo",
            label: controller.tr("ui_debug_flutter_windowing_demo"),
            tips: controller.tr("ui_debug_flutter_windowing_demo_tips"),
            child: WoxButton.secondary(
              text: controller.tr("ui_debug_flutter_windowing_demo_open"),
              icon: const Icon(Icons.open_in_new_rounded, size: 16),
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              onPressed: () {
                unawaited(_showFlutterWindowingDemo());
              },
            ),
          ),
        ],
      );
    });
  }
}

class _FlutterWindowingDemoWindow extends StatelessWidget {
  const _FlutterWindowingDemoWindow({required this.title, required this.markdown, required this.onClose});

  final String title;
  final String markdown;
  final VoidCallback onClose;

  @override
  Widget build(BuildContext context) {
    const textColor = Colors.white;
    final dividerColor = Colors.white.withValues(alpha: 0.16);
    final borderColor = Colors.white.withValues(alpha: 0.18);

    return Directionality(
      textDirection: TextDirection.ltr,
      child: Material(
        color: Colors.transparent,
        child: SizedBox.expand(
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: const Color(0x99151515),
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: borderColor),
              boxShadow: const [BoxShadow(color: Color(0x66000000), blurRadius: 24, offset: Offset(0, 12))],
            ),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  WoxMultipleWindowDragMoveArea(
                    windowId: WoxMultipleWindowIds.debugDemo,
                    child: Container(
                      height: 56,
                      padding: const EdgeInsets.only(left: 22, right: 12),
                      decoration: BoxDecoration(border: Border(bottom: BorderSide(color: dividerColor))),
                      child: Row(
                        children: [
                          Expanded(
                            child: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: textColor, fontSize: 17, fontWeight: FontWeight.w700)),
                          ),
                          GestureDetector(
                            behavior: HitTestBehavior.opaque,
                            onTap: onClose,
                            child: Padding(padding: const EdgeInsets.all(10), child: Icon(Icons.close_rounded, color: textColor.withValues(alpha: 0.94), size: 24)),
                          ),
                        ],
                      ),
                    ),
                  ),
                  Expanded(
                    child: SingleChildScrollView(
                      padding: const EdgeInsets.fromLTRB(24, 22, 24, 26),
                      child: WoxMarkdownView(data: markdown, fontColor: textColor, fontSize: 16, selectable: true),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
