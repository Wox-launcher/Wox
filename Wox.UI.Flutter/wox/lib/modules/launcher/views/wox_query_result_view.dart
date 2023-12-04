import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_result_item_view.dart';
import 'package:wox/entity/wox_query.dart';

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
              if (controller.queryResults.isNotEmpty)
                Expanded(
                  child: Container(
                    padding: EdgeInsets.only(
                      top: controller.woxTheme.resultContainerPaddingTop.toDouble(),
                      right: controller.woxTheme.resultContainerPaddingRight.toDouble(),
                      bottom: controller.woxTheme.resultContainerPaddingBottom.toDouble(),
                      left: controller.woxTheme.resultContainerPaddingLeft.toDouble(),
                    ),
                    child: ListView.builder(
                      shrinkWrap: true,
                      physics: const AlwaysScrollableScrollPhysics(),
                      itemCount: controller.queryResults.length,
                      itemExtent: 50,
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
                ),
            ],
          )
        ]),
      );
    });
  }
}
