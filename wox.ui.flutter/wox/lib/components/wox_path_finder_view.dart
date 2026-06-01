import 'package:flutter/material.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/utils/picker.dart';

class WoxPathFinder extends StatefulWidget {
  final TextEditingController controller;
  final FocusNode focusNode;
  final ValueChanged<String>? onChanged;
  final double? width;

  const WoxPathFinder({
    super.key,
    required this.controller,
    required this.focusNode,
    this.onChanged,
    this.width,
  });

  @override
  State<WoxPathFinder> createState() => _WoxPathFinderState();
}

class _WoxPathFinderState extends State<WoxPathFinder> {
  bool _disableBrowse = false;

  Future<void> _browse() async {
    if (_disableBrowse) return;
    _disableBrowse = true;
    try {
      final selectedDirectory = await FileSelector.pick(
        const UuidV4().generate(),
        FileSelectorParams(isDirectory: true),
      );
      if (selectedDirectory.isNotEmpty) {
        widget.controller.text = selectedDirectory[0];
        widget.onChanged?.call(selectedDirectory[0]);
      }
    } finally {
      _disableBrowse = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    return WoxTextField(
      controller: widget.controller,
      focusNode: widget.focusNode,
      width: widget.width,
      onChanged: widget.onChanged,
      suffixIcon: Padding(
        padding: const EdgeInsets.only(right: 4),
        child: SizedBox(
          height: 24,
          child: TextButton(
            onPressed: _browse,
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 8),
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: const Text('Browse', style: TextStyle(fontSize: 12)),
          ),
        ),
      ),
    );
  }
}
