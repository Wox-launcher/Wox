import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/plugin/wox_setting_plugin_checkbox_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_head_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_label_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_newline_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_ai_model_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_select_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_table_view.dart';
import 'package:wox/components/plugin/wox_setting_plugin_textbox_view.dart';
import 'package:wox/entity/setting/wox_plugin_setting_checkbox.dart';
import 'package:wox/entity/setting/wox_plugin_setting_head.dart';
import 'package:wox/entity/setting/wox_plugin_setting_label.dart';
import 'package:wox/entity/setting/wox_plugin_setting_newline.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select.dart';
import 'package:wox/entity/setting/wox_plugin_setting_select_ai_model.dart';
import 'package:wox/entity/setting/wox_plugin_setting_table.dart';
import 'package:wox/entity/setting/wox_plugin_setting_textbox.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/utils/color_util.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxFormActionView extends StatefulWidget {
  final WoxResultAction action;
  final Map<String, String> initialValues;
  final String Function(String key) translate;

  const WoxFormActionView({
    super.key,
    required this.action,
    required this.initialValues,
    required this.translate,
  });

  static Future<Map<String, String>?> collectValues({
    required WoxResultAction action,
    required String Function(String key) translate,
  }) async {
    // Nothing to collect
    if (action.form.isEmpty) {
      return {};
    }

    final initialValues = <String, String>{};
    for (final item in action.form) {
      final key = _getItemKey(item);
      if (key != null) {
        final defaultValue = _getDefaultValue(item);
        initialValues[key] = defaultValue;
      }
    }

    final overlayCtx = Get.overlayContext ?? Get.context ?? Get.key.currentContext;
    if (overlayCtx == null) {
      return null;
    }

    return await showDialog<Map<String, String>?>(
      context: overlayCtx,
      barrierDismissible: true,
      builder: (_) => WoxFormActionView(
        action: action,
        initialValues: initialValues,
        translate: translate,
      ),
    );
  }

  @override
  State<WoxFormActionView> createState() => _WoxFormActionViewState();

  static String _getDefaultValue(PluginSettingDefinitionItem item) {
    switch (item.type) {
      case "checkbox":
        return (item.value as PluginSettingValueCheckBox).defaultValue;
      case "textbox":
        return (item.value as PluginSettingValueTextBox).defaultValue;
      case "select":
        return (item.value as PluginSettingValueSelect).defaultValue;
      case "selectAIModel":
        return (item.value as PluginSettingValueSelectAIModel).defaultValue;
      case "table":
        return (item.value as PluginSettingValueTable).defaultValue;
      default:
        return "";
    }
  }

  static String? _getItemKey(PluginSettingDefinitionItem item) {
    switch (item.type) {
      case "checkbox":
        return (item.value as PluginSettingValueCheckBox).key;
      case "textbox":
        return (item.value as PluginSettingValueTextBox).key;
      case "select":
        return (item.value as PluginSettingValueSelect).key;
      case "selectAIModel":
        return (item.value as PluginSettingValueSelectAIModel).key;
      case "table":
        return (item.value as PluginSettingValueTable).key;
      default:
        return null;
    }
  }
}

class _WoxFormActionViewState extends State<WoxFormActionView> {
  late Map<String, String> values;

  @override
  void initState() {
    super.initState();
    values = Map<String, String>.from(widget.initialValues);
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: safeFromCssColor(WoxThemeUtil.instance.currentTheme.value.appBackgroundColor),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
      ),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 720, maxHeight: 520),
        child: Padding(
          padding: const EdgeInsets.all(16.0),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Flexible(
                    child: Text(
                      widget.action.name,
                      style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.close),
                    color: getThemeTextColor(),
                    onPressed: () => Navigator.of(context).pop(null),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              Expanded(
                child: SingleChildScrollView(
                  child: Wrap(
                    runSpacing: 8,
                    spacing: 8,
                    children: widget.action.form.map(_buildField).toList(),
                  ),
                ),
              ),
              const SizedBox(height: 12),
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  TextButton(
                    onPressed: () => Navigator.of(context).pop(null),
                    child: Text(widget.translate("ui_cancel")),
                  ),
                  const SizedBox(width: 8),
                  ElevatedButton(
                    onPressed: () => Navigator.of(context).pop(values),
                    child: Text(widget.translate("ui_save")),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildField(PluginSettingDefinitionItem item) {
    switch (item.type) {
      case "checkbox":
        final checkbox = item.value as PluginSettingValueCheckBox;
        return WoxSettingPluginCheckbox(
          value: values[checkbox.key] ?? "",
          item: checkbox,
          onUpdate: (key, value) {
            setState(() {
              values[key] = value;
            });
          },
        );
      case "textbox":
        final textbox = item.value as PluginSettingValueTextBox;
        return WoxSettingPluginTextBox(
          value: values[textbox.key] ?? "",
          item: textbox,
          onUpdate: (key, value) {
            setState(() {
              values[key] = value;
            });
          },
        );
      case "select":
        final select = item.value as PluginSettingValueSelect;
        return WoxSettingPluginSelect(
          value: values[select.key] ?? "",
          item: select,
          onUpdate: (key, value) {
            setState(() {
              values[key] = value;
            });
          },
        );
      case "selectAIModel":
        final select = item.value as PluginSettingValueSelectAIModel;
        return WoxSettingPluginSelectAIModel(
          value: values[select.key] ?? "",
          item: select,
          onUpdate: (key, value) {
            setState(() {
              values[key] = value;
            });
          },
        );
      case "table":
        final table = item.value as PluginSettingValueTable;
        return WoxSettingPluginTable(
          value: values[table.key] ?? "",
          item: table,
          onUpdate: (key, value) {
            setState(() {
              values[key] = value;
            });
          },
        );
      case "head":
        return WoxSettingPluginHead(
          value: "",
          item: item.value as PluginSettingValueHead,
          onUpdate: (_, __) {},
        );
      case "label":
        return WoxSettingPluginLabel(
          value: "",
          item: item.value as PluginSettingValueLabel,
          onUpdate: (_, __) {},
        );
      case "newline":
        return WoxSettingPluginNewLine(
          value: "",
          item: item.value as PluginSettingValueNewLine,
          onUpdate: (_, __) {},
        );
      default:
        return const SizedBox.shrink();
    }
  }
}
