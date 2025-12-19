import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_textfield.dart';

import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/utils/colors.dart';

class WoxAIModelSelectorView extends StatefulWidget {
  /// Initial selected model in JSON string format
  final String? initialValue;

  /// Callback when a model is selected
  final Function(String modelJson) onModelSelected;

  /// Whether to allow editing the model
  final bool allowEdit;

  const WoxAIModelSelectorView({
    super.key,
    this.initialValue,
    required this.onModelSelected,
    this.allowEdit = true,
  });

  @override
  State<WoxAIModelSelectorView> createState() => _WoxAIModelSelectorViewState();
}

class _WoxAIModelSelectorViewState extends State<WoxAIModelSelectorView> {
  bool _isLoading = true;
  List<AIModel> allModels = [];
  Set<String> providerKeys = {};
  String? selectedProviderKey;
  AIModel? selectedModel;
  bool isEditMode = false;

  // For custom model editing
  final TextEditingController nameController = TextEditingController();

  String makeProviderKey(String provider, String alias) {
    return alias.isEmpty ? provider : "${provider}_$alias";
  }

  (String provider, String alias) parseProviderKey(String providerKey) {
    final idx = providerKey.indexOf("_");
    if (idx <= 0) return (providerKey, "");
    return (providerKey.substring(0, idx), providerKey.substring(idx + 1));
  }

  @override
  void initState() {
    super.initState();
    _loadModels();
  }

  @override
  void dispose() {
    nameController.dispose();
    super.dispose();
  }

  Future<void> _loadModels() async {
    if (!mounted) return;
    setState(() => _isLoading = true);

    try {
      allModels = await WoxApi.instance.findAIModels();

      // Extract unique providers
      providerKeys = allModels.map((e) => makeProviderKey(e.provider, e.providerAlias)).toSet();

      // Set initial selection if provided
      if (widget.initialValue != null && widget.initialValue!.isNotEmpty) {
        try {
          final modelJson = jsonDecode(widget.initialValue!);
          selectedModel = AIModel.fromJson(modelJson);
          selectedProviderKey = makeProviderKey(selectedModel?.provider ?? "", selectedModel?.providerAlias ?? "");
        } catch (e) {
          // Invalid JSON, ignore
        }
      }

      // Default to first provider if none selected
      if (selectedProviderKey == null && providerKeys.isNotEmpty) {
        selectedProviderKey = providerKeys.first;
      }

      // If the selected provider key isn't available (e.g. old saved model without alias),
      // try to fall back to any available config for the same provider.
      if (selectedProviderKey != null && !providerKeys.contains(selectedProviderKey)) {
        final providerInfo = parseProviderKey(selectedProviderKey!);
        final provider = providerInfo.$1;
        final candidates = providerKeys.where((k) => parseProviderKey(k).$1 == provider).toList()..sort();
        if (candidates.isNotEmpty) {
          selectedProviderKey = candidates.first;
        } else if (providerKeys.isNotEmpty) {
          selectedProviderKey = providerKeys.first;
        }
      }

      // Default to first model of selected provider if none selected
      if (selectedModel == null && selectedProviderKey != null) {
        final providerModels = allModels.where((m) => makeProviderKey(m.provider, m.providerAlias) == selectedProviderKey).toList();
        if (providerModels.isNotEmpty) {
          selectedModel = providerModels.first;
        }
      }

      // if the initial value model is not in the list, switch to the edit mode
      if (selectedModel != null &&
          !allModels.any((m) => m.name == selectedModel!.name && m.provider == selectedModel!.provider && m.providerAlias == selectedModel!.providerAlias)) {
        isEditMode = true;
        nameController.text = selectedModel!.name;
      }
    } catch (e) {
      // Handle error
    } finally {
      if (mounted) {
        setState(() => _isLoading = false);
      }
    }
  }

  String tr(String key) {
    return Get.find<WoxSettingController>().tr(key);
  }

  @override
  Widget build(BuildContext context) {
    if (_isLoading) {
      return Center(child: CircularProgressIndicator(strokeWidth: 2, color: getThemeActiveBackgroundColor()));
    }

    // When there are no models/providers available, guide user to AI settings
    if (providerKeys.isEmpty || allModels.isEmpty) {
      final bg = getThemeActiveBackgroundColor().withAlpha(70);
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: bg,
          borderRadius: BorderRadius.circular(6),
          border: Border.all(color: getThemeActiveBackgroundColor().withAlpha(90)),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            const Padding(
              padding: EdgeInsets.only(top: 2.0),
              child: Icon(Icons.info, size: 14),
            ),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    tr('ui_ai_model_selector_no_models_title'),
                    style: TextStyle(color: getThemeTextColor(), fontWeight: FontWeight.w600),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    tr('ui_ai_model_selector_no_models_desc'),
                    style: TextStyle(color: getThemeTextColor()),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            WoxButton.text(
              text: tr('ui_ai_model_selector_open_ai_settings'),
              onPressed: () {
                // Switch to the AI settings page within the settings view
                Get.find<WoxSettingController>().activeNavPath.value = 'ai';
              },
            )
          ],
        ),
      );
    }

    List<AIModel> getProviderModels() {
      if (selectedProviderKey != null) {
        final models = allModels.where((m) => makeProviderKey(m.provider, m.providerAlias) == selectedProviderKey).toList();
        models.sort((a, b) => a.name.compareTo(b.name));
        return models;
      }

      return <AIModel>[];
    }

    String getProviderLabel(String providerKey) {
      final providerInfo = parseProviderKey(providerKey);
      final provider = providerInfo.$1;
      final alias = providerInfo.$2;
      return alias.isEmpty ? provider : "$provider ($alias)";
    }

    return Row(
      children: [
        // Provider selector
        Expanded(
          flex: 1,
          child: WoxDropdownButton<String>(
            value: selectedProviderKey,
            isExpanded: true,
            items: providerKeys
                .map((providerKey) => WoxDropdownItem<String>(
                      value: providerKey,
                      label: getProviderLabel(providerKey),
                    ))
                .toList(),
            onChanged: (providerKey) {
              if (providerKey != null) {
                setState(() {
                  selectedProviderKey = providerKey;
                  // Default to first model of the provider
                  final models = allModels.where((m) => makeProviderKey(m.provider, m.providerAlias) == providerKey).toList();
                  if (models.isNotEmpty) {
                    selectedModel = models.first;
                    nameController.text = models.first.name;
                    widget.onModelSelected(jsonEncode(selectedModel!));
                  } else {
                    selectedModel = null;
                  }
                  isEditMode = false;
                });
              }
            },
          ),
        ),

        const SizedBox(width: 8),

        // Model selector or editor
        Expanded(
          flex: 2,
          child: isEditMode
              ? WoxTextField(
                  controller: nameController,
                  hintText: tr('ui_ai_model_selector_model_name'),
                  width: double.infinity, // fill the Expanded width
                  onChanged: (value) {
                    if (value.isNotEmpty && selectedProviderKey != null) {
                      final providerInfo = parseProviderKey(selectedProviderKey!);
                      final updatedModel = AIModel(
                        name: value,
                        provider: providerInfo.$1,
                        providerAlias: providerInfo.$2,
                      );
                      setState(() => this.selectedModel = updatedModel);
                      widget.onModelSelected(jsonEncode(updatedModel));
                    }
                  },
                )
              : WoxDropdownButton<String>(
                  value: selectedModel?.name,
                  isExpanded: true,
                  enableFilter: true,
                  items: getProviderModels()
                      .map((model) => WoxDropdownItem<String>(
                            value: model.name,
                            label: model.name,
                          ))
                      .toList(),
                  onChanged: (modelName) {
                    if (modelName != null && selectedProviderKey != null) {
                      final providerInfo = parseProviderKey(selectedProviderKey!);
                      final selectedModel = allModels.firstWhere(
                        (m) => m.provider == providerInfo.$1 && m.providerAlias == providerInfo.$2 && m.name == modelName,
                        orElse: () => AIModel(
                          name: modelName,
                          provider: providerInfo.$1,
                          providerAlias: providerInfo.$2,
                        ),
                      );

                      setState(() {
                        this.selectedModel = selectedModel;
                        nameController.text = selectedModel.name;
                      });
                      widget.onModelSelected(jsonEncode(selectedModel));
                    }
                  },
                ),
        ),

        // Toggle edit/select mode button
        if (widget.allowEdit && this.selectedModel != null)
          IconButton(
            icon: Icon(isEditMode ? Icons.list : Icons.edit, color: getThemeTextColor()),
            onPressed: () {
              setState(() {
                isEditMode = !isEditMode;
                if (isEditMode) {
                  nameController.text = selectedModel?.name ?? '';
                } else {
                  // select the model by name or first model if not found
                  if (selectedProviderKey != null) {
                    final providerInfo = parseProviderKey(selectedProviderKey!);
                    final models = allModels.where((m) => m.provider == providerInfo.$1 && m.providerAlias == providerInfo.$2).toList();
                    if (models.isNotEmpty) {
                      selectedModel = models.firstWhere(
                        (m) => m.name == nameController.text && m.provider == providerInfo.$1 && m.providerAlias == providerInfo.$2,
                        orElse: () => models.first,
                      );
                      nameController.text = selectedModel?.name ?? '';
                      widget.onModelSelected(jsonEncode(selectedModel!));
                    }
                  }
                }
              });
            },
          ),
      ],
    );
  }
}
