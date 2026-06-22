import 'package:flutter/material.dart';
import 'package:wox/components/wox_dropdown_button.dart';

import 'wox_query_refinement_base_view.dart';

class WoxQueryRefinementMultiSelectView extends WoxQueryRefinementBaseView {
  const WoxQueryRefinementMultiSelectView({super.key, required super.refinement, required super.selectedValues, required super.onChanged});

  @override
  Widget build(BuildContext context) {
    if (refinement.options.length <= chipOptionLimit) {
      return buildShell(
        child: buildChipRow(
          refinement.options.map((option) {
            final isSelected = selectedValues.contains(option.value);
            return buildChip(
              label: optionLabel(option),
              leading: optionLeading(option),
              selected: isSelected,
              onTap: () {
                final nextValues = List<String>.from(selectedValues);
                if (isSelected) {
                  nextValues.remove(option.value);
                } else {
                  nextValues.add(option.value);
                }
                onChanged(nextValues);
              },
            );
          }).toList(),
        ),
      );
    }

    return buildShell(
      child: WoxDropdownButton<String>(
        value: null,
        width: dropdownWidth,
        isExpanded: true,
        // Bug fix: multi-select uses a custom compact trigger already, while
        // menu rows should keep the shared dropdown defaults for readable,
        // keyboard-safe choices.
        multiSelect: true,
        multiValues: selectedValues,
        items: dropdownItems(),
        onChanged: (_) {},
        onMultiChanged: onChanged,
      ),
    );
  }
}
