import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/entity/validator/wox_setting_validator.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginTextBox extends StatefulWidget {
  final PluginSettingValueTextBox item;
  final String value;
  final Future<String?> Function(String key, String value) onUpdate;
  final double labelWidth;

  const WoxSettingPluginTextBox({super.key, required this.item, required this.value, required this.onUpdate, required this.labelWidth});

  @override
  State<WoxSettingPluginTextBox> createState() => _WoxSettingPluginTextBoxState();
}

class _WoxSettingPluginTextBoxState extends State<WoxSettingPluginTextBox> with WoxSettingPluginItemMixin<WoxSettingPluginTextBox> {
  late final TextEditingController _controller;
  late final FocusNode _focusNode;
  late String _errorMessage;
  bool _hasInteracted = false;

  @override
  double get labelWidth => widget.labelWidth;

  @override
  void initState() {
    super.initState();
    if (widget.item.maxLines < 1) {
      widget.item.maxLines = 1;
    }

    _controller = TextEditingController(text: widget.value);
    _focusNode = FocusNode();
    _focusNode.addListener(_onFocusChange);
    _errorMessage = "";
  }

  @override
  void didUpdateWidget(covariant WoxSettingPluginTextBox oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.value != widget.value && widget.value != _controller.text) {
      _controller.text = widget.value;
      _errorMessage = _hasInteracted ? _validateValue(widget.value) : "";
    }
  }

  @override
  void dispose() {
    _focusNode.removeListener(_onFocusChange);
    _focusNode.dispose();
    _controller.dispose();
    super.dispose();
  }

  String _validateValue(String value) {
    return PluginSettingValidators.validateAll(value, widget.item.validators);
  }

  Future<void> _onFocusChange() async {
    if (!_focusNode.hasFocus) {
      final validationError = _validateValue(_controller.text);
      if (mounted) {
        setState(() {
          _hasInteracted = true;
          _errorMessage = validationError;
        });
      }
      if (validationError.isNotEmpty) {
        return;
      }

      final saveError = await updateConfig(widget.onUpdate, widget.item.key, _controller.text);
      if (!mounted) {
        return;
      }

      setState(() {
        _errorMessage = saveError ?? "";
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final hasExplicitWidth = widget.item.style.width > 0;
    final requestedInputWidth = hasExplicitWidth ? widget.item.style.width.toDouble() : double.infinity;
    final maxLines = widget.item.maxLines > 0 ? widget.item.maxLines : 1;

    Widget buildField(double fieldWidth) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Flexible(
                fit: hasExplicitWidth ? FlexFit.loose : FlexFit.tight,
                child: WoxTextField(
                  maxLines: maxLines,
                  controller: _controller,
                  focusNode: _focusNode,
                  width: fieldWidth,
                  onChanged: (value) {
                    final validationError = _validateValue(value);
                    if (_hasInteracted && _errorMessage == validationError) {
                      return;
                    }

                    setState(() {
                      _hasInteracted = true;
                      _errorMessage = validationError;
                    });
                  },
                ),
              ),
              suffix(widget.item.suffix),
            ],
          ),
          validationMessage(_errorMessage),
        ],
      );
    }

    return layout(
      label: widget.item.label,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final effectiveWidth = hasExplicitWidth ? math.min(requestedInputWidth, constraints.maxWidth) : constraints.maxWidth;
          return buildField(effectiveWidth);
        },
      ),
      style: widget.item.style,
      tooltip: widget.item.tooltip,
    );
  }
}
