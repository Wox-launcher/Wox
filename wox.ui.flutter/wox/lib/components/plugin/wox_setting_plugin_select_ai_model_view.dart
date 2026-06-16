import 'package:flutter/material.dart';
import 'package:wox/components/wox_ai_model_selector_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/entity/validator/wox_setting_validator.dart';

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginSelectAIModel extends StatefulWidget {
  final PluginSettingValueSelectAIModel item;
  final String value;
  final Future<String?> Function(String key, String value) onUpdate;
  final double labelWidth;

  const WoxSettingPluginSelectAIModel({super.key, required this.item, required this.value, required this.onUpdate, required this.labelWidth});

  @override
  State<WoxSettingPluginSelectAIModel> createState() => _WoxSettingPluginSelectAIModelState();
}

class _WoxSettingPluginSelectAIModelState extends State<WoxSettingPluginSelectAIModel> with WoxSettingPluginItemMixin<WoxSettingPluginSelectAIModel> {
  late String _currentValue;
  late String _errorMessage;
  bool _hasInteracted = false;

  @override
  double get labelWidth => widget.labelWidth;

  @override
  void initState() {
    super.initState();
    _currentValue = widget.value;
    _errorMessage = "";
  }

  @override
  void didUpdateWidget(covariant WoxSettingPluginSelectAIModel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.value != widget.value) {
      _currentValue = widget.value;
      _errorMessage = _hasInteracted ? PluginSettingValidators.validateAll(_currentValue, widget.item.validators) : "";
    }
  }

  Future<void> _saveValue(String modelJson) async {
    final validationError = PluginSettingValidators.validateAll(modelJson, widget.item.validators);
    setState(() {
      _currentValue = modelJson;
      _hasInteracted = true;
      _errorMessage = validationError;
    });

    if (validationError.isNotEmpty) {
      return;
    }

    final saveError = await updateConfig(widget.onUpdate, widget.item.key, modelJson);
    if (!mounted) {
      return;
    }

    setState(() {
      _errorMessage = saveError ?? "";
    });
  }

  @override
  Widget build(BuildContext context) {
    return layout(
      label: widget.item.label,
      child: Wrap(
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 6),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [WoxAIModelSelectorView(initialValue: _currentValue, onModelSelected: _saveValue), validationMessage(_errorMessage)],
            ),
          ),
          suffix(widget.item.suffix),
        ],
      ),
      style: widget.item.style,
      tooltip: widget.item.tooltip,
    );
  }
}
