import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_loading_indicator.dart';
import 'package:wox/components/wox_textfield.dart';

import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/utils/colors.dart';

class WoxAIModelSelectorView extends StatefulWidget {
  /// Initial selected model in JSON string format
  final String? initialValue;

  /// Callback when a model is selected
  final Function(String modelJson) onModelSelected;

  /// Callback when the widget resolves an initial/default model selection.
  final Function(String modelJson)? onInitialModelResolved;

  /// Whether to allow editing the model
  final bool allowEdit;

  const WoxAIModelSelectorView({super.key, this.initialValue, required this.onModelSelected, this.onInitialModelResolved, this.allowEdit = true});

  @override
  State<WoxAIModelSelectorView> createState() => _WoxAIModelSelectorViewState();
}

class _WoxAIModelSelectorViewState extends State<WoxAIModelSelectorView> {
  bool _isLoading = true;
  List<AIModel> allModels = [];
  List<AIProviderInfo> allProviders = [];
  Set<String> providerKeys = {};
  String? selectedProviderKey;
  AIModel? selectedModel;
  bool isEditMode = false;
  bool _hasResolvedInitialModel = false;
  Map<String, WoxImage> providerIcons = {};

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
      final resources = await Get.find<WoxSettingController>().loadAIModelSelectorResources();
      allModels = resources.models;
      allProviders = resources.providers;
      providerIcons = {for (final provider in allProviders) provider.name: provider.icon};

      // Extract unique providers
      providerKeys = allModels.map((e) => makeProviderKey(e.provider, e.providerAlias)).toSet();

      // Set initial selection if provided
      if (widget.initialValue != null && widget.initialValue!.isNotEmpty) {
        try {
          final modelJson = jsonDecode(widget.initialValue!);
          selectedModel = AIModel.fromJson(modelJson);
          selectedProviderKey = makeProviderKey(selectedModel?.provider ?? "", selectedModel?.providerAlias ?? "");
          // Keep the persisted provider visible even when the live model endpoint omits it.
          // Previously the selector fell back to another provider, so reopening a saved
          // plugin setting could look like the model choice had not been saved.
          final persistedProviderKey = selectedProviderKey;
          if (persistedProviderKey != null && persistedProviderKey.isNotEmpty) {
            providerKeys.add(persistedProviderKey);
          }
        } catch (e) {
          // Invalid JSON, ignore
        }
      }

      // When no initial value was provided, leave the selection empty so the
      // dropdowns show a "not selected" state instead of silently picking the
      // first available provider/model. The user must explicitly choose a model
      // for the setting to be persisted.

      // If the selected provider key isn't available (e.g. old saved model without alias),
      // try to fall back to any available config for the same provider.
      if (selectedProviderKey != null && !providerKeys.contains(selectedProviderKey)) {
        final providerInfo = parseProviderKey(selectedProviderKey!);
        final provider = providerInfo.$1;
        final candidates = providerKeys.where((k) => parseProviderKey(k).$1 == provider).toList()..sort();
        if (candidates.isNotEmpty) {
          selectedProviderKey = candidates.first;
        } else {
          selectedProviderKey = null;
          selectedModel = null;
        }
      }

      // If the initial value model is not in the live list, keep it selected.
      final initialModel = selectedModel;
      if (initialModel != null && !allModels.any((m) => m.name == initialModel.name && m.provider == initialModel.provider && m.providerAlias == initialModel.providerAlias)) {
        // Keep the saved custom or temporarily unavailable model in select mode.
        // The previous edit-mode fallback made the UI show a different provider/model shape
        // on reopen, even though the persisted setting was still correct.
        isEditMode = false;
        nameController.text = initialModel.name;
      }

      _notifyInitialModelResolved();
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

  void _notifyInitialModelResolved() {
    if (_hasResolvedInitialModel || selectedModel == null) {
      return;
    }

    _hasResolvedInitialModel = true;
    widget.onInitialModelResolved?.call(jsonEncode(selectedModel!));
  }

  @override
  Widget build(BuildContext context) {
    if (_isLoading) {
      return Center(child: WoxLoadingIndicator(size: 16, color: getThemeActiveBackgroundColor()));
    }

    // When there are no models/providers available, guide user to AI settings.
    // A persisted model still counts as displayable state because the setting may
    // be valid even while the provider API is temporarily unavailable.
    if (providerKeys.isEmpty || (allModels.isEmpty && selectedModel == null)) {
      final bg = getThemeActiveBackgroundColor().withAlpha(70);
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(color: bg, borderRadius: BorderRadius.circular(6), border: Border.all(color: getThemeActiveBackgroundColor().withAlpha(90))),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            const Padding(padding: EdgeInsets.only(top: 2.0), child: Icon(Icons.info, size: 14)),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(tr('ui_ai_model_selector_no_models_title'), style: TextStyle(color: getThemeTextColor(), fontWeight: FontWeight.w600)),
                  const SizedBox(height: 4),
                  Text(tr('ui_ai_model_selector_no_models_desc'), style: TextStyle(color: getThemeTextColor())),
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
            ),
          ],
        ),
      );
    }

    List<AIModel> getProviderModels() {
      if (selectedProviderKey != null) {
        final models = allModels.where((m) => makeProviderKey(m.provider, m.providerAlias) == selectedProviderKey).toList();
        models.sort((a, b) => a.name.compareTo(b.name));
        final model = selectedModel;
        if (model != null &&
            makeProviderKey(model.provider, model.providerAlias) == selectedProviderKey &&
            !models.any((m) => m.name == model.name && m.provider == model.provider && m.providerAlias == model.providerAlias)) {
          // Preserve the saved model as a real dropdown option when the provider API no
          // longer returns it. This avoids showing the first available model as a false
          // replacement for the persisted value.
          models.insert(0, model);
        }
        return models;
      }

      return <AIModel>[];
    }

    String getProviderLabel(String providerKey) {
      final providerInfo = parseProviderKey(providerKey);
      final provider = providerInfo.$1;
      final alias = providerInfo.$2;
      return alias.isEmpty ? provider : alias;
    }

    WoxImage getProviderIcon(String providerKey) {
      final providerInfo = parseProviderKey(providerKey);
      return providerIcons[providerInfo.$1] ?? WoxImage.empty();
    }

    Widget? buildProviderLeading(String providerKey) {
      final icon = getProviderIcon(providerKey);
      if (icon.imageData.isEmpty) {
        return null;
      }

      return WoxImageView(woxImage: icon, width: 16, height: 16);
    }

    return Row(
      children: [
        // Provider selector
        Expanded(
          flex: 1,
          child: WoxDropdownButton<String>(
            value: selectedProviderKey,
            isExpanded: true,
            hint: Text(tr('ui_ai_model_selector_not_selected'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 14)),
            items:
                providerKeys
                    .map((providerKey) => WoxDropdownItem<String>(value: providerKey, label: getProviderLabel(providerKey), leading: buildProviderLeading(providerKey)))
                    .toList(),
            onChanged: (providerKey) {
              if (providerKey != null) {
                setState(() {
                  selectedProviderKey = providerKey;
                  // Default to first model of the provider
                  final models = getProviderModels();
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
          child:
              isEditMode
                  ? WoxTextField(
                    controller: nameController,
                    hintText: tr('ui_ai_model_selector_model_name'),
                    width: double.infinity, // fill the Expanded width
                    onChanged: (value) {
                      if (value.isNotEmpty && selectedProviderKey != null) {
                        final providerInfo = parseProviderKey(selectedProviderKey!);
                        final updatedModel = AIModel(name: value, provider: providerInfo.$1, providerAlias: providerInfo.$2);
                        setState(() => selectedModel = updatedModel);
                        widget.onModelSelected(jsonEncode(updatedModel));
                      }
                    },
                  )
                  : WoxDropdownButton<String>(
                    value: selectedModel?.name,
                    isExpanded: true,
                    enableFilter: true,
                    filterHintText: tr('ui_filter_placeholder'),
                    hint: Text(tr('ui_ai_model_selector_not_selected'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 14)),
                    items:
                        getProviderModels()
                            .map(
                              (model) => WoxDropdownItem<String>(
                                value: model.name,
                                label: model.name,
                                leading: buildProviderLeading(makeProviderKey(model.provider, model.providerAlias)),
                              ),
                            )
                            .toList(),
                    onChanged: (modelName) {
                      if (modelName != null && selectedProviderKey != null) {
                        final providerInfo = parseProviderKey(selectedProviderKey!);
                        final selectedModel = allModels.firstWhere(
                          (m) => m.provider == providerInfo.$1 && m.providerAlias == providerInfo.$2 && m.name == modelName,
                          orElse: () => AIModel(name: modelName, provider: providerInfo.$1, providerAlias: providerInfo.$2),
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
        if (widget.allowEdit && selectedModel != null)
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
