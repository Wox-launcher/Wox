import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/plugin/wox_ai_command_default_action_dropdown.dart';
import 'package:wox/components/wox_ai_model_selector_view.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/components/wox_query_variable_textfield.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_ai_command_template.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/utils/colors.dart';

Future<void> showAICommandTemplateDialog({required BuildContext context, required String pluginId, required List<dynamic> currentRows, required String triggerKeyword}) async {
  await showDialog(
    context: context,
    barrierColor: getThemePopupBarrierColor(),
    builder: (context) => _AICommandTemplateDialog(pluginId: pluginId, currentRows: currentRows, triggerKeyword: triggerKeyword),
  );
}

class _AICommandTemplateDialog extends StatefulWidget {
  final String pluginId;
  final List<dynamic> currentRows;
  final String triggerKeyword;

  const _AICommandTemplateDialog({required this.pluginId, required this.currentRows, required this.triggerKeyword});

  @override
  State<_AICommandTemplateDialog> createState() => _AICommandTemplateDialogState();
}

class _AICommandTemplateDialogState extends State<_AICommandTemplateDialog> {
  final WoxSettingController controller = Get.find<WoxSettingController>();
  final TextEditingController nameController = TextEditingController();
  final TextEditingController commandController = TextEditingController();
  final TextEditingController promptController = TextEditingController();
  final FocusNode promptFocusNode = FocusNode();

  List<AICommandTemplate> templates = [];
  AICommandTemplate? selectedTemplate;
  String selectedModelJson = "";
  bool isLoading = true;
  bool isSaving = false;
  bool createQueryHotkey = false;
  String selectedCategory = "";
  String thinkingMode = AICommandThinkingModeValue.providerDefault;
  String defaultAction = AICommandDefaultActionValue.run;
  String queryHotkey = "";
  String hotkeyAvailabilityError = "";
  String errorMessage = "";

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    nameController.dispose();
    commandController.dispose();
    promptController.dispose();
    promptFocusNode.dispose();
    super.dispose();
  }

  String tr(String key) => controller.tr(key);

  Future<void> _load() async {
    final traceId = const UuidV4().generate();
    try {
      final loadedTemplates = await WoxApi.instance.findAICommandTemplates(traceId);
      String loadedModelJson = "";
      try {
        final loadedDefaultModel = await WoxApi.instance.findDefaultAIModel(traceId);
        if (loadedDefaultModel.name.trim().isNotEmpty && loadedDefaultModel.provider.trim().isNotEmpty) {
          loadedModelJson = json.encode(loadedDefaultModel.toJson());
        }
      } catch (_) {
        loadedModelJson = "";
      }

      if (!mounted) {
        return;
      }

      setState(() {
        templates = loadedTemplates;
        selectedModelJson = loadedModelJson;
        isLoading = false;
      });

      if (loadedTemplates.isNotEmpty) {
        _selectTemplate(loadedTemplates.first);
      }
    } catch (error) {
      if (!mounted) {
        return;
      }

      setState(() {
        isLoading = false;
        errorMessage = tr("ui_ai_command_template_load_failed").replaceAll("{error}", error.toString());
      });
    }
  }

  List<String> get categories {
    final values = templates.map((template) => template.category.trim()).where((category) => category.isNotEmpty).toSet().toList();
    values.sort();
    return values;
  }

  List<AICommandTemplate> get visibleTemplates {
    if (selectedCategory.isEmpty) {
      return templates;
    }
    return templates.where((template) => template.category == selectedCategory).toList();
  }

  void _applyTemplate(AICommandTemplate template) {
    selectedTemplate = template;
    nameController.text = template.name;
    commandController.text = template.command;
    promptController.text = template.prompt;
    thinkingMode = _normalizeThinkingMode(template.thinkingMode);
    defaultAction = _normalizeDefaultAction(template.defaultAction);
    createQueryHotkey = template.recommendedQueryHotkey.hasQuery;
    queryHotkey = template.recommendedQueryHotkey.hotkey;
    hotkeyAvailabilityError = "";
    errorMessage = "";
  }

  void _selectTemplate(AICommandTemplate template) {
    _applyTemplate(template);
    setState(() {});
  }

  void _selectCategory(String category) {
    final nextTemplates = category.isEmpty ? templates : templates.where((template) => template.category == category).toList();
    setState(() {
      selectedCategory = category;
      if (nextTemplates.isNotEmpty && !nextTemplates.any((template) => template.id == selectedTemplate?.id)) {
        _applyTemplate(nextTemplates.first);
      }
    });
  }

  // Keep conflict checks stable when stored settings and recorder output use different platform aliases.
  String _normalizeHotkey(String hotkey) {
    final tokens = hotkey.split("+").map(_normalizeHotkeyToken).where((token) => token.isNotEmpty).toList();
    if (tokens.length == 2 && tokens[0] == tokens[1] && _isHotkeyModifierToken(tokens[0])) {
      return tokens.join("+");
    }

    final modifiers = <String>{};
    var key = "";
    for (final token in tokens) {
      if (token == "capslock") {
        modifiers.add(token);
      } else if (_isHotkeyModifierToken(token)) {
        modifiers.add(token);
      } else if (key.isEmpty) {
        key = token;
      }
    }

    if (modifiers.contains("capslock") && key.isNotEmpty) {
      return "capslock+$key";
    }

    final parts = <String>[];
    for (final modifier in ["ctrl", "shift", "alt", "meta"]) {
      if (modifiers.contains(modifier)) {
        parts.add(modifier);
      }
    }
    if (key.isNotEmpty) {
      parts.add(key);
    }

    return parts.join("+");
  }

  String _normalizeHotkeyToken(String token) {
    switch (token.trim().toLowerCase()) {
      case "":
        return "";
      case "control":
        return "ctrl";
      case "option":
        return "alt";
      case "cmd":
      case "command":
      case "win":
      case "windows":
      case "super":
        return "meta";
      case "caps_lock":
      case "caps lock":
        return "capslock";
      case "return":
        return "enter";
      case "arrowleft":
        return "left";
      case "arrowright":
        return "right";
      case "arrowup":
        return "up";
      case "arrowdown":
        return "down";
      default:
        return token.trim().toLowerCase();
    }
  }

  bool _isHotkeyModifierToken(String token) => token == "ctrl" || token == "shift" || token == "alt" || token == "meta";

  // Keeps template installation compatible with older templates that do not define defaultAction.
  String _normalizeDefaultAction(String value) {
    if (value == AICommandDefaultActionValue.runAndShow || value == AICommandDefaultActionValue.runAndPaste || value == AICommandDefaultActionValue.run) {
      return value;
    }
    return AICommandDefaultActionValue.run;
  }

  String _normalizeThinkingMode(String value) {
    if (value == AICommandThinkingModeValue.thinking || value == AICommandThinkingModeValue.nonThinking || value == AICommandThinkingModeValue.providerDefault) {
      return value;
    }
    return AICommandThinkingModeValue.providerDefault;
  }

  String _internalHotkeyConflict(String hotkey) {
    final normalized = _normalizeHotkey(hotkey);
    if (normalized.isEmpty) {
      return "";
    }

    if (_normalizeHotkey(controller.woxSetting.value.mainHotkey) == normalized) {
      return tr("ui_ai_command_template_hotkey_conflict_main");
    }
    if (_normalizeHotkey(controller.woxSetting.value.selectionHotkey) == normalized) {
      return tr("ui_ai_command_template_hotkey_conflict_selection");
    }

    for (final existing in controller.woxSetting.value.queryHotkeys) {
      if (!existing.disabled && _normalizeHotkey(existing.hotkey) == normalized) {
        return tr("ui_ai_command_template_hotkey_conflict_query").replaceAll("{query}", existing.query);
      }
    }

    return "";
  }

  String _commandConflict(String command, {bool includeEmpty = true}) {
    final normalized = command.trim().toLowerCase();
    if (normalized.isEmpty) {
      return includeEmpty ? tr("ui_ai_command_template_command_empty") : "";
    }

    for (final row in widget.currentRows) {
      if (row is! Map) {
        continue;
      }
      final existing = (row["command"] ?? row["Command"] ?? "").toString().trim().toLowerCase();
      if (existing == normalized) {
        return tr("ui_ai_command_template_command_conflict").replaceAll("{command}", command.trim());
      }
    }

    return "";
  }

  String _currentRecommendedQuery() {
    final triggerKeyword = widget.triggerKeyword.trim().isEmpty ? "ai" : widget.triggerKeyword.trim();
    final currentCommand = commandController.text.trim();
    final queryParts = [triggerKeyword];
    if (currentCommand.isNotEmpty) {
      queryParts.add(currentCommand);
    }
    queryParts.add("{wox:selected_text}");
    return queryParts.join(" ");
  }

  bool _hasInstallBlockingError() {
    final template = selectedTemplate;
    if (template == null || isSaving) {
      return true;
    }

    if (_commandConflict(commandController.text).isNotEmpty) {
      return true;
    }
    if (promptController.text.trim().isEmpty || selectedModelJson.trim().isEmpty) {
      return true;
    }
    if (createQueryHotkey && template.recommendedQueryHotkey.hasQuery) {
      return queryHotkey.trim().isEmpty || _internalHotkeyConflict(queryHotkey).isNotEmpty || hotkeyAvailabilityError.isNotEmpty;
    }

    return false;
  }

  Future<void> _install() async {
    final template = selectedTemplate;
    if (template == null || isSaving) {
      return;
    }

    final commandError = _commandConflict(commandController.text);
    if (commandError.isNotEmpty) {
      setState(() => errorMessage = commandError);
      return;
    }

    if (promptController.text.trim().isEmpty) {
      setState(() => errorMessage = tr("ui_ai_command_template_prompt_empty"));
      return;
    }

    if (selectedModelJson.trim().isEmpty) {
      setState(() => errorMessage = tr("ui_ai_command_template_model_empty"));
      return;
    }

    if (createQueryHotkey && template.recommendedQueryHotkey.hasQuery) {
      if (queryHotkey.trim().isEmpty) {
        setState(() => errorMessage = tr("ui_ai_command_template_hotkey_empty"));
        return;
      }

      final internalConflict = _internalHotkeyConflict(queryHotkey);
      if (internalConflict.isNotEmpty) {
        setState(() => errorMessage = internalConflict);
        return;
      }

      if (hotkeyAvailabilityError.isNotEmpty) {
        setState(() => errorMessage = hotkeyAvailabilityError);
        return;
      }

      final traceId = const UuidV4().generate();
      final available = await WoxApi.instance.isHotkeyAvailable(traceId, queryHotkey);
      if (!available) {
        setState(() => errorMessage = tr("ui_ai_command_template_hotkey_unavailable"));
        return;
      }
    }

    setState(() {
      isSaving = true;
      errorMessage = "";
    });

    final rows = widget.currentRows.map((row) => row is Map ? Map<String, dynamic>.from(row) : row).toList();
    for (final row in rows) {
      if (row is Map<String, dynamic>) {
        row.remove(WoxSettingPluginTableRowKeys.rowUniqueIdKey);
      }
    }

    rows.add({
      "name": nameController.text.trim(),
      "command": commandController.text.trim(),
      "model": selectedModelJson,
      "thinkingMode": thinkingMode,
      "prompt": promptController.text,
      "vision": template.vision,
      "defaultAction": defaultAction,
    });

    final saveError = await controller.updatePluginSetting(widget.pluginId, "commands", json.encode(rows));
    if (saveError != null && saveError.isNotEmpty) {
      setState(() {
        isSaving = false;
        errorMessage = saveError;
      });
      return;
    }

    if (createQueryHotkey && template.recommendedQueryHotkey.hasQuery) {
      final queryHotkeys = controller.woxSetting.value.queryHotkeys.map((queryHotkey) => queryHotkey.toJson()).toList();
      queryHotkeys.add({
        "Hotkey": queryHotkey.trim(),
        "Query": _currentRecommendedQuery(),
        "HideQueryBox": template.recommendedQueryHotkey.hideQueryBox,
        "HideToolbar": template.recommendedQueryHotkey.hideToolbar,
        "IsSilentExecution": true,
        "Width": template.recommendedQueryHotkey.width,
        "MaxResultCount": template.recommendedQueryHotkey.maxResultCount,
        "Position": template.recommendedQueryHotkey.position,
        "Disabled": false,
      });
      await controller.updateConfig("QueryHotkeys", json.encode(queryHotkeys));
    }

    if (mounted) {
      Navigator.pop(context);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      final bool darkTheme = isThemeDark();
      final Color accentColor = getThemeActiveBackgroundColor();
      final Color cardColor = getThemePopupSurfaceColor();
      final Color textColor = getThemeTextColor();
      final Color outlineColor = getThemePopupOutlineColor();
      final bool canInstall = !_hasInstallBlockingError();
      final baseTheme = Theme.of(context);
      final dialogTheme = baseTheme.copyWith(
        colorScheme: ColorScheme.fromSeed(seedColor: accentColor, brightness: darkTheme ? Brightness.dark : Brightness.light),
        scaffoldBackgroundColor: Colors.transparent,
        cardColor: cardColor,
        shadowColor: textColor.withAlpha(50),
      );

      return Theme(
        data: dialogTheme,
        child: AlertDialog(
          backgroundColor: cardColor,
          surfaceTintColor: Colors.transparent,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20), side: BorderSide(color: outlineColor)),
          elevation: 18,
          insetPadding: const EdgeInsets.symmetric(horizontal: 32, vertical: 28),
          contentPadding: const EdgeInsets.fromLTRB(28, 24, 28, 0),
          actionsPadding: const EdgeInsets.fromLTRB(28, 12, 38, 24),
          actionsAlignment: MainAxisAlignment.end,
          content: SizedBox(width: 880, height: 600, child: isLoading ? Center(child: CircularProgressIndicator(color: accentColor)) : _buildContent()),
          actions: [
            WoxButton.secondary(
              text: tr("ui_cancel"),
              padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12),
              onPressed: isSaving ? null : () => Navigator.pop(context),
            ),
            const SizedBox(width: 12),
            WoxButton.primary(
              text: isSaving ? tr("ui_saving") : tr("ui_ai_command_template_install"),
              padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 12),
              onPressed: canInstall ? _install : null,
            ),
          ],
        ),
      );
    });
  }

  Color _subtleSurfaceColor() {
    return Color.alphaBlend(getThemeTextColor().withValues(alpha: isThemeDark() ? 0.05 : 0.03), getThemePopupSurfaceColor());
  }

  Color _softBorderColor() {
    return getThemeSubTextColor().withValues(alpha: isThemeDark() ? 0.24 : 0.18);
  }

  Widget _buildContent() {
    if (errorMessage.isNotEmpty && templates.isEmpty) {
      return Text(errorMessage, style: TextStyle(color: getThemeSubTextColor(), fontSize: 13));
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(tr("ui_ai_command_template_store"), style: TextStyle(color: getThemeTextColor().withValues(alpha: 0.94), fontSize: 18, fontWeight: FontWeight.w700)),
        const SizedBox(height: 16),
        Expanded(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [SizedBox(width: 280, child: _buildTemplateList()), const SizedBox(width: 26), Expanded(child: _buildTemplateEditor())],
          ),
        ),
      ],
    );
  }

  Widget _buildTemplateList() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          height: 32,
          child: SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(children: [_buildCategoryChip("", tr("ui_all")), for (final category in categories) _buildCategoryChip(category, category)]),
          ),
        ),
        const SizedBox(height: 12),
        Expanded(
          child: ListView(
            children: [
              for (final template in visibleTemplates) _TemplateListItem(template: template, selected: selectedTemplate?.id == template.id, onTap: () => _selectTemplate(template)),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildCategoryChip(String category, String label) {
    final selected = selectedCategory == category;
    return GestureDetector(
      onTap: () => _selectCategory(category),
      child: Container(
        margin: const EdgeInsets.only(right: 8),
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
        decoration: BoxDecoration(
          color: selected ? getThemeActiveBackgroundColor().withValues(alpha: 0.18) : _subtleSurfaceColor(),
          border: Border.all(color: selected ? getThemeActiveBackgroundColor().withValues(alpha: 0.7) : _softBorderColor()),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Text(label, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: selected ? getThemeTextColor() : getThemeSubTextColor(), fontSize: 12)),
      ),
    );
  }

  Widget _buildTemplateEditor() {
    final template = selectedTemplate;
    if (template == null) {
      return Text(tr("ui_ai_command_template_empty"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13));
    }

    final hotkeyConflict = _internalHotkeyConflict(queryHotkey);
    final commandError = _commandConflict(commandController.text);
    final promptError = promptController.text.trim().isEmpty ? tr("ui_ai_command_template_prompt_empty") : "";

    return SingleChildScrollView(
      padding: const EdgeInsets.only(right: 10),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: 1,
                child: _buildField(tr("plugin_ai_command_name"), WoxTextField(width: double.infinity, controller: nameController, onChanged: (_) => setState(() {}))),
              ),
              const SizedBox(width: 8),
              Expanded(
                flex: 2,
                child: _buildField(
                  tr("plugin_ai_command_command"),
                  WoxTextField(
                    width: double.infinity,
                    controller: commandController,
                    onChanged: (_) {
                      setState(() => errorMessage = "");
                    },
                  ),
                  errorText: commandError,
                ),
              ),
            ],
          ),
          _buildField(
            tr("plugin_ai_command_model"),
            WoxAIModelSelectorView(
              initialValue: selectedModelJson,
              onInitialModelResolved: (modelJson) {
                if (!mounted || selectedModelJson == modelJson) {
                  return;
                }

                setState(() => selectedModelJson = modelJson);
              },
              onModelSelected: (modelJson) {
                if (!mounted) {
                  return;
                }

                setState(() => selectedModelJson = modelJson);
              },
            ),
          ),
          _buildField(
            tr("plugin_ai_command_thinking_mode"),
            WoxDropdownButton<String>(
              width: double.infinity,
              value: thinkingMode,
              isExpanded: true,
              onChanged: (value) {
                setState(() {
                  thinkingMode = _normalizeThinkingMode(value ?? "");
                  errorMessage = "";
                });
              },
              items: [
                WoxDropdownItem(value: AICommandThinkingModeValue.providerDefault, label: tr("plugin_ai_command_thinking_mode_provider_default")),
                WoxDropdownItem(value: AICommandThinkingModeValue.thinking, label: tr("plugin_ai_command_thinking_mode_thinking")),
                WoxDropdownItem(value: AICommandThinkingModeValue.nonThinking, label: tr("plugin_ai_command_thinking_mode_non_thinking")),
              ],
            ),
          ),
          _buildField(
            tr("plugin_ai_command_default_action"),
            WoxAICommandDefaultActionDropdown(
              width: double.infinity,
              value: defaultAction,
              onChanged: (value) {
                setState(() {
                  defaultAction = value;
                  errorMessage = "";
                });
              },
            ),
          ),
          _buildField(
            tr("plugin_ai_command_prompt"),
            WoxQueryVariableTextField(
              width: double.infinity,
              controller: promptController,
              focusNode: promptFocusNode,
              maxLines: 7,
              source: WoxQueryVariableSource.aiCommand,
              onChanged: (_) {
                setState(() => errorMessage = "");
              },
            ),
            errorText: promptError,
          ),
          if (template.recommendedQueryHotkey.hasQuery) _buildQueryHotkeySection(template, hotkeyConflict),
          if (errorMessage.isNotEmpty) Padding(padding: const EdgeInsets.only(top: 14), child: _buildErrorBanner(errorMessage)),
        ],
      ),
    );
  }

  Widget _buildField(String label, Widget child, {String errorText = ""}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: TextStyle(color: getThemeTextColor().withValues(alpha: 0.92), fontSize: 12.5, fontWeight: FontWeight.w600)),
          const SizedBox(height: 7),
          child,
          if (errorText.isNotEmpty) Padding(padding: const EdgeInsets.only(top: 6), child: Text(errorText, style: const TextStyle(color: Colors.red, fontSize: 12))),
        ],
      ),
    );
  }

  Widget _buildErrorBanner(String message) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(color: Colors.red.withValues(alpha: 0.08), border: Border.all(color: Colors.red.withValues(alpha: 0.28)), borderRadius: BorderRadius.circular(6)),
      child: Text(message, style: const TextStyle(color: Colors.red, fontSize: 12)),
    );
  }

  Widget _buildQueryHotkeySection(AICommandTemplate template, String hotkeyConflict) {
    final recommendedQuery = _currentRecommendedQuery();
    final missingHotkey = createQueryHotkey && queryHotkey.trim().isEmpty;
    final hasConflict = createQueryHotkey && hotkeyConflict.isNotEmpty;
    final hasUnavailable = createQueryHotkey && hotkeyConflict.isEmpty && hotkeyAvailabilityError.isNotEmpty;
    final hasHotkeyError = hasConflict || hasUnavailable;
    final hasBlockingIssue = missingHotkey || hasHotkeyError;
    final hotkeyErrorText =
        !createQueryHotkey
            ? ""
            : hasConflict
            ? hotkeyConflict
            : hasUnavailable
            ? hotkeyAvailabilityError
            : missingHotkey
            ? tr("ui_ai_command_template_hotkey_empty")
            : "";

    return Padding(
      padding: const EdgeInsets.only(top: 2),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: _subtleSurfaceColor(),
          border: Border.all(color: hasBlockingIssue ? Colors.red.withValues(alpha: 0.34) : _softBorderColor()),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            GestureDetector(
              onTap: () => setState(() => createQueryHotkey = !createQueryHotkey),
              child: Row(
                children: [
                  WoxCheckbox(value: createQueryHotkey, onChanged: (value) => setState(() => createQueryHotkey = value ?? false), size: 18),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(tr("ui_ai_command_template_create_query_hotkey"), style: TextStyle(color: getThemeTextColor(), fontSize: 12.5, fontWeight: FontWeight.w700)),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
            Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                SizedBox(width: 118, child: Text(tr("ui_ai_command_template_recommended_hotkey"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      WoxHotkeyRecorder(
                        hotkey: WoxHotkey.parseHotkeyFromString(queryHotkey),
                        tipPosition: WoxHotkeyRecorderTipPosition.right,
                        onHotKeyRecorded: (hotkey) {
                          setState(() {
                            queryHotkey = hotkey;
                            hotkeyAvailabilityError = "";
                            errorMessage = "";
                          });
                        },
                        onUnavailableHotKeyRecorded: (hotkey) {
                          setState(() {
                            queryHotkey = hotkey;
                            hotkeyAvailabilityError = tr("ui_ai_command_template_hotkey_unavailable");
                            errorMessage = "";
                          });
                        },
                        recordUnavailableHotkey: true,
                      ),
                      if (hotkeyErrorText.isNotEmpty)
                        Padding(
                          padding: const EdgeInsets.only(top: 6),
                          child: Text(hotkeyErrorText, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(color: Colors.red, fontSize: 12)),
                        ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(width: 118, child: Text(tr("ui_ai_command_template_recommended_query"), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))),
                Expanded(
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                    decoration: BoxDecoration(border: Border.all(color: _softBorderColor()), borderRadius: BorderRadius.circular(4)),
                    child: Text(
                      recommendedQuery,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(color: getThemeTextColor().withValues(alpha: 0.88), fontSize: 12, height: 1.25),
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _TemplateListItem extends StatelessWidget {
  final AICommandTemplate template;
  final bool selected;
  final VoidCallback onTap;

  const _TemplateListItem({required this.template, required this.selected, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
        decoration: BoxDecoration(
          color:
              selected
                  ? getThemeActiveBackgroundColor().withValues(alpha: 0.14)
                  : Color.alphaBlend(getThemeTextColor().withValues(alpha: isThemeDark() ? 0.04 : 0.025), getThemePopupSurfaceColor()),
          border: Border.all(color: selected ? getThemeActiveBackgroundColor().withValues(alpha: 0.55) : getThemeSubTextColor().withValues(alpha: isThemeDark() ? 0.2 : 0.14)),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(template.name, maxLines: 1, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w700)),
                ),
                if (selected) Container(width: 6, height: 6, decoration: BoxDecoration(color: getThemeActiveBackgroundColor(), borderRadius: BorderRadius.circular(999))),
              ],
            ),
            if (template.description.trim().isNotEmpty) ...[
              const SizedBox(height: 5),
              Text(template.description, maxLines: 2, overflow: TextOverflow.ellipsis, style: TextStyle(color: getThemeSubTextColor(), fontSize: 11.5, height: 1.25)),
            ],
            const SizedBox(height: 8),
            Text(
              template.command,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: getThemeSubTextColor().withValues(alpha: 0.82), fontSize: 11.5, fontWeight: FontWeight.w600),
            ),
          ],
        ),
      ),
    );
  }
}

class WoxSettingPluginTableRowKeys {
  static const String rowUniqueIdKey = "wox_table_row_id";
}
