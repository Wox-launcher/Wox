import 'dart:convert';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_app_selector.dart';
import 'package:wox/components/wox_button.dart';
import 'package:wox/components/wox_dialog.dart';
import 'package:wox/components/wox_image_view.dart';
import 'package:wox/components/wox_textfield.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_window_manager.dart';
import 'package:wox/utils/colors.dart';
import 'package:wox/utils/log.dart';

class WoxWindowManagerGroupsSetting extends StatefulWidget {
  final String value;
  final double labelWidth;
  final Future<String?> Function(String key, String value) onUpdate;

  const WoxWindowManagerGroupsSetting({super.key, required this.value, required this.labelWidth, required this.onUpdate});

  @override
  State<WoxWindowManagerGroupsSetting> createState() => _WoxWindowManagerGroupsSettingState();
}

class _WoxWindowManagerGroupsSettingState extends State<WoxWindowManagerGroupsSetting> {
  static const String _settingKey = 'windowGroups';

  late final WoxSettingController controller;
  late List<_WindowManagerGroup> _groups;
  late String _lastSavedValue;

  List<WindowManagerDisplay> _displays = <WindowManagerDisplay>[];
  bool _isLoadingDisplays = false;
  bool _isSaving = false;
  String _displayError = '';

  String tr(String key) => controller.tr(key);

  @override
  void initState() {
    super.initState();
    controller = Get.find<WoxSettingController>();
    _groups = _decodeGroups(widget.value);
    _lastSavedValue = _encodeGroups(_groups);
    _loadDisplays();
  }

  @override
  void didUpdateWidget(covariant WoxWindowManagerGroupsSetting oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.value == widget.value || _isDirty) {
      return;
    }
    _groups = _decodeGroups(widget.value);
    _lastSavedValue = _encodeGroups(_groups);
  }

  bool get _isDirty => _encodeGroups(_groups) != _lastSavedValue;

  Future<void> _loadDisplays() async {
    final traceId = const UuidV4().generate();
    setState(() {
      _isLoadingDisplays = true;
      _displayError = '';
    });

    try {
      final displays = await WoxApi.instance.getWindowManagerDisplays(traceId);
      if (!mounted) {
        return;
      }
      setState(() {
        _displays = displays;
        _isLoadingDisplays = false;
      });
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to load window manager displays: $e');
      if (!mounted) {
        return;
      }
      setState(() {
        _isLoadingDisplays = false;
        _displayError = e.toString();
      });
    }
  }

  Future<void> _saveGroups() async {
    if (_isSaving) {
      return;
    }

    setState(() {
      _isSaving = true;
    });

    final nextValue = _encodeGroups(_groups);
    try {
      await widget.onUpdate(_settingKey, nextValue);
      if (mounted) {
        setState(() {
          _lastSavedValue = nextValue;
          _isSaving = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _isSaving = false;
        });
      }
      rethrow;
    }
  }

  Future<void> _openAddGroupDialog() async {
    final group = _WindowManagerGroup(
      id: const UuidV4().generate(),
      name: tr('plugin_window_manager_group_default_name'),
      screens:
          _displays
              .asMap()
              .entries
              .map(
                (entry) =>
                    _WindowManagerGroupScreen(displayId: entry.value.id, displayIndex: entry.key, layout: _WindowGroupLayouts.full.id, assignments: <_WindowManagerAssignment>[]),
              )
              .toList(),
    );

    final saved = await _showGroupDialog(group);
    if (!mounted || saved == null) {
      return;
    }

    setState(() {
      _groups.add(saved);
    });
    await _saveGroups();
  }

  Future<void> _openEditGroupDialog(_WindowManagerGroup group) async {
    final saved = await _showGroupDialog(group.copy());
    if (!mounted || saved == null) {
      return;
    }

    setState(() {
      final index = _groups.indexWhere((item) => item.id == group.id);
      if (index >= 0) {
        _groups[index] = saved;
      }
    });
    await _saveGroups();
  }

  Future<_WindowManagerGroup?> _showGroupDialog(_WindowManagerGroup group) async {
    return await showDialog<_WindowManagerGroup>(
      context: context,
      barrierColor: getThemePopupBarrierColor(),
      builder: (context) => _WindowGroupDialog(group: group, displays: _displays, tr: tr),
    );
  }

  Future<void> _deleteGroup(_WindowManagerGroup group) async {
    setState(() {
      _groups.removeWhere((item) => item.id == group.id);
    });

    await _saveGroups();
  }

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: widget.labelWidth,
          child: Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(tr('plugin_window_manager_setting_groups'), style: TextStyle(color: getThemeTextColor(), fontSize: 13)),
          ),
        ),
        const SizedBox(width: 16),
        Expanded(child: _buildContent()),
      ],
    );
  }

  Widget _buildContent() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            WoxButton.secondary(
              text: tr('plugin_window_manager_group_add'),
              icon: Icon(Icons.add, size: 14, color: getThemeTextColor()),
              height: 30,
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
              onPressed: _isLoadingDisplays ? null : _openAddGroupDialog,
            ),
            if (_isDirty)
              WoxButton.primary(
                text: _isSaving ? tr('ui_saving') : tr('ui_save'),
                icon: Icon(Icons.save_outlined, size: 14, color: getThemeActionItemActiveColor()),
                height: 30,
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                onPressed: _isSaving ? null : _saveGroups,
              ),
          ],
        ),
        const SizedBox(height: 12),
        if (_isLoadingDisplays) Text(tr('plugin_window_manager_group_loading_displays'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
        if (_displayError.isNotEmpty) _buildDisplayError(),
        if (!_isLoadingDisplays && _displayError.isEmpty && _groups.isEmpty) _buildEmptyState(),
        if (!_isLoadingDisplays && _displayError.isEmpty && _groups.isNotEmpty) ..._groups.map(_buildGroupRow),
      ],
    );
  }

  Widget _buildDisplayError() {
    return Row(
      children: [
        Expanded(child: Text(_displayError, style: TextStyle(color: getThemeSubTextColor(), fontSize: 12), maxLines: 2, overflow: TextOverflow.ellipsis)),
        const SizedBox(width: 8),
        WoxButton.secondary(text: tr('plugin_window_manager_group_retry'), height: 30, padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6), onPressed: _loadDisplays),
      ],
    );
  }

  Widget _buildEmptyState() {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)), borderRadius: BorderRadius.circular(6)),
      child: Text(tr('plugin_window_manager_group_empty_subtitle'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
    );
  }

  Widget _buildGroupRow(_WindowManagerGroup group) {
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)), borderRadius: BorderRadius.circular(6)),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(group.name.trim().isEmpty ? group.id : group.name, style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w600)),
                const SizedBox(height: 3),
                Text(
                  tr('plugin_window_manager_group_subtitle').replaceFirst('%d', '${group.appCount}').replaceFirst('%d', '${group.screens.length}'),
                  style: TextStyle(color: getThemeSubTextColor(), fontSize: 11),
                ),
              ],
            ),
          ),
          WoxButton.secondary(
            text: tr('plugin_window_manager_group_edit'),
            icon: Icon(Icons.edit_outlined, size: 14, color: getThemeTextColor()),
            height: 30,
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            onPressed: () => _openEditGroupDialog(group),
          ),
          const SizedBox(width: 8),
          IconButton(
            tooltip: tr('ui_delete'),
            icon: Icon(Icons.delete_outline, size: 18, color: getThemeSubTextColor()),
            padding: EdgeInsets.zero,
            constraints: const BoxConstraints.tightFor(width: 30, height: 30),
            onPressed: () => _deleteGroup(group),
          ),
        ],
      ),
    );
  }

  List<_WindowManagerGroup> _decodeGroups(String value) {
    final raw = value.trim().isEmpty ? '[]' : value.trim();
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! List) {
        return <_WindowManagerGroup>[];
      }
      return decoded.whereType<Map>().map((item) => _WindowManagerGroup.fromJson(Map<String, dynamic>.from(item))).where((group) => group.id.trim().isNotEmpty).toList();
    } catch (_) {
      return <_WindowManagerGroup>[];
    }
  }

  String _encodeGroups(List<_WindowManagerGroup> groups) {
    return jsonEncode(groups.map((group) => group.toJson()).toList());
  }
}

class _WindowGroupDialog extends StatefulWidget {
  final _WindowManagerGroup group;
  final List<WindowManagerDisplay> displays;
  final String Function(String key) tr;

  const _WindowGroupDialog({required this.group, required this.displays, required this.tr});

  @override
  State<_WindowGroupDialog> createState() => _WindowGroupDialogState();
}

class _WindowGroupDialogState extends State<_WindowGroupDialog> {
  static const List<int> _slotCounts = <int>[1, 2, 3, 4];

  late final TextEditingController _nameController;
  late _WindowManagerGroup _group;
  int _selectedDisplayIndex = 0;
  List<IgnoredHotkeyApp> _availableApps = <IgnoredHotkeyApp>[];

  String tr(String key) => widget.tr(key);

  @override
  void initState() {
    super.initState();
    _group = widget.group.copy();
    _nameController = TextEditingController(text: _group.name);
    _nameController.addListener(() {
      _group.name = _nameController.text;
    });
    if (widget.displays.isNotEmpty) {
      _selectedDisplayIndex = 0;
      for (final entry in widget.displays.asMap().entries) {
        _group.screenFor(entry.value, entry.key);
      }
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    super.dispose();
  }

  WindowManagerDisplay? get _selectedDisplay {
    if (widget.displays.isEmpty) {
      return null;
    }
    final index = _selectedDisplayIndex.clamp(0, widget.displays.length - 1).toInt();
    return widget.displays[index];
  }

  _WindowManagerGroupScreen? get _selectedScreen {
    final display = _selectedDisplay;
    if (display == null) {
      return null;
    }
    return _group.screenFor(display, _selectedDisplayIndex);
  }

  _WindowGroupLayout get _selectedLayout {
    final screen = _selectedScreen;
    return _WindowGroupLayouts.byId(screen?.layout ?? _WindowGroupLayouts.fullId);
  }

  Future<List<IgnoredHotkeyApp>> _loadAvailableApps() async {
    if (_availableApps.isNotEmpty) {
      return _availableApps;
    }

    final traceId = const UuidV4().generate();
    try {
      _availableApps = await WoxApi.instance.getHotkeyAppCandidates(traceId);
    } catch (e) {
      Logger.instance.error(traceId, 'Failed to load window group app candidates: $e');
      _availableApps = <IgnoredHotkeyApp>[];
    }
    return _availableApps;
  }

  Future<void> _openSlotAppSelector(_WindowGroupSlot slot) async {
    final screen = _selectedScreen;
    if (screen == null) {
      return;
    }

    final current = screen.assignmentFor(slot.id)?.app ?? IgnoredHotkeyApp.empty();
    final apps = await _loadAvailableApps();
    if (!mounted) {
      return;
    }

    final selectedApp = await showWoxAppSelectorDialog(
      context: context,
      selectedApp: current,
      initialApps: _mergeSelectedApp(current, apps),
      loadApps: () async => _mergeSelectedApp(current, await _loadAvailableApps()),
    );
    if (!mounted || selectedApp == null) {
      return;
    }

    setState(() {
      _setSlotApp(slot.id, selectedApp);
    });
  }

  void _setSlotApp(String slotId, IgnoredHotkeyApp app) {
    final identity = app.identity.trim().toLowerCase();
    if (identity.isNotEmpty) {
      for (final screen in _group.screens) {
        screen.assignments.removeWhere((assignment) => assignment.app.identity.trim().toLowerCase() == identity);
      }
    }

    final screen = _selectedScreen;
    if (screen == null) {
      return;
    }
    screen.assignments.removeWhere((assignment) => assignment.slot == slotId);
    if (identity.isNotEmpty) {
      screen.assignments.add(_WindowManagerAssignment(slot: slotId, app: app));
    }
  }

  List<IgnoredHotkeyApp> _mergeSelectedApp(IgnoredHotkeyApp selectedApp, List<IgnoredHotkeyApp> apps) {
    final merged = <IgnoredHotkeyApp>[];
    final seen = <String>{};
    final selectedIdentity = selectedApp.identity.trim().toLowerCase();
    if (selectedIdentity.isNotEmpty) {
      merged.add(selectedApp);
      seen.add(selectedIdentity);
    }
    for (final app in apps) {
      final identity = app.identity.trim().toLowerCase();
      if (identity.isEmpty || seen.contains(identity)) {
        continue;
      }
      seen.add(identity);
      merged.add(app);
    }
    return merged;
  }

  void _setSelectedLayout(_WindowGroupLayout layout) {
    final screen = _selectedScreen;
    if (screen == null) {
      return;
    }

    setState(() {
      screen.layout = layout.id;
      screen.assignments.removeWhere((assignment) => !layout.slots.any((slot) => slot.id == assignment.slot));
    });
  }

  @override
  Widget build(BuildContext context) {
    final textColor = getThemeTextColor();
    return WoxDialog(
      title: Text(tr('plugin_window_manager_group_dialog_title'), style: TextStyle(color: textColor, fontSize: 16)),
      titleTextStyle: TextStyle(color: textColor, fontSize: 16),
      insetPadding: const EdgeInsets.symmetric(horizontal: 28, vertical: 24),
      content: SizedBox(
        width: 1040,
        height: 650,
        child: Column(
          children: [
            Row(
              children: [
                SizedBox(
                  width: 360,
                  child: WoxTextField(
                    controller: _nameController,
                    hintText: tr('plugin_window_manager_group_name'),
                    width: double.infinity,
                    contentPadding: const EdgeInsets.symmetric(horizontal: 10, vertical: 9),
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(child: Text(tr('plugin_window_manager_group_select_display'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12))),
              ],
            ),
            const SizedBox(height: 14),
            Expanded(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [Expanded(flex: 11, child: _buildDisplayArrangement()), const SizedBox(width: 18), SizedBox(width: 330, child: _buildLayoutPanel())],
              ),
            ),
          ],
        ),
      ),
      actions: [
        WoxButton.secondary(text: tr('ui_cancel'), padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12), onPressed: () => Navigator.pop(context)),
        WoxButton.primary(text: tr('ui_save'), padding: const EdgeInsets.symmetric(horizontal: 22, vertical: 12), onPressed: () => Navigator.pop(context, _group)),
      ],
    );
  }

  Widget _buildDisplayArrangement() {
    if (widget.displays.isEmpty) {
      return Container(
        decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)), borderRadius: BorderRadius.circular(6)),
        alignment: Alignment.center,
        child: Text(tr('plugin_window_manager_group_no_displays'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 13)),
      );
    }

    final rects = widget.displays.map(_displayRect).toList();
    final minX = rects.map((rect) => rect.left).reduce(math.min);
    final minY = rects.map((rect) => rect.top).reduce(math.min);
    final maxX = rects.map((rect) => rect.right).reduce(math.max);
    final maxY = rects.map((rect) => rect.bottom).reduce(math.max);
    final desktopWidth = math.max(1.0, maxX - minX);
    final desktopHeight = math.max(1.0, maxY - minY);

    return Container(
      decoration: BoxDecoration(
        color: getThemeSubTextColor().withValues(alpha: 0.06),
        border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)),
        borderRadius: BorderRadius.circular(6),
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          const padding = 24.0;
          final scale =
              math.min((constraints.maxWidth - padding * 2) / desktopWidth, (constraints.maxHeight - padding * 2) / desktopHeight).clamp(0.01, double.infinity).toDouble();
          final contentWidth = desktopWidth * scale;
          final contentHeight = desktopHeight * scale;
          final offsetX = (constraints.maxWidth - contentWidth) / 2;
          final offsetY = (constraints.maxHeight - contentHeight) / 2;

          return Stack(
            children: [
              for (final entry in widget.displays.asMap().entries)
                Positioned(
                  left: offsetX + (rects[entry.key].left - minX) * scale,
                  top: offsetY + (rects[entry.key].top - minY) * scale,
                  width: math.max(90.0, rects[entry.key].width * scale),
                  height: math.max(58.0, rects[entry.key].height * scale),
                  child: _buildDisplayTile(entry.value, entry.key),
                ),
            ],
          );
        },
      ),
    );
  }

  Widget _buildDisplayTile(WindowManagerDisplay display, int displayIndex) {
    final selected = displayIndex == _selectedDisplayIndex;
    final layout = selected ? _selectedLayout : _WindowGroupLayouts.byId(_group.screenFor(display, displayIndex).layout);
    final screen = _group.screenFor(display, displayIndex);

    return Material(
      color: selected ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.26 : 0.18) : getThemePopupSurfaceColor(),
      borderRadius: BorderRadius.circular(6),
      child: InkWell(
        borderRadius: BorderRadius.circular(6),
        onTap: () => setState(() => _selectedDisplayIndex = displayIndex),
        child: Container(
          decoration: BoxDecoration(
            border: Border.all(color: selected ? getThemeActiveBackgroundColor() : getThemeSubTextColor().withValues(alpha: 0.4), width: selected ? 2 : 1),
            borderRadius: BorderRadius.circular(6),
          ),
          child: Stack(
            children: [
              Positioned.fill(
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    return Stack(
                      children:
                          layout.slots.map((slot) {
                            final assignment = screen.assignmentFor(slot.id);
                            return Positioned(
                              left: slot.left * constraints.maxWidth,
                              top: slot.top * constraints.maxHeight,
                              width: slot.width * constraints.maxWidth,
                              height: slot.height * constraints.maxHeight,
                              child: Padding(padding: const EdgeInsets.all(3), child: _buildSlotTile(slot, assignment, selected, displayIndex)),
                            );
                          }).toList(),
                    );
                  },
                ),
              ),
              Positioned(left: 8, top: 6, child: Text('${displayIndex + 1}', style: TextStyle(color: selected ? getThemeTextColor() : getThemeSubTextColor(), fontSize: 11))),
              if (display.isPrimary)
                Positioned(right: 8, top: 6, child: Text(tr('plugin_window_manager_group_display_primary'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 10))),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSlotTile(_WindowGroupSlot slot, _WindowManagerAssignment? assignment, bool selectedDisplay, int displayIndex) {
    final label = assignment?.app.name.trim().isNotEmpty == true ? assignment!.app.name : tr(slot.titleKey);
    return Material(
      color: assignment == null ? Colors.transparent : getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.28 : 0.2),
      borderRadius: BorderRadius.circular(4),
      child: InkWell(
        borderRadius: BorderRadius.circular(4),
        onTap: selectedDisplay ? () => _openSlotAppSelector(slot) : () => setState(() => _selectedDisplayIndex = displayIndex),
        child: Container(
          alignment: Alignment.center,
          decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.38)), borderRadius: BorderRadius.circular(4)),
          padding: const EdgeInsets.symmetric(horizontal: 6),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            mainAxisSize: MainAxisSize.min,
            children: [
              if (assignment != null && assignment.app.icon.imageData.isNotEmpty) ...[
                ClipRRect(borderRadius: BorderRadius.circular(4), child: WoxImageView(woxImage: assignment.app.icon, width: 16, height: 16)),
                const SizedBox(width: 5),
              ],
              Flexible(child: Text(label, style: TextStyle(color: getThemeTextColor(), fontSize: 11), maxLines: 1, overflow: TextOverflow.ellipsis, textAlign: TextAlign.center)),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildLayoutPanel() {
    return Container(
      decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.35)), borderRadius: BorderRadius.circular(6)),
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(tr('plugin_window_manager_group_layouts'), style: TextStyle(color: getThemeTextColor(), fontSize: 13, fontWeight: FontWeight.w600)),
            const SizedBox(height: 10),
            for (final count in _slotCounts) _buildLayoutGroup(count),
          ],
        ),
      ),
    );
  }

  Widget _buildLayoutGroup(int slotCount) {
    final layouts = _WindowGroupLayouts.bySlotCount(slotCount);
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(tr('plugin_window_manager_group_slot_count').replaceAll('{count}', '$slotCount'), style: TextStyle(color: getThemeSubTextColor(), fontSize: 12)),
          const SizedBox(height: 8),
          Wrap(spacing: 8, runSpacing: 8, children: layouts.map(_buildLayoutCard).toList()),
        ],
      ),
    );
  }

  Widget _buildLayoutCard(_WindowGroupLayout layout) {
    final selected = _selectedLayout.id == layout.id;
    return InkWell(
      borderRadius: BorderRadius.circular(4),
      onTap: () => _setSelectedLayout(layout),
      child: Container(
        width: 142,
        height: 82,
        padding: const EdgeInsets.all(7),
        decoration: BoxDecoration(
          color: selected ? getThemeActiveBackgroundColor().withValues(alpha: isThemeDark() ? 0.22 : 0.12) : Colors.transparent,
          borderRadius: BorderRadius.circular(4),
          border: Border.all(color: selected ? getThemeActiveBackgroundColor() : getThemeSubTextColor().withValues(alpha: 0.35)),
        ),
        child: Column(
          children: [
            Expanded(child: _buildMiniLayout(layout)),
            const SizedBox(height: 5),
            Text(tr(layout.titleKey), style: TextStyle(color: getThemeTextColor(), fontSize: 10), maxLines: 1, overflow: TextOverflow.ellipsis),
          ],
        ),
      ),
    );
  }

  Widget _buildMiniLayout(_WindowGroupLayout layout) {
    return Container(
      decoration: BoxDecoration(border: Border.all(color: getThemeSubTextColor().withValues(alpha: 0.45)), borderRadius: BorderRadius.circular(3)),
      child: LayoutBuilder(
        builder: (context, constraints) {
          return Stack(
            children:
                layout.slots.map((slot) {
                  return Positioned(
                    left: slot.left * constraints.maxWidth,
                    top: slot.top * constraints.maxHeight,
                    width: slot.width * constraints.maxWidth,
                    height: slot.height * constraints.maxHeight,
                    child: Padding(
                      padding: const EdgeInsets.all(1.5),
                      child: Container(decoration: BoxDecoration(color: getThemeActiveBackgroundColor().withValues(alpha: 0.7), borderRadius: BorderRadius.circular(2))),
                    ),
                  );
                }).toList(),
          );
        },
      ),
    );
  }

  Rect _displayRect(WindowManagerDisplay display) {
    final rect = display.bounds.width > 0 && display.bounds.height > 0 ? display.bounds : display.workArea;
    return Rect.fromLTWH(rect.x.toDouble(), rect.y.toDouble(), math.max(1.0, rect.width.toDouble()), math.max(1.0, rect.height.toDouble()));
  }
}

class _WindowManagerGroup {
  String id;
  String name;
  List<_WindowManagerGroupScreen> screens;

  _WindowManagerGroup({required this.id, required this.name, required this.screens});

  int get appCount {
    var count = 0;
    for (final screen in screens) {
      count += screen.assignments.where((assignment) => assignment.app.identity.trim().isNotEmpty).length;
    }
    return count;
  }

  factory _WindowManagerGroup.fromJson(Map<String, dynamic> json) {
    return _WindowManagerGroup(
      id: json['Id'] ?? '',
      name: json['Name'] ?? '',
      screens:
          (json['Screens'] is List ? json['Screens'] as List : <dynamic>[])
              .whereType<Map>()
              .map((item) => _WindowManagerGroupScreen.fromJson(Map<String, dynamic>.from(item)))
              .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'Id': id, 'Name': name, 'Screens': screens.map((screen) => screen.toJson()).toList()};
  }

  _WindowManagerGroup copy() {
    return _WindowManagerGroup(id: id, name: name, screens: screens.map((screen) => screen.copy()).toList());
  }

  _WindowManagerGroupScreen screenFor(WindowManagerDisplay display, int displayIndex) {
    for (final screen in screens) {
      if (screen.matches(display, displayIndex)) {
        return screen;
      }
    }
    final screen = _WindowManagerGroupScreen(displayId: display.id, displayIndex: displayIndex, layout: _WindowGroupLayouts.full.id, assignments: <_WindowManagerAssignment>[]);
    screens.add(screen);
    return screen;
  }
}

class _WindowManagerGroupScreen {
  String displayId;
  int displayIndex;
  String layout;
  List<_WindowManagerAssignment> assignments;

  _WindowManagerGroupScreen({required this.displayId, required this.displayIndex, required this.layout, required this.assignments});

  factory _WindowManagerGroupScreen.fromJson(Map<String, dynamic> json) {
    return _WindowManagerGroupScreen(
      displayId: json['DisplayId'] ?? '',
      displayIndex: json['DisplayIndex'] ?? 0,
      layout: json['Layout'] ?? _WindowGroupLayouts.fullId,
      assignments:
          (json['Assignments'] is List ? json['Assignments'] as List : <dynamic>[])
              .whereType<Map>()
              .map((item) => _WindowManagerAssignment.fromJson(Map<String, dynamic>.from(item)))
              .toList(),
    );
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'DisplayId': displayId, 'DisplayIndex': displayIndex, 'Layout': layout, 'Assignments': assignments.map((assignment) => assignment.toJson()).toList()};
  }

  _WindowManagerGroupScreen copy() {
    return _WindowManagerGroupScreen(displayId: displayId, displayIndex: displayIndex, layout: layout, assignments: assignments.map((assignment) => assignment.copy()).toList());
  }

  bool matches(WindowManagerDisplay display, int index) {
    return (displayId.trim().isNotEmpty && displayId == display.id) || (displayId.trim().isEmpty && displayIndex == index);
  }

  _WindowManagerAssignment? assignmentFor(String slotId) {
    for (final assignment in assignments) {
      if (assignment.slot == slotId) {
        return assignment;
      }
    }
    return null;
  }
}

class _WindowManagerAssignment {
  String slot;
  IgnoredHotkeyApp app;

  _WindowManagerAssignment({required this.slot, required this.app});

  factory _WindowManagerAssignment.fromJson(Map<String, dynamic> json) {
    final rawApp = json['App'];
    return _WindowManagerAssignment(slot: json['Slot'] ?? '', app: rawApp is Map ? IgnoredHotkeyApp.fromJson(Map<String, dynamic>.from(rawApp)) : IgnoredHotkeyApp.empty());
  }

  Map<String, dynamic> toJson() {
    return <String, dynamic>{'Slot': slot, 'App': app.toJson()};
  }

  _WindowManagerAssignment copy() {
    return _WindowManagerAssignment(slot: slot, app: IgnoredHotkeyApp(name: app.name, identity: app.identity, path: app.path, icon: app.icon));
  }
}

class _WindowGroupLayouts {
  static const String fullId = 'full';
  static _WindowGroupLayout get full => layouts.first;

  static final List<_WindowGroupLayout> layouts = <_WindowGroupLayout>[
    _WindowGroupLayout(
      id: 'full',
      titleKey: 'plugin_window_manager_group_layout_full',
      slotCount: 1,
      slots: <_WindowGroupSlot>[_WindowGroupSlot(id: 'full', titleKey: 'plugin_window_manager_group_slot_full', left: 0, top: 0, width: 1, height: 1)],
    ),
    _WindowGroupLayout(
      id: 'halves-horizontal',
      titleKey: 'plugin_window_manager_group_layout_halves_horizontal',
      slotCount: 2,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'left', titleKey: 'plugin_window_manager_group_slot_left', left: 0, top: 0, width: 0.5, height: 1),
        _WindowGroupSlot(id: 'right', titleKey: 'plugin_window_manager_group_slot_right', left: 0.5, top: 0, width: 0.5, height: 1),
      ],
    ),
    _WindowGroupLayout(
      id: 'halves-vertical',
      titleKey: 'plugin_window_manager_group_layout_halves_vertical',
      slotCount: 2,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'top', titleKey: 'plugin_window_manager_group_slot_top', left: 0, top: 0, width: 1, height: 0.5),
        _WindowGroupSlot(id: 'bottom', titleKey: 'plugin_window_manager_group_slot_bottom', left: 0, top: 0.5, width: 1, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-left-main',
      titleKey: 'plugin_window_manager_group_layout_three_left_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'left', titleKey: 'plugin_window_manager_group_slot_left', left: 0, top: 0, width: 0.5, height: 1),
        _WindowGroupSlot(id: 'rightTop', titleKey: 'plugin_window_manager_group_slot_right_top', left: 0.5, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'rightBottom', titleKey: 'plugin_window_manager_group_slot_right_bottom', left: 0.5, top: 0.5, width: 0.5, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-right-main',
      titleKey: 'plugin_window_manager_group_layout_three_right_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'leftTop', titleKey: 'plugin_window_manager_group_slot_left_top', left: 0, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'leftBottom', titleKey: 'plugin_window_manager_group_slot_left_bottom', left: 0, top: 0.5, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'right', titleKey: 'plugin_window_manager_group_slot_right', left: 0.5, top: 0, width: 0.5, height: 1),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-top-main',
      titleKey: 'plugin_window_manager_group_layout_three_top_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'top', titleKey: 'plugin_window_manager_group_slot_top', left: 0, top: 0, width: 1, height: 0.5),
        _WindowGroupSlot(id: 'bottomLeft', titleKey: 'plugin_window_manager_group_slot_bottom_left', left: 0, top: 0.5, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottomRight', titleKey: 'plugin_window_manager_group_slot_bottom_right', left: 0.5, top: 0.5, width: 0.5, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'three-bottom-main',
      titleKey: 'plugin_window_manager_group_layout_three_bottom_main',
      slotCount: 3,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'topLeft', titleKey: 'plugin_window_manager_group_slot_top_left', left: 0, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'topRight', titleKey: 'plugin_window_manager_group_slot_top_right', left: 0.5, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottom', titleKey: 'plugin_window_manager_group_slot_bottom', left: 0, top: 0.5, width: 1, height: 0.5),
      ],
    ),
    _WindowGroupLayout(
      id: 'quarters',
      titleKey: 'plugin_window_manager_group_layout_quarters',
      slotCount: 4,
      slots: <_WindowGroupSlot>[
        _WindowGroupSlot(id: 'topLeft', titleKey: 'plugin_window_manager_group_slot_top_left', left: 0, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'topRight', titleKey: 'plugin_window_manager_group_slot_top_right', left: 0.5, top: 0, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottomLeft', titleKey: 'plugin_window_manager_group_slot_bottom_left', left: 0, top: 0.5, width: 0.5, height: 0.5),
        _WindowGroupSlot(id: 'bottomRight', titleKey: 'plugin_window_manager_group_slot_bottom_right', left: 0.5, top: 0.5, width: 0.5, height: 0.5),
      ],
    ),
  ];

  static _WindowGroupLayout byId(String id) {
    for (final layout in layouts) {
      if (layout.id == id) {
        return layout;
      }
    }
    return full;
  }

  static List<_WindowGroupLayout> bySlotCount(int slotCount) {
    return layouts.where((layout) => layout.slotCount == slotCount).toList();
  }
}

class _WindowGroupLayout {
  final String id;
  final String titleKey;
  final int slotCount;
  final List<_WindowGroupSlot> slots;

  _WindowGroupLayout({required this.id, required this.titleKey, required this.slotCount, required this.slots});
}

class _WindowGroupSlot {
  final String id;
  final String titleKey;
  final double left;
  final double top;
  final double width;
  final double height;

  _WindowGroupSlot({required this.id, required this.titleKey, required this.left, required this.top, required this.width, required this.height});
}
