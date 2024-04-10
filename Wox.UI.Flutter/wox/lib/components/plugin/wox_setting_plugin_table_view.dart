import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_plugin_setting_table.dart';
import 'package:flutter/material.dart' as material;

import 'wox_setting_plugin_item_view.dart';

class WoxSettingPluginTable extends WoxSettingPluginItem {
  final PluginSettingValueTable item;

  const WoxSettingPluginTable(super.plugin, this.item, super.onUpdate, {super.key, required});

  Widget columnWidth({required Widget child, required int width}) {
    if (width == 0) return Expanded(child: child);

    return SizedBox(
      width: width.toDouble(),
      child: child,
    );
  }

  Widget buildHeader() {
    return Column(
      children: [
        Row(
          children: [
            for (var column in item.columns)
              columnWidth(
                child: Padding(
                  padding: const EdgeInsets.only(left: 0, right: 8, top: 8, bottom: 8),
                  child: Text(
                    column.label,
                    style: const TextStyle(
                      overflow: TextOverflow.ellipsis,
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                width: column.width,
              )
          ],
        ),
        const material.Divider(thickness: 0.4, indent: 0)
      ],
    );
  }

  Widget buildRowValue(PluginSettingValueTableColumn column, Map<String, dynamic> row) {
    var value = row[column.key] ?? "";

    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeText) {
      return Text(
        value,
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeCheckbox) {
      return Row(
        children: [
          value == "true" ? const Icon(material.Icons.check_box) : const Icon(material.Icons.check_box_outline_blank),
        ],
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeTextList) {
      return Column(
        children: [
          for (var txt in value)
            Padding(
              padding: const EdgeInsets.only(bottom: 6.0),
              child: Text("${(value as List<dynamic>).length == 1 ? "" : "-"} $txt"),
            ),
        ],
      );
    }
    if (column.type == PluginSettingValueType.pluginSettingValueTableColumnTypeWoxImage) {
      final woxImage = WoxImage.fromJson(value);
      return Row(
        children: [
          WoxImageView(woxImage: woxImage, width: 24, height: 24),
        ],
      );
    }

    return Text("Unknown column type: ${column.type}");
  }

  Widget buildRows() {
    var rowsJson = getSetting(item.key);
    if (rowsJson == "") {
      return const Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text("No rows"),
        ],
      );
    }

    var rows = json.decode(rowsJson);
    return Column(
      children: [
        for (var row in rows)
          Column(
            children: [
              Row(
                children: [
                  for (var column in item.columns)
                    columnWidth(
                      child: Padding(
                        padding: const EdgeInsets.only(top: 8, bottom: 8, left: 0, right: 8),
                        child: buildRowValue(column, row),
                      ),
                      width: column.width,
                    ),
                ],
              ),
              const material.Divider(thickness: 0.4, indent: 0)
            ],
          ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    return layout(
      children: [
        buildHeader(),
        buildRows(),
      ],
      style: item.style,
    );
  }
}
