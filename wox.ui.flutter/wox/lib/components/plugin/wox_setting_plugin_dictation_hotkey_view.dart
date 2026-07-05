import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_dictation_hotkey.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/colors.dart';

class WoxSettingPluginDictationHotkey extends StatelessWidget {
  final String value;
  final PluginSettingValueDictationHotkey item;
  final double labelWidth;
  final Future<String?> Function(String key, String value) onUpdate;

  const WoxSettingPluginDictationHotkey({super.key, required this.value, required this.item, required this.labelWidth, required this.onUpdate});

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  Widget build(BuildContext context) {
    final hotkey = value.isNotEmpty ? WoxHotkey.parseHotkeyFromString(value) : null;
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(tr(item.label), style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500)),
          if (item.tooltip.trim().isNotEmpty) ...[const SizedBox(height: 4), Text(tr(item.tooltip), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))],
          const SizedBox(height: 8),
          WoxHotkeyRecorder(
            hotkey: hotkey,
            tipPosition: WoxHotkeyRecorderTipPosition.right,
            onHotKeyRecorded: (hotkeyStr) {
              onUpdate(item.key, hotkeyStr);
            },
            onUnavailableHotKeyRecorded: (hotkeyStr) {
              onUpdate(item.key, "");
            },
          ),
        ],
      ),
    );
  }
}
