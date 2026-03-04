import 'package:flutter/material.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginTextBox extends WoxSettingPluginItem {
  final PluginSettingValueTextBox item;

  WoxSettingPluginTextBox({super.key, required this.item, required super.value, required super.onUpdate, required super.labelWidth}) {
    if (item.maxLines < 1) {
      item.maxLines = 1;
    }
  }

  @override
  Widget build(BuildContext context) {
    final inputWidth = item.style.width > 0 ? item.style.width.toDouble() : 100.0;
    return layout(
      label: item.label,
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [_TextBoxField(item: item, initialValue: getSetting(item.key), inputWidth: inputWidth, onSave: (value) => updateConfig(item.key, value)), suffix(item.suffix)],
      ),
      style: item.style,
      tooltip: item.tooltip,
    );
  }
}

class _TextBoxField extends StatefulWidget {
  final PluginSettingValueTextBox item;
  final String initialValue;
  final double inputWidth;
  final ValueChanged<String> onSave;

  const _TextBoxField({required this.item, required this.initialValue, required this.inputWidth, required this.onSave});

  @override
  State<_TextBoxField> createState() => _TextBoxFieldState();
}

class _TextBoxFieldState extends State<_TextBoxField> {
  late final TextEditingController _controller;
  late final FocusNode _focusNode;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialValue);
    _focusNode = FocusNode();
    _focusNode.addListener(_onFocusChange);
  }

  @override
  void dispose() {
    _focusNode.removeListener(_onFocusChange);
    _focusNode.dispose();
    _controller.dispose();
    super.dispose();
  }

  void _onFocusChange() {
    if (!_focusNode.hasFocus) {
      for (var element in widget.item.validators) {
        var errMsg = element.validator.validate(_controller.text);
        widget.item.tooltip = errMsg;
        if (errMsg != "") {
          return;
        }
      }

      widget.onSave(_controller.text);
    }
  }

  @override
  Widget build(BuildContext context) {
    return WoxTextField(
      maxLines: widget.item.maxLines,
      controller: _controller,
      focusNode: _focusNode,
      width: widget.inputWidth,
      onChanged: (value) {
        for (var element in widget.item.validators) {
          var errMsg = element.validator.validate(value);
          widget.item.tooltip = errMsg;
          break;
        }
      },
    );
  }
}
