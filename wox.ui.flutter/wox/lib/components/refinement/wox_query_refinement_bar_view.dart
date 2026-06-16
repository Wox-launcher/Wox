import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/utils/wox_interface_size_util.dart';

import 'wox_query_refinement_multi_select_view.dart';
import 'wox_query_refinement_single_select_view.dart';
import 'wox_query_refinement_sort_view.dart';
import 'wox_query_refinement_toggle_view.dart';

class WoxQueryRefinementBarView extends GetView<WoxLauncherController> {
  const WoxQueryRefinementBarView({super.key});

  Widget _buildRefinement(WoxQueryRefinement refinement) {
    final selectedValues = controller.getQueryRefinementSelectedValues(refinement.id);
    void handleChanged(List<String> values) {
      controller.updateQueryRefinementSelection(const UuidV4().generate(), refinement, values);
    }

    switch (refinement.type) {
      case "singleSelect":
        return WoxQueryRefinementSingleSelectView(refinement: refinement, selectedValues: selectedValues, onChanged: handleChanged);
      case "multiSelect":
        return WoxQueryRefinementMultiSelectView(refinement: refinement, selectedValues: selectedValues, onChanged: handleChanged);
      case "toggle":
        return WoxQueryRefinementToggleView(refinement: refinement, selectedValues: selectedValues, onChanged: handleChanged);
      case "sort":
        return WoxQueryRefinementSortView(refinement: refinement, selectedValues: selectedValues, onChanged: handleChanged);
      default:
        return const SizedBox.shrink();
    }
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      if (!controller.shouldShowQueryRefinements) {
        return const SizedBox.shrink();
      }

      final metrics = WoxInterfaceSizeUtil.instance.current;
      return SizedBox(
        height: controller.getQueryRefinementBarHeight(),
        child: Padding(
          padding: EdgeInsets.only(left: metrics.scaledSpacing(8), right: metrics.scaledSpacing(8), top: metrics.scaledSpacing(10), bottom: metrics.scaledSpacing(8)),
          child: ClipRect(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  // Feature addition: keep each refinement as a compact control
                  // in one horizontal strip so plugins can add filters without
                  // changing the result-list layout contract.
                  for (final refinement in controller.queryRefinements) _buildRefinement(refinement),
                ],
              ),
            ),
          ),
        ),
      );
    });
  }
}
