import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_setting_plugin_checkbox_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_ai_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_textbox_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_newline.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/entity/validator/wox_setting_validator.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/entity/wox_query_requirement_settings_preview.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';
import 'package:wox/utils/log.dart';

class WoxQueryRequirementSettingsPreviewView extends StatefulWidget {
  final QueryRequirementSettingsPreviewData data;

  const WoxQueryRequirementSettingsPreviewView({super.key, required this.data});

  @override
  State<WoxQueryRequirementSettingsPreviewView> createState() => _WoxQueryRequirementSettingsPreviewViewState();
}

class _WoxQueryRequirementSettingsPreviewViewState extends State<WoxQueryRequirementSettingsPreviewView> {
  late Map<String, String> _values;
  String _saveError = "";
  bool _isSaving = false;

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  void initState() {
    super.initState();
    _values = Map<String, String>.from(widget.data.values);
  }

  @override
  void didUpdateWidget(covariant WoxQueryRequirementSettingsPreviewView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.data.values != widget.data.values) {
      _values = Map<String, String>.from(widget.data.values);
      _saveError = "";
    }
  }

  Future<String?> _updateLocalValue(String key, String value) async {
    setState(() {
      _values[key] = value;
      _saveError = "";
    });
    return null;
  }

  String _definitionKey(PluginSettingDefinitionItem item) {
    if (item.type == "newline" || item.value == null) {
      return "";
    }
    if (item.type == "checkbox" || item.type == "textbox" || item.type == "select" || item.type == "selectAIModel" || item.type == "table") {
      return (item.value as dynamic).key as String? ?? "";
    }
    return "";
  }

  List<PluginSettingValidatorItem> _definitionValidators(PluginSettingDefinitionItem item) {
    if (item.type == "textbox") {
      return (item.value as PluginSettingValueTextBox).validators;
    }
    if (item.type == "select") {
      return (item.value as PluginSettingValueSelect).validators;
    }
    if (item.type == "selectAIModel") {
      return (item.value as PluginSettingValueSelectAIModel).validators;
    }
    return [];
  }

  String _validateValues() {
    for (final item in widget.data.settingDefinitions) {
      final key = _definitionKey(item);
      if (key.isEmpty) {
        continue;
      }
      final validationMessage = PluginSettingValidators.validateAll(_values[key] ?? "", _definitionValidators(item));
      if (validationMessage.trim().isNotEmpty) {
        return validationMessage;
      }
    }
    return "";
  }

  Future<void> _saveAndRefresh() async {
    FocusScope.of(context).unfocus();
    await Future<void>.delayed(const Duration(milliseconds: 60));

    final validationMessage = _validateValues();
    if (validationMessage.trim().isNotEmpty) {
      setState(() {
        _saveError = tr(validationMessage);
      });
      return;
    }

    setState(() {
      _isSaving = true;
      _saveError = "";
    });

    final traceId = const UuidV4().generate();
    try {
      // Settings are saved only after the compact requirement form validates.
      // The previous full-settings detour forced users out of their query flow,
      // so this keeps the fix local and refreshes the same query immediately.
      for (final item in widget.data.settingDefinitions) {
        final key = _definitionKey(item);
        if (key.isEmpty) {
          continue;
        }
        await WoxApi.instance.updatePluginSetting(traceId, widget.data.pluginId, key, _values[key] ?? "");
      }

      if (!mounted) {
        return;
      }
      Get.find<WoxLauncherController>().onRefreshQuery(traceId, false);
    } catch (e) {
      Logger.instance.error(traceId, "failed to save query requirement settings: $e");
      if (!mounted) {
        return;
      }
      setState(() {
        _saveError = e.toString();
      });
    } finally {
      if (mounted) {
        setState(() {
          _isSaving = false;
        });
      }
    }
  }

  Widget _buildSetting(PluginSettingDefinitionItem item) {
    const labelWidth = PLUGIN_SETTING_PREVIEW_LABEL_WIDTH;
    final key = _definitionKey(item);
    final value = key.isEmpty ? "" : (_values[key] ?? "");

    if (item.type == "checkbox") {
      return WoxSettingPluginCheckbox(value: value, item: item.value as PluginSettingValueCheckBox, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }
    if (item.type == "textbox") {
      return WoxSettingPluginTextBox(value: value, item: item.value as PluginSettingValueTextBox, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }
    if (item.type == "newline") {
      return WoxSettingPluginNewLine(value: "", item: item.value as PluginSettingValueNewLine, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }
    if (item.type == "select") {
      return WoxSettingPluginSelect(value: value, item: item.value as PluginSettingValueSelect, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }
    if (item.type == "selectAIModel") {
      return WoxSettingPluginSelectAIModel(value: value, item: item.value as PluginSettingValueSelectAIModel, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }
    if (item.type == "head") {
      return WoxSettingPluginHead(value: "", item: item.value as PluginSettingValueHead, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }
    if (item.type == "label") {
      return WoxSettingPluginLabel(value: "", item: item.value as PluginSettingValueLabel, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }
    if (item.type == "table") {
      return WoxSettingPluginTable(value: value, item: item.value as PluginSettingValueTable, labelWidth: labelWidth, onUpdate: _updateLocalValue);
    }

    return Text(item.type, style: TextStyle(color: getThemeTextColor()));
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(18),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(widget.data.title, style: TextStyle(color: getThemeTextColor(), fontSize: 18, fontWeight: FontWeight.w600)),
          const SizedBox(height: 8),
          Text(widget.data.message, style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
          const SizedBox(height: 18),
          Expanded(child: SingleChildScrollView(child: Wrap(crossAxisAlignment: WrapCrossAlignment.center, children: widget.data.settingDefinitions.map(_buildSetting).toList()))),
          if (_saveError.trim().isNotEmpty)
            Padding(padding: const EdgeInsets.only(top: 8, bottom: 8), child: Text(_saveError, style: const TextStyle(color: Colors.red, fontSize: 12))),
          Align(
            alignment: Alignment.centerRight,
            child: WoxButton.primary(
              text: _isSaving ? "${tr("ui_save")}..." : tr("ui_save"),
              padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 12),
              onPressed: _isSaving ? null : _saveAndRefresh,
            ),
          ),
        ],
      ),
    );
  }
}
