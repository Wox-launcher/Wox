import 'package:get/get.dart';
import 'package:flutter/material.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/colors.dart';

abstract class WoxSettingBaseView extends GetView<WoxSettingController> {
  const WoxSettingBaseView({super.key});

  Widget form({double width = 960, required List<Widget> children}) {
    return Align(
      alignment: Alignment.topLeft,
      child: SingleChildScrollView(
        child: Padding(
          padding: const EdgeInsets.only(left: 20, right: 40, bottom: 20, top: 20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              ...children.map((e) => SizedBox(
                    width: width,
                    child: e,
                  )),
            ],
          ),
        ),
      ),
    );
  }

  Widget formField({required String label, required Widget child, String? tips, Widget? customTips, double labelWidth = 160}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 20),
      child: Column(
        children: [
          Row(
            children: [
              Padding(
                padding: const EdgeInsets.only(right: 20),
                child: SizedBox(
                  width: labelWidth,
                  child: Text(
                    label,
                    textAlign: TextAlign.right,
                    style: TextStyle(
                      color: getThemeTextColor(),
                      fontSize: 13,
                    ),
                  ),
                ),
              ),
              Flexible(
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: child,
                ),
              ),
            ],
          ),
          if (tips != null || customTips != null)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Padding(
                    padding: const EdgeInsets.only(right: 20),
                    child: SizedBox(width: labelWidth, child: const Text("")),
                  ),
                  Flexible(
                    child: customTips ??
                        Text(
                          tips!,
                          style: TextStyle(color: getThemeSubTextColor(), fontSize: 13),
                        ),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
