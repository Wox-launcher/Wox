import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_result_item_view.dart';
import 'package:wox/entity/wox_query_result.dart';

import '../wox_launcher_controller.dart';

class WoxQueryResultView extends GetView<WoxLauncherController> {
  const WoxQueryResultView({super.key});

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      return ConstrainedBox(
        constraints: BoxConstraints(maxHeight: controller.getMaxHeight()),
        child: Stack(fit: (controller.isShowActionPanel.value || controller.isShowPreviewPanel.value) ? StackFit.expand : StackFit.loose, children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: ListView.builder(
                  shrinkWrap: true,
                  physics: const AlwaysScrollableScrollPhysics(),
                  itemCount: controller.queryResults.length,
                  itemExtent: 40,
                  itemBuilder: (context, index) {
                    WoxQueryResult queryResult = controller.getQueryResultByIndex(index);
                    return WoxResultItemView(
                        woxTheme: controller.woxTheme,
                        icon: queryResult.icon,
                        title: queryResult.title,
                        subTitle: queryResult.subTitle,
                        isActive: index == controller.activeResultIndex.value);
                  },
                ),
              ),
            ],
          )
        ]),
      );
    });
  }
}
