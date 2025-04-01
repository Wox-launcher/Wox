import 'dart:convert';

import 'package:fluent_ui/fluent_ui.dart';
import 'package:get/get.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';

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
      return const Center(child: ProgressRing(strokeWidth: 2));
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
          child: ComboBox<String>(
            value: _selectedProvider,
            items: _providers
                .map((provider) => ComboBoxItem<String>(
                      value: provider,
                      child: Text(provider),
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
              ? TextBox(
                  controller: _nameController,
                  placeholder: tr('ui_ai_model_selector_model_name'),
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
              : ComboBox<String>(
                  value: _selectedModel?.name,
                  items: getProviderModels()
                      .map((model) => ComboBoxItem<String>(
                            value: model.name,
                            child: Text(model.name),
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
            icon: Icon(_isEditMode ? FluentIcons.bulleted_list : FluentIcons.edit),
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
