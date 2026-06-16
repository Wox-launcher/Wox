import 'package:flutter/material.dart';
import 'wox_query_refinement_base_view.dart';

class WoxQueryRefinementToggleView extends WoxQueryRefinementBaseView {
  const WoxQueryRefinementToggleView({super.key, required super.refinement, required super.selectedValues, required super.onChanged});

  String get enabledValue {
    if (refinement.options.isNotEmpty && refinement.options.first.value.isNotEmpty) {
      return refinement.options.first.value;
    }
    return "true";
  }

  @override
  Widget build(BuildContext context) {
    final isEnabled = selectedValues.contains(enabledValue);
    final toggleLabel = refinement.options.isNotEmpty ? optionLabel(refinement.options.first) : tr(refinement.title);

    return buildShell(
      child: buildChip(
        label: toggleLabel,
        selected: isEnabled,
        leading: refinement.options.isNotEmpty ? optionLeading(refinement.options.first) : null,
        onTap: () => onChanged(isEnabled ? const <String>[] : [enabledValue]),
      ),
    );
  }
}
