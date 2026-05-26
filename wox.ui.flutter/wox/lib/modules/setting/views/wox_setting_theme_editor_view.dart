import 'package:flutter/material.dart';
import 'package:wox/components/wox_theme_editor.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/wox_theme_util.dart';

class WoxSettingThemeEditorView extends StatefulWidget {
  const WoxSettingThemeEditorView({super.key});

  @override
  State<WoxSettingThemeEditorView> createState() => _WoxSettingThemeEditorViewState();
}

class _WoxSettingThemeEditorViewState extends State<WoxSettingThemeEditorView> {
  late final WoxTheme _initialTheme;

  @override
  void initState() {
    super.initState();
    _initialTheme = WoxTheme.fromJson(Map<String, dynamic>.from(WoxThemeUtil.instance.currentTheme.value.toJson()));
  }

  @override
  Widget build(BuildContext context) {
    return Padding(padding: const EdgeInsets.all(20), child: WoxThemeEditor(initialTheme: _initialTheme));
  }
}
