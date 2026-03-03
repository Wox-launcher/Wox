import 'package:flutter/material.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginSelect extends WoxSettingPluginItem {
  final PluginSettingValueSelect item;

  const WoxSettingPluginSelect({super.key, required this.item, required super.value, required super.onUpdate, required super.labelWidth});

  List<String> _parseMultiValues(String rawValue) {
    if (rawValue.trim().isEmpty) {
      return [];
    }
    return rawValue.split(',').map((value) => value.trim()).where((value) => value.isNotEmpty).toList();
  }

  String _encodeMultiValues(List<String> values) {
    return values.join(',');
  }

  Widget? _buildOptionLeading(PluginSettingValueSelectOption option) {
    if (option.icon.imageData.isEmpty) {
      return null;
    }
    return WoxImageView(woxImage: option.icon, width: 16, height: 16);
  }

  @override
  Widget build(BuildContext context) {
    final dropdownWidth = item.style.width > 0 ? item.style.width.toDouble() : null;
    return layout(
      label: item.label,
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          WoxDropdownButton<String>(
            value: item.isMulti ? null : getSetting(item.key),
            isExpanded: true,
            width: dropdownWidth,
            multiSelect: item.isMulti,
            multiValues: item.isMulti ? _parseMultiValues(getSetting(item.key)) : const [],
            items:
                item.options.map((e) {
                  return WoxDropdownItem(value: e.value, label: e.label, leading: _buildOptionLeading(e), isSelectAll: e.isSelectAll);
                }).toList(),
            onChanged: (v) {
              if (item.isMulti) {
                return;
              }
              updateConfig(item.key, v ?? "");
            },
            onMultiChanged: (values) {
              if (!item.isMulti) {
                return;
              }
              updateConfig(item.key, _encodeMultiValues(values));
            },
          ),
          suffix(item.suffix),
        ],
      ),
      style: item.style,
      tooltip: item.tooltip,
    );
  }
}
