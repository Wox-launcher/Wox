import 'package:flutter/material.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/validator/wox_setting_validator.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginSelect extends StatefulWidget {
  final PluginSettingValueSelect item;
  final String value;
  final Future<String?> Function(String key, String value) onUpdate;
  final double labelWidth;

  const WoxSettingPluginSelect({super.key, required this.item, required this.value, required this.onUpdate, required this.labelWidth});

  @override
  State<WoxSettingPluginSelect> createState() => _WoxSettingPluginSelectState();
}

class _WoxSettingPluginSelectState extends State<WoxSettingPluginSelect> with WoxSettingPluginItemMixin<WoxSettingPluginSelect> {
  late String _rawValue;
  late String _errorMessage;

  @override
  double get labelWidth => widget.labelWidth;

  @override
  void initState() {
    super.initState();
    _rawValue = widget.value;
    _errorMessage = _validateCurrentValue();
  }

  @override
  void didUpdateWidget(covariant WoxSettingPluginSelect oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.value != widget.value) {
      _rawValue = widget.value;
      _errorMessage = _validateCurrentValue();
    }
  }

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

  dynamic _currentValidationValue() {
    if (!widget.item.isMulti) {
      return _rawValue;
    }

    return _parseMultiValues(_rawValue);
  }

  String _validateCurrentValue() {
    return PluginSettingValidators.validateAll(_currentValidationValue(), widget.item.validators);
  }

  Future<void> _saveValue(String rawValue) async {
    setState(() {
      _rawValue = rawValue;
      _errorMessage = _validateCurrentValue();
    });

    if (_errorMessage.isNotEmpty) {
      return;
    }

    final saveError = await updateConfig(widget.onUpdate, widget.item.key, rawValue);
    if (!mounted) {
      return;
    }

    setState(() {
      _errorMessage = saveError ?? "";
    });
  }

  @override
  Widget build(BuildContext context) {
    final dropdownWidth = widget.item.style.width > 0 ? widget.item.style.width.toDouble() : null;
    final valueExists = widget.item.options.any((option) => option.value == _rawValue);
    final effectiveValue = valueExists ? _rawValue : (widget.item.options.isNotEmpty ? widget.item.options.first.value : null);

    return layout(
      label: widget.item.label,
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              WoxDropdownButton<String>(
                value: widget.item.isMulti ? null : effectiveValue,
                isExpanded: true,
                width: dropdownWidth,
                multiSelect: widget.item.isMulti,
                multiValues: widget.item.isMulti ? _parseMultiValues(_rawValue) : const [],
                items:
                    widget.item.options.map((e) {
                      return WoxDropdownItem(value: e.value, label: e.label, leading: _buildOptionLeading(e), isSelectAll: e.isSelectAll);
                    }).toList(),
                onChanged: (v) {
                  if (widget.item.isMulti) {
                    return;
                  }
                  _saveValue(v ?? "");
                },
                onMultiChanged: (values) {
                  if (!widget.item.isMulti) {
                    return;
                  }
                  _saveValue(_encodeMultiValues(values));
                },
              ),
              validationMessage(_errorMessage),
            ],
          ),
          suffix(widget.item.suffix),
        ],
      ),
      style: widget.item.style,
      tooltip: widget.item.tooltip,
    );
  }
}
