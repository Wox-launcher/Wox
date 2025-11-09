import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/picker.dart';

/// Reusable directory path picker field
/// - Shows a text field with consistent Wox border style
/// - Optional Open and Browse/Change buttons
/// - Can confirm before applying change
class WoxPathFinder extends StatefulWidget {
  final String value;
  final ValueChanged<String> onChanged;
  final bool enabled; // whether text can be edited directly
  final bool showOpenButton;
  final bool showChangeButton;
  final bool confirmOnChange;
  final String? changeButtonTextKey; // i18n key, defaults to ui_runtime_browse
  final double? width; // width for the text field; defaults to fill via double.infinity when placed in Expanded

  const WoxPathFinder({
    super.key,
    required this.value,
    required this.onChanged,
    this.enabled = false,
    this.showOpenButton = true,
    this.showChangeButton = true,
    this.confirmOnChange = false,
    this.changeButtonTextKey,
    this.width,
  });

  @override
  State<WoxPathFinder> createState() => _WoxPathFinderState();
}

class _WoxPathFinderState extends State<WoxPathFinder> {
  bool _picking = false;
  late TextEditingController _controller;

  String tr(String key) => Get.find<WoxSettingController>().tr(key);

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.value);
  }

  @override
  void didUpdateWidget(covariant WoxPathFinder oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.value != widget.value && _controller.text != widget.value) {
      _controller.text = widget.value;
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _browseAndMaybeConfirm() async {
    if (_picking) return;
    setState(() => _picking = true);
    try {
      final selectedDirectory = await FileSelector.pick(
        const UuidV4().generate(),
        FileSelectorParams(isDirectory: true),
      );
      if (selectedDirectory.isEmpty) return;
      final picked = selectedDirectory[0];

      if (!mounted) return;
      if (widget.confirmOnChange) {
        await showDialog(
          context: context,
          builder: (ctx) => AlertDialog(
            content: Text(tr("ui_data_config_location_change_confirm").replaceAll("{0}", picked)),
            actions: [
              WoxButton.secondary(
                text: tr("ui_data_config_location_change_cancel"),
                onPressed: () => Navigator.pop(ctx),
              ),
              WoxButton.primary(
                text: tr("ui_data_config_location_change_confirm_button"),
                onPressed: () {
                  Navigator.pop(ctx);
                  widget.onChanged(picked);
                },
              ),
            ],
          ),
        );
      } else {
        widget.onChanged(picked);
      }
    } finally {
      if (mounted) setState(() => _picking = false);
    }
  }

  void _openFolder() {
    // Use controller utility to open folder for minimal boilerplate
    final c = Get.find<WoxSettingController>();
    c.openFolder(widget.value);
  }

  @override
  Widget build(BuildContext context) {
    final changeText = tr(widget.changeButtonTextKey ?? 'ui_runtime_browse');

    return Row(
      children: [
        // Text field
        Expanded(
          child: WoxTextField(
            controller: _controller,
            enabled: widget.enabled,
            width: widget.width ?? double.infinity,
            onChanged: (v) => widget.onChanged(v),
          ),
        ),
        if (widget.showOpenButton) ...[
          const SizedBox(width: 10),
          WoxButton.secondary(
            text: tr('plugin_file_open'),
            onPressed: _openFolder,
          ),
        ],
        if (widget.showChangeButton) ...[
          const SizedBox(width: 10),
          WoxButton.primary(
            text: changeText,
            onPressed: _picking ? null : _browseAndMaybeConfirm,
          ),
        ],
      ],
    );
  }
}
