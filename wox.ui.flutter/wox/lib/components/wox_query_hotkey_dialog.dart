import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_checkbox.dart';
import 'package:wox/components/demo/wox_demo.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_dropdown_button.dart';
import 'package:wox/components/wox_hotkey_recorder_view.dart';
import 'package:wox/components/wox_markdown.dart';
import 'package:wox/components/wox_query_variable_textfield.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_hotkey.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/wox_dialog_util.dart';

enum _QueryHotkeyPreset { normal, webPanel, silent, custom }

Future<void> showWoxQueryHotkeyDialog({
  required BuildContext context,
  Map<String, dynamic> initialRow = const {},
  bool isEditing = false,
  required Future<String?> Function(Map<String, dynamic> row) onSave,
}) async {
  await showWoxDialog(
    context: context,
    barrierColor: getThemePopupBarrierColor(),
    builder: (context) => WoxQueryHotkeyDialog(initialRow: initialRow, isEditing: isEditing, onSave: onSave),
  );
}

class WoxQueryHotkeyDialog extends StatefulWidget {
  final Map<String, dynamic> initialRow;
  final bool isEditing;
  final Future<String?> Function(Map<String, dynamic> row) onSave;

  const WoxQueryHotkeyDialog({super.key, this.initialRow = const {}, this.isEditing = false, required this.onSave});

  @override
  State<WoxQueryHotkeyDialog> createState() => _WoxQueryHotkeyDialogState();
}

class _WoxQueryHotkeyDialogState extends State<WoxQueryHotkeyDialog> {
  static const double _dialogContentWidth = 700;
  static const double _dialogContentHeight = 520;
  static const double _dialogScrollGutter = 18;

  late final WoxSettingController controller;
  late final QueryHotkey _initialSnapshot;
  late final QueryHotkey _draft;
  late final TextEditingController _nameController;
  late final TextEditingController _queryController;
  late final TextEditingController _widthController;
  late final TextEditingController _maxResultCountController;
  late final FocusNode _queryFocusNode;
  late _QueryHotkeyPreset _selectedPreset;

  bool _isSaving = false;
  String _saveErrorMessage = "";
  String _hotkeyAvailabilityError = "";
  int _hotkeyValidationToken = 0;
  final Map<String, String> _fieldErrors = <String, String>{};

  @override
  void initState() {
    super.initState();
    controller = Get.find<WoxSettingController>();
    _initialSnapshot = _cloneDraft(_parseDraft(widget.initialRow));
    _draft = _cloneDraft(_initialSnapshot);
    _nameController = TextEditingController(text: _draft.name);
    _queryController = TextEditingController(text: _draft.query);
    _widthController = TextEditingController(text: _draft.width);
    _maxResultCountController = TextEditingController(text: _draft.maxResultCount);
    _queryFocusNode = FocusNode();
    _selectedPreset = _inferPreset(_draft);
  }

  @override
  void dispose() {
    _nameController.dispose();
    _queryController.dispose();
    _widthController.dispose();
    _maxResultCountController.dispose();
    _queryFocusNode.dispose();
    super.dispose();
  }

  bool get _isEditing => widget.isEditing;

  bool get _showsDisplayFields => _selectedPreset == _QueryHotkeyPreset.webPanel || _selectedPreset == _QueryHotkeyPreset.custom;

  bool get _supportsWindowPositionSetting => !controller.woxSetting.value.isLinuxWaylandSession;

  bool get _showsCustomChromeFields => _selectedPreset == _QueryHotkeyPreset.custom;

  String tr(String key) {
    return controller.tr(key);
  }

  // Keeps dialog state isolated from the shared settings model while still using
  // the same persisted QueryHotkey shape as the backend and table rows.
  QueryHotkey _parseDraft(Map<String, dynamic> row) {
    if (row.isEmpty) {
      return QueryHotkey(
        name: "",
        hotkey: "",
        query: "",
        isSilentExecution: false,
        hideQueryBox: false,
        hideToolbar: false,
        width: "",
        maxResultCount: "",
        position: 'system_default',
        disabled: false,
      );
    }

    return QueryHotkey.fromJson(Map<String, dynamic>.from(row));
  }

  QueryHotkey _cloneDraft(QueryHotkey source) {
    return QueryHotkey(
      name: source.name,
      hotkey: source.hotkey,
      query: source.query,
      isSilentExecution: source.isSilentExecution,
      hideQueryBox: source.hideQueryBox,
      hideToolbar: source.hideToolbar,
      width: source.width,
      maxResultCount: source.maxResultCount,
      position: source.position,
      disabled: source.disabled,
    );
  }

  // Presets act as task-oriented starting points, so switching presets also
  // normalizes hidden values instead of keeping stale display settings around.
  void _applyPreset(_QueryHotkeyPreset preset) {
    setState(() {
      switch (preset) {
        case _QueryHotkeyPreset.normal:
          _resetDisplayOptions();
          _draft.isSilentExecution = false;
          break;
        case _QueryHotkeyPreset.webPanel:
          _draft.isSilentExecution = false;
          _draft.hideQueryBox = true;
          _draft.hideToolbar = true;
          _draft.width = '500';
          _draft.maxResultCount = '12';
          _draft.position = 'center';
          break;
        case _QueryHotkeyPreset.silent:
          _resetDisplayOptions();
          _draft.isSilentExecution = true;
          break;
        case _QueryHotkeyPreset.custom:
          // Custom is the "show every option" mode, so keep the current values
          // instead of collapsing back to a narrower preset state.
          break;
      }

      _widthController.text = _draft.width;
      _maxResultCountController.text = _draft.maxResultCount;
      _selectedPreset = preset;
      _saveErrorMessage = "";
      _fieldErrors.remove('Width');
      _fieldErrors.remove('MaxResultCount');
    });
  }

  void _resetDisplayOptions() {
    _draft.hideQueryBox = false;
    _draft.hideToolbar = false;
    _draft.width = "";
    _draft.maxResultCount = "";
    _draft.position = 'system_default';
  }

  _QueryHotkeyPreset _inferPreset(QueryHotkey draft) {
    if (draft.isSilentExecution && !_hasCustomDisplayOptions(draft)) {
      return _QueryHotkeyPreset.silent;
    }
    if (draft.hideQueryBox && draft.hideToolbar) {
      return _QueryHotkeyPreset.webPanel;
    }
    if (!_hasCustomDisplayOptions(draft)) {
      return _QueryHotkeyPreset.normal;
    }
    return _QueryHotkeyPreset.custom;
  }

  bool _hasCustomDisplayOptions(QueryHotkey draft) {
    return draft.position != 'system_default' ||
        draft.hideQueryBox ||
        draft.hideToolbar ||
        (draft.width.trim().isNotEmpty && draft.width.trim() != "0") ||
        (draft.maxResultCount.trim().isNotEmpty && draft.maxResultCount.trim() != "0");
  }

  void _setFieldError(String key, String value) {
    if (value.isEmpty) {
      _fieldErrors.remove(key);
    } else {
      _fieldErrors[key] = value;
    }
  }

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
    for (final modifier in ['ctrl', 'shift', 'alt', 'meta']) {
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

  bool _isHotkeyModifierToken(String token) {
    return token == 'ctrl' || token == 'shift' || token == 'alt' || token == 'meta';
  }

  bool _isEditingSameHotkey(String hotkey) {
    if (!_isEditing) {
      return false;
    }

    return _normalizeHotkey(hotkey) == _normalizeHotkey(_initialSnapshot.hotkey);
  }

  bool _isSameRow(QueryHotkey left, QueryHotkey right) {
    return left.name.trim() == right.name.trim() &&
        _normalizeHotkey(left.hotkey) == _normalizeHotkey(right.hotkey) &&
        left.query == right.query &&
        left.isSilentExecution == right.isSilentExecution &&
        left.hideQueryBox == right.hideQueryBox &&
        left.hideToolbar == right.hideToolbar &&
        left.width.trim() == right.width.trim() &&
        left.maxResultCount.trim() == right.maxResultCount.trim() &&
        left.position == right.position &&
        left.disabled == right.disabled;
  }

  String _internalHotkeyConflict(String hotkey) {
    final normalized = _normalizeHotkey(hotkey);
    if (normalized.isEmpty) {
      return "";
    }

    if (_normalizeHotkey(controller.woxSetting.value.mainHotkey) == normalized) {
      return tr('ui_hotkey_conflict_main');
    }
    if (_normalizeHotkey(controller.woxSetting.value.selectionHotkey) == normalized) {
      return tr('ui_hotkey_conflict_selection');
    }

    var skippedInitialRow = false;
    for (final existing in controller.woxSetting.value.queryHotkeys) {
      if (_isEditing && !skippedInitialRow && _isSameRow(existing, _initialSnapshot)) {
        skippedInitialRow = true;
        continue;
      }
      if (!existing.disabled && _normalizeHotkey(existing.hotkey) == normalized) {
        return tr('ui_hotkey_conflict_query').replaceAll('{query}', existing.displayName);
      }
    }

    return "";
  }

  // Splits local Wox-managed conflicts from system-level conflicts so editing an
  // existing row can keep its own hotkey without being flagged as unavailable.
  Future<void> _validateHotkeyAvailability() async {
    final hotkey = _draft.hotkey.trim();
    final validationToken = ++_hotkeyValidationToken;

    if (hotkey.isEmpty) {
      if (!mounted) {
        return;
      }
      setState(() {
        _hotkeyAvailabilityError = "";
      });
      return;
    }

    final internalConflict = _internalHotkeyConflict(hotkey);
    if (internalConflict.isNotEmpty) {
      if (!mounted || validationToken != _hotkeyValidationToken) {
        return;
      }
      setState(() {
        _hotkeyAvailabilityError = internalConflict;
      });
      return;
    }

    if (_isEditingSameHotkey(hotkey)) {
      if (!mounted || validationToken != _hotkeyValidationToken) {
        return;
      }
      setState(() {
        _hotkeyAvailabilityError = "";
      });
      return;
    }

    final available = await WoxApi.instance.isHotkeyAvailable(const UuidV4().generate(), hotkey);
    if (!mounted || validationToken != _hotkeyValidationToken) {
      return;
    }

    setState(() {
      _hotkeyAvailabilityError = available ? "" : tr('ui_hotkey_unavailable');
    });
  }

  Future<void> _handleHotkeyChanged(String hotkey) async {
    setState(() {
      _draft.hotkey = hotkey;
      _saveErrorMessage = "";
      _fieldErrors.remove('Hotkey');
    });

    await _validateHotkeyAvailability();

    if (!mounted) {
      return;
    }

    setState(() {
      if (_hotkeyAvailabilityError.isNotEmpty) {
        _fieldErrors['Hotkey'] = _hotkeyAvailabilityError;
      } else {
        _fieldErrors.remove('Hotkey');
      }
    });
  }

  bool _validateForm() {
    _draft.name = _nameController.text.trim();
    _draft.query = _queryController.text;
    _draft.width = _widthController.text.trim();
    _draft.maxResultCount = _maxResultCountController.text.trim();

    _setFieldError('Hotkey', _draft.hotkey.trim().isEmpty ? tr('ui_validator_value_can_not_be_empty') : _hotkeyAvailabilityError);
    _setFieldError('Query', _draft.query.trim().isEmpty ? tr('ui_validator_value_can_not_be_empty') : "");

    if (_showsDisplayFields) {
      if (_draft.width.trim().isNotEmpty && int.tryParse(_draft.width.trim()) == null) {
        _setFieldError('Width', tr('ui_validator_must_be_number'));
      } else {
        _setFieldError('Width', "");
      }

      final maxResultCount = _draft.maxResultCount.trim();
      final parsedMaxResultCount = maxResultCount.isEmpty ? null : int.tryParse(maxResultCount);
      if (maxResultCount.isNotEmpty && parsedMaxResultCount == null) {
        _setFieldError('MaxResultCount', tr('ui_validator_must_be_number'));
      } else if (parsedMaxResultCount != null && (parsedMaxResultCount < 5 || parsedMaxResultCount > 15)) {
        _setFieldError('MaxResultCount', tr('ui_query_hotkeys_max_result_count_range_error'));
      } else {
        _setFieldError('MaxResultCount', "");
      }
    } else {
      _setFieldError('Width', "");
      _setFieldError('MaxResultCount', "");
    }

    return _fieldErrors.isEmpty;
  }

  Future<void> _save() async {
    if (_isSaving) {
      return;
    }

    await _validateHotkeyAvailability();
    if (!mounted) {
      return;
    }

    setState(() {
      _saveErrorMessage = "";
    });

    if (!_validateForm()) {
      setState(() {});
      return;
    }

    setState(() {
      _isSaving = true;
    });

    String? saveError;
    try {
      saveError = await widget.onSave(_draft.toJson());
    } catch (error) {
      saveError = error.toString().replaceFirst('Exception: ', '');
    } finally {
      if (mounted) {
        setState(() {
          _isSaving = false;
        });
      }
    }

    if (!mounted) {
      return;
    }
    if (saveError != null && saveError.trim().isNotEmpty) {
      setState(() {
        _saveErrorMessage = saveError!;
      });
      return;
    }

    Navigator.pop(context);
  }

  List<WoxDropdownItem<String>> _buildPositionItems() {
    return [
      WoxDropdownItem(value: 'system_default', label: tr('ui_query_position_system_default')),
      WoxDropdownItem(value: 'top_left', label: tr('ui_query_position_top_left')),
      WoxDropdownItem(value: 'top_center', label: tr('ui_query_position_top_center')),
      WoxDropdownItem(value: 'top_right', label: tr('ui_query_position_top_right')),
      WoxDropdownItem(value: 'center', label: tr('ui_query_position_center')),
      WoxDropdownItem(value: 'bottom_left', label: tr('ui_query_position_bottom_left')),
      WoxDropdownItem(value: 'bottom_center', label: tr('ui_query_position_bottom_center')),
      WoxDropdownItem(value: 'bottom_right', label: tr('ui_query_position_bottom_right')),
    ];
  }

  String _presetDescription(_QueryHotkeyPreset preset) {
    switch (preset) {
      case _QueryHotkeyPreset.normal:
        return tr('ui_query_hotkeys_preset_normal_description');
      case _QueryHotkeyPreset.webPanel:
        return tr('ui_query_hotkeys_preset_web_panel_description');
      case _QueryHotkeyPreset.silent:
        return tr('ui_query_hotkeys_preset_silent_description');
      case _QueryHotkeyPreset.custom:
        return tr('ui_query_hotkeys_preset_custom_description');
    }
  }

  WoxQueryHotkeysDemoMode _presetDemoMode(_QueryHotkeyPreset preset) {
    switch (preset) {
      case _QueryHotkeyPreset.normal:
        return WoxQueryHotkeysDemoMode.normal;
      case _QueryHotkeyPreset.webPanel:
        return WoxQueryHotkeysDemoMode.webPanel;
      case _QueryHotkeyPreset.silent:
        return WoxQueryHotkeysDemoMode.silent;
      case _QueryHotkeyPreset.custom:
        return WoxQueryHotkeysDemoMode.custom;
    }
  }

  Color _presetDemoAccent(_QueryHotkeyPreset preset) {
    switch (preset) {
      case _QueryHotkeyPreset.normal:
        return const Color(0xFF3B82F6);
      case _QueryHotkeyPreset.webPanel:
        return const Color(0xFFF43F5E);
      case _QueryHotkeyPreset.silent:
        return const Color(0xFF22C55E);
      case _QueryHotkeyPreset.custom:
        return const Color(0xFFF59E0B);
    }
  }

  bool _showsPresetDemo(_QueryHotkeyPreset preset) {
    return preset != _QueryHotkeyPreset.custom;
  }

  // The trigger lives on the small icon so hovering the preview affordance does
  // not interfere with clicking the pill itself to switch presets.
  Widget _buildPresetDemoTrigger({required _QueryHotkeyPreset preset, required bool selected}) {
    final Color iconColor = selected ? getThemeActiveBackgroundColor().withValues(alpha: 0.92) : getThemeTextColor().withValues(alpha: 0.70);

    return WoxDemoPopover(
      key: ValueKey('query-hotkey-preset-demo-trigger-${preset.name}'),
      popoverKey: ValueKey('query-hotkey-preset-demo-${preset.name}'),
      width: 680,
      height: 460,
      demo: WoxQueryHotkeysDemo(accent: _presetDemoAccent(preset), tr: tr, mode: _presetDemoMode(preset)),
      child: Semantics(
        label: tr('ui_demo_preview'),
        button: true,
        child: MouseRegion(cursor: SystemMouseCursors.help, child: SizedBox(width: 16, height: 16, child: Icon(Icons.play_circle_outline_rounded, color: iconColor, size: 15))),
      ),
    );
  }

  Widget _buildPresetButton({required _QueryHotkeyPreset preset, required String title}) {
    final selected = _selectedPreset == preset;
    final backgroundColor = selected ? getThemeActiveBackgroundColor().withValues(alpha: 0.14) : Colors.transparent;
    final borderColor = selected ? getThemeActiveBackgroundColor().withValues(alpha: 0.68) : getThemeSubTextColor().withValues(alpha: 0.28);

    return InkWell(
      onTap: () => _applyPreset(preset),
      borderRadius: BorderRadius.circular(8),
      splashFactory: NoSplash.splashFactory,
      highlightColor: Colors.transparent,
      splashColor: Colors.transparent,
      hoverColor: Colors.transparent,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 140),
        height: 38,
        alignment: Alignment.center,
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
        decoration: BoxDecoration(color: backgroundColor, borderRadius: BorderRadius.circular(8), border: Border.all(color: borderColor)),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text(
              title,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(color: getThemeTextColor(), fontSize: 12, fontWeight: selected ? FontWeight.w700 : FontWeight.w600),
            ),
            if (_showsPresetDemo(preset)) ...[const SizedBox(width: 6), _buildPresetDemoTrigger(preset: preset, selected: selected)],
          ],
        ),
      ),
    );
  }

  Widget _buildPresetSelector() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(child: _buildPresetButton(preset: _QueryHotkeyPreset.normal, title: tr('ui_query_hotkeys_preset_normal'))),
            const SizedBox(width: 8),
            Expanded(child: _buildPresetButton(preset: _QueryHotkeyPreset.webPanel, title: tr('ui_query_hotkeys_preset_web_panel'))),
            const SizedBox(width: 8),
            Expanded(child: _buildPresetButton(preset: _QueryHotkeyPreset.silent, title: tr('ui_query_hotkeys_preset_silent'))),
            const SizedBox(width: 8),
            Expanded(child: _buildPresetButton(preset: _QueryHotkeyPreset.custom, title: tr('ui_query_hotkeys_preset_custom'))),
          ],
        ),
        const SizedBox(height: 8),
        WoxMarkdownView(data: _presetDescription(_selectedPreset), fontColor: getThemeSubTextColor(), fontSize: 12, selectable: false),
      ],
    );
  }

  Widget _buildFormRow({required String label, required Widget child, String helperMarkdown = '', String error = ''}) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 96,
            child: Padding(padding: const EdgeInsets.only(top: 8), child: Text(label, style: TextStyle(color: getThemeTextColor(), fontSize: 14, fontWeight: FontWeight.w700))),
          ),
          const SizedBox(width: 18),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                child,
                if (helperMarkdown.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: WoxMarkdownView(
                      data: helperMarkdown,
                      fontColor: getThemeSubTextColor().withValues(alpha: 0.82),
                      fontSize: 12,
                      linkColor: getThemeActiveBackgroundColor(),
                      linkHoverColor: getThemeActiveBackgroundColor().withValues(alpha: 0.85),
                    ),
                  ),
                if (error.isNotEmpty) Padding(padding: const EdgeInsets.only(top: 6), child: Text(error, style: const TextStyle(color: Colors.red, fontSize: 12))),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCheckboxControl({required bool value, required ValueChanged<bool> onChanged}) {
    return InkWell(
      onTap: () => onChanged(!value),
      borderRadius: BorderRadius.circular(4),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 2),
        child: WoxCheckbox(
          value: value,
          onChanged: (next) {
            onChanged(next ?? false);
          },
        ),
      ),
    );
  }

  Widget _buildSelectControl() {
    return WoxDropdownButton<String>(
      value: _draft.position,
      width: 320,
      isExpanded: true,
      underline: const SizedBox.shrink(),
      onChanged: (value) {
        setState(() {
          _draft.position = value ?? 'system_default';
        });
      },
      items: _buildPositionItems(),
    );
  }

  @override
  Widget build(BuildContext context) {
    return WoxDialog(
      contentPadding: const EdgeInsets.fromLTRB(28, 24, 28, 0),
      actionsPadding: const EdgeInsets.fromLTRB(28, 12, 28, 24),
      content: SizedBox(
        width: _dialogContentWidth,
        height: _dialogContentHeight,
        child: SingleChildScrollView(
          child: Padding(
            // Desktop scrollbars can overlay scroll views instead of taking
            // layout space, so keep a small gutter on the right to prevent
            // helper text from ending up underneath the thumb.
            padding: const EdgeInsets.only(right: _dialogScrollGutter),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  tr(_isEditing ? 'ui_query_hotkeys_dialog_edit_title' : 'ui_query_hotkeys_dialog_create_title'),
                  style: TextStyle(color: getThemeTextColor().withValues(alpha: 0.94), fontSize: 18, fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: 14),
                _buildPresetSelector(),
                const SizedBox(height: 18),
                _buildFormRow(
                  label: tr('ui_query_hotkeys_name'),
                  helperMarkdown: tr('ui_query_hotkeys_name_tooltip'),
                  child: WoxTextField(
                    width: double.infinity,
                    controller: _nameController,
                    onChanged: (value) {
                      setState(() {
                        _draft.name = value.trim();
                        _saveErrorMessage = "";
                      });
                    },
                  ),
                ),
                _buildFormRow(
                  label: tr('ui_query_hotkeys_hotkey'),
                  helperMarkdown: tr('ui_query_hotkeys_hotkey_tooltip'),
                  error: _fieldErrors['Hotkey'] ?? '',
                  child: WoxHotkeyRecorder(
                    hotkey: WoxHotkey.parseHotkeyFromString(_draft.hotkey),
                    tipPosition: WoxHotkeyRecorderTipPosition.right,
                    recordUnavailableHotkey: true,
                    onHotKeyRecorded: (hotkey) {
                      _handleHotkeyChanged(hotkey);
                    },
                    onUnavailableHotKeyRecorded: (hotkey) {
                      _handleHotkeyChanged(hotkey);
                    },
                  ),
                ),
                _buildFormRow(
                  label: tr('ui_query_hotkeys_query'),
                  helperMarkdown: tr('ui_query_hotkeys_query_tooltip'),
                  error: _fieldErrors['Query'] ?? '',
                  child: WoxQueryVariableTextField(
                    key: const ValueKey('Query'),
                    controller: _queryController,
                    focusNode: _queryFocusNode,
                    width: double.infinity,
                    maxLines: 1,
                    source: WoxQueryVariableSource.queryHotkey,
                    onChanged: (value) {
                      setState(() {
                        _draft.query = value;
                        _fieldErrors.remove('Query');
                        _saveErrorMessage = "";
                      });
                    },
                  ),
                ),
                if (_selectedPreset == _QueryHotkeyPreset.custom)
                  _buildFormRow(
                    label: tr('ui_query_hotkeys_silent'),
                    helperMarkdown: tr('ui_query_hotkeys_silent_tooltip'),
                    child: _buildCheckboxControl(
                      value: _draft.isSilentExecution,
                      onChanged: (value) {
                        setState(() {
                          _draft.isSilentExecution = value;
                        });
                      },
                    ),
                  ),
                if (_showsDisplayFields) ...[
                  if (_supportsWindowPositionSetting)
                    _buildFormRow(label: tr('ui_query_hotkeys_position'), helperMarkdown: tr('ui_query_hotkeys_position_tooltip'), child: _buildSelectControl()),
                  _buildFormRow(
                    label: tr('ui_query_hotkeys_width'),
                    helperMarkdown: tr('ui_query_hotkeys_width_tooltip'),
                    error: _fieldErrors['Width'] ?? '',
                    child: WoxTextField(
                      width: 260,
                      controller: _widthController,
                      onChanged: (value) {
                        setState(() {
                          _draft.width = value.trim();
                          _fieldErrors.remove('Width');
                        });
                      },
                    ),
                  ),
                  _buildFormRow(
                    label: tr('ui_query_hotkeys_max_result_count'),
                    helperMarkdown: tr('ui_query_hotkeys_max_result_count_tooltip'),
                    error: _fieldErrors['MaxResultCount'] ?? '',
                    child: WoxTextField(
                      width: 260,
                      controller: _maxResultCountController,
                      onChanged: (value) {
                        setState(() {
                          _draft.maxResultCount = value.trim();
                          _fieldErrors.remove('MaxResultCount');
                        });
                      },
                    ),
                  ),
                  if (_showsCustomChromeFields)
                    _buildFormRow(
                      label: tr('ui_query_hotkeys_hide_query_box'),
                      helperMarkdown: tr('ui_query_hotkeys_hide_query_box_tooltip'),
                      child: _buildCheckboxControl(
                        value: _draft.hideQueryBox,
                        onChanged: (value) {
                          setState(() {
                            _draft.hideQueryBox = value;
                          });
                        },
                      ),
                    ),
                  if (_showsCustomChromeFields)
                    _buildFormRow(
                      label: tr('ui_query_hotkeys_hide_toolbar'),
                      helperMarkdown: tr('ui_query_hotkeys_hide_toolbar_tooltip'),
                      child: _buildCheckboxControl(
                        value: _draft.hideToolbar,
                        onChanged: (value) {
                          setState(() {
                            _draft.hideToolbar = value;
                          });
                        },
                      ),
                    ),
                ],
                if (_isEditing)
                  _buildFormRow(
                    label: tr('ui_disabled'),
                    helperMarkdown: tr('ui_disabled_tooltip'),
                    child: _buildCheckboxControl(
                      value: _draft.disabled,
                      onChanged: (value) {
                        setState(() {
                          _draft.disabled = value;
                        });
                      },
                    ),
                  ),
                if (_saveErrorMessage.isNotEmpty)
                  Padding(padding: const EdgeInsets.only(top: 12), child: Text(_saveErrorMessage, style: const TextStyle(color: Colors.red, fontSize: 12))),
              ],
            ),
          ),
        ),
      ),
      actions: [
        WoxButton.secondary(text: tr('ui_cancel'), padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12), onPressed: _isSaving ? null : () => Navigator.pop(context)),
        const SizedBox(width: 12),
        WoxButton.primary(
          text: _isSaving ? tr('ui_saving') : tr('ui_save'),
          padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 12),
          onPressed: _isSaving ? null : _save,
        ),
      ],
    );
  }
}
