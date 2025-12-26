import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_tooltip.dart';
import 'package:wox/controllers/wox_setting_controller.dart';

class WoxTooltipIconView extends StatefulWidget {
  final String tooltip;
  final double paddingLeft;
  final double paddingRight;
  final Color color;

  const WoxTooltipIconView({super.key, required this.tooltip, this.paddingLeft = 4.0, this.paddingRight = 4.0, this.color = Colors.black});

  @override
  State<WoxTooltipIconView> createState() => _WoxTooltipIconViewState();
}

class _WoxTooltipIconViewState extends State<WoxTooltipIconView> {
  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(left: widget.paddingLeft, right: widget.paddingRight),
      child: WoxTooltip(
        message: tr(widget.tooltip),
        child: Icon(
          Icons.info,
          size: 13,
          color: widget.color,
        ),
      ),
    );
  }
}
