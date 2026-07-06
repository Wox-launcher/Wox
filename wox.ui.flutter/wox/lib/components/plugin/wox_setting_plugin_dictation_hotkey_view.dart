import 'package:flutter/material.dart';
import 'package:wox/components/wox_hold_hotkey_recorder_view.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_dictation_hotkey.dart';
import 'package:wox/entity/wox_hotkey.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginDictationHotkey extends WoxSettingPluginItem {
  final PluginSettingValueDictationHotkey item;

  const WoxSettingPluginDictationHotkey({super.key, required this.item, required super.value, required super.onUpdate, required super.labelWidth});

  @override
  Widget build(BuildContext context) {
    final isHoldMode = item.triggerMode == 'hold';

    if (isHoldMode) {
      return layout(
        label: item.label,
        child: WoxHoldHotkeyRecorder(
          value: value,
          onRecorded: (hotkeyStr) {
            updateConfig(item.key, hotkeyStr);
          },
        ),
        style: item.style,
        tooltip: item.tooltip,
      );
    }

    final hotkey = value.isNotEmpty ? WoxHotkey.parseHotkeyFromString(value) : null;
    return layout(
      label: item.label,
      child: WoxHotkeyRecorder(
        hotkey: hotkey,
        tipPosition: WoxHotkeyRecorderTipPosition.right,
        onHotKeyRecorded: (hotkeyStr) {
          updateConfig(item.key, hotkeyStr);
        },
        onUnavailableHotKeyRecorded: (hotkeyStr) {
          updateConfig(item.key, "");
        },
      ),
      style: item.style,
      tooltip: item.tooltip,
    );
  }
}