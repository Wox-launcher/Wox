import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';

class AICommandDefaultActionValue {
  static const run = "run";
  static const runAndShow = "run_and_show";
  static const runAndPaste = "run_and_paste";
}

/// Shared default action picker for AI command creation and template installation.
class WoxAICommandDefaultActionDropdown extends StatelessWidget {
  final String value;
  final ValueChanged<String> onChanged;
  final bool isExpanded;
  final double fontSize;
  final double? width;
  final Widget? underline;

  const WoxAICommandDefaultActionDropdown({super.key, required this.value, required this.onChanged, this.isExpanded = true, this.fontSize = 13, this.width, this.underline});

  // Falls back to run so older rows with missing values still render a valid choice.
  String _normalizeValue(String value) {
    if (value == AICommandDefaultActionValue.runAndShow || value == AICommandDefaultActionValue.runAndPaste || value == AICommandDefaultActionValue.run) {
      return value;
    }
    return AICommandDefaultActionValue.run;
  }

  Widget _buildDemoTrigger(WoxSettingController controller, WoxAICommandDefaultActionDemoMode mode) {
    final foreground = getThemeTextColor();

    return WoxDemoPopover(
      popoverKey: ValueKey('ai-command-default-action-demo-${mode.name}'),
      width: 540,
      height: 360,
      demo: WoxAICommandDefaultActionDemo(mode: mode, accent: getThemeActiveBackgroundColor(), tr: controller.tr),
      child: Semantics(
        label: controller.tr("ui_demo_preview"),
        button: true,
        child: MouseRegion(
          cursor: SystemMouseCursors.help,
          child: SizedBox(width: 22, height: 22, child: Icon(Icons.play_circle_outline_rounded, color: foreground.withValues(alpha: 0.88), size: 18)),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final controller = Get.find<WoxSettingController>();
    return WoxDropdownButton<String>(
      value: _normalizeValue(value),
      isExpanded: isExpanded,
      fontSize: fontSize,
      width: width,
      underline: underline,
      onChanged: (nextValue) {
        if (nextValue != null) {
          onChanged(nextValue);
        }
      },
      items: [
        WoxDropdownItem(
          value: AICommandDefaultActionValue.run,
          label: controller.tr("plugin_ai_command_default_action_run"),
          menuTrailing: _buildDemoTrigger(controller, WoxAICommandDefaultActionDemoMode.run),
        ),
        WoxDropdownItem(
          value: AICommandDefaultActionValue.runAndShow,
          label: controller.tr("plugin_ai_command_default_action_run_and_show"),
          menuTrailing: _buildDemoTrigger(controller, WoxAICommandDefaultActionDemoMode.runAndShow),
        ),
        WoxDropdownItem(
          value: AICommandDefaultActionValue.runAndPaste,
          label: controller.tr("plugin_ai_command_default_action_run_and_paste"),
          menuTrailing: _buildDemoTrigger(controller, WoxAICommandDefaultActionDemoMode.runAndPaste),
        ),
      ],
    );
  }
}
