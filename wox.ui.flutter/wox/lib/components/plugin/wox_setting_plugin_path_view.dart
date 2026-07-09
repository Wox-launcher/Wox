import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:wox/components/wox_path_finder.dart';
import 'package:wox/entity/setting/wox_plugin_setting_path.dart';
import 'package:wox/entity/validator/wox_setting_validator.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginPath extends StatefulWidget {
  final PluginSettingValuePath item;
  final String value;
  final Future<String?> Function(String key, String value) onUpdate;
  final double labelWidth;

  const WoxSettingPluginPath({super.key, required this.item, required this.value, required this.onUpdate, required this.labelWidth});

  @override
  State<WoxSettingPluginPath> createState() => _WoxSettingPluginPathState();
}

class _WoxSettingPluginPathState extends State<WoxSettingPluginPath> with WoxSettingPluginItemMixin<WoxSettingPluginPath> {
  late String _currentValue;
  late String _errorMessage;
  bool _hasInteracted = false;
  final _focusNode = FocusNode();

  @override
  double get labelWidth => widget.labelWidth;

  @override
  void initState() {
    super.initState();
    _currentValue = widget.value;
    _errorMessage = "";
    _focusNode.addListener(_onFocusChange);
  }

  @override
  void didUpdateWidget(covariant WoxSettingPluginPath oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.value != widget.value) {
      _currentValue = widget.value;
      _errorMessage = _hasInteracted ? _validateValue(widget.value) : "";
    }
  }

  @override
  void dispose() {
    _focusNode.removeListener(_onFocusChange);
    _focusNode.dispose();
    super.dispose();
  }

  String _validateValue(String value) {
    return PluginSettingValidators.validateAll(value, widget.item.validators);
  }

  Future<void> _onFocusChange() async {
    if (!_focusNode.hasFocus && _hasInteracted) {
      final validationError = _validateValue(_currentValue);
      if (mounted) {
        setState(() {
          _errorMessage = validationError;
        });
      }
      if (validationError.isNotEmpty) {
        return;
      }

      final saveError = await updateConfig(widget.onUpdate, widget.item.key, _currentValue);
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

    Widget buildField(double fieldWidth) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Flexible(
                fit: hasExplicitWidth ? FlexFit.loose : FlexFit.tight,
                child: WoxPathFinder(
                  value: _currentValue,
                  enabled: true,
                  showOpenButton: false,
                  width: fieldWidth,
                  focusNode: _focusNode,
                  isDirectory: widget.item.isDirectory,
                  allowedExtensions: widget.item.allowedExtensions.isEmpty ? null : widget.item.allowedExtensions,
                  allowMultiple: widget.item.allowMultiple,
                  onChanged: (value) {
                    _currentValue = value;
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
