import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_platform_focus.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_platform_hotkey_util.dart';
import 'package:wox/utils/wox_text_measure_util.dart';

/// A form panel for collecting form action values
class WoxFormActionView extends StatefulWidget {
  final WoxResultAction action;
  final Map<String, String> initialValues;
  final String Function(String key) translate;
  final void Function(Map<String, String> values) onSave;
  final VoidCallback onCancel;

  const WoxFormActionView({super.key, required this.action, required this.initialValues, required this.translate, required this.onSave, required this.onCancel});

  @override
  State<WoxFormActionView> createState() => _WoxFormActionViewState();
}

class _WoxFormActionViewState extends State<WoxFormActionView> {
  final FocusNode _firstFocusNode = FocusNode();
  final FocusNode _formFocusNode = FocusNode();
  int _firstFocusableIndex = -1;
  late Map<String, String> _values;
  final Map<String, TextEditingController> _textControllers = {};
  double _maxLabelWidth = 60;
  bool _formInitialized = false;

  WoxInterfaceSizeMetrics get _metrics => WoxInterfaceSizeUtil.instance.current;
  double get _bodyFontSize => _metrics.scaledSpacing(13);
  double get _labelFontSize => _metrics.scaledSpacing(14);
  double get _helpFontSize => _metrics.scaledSpacing(12);
  double get _headFontSize => _metrics.scaledSpacing(15);
  double get _labelGap => _metrics.scaledSpacing(10);

  double _measureLabelWidth(BuildContext context, String text) {
    final trimmed = text.trim();
    if (trimmed.isEmpty) {
      return 60;
    }

    return WoxTextMeasureUtil.measureTextWidth(context: context, text: trimmed, style: TextStyle(fontSize: _labelFontSize, fontWeight: FontWeight.w600)) +
        _metrics.scaledSpacing(8);
  }

  @override
  void initState() {
    super.initState();
    _values = Map<String, String>.from(widget.initialValues);

    // Find first focusable control and init text controllers.
    for (int i = 0; i < widget.action.form.length; i++) {
      final item = widget.action.form[i];
      if (item.type == "textbox") {
        if (_firstFocusableIndex == -1) {
          _firstFocusableIndex = i;
        }

        final textbox = item.value as PluginSettingValueTextBox;
        _textControllers[textbox.key] = TextEditingController(text: _values[textbox.key] ?? textbox.defaultValue);
      } else if (item.type == "select") {
        if (_firstFocusableIndex == -1) {
          _firstFocusableIndex = i;
        }
      }
    }
    _formInitialized = true;

    // Request focus on the first focusable control after build
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_firstFocusableIndex != -1) {
        _firstFocusNode.requestFocus();
      } else {
        _formFocusNode.requestFocus();
      }
    });
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_formInitialized) {
      return;
    }

    double maxLabelWidth = 60;
    for (int i = 0; i < widget.action.form.length; i++) {
      final item = widget.action.form[i];
      if (item.type == "textbox") {
        final textbox = item.value as PluginSettingValueTextBox;
        final measuredWidth = _measureLabelWidth(context, widget.translate(textbox.label));
        if (measuredWidth > maxLabelWidth) {
          maxLabelWidth = measuredWidth;
        }
      } else if (item.type == "select") {
        final select = item.value as PluginSettingValueSelect;
        final measuredWidth = _measureLabelWidth(context, widget.translate(select.label));
        if (measuredWidth > maxLabelWidth) {
          maxLabelWidth = measuredWidth;
        }
      } else if (item.type == "checkbox") {
        final checkbox = item.value as PluginSettingValueCheckBox;
        final measuredWidth = _measureLabelWidth(context, widget.translate(checkbox.label));
        if (measuredWidth > maxLabelWidth) {
          maxLabelWidth = measuredWidth;
        }
      }
    }
    _maxLabelWidth = maxLabelWidth;
  }

  @override
  void dispose() {
    _firstFocusNode.dispose();
    _formFocusNode.dispose();
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
      bindings: {WoxPlatformHotkeyUtil.primaryActivator(LogicalKeyboardKey.enter): _handleSave},
      child: WoxPlatformFocus(
        focusNode: _formFocusNode,
        autofocus: true,
        onKeyEvent: (node, event) {
          if (event is KeyDownEvent && event.logicalKey == LogicalKeyboardKey.escape) {
            widget.onCancel();
            return KeyEventResult.handled;
          }
          return KeyEventResult.ignored;
        },
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
            SizedBox(height: _labelGap),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                WoxButton.secondary(
                  text: "${_tr("ui_cancel")} (Esc)",
                  fontSize: _bodyFontSize,
                  padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(22), vertical: _metrics.scaledSpacing(12)),
                  onPressed: widget.onCancel,
                ),
                SizedBox(width: _metrics.scaledSpacing(12)),
                WoxButton.primary(
                  text: "${_tr("ui_save")} (${WoxPlatformHotkeyUtil.primaryHotkeyLabel("enter")})",
                  fontSize: _bodyFontSize,
                  padding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(28), vertical: _metrics.scaledSpacing(12)),
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
        return _buildSelect(select, isFirstFocusable);
      case "head":
        final head = item.value as PluginSettingValueHead;
        return _buildHead(head);
      case "label":
        final label = item.value as PluginSettingValueLabel;
        return _buildLabel(label);
      case "newline":
        return SizedBox(height: _metrics.scaledSpacing(8));
      default:
        return Padding(
          padding: EdgeInsets.only(bottom: _metrics.scaledSpacing(8)),
          child: Text(widget.translate("ui_not_supported_field"), style: TextStyle(color: getThemeTextColor(), fontSize: _helpFontSize)),
        );
    }
  }

  Widget _buildCheckbox(PluginSettingValueCheckBox item) {
    final currentValue = _values[item.key] ?? item.defaultValue;
    final isChecked = currentValue == "true";

    return Padding(
      padding: EdgeInsets.only(bottom: _labelGap),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              SizedBox(
                width: _maxLabelWidth,
                child: Text(_tr(item.label), style: TextStyle(color: _textColor.withValues(alpha: 0.92), fontSize: _labelFontSize, fontWeight: FontWeight.w600)),
              ),
              SizedBox(width: _labelGap),
              WoxCheckbox(
                size: _metrics.quickSelectSize,
                value: isChecked,
                onChanged: (value) {
                  _updateValue(item.key, value.toString());
                },
              ),
            ],
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: _metrics.scaledSpacing(4), left: _maxLabelWidth + _labelGap),
              child: Text(_tr(item.tooltip), style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: _helpFontSize)),
            ),
        ],
      ),
    );
  }

  Widget _buildTextbox(PluginSettingValueTextBox item, bool isFirstFocusable) {
    return Padding(
      padding: EdgeInsets.only(bottom: _labelGap),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              SizedBox(
                width: _maxLabelWidth,
                child: Text(_tr(item.label), style: TextStyle(color: _textColor.withValues(alpha: 0.92), fontSize: _labelFontSize, fontWeight: FontWeight.w600)),
              ),
              SizedBox(width: _labelGap),
              Expanded(
                // Form actions live inside the launcher action panel, so their
                // text controls follow interface density while plugin settings
                // continue to use their existing fixed setting components.
                child: WoxTextField(
                  controller: _textControllers[item.key],
                  focusNode: isFirstFocusable ? _firstFocusNode : null,
                  maxLines: item.maxLines > 0 ? item.maxLines : 1,
                  style: TextStyle(color: _textColor, fontSize: _bodyFontSize),
                  hintStyle: TextStyle(color: _textColor.withValues(alpha: 0.5), fontSize: _bodyFontSize),
                  contentPadding: EdgeInsets.symmetric(horizontal: _metrics.scaledSpacing(8), vertical: _labelGap),
                  onChanged: (value) {
                    _updateValue(item.key, value);
                  },
                ),
              ),
              if (item.suffix.isNotEmpty) ...[SizedBox(width: _metrics.scaledSpacing(4)), Text(_tr(item.suffix), style: TextStyle(color: _textColor, fontSize: _bodyFontSize))],
            ],
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: _metrics.scaledSpacing(4), left: _maxLabelWidth + _labelGap),
              child: Text(_tr(item.tooltip), style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: _helpFontSize)),
            ),
        ],
      ),
    );
  }

  Widget _buildSelect(PluginSettingValueSelect item, bool isFirstFocusable) {
    final currentValue = _values[item.key] ?? item.defaultValue;

    return Padding(
      padding: EdgeInsets.only(bottom: _labelGap),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              SizedBox(
                width: _maxLabelWidth,
                child: Text(_tr(item.label), style: TextStyle(color: _textColor.withValues(alpha: 0.92), fontSize: _labelFontSize, fontWeight: FontWeight.w600)),
              ),
              SizedBox(width: _labelGap),
              Expanded(
                child: WoxDropdownButton<String>(
                  value: currentValue,
                  isExpanded: true,
                  fontSize: _bodyFontSize,
                  iconSize: _metrics.toolbarIconSize,
                  focusNode: isFirstFocusable ? _firstFocusNode : null,
                  onChanged: (value) {
                    if (value != null) {
                      _updateValue(item.key, value);
                    }
                  },
                  items:
                      item.options.map((option) {
                        return WoxDropdownItem(value: option.value, label: _tr(option.label));
                      }).toList(),
                ),
              ),
              if (item.suffix.isNotEmpty) ...[SizedBox(width: _metrics.scaledSpacing(4)), Text(_tr(item.suffix), style: TextStyle(color: _textColor, fontSize: _bodyFontSize))],
            ],
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: _metrics.scaledSpacing(4), left: _maxLabelWidth + _labelGap),
              child: Text(_tr(item.tooltip), style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: _helpFontSize)),
            ),
        ],
      ),
    );
  }

  Widget _buildHead(PluginSettingValueHead item) {
    return Padding(
      padding: EdgeInsets.only(bottom: _metrics.scaledSpacing(8), top: _metrics.scaledSpacing(8)),
      child: Text(_tr(item.content), style: TextStyle(color: _textColor, fontSize: _headFontSize, fontWeight: FontWeight.w600)),
    );
  }

  Widget _buildLabel(PluginSettingValueLabel item) {
    return Padding(
      padding: EdgeInsets.only(bottom: _labelGap),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: EdgeInsets.only(left: _maxLabelWidth + _labelGap),
            child: Text(_tr(item.content), style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: _helpFontSize)),
          ),
          if (item.tooltip.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(top: _metrics.scaledSpacing(4), left: _maxLabelWidth + _labelGap),
              child: Text(_tr(item.tooltip), style: TextStyle(color: _textColor.withValues(alpha: 0.6), fontSize: _helpFontSize)),
            ),
        ],
      ),
    );
  }
}
