import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/consts.dart';

abstract class WoxSettingPluginItem extends StatelessWidget {
  static const double defaultLabelGap = 12;
  final String value;
  final Future<String?> Function(String key, String value) onUpdate;
  final double labelWidth;

  const WoxSettingPluginItem({super.key, required this.value, required this.onUpdate, required this.labelWidth});

  Future<String?> updateConfig(String key, String value) async {
    return onUpdate(key, value);
  }

  String getSetting(String key) {
    return value;
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  static Widget validationMessage(String message, String Function(String key) translator) {
    if (message.trim().isEmpty) {
      return const SizedBox.shrink();
    }

    return Padding(padding: const EdgeInsets.only(top: 4), child: Text(translator(message), style: const TextStyle(color: Colors.red, fontSize: 12)));
  }

  Widget tooltipText(String tooltip) {
    if (tooltip.trim().isEmpty) {
      return const SizedBox.shrink();
    }

    final accentColor = getThemeActiveBackgroundColor();

    return Padding(
      padding: EdgeInsets.only(top: 2),
      child: ExcludeFocus(
        child: WoxMarkdownView(
          data: tr(tooltip),
          fontColor: getThemeSubTextColor(),
          fontSize: SETTING_TOOLTIP_DEFAULT_SIZE,
          linkColor: accentColor,
          linkHoverColor: accentColor.withValues(alpha: 0.8),
          selectable: true,
        ),
      ),
    );
  }

  Widget applyStylePadding({required PluginSettingValueStyle style, required Widget child}) {
    return Padding(padding: EdgeInsets.only(top: style.paddingTop, bottom: style.paddingBottom, left: style.paddingLeft, right: style.paddingRight), child: child);
  }

  Widget layout({
    required String label,
    required Widget child,
    required PluginSettingValueStyle style,
    String tooltip = "",
    bool includeBottomSpacing = true,
    List<Widget> labelActions = const [],
  }) {
    final hasLabel = label.trim().isNotEmpty;
    final tipsWidget = tooltip.trim().isNotEmpty ? tooltipText(tooltip) : null;
    final bottomSpacing = includeBottomSpacing ? 10.0 : 0.0;
    // Text-only labels need a small top offset to align with controls. Title-side
    // actions are taller, so keep them centered with 24px controls instead.
    final labelTopPadding = labelActions.isEmpty ? 6.0 : 1.0;

    if (!hasLabel) {
      final content = Column(crossAxisAlignment: CrossAxisAlignment.start, children: [child, if (tipsWidget != null) tipsWidget]);
      final wrappedContent = bottomSpacing > 0 ? Padding(padding: EdgeInsets.only(bottom: bottomSpacing), child: content) : content;
      return applyStylePadding(style: style, child: wrappedContent);
    }

    return applyStylePadding(
      style: style,
      child: Padding(
        padding: EdgeInsets.only(bottom: bottomSpacing),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final shouldStack = constraints.maxWidth < labelWidth + defaultLabelGap + 160;
            final labelContent = Padding(
              padding: EdgeInsets.only(top: shouldStack ? 0 : labelTopPadding),
              child: Row(
                children: [
                  Flexible(
                    child: Text(label, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500)),
                  ),
                  if (labelActions.isNotEmpty) ...[const SizedBox(width: 6), ...labelActions],
                ],
              ),
            );
            final fieldContent = Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                ConstrainedBox(constraints: BoxConstraints(maxWidth: constraints.maxWidth), child: child),
                if (tipsWidget != null) Padding(padding: const EdgeInsets.only(top: 4), child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 620), child: tipsWidget)),
              ],
            );

            if (shouldStack) {
              // Search jumps can briefly lay out plugin settings inside a very
              // narrow pane while routes switch; stack the label above the control
              // so the fixed plugin label column does not overflow.
              return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [labelContent, const SizedBox(height: 8), fieldContent]);
            }

            return Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Plugin setting panes and table-edit dialogs are narrower than top-level
                // settings, so the classic label/control split keeps controls aligned while
                // leaving the right column to carry longer descriptions.
                SizedBox(width: labelWidth, child: labelContent),
                const SizedBox(width: defaultLabelGap),
                Expanded(child: fieldContent),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget suffix(String text) {
    if (text != "") {
      return Padding(padding: const EdgeInsets.only(left: 4), child: Text(text, style: TextStyle(color: getThemeTextColor(), fontSize: 13)));
    }

    return const SizedBox.shrink();
  }
}

mixin WoxSettingPluginItemMixin<T extends StatefulWidget> on State<T> {
  double get labelWidth;

  Future<String?> updateConfig(Future<String?> Function(String key, String value) onUpdate, String key, String value) async {
    return onUpdate(key, value);
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  Widget validationMessage(String message) {
    return WoxSettingPluginItem.validationMessage(message, tr);
  }

  Widget tooltipText(String tooltip) {
    if (tooltip.trim().isEmpty) {
      return const SizedBox.shrink();
    }

    final accentColor = getThemeActiveBackgroundColor();

    return Padding(
      padding: EdgeInsets.only(top: 2),
      child: ExcludeFocus(
        child: WoxMarkdownView(
          data: tr(tooltip),
          fontColor: getThemeSubTextColor(),
          fontSize: SETTING_TOOLTIP_DEFAULT_SIZE,
          linkColor: accentColor,
          linkHoverColor: accentColor.withValues(alpha: 0.8),
          selectable: true,
        ),
      ),
    );
  }

  Widget applyStylePadding({required PluginSettingValueStyle style, required Widget child}) {
    return Padding(padding: EdgeInsets.only(top: style.paddingTop, bottom: style.paddingBottom, left: style.paddingLeft, right: style.paddingRight), child: child);
  }

  Widget layout({
    required String label,
    required Widget child,
    required PluginSettingValueStyle style,
    String tooltip = "",
    bool includeBottomSpacing = true,
    List<Widget> labelActions = const [],
  }) {
    final hasLabel = label.trim().isNotEmpty;
    final tipsWidget = tooltip.trim().isNotEmpty ? tooltipText(tooltip) : null;
    final bottomSpacing = includeBottomSpacing ? 10.0 : 0.0;
    // Text-only labels need a small top offset to align with controls. Title-side
    // actions are taller, so keep them centered with 24px controls instead.
    final labelTopPadding = labelActions.isEmpty ? 6.0 : 1.0;

    if (!hasLabel) {
      final content = Column(crossAxisAlignment: CrossAxisAlignment.start, children: [child, if (tipsWidget != null) tipsWidget]);
      final wrappedContent = bottomSpacing > 0 ? Padding(padding: EdgeInsets.only(bottom: bottomSpacing), child: content) : content;
      return applyStylePadding(style: style, child: wrappedContent);
    }

    return applyStylePadding(
      style: style,
      child: Padding(
        padding: EdgeInsets.only(bottom: bottomSpacing),
        child: LayoutBuilder(
          builder: (context, constraints) {
            final shouldStack = constraints.maxWidth < labelWidth + WoxSettingPluginItem.defaultLabelGap + 160;
            final labelContent = Padding(
              padding: EdgeInsets.only(top: shouldStack ? 0 : labelTopPadding),
              child: Row(
                children: [
                  Flexible(
                    child: Text(label, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w500)),
                  ),
                  if (labelActions.isNotEmpty) ...[const SizedBox(width: 6), ...labelActions],
                ],
              ),
            );
            final fieldContent = Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                ConstrainedBox(constraints: BoxConstraints(maxWidth: constraints.maxWidth), child: child),
                if (tipsWidget != null) Padding(padding: const EdgeInsets.only(top: 4), child: ConstrainedBox(constraints: const BoxConstraints(maxWidth: 620), child: tipsWidget)),
              ],
            );

            if (shouldStack) {
              // Search jumps can briefly lay out plugin settings inside a very
              // narrow pane while routes switch; stack the label above the control
              // so the fixed plugin label column does not overflow.
              return Column(crossAxisAlignment: CrossAxisAlignment.start, children: [labelContent, const SizedBox(height: 8), fieldContent]);
            }

            return Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Plugin setting panes and table-edit dialogs are narrower than top-level
                // settings, so the classic label/control split keeps controls aligned while
                // leaving the right column to carry longer descriptions.
                SizedBox(width: labelWidth, child: labelContent),
                const SizedBox(width: WoxSettingPluginItem.defaultLabelGap),
                Expanded(child: fieldContent),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget suffix(String text) {
    if (text != "") {
      return Padding(padding: const EdgeInsets.only(left: 4), child: Text(text, style: TextStyle(color: getThemeTextColor(), fontSize: 13)));
    }

    return const SizedBox.shrink();
  }
}
