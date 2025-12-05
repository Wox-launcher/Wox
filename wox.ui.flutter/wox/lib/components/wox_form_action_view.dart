import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/utils/colors.dart';

/// A form panel for collecting form action values
class WoxFormActionView extends StatefulWidget {
  final WoxResultAction action;
  final Map<String, String> initialValues;
  final String Function(String key) translate;
  final void Function(Map<String, String> values) onSave;
  final VoidCallback onCancel;

  const WoxFormActionView({
    super.key,
    required this.action,
    required this.initialValues,
    required this.translate,
    required this.onSave,
    required this.onCancel,
  });

  @override
  State<WoxFormActionView> createState() => _WoxFormActionViewState();
}

class _WoxFormActionViewState extends State<WoxFormActionView> {
  final FocusNode _firstFocusNode = FocusNode();
  int _firstFocusableIndex = -1;
  late Map<String, String> _values;
  final Map<String, TextEditingController> _textControllers = {};
  double _maxLabelWidth = 60;

  @override
  void initState() {
    super.initState();
    _values = Map<String, String>.from(widget.initialValues);

    // Find first focusable control, init text controllers, and calculate max label width
    for (int i = 0; i < widget.action.form.length; i++) {
      final item = widget.action.form[i];
      if (item.type == "textbox") {
        if (_firstFocusableIndex == -1) {
          _firstFocusableIndex = i;
        }
        final textbox = item.value as PluginSettingValueTextBox;
        _textControllers[textbox.key] = TextEditingController(
          text: _values[textbox.key] ?? textbox.defaultValue,
        );
        if (textbox.style.labelWidth > _maxLabelWidth) {
          _maxLabelWidth = textbox.style.labelWidth.toDouble();
        }
      } else if (item.type == "select") {
        final select = item.value as PluginSettingValueSelect;
        if (select.style.labelWidth > _maxLabelWidth) {
          _maxLabelWidth = select.style.labelWidth.toDouble();
        }
      }
    }

    // Request focus on the first focusable control after build
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _firstFocusNode.requestFocus();
    });
  }

  @override
  void dispose() {
    _firstFocusNode.dispose();
    for (var controller in _textControllers.values) {
      controller.dispose();
    }
    super.dispose();
  }

  void _handleSave() {
    widget.onSave(_values);
  }

  void _updateValue(String key, String value) {
    setState(() {
      _values[key] = value;
    });
  }

  Color get _textColor => getThemeTextColor();

  String _tr(String key) {
    return widget.translate(key);
  }

  @override
  Widget build(BuildContext context) {
    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.enter, control: true): _handleSave,
        const SingleActivator(LogicalKeyboardKey.escape): widget.onCancel,
      },
      child: Focus(
        autofocus: true,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Flexible(
              child: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: widget.action.form.asMap().entries.map((entry) => _buildField(entry.key, entry.value)).toList(),
                ),
              ),
            ),
            const SizedBox(height: 10),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                WoxButton.secondary(
                  text: "${_tr("ui_cancel")} (Esc)",
                  padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12),
                  onPressed: widget.onCancel,
                ),
                const SizedBox(width: 12),
                WoxButton.primary(
                  text: "${_tr("ui_save")} (Ctrl+Enter)",
                  padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 12),
                  onPressed: _handleSave,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildField(int index, PluginSettingDefinitionItem item) {
    final isFirstFocusable = index == _firstFocusableIndex;

    switch (item.type) {
      case "checkbox":
        final checkbox = item.value as PluginSettingValueCheckBox;
        return _buildCheckbox(checkbox);
      case "textbox":
        final textbox = item.value as PluginSettingValueTextBox;
        return _buildTextbox(textbox, isFirstFocusable);
      case "select":
        final select = item.value as PluginSettingValueSelect;
        return _buildSelect(select);
      case "head":
        final head = item.value as PluginSettingValueHead;
        return _buildHead(head);
      case "label":
        final label = item.value as PluginSettingValueLabel;
        return _buildLabel(label);
      case "newline":
        return const SizedBox(height: 8);
      default:
        return Padding(
          padding: const EdgeInsets.only(bottom: 8),
          child: Text(
            widget.translate("ui_not_supported_field"),
            style: TextStyle(color: getThemeTextColor(), fontSize: 12),
          ),
        );
    }
  }

  Widget _buildCheckbox(PluginSettingValueCheckBox item) {
    final currentValue = _values[item.key] ?? item.defaultValue;
    final isChecked = currentValue == "true";

    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              SizedBox(
                width: _maxLabelWidth,
                child: Text(
                  _tr(item.label),
                  style: TextStyle(color: _textColor.withValues(alpha: 0.92), fontSize: 14, fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(width: 10),
              WoxCheckbox(
                value: isChecked,
                onChanged: (value) {
                  _updateValue(item.key, value.toString());
                },
              ),
            ],
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 4, left: _maxLabelWidth + 10),
              child: Text(
                _tr(item.tooltip),
                style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: 12),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildTextbox(PluginSettingValueTextBox item, bool isFirstFocusable) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              SizedBox(
                width: _maxLabelWidth,
                child: Text(
                  _tr(item.label),
                  style: TextStyle(color: _textColor.withValues(alpha: 0.92), fontSize: 14, fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: WoxTextField(
                  controller: _textControllers[item.key],
                  focusNode: isFirstFocusable ? _firstFocusNode : null,
                  maxLines: item.maxLines > 0 ? item.maxLines : 1,
                  onChanged: (value) {
                    _updateValue(item.key, value);
                  },
                ),
              ),
              if (item.suffix.isNotEmpty) ...[
                const SizedBox(width: 4),
                Text(
                  _tr(item.suffix),
                  style: TextStyle(color: _textColor, fontSize: 13),
                ),
              ],
            ],
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 4, left: _maxLabelWidth + 10),
              child: Text(
                _tr(item.tooltip),
                style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: 12),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildSelect(PluginSettingValueSelect item) {
    final currentValue = _values[item.key] ?? item.defaultValue;

    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              SizedBox(
                width: _maxLabelWidth,
                child: Text(
                  _tr(item.label),
                  style: TextStyle(color: _textColor.withValues(alpha: 0.92), fontSize: 14, fontWeight: FontWeight.w600),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: WoxDropdownButton<String>(
                  value: currentValue,
                  isExpanded: true,
                  fontSize: 13,
                  onChanged: (value) {
                    if (value != null) {
                      _updateValue(item.key, value);
                    }
                  },
                  items: item.options.map((option) {
                    return WoxDropdownItem(
                      value: option.value,
                      label: _tr(option.label),
                    );
                  }).toList(),
                ),
              ),
              if (item.suffix.isNotEmpty) ...[
                const SizedBox(width: 4),
                Text(
                  _tr(item.suffix),
                  style: TextStyle(color: _textColor, fontSize: 13),
                ),
              ],
            ],
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 4, left: _maxLabelWidth + 10),
              child: Text(
                _tr(item.tooltip),
                style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: 12),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildHead(PluginSettingValueHead item) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8, top: 8),
      child: Text(
        _tr(item.content),
        style: TextStyle(
          color: _textColor,
          fontSize: 15,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }

  Widget _buildLabel(PluginSettingValueLabel item) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: EdgeInsets.only(left: _maxLabelWidth + 10),
            child: Text(
              _tr(item.content),
              style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: 12),
            ),
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: 4, left: _maxLabelWidth + 10),
              child: Text(
                _tr(item.tooltip),
                style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: 12),
              ),
            ),
        ],
      ),
    );
  }
}
