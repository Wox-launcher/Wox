import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_plugin_setting_table.dart';

class WoxSettingPluginTableUpdate extends StatefulWidget {
  final PluginSettingValueTable item;
  final Map<String, dynamic> row;
  final PluginDetail plugin;
  final Function onUpdate;

  const WoxSettingPluginTableUpdate({super.key, required this.item, required this.row, required this.plugin, required this.onUpdate});

  @override
  State<WoxSettingPluginTableUpdate> createState() => _WoxSettingPluginTableUpdateState();
}

class _WoxSettingPluginTableUpdateState extends State<WoxSettingPluginTableUpdate> {
  Map<String, dynamic> values = {};
  bool isUpdate = false;

  @override
  void initState() {
    super.initState();

    widget.row.forEach((key, value) {
      values[key] = value;
    });

    if (values.isEmpty) {
      for (var column in widget.item.columns) {
        values[column.key] = "";
      }
    } else {
      isUpdate = true;
    }
  }

  dynamic getValue(String key) {
    return values[key] ?? "";
  }

  bool getValueBool(String key) {
    if (values[key] == null) {
      return false;
    }
    if (values[key] is bool) {
      return values[key];
    }
    if (values[key] is String) {
      return values[key] == "true";
    }

    return false;
  }

  void updateValue(String key, dynamic value) {
    values[key] = value;
  }

  double getMaxColumnWidth() {
    double max = 0;
    for (var column in widget.item.columns) {
      if (column.width > max) {
        max = column.width.toDouble();
      }
    }

    return max > 0 ? max : 100;
  }

  Widget buildColumn(PluginSettingValueTableColumn column) {
    switch (column.type) {
      case PluginSettingValueType.pluginSettingValueTableColumnTypeText:
        return Expanded(
          child: TextBox(
            controller: TextEditingController(text: isUpdate ? getValue(column.key) : ""),
            onChanged: (value) {
              updateValue(column.key, value);
            },
            maxLines: column.textMaxLines,
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox:
        return Checkbox(
          checked: getValueBool(column.key),
          onChanged: (value) {
            updateValue(column.key, value);
            setState(() {});
          },
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeDirPath:
        return const Text("dir path...");
      case PluginSettingValueType.pluginSettingValueTableColumnTypeSelect:
        return Expanded(
          child: ComboBox<String>(
            value: getValue(column.key),
            onChanged: (value) {
              updateValue(column.key, value);
            },
            items: column.selectOptions.map((e) {
              return ComboBoxItem(
                value: e.value,
                child: Text(e.label),
              );
            }).toList(),
          ),
        );
      case PluginSettingValueType.pluginSettingValueTableColumnTypeWoxImage:
        return Text("wox image...");
      default:
        return const SizedBox();
    }
  }

  @override
  Widget build(BuildContext context) {
    return ContentDialog(
      constraints: const BoxConstraints(maxWidth: 600, maxHeight: 400),
      content: Column(children: [
        for (var column in widget.item.columns)
          Padding(
            padding: const EdgeInsets.only(bottom: 8.0),
            child: Row(children: [
              SizedBox(
                width: getMaxColumnWidth(),
                child: Text(
                  column.label,
                  style: const TextStyle(overflow: TextOverflow.ellipsis),
                  textAlign: TextAlign.right,
                ),
              ),
              const SizedBox(width: 16),
              buildColumn(column),
            ]),
          ),
      ]),
      actions: [
        Row(
          mainAxisAlignment: MainAxisAlignment.end,
          children: [
            Button(
              child: const Text('Cancel'),
              onPressed: () => Navigator.pop(context, 'User canceled dialog'),
            ),
            const SizedBox(width: 16),
            FilledButton(
              child: const Text('Confirm'),
              onPressed: () {
                Navigator.pop(context, 'User confirmed');
                widget.onUpdate(widget.item.key, values);
              },
            ),
          ],
        )
      ],
    );
  }
}
