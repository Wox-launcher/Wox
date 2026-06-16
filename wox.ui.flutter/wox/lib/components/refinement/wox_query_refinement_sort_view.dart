import 'package:flutter/material.dart';
import 'package:wox/components/wox_dropdown_button.dart';

import 'wox_query_refinement_base_view.dart';

class WoxQueryRefinementSortView extends WoxQueryRefinementBaseView {
  const WoxQueryRefinementSortView({super.key, required super.refinement, required super.selectedValues, required super.onChanged});

  @override
  Widget build(BuildContext context) {
    final currentValue = selectedValues.isNotEmpty ? selectedValues.first : (refinement.options.isNotEmpty ? refinement.options.first.value : null);

    if (refinement.options.length <= chipOptionLimit) {
      return buildShell(
        child: buildChipRow(
          refinement.options.map((option) {
            return buildChip(label: optionLabel(option), leading: optionLeading(option), selected: option.value == currentValue, onTap: () => onChanged([option.value]));
          }).toList(),
        ),
      );
    }

    return buildShell(
      child: WoxDropdownButton<String>(
        value: currentValue,
        width: dropdownWidth,
        isExpanded: true,
        // Bug fix: keep Flutter's default menu row height because DropdownButton
        // asserts on rows below 48px; the compact launcher height is provided by
        // buildShell instead of shrinking the popup menu.
        items: dropdownItems(),
        onChanged: (value) {
          if (value == null) {
            return;
          }
          onChanged([value]);
        },
      ),
    );
  }
}
