import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
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
  List<AIModel> _allModels = [];
  Set<String> _providers = {};
  String? _selectedProvider;
  AIModel? _selectedModel;
  bool _isEditMode = false;

  // For custom model editing
  final TextEditingController _nameController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _loadModels();
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  Future<void> _loadModels() async {
    if (!mounted) return;
    setState(() => _isLoading = true);

    try {
      _allModels = await WoxApi.instance.findAIModels();

      // Extract unique providers
      _providers = _allModels.map((e) => e.provider).toSet();

      // Set initial selection if provided
      if (widget.initialValue != null && widget.initialValue!.isNotEmpty) {
        try {
          final modelJson = jsonDecode(widget.initialValue!);
          _selectedModel = AIModel.fromJson(modelJson);
          _selectedProvider = _selectedModel?.provider;
        } catch (e) {
          // Invalid JSON, ignore
        }
      }

      // Default to first provider if none selected
      if (_selectedProvider == null && _providers.isNotEmpty) {
        _selectedProvider = _providers.first;
      }

      // Default to first model of selected provider if none selected
      if (_selectedModel == null && _selectedProvider != null) {
        final providerModels = _allModels.where((m) => m.provider == _selectedProvider).toList();
        if (providerModels.isNotEmpty) {
          _selectedModel = providerModels.first;
        }
      }

      // if the initial value model is not in the list, switch to the edit mode
      if (_selectedModel != null && !_allModels.any((m) => m.name == _selectedModel!.name)) {
        _isEditMode = true;
        _nameController.text = _selectedModel!.name;
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
    if (_providers.isEmpty || _allModels.isEmpty) {
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
            TextButton(
              child: Text(
                tr('ui_ai_model_selector_open_ai_settings'),
                style: TextStyle(color: getThemeTextColor(), fontWeight: FontWeight.w600),
              ),
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
      if (_selectedProvider != null) {
        final models = _allModels.where((m) => m.provider == _selectedProvider).toList();
        models.sort((a, b) => a.name.compareTo(b.name));
        return models;
      }

      return <AIModel>[];
    }

    return Row(
      children: [
        // Provider selector
        Expanded(
          flex: 1,
          child: DropdownButton<String>(
            value: _selectedProvider,
            isExpanded: true,
            dropdownColor: getThemeCardBackgroundColor(),
            style: TextStyle(color: getThemeTextColor(), fontSize: 13),
            items: _providers
                .map((provider) => DropdownMenuItem<String>(
                      value: provider,
                      child: SizedBox(
                        width: 100, // Limit width to prevent overflow
                        child: Text(
                          provider,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ))
                .toList(),
            onChanged: (provider) {
              if (provider != null) {
                setState(() {
                  _selectedProvider = provider;
                  // Default to first model of the provider
                  final models = _allModels.where((m) => m.provider == provider).toList();
                  if (models.isNotEmpty) {
                    _selectedModel = models.first;
                    _nameController.text = models.first.name;
                    widget.onModelSelected(jsonEncode(_selectedModel!));
                  } else {
                    _selectedModel = null;
                  }
                  _isEditMode = false;
                });
              }
            },
          ),
        ),

        const SizedBox(width: 8),

        // Model selector or editor
        Expanded(
          flex: 2,
          child: _isEditMode
              ? TextField(
                  controller: _nameController,
                  style: TextStyle(color: getThemeTextColor(), fontSize: 13),
                  decoration: InputDecoration(
                    hintText: tr('ui_ai_model_selector_model_name'),
                    hintStyle: TextStyle(color: getThemeSubTextColor()),
                    enabledBorder: UnderlineInputBorder(
                      borderSide: BorderSide(color: getThemeTextColor().withOpacity(0.3)),
                    ),
                    focusedBorder: UnderlineInputBorder(
                      borderSide: BorderSide(color: getThemeActiveBackgroundColor(), width: 2),
                    ),
                  ),
                  onChanged: (value) {
                    if (value.isNotEmpty && _selectedProvider != null) {
                      final updatedModel = AIModel(
                        provider: _selectedProvider!,
                        name: value,
                      );
                      setState(() => _selectedModel = updatedModel);
                      widget.onModelSelected(jsonEncode(updatedModel));
                    }
                  },
                )
              : DropdownButton<String>(
                  value: _selectedModel?.name,
                  isExpanded: true,
                  dropdownColor: getThemeCardBackgroundColor(),
                  style: TextStyle(color: getThemeTextColor(), fontSize: 13),
                  items: getProviderModels()
                      .map((model) => DropdownMenuItem<String>(
                            value: model.name,
                            child: SizedBox(
                              width: 200, // Limit width to prevent overflow
                              child: Text(
                                model.name,
                                overflow: TextOverflow.ellipsis,
                              ),
                            ),
                          ))
                      .toList(),
                  onChanged: (modelName) {
                    if (modelName != null && _selectedProvider != null) {
                      final selectedModel = _allModels.firstWhere(
                        (m) => m.provider == _selectedProvider && m.name == modelName,
                        orElse: () => AIModel(
                          provider: _selectedProvider!,
                          name: modelName,
                        ),
                      );

                      setState(() {
                        _selectedModel = selectedModel;
                        _nameController.text = selectedModel.name;
                      });
                      widget.onModelSelected(jsonEncode(selectedModel));
                    }
                  },
                ),
        ),

        // Toggle edit/select mode button
        if (widget.allowEdit && _selectedModel != null)
          IconButton(
            icon: Icon(_isEditMode ? Icons.list : Icons.edit, color: getThemeTextColor()),
            onPressed: () {
              setState(() {
                _isEditMode = !_isEditMode;
                if (_isEditMode) {
                  _nameController.text = _selectedModel?.name ?? '';
                } else {
                  // select the model by name or first model if not found
                  if (_selectedProvider != null) {
                    final models = _allModels.where((m) => m.provider == _selectedProvider).toList();
                    if (models.isNotEmpty) {
                      _selectedModel = models.firstWhere(
                        (m) => m.name == _nameController.text,
                        orElse: () => models.first,
                      );
                      _nameController.text = _selectedModel?.name ?? '';
                      widget.onModelSelected(jsonEncode(_selectedModel!));
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
