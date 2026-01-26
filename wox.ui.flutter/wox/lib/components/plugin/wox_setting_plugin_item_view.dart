import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_plugin_setting.dart';
import 'package:wox/utils/colors.dart';

abstract class WoxSettingPluginItem extends StatelessWidget {
  final String value;
  final Function onUpdate;

  const WoxSettingPluginItem({super.key, required this.value, required this.onUpdate});

  Future<void> updateConfig(String key, String value) async {
    onUpdate(key, value);
  }

  String getSetting(String key) {
    return value;
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  PluginSettingValueStyle resolveStyle(PluginSettingValueStyle style) {
    final woxSettingController = Get.find<WoxSettingController>();
    return style.resolve(woxSettingController.woxSetting.value.langCode);
  }

  Widget withFlexible(List<Widget> children) {
    return Wrap(crossAxisAlignment: WrapCrossAlignment.center, children: children);
  }

  Widget layout({required List<Widget> children, required PluginSettingValueStyle style}) {
    final resolvedStyle = resolveStyle(style);

    if (resolvedStyle.hasAnyPadding()) {
      return Padding(
        padding: EdgeInsets.only(top: resolvedStyle.paddingTop, bottom: resolvedStyle.paddingBottom, left: resolvedStyle.paddingLeft, right: resolvedStyle.paddingRight),
        child: withFlexible(children),
      );
    }

    return withFlexible(children);
  }

  Widget label(String text, PluginSettingValueStyle style) {
    if (text != "") {
      final resolvedStyle = resolveStyle(style);

      if (resolvedStyle.labelWidth > 0) {
        return Padding(
          padding: const EdgeInsets.only(right: 4),
          child: SizedBox(
            width: resolvedStyle.labelWidth,
            child: Text(text, style: TextStyle(overflow: TextOverflow.ellipsis, color: getThemeTextColor(), fontSize: 13), textAlign: TextAlign.right),
          ),
        );
      } else {
        return Padding(padding: const EdgeInsets.only(right: 4), child: Text(text, style: TextStyle(color: getThemeTextColor(), fontSize: 13)));
      }
    }

    return const SizedBox.shrink();
  }

  Widget suffix(String text) {
    if (text != "") {
      return Padding(padding: const EdgeInsets.only(left: 4), child: Text(text, style: TextStyle(color: getThemeTextColor(), fontSize: 13)));
    }

    return const SizedBox.shrink();
  }
}
